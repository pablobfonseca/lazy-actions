package tui

import "github.com/charmbracelet/lipgloss"

// Each color has a variant per terminal background: Dark values are the original bright palette, Light values are darker equivalents that stay readable on white. lipgloss detects the background once at startup.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "26", Dark: "39"})

	repoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "162", Dark: "212"})

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "220"})

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "82"})

	failureStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "196"})

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "245"})

	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "31", Dark: "117"})

	branchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "97", Dark: "141"})

	waitingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "245"}).
			Italic(true)

	sectionLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "245"}).
				Bold(true)

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "82"})

	modalBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(0, 1)

	searchMatchStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("220")).
				Foreground(lipgloss.Color("0"))

	paneBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).
			Padding(0, 1)

	activePaneBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "26", Dark: "39"}).
				Padding(0, 1)

	selectedBarStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "26", Dark: "39"}).
				Bold(true)

	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"})

	logTimestampStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "247", Dark: "240"})
	logCommandStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "32", Dark: "75"})
	logGroupStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "97", Dark: "141"}).Bold(true)
	logWarningStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "220"})
	logErrorStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "196"})
	logNoticeStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "26", Dark: "39"})
	logDebugStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "245"})

	toastSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "82"}).Bold(true)
	toastErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "196"}).Bold(true)
	toastInfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "26", Dark: "39"}).Bold(true)
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
