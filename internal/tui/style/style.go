package style

import "github.com/charmbracelet/lipgloss"

var (
	// Base colors.
	colorPrimary  = lipgloss.Color("#E8A020") // amber — STALKER feel
	colorDim      = lipgloss.Color("#7A6040")
	colorMuted    = lipgloss.Color("#555555")
	colorSuccess  = lipgloss.Color("#4CAF50")
	colorWarning  = lipgloss.Color("#FFC107")
	colorDanger   = lipgloss.Color("#F44336")
	colorSubtle   = lipgloss.Color("#444444")
	colorBg       = lipgloss.Color("#1A1A1A")
	colorText     = lipgloss.Color("#DDDDDD")

	// Title / header.
	StyleTitle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			MarginBottom(1)

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	// Body text.
	StyleBody = lipgloss.NewStyle().
			Foreground(colorText)

	StyleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Status variants.
	StyleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess)

	StyleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	StyleDanger = lipgloss.NewStyle().
			Foreground(colorDanger)

	// Highlighted / selected item.
	StyleSelected = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Box / panel.
	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(0, 1)

	StylePanelFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	// Key hint bar at the bottom of screens.
	StyleKeyHint = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleKeyName = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Progress label.
	StyleProgressLabel = lipgloss.NewStyle().
				Foreground(colorDim)

	// Error item in a list.
	StyleErrorItem = lipgloss.NewStyle().
			Foreground(colorDanger)

	// Separator line.
	StyleSep = lipgloss.NewStyle().
			Foreground(colorSubtle)
)

// KeyHint formats a single keybinding hint: "key  description".
func KeyHint(key, desc string) string {
	return StyleKeyName.Render(key) + StyleKeyHint.Render("  "+desc)
}
