package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	tracker := NewNotifyTracker()
	p := tea.NewProgram(newModel(cfg, tracker), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		tracker.ClearAll()
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	tracker.ClearAll()
}
