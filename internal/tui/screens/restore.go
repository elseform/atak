package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/noisethanks/stalker-tex/internal/archive"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/tools"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

type restoreState int

const (
	restoreStatePickArchive restoreState = iota
	restoreStatePickMod
	restoreStateConfirm
	restoreStateRunning
	restoreStateDone
)

type modsListedMsg struct {
	mods       []string
	backupName string
	backupPath string
}

type restoreTickMsg struct {
	prog   archive.ProgressMsg
	progCh <-chan archive.ProgressMsg
	doneCh <-chan error
}
type restoreFinishedMsg struct{ err error }
type restoreSizeMsg struct{ bytes int64 }

type modItem struct{ name string }

func (i modItem) Title() string       { return i.name }
func (i modItem) Description() string { return "" }
func (i modItem) FilterValue() string { return i.name }

// RestoreModel handles archive selection, mod search, confirm, and progress.
type RestoreModel struct {
	cfg         *config.Config
	tools       *tools.EmbeddedTools
	state       restoreState
	backups     []archive.BackupInfo
	backupCur   int
	modList     list.Model
	selectedMod string
	selectedBak archive.BackupInfo
	bar         progress.Model
	spinner     spinner.Model
	curPct      float64
	curFile     string
	restoreSize int64
	errMsg      string
	statusMsg   string
	width       int
	height      int
}

func NewRestore(cfg *config.Config, t *tools.EmbeddedTools) RestoreModel {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	delegate.ShowDescription = false
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("#E8A020"))
	l := list.New(nil, delegate, 60, 20)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	bar := progress.New(progress.WithDefaultGradient())
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return RestoreModel{
		cfg:     cfg,
		tools:   t,
		modList: l,
		bar:     bar,
		spinner: sp,
	}
}

func (m RestoreModel) Init() tea.Cmd {
	return m.loadBackups()
}

func (m RestoreModel) loadBackups() tea.Cmd {
	dir := m.cfg.BackupDir
	return func() tea.Msg {
		backups, _ := archive.ListBackups(dir)
		return backupsLoadedMsg{backups: backups}
	}
}

func (m RestoreModel) Update(msg tea.Msg) (RestoreModel, tea.Cmd) {
	switch msg := msg.(type) {
	case backupsLoadedMsg:
		m.backups = msg.backups
		return m, nil

	case modsListedMsg:
		items := make([]list.Item, len(msg.mods))
		for i, name := range msg.mods {
			items[i] = modItem{name: name}
		}
		m.modList.SetItems(items)
		m.selectedBak = archive.BackupInfo{
			Name: msg.backupName,
			Path: msg.backupPath,
		}
		for _, b := range m.backups {
			if b.Path == msg.backupPath {
				m.selectedBak = b
				break
			}
		}
		m.state = restoreStatePickMod
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.state == restoreStateRunning {
			return m, cmd
		}
		return m, nil

	case restoreSizeMsg:
		m.restoreSize = msg.bytes
		if m.state == restoreStateRunning {
			return m, pollRestoreSize(filepath.Join(m.cfg.ModsDir, m.selectedMod))
		}
		return m, nil

	case restoreTickMsg:
		m.curPct = float64(msg.prog.Percent) / 100.0
		m.curFile = msg.prog.CurrentFile
		progCh, doneCh := msg.progCh, msg.doneCh
		return m, tea.Batch(
			m.bar.SetPercent(m.curPct),
			func() tea.Msg { return readRestoreProgress(progCh, doneCh) },
		)

	case restoreFinishedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("Restored %q successfully.", m.selectedMod)
		}
		m.state = restoreStateDone
		return m, nil

	case progress.FrameMsg:
		updated, cmd := m.bar.Update(msg)
		m.bar = updated.(progress.Model)
		return m, cmd

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	if m.state == restoreStatePickMod {
		var cmd tea.Cmd
		m.modList, cmd = m.modList.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m RestoreModel) handleKey(msg tea.KeyMsg) (RestoreModel, tea.Cmd) {
	switch m.state {
	case restoreStatePickArchive:
		switch msg.String() {
		case "up", "k":
			if m.backupCur > 0 {
				m.backupCur--
			}
		case "down", "j":
			if m.backupCur < len(m.backups)-1 {
				m.backupCur++
			}
		case "enter":
			if len(m.backups) == 0 {
				return m, nil
			}
			bk := m.backups[m.backupCur]
			szPath := m.tools.SevenZipPath
			return m, func() tea.Msg {
				mods, err := archive.ListMods(szPath, bk.Path)
				if err != nil {
					return modsListedMsg{backupName: bk.Name, backupPath: bk.Path}
				}
				return modsListedMsg{mods: mods, backupName: bk.Name, backupPath: bk.Path}
			}
		case "esc", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
		}

	case restoreStatePickMod:
		if msg.String() == "enter" {
			if sel := m.modList.SelectedItem(); sel != nil {
				m.selectedMod = sel.(modItem).name
				m.state = restoreStateConfirm
				return m, nil
			}
		}
		if msg.String() == "esc" {
			m.state = restoreStatePickArchive
			return m, nil
		}
		var cmd tea.Cmd
		m.modList, cmd = m.modList.Update(msg)
		return m, cmd

	case restoreStateConfirm:
		switch msg.String() {
		case "y", "enter":
			return m.startRestore()
		case "n", "esc":
			m.state = restoreStatePickMod
		}

	case restoreStateDone:
		return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
	}
	return m, nil
}

func (m RestoreModel) startRestore() (RestoreModel, tea.Cmd) {
	m.state = restoreStateRunning
	m.curPct = 0
	m.restoreSize = 0
	szPath := m.tools.SevenZipPath
	archivePath := m.selectedBak.Path
	modName := m.selectedMod
	modDir := m.cfg.ModsDir

	return m, tea.Batch(
		m.spinner.Tick,
		pollRestoreSize(filepath.Join(modDir, modName)),
		func() tea.Msg {
			progCh, doneCh := archive.Restore(szPath, archivePath, modName, modDir)
			return readRestoreProgress(progCh, doneCh)
		},
	)
}

func pollRestoreSize(dir string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Second)
		var total int64
		_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			total += info.Size()
			return nil
		})
		return restoreSizeMsg{bytes: total}
	}
}

