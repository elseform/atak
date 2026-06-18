package screens

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/noisethanks/stalker-tex/internal/archive"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/tools"
	"github.com/noisethanks/stalker-tex/internal/tui/components"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

type backupState int

const (
	backupStateMenu backupState = iota
	backupStatePickArchive
	backupStateCreating
	backupStatePickMod
	backupStateConfirmRestore
	backupStateRestoring
	backupStateRestoreDone
	backupStateConfirmRestoreAll
	backupStateRestoringAll
	backupStateRestoringAllDone
	backupStateVerifying
	backupStateConfirmDelete
)

type backupPendingAction int

const (
	pendingRestoreSingle backupPendingAction = iota
	pendingRestoreAll
	pendingVerify
	pendingDelete
)

// archiveTickMsg carries one progress update and the channels for the next read.
type archiveTickMsg struct {
	prog   archive.ProgressMsg
	progCh <-chan archive.ProgressMsg
	doneCh <-chan error
}

// archiveFinishedMsg signals the backup creation operation is complete.
type archiveFinishedMsg struct{ err error }

type backupsLoadedMsg struct{ backups []archive.BackupInfo }
type verifyDoneMsg struct{ err error }
type backupSizeMsg struct{ bytes int64 }

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

var backupMenuItems = []string{
	"Create New Backup",
	"Restore Single Mod",
	"Restore All",
	"Verify Archive",
	"Delete Backup",
}

// BackupModel manages all archive operations: create, restore single, restore all, verify, delete.
type BackupModel struct {
	cfg           *config.Config
	tools         *tools.EmbeddedTools
	state         backupState
	backups       []archive.BackupInfo
	menuCursor    int
	cursor        int
	pendingAction backupPendingAction
	modPicker     components.ModPicker
	selectedMod   string
	selectedBak   archive.BackupInfo
	bar           progress.Model
	spinner       spinner.Model
	curPct        float64
	curFile       string
	archiveSize   int64
	restoreSize   int64
	outPath       string
	statusMsg     string
	errMsg        string
	width         int
	height        int
}

