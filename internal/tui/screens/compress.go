package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/noisethanks/stalker-tex/internal/compress"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/scan"
	"github.com/noisethanks/stalker-tex/internal/tools"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

// compressionDoneMsg carries one result and the channel for the next.
type compressionDoneMsg struct {
	result compress.CompressionResult
	ch     <-chan compress.CompressionResult
}

// compressAllDoneMsg signals all jobs are finished.
type compressAllDoneMsg struct{}

// CompressModel shows a progress bar and live log during compression.
type CompressModel struct {
	data        CompressJobData
	cfg         *config.Config
	tools       *tools.EmbeddedTools
	progress    progress.Model
	total       int
	done        int
	succeeded   int
	failed      int
	currentFile string
	log         []string
	errors      []string
	finished    bool
	totalBefore int64
	totalAfter  int64
	width       int
	height      int
}

const maxLogLines = 6

func NewCompress(data CompressJobData, cfg *config.Config, t *tools.EmbeddedTools) CompressModel {
	total := 0
	for _, g := range data.Groups {
		total += len(g.Paths)
	}
	bar := progress.New(progress.WithDefaultGradient())
	return CompressModel{
		data:     data,
		cfg:      cfg,
		tools:    t,
		progress: bar,
		total:    total,
	}
}

func (m CompressModel) Init() tea.Cmd {
	return m.startCompression()
}

func (m CompressModel) startCompression() tea.Cmd {
	data := m.data
	texconvPath := m.tools.TexconvPath
	cfg := m.cfg
	return func() tea.Msg {
		jobs := buildJobs(data, cfg)
		ch := compress.RunPool(texconvPath, jobs, data.WorkerCount, cfg)
		return readNextResult(ch)
	}
}

func readNextResult(ch <-chan compress.CompressionResult) tea.Msg {
	r, ok := <-ch
	if !ok {
		return compressAllDoneMsg{}
	}
	return compressionDoneMsg{result: r, ch: ch}
}

func (m CompressModel) Update(msg tea.Msg) (CompressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case compressionDoneMsg:
		r := msg.result
		m.done++
		m.totalBefore += r.Before
		m.totalAfter += r.After
		if r.Success {
			m.succeeded++
			m.currentFile = filepath.Base(r.Asset.Path)
			m.addLog(style.StyleSuccess.Render("✓ ") + m.currentFile)
		} else {
			m.failed++
			errLine := fmt.Sprintf("%s: %v", filepath.Base(r.Asset.Path), r.Err)
			m.errors = append(m.errors, errLine)
			m.addLog(style.StyleDanger.Render("✗ ") + errLine)
		}

		pct := float64(m.done) / float64(max(1, m.total))
		progressCmd := m.progress.SetPercent(pct)

		ch := msg.ch
		next := func() tea.Msg { return readNextResult(ch) }
		return m, tea.Batch(progressCmd, next)

	case compressAllDoneMsg:
		m.finished = true
		summary := SummaryData{
			Succeeded:   m.succeeded,
			Failed:      m.failed,
			TotalBefore: m.totalBefore,
			TotalAfter:  m.totalAfter,
			Errors:      m.errors,
		}
		return m, func() tea.Msg { return NavigateMsg{To: NavSummary, Data: summary} }

	case progress.FrameMsg:
		updated, cmd := m.progress.Update(msg)
		m.progress = updated.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m *CompressModel) addLog(line string) {
	m.log = append(m.log, line)
	if len(m.log) > maxLogLines {
		m.log = m.log[len(m.log)-maxLogLines:]
	}
}

func (m CompressModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Compressing Textures") + "\n\n")

	pct := float64(m.done) / float64(max(1, m.total))
	b.WriteString(m.progress.ViewAs(pct) + "\n")
	b.WriteString(style.StyleBody.Render(fmt.Sprintf(
		"%d / %d  (%d errors)",
		m.done, m.total, m.failed,
	)) + "\n\n")

	if m.currentFile != "" {
		b.WriteString(style.StyleMuted.Render("Current: "+m.currentFile) + "\n\n")
	}

	for _, l := range m.log {
		b.WriteString("  " + l + "\n")
	}

	b.WriteString("\n" + style.StyleMuted.Render("Running…  ctrl+c to abort (partial results saved)"))
	return b.String()
}

func (m *CompressModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.progress.Width = w - 4
}

// buildJobs converts ConfiguredGroups into compress.Jobs.
func buildJobs(data CompressJobData, cfg *config.Config) []compress.Job {
	_ = cfg
	var jobs []compress.Job
	for _, g := range data.Groups {
		for _, path := range g.Paths {
			outDir := g.OutputDir
			if outDir == "" {
				outDir = filepath.Dir(path)
			}
			jobs = append(jobs, compress.Job{
				Asset: scan.Asset{
					Path: path,
				},
				Format:       g.Format,
				GenerateMips: g.GenerateMips,
				OutputDir:    outDir,
			})
		}
	}
	return jobs
}
