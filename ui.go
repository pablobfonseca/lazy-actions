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

// Each color has a variant per terminal background: Dark values are the
// original bright palette, Light values are darker equivalents that stay
// readable on white. lipgloss detects the background once at startup.
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

	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "26", Dark: "39"})

	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
)

const bellMarker = "🔔"

const (
	activeInterval   = 10 * time.Second
	inactiveInterval = 180 * time.Second
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

type repoFetchResultMsg struct {
	repo      string
	results   map[string][]RunInfo // workflow file name -> runs
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
	repoFetching   map[string]bool     // prevent overlapping repo-level fetches
	repoWorkflows  map[string][]string // repo -> watched workflow file names
	spinnerIndex   int
	width          int
	height         int
	branchCache    *BranchCache
	jobCache       *JobCache
	notifyTracker  *NotifyTracker
	selectMode     bool // mobile-notification selection mode (toggled with "n")
	cursor         int  // index into the displayed groups while in selectMode
}

func newModel(cfg Config, tracker *NotifyTracker) model {
	watches := make(map[watchKey]*watchState)
	var order []watchKey
	repoWorkflows := make(map[string][]string)
	for _, w := range cfg.Watches {
		for _, wf := range w.Workflows {
			key := watchKey{w.Repo, wf}
			if _, exists := watches[key]; !exists {
				watches[key] = &watchState{}
				order = append(order, key)
				repoWorkflows[w.Repo] = append(repoWorkflows[w.Repo], wf)
			}
		}
	}
	return model{
		config:        cfg,
		watches:       watches,
		watchOrder:    order,
		repoFetching:  make(map[string]bool),
		repoWorkflows: repoWorkflows,
		width:         80,
		branchCache:   NewBranchCache(),
		jobCache:      NewJobCache(),
		notifyTracker: tracker,
	}
}

func (m model) Init() tea.Cmd {
	// Fetch all repos on startup in parallel
	var cmds []tea.Cmd
	cmds = append(cmds, tickCmd())
	for repo, workflows := range m.repoWorkflows {
		cmds = append(cmds, repoFetchCmd(repo, workflows, m.branchCache, m.jobCache))
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.selectMode {
			return m.handleSelectKey(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "n":
			if _, order := m.groupedRuns(); len(order) > 0 {
				m.selectMode = true
				if m.cursor >= len(order) {
					m.cursor = 0
				}
			}
			return m, nil
		case "r":
			var cmds []tea.Cmd
			for repo, workflows := range m.repoWorkflows {
				if !m.repoFetching[repo] {
					m.repoFetching[repo] = true
					cmds = append(cmds, repoFetchCmd(repo, workflows, m.branchCache, m.jobCache))
				}
			}
			return m, tea.Batch(cmds...)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)

		var cmds []tea.Cmd
		cmds = append(cmds, tickCmd())

		if !m.rateLimited {
			now := time.Now()
			for repo, workflows := range m.repoWorkflows {
				if m.repoFetching[repo] {
					continue
				}
				// Use the most aggressive interval among all workflows in this repo
				interval := inactiveInterval
				needsFetch := false
				for _, wf := range workflows {
					state := m.watches[watchKey{repo, wf}]
					if state.active {
						interval = activeInterval
					}
					if now.Sub(state.lastFetch) >= interval {
						needsFetch = true
					}
				}
				if needsFetch {
					m.repoFetching[repo] = true
					cmds = append(cmds, repoFetchCmd(repo, workflows, m.branchCache, m.jobCache))
				}
			}
		}

		return m, tea.Batch(cmds...)

	case rateLimitRetryMsg:
		return handleRateLimitRetry(m)

	case repoFetchResultMsg:
		delete(m.repoFetching, msg.repo)

		if msg.rateLimit {
			m.rateLimited = true
			m.rateLimitReset = fetchRateLimitReset()
			return m, scheduleRateLimitRetry(m.rateLimitReset)
		}

		m.rateLimited = false
		m.rateLimitReset = time.Time{}

		if msg.err != nil {
			m.err = msg.err
		} else {
			now := time.Now()
			for wf, runs := range msg.results {
				key := watchKey{msg.repo, wf}
				state := m.watches[key]
				m.notifyTracker.CheckAndNotify(key, runs)
				state.runs = runs
				state.lastFetch = now
				state.active = isActive(runs)
			}
			// Mark workflows with no results as fetched (no recent runs)
			for _, wf := range m.repoWorkflows[msg.repo] {
				key := watchKey{msg.repo, wf}
				if _, ok := msg.results[wf]; !ok {
					state := m.watches[key]
					state.lastFetch = now
					state.active = false
				}
			}
			m.err = nil
		}
	}

	return m, nil
}

// isActive returns true if any run is in-progress or started recently.
func isActive(runs []RunInfo) bool {
	for _, r := range runs {
		if r.Run.Status == "in_progress" || r.Run.Status == "waiting" {
			return true
		}
		if time.Since(r.Run.RunStartedAt) < recentThreshold {
			return true
		}
	}
	return false
}

// handleSelectKey processes key presses while in mobile-notification selection mode.
func (m model) handleSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	_, order := m.groupedRuns()
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "n", "esc", "q":
		m.selectMode = false
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(order)-1 {
			m.cursor++
		}
	case " ", "enter":
		if m.cursor < len(order) {
			m.notifyTracker.ToggleMobile(order[m.cursor])
		}
	}
	return m, nil
}

