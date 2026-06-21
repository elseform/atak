package screens

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/scan"
	"github.com/noisethanks/stalker-tex/internal/tui/components"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

// ScanResultData is passed from Scan → Results via NavigateMsg.
type ScanResultData struct {
	Assets  []scan.Asset
	Skipped int
}

// ResultsModel shows scan results grouped by compression profile.
type ResultsModel struct {
	groups        []AssetGroup // compressible profile groups — cursor navigates these
	unmatched     []assetRef   // informational only, not selectable
	excluded      []assetRef   // informational only, not selectable
	skipped       int
	cursor        int
	mods          []string
	showModPicker bool
	modPicker     components.ModPicker
	cfg           *config.Config
	width         int
	height        int
}

func NewResults(data ScanResultData, cfg *config.Config) ResultsModel {
	var ordered []AssetGroup
	groupIdx := make(map[string]int) // profile name -> index in ordered

	// Pre-populate all named profiles so zero-hit profiles still render.
	profiles, _, _, _ := config.LoadProfiles()
	for _, p := range profiles {
		groupIdx[p.Name] = len(ordered)
		ordered = append(ordered, AssetGroup{
			ProfileName:  p.Name,
			SuggestedFmt: p.Format,
		})
	}

	var unmatched, excluded []assetRef

	for _, a := range data.Assets {
		ref := assetRef{
			Path:       a.Path,
			ModName:    a.ModName,
			CurrentFmt: a.CurrentFmt,
			Width:      a.Width,
			Height:     a.Height,
			Compressed: a.Compressed,
		}
		switch a.ProfileMatch {
		case "Excluded":
			excluded = append(excluded, ref)
		case "Unmatched", "":
			unmatched = append(unmatched, ref)
		default:
			idx, ok := groupIdx[a.ProfileMatch]
			if !ok {
				// Unknown profile name — treat as unmatched.
				unmatched = append(unmatched, ref)
				continue
			}
			ordered[idx].Assets = append(ordered[idx].Assets, ref)
		}
	}

	// Compute sorted unique mod names from compressible groups only.
	seen := make(map[string]bool)
	var mods []string
	for _, g := range ordered {
		for _, a := range g.Assets {
			if a.ModName != "" && !seen[a.ModName] {
				seen[a.ModName] = true
				mods = append(mods, a.ModName)
			}
		}
	}
	sort.Strings(mods)

	return ResultsModel{
		groups:    ordered,
		unmatched: unmatched,
		excluded:  excluded,
		skipped:   data.Skipped,
		mods:      mods,
		cfg:       cfg,
	}
}

func (m ResultsModel) Init() tea.Cmd { return nil }

func (m ResultsModel) Update(msg tea.Msg) (ResultsModel, tea.Cmd) {
	// ModPicker messages bubble up regardless of showModPicker state.
	switch msg := msg.(type) {
	case components.ModSelectedMsg:
		m.showModPicker = false
		return m, m.buildJobs(2, "", msg.Mod)
	case components.ModPickerCancelledMsg:
		m.showModPicker = false
		return m, nil
	}

	if m.showModPicker {
		var cmd tea.Cmd
		m.modPicker, cmd = m.modPicker.Update(msg)
		return m, cmd
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.groups)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.groups) == 0 {
				return m, nil
			}
			return m, m.buildJobs(1, m.groups[m.cursor].ProfileName, "")
		case "r":
			return m, m.buildJobs(0, "", "")
		case "m":
			if len(m.mods) > 0 {
				m.showModPicker = true
				m.modPicker = components.NewModPicker(m.mods, m.width-4, max(5, m.height-8))
			}
		case "esc", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavMenu} }
		}
	}
	return m, nil
}

// buildJobs constructs a CompressJobData and navigates to NavCompress.
// scope: 0=all, 1=single profile, 2=single mod across all profiles.
func (m ResultsModel) buildJobs(scope int, selectedProfile, selectedMod string) tea.Cmd {
	groups := m.groups
	cfg := m.cfg
	return func() tea.Msg {
		profiles, _, _, _ := config.LoadProfiles()
		mipsFor := func(name string) bool {
			for _, p := range profiles {
				if p.Name == name {
					return p.GenerateMips
				}
			}
			return true // default to mips on
		}

		var filtered []AssetGroup
		switch scope {
		case 0: // Run All
			filtered = groups
		case 1: // Run Selected Profile
			for _, g := range groups {
				if g.ProfileName == selectedProfile {
					filtered = append(filtered, g)
					break
				}
			}
		case 2: // Run Selected Mod — collect that mod's assets from every profile group
			for _, g := range groups {
				var modAssets []assetRef
				for _, a := range g.Assets {
					if a.ModName == selectedMod {
						modAssets = append(modAssets, a)
					}
				}
				if len(modAssets) > 0 {
					filtered = append(filtered, AssetGroup{
						ProfileName:  g.ProfileName,
						SuggestedFmt: g.SuggestedFmt,
						Assets:       modAssets,
					})
				}
			}
		}

		var configured []ConfiguredGroup
		for _, g := range filtered {
			var paths []string
			for _, a := range g.Assets {
				paths = append(paths, a.Path)
			}
			configured = append(configured, ConfiguredGroup{
				ProfileName:  g.ProfileName,
				Format:       g.SuggestedFmt,
				GenerateMips: mipsFor(g.ProfileName),
				Paths:        paths,
				OutputDir:    "",
			})
		}
		return NavigateMsg{
			To: NavCompress,
			Data: CompressJobData{
				Groups:      configured,
				WorkerCount: cfg.WorkerCount,
			},
		}
	}
}

func (m ResultsModel) View() string {
	if m.showModPicker {
		var b strings.Builder
		b.WriteString(style.StyleTitle.Render("Select Mod") + "\n\n")
		b.WriteString(m.modPicker.View())
		return b.String()
	}

	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Scan Results") + "\n\n")

	// Counter line.
	total := 0
	for _, g := range m.groups {
		total += len(g.Assets)
	}
	b.WriteString(fmt.Sprintf(
		"%s  %s  %s  %s\n\n",
		style.StyleBody.Render(fmt.Sprintf("%d to compress", total)),
		style.StyleMuted.Render(fmt.Sprintf("%d skipped (compressed)", m.skipped)),
		style.StyleMuted.Render(fmt.Sprintf("%d unmatched", len(m.unmatched))),
		style.StyleMuted.Render(fmt.Sprintf("%d excluded", len(m.excluded))),
	))

	// Compressible profile groups (cursor navigates these).
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

	// Unmatched — informational, not selectable.
	b.WriteString("\n")
	b.WriteString(style.StyleMuted.Render(fmt.Sprintf("  Unmatched (%d files)", len(m.unmatched))) + "\n")
	if len(m.unmatched) > 0 {
		b.WriteString(style.StyleMuted.Render("  Add patterns to profiles.json to compress these") + "\n")
	}

	// Excluded — informational, not selectable.
	b.WriteString(style.StyleMuted.Render(fmt.Sprintf("  Excluded (%d files)", len(m.excluded))) + "\n")

	b.WriteString("\n")
	b.WriteString(style.KeyHint("enter", "run profile") + "  ")
	b.WriteString(style.KeyHint("r", "run all") + "  ")
	b.WriteString(style.KeyHint("m", "run single mod") + "  ")
	b.WriteString(style.KeyHint("q", "back") + "\n")
	b.WriteString(style.StyleMuted.Render("Tip: press [m] to test on one mod first"))
	return b.String()
}

func (m *ResultsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	if m.showModPicker {
		m.modPicker.SetSize(w-4, max(5, h-8))
	}
}
