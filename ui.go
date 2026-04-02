package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

type tickMsg time.Time

type pollResultMsg struct {
	runs      []RunInfo
	err       error
	rateLimit bool
}

type model struct {
	config       Config
	runs         []RunInfo
	err          error
	rateLimited  bool
	spinnerIndex int
	width        int
}

func newModel(cfg Config) model {
	return model{
		config: cfg,
		width:  80,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), pollCmd(m.config, 0))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tickMsg:
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
		return m, tickCmd()

	case pollResultMsg:
		if msg.rateLimit {
			m.rateLimited = true
			return m, pollCmd(m.config, 30*time.Second)
		}
		m.rateLimited = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.runs = msg.runs
			m.err = nil
		}
		delay := 10 * time.Second
		if hasActiveRuns(m.runs) {
			delay = 5 * time.Second
		}
		return m, pollCmd(m.config, delay)
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	title := "  GitHub Actions Monitor"
	if m.rateLimited {
		title += runningStyle.Render("  [rate limited, backing off]")
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", min(m.width, 70))))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(failureStyle.Render(fmt.Sprintf("  Error: %s", m.err)))
		b.WriteString("\n")
	}

	if len(m.runs) == 0 && m.err == nil {
		b.WriteString(dimStyle.Render("  Loading..."))
		b.WriteString("\n")
		return b.String()
	}

	// Check if all repos share the same owner
	sameOwner := true
	var commonOwner string
	for _, r := range m.runs {
		owner := r.Repo
		if idx := strings.Index(r.Repo, "/"); idx >= 0 {
			owner = r.Repo[:idx]
		}
		if commonOwner == "" {
			commonOwner = owner
		} else if owner != commonOwner {
			sameOwner = false
			break
		}
	}

	displayRepo := func(repo string) string {
		if sameOwner {
			if idx := strings.Index(repo, "/"); idx >= 0 {
				return repo[idx+1:]
			}
		}
		return repo
	}

	// Group runs by repo/workflow
	type groupKey struct{ repo, workflow string }
	groups := make(map[groupKey][]RunInfo)
	var order []groupKey
	for _, r := range m.runs {
		key := groupKey{r.Repo, r.Workflow}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], r)
	}

	// Sort groups by latest run start time (most recent first), stable tiebreak by name
	sort.SliceStable(order, func(i, j int) bool {
		latestI := latestStartTime(groups[order[i]])
		latestJ := latestStartTime(groups[order[j]])
		if latestI.Equal(latestJ) {
			if order[i].repo != order[j].repo {
				return order[i].repo < order[j].repo
			}
			return order[i].workflow < order[j].workflow
		}
		return latestI.After(latestJ)
	})

	for _, key := range order {
		runs := groups[key]
		b.WriteString(repoStyle.Render(fmt.Sprintf("  %s", displayRepo(key.repo))))
		b.WriteString(dimStyle.Render(fmt.Sprintf(" / %s", key.workflow)))
		b.WriteString("\n")

		hasContent := false
		for _, r := range runs {
			if r.Run.Status == "in_progress" {
				hasContent = true
				elapsed := formatDuration(time.Since(r.Run.RunStartedAt))
				spinner := runningStyle.Render(spinnerFrames[m.spinnerIndex])
				branch := branchStyle.Render(r.Run.HeadBranch)
				step := ""
				if r.CurrentStep != "" {
					step = stepStyle.Render(r.CurrentStep)
				}

				b.WriteString(fmt.Sprintf("    %s %s  %s  %s",
					spinner,
					runningStyle.Render(elapsed),
					branch,
					hyperlink(runURL(r), step),
				))
				b.WriteString("\n")
			}
		}

		// Show latest completed
		for _, r := range runs {
			if r.Run.Status == "completed" {
				hasContent = true
				duration := formatDuration(r.Run.UpdatedAt.Sub(r.Run.RunStartedAt))
				ago := formatTimeAgo(time.Since(r.Run.UpdatedAt))
				branch := branchStyle.Render(r.Run.HeadBranch)

				icon, style := conclusionDisplay(r.Run.Conclusion)
				b.WriteString(fmt.Sprintf("    %s %s  %s  %s  %s",
					style.Render(icon),
					style.Render(duration),
					branch,
					dimStyle.Render(ago+" ago"),
					hyperlink(runURL(r), style.Render(r.Run.Conclusion)),
				))
				b.WriteString("\n")
				break
			}
		}

		if !hasContent {
			b.WriteString(dimStyle.Render("    No recent runs"))
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	b.WriteString(dimStyle.Render("  Press q to quit"))
	b.WriteString("\n")

	return b.String()
}

// runURL returns the most specific URL available: job URL if present, otherwise run URL.
func runURL(r RunInfo) string {
	if r.JobURL != "" {
		return r.JobURL
	}
	return r.Run.HTMLURL
}

// hyperlink wraps text in an OSC 8 terminal hyperlink escape sequence.
// Clicking the text in supported terminals (iTerm2, Kitty, etc.) opens the URL.
func hyperlink(url, text string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, text)
}

func latestStartTime(runs []RunInfo) time.Time {
	var latest time.Time
	for _, r := range runs {
		if r.Run.RunStartedAt.After(latest) {
			latest = r.Run.RunStartedAt
		}
	}
	return latest
}

func conclusionDisplay(conclusion string) (string, lipgloss.Style) {
	switch conclusion {
	case "success":
		return "✓", successStyle
	case "failure":
		return "✗", failureStyle
	case "cancelled":
		return "⊘", dimStyle
	default:
		return "?", dimStyle
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatTimeAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func hasActiveRuns(runs []RunInfo) bool {
	for _, r := range runs {
		if r.Run.Status == "in_progress" {
			return true
		}
	}
	return false
}

func pollCmd(cfg Config, delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(delay)

		type result struct {
			runs      []RunInfo
			err       error
			rateLimit bool
		}

		type fetchTask struct {
			repo     string
			workflow string
		}

		var tasks []fetchTask
		for _, w := range cfg.Watches {
			for _, wf := range w.Workflows {
				tasks = append(tasks, fetchTask{w.Repo, wf})
			}
		}

		ch := make(chan result, len(tasks))
		for _, t := range tasks {
			go func(repo, wf string) {
				runs, err := fetchRuns(repo, wf)
				if err != nil && errors.Is(err, errRateLimit) {
					ch <- result{rateLimit: true}
					return
				}
				ch <- result{runs: runs, err: err}
			}(t.repo, t.workflow)
		}

		var allRuns []RunInfo
		for range tasks {
			r := <-ch
			if r.rateLimit {
				return pollResultMsg{rateLimit: true}
			}
			if r.err != nil {
				return pollResultMsg{err: r.err}
			}
			allRuns = append(allRuns, r.runs...)
		}
		return pollResultMsg{runs: allRuns}
	}
}
