package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/stalker-tex/internal/config"
	"github.com/noisethanks/stalker-tex/internal/tools"
	"github.com/noisethanks/stalker-tex/internal/tui"
)

func main() {
	t, err := tools.Extract()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stalker-tex: failed to extract embedded tools: %v\n", err)
		os.Exit(1)
	}
	defer t.Cleanup()

	firstRun := config.IsFirstRun()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stalker-tex: failed to load config: %v\n", err)
		os.Exit(1)
	}

	m := tui.New(cfg, t, firstRun)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "stalker-tex: %v\n", err)
		os.Exit(1)
	}
}
