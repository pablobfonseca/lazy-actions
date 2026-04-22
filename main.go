package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/upscopeio/lazy-actions/internal/config"
	"github.com/upscopeio/lazy-actions/internal/tui"
)

func main() {
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

// resolveConfig combines the user's saved config with a CWD auto-detection,
// prompting the user when both exist and the CWD introduces a new repo. The
// prompt runs before we hand the terminal to bubbletea so reading stdin is
// safe.
func resolveConfig() (config.Config, error) {
	saved, savedErr := config.Load()
	detected, detectErr := config.DetectFromCWD()
	hasSaved := savedErr == nil
	hasDetected := detectErr == nil

	switch {
	case !hasSaved && !hasDetected:
		return config.Config{}, fmt.Errorf(
			"no config found and auto-detect failed\n  saved: %v\n  auto:  %v",
			savedErr, detectErr,
		)
	case hasSaved && !hasDetected:
		if !errors.Is(detectErr, config.ErrNotAutoDetectable) {
			fmt.Fprintf(os.Stderr, "warning: auto-detect failed: %v\n", detectErr)
		}
		return saved, nil
	case !hasSaved && hasDetected:
		return config.Config{Watches: []config.WatchEntry{detected}}, nil
	}

	if containsRepo(saved.Watches, detected.Repo) {
		return saved, nil
	}

	fmt.Printf("Detected %s with %d workflow(s) in current directory.\n",
		detected.Repo, len(detected.Workflows))
	fmt.Print("Merge with your saved config for this session? [Y/n] ")
	if askYes(os.Stdin) {
		return config.Config{Watches: append(saved.Watches, detected)}, nil
	}
	return saved, nil
}

func containsRepo(watches []config.WatchEntry, repo string) bool {
	for _, w := range watches {
		if w.Repo == repo {
			return true
		}
	}
	return false
}

// askYes reads a line from r and returns true for empty/y/yes (case-insensitive).
func askYes(r *os.File) bool {
	sc := bufio.NewScanner(r)
	if !sc.Scan() {
		return true
	}
	ans := strings.TrimSpace(strings.ToLower(sc.Text()))
	return ans == "" || ans == "y" || ans == "yes"
}