func readRestoreProgress(progCh <-chan archive.ProgressMsg, doneCh <-chan error) tea.Msg {
	select {
	case p, ok := <-progCh:
		if !ok {
			return restoreFinishedMsg{err: <-doneCh}
		}
		return restoreTickMsg{prog: p, progCh: progCh, doneCh: doneCh}
	case err := <-doneCh:
		return restoreFinishedMsg{err: err}
	}
}

func (m RestoreModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Restore Mod") + "\n")

	switch m.state {
	case restoreStatePickArchive:
		if len(m.backups) == 0 {
			b.WriteString(style.StyleMuted.Render("No backups found in "+m.cfg.BackupDir) + "\n")
			b.WriteString("\n" + style.KeyHint("q", "back"))
			return b.String()
		}
		b.WriteString(style.StyleBody.Render("Select backup archive:") + "\n\n")
		for i, bk := range m.backups {
			prefix := "  "
			if i == m.backupCur {
				prefix = style.StyleSelected.Render("▶ ")
			}
			b.WriteString(fmt.Sprintf("%s%s  %s\n",
				prefix, bk.Name, bk.ModTime.Format("2006-01-02 15:04")))
		}
		b.WriteString("\n" + style.KeyHint("enter", "list mods") + "  " + style.KeyHint("q", "back"))

	case restoreStatePickMod:
		b.WriteString(style.StyleMuted.Render("Archive: "+m.selectedBak.Name) + "\n")
		b.WriteString(m.modList.View())
		b.WriteString(style.KeyHint("enter", "select") + "  " + style.KeyHint("esc", "back"))

	case restoreStateConfirm:
		b.WriteString(style.StyleBody.Render(fmt.Sprintf(
			"Restore %q from %q?\n\nThis will overwrite existing files.",
			m.selectedMod, m.selectedBak.Name,
		)) + "\n\n")
		b.WriteString(style.KeyHint("y / enter", "yes") + "  " + style.KeyHint("n / esc", "no"))

	case restoreStateRunning:
		b.WriteString(style.StyleBody.Render(m.spinner.View()+" Restoring "+m.selectedMod+"…") + "\n")
		b.WriteString(style.StyleMuted.Render("Mod: "+formatBytes(m.restoreSize)) + "\n")
		b.WriteString(m.bar.ViewAs(m.curPct) + "\n")
		if m.curFile != "" {
			const label = "  Processing: "
			maxPath := m.width - len(label)
			b.WriteString(style.StyleMuted.Render(label+truncateLeft(m.curFile, maxPath)) + "\n")
		}

	case restoreStateDone:
		if m.errMsg != "" {
			b.WriteString(style.StyleDanger.Render("Error: "+m.errMsg) + "\n")
		} else {
			b.WriteString(style.StyleSuccess.Render(m.statusMsg) + "\n")
		}
		b.WriteString("\n" + style.KeyHint("any key", "main menu"))
	}

	return b.String()
}

func (m *RestoreModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.modList.SetSize(w-4, h-3)
	m.bar.Width = w - 4
}
