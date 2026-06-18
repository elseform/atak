package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

// FirstRunModel is shown exactly once when profiles.json is created for the first time.
type FirstRunModel struct {
	configDir string
	width     int
	height    int
}

func NewFirstRun(configDir string) FirstRunModel {
	return FirstRunModel{configDir: configDir}
}

func (m FirstRunModel) Init() tea.Cmd { return nil }

func (m FirstRunModel) Update(msg tea.Msg) (FirstRunModel, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
	}
	return m, nil
}

func (m FirstRunModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Compression profiles created") + "\n\n")
	b.WriteString(style.StyleBody.Render("A default profiles.json has been created at:") + "\n")
	b.WriteString(style.StyleMuted.Render(m.configDir+"/profiles.json") + "\n\n")
	b.WriteString(style.StyleBody.Render("Edit this file to customize which textures get") + "\n")
	b.WriteString(style.StyleBody.Render("compressed and with which format. Changes take") + "\n")
	b.WriteString(style.StyleBody.Render("effect on the next scan.") + "\n\n")
	b.WriteString(style.StyleMuted.Render("Press any key to continue"))
	return b.String()
}

func (m *FirstRunModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
