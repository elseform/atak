package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

// SummaryModel shows compression stats and any errors.
type SummaryModel struct {
	data    SummaryData
	cursor  int // index into data.Errors for scrolling
	width   int
	height  int
}

func NewSummary(data SummaryData) SummaryModel {
	return SummaryModel{data: data}
}

func (m SummaryModel) Init() tea.Cmd { return nil }

func (m SummaryModel) Update(msg tea.Msg) (SummaryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.data.Errors)-1 {
				m.cursor++
			}
		case "enter", "m":
			return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
		case "r", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavResults} }
		case "ctrl+c":
			return m, func() tea.Msg { return NavigateMsg{To: NavQuit} }
		}
	}
	return m, nil
}

func (m SummaryModel) View() string {
	d := m.data
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Done") + "\n\n")

	b.WriteString(style.StyleSuccess.Render(fmt.Sprintf("✓  %d succeeded", d.Succeeded)) + "\n")
	if d.Failed > 0 {
		b.WriteString(style.StyleDanger.Render(fmt.Sprintf("✗  %d failed", d.Failed)) + "\n")
	}
	b.WriteString("\n")

	if d.TotalBefore > 0 {
		saved := d.TotalBefore - d.TotalAfter
		pct := float64(saved) / float64(d.TotalBefore) * 100
		b.WriteString(style.StyleBody.Render(fmt.Sprintf(
			"VRAM delta:  %s → %s  (saved %s, %.1f%%)",
			formatBytes(d.TotalBefore),
			formatBytes(d.TotalAfter),
			formatBytes(saved),
			pct,
		)) + "\n\n")
	}

	if len(d.Errors) > 0 {
		b.WriteString(style.StyleWarning.Render(fmt.Sprintf("Errors (%d):", len(d.Errors))) + "\n")
		visible := d.Errors
		if len(visible) > 10 {
			visible = visible[m.cursor : min(m.cursor+10, len(visible))]
		}
		for _, e := range visible {
			b.WriteString(style.StyleErrorItem.Render("  • "+e) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(style.KeyHint("m / enter", "main menu") + "  ")
	b.WriteString(style.KeyHint("r / q", "back to results"))
	return b.String()
}

func (m *SummaryModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func formatBytes(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.2f GB", float64(n)/GB)
	case n >= MB:
		return fmt.Sprintf("%.1f MB", float64(n)/MB)
	case n >= KB:
		return fmt.Sprintf("%.0f KB", float64(n)/KB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