func NewBackup(cfg *config.Config, t *tools.EmbeddedTools) BackupModel {
	bar := progress.New(progress.WithDefaultGradient())
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return BackupModel{
		cfg:       cfg,
		tools:     t,
		bar:       bar,
		spinner:   sp,
		modPicker: components.NewModPicker(nil, 60, 20),
	}
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

	case modsListedMsg:
		m.modPicker = components.NewModPicker(msg.mods, m.width-4, m.height-6)
		m.selectedBak = archive.BackupInfo{Name: msg.backupName, Path: msg.backupPath}
		for _, b := range m.backups {
			if b.Path == msg.backupPath {
				m.selectedBak = b
				break
			}
		}
		m.state = backupStatePickMod
		return m, nil

	case components.ModSelectedMsg:
		m.selectedMod = msg.Mod
		m.state = backupStateConfirmRestore
		return m, nil

	case components.ModPickerCancelledMsg:
		m.state = backupStatePickArchive
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.state == backupStateCreating || m.state == backupStateRestoring || m.state == backupStateRestoringAll {
			return m, cmd
		}
		return m, nil

	case backupSizeMsg:
		m.archiveSize = msg.bytes
		if m.state == backupStateCreating {
			return m, pollBackupSize(m.outPath)
		}
		return m, nil

	case restoreSizeMsg:
		m.restoreSize = msg.bytes
		if m.state == backupStateRestoring {
			return m, pollRestoreSize(filepath.Join(m.cfg.ModsDir, m.selectedMod))
		}
		if m.state == backupStateRestoringAll {
			return m, pollRestoreSize(m.cfg.ModsDir)
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
		m.state = backupStateMenu
		return m, m.loadBackups()

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
			if m.state == backupStateRestoringAll {
				m.statusMsg = "Restore all complete."
			} else {
				m.statusMsg = fmt.Sprintf("Restored %q successfully.", m.selectedMod)
			}
		}
		if m.state == backupStateRestoringAll {
			m.state = backupStateRestoringAllDone
		} else {
			m.state = backupStateRestoreDone
		}
		return m, nil

	case verifyDoneMsg:
		m.state = backupStateMenu
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
		if m.state == backupStatePickMod {
			var cmd tea.Cmd
			m.modPicker, cmd = m.modPicker.Update(msg)
			return m, cmd
		}
		return m.handleKey(msg.String())
	}

	if m.state == backupStatePickMod {
		var cmd tea.Cmd
		m.modPicker, cmd = m.modPicker.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m BackupModel) handleKey(key string) (BackupModel, tea.Cmd) {
	switch m.state {
	case backupStateMenu:
		m.statusMsg = ""
		m.errMsg = ""
		switch key {
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < len(backupMenuItems)-1 {
				m.menuCursor++
			}
		case "enter", " ":
			m.cursor = 0
			switch m.menuCursor {
			case 0:
				return m.startBackup()
			case 1:
				return m.selectArchiveOrPick(pendingRestoreSingle)
			case 2:
				return m.selectArchiveOrPick(pendingRestoreAll)
			case 3:
				return m.selectArchiveOrPick(pendingVerify)
			case 4:
				return m.selectArchiveOrPick(pendingDelete)
			}
		case "esc", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
		}

	case backupStatePickArchive:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.backups)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.backups) == 0 {
				return m, nil
			}
			return m.handleSelectedArchive(m.backups[m.cursor])
		case "esc", "q":
			m.state = backupStateMenu
		}

	case backupStateConfirmRestore:
		switch key {
		case "y", "enter":
			return m.startRestore()
		case "n", "esc":
			m.state = backupStatePickMod
		}

	case backupStateRestoreDone, backupStateRestoringAllDone:
		m.state = backupStateMenu
		return m, m.loadBackups()

	case backupStateConfirmRestoreAll:
		switch key {
		case "y", "enter":
			return m.startRestoreAll()
		case "n", "esc":
			m.state = backupStatePickArchive
		}

	case backupStateConfirmDelete:
		switch key {
		case "y":
			if err := archive.Delete(m.selectedBak.Path); err != nil {
				m.errMsg = err.Error()
			} else {
				m.statusMsg = "Deleted."
				if m.cursor > 0 {
					m.cursor--
				}
			}
			m.state = backupStateMenu
			return m, m.loadBackups()
		case "n", "esc":
			m.state = backupStatePickArchive
		}
	}
	return m, nil
}

func (m BackupModel) selectArchiveOrPick(action backupPendingAction) (BackupModel, tea.Cmd) {
	m.pendingAction = action
	if len(m.backups) == 0 {
		return m, nil
	}
	if len(m.backups) == 1 {
		m.cursor = 0
		return m.handleSelectedArchive(m.backups[0])
	}
	m.state = backupStatePickArchive
	return m, nil
}

func (m BackupModel) handleSelectedArchive(bk archive.BackupInfo) (BackupModel, tea.Cmd) {
	switch m.pendingAction {
	case pendingRestoreSingle:
		szPath := m.tools.SevenZipPath
		return m, func() tea.Msg {
			mods, err := archive.ListMods(szPath, bk.Path)
			if err != nil {
				return modsListedMsg{backupName: bk.Name, backupPath: bk.Path}
			}
			return modsListedMsg{mods: mods, backupName: bk.Name, backupPath: bk.Path}
		}
	case pendingRestoreAll:
		m.selectedBak = bk
		m.state = backupStateConfirmRestoreAll
	case pendingVerify:
		m.selectedBak = bk
		return m.startVerify()
	case pendingDelete:
		m.selectedBak = bk
		m.state = backupStateConfirmDelete
	}
	return m, nil
}

func (m BackupModel) startBackup() (BackupModel, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.state = backupStateCreating
	m.curPct = 0
	m.archiveSize = 0
	szPath := m.tools.SevenZipPath
	modsDir := m.cfg.ModsDir
	outPath := filepath.Join(m.cfg.BackupDir, fmt.Sprintf(
		"gamma-backup-%s.7z", time.Now().Format("2006-01-02-150405")))
	m.outPath = outPath

	return m, tea.Batch(
		func() tea.Msg { return OperationStartedMsg{Cancel: cancel} },
		m.spinner.Tick,
		pollBackupSize(outPath),
		func() tea.Msg {
			progCh, doneCh := archive.Backup(ctx, szPath, modsDir, outPath, m.cfg.BackupLevel)
			return readArchiveProgress(progCh, doneCh)
		},
	)
}

