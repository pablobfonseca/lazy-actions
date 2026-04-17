package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39"))

	repoStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212"))

	runningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("220"))

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("82"))

	failureStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	dimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	stepStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))

	branchStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("141"))

	waitingStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	// Added for the new layout — unused until later tasks.
	paneBorderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("238"))

	activePaneBorder = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))

	sectionLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Bold(true)

	keyStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("82"))

	modalBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	searchMatchStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("220")).
		Foreground(lipgloss.Color("0"))
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
