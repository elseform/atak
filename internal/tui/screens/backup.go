package screens

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/noisethanks/stalker-tex/internal/archive"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/tools"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

type backupState int

const (
	backupStateList backupState = iota
	backupStateCreating
	backupStateVerifying
	backupStateConfirmDelete
)

// archiveTickMsg carries one progress update and the channels for the next read.
type archiveTickMsg struct {
	prog   archive.ProgressMsg
	progCh <-chan archive.ProgressMsg
	doneCh <-chan error
}

// archiveFinishedMsg signals the archive operation is complete.
type archiveFinishedMsg struct{ err error }

type backupsLoadedMsg struct{ backups []archive.BackupInfo }
type verifyDoneMsg struct{ err error }

// BackupModel manages listing, creating, deleting, and verifying backups.
type BackupModel struct {
	cfg       *config.Config
	tools     *tools.EmbeddedTools
	state     backupState
	backups   []archive.BackupInfo
	cursor    int
	bar       progress.Model
	curPct    float64
	curFile   string
	statusMsg string
	errMsg    string
	width     int
	height    int
}

func NewBackup(cfg *config.Config, t *tools.EmbeddedTools) BackupModel {
	bar := progress.New(progress.WithDefaultGradient())
	return BackupModel{cfg: cfg, tools: t, bar: bar}
}

func (m BackupModel) Init() tea.Cmd {
	return m.loadBackups()
}

func (m BackupModel) loadBackups() tea.Cmd {
	dir := m.cfg.BackupDir
	return func() tea.Msg {
		backups, _ := archive.ListBackups(dir)
		return backupsLoadedMsg{backups: backups}
	}
}

func (m BackupModel) Update(msg tea.Msg) (BackupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case backupsLoadedMsg:
		m.backups = msg.backups
		if m.cursor >= len(m.backups) && len(m.backups) > 0 {
			m.cursor = len(m.backups) - 1
		}
		return m, nil

	case archiveTickMsg:
		m.curPct = float64(msg.prog.Percent) / 100.0
		m.curFile = msg.prog.CurrentFile
		progCh, doneCh := msg.progCh, msg.doneCh
		return m, tea.Batch(
			m.bar.SetPercent(m.curPct),
			func() tea.Msg { return readArchiveProgress(progCh, doneCh) },
		)

	case archiveFinishedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.statusMsg = "Backup complete."
		}
		m.state = backupStateList
		return m, m.loadBackups()

	case verifyDoneMsg:
		m.state = backupStateList
		if msg.err != nil {
			m.errMsg = "Verify failed: " + msg.err.Error()
		} else {
			m.statusMsg = "Archive verified OK."
		}
		return m, nil

	case progress.FrameMsg:
		updated, cmd := m.bar.Update(msg)
		m.bar = updated.(progress.Model)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg.String())
	}
	return m, nil
}

func (m BackupModel) handleKey(key string) (BackupModel, tea.Cmd) {
	m.statusMsg = ""
	m.errMsg = ""

	switch m.state {
	case backupStateList:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.backups)-1 {
				m.cursor++
			}
		case "n":
			return m.startBackup()
		case "d":
			if len(m.backups) > 0 {
				m.state = backupStateConfirmDelete
			}
		case "v":
			if len(m.backups) > 0 {
				return m.startVerify()
			}
		case "esc", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
		}

	case backupStateConfirmDelete:
		switch key {
		case "y":
			if len(m.backups) > 0 {
				path := m.backups[m.cursor].Path
				if err := archive.Delete(path); err != nil {
					m.errMsg = err.Error()
				} else {
					m.statusMsg = "Deleted."
					if m.cursor > 0 {
						m.cursor--
					}
				}
			}
			m.state = backupStateList
			return m, m.loadBackups()
		case "n", "esc":
			m.state = backupStateList
		}
	}
	return m, nil
}

func (m BackupModel) startBackup() (BackupModel, tea.Cmd) {
	m.state = backupStateCreating
	m.curPct = 0
	szPath := m.tools.SevenZipPath
	modsDir := m.cfg.ModsDir
	outPath := filepath.Join(m.cfg.BackupDir, fmt.Sprintf(
		"gamma-backup-%s.7z", time.Now().Format("2006-01-02-150405")))

	return m, func() tea.Msg {
		progCh, doneCh := archive.Backup(szPath, modsDir, outPath)
		return readArchiveProgress(progCh, doneCh)
	}
}

func (m BackupModel) startVerify() (BackupModel, tea.Cmd) {
	m.state = backupStateVerifying
	szPath := m.tools.SevenZipPath
	archivePath := m.backups[m.cursor].Path
	return m, func() tea.Msg {
		return verifyDoneMsg{err: archive.Verify(szPath, archivePath)}
	}
}

func readArchiveProgress(progCh <-chan archive.ProgressMsg, doneCh <-chan error) tea.Msg {
	select {
	case p, ok := <-progCh:
		if !ok {
			return archiveFinishedMsg{err: <-doneCh}
		}
		return archiveTickMsg{prog: p, progCh: progCh, doneCh: doneCh}
	case err := <-doneCh:
		return archiveFinishedMsg{err: err}
	}
}

func (m BackupModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Backup Manager") + "\n\n")

	switch m.state {
	case backupStateCreating:
		b.WriteString(style.StyleBody.Render("Creating backup…") + "\n")
		b.WriteString(m.bar.ViewAs(m.curPct) + "\n")
		if m.curFile != "" {
			b.WriteString(style.StyleMuted.Render("  "+m.curFile) + "\n")
		}
		return b.String()

	case backupStateVerifying:
		b.WriteString(style.StyleBody.Render("Verifying archive…") + "\n")
		return b.String()

	case backupStateConfirmDelete:
		if len(m.backups) > 0 {
			b.WriteString(style.StyleWarning.Render(
				"Delete "+m.backups[m.cursor].Name+"? (y/n)",
			) + "\n")
		}
		return b.String()
	}

	if m.errMsg != "" {
		b.WriteString(style.StyleDanger.Render("Error: "+m.errMsg) + "\n\n")
	}
	if m.statusMsg != "" {
		b.WriteString(style.StyleSuccess.Render(m.statusMsg) + "\n\n")
	}

	if len(m.backups) == 0 {
		b.WriteString(style.StyleMuted.Render("No backups found in "+m.cfg.BackupDir) + "\n\n")
	} else {
		for i, bk := range m.backups {
			prefix := "  "
			if i == m.cursor {
				prefix = style.StyleSelected.Render("▶ ")
			}
			b.WriteString(fmt.Sprintf("%s%-40s  %s  %s\n",
				prefix, bk.Name, formatBytes(bk.Size),
				bk.ModTime.Format("2006-01-02 15:04"),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(style.KeyHint("n", "new backup") + "  ")
	b.WriteString(style.KeyHint("d", "delete") + "  ")
	b.WriteString(style.KeyHint("v", "verify") + "  ")
	b.WriteString(style.KeyHint("q", "back"))
	return b.String()
}

func (m *BackupModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.bar.Width = w - 4
}
