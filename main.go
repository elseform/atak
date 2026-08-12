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
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "scan":
			os.Exit(runScanCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "compress":
			os.Exit(runCompressCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "help", "-h", "--help":
			printRootUsage(os.Stdout)
			return
		default:
			fmt.Fprintf(os.Stderr, "atak: unknown command %q\n", os.Args[1])
			printRootUsage(os.Stderr)
			os.Exit(2)
		}
	}

	runTUI()
}

func runTUI() {
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

	_, _, _, profilesCreated, err := config.LoadProfiles()
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