func (m BackupModel) startRestore() (BackupModel, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.state = backupStateRestoring
	m.curPct = 0
	m.restoreSize = 0
	szPath := m.tools.SevenZipPath
	archivePath := m.selectedBak.Path
	modName := m.selectedMod
	modDir := m.cfg.ModsDir

	return m, tea.Batch(
		func() tea.Msg { return OperationStartedMsg{Cancel: cancel} },
		m.spinner.Tick,
		pollRestoreSize(filepath.Join(modDir, modName)),
		func() tea.Msg {
			progCh, doneCh := archive.Restore(ctx, szPath, archivePath, modName, modDir)
			return readRestoreProgress(progCh, doneCh)
		},
	)
}

func (m BackupModel) startRestoreAll() (BackupModel, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.state = backupStateRestoringAll
	m.curPct = 0
	m.restoreSize = 0
	szPath := m.tools.SevenZipPath
	archivePath := m.selectedBak.Path
	modsParentDir := filepath.Dir(m.cfg.ModsDir)

	return m, tea.Batch(
		func() tea.Msg { return OperationStartedMsg{Cancel: cancel} },
		m.spinner.Tick,
		pollRestoreSize(m.cfg.ModsDir),
		func() tea.Msg {
			progCh, doneCh := archive.RestoreAll(ctx, szPath, archivePath, modsParentDir)
			return readRestoreProgress(progCh, doneCh)
		},
	)
}

func pollBackupSize(path string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Second)
		info, err := os.Stat(path)
		if err != nil {
			return backupSizeMsg{}
		}
		return backupSizeMsg{bytes: info.Size()}
	}
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

