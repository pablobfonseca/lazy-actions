package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pablobfonseca/lazy-actions/internal/config"
	"github.com/pablobfonseca/lazy-actions/internal/tui"
)

func main() {
	loadDotEnv()

	cfg, err := resolveConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// resolveConfig loads the saved config, falling back to auto-detection from
// the current directory when no config file is found. Saved config always
// wins; detection is never merged into it.
func resolveConfig() (config.Config, error) {
	cfg, err := config.Load()
	if err == nil {
		return cfg, nil
	}
	detected, detectErr := config.DetectFromCWD()
	if detectErr != nil {
		return config.Config{}, fmt.Errorf("no config found and auto-detect failed\n  config: %v\n  auto:   %v", err, detectErr)
	}
	return config.Config{Watches: []config.WatchEntry{detected}}, nil
}
