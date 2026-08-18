package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	loadDotEnv()

	cfg, err := resolveConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	tracker := NewNotifyTracker()
	p := tea.NewProgram(newModel(cfg, tracker), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// resolveConfig loads the saved config, falling back to auto-detection from
// the current directory when no config file is found. Saved config always
// wins; detection is never merged into it.
func resolveConfig() (Config, error) {
	cfg, err := loadConfig()
	if err == nil {
		return cfg, nil
	}
	detected, detectErr := DetectFromCWD()
	if detectErr != nil {
		return Config{}, fmt.Errorf("no config found and auto-detect failed\n  config: %v\n  auto:   %v", err, detectErr)
	}
	return Config{Watches: []WatchEntry{detected}}, nil
}