func (m BackupModel) startVerify() (BackupModel, tea.Cmd) {
	m.state = backupStateVerifying
	szPath := m.tools.SevenZipPath
	archivePath := m.selectedBak.Path
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

func (m BackupModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Backup Manager") + "\n\n")

	switch m.state {
	case backupStateMenu:
		b.WriteString(style.StyleMuted.Render("Backups in "+m.cfg.BackupDir+":") + "\n\n")
		if len(m.backups) == 0 {
			b.WriteString(style.StyleMuted.Render("  No backups found") + "\n")
		} else {
			for _, bk := range m.backups {
				b.WriteString(fmt.Sprintf("  %-40s  %s  %s\n",
					bk.Name, formatBytes(bk.Size), bk.ModTime.Format("Jan 02 15:04")))
			}
		}
		b.WriteString("\n")
		if m.errMsg != "" {
			b.WriteString(style.StyleDanger.Render("Error: "+m.errMsg) + "\n\n")
		}
		if m.statusMsg != "" {
			b.WriteString(style.StyleSuccess.Render(m.statusMsg) + "\n\n")
		}
		for i, item := range backupMenuItems {
			if i == m.menuCursor {
				b.WriteString(style.StyleSelected.Render("▶ "+item) + "\n")
			} else {
				b.WriteString(style.StyleBody.Render("  "+item) + "\n")
			}
		}
		b.WriteString("\n" + style.KeyHint("↑↓", "navigate") + "  " + style.KeyHint("enter", "select") + "  " + style.KeyHint("q", "back"))

	case backupStatePickArchive:
		if len(m.backups) == 0 {
			b.WriteString(style.StyleMuted.Render("No backups found in "+m.cfg.BackupDir) + "\n\n")
			b.WriteString(style.KeyHint("esc", "back"))
			return b.String()
		}
		b.WriteString(style.StyleBody.Render("Select backup:") + "\n\n")
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
		b.WriteString("\n" + style.KeyHint("enter", "select") + "  " + style.KeyHint("esc", "back"))

	case backupStateCreating:
		b.WriteString(style.StyleBody.Render(m.spinner.View()+" Creating backup…") + "\n")
		b.WriteString(style.StyleMuted.Render("Archive: "+formatBytes(m.archiveSize)) + "\n")
		b.WriteString(m.bar.ViewAs(m.curPct) + "\n")
		if m.curFile != "" {
			const label = "  Processing: "
			maxPath := m.width - len(label)
			b.WriteString(style.StyleMuted.Render(label+truncateLeft(m.curFile, maxPath)) + "\n")
		}

	case backupStatePickMod:
		b.WriteString(style.StyleMuted.Render("Archive: "+m.selectedBak.Name) + "\n")
		b.WriteString(m.modPicker.View())

	case backupStateConfirmRestore:
		b.WriteString(style.StyleBody.Render(fmt.Sprintf(
			"Restore %q from %q?\n\nThis will overwrite existing files.",
			m.selectedMod, m.selectedBak.Name,
		)) + "\n\n")
		b.WriteString(style.KeyHint("y/enter", "yes") + "  " + style.KeyHint("n/esc", "no"))

	case backupStateRestoring:
		b.WriteString(style.StyleBody.Render(m.spinner.View()+" Restoring "+m.selectedMod+"…") + "\n")
		b.WriteString(style.StyleMuted.Render("Mod: "+formatBytes(m.restoreSize)) + "\n")
		b.WriteString(m.bar.ViewAs(m.curPct) + "\n")
		if m.curFile != "" {
			const label = "  Processing: "
			maxPath := m.width - len(label)
			b.WriteString(style.StyleMuted.Render(label+truncateLeft(m.curFile, maxPath)) + "\n")
		}

	case backupStateRestoreDone:
		if m.errMsg != "" {
			b.WriteString(style.StyleDanger.Render("Error: "+m.errMsg) + "\n")
		} else {
			b.WriteString(style.StyleSuccess.Render(m.statusMsg) + "\n")
		}
		b.WriteString("\n" + style.KeyHint("any key", "back"))

	case backupStateConfirmRestoreAll:
		modsParentDir := filepath.Dir(m.cfg.ModsDir)
		b.WriteString(style.StyleWarning.Render(fmt.Sprintf(
			"Extract entire archive %q to %q?\n\nThis will overwrite all existing mod files.",
			m.selectedBak.Name, modsParentDir,
		)) + "\n\n")
		b.WriteString(style.KeyHint("y/enter", "yes") + "  " + style.KeyHint("n/esc", "no"))

	case backupStateRestoringAll:
		b.WriteString(style.StyleBody.Render(m.spinner.View()+" Restoring all mods…") + "\n")
		b.WriteString(style.StyleMuted.Render("Mods dir: "+formatBytes(m.restoreSize)) + "\n")
		b.WriteString(m.bar.ViewAs(m.curPct) + "\n")
		if m.curFile != "" {
			const label = "  Processing: "
			maxPath := m.width - len(label)
			b.WriteString(style.StyleMuted.Render(label+truncateLeft(m.curFile, maxPath)) + "\n")
		}

	case backupStateRestoringAllDone:
		if m.errMsg != "" {
			b.WriteString(style.StyleDanger.Render("Error: "+m.errMsg) + "\n")
		} else {
			b.WriteString(style.StyleSuccess.Render(m.statusMsg) + "\n")
		}
		b.WriteString("\n" + style.KeyHint("any key", "back"))

	case backupStateVerifying:
		b.WriteString(style.StyleBody.Render("Verifying "+m.selectedBak.Name+"…") + "\n")

	case backupStateConfirmDelete:
		b.WriteString(style.StyleWarning.Render(
			"Delete "+m.selectedBak.Name+"? (y/n)",
		) + "\n")
	}

	return b.String()
}

func (m *BackupModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.bar.Width = w - 4
	m.modPicker.SetSize(w-4, h-6)
}

// truncateLeft shortens s from the left to maxLen, prefixing "..." if truncated.
func truncateLeft(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[len(s)-maxLen:]
	}
	return "..." + s[len(s)-(maxLen-3):]
}
