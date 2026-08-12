package screens

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/atak/internal/config"
	"github.com/noisethanks/atak/internal/tui/style"
)

func detectModsDir(home string) string {
	if home == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(home, "gamma/mo2/mods"),
		filepath.Join(home, "Games/gamma/mo2/mods"),
		filepath.Join(home, "Games/Anomaly/mods"),
	}
	for _, c := range candidates {
		if stat, err := os.Stat(c); err == nil && stat.IsDir() {
			return c
		}
	}
	return ""
}

// WelcomeModel handles first-run path configuration.
type WelcomeModel struct {
	cfg       *config.Config
	modsInput textinput.Model
	bkupInput textinput.Model
	focused   int // 0 = mods, 1 = backup
	width     int
	height    int
	err       string
}

func NewWelcome(cfg *config.Config) WelcomeModel {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/home/user"
	}

	mods := textinput.New()
	mods.Placeholder = filepath.Join(home, "Games/Anomaly/mods")
	val := cfg.ModsDir
	if val == "" {
		val = detectModsDir(home)
	}
	mods.SetValue(val)
	mods.Focus()
	mods.Width = 60

	bkup := textinput.New()
	bkup.Placeholder = filepath.Join(home, "backups")
	bkupVal := cfg.BackupDir
	if bkupVal == "" {
		bkupVal = filepath.Join(home, "gamma/backups")
	}
	bkup.SetValue(bkupVal)
	bkup.Width = 60

	return WelcomeModel{
		cfg:       cfg,
		modsInput: mods,
		bkupInput: bkup,
	}
}

func (m WelcomeModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m WelcomeModel) Update(msg tea.Msg) (WelcomeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			m.focused = (m.focused + 1) % 2
			if m.focused == 0 {
				m.modsInput.Focus()
				m.bkupInput.Blur()
			} else {
				m.bkupInput.Focus()
				m.modsInput.Blur()
			}
		case "shift+tab", "up":
			m.focused = (m.focused + 1) % 2
			if m.focused == 0 {
				m.modsInput.Focus()
				m.bkupInput.Blur()
			} else {
				m.bkupInput.Focus()
				m.modsInput.Blur()
			}
		case "enter":
			if m.focused == 0 {
				// Move to backup field.
				m.focused = 1
				m.bkupInput.Focus()
				m.modsInput.Blur()
				return m, nil
			}
			// Validate and save.
			modsDir := strings.TrimSpace(m.modsInput.Value())
			bkupDir := strings.TrimSpace(m.bkupInput.Value())
			if modsDir == "" {
				m.err = "Mods directory is required."
				return m, nil
			}
			m.cfg.ModsDir = modsDir
			m.cfg.BackupDir = bkupDir
			return m, func() tea.Msg {
				return NavigateMsg{To: NavSaveConfig, Data: m.cfg}
			}
		case "ctrl+c", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavQuit} }
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	if m.focused == 0 {
		m.modsInput, cmd = m.modsInput.Update(msg)
	} else {
		m.bkupInput, cmd = m.bkupInput.Update(msg)
	}
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m WelcomeModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("atak") + "\n")
	b.WriteString(style.StyleSubtitle.Render("S.T.A.L.K.E.R. Anomaly texture compressor & backup tool") + "\n\n")
	b.WriteString(style.StyleBody.Render("Welcome! Let's set up your paths before we begin.") + "\n\n")

	b.WriteString(style.StyleSelected.Render("Anomaly Mods Directory") + "\n")
	b.WriteString(m.modsInput.View() + "\n\n")

	b.WriteString(style.StyleSelected.Render("Backup Directory") + "\n")
	b.WriteString(m.bkupInput.View() + "\n\n")

	if m.err != "" {
		b.WriteString(style.StyleDanger.Render("  "+m.err) + "\n\n")
	}

	b.WriteString(style.StyleKeyHint.Render("tab") + style.StyleMuted.Render(" next field  ") +
		style.StyleKeyHint.Render("enter") + style.StyleMuted.Render(" confirm  ") +
		style.StyleKeyHint.Render("q") + style.StyleMuted.Render(" quit"))

	return b.String()
}

func (m *WelcomeModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
