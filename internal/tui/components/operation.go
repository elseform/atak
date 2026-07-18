package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/noisethanks/atak/internal/tui/style"
)

// OperationProgressMsg is sent via the progress channel to update operation state.
type OperationProgressMsg struct {
	Percent       int
	Status        string // current file or status line
	Size          int64  // bytes written so far
	OutputSkipped int    // running total of files skipped (already in output dir); 0 = unused
	Done          bool
	Err           error
}

type opTickMsg OperationProgressMsg
type opElapsedMsg struct{}

// OperationScreen renders a shared progress UI for all long-running operations.
type OperationScreen struct {
	title         string
	ch            <-chan OperationProgressMsg
	cancel        func()
	spinner       spinner.Model
	bar           progress.Model
	percent       float64
	status        string
	size          int64
	outputSkipped int
	startedAt     time.Time
	elapsed       time.Duration
	done          bool
	err           error
	width         int
	height        int
}

func NewOperationScreen(title string, ch <-chan OperationProgressMsg, cancel func()) OperationScreen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	bar := progress.New(progress.WithDefaultGradient())
	return OperationScreen{
		title:     title,
		ch:        ch,
		cancel:    cancel,
		spinner:   sp,
		bar:       bar,
		startedAt: time.Now(),
	}
}

func (m OperationScreen) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.readNext(), m.tickElapsed())
}

func (m OperationScreen) readNext() tea.Cmd {
	ch := m.ch
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return opTickMsg{Done: true}
		}
		return opTickMsg(msg)
	}
}

func (m OperationScreen) tickElapsed() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return opElapsedMsg{}
	})
}

func (m OperationScreen) Update(msg tea.Msg) (OperationScreen, tea.Cmd) {
	switch msg := msg.(type) {
	case opTickMsg:
		if msg.Status != "" {
			m.status = msg.Status
		}
		if msg.Percent > 0 {
			m.percent = float64(msg.Percent) / 100.0
		}
		if msg.Size > 0 {
			m.size = msg.Size
		}
		if msg.OutputSkipped > 0 {
			m.outputSkipped = msg.OutputSkipped
		}
		if msg.Done {
			m.done = true
			m.err = msg.Err
			return m, nil
		}
		return m, tea.Batch(
			m.bar.SetPercent(m.percent),
			m.readNext(),
		)

	case opElapsedMsg:
		m.elapsed = time.Since(m.startedAt).Round(time.Second)
		if !m.done {
			return m, m.tickElapsed()
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		updated, cmd := m.bar.Update(msg)
		m.bar = updated.(progress.Model)
		return m, cmd
	}
	return m, nil
}

func (m OperationScreen) IsDone() bool { return m.done }
func (m OperationScreen) Err() error   { return m.err }

func (m OperationScreen) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render(m.title) + "\n\n")
	b.WriteString(style.StyleBody.Render(m.spinner.View()) + "\n")
	if m.percent > 0 {
		b.WriteString(m.bar.ViewAs(m.percent) + "\n")
	}
	if m.status != "" {
		const label = "  Last completed: "
		status := m.status
		if m.width > 0 {
			max := m.width - len(label)
			if max > 0 && len(status) > max {
				if max > 3 {
					status = "..." + status[len(status)-(max-3):]
				} else {
					status = status[len(status)-max:]
				}
			}
		}
		b.WriteString(style.StyleMuted.Render(label+status) + "\n")
	}
	if m.size > 0 {
		b.WriteString(style.StyleMuted.Render("  Size: "+opFormatBytes(m.size)) + "\n")
	}
	if m.outputSkipped > 0 {
		b.WriteString(style.StyleMuted.Render(fmt.Sprintf("  %d already in output folder", m.outputSkipped)) + "\n")
	}
	b.WriteString(style.StyleMuted.Render(fmt.Sprintf("  Elapsed: %s", m.elapsed)) + "\n")
	b.WriteString("\n" + style.StyleMuted.Render("  ctrl+c to cancel"))
	return b.String()
}

func (m *OperationScreen) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.bar.Width = w - 4
}

func opFormatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
