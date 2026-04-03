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

const (
	activeInterval   = 10 * time.Second
	inactiveInterval = 60 * time.Second
	recentThreshold  = 60 * time.Second // run started within this = "active"
)

type watchKey struct {
	repo     string
	workflow string
}

type watchState struct {
	runs      []RunInfo
	lastFetch time.Time
	active    bool // has in-progress run or recent activity
}

type tickMsg time.Time

type fetchResultMsg struct {
	key       watchKey
	runs      []RunInfo
	err       error
	rateLimit bool
}

type model struct {
	config         Config
	watches        map[watchKey]*watchState
	watchOrder     []watchKey // stable ordering for display
	err            error
	rateLimited    bool
	rateLimitReset time.Time
	fetching       map[watchKey]bool // prevent overlapping fetches
	spinnerIndex   int
	width          int
}

func newModel(cfg Config) model {
	watches := make(map[watchKey]*watchState)
	var order []watchKey
	for _, w := range cfg.Watches {
		for _, wf := range w.Workflows {
			key := watchKey{w.Repo, wf}
			if _, exists := watches[key]; !exists {
				watches[key] = &watchState{}
				order = append(order, key)
			}
		}
	}
	return model{
		config:     cfg,
		watches:    watches,
		watchOrder: order,
		fetching:   make(map[watchKey]bool),
		width:      80,
	}
}

func (m model) Init() tea.Cmd {
	// Fetch all on startup in parallel
	var cmds []tea.Cmd
	cmds = append(cmds, tickCmd())
	for key := range m.watches {
		cmds = append(cmds, fetchCmd(key))
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			var cmds []tea.Cmd
			for key := range m.watches {
				if !m.fetching[key] {
					m.fetching[key] = true
					cmds = append(cmds, fetchCmd(key))
				}
			}
			return m, tea.Batch(cmds...)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width

	case tickMsg:
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)

		var cmds []tea.Cmd
		cmds = append(cmds, tickCmd())

		if !m.rateLimited {
			now := time.Now()
			for key, state := range m.watches {
				if m.fetching[key] {
					continue
				}
				interval := inactiveInterval
				if state.active {
					interval = activeInterval
				}
				if now.Sub(state.lastFetch) >= interval {
					m.fetching[key] = true
					cmds = append(cmds, fetchCmd(key))
				}
			}
		}

		return m, tea.Batch(cmds...)

	case rateLimitRetryMsg:
		return handleRateLimitRetry(m)

	case fetchResultMsg:
		delete(m.fetching, msg.key)

		if msg.rateLimit {
			m.rateLimited = true
			m.rateLimitReset = fetchRateLimitReset()
			return m, scheduleRateLimitRetry(m.rateLimitReset)
		}

		m.rateLimited = false
		m.rateLimitReset = time.Time{}

		state := m.watches[msg.key]
		if msg.err != nil {
			m.err = msg.err
		} else {
			state.runs = msg.runs
			state.lastFetch = time.Now()
			state.active = isActive(msg.runs)
			m.err = nil
		}
	}

	return m, nil
}

// isActive returns true if any run is in-progress or started recently.
func isActive(runs []RunInfo) bool {
	for _, r := range runs {
		if r.Run.Status == "in_progress" {
			return true
		}
		if time.Since(r.Run.RunStartedAt) < recentThreshold {
			return true
		}
	}
	return false
}

func (m model) View() string {
	var b strings.Builder

	title := "  GitHub Actions Monitor"
	if m.rateLimited {
		remaining := time.Until(m.rateLimitReset)
		if remaining > 0 {
			title += runningStyle.Render(fmt.Sprintf("  [rate limited, resets in %s]", formatDuration(remaining)))
		} else {
			title += runningStyle.Render("  [rate limited, retrying...]")
		}
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", min(m.width, 70))))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(failureStyle.Render(fmt.Sprintf("  Error: %s", m.err)))
		b.WriteString("\n")
	}

	// Collect all runs for display
	var allRuns []RunInfo
	for _, state := range m.watches {
		allRuns = append(allRuns, state.runs...)
	}

	if len(allRuns) == 0 && m.err == nil {
		b.WriteString(dimStyle.Render("  Loading..."))
		b.WriteString("\n")
		return b.String()
	}

	// Check if all repos share the same owner
	sameOwner := true
	var commonOwner string
	for _, r := range allRuns {
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
	for _, r := range allRuns {
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

		// Show completed runs (sorted most recent first)
		var completed []RunInfo
		for _, r := range runs {
			if r.Run.Status == "completed" {
				completed = append(completed, r)
			}
		}
		sort.Slice(completed, func(i, j int) bool {
			return completed[i].Run.RunStartedAt.After(completed[j].Run.RunStartedAt)
		})
		for _, r := range completed {
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
		}

		if !hasContent {
			b.WriteString(dimStyle.Render("    No recent runs"))
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	b.WriteString(dimStyle.Render("  r: refresh  q: quit"))
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

func fetchCmd(key watchKey) tea.Cmd {
	return func() tea.Msg {
		runs, err := fetchRuns(key.repo, key.workflow)
		if err != nil && errors.Is(err, errRateLimit) {
			return fetchResultMsg{key: key, rateLimit: true}
		}
		return fetchResultMsg{key: key, runs: runs, err: err}
	}
}

type rateLimitRetryMsg struct{}

func scheduleRateLimitRetry(resetAt time.Time) tea.Cmd {
	wait := time.Until(resetAt) + 2*time.Second
	if wait < 10*time.Second {
		wait = 10 * time.Second
	}
	return tea.Tick(wait, func(t time.Time) tea.Msg {
		return rateLimitRetryMsg{}
	})
}

// rateLimitRetryMsg triggers a re-fetch of all watches after rate limit expires.
func handleRateLimitRetry(m model) (model, tea.Cmd) {
	m.rateLimited = false
	m.rateLimitReset = time.Time{}
	var cmds []tea.Cmd
	for key := range m.watches {
		m.fetching[key] = true
		cmds = append(cmds, fetchCmd(key))
	}
	return m, tea.Batch(cmds...)
}
