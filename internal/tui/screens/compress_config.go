package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

var bcnFormats = []string{
	"BC1_UNORM",
	"BC3_UNORM",
	"BC4_UNORM",
	"BC5_UNORM",
	"BC6H_UF16",
	"BC7_UNORM",
}

type configRow struct {
	group        AssetGroup
	formatIdx    int // index into bcnFormats
	generateMips bool
}

// CompressConfigModel lets the user override per-category format before compressing.
type CompressConfigModel struct {
	rows    []configRow
	cursor  int
	inPlace bool
	cfg     *config.Config
	width   int
	height  int
}

func NewCompressConfig(data CompressConfigData, cfg *config.Config) CompressConfigModel {
	rows := make([]configRow, len(data.Groups))
	for i, g := range data.Groups {
		fmtIdx := 0
		for j, f := range bcnFormats {
			if f == g.SuggestedFmt {
				fmtIdx = j
				break
			}
		}
		rows[i] = configRow{
			group:        g,
			formatIdx:    fmtIdx,
			generateMips: false,
		}
	}
	return CompressConfigModel{
		rows:    rows,
		inPlace: cfg.CompressInPlace,
		cfg:     cfg,
	}
}

func (m CompressConfigModel) Init() tea.Cmd { return nil }

func (m CompressConfigModel) Update(msg tea.Msg) (CompressConfigModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "left", "h":
			r := &m.rows[m.cursor]
			if r.formatIdx > 0 {
				r.formatIdx--
			}
		case "right", "l":
			r := &m.rows[m.cursor]
			if r.formatIdx < len(bcnFormats)-1 {
				r.formatIdx++
			}
		case "m":
			m.rows[m.cursor].generateMips = !m.rows[m.cursor].generateMips
		case "i":
			m.inPlace = !m.inPlace
		case "enter":
			return m, m.buildJobs()
		case "esc", "q":
			return m, func() tea.Msg { return NavigateMsg{To: NavResults} }
		}
	}
	return m, nil
}

func (m CompressConfigModel) buildJobs() tea.Cmd {
	rows := m.rows
	cfg := m.cfg
	inPlace := m.inPlace
	return func() tea.Msg {
		var groups []ConfiguredGroup
		for _, r := range rows {
			var paths []string
			for _, a := range r.group.Assets {
				paths = append(paths, a.Path)
			}
			outputDir := ""
			if !inPlace && cfg.StagingDir != "" {
				outputDir = cfg.StagingDir
			}
			groups = append(groups, ConfiguredGroup{
				ProfileName:  r.group.ProfileName,
				Format:       bcnFormats[r.formatIdx],
				GenerateMips: r.generateMips,
				Paths:        paths,
				OutputDir:    outputDir,
			})
		}
		return NavigateMsg{
			To: NavCompress,
			Data: CompressJobData{
				Groups:      groups,
				WorkerCount: cfg.WorkerCount,
			},
		}
	}
}

func (m CompressConfigModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Compression Settings") + "\n\n")

	for i, r := range m.rows {
		selected := i == m.cursor
		total := len(r.group.Assets)
		prefix := "  "
		if selected {
			prefix = style.StyleSelected.Render("▶ ")
		}

		fmtDisplay := fmt.Sprintf("← %s →", bcnFormats[r.formatIdx])
		mipsDisplay := "mips:off"
		if r.generateMips {
			mipsDisplay = "mips:on"
		}

		line := fmt.Sprintf("%s%-20s  %s  %s  %s",
			prefix,
			r.group.ProfileName,
			style.StyleSelected.Render(fmtDisplay),
			style.StyleMuted.Render(mipsDisplay),
			style.StyleMuted.Render(fmt.Sprintf("(%d files)", total)),
		)
		if selected {
			b.WriteString(style.StylePanelFocused.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")

		// Show sample paths for focused row.
		if selected && len(r.group.Assets) > 0 {
			limit := 3
			if len(r.group.Assets) < limit {
				limit = len(r.group.Assets)
			}
			for _, a := range r.group.Assets[:limit] {
				b.WriteString(style.StyleMuted.Render("    " + filepath.Base(a.Path)) + "\n")
			}
			if len(r.group.Assets) > limit {
				b.WriteString(style.StyleMuted.Render(fmt.Sprintf("    … and %d more\n", len(r.group.Assets)-limit)))
			}
		}
	}

	outputMode := "in-place (overwrite originals)"
	if !m.inPlace {
		outputMode = "staging dir: " + m.cfg.StagingDir
	}
	b.WriteString("\n" + style.StyleBody.Render("Output: "+outputMode) + "\n\n")

	b.WriteString(style.KeyHint("↑↓", "select row") + "  ")
	b.WriteString(style.KeyHint("←→", "change format") + "  ")
	b.WriteString(style.KeyHint("m", "toggle mips") + "  ")
	b.WriteString(style.KeyHint("i", "toggle in-place") + "  ")
	b.WriteString(style.KeyHint("enter", "start") + "  ")
	b.WriteString(style.KeyHint("q", "back"))
	return b.String()
}

func (m *CompressConfigModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
