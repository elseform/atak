package screens

import (
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/noisethanks/atak/internal/config"
	"github.com/noisethanks/atak/internal/tui/style"
)

type settingsField int

const (
	fieldModsDir settingsField = iota
	fieldBackupDir
	fieldWorkers
	fieldBackupLevel
	fieldCount
)

// SettingsModel handles configuring user preferences.
type SettingsModel struct {
	cfg     *config.Config
	inputs  [4]textinput.Model // mods, backup, workers, backup-level
	focused settingsField
	errMsg  string
	width   int
	height  int
}

func NewSettings(cfg *config.Config) SettingsModel {
	mods := textinput.New()
	mods.SetValue(cfg.ModsDir)
	mods.Width = 60
	mods.Focus()

	backup := textinput.New()
	backup.SetValue(cfg.BackupDir)
	backup.Width = 60

	workers := textinput.New()
	workers.SetValue(strconv.Itoa(cfg.WorkerCount))
	workers.Width = 6

	backupLvl := textinput.New()
	backupLvl.SetValue(strconv.Itoa(cfg.BackupLevel))
	backupLvl.Width = 6

	return SettingsModel{
		cfg:     cfg,
		inputs:  [4]textinput.Model{mods, backup, workers, backupLvl},
		focused: fieldModsDir,
	}
}

func (m SettingsModel) Init() tea.Cmd { return textinput.Blink }

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focused = (m.focused + 1) % fieldCount
			m.refocus()
		case "shift+tab", "up":
			m.focused = (m.focused + fieldCount - 1) % fieldCount
			m.refocus()
		case "enter":
			return m.save()
		case "esc", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
		}
	}

	var cmd tea.Cmd
	switch m.focused {
	case fieldModsDir:
		m.inputs[0], cmd = m.inputs[0].Update(msg)
	case fieldBackupDir:
		m.inputs[1], cmd = m.inputs[1].Update(msg)
	case fieldWorkers:
		m.inputs[2], cmd = m.inputs[2].Update(msg)
	case fieldBackupLevel:
		m.inputs[3], cmd = m.inputs[3].Update(msg)
	}
	return m, cmd
}

func (m *SettingsModel) refocus() {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	switch m.focused {
	case fieldModsDir:
		m.inputs[0].Focus()
	case fieldBackupDir:
		m.inputs[1].Focus()
	case fieldWorkers:
		m.inputs[2].Focus()
	case fieldBackupLevel:
		m.inputs[3].Focus()
	}
}

func (m SettingsModel) save() (SettingsModel, tea.Cmd) {
	workers, err := strconv.Atoi(strings.TrimSpace(m.inputs[2].Value()))
	if err != nil || workers < 1 {
		workers = max(1, runtime.NumCPU()/4)
	}
	backupLevel, err := strconv.Atoi(strings.TrimSpace(m.inputs[3].Value()))
	if err != nil || backupLevel < 1 || backupLevel > 9 {
		backupLevel = m.cfg.BackupLevel
	}
	updated := *m.cfg
	updated.ModsDir = strings.TrimSpace(m.inputs[0].Value())
	updated.BackupDir = strings.TrimSpace(m.inputs[1].Value())
	updated.WorkerCount = workers
	updated.BackupLevel = backupLevel
	return m, func() tea.Msg {
		return NavigateMsg{To: NavSaveConfig, Data: &updated}
	}
}

func (m SettingsModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Settings") + "\n\n")

	rows := []struct {
		label   string
		field   settingsField
		content string
		hint    string
	}{
		{"GAMMA Mods Directory", fieldModsDir, m.inputs[0].View(), ""},
		{"Backup Directory", fieldBackupDir, m.inputs[1].View(), ""},
		{"Worker Threads", fieldWorkers, m.inputs[2].View(), "Conservative default (CPU/4). Increase if compression feels slow and your system has headroom."},
		{"Backup Compression Level", fieldBackupLevel, m.inputs[3].View(),
			"1–9  ·  3 = Fast  ·  6 = Balanced (default)  ·  9 = Maximum"},
	}

	for _, r := range rows {
		label := style.StyleBody.Render(r.label)
		if m.focused == r.field {
			label = style.StyleSelected.Render(r.label)
		}
		b.WriteString(label + "\n" + r.content + "\n")
		if r.hint != "" {
			b.WriteString(style.StyleMuted.Render(r.hint) + "\n")
		}
		b.WriteString("\n")
	}

	if m.errMsg != "" {
		b.WriteString(style.StyleDanger.Render(m.errMsg) + "\n\n")
	}

	b.WriteString(style.KeyHint("tab", "next") + "  ")
	b.WriteString(style.KeyHint("enter", "save") + "  ")
	b.WriteString(style.KeyHint("q", "cancel"))
	return b.String()
}

func (m *SettingsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
