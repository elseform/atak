package screens

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/scan"
	"github.com/noisethanks/stalker-tex/internal/tools"
	"github.com/noisethanks/stalker-tex/internal/tui/style"
)

// assetFoundMsg carries one discovered asset and the channel to read the next from.
type assetFoundMsg struct {
	asset scan.Asset
	ch    <-chan scan.Asset
}

// scanCompleteMsg signals the walker is finished.
type scanCompleteMsg struct{ total int }

// ScanModel shows a live counter while the walker runs.
type ScanModel struct {
	cfg     *config.Config
	tools   *tools.EmbeddedTools
	spinner spinner.Model
	ctx     context.Context
	cancel  context.CancelFunc
	assets  []scan.Asset
	found   int
	done    bool
	err     string
	width   int
	height  int
}

func NewScan(cfg *config.Config, t *tools.EmbeddedTools) ScanModel {
	ctx, cancel := context.WithCancel(context.Background())
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return ScanModel{cfg: cfg, tools: t, spinner: sp, ctx: ctx, cancel: cancel}
}

func (m ScanModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return OperationStartedMsg{Cancel: m.cancel, CancelMsg: "Scan cancelled"} },
		m.spinner.Tick,
		m.startScan(),
	)
}

func (m ScanModel) startScan() tea.Cmd {
	modsDir := m.cfg.ModsDir
	ctx := m.ctx
	return func() tea.Msg {
		profiles, err := config.LoadProfiles()
		if err != nil || len(profiles) == 0 {
			return scanCompleteMsg{total: 0}
		}
		ch, _ := scan.Walk(modsDir, profiles, m.cfg.ScanExclusions)
		return readNextAsset(ctx, ch)
	}
}

func readNextAsset(ctx context.Context, ch <-chan scan.Asset) tea.Msg {
	select {
	case <-ctx.Done():
		go func() { for range ch {} }() // drain so walker goroutine exits
		return scanCompleteMsg{}
	case asset, ok := <-ch:
		if !ok {
			return scanCompleteMsg{}
		}
		return assetFoundMsg{asset: asset, ch: ch}
	}
}

func (m ScanModel) Update(msg tea.Msg) (ScanModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case assetFoundMsg:
		m.assets = append(m.assets, msg.asset)
		m.found++
		ch := msg.ch
		ctx := m.ctx
		return m, func() tea.Msg { return readNextAsset(ctx, ch) }

	case scanCompleteMsg:
		m.done = true
		if m.found == 0 {
			m.err = "No .dds files found in " + m.cfg.ModsDir
			return m, nil
		}
		assets := m.assets
		return m, func() tea.Msg {
			return NavigateMsg{To: NavResults, Data: assets}
		}
	}
	return m, nil
}

func (m ScanModel) View() string {
	var b strings.Builder
	b.WriteString(style.StyleTitle.Render("Scanning...") + "\n\n")
	b.WriteString(m.spinner.View() + "  ")
	b.WriteString(style.StyleBody.Render(fmt.Sprintf("Found %d textures", m.found)) + "\n\n")
	if m.err != "" {
		b.WriteString(style.StyleDanger.Render(m.err) + "\n")
	}
	b.WriteString(style.StyleMuted.Render(m.cfg.ModsDir) + "\n\n")
	b.WriteString(style.KeyHint("ctrl+c", "cancel"))
	return b.String()
}

func (m *ScanModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
