package screens

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/tui/components"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

// CompressConfigModel is a confirmation and run-scope selection screen.
// Format and mip settings come from profiles.json — no per-profile overrides here.
type CompressConfigModel struct {
	groups    []AssetGroup
	mods      []string // unique mod names, sorted alphabetically
	mode      int      // 0=scope-select  1=profile-pick  2=mod-pick
	cursor    int
	modPicker components.ModPicker
	cfg       *config.Config
	width     int
	height    int
}

func NewCompressConfig(data CompressConfigData, cfg *config.Config) CompressConfigModel {
	seen := make(map[string]bool)
	var mods []string
	for _, g := range data.Groups {
		for _, a := range g.Assets {
			if !seen[a.ModName] {
				seen[a.ModName] = true
				mods = append(mods, a.ModName)
			}
		}
	}
	sort.Strings(mods)
	return CompressConfigModel{
		groups:    data.Groups,
		mods:      mods,
		cfg:       cfg,
		modPicker: components.NewModPicker(mods, 0, 0),
	}
}

func (m CompressConfigModel) Init() tea.Cmd { return nil }

func (m CompressConfigModel) Update(msg tea.Msg) (CompressConfigModel, tea.Cmd) {
	switch msg := msg.(type) {
	case components.ModSelectedMsg:
		return m, m.buildJobs(2, "", msg.Mod)

	case components.ModPickerCancelledMsg:
		m.mode = 0
		m.cursor = 2
		return m, nil

	case tea.KeyMsg:
		if m.mode != 2 {
			return m.handleKey(msg.String())
		}
	}
	if m.mode == 2 {
		var cmd tea.Cmd
		m.modPicker, cmd = m.modPicker.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m CompressConfigModel) handleKey(key string) (CompressConfigModel, tea.Cmd) {
	switch m.mode {
	case 0: // scope select
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 2 {
				m.cursor++
			}
		case "enter":
			switch m.cursor {
			case 0:
				return m, m.buildJobs(0, "", "")
			case 1:
				if len(m.groups) > 0 {
					m.mode = 1
					m.cursor = 0
				}
			case 2:
				if len(m.mods) > 0 {
					m.mode = 2
					m.modPicker = components.NewModPicker(m.mods, m.width-4, max(5, m.height-8))
				}
			}
		case "esc", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavResults} }
		}

	case 1: // profile pick
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.groups)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.groups) > 0 {
				return m, m.buildJobs(1, m.groups[m.cursor].ProfileName, "")
			}
		case "esc":
			m.mode = 0
			m.cursor = 1
		}

	}
	return m, nil
}

func (m CompressConfigModel) buildJobs(scope int, selectedProfile, selectedMod string) tea.Cmd {
	groups := m.groups
	cfg := m.cfg
	return func() tea.Msg {
		profiles, _ := config.LoadProfiles()
		mipsFor := func(name string) bool {
			for _, p := range profiles {
				if p.Name == name {
					return p.GenerateMips
				}
			}
			return true // auto groups ("Auto (alpha)", "Auto (no alpha)") default to generating mips
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
			outputDir := ""
			if !cfg.CompressInPlace && cfg.StagingDir != "" {
				outputDir = cfg.StagingDir
			}
			configured = append(configured, ConfiguredGroup{
				ProfileName:  g.ProfileName,
				Format:       g.SuggestedFmt,
				GenerateMips: mipsFor(g.ProfileName),
				Paths:        paths,
				OutputDir:    outputDir,
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

func (m CompressConfigModel) totalFiles() int {
	n := 0
	for _, g := range m.groups {
		n += len(g.Assets)
	}
	return n
}

func (m CompressConfigModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Compression Config") + "\n\n")

	total := m.totalFiles()
	outputMode := "in-place"
	if !m.cfg.CompressInPlace {
		if m.cfg.StagingDir != "" {
			outputMode = m.cfg.StagingDir
		} else {
			outputMode = "staging (no dir set)"
		}
	}
	b.WriteString(style.StyleMuted.Render(fmt.Sprintf(
		"%d files  ·  %d workers  ·  %s",
		total, m.cfg.WorkerCount, outputMode,
	)) + "\n\n")

	switch m.mode {
	case 0:
		m.viewScope(&b, total)
	case 1:
		m.viewProfilePick(&b)
	case 2:
		m.viewModPick(&b)
	}
	return b.String()
}

func (m CompressConfigModel) viewScope(b *strings.Builder, total int) {
	type scopeOpt struct {
		label string
		hint  string
	}
	opts := []scopeOpt{
		{fmt.Sprintf("Run All  (%d files)", total), ""},
		{"Run Selected Profile  →", ""},
		{"Run Selected Mod  →", "start here if first time"},
	}
	for i, o := range opts {
		prefix := "  "
		if i == m.cursor {
			prefix = style.StyleSelected.Render("▶ ")
		}
		line := prefix + o.label
		if o.hint != "" {
			line += "   " + style.StyleMuted.Render(o.hint)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	b.WriteString(style.KeyHint("↑↓", "select") + "  ")
	b.WriteString(style.KeyHint("enter", "run / drill down") + "  ")
	b.WriteString(style.KeyHint("q", "back"))
}

func (m CompressConfigModel) viewProfilePick(b *strings.Builder) {
	b.WriteString(style.StyleBody.Render("Select Profile:") + "\n\n")
	for i, g := range m.groups {
		prefix := "  "
		if i == m.cursor {
			prefix = style.StyleSelected.Render("▶ ")
		}
		b.WriteString(fmt.Sprintf("%s%-30s  %s\n",
			prefix,
			g.ProfileName,
			style.StyleMuted.Render(fmt.Sprintf("(%d files)", len(g.Assets))),
		))
	}
	b.WriteString("\n")
	b.WriteString(style.KeyHint("↑↓", "select") + "  ")
	b.WriteString(style.KeyHint("enter", "run") + "  ")
	b.WriteString(style.KeyHint("esc", "back"))
}

func (m CompressConfigModel) viewModPick(b *strings.Builder) {
	b.WriteString(style.StyleBody.Render("Select Mod:") + "\n\n")
	b.WriteString(fmt.Sprintf("DEBUG: %d mods, %d groups\n", len(m.mods), len(m.groups)))
	b.WriteString(m.modPicker.View())
	b.WriteString("\n")
	b.WriteString(style.KeyHint("enter", "run") + "  ")
	b.WriteString(style.KeyHint("esc", "back"))
}

func (m *CompressConfigModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.modPicker.SetSize(w-4, max(5, h-8))
}
