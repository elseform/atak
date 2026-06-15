package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/scan"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

// ResultsModel shows scan results grouped by compression profile.
type ResultsModel struct {
	groups      []AssetGroup
	unmatched   []scan.Asset
	alreadyDone []scan.Asset
	cursor      int
	cfg         *config.Config
	width       int
	height      int
}

func NewResults(assets []scan.Asset, cfg *config.Config) ResultsModel {
	groups := make(map[string]*AssetGroup)
	var unmatched, done []scan.Asset

	for _, a := range assets {
		if a.Compressed && a.ProfileMatch == "" {
			done = append(done, a)
			continue
		}
		if a.ProfileMatch == "" {
			unmatched = append(unmatched, a)
			continue
		}
		g, ok := groups[a.ProfileMatch]
		if !ok {
			g = &AssetGroup{
				ProfileName:  a.ProfileMatch,
				SuggestedFmt: a.SuggestedFmt,
			}
			groups[a.ProfileMatch] = g
		}
		g.Assets = append(g.Assets, assetRef{
			Path:       a.Path,
			ModName:    a.ModName,
			CurrentFmt: a.CurrentFmt,
			Width:      a.Width,
			Height:     a.Height,
			Compressed: a.Compressed,
		})
	}

	// Stable order — put unmatched last.
	var ordered []AssetGroup
	for _, g := range groups {
		ordered = append(ordered, *g)
	}

	return ResultsModel{
		groups:      ordered,
		unmatched:   unmatched,
		alreadyDone: done,
		cfg:         cfg,
	}
}

func (m ResultsModel) Init() tea.Cmd { return nil }

func (m ResultsModel) Update(msg tea.Msg) (ResultsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.groups)-1 {
				m.cursor++
			}
		case "enter", "c":
			if len(m.groups) == 0 {
				return m, nil
			}
			return m, func() tea.Msg {
				return NavigateMsg{
					To:   NavCompressConfig,
					Data: CompressConfigData{Groups: m.groups},
				}
			}
		case "b":
			return m, func() tea.Msg { return NavigateMsg{To: NavBackup} }
		case "esc", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
		}
	}
	return m, nil
}

func (m ResultsModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Scan Results") + "\n\n")

	total := 0
	for _, g := range m.groups {
		total += len(g.Assets)
	}
	b.WriteString(fmt.Sprintf(
		"%s  %s  %s\n\n",
		style.StyleBody.Render(fmt.Sprintf("%d to compress", total)),
		style.StyleSuccess.Render(fmt.Sprintf("%d already done", len(m.alreadyDone))),
		style.StyleMuted.Render(fmt.Sprintf("%d unmatched", len(m.unmatched))),
	))

	if len(m.groups) == 0 {
		b.WriteString(style.StyleSuccess.Render("Nothing to compress — all textures are already in a BCn format.") + "\n")
	} else {
		for i, g := range m.groups {
			prefix := "  "
			if i == m.cursor {
				prefix = style.StyleSelected.Render("▶ ")
			}
			line := fmt.Sprintf("%s%s  →  %s  (%d files)",
				prefix,
				g.ProfileName,
				style.StyleSelected.Render(g.SuggestedFmt),
				len(g.Assets),
			)
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(style.KeyHint("enter", "configure & compress") + "  ")
	b.WriteString(style.KeyHint("b", "backup first") + "  ")
	b.WriteString(style.KeyHint("q", "back"))
	return b.String()
}

func (m *ResultsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
