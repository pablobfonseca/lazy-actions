package tui

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

	paneBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(0, 1)

	activePaneBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1)

	selectedBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	selectedRowStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("236"))

	logTimestampStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	logCommandStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	logGroupStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	logWarningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	logErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	logNoticeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	logDebugStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
