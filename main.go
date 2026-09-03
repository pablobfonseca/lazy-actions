package main

import (
	"errors"
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
// A missing config file falls back to auto-detection; a config file that
// exists but is invalid is an error, never a silent fallback.
func resolveConfig() (config.Config, error) {
	cfg, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return config.Config{}, err
	}
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
	cfg.Watches = merged
	return cfg, nil
}
