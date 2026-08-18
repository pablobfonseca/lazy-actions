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

// resolveConfig merges the saved config with auto-detection from the current
// directory: the detected entry is appended to the saved watches unless its
// repo is already watched, in which case the saved entry wins untouched.
// Either source may be missing; only when both fail is it an error.
func resolveConfig() (config.Config, error) {
	cfg, err := config.Load()
	detected, detectErr := config.DetectFromCWD()

	switch {
	case err != nil && detectErr != nil:
		return config.Config{}, fmt.Errorf("no config found and auto-detect failed\n  config: %v\n  auto:   %v", err, detectErr)
	case err != nil:
		return config.Config{Watches: []config.WatchEntry{detected}}, nil
	case detectErr != nil:
		return cfg, nil
	}

	for _, w := range cfg.Watches {
		if w.Repo == detected.Repo {
			return cfg, nil
		}
	}

	merged := make([]config.WatchEntry, 0, len(cfg.Watches)+1)
	merged = append(merged, cfg.Watches...)
	merged = append(merged, detected)
	return config.Config{Watches: merged}, nil
}
