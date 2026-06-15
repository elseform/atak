package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

type menuItem struct {
	label string
	desc  string
	nav   NavTarget
}

var menuItems = []menuItem{
	{label: "Scan & Compress", desc: "Walk mods dir, identify and compress textures", nav: NavScan},
	{label: "Backup Manager", desc: "Create, list, verify, and delete backups", nav: NavBackup},
	{label: "Restore Mod", desc: "Extract a mod from a backup archive", nav: NavRestore},
	{label: "Settings", desc: "Configure paths and worker threads", nav: NavSettings},
	{label: "Quit", desc: "", nav: NavQuit},
}

// MenuModel is the main hub screen.
type MenuModel struct {
	cursor int
	width  int
	height int
}

func NewMenu() MenuModel {
	return MenuModel{}
}

func (m MenuModel) Init() tea.Cmd { return nil }

func (m MenuModel) Update(msg tea.Msg) (MenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(menuItems)-1 {
				m.cursor++
			}
		case "enter", " ":
			item := menuItems[m.cursor]
			return m, func() tea.Msg { return NavigateMsg{To: item.nav} }
		case "ctrl+c":
			return m, func() tea.Msg { return NavigateMsg{To: NavQuit} }
		}
	}
	return m, nil
}

func (m MenuModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("stalker-tex") + "\n")
	b.WriteString(style.StyleSubtitle.Render("GAMMA texture compressor & backup tool") + "\n\n")

	for i, item := range menuItems {
		if i == m.cursor {
			b.WriteString(style.StyleSelected.Render("▶ " + item.label))
		} else {
			b.WriteString(style.StyleBody.Render("  " + item.label))
		}
		if item.desc != "" {
			b.WriteString("  " + style.StyleMuted.Render(item.desc))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n" + style.KeyHint("↑↓", "navigate") + "  " + style.KeyHint("enter", "select"))
	return b.String()
}

func (m *MenuModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
