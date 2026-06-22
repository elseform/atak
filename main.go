package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noisethanks/atak/internal/config"
	"github.com/noisethanks/atak/internal/tools"
	"github.com/noisethanks/atak/internal/tui"
)

var version = "dev"

func main() {
	t, err := tools.Extract()
	if err != nil {
		fmt.Fprintf(os.Stderr, "atak: failed to extract embedded tools: %v\n", err)
		os.Exit(1)
	}
	defer t.Cleanup()

	tools.CheckStaleLock()

	firstRun := config.IsFirstRun()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "atak: failed to load config: %v\n", err)
		os.Exit(1)
	}

	_, _, profilesCreated, err := config.LoadProfiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "atak: failed to load profiles: %v\n", err)
		os.Exit(1)
	}

	m := tui.New(cfg, t, firstRun, profilesCreated, version)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "atak: %v\n", err)
		os.Exit(1)
	}
}