// groupedRuns collects all known runs grouped by repo/workflow, ordered with the
// most recently active group first. Shared by the renderer and the selection
// cursor so both agree on what each row points at.
func (m model) groupedRuns() (map[watchKey][]RunInfo, []watchKey) {
	groups := make(map[watchKey][]RunInfo)
	var order []watchKey
	for _, state := range m.watches {
		for _, r := range state.runs {
			key := watchKey{r.Repo, r.Workflow}
			if _, exists := groups[key]; !exists {
				order = append(order, key)
			}
			groups[key] = append(groups[key], r)
		}
	}

	// Sort groups by latest run start time (most recent first), stable tiebreak by name.
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

	return groups, order
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

	groups, order := m.groupedRuns()

	if len(order) == 0 && m.err == nil {
		b.WriteString(dimStyle.Render("  Loading..."))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render(m.helpText()))
		b.WriteString("\n")
		return b.String()
	}

	// Check if all repos share the same owner
	sameOwner := true
	var commonOwner string
	for _, key := range order {
		owner := key.repo
		if idx := strings.Index(key.repo, "/"); idx >= 0 {
			owner = key.repo[:idx]
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

	for i, key := range order {
		runs := groups[key]
		if m.selectMode && i == m.cursor {
			b.WriteString(cursorStyle.Render("> "))
		} else {
			b.WriteString("  ")
		}
		b.WriteString(repoStyle.Render(displayRepo(key.repo)))
		b.WriteString(dimStyle.Render(fmt.Sprintf(" / %s", key.workflow)))
		if m.notifyTracker.IsMobileEnabled(key) {
			b.WriteString(" " + bellMarker)
		}
		b.WriteString("\n")

		hasContent := false
		for _, r := range runs {
			if r.Run.Status == "in_progress" || r.Run.Status == "waiting" {
				hasContent = true
				elapsed := formatDuration(time.Since(r.Run.RunStartedAt))
				spinner := runningStyle.Render(spinnerFrames[m.spinnerIndex])
				branch := branchStyle.Render(r.Run.HeadBranch)

				if len(r.Jobs) <= 1 {
					url := r.Run.HTMLURL
					detail := ""
					if r.Run.Status == "waiting" {
						// Always link to main run page so user can approve/skip
						if len(r.Jobs) == 1 {
							detail = renderJobDetail(r.Jobs[0], m.spinnerIndex)
						} else {
							detail = waitingStyle.Render("waiting")
						}
					} else if len(r.Jobs) == 1 {
						url = r.Jobs[0].URL
						detail = renderJobDetail(r.Jobs[0], m.spinnerIndex)
					}
					b.WriteString(fmt.Sprintf("    %s %s  %s  %s",
						spinner,
						runningStyle.Render(elapsed),
						branch,
						hyperlink(url, detail),
					))
					b.WriteString("\n")
				} else {
					b.WriteString(fmt.Sprintf("    %s %s  %s",
						spinner,
						runningStyle.Render(elapsed),
						branch,
					))
					b.WriteString("\n")
					for _, job := range r.Jobs {
						b.WriteString(fmt.Sprintf("      %s %s",
							dimStyle.Render("├"),
							renderJobLine(job, m.spinnerIndex),
						))
						b.WriteString("\n")
					}
				}
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
			conclusionURL := r.Run.HTMLURL
			conclusionText := style.Render(r.Run.Conclusion)

			if len(r.Jobs) == 1 {
				conclusionURL = r.Jobs[0].URL
				conclusionText = style.Render(r.Run.Conclusion + ": " + r.Jobs[0].Name)
			} else if len(r.Jobs) > 1 {
				// Multiple failed jobs — show them individually
				b.WriteString(fmt.Sprintf("    %s %s  %s  %s  %s",
					style.Render(icon),
					style.Render(duration),
					branch,
					dimStyle.Render(ago+" ago"),
					style.Render(r.Run.Conclusion),
				))
				b.WriteString("\n")
				for _, job := range r.Jobs {
					b.WriteString(fmt.Sprintf("      %s %s",
						dimStyle.Render("├"),
						hyperlink(job.URL, style.Render(job.Name)),
					))
					b.WriteString("\n")
				}
				continue
			}

			b.WriteString(fmt.Sprintf("    %s %s  %s  %s  %s",
				style.Render(icon),
				style.Render(duration),
				branch,
				dimStyle.Render(ago+" ago"),
				hyperlink(conclusionURL, conclusionText),
			))
			b.WriteString("\n")
		}

		if !hasContent {
			b.WriteString(dimStyle.Render("    No recent runs"))
			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	footer := dimStyle.Render(m.helpText())

	// Keep the body within the terminal height, but always pin the footer to the
	// last line so the keyboard shortcuts stay visible even when runs overflow.
	body := strings.TrimRight(b.String(), "\n")
	if m.height > 1 {
		lines := strings.Split(body, "\n")
		if max := m.height - 1; len(lines) > max {
			lines = lines[:max]
		}
		body = strings.Join(lines, "\n")
	}
	return body + "\n" + footer
}

// helpText returns the keyboard-shortcut line shown pinned at the bottom.
func (m model) helpText() string {
	if m.selectMode {
		help := fmt.Sprintf("  ↑/↓: move  space: toggle %s  n/esc: done", bellMarker)
		if !m.notifyTracker.MobileConfigured() {
			help += "  (set BRRR_TOKEN in .env)"
		}
		return help
	}
	return "  r: refresh  n: mobile notifications  q: quit"
}

var waitingStyle = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "243", Dark: "245"}).
	Italic(true)

// renderJobDetail renders a single job's status for the compact (single-job) view.
func renderJobDetail(job JobInfo, _ int) string {
	switch job.Status {
	case "waiting", "queued", "pending":
		elapsed := ""
		if !job.StartedAt.IsZero() {
			elapsed = formatDuration(time.Since(job.StartedAt)) + " "
		}
		return waitingStyle.Render(fmt.Sprintf("%s%s", elapsed, job.Status))
	default:
		return stepStyle.Render(job.CurrentStep)
	}
}

// renderJobLine renders a single job's status for the multi-job tree view.
func renderJobLine(job JobInfo, spinnerIndex int) string {
	switch job.Status {
	case "waiting", "queued", "pending":
		elapsed := ""
		if !job.StartedAt.IsZero() {
			elapsed = " " + formatDuration(time.Since(job.StartedAt))
		}
		return hyperlink(job.URL, waitingStyle.Render(fmt.Sprintf("%s  %s%s", job.Name, job.Status, elapsed)))
	default:
		return fmt.Sprintf("%s  %s",
			hyperlink(job.URL, runningStyle.Render(job.Name)),
			stepStyle.Render(job.CurrentStep),
		)
	}
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

func repoFetchCmd(repo string, workflows []string, bc *BranchCache, jc *JobCache) tea.Cmd {
	return func() tea.Msg {
		results, err := fetchRepoRuns(repo, workflows, bc, jc)
		if err != nil && errors.Is(err, errRateLimit) {
			return repoFetchResultMsg{repo: repo, rateLimit: true}
		}
		return repoFetchResultMsg{repo: repo, results: results, err: err}
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

// handleRateLimitRetry re-fetches all repos after rate limit expires.
func handleRateLimitRetry(m model) (model, tea.Cmd) {
	m.rateLimited = false
	m.rateLimitReset = time.Time{}
	var cmds []tea.Cmd
	for repo, workflows := range m.repoWorkflows {
		m.repoFetching[repo] = true
		cmds = append(cmds, repoFetchCmd(repo, workflows, m.branchCache, m.jobCache))
	}
	return m, tea.Batch(cmds...)
}
