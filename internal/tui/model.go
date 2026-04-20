package tui

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/upscopeio/lazy-actions/internal/clip"
	"github.com/upscopeio/lazy-actions/internal/config"
	"github.com/upscopeio/lazy-actions/internal/gh"
	"github.com/upscopeio/lazy-actions/internal/notify"
)

type mode int

const (
	modeNormal mode = iota
	modeLogs
	modeFilter
	modeConfirm
	modeHelp
)

const (
	activeInterval   = 10 * time.Second
	inactiveInterval = 180 * time.Second
	recentThreshold  = 60 * time.Second
	logTailSettle    = 300 * time.Millisecond
	logTailLines     = 10
	toastTTL         = 3 * time.Second
)

type toastLevel int

const (
	toastInfo toastLevel = iota
	toastSuccess
	toastError
)

type toast struct {
	text    string
	level   toastLevel
	expires time.Time
}

type watchKey struct {
	repo     string
	workflow string
}

type watchState struct {
	runs      []gh.RunInfo
	lastFetch time.Time
	active    bool
}

type tickMsg time.Time
type rateLimitRetryMsg struct{}
type logTailDueMsg struct{ runID int64 }
type logTailResultMsg struct {
	runID int64
	step  string
	lines []string
	err   error
}

type actionKind int

const (
	actionRerunFailed actionKind = iota
	actionCancel
)

type actionResultMsg struct {
	kind actionKind
	repo string
	err  error
}

type downloadResultMsg struct {
	dest string
	err  error
}
type repoFetchResultMsg struct {
	repo      string
	results   map[string][]gh.RunInfo
	err       error
	rateLimit bool
}
type logViewerLoadedMsg struct {
	runID int64
	lines []string
	err   error
}

type model struct {
	config         config.Config
	mode           mode
	watches        map[watchKey]*watchState
	repoWorkflows  map[string][]string
	repoFetching   map[string]bool
	err            error
	rateLimited    bool
	rateLimitReset time.Time
	spinnerIndex   int
	width          int
	height         int

	filter      filterState
	filterInput textinput.Model

	overview  *overviewModel
	detail    *detailModel
	help      *helpModel
	confirm   *confirmModel
	prompt    *promptModel
	logviewer *logViewer

	branchCache *gh.BranchCache
	jobCache    *gh.JobCache
	logCache    *gh.LogCache

	notifyTracker *notify.NotifyTracker

	toast *toast
}

func New(cfg config.Config) tea.Model {
	watches := make(map[watchKey]*watchState)
	repoWorkflows := make(map[string][]string)
	for _, w := range cfg.Watches {
		for _, wf := range w.Workflows {
			key := watchKey{w.Repo, wf}
			if _, ok := watches[key]; !ok {
				watches[key] = &watchState{}
				repoWorkflows[w.Repo] = append(repoWorkflows[w.Repo], wf)
			}
		}
	}
	ti := textinput.New()
	ti.Placeholder = "filter (branch/workflow/repo)…"
	ti.CharLimit = 64
	return model{
		config:        cfg,
		watches:       watches,
		repoWorkflows: repoWorkflows,
		repoFetching:  make(map[string]bool),
		filter:        filterState{status: statusAll},
		filterInput:   ti,
		overview:      newOverview(),
		detail:        newDetail(),
		help:          newHelp(),
		confirm:       newConfirm(),
		prompt:        newPrompt(),
		logviewer:     newLogViewer(),
		branchCache:   gh.NewBranchCache(),
		jobCache:      gh.NewJobCache(),
		logCache:      gh.NewLogCache(),
		notifyTracker: notify.NewNotifyTracker(),
		width:         80,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd()}
	for repo, wfs := range m.repoWorkflows {
		cmds = append(cmds, repoFetchCmd(repo, wfs, m.branchCache, m.jobCache))
	}
	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.overview.SetSize(overviewWidth(m.width))
		m.detail.SetSize(detailWidth(m.width))
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
		cmds := []tea.Cmd{tickCmd()}
		if !m.rateLimited {
			now := time.Now()
			for repo, wfs := range m.repoWorkflows {
				if m.repoFetching[repo] {
					continue
				}
				interval := inactiveInterval
				needs := false
				for _, wf := range wfs {
					st := m.watches[watchKey{repo, wf}]
					if st.active {
						interval = activeInterval
					}
					if now.Sub(st.lastFetch) >= interval {
						needs = true
					}
				}
				if needs {
					m.repoFetching[repo] = true
					cmds = append(cmds, repoFetchCmd(repo, wfs, m.branchCache, m.jobCache))
				}
			}
		}
		return m, tea.Batch(cmds...)

	case rateLimitRetryMsg:
		m.rateLimited = false
		m.rateLimitReset = time.Time{}
		var cmds []tea.Cmd
		for repo, wfs := range m.repoWorkflows {
			m.repoFetching[repo] = true
			cmds = append(cmds, repoFetchCmd(repo, wfs, m.branchCache, m.jobCache))
		}
		return m, tea.Batch(cmds...)

	case repoFetchResultMsg:
		delete(m.repoFetching, msg.repo)
		if msg.rateLimit {
			m.rateLimited = true
			m.rateLimitReset = gh.FetchRateLimitReset()
			return m, scheduleRateLimitRetry(m.rateLimitReset)
		}
		m.rateLimited = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			now := time.Now()
			for wf, runs := range msg.results {
				key := watchKey{msg.repo, wf}
				st := m.watches[key]
				m.notifyTracker.CheckAndNotify(msg.repo, wf, runs)
				st.runs = runs
				st.lastFetch = now
				st.active = isActive(runs)
			}
			for _, wf := range m.repoWorkflows[msg.repo] {
				key := watchKey{msg.repo, wf}
				if _, ok := msg.results[wf]; !ok {
					st := m.watches[key]
					st.lastFetch = now
					st.active = false
				}
			}
			m.err = nil
			m.overview.SetRuns(m.allRuns())
			m.syncDetailSelection()
		}
		return m, nil

	case logTailDueMsg:
		if m.overview.SelectedID() != msg.runID {
			return m, nil
		}
		return m, m.fetchLogTailCmd(msg.runID)

	case logTailResultMsg:
		if m.overview.SelectedID() == msg.runID {
			m.detail.SetLogTail(msg.step, msg.lines, msg.err)
		}
		return m, nil

	case logViewerLoadedMsg:
		m.logviewer.SetContent(msg.lines, msg.err)
		return m, nil

	case actionResultMsg:
		m.setToast(toastForActionResult(msg))
		if msg.err == nil && !m.repoFetching[msg.repo] {
			m.repoFetching[msg.repo] = true
			return m, repoFetchCmd(msg.repo, m.repoWorkflows[msg.repo], m.branchCache, m.jobCache)
		}
		return m, nil

	case downloadResultMsg:
		if msg.err != nil {
			m.setToast(toastError, msg.err.Error())
		} else {
			m.setToast(toastSuccess, "saved to "+msg.dest)
		}
		return m, nil
	}
	return m, nil
}

func toastForActionResult(msg actionResultMsg) (toastLevel, string) {
	if msg.err != nil {
		switch {
		case errors.Is(msg.err, gh.ErrRateLimit):
			return toastError, "rate limited"
		case errors.Is(msg.err, gh.ErrNotRerunnable):
			return toastError, "nothing to re-run"
		case errors.Is(msg.err, gh.ErrNotCancellable):
			return toastError, "run cannot be cancelled"
		default:
			return toastError, "action failed"
		}
	}
	switch msg.kind {
	case actionRerunFailed:
		return toastSuccess, "re-run queued"
	case actionCancel:
		return toastSuccess, "cancel requested"
	}
	return toastInfo, ""
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.mode == modeFilter {
		switch msg.String() {
		case "enter":
			m.filter.fuzzy = m.filterInput.Value()
			m.overview.SetFilter(m.filter)
			m.syncDetailSelection()
			m.filterInput.Blur()
			m.mode = modeNormal
			return m, nil
		case "esc":
			m.filter.fuzzy = ""
			m.overview.SetFilter(m.filter)
			m.syncDetailSelection()
			m.filterInput.Blur()
			m.filterInput.SetValue("")
			m.mode = modeNormal
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.filter.fuzzy = m.filterInput.Value()
		m.overview.SetFilter(m.filter)
		m.syncDetailSelection()
		return m, cmd
	}

	if m.mode == modeLogs {
		cmd, _ := m.logviewer.Update(msg)
		if !m.logviewer.IsOpen() {
			m.mode = modeNormal
		}
		return m, cmd
	}
	if m.help.IsOpen() {
		switch msg.String() {
		case "esc", "?", "q":
			m.help.Close()
		}
		return m, nil
	}
	if m.confirm.IsOpen() {
		handled, cmd := m.confirm.Update(msg)
		if handled {
			m.confirm.Close()
			return m, cmd
		}
		return m, nil
	}
	if m.prompt.IsOpen() {
		handled, cmd := m.prompt.Update(msg)
		if handled {
			m.prompt.Close()
			return m, cmd
		}
		return m, cmd
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.help.Toggle()
		return m, nil
	case "r":
		var cmds []tea.Cmd
		for repo, wfs := range m.repoWorkflows {
			if !m.repoFetching[repo] {
				m.repoFetching[repo] = true
				cmds = append(cmds, repoFetchCmd(repo, wfs, m.branchCache, m.jobCache))
			}
		}
		return m, tea.Batch(cmds...)
	case "o":
		if r, ok := m.overview.Selected(); ok && r.Run.HTMLURL != "" {
			openURL(r.Run.HTMLURL)
		}
		return m, nil
	case "y":
		if r, ok := m.overview.Selected(); ok && r.Run.HTMLURL != "" {
			if err := clip.Copy(r.Run.HTMLURL); err != nil {
				m.setToast(toastError, "copy failed")
			} else {
				m.setToast(toastSuccess, "copied run URL")
			}
		}
		return m, nil
	case "Y":
		if r, ok := m.overview.Selected(); ok && r.Run.HeadSHA != "" {
			if err := clip.Copy(r.Run.HeadSHA); err != nil {
				m.setToast(toastError, "copy failed")
			} else {
				short := r.Run.HeadSHA
				if len(short) > 7 {
					short = short[:7]
				}
				m.setToast(toastSuccess, "copied SHA "+short)
			}
		}
		return m, nil
	case "F":
		r, ok := m.overview.Selected()
		if !ok {
			return m, nil
		}
		if !isRerunnable(r.Run) {
			m.setToast(toastError, "nothing to re-run")
			return m, nil
		}
		m.confirm.Open(
			fmt.Sprintf("Re-run failed jobs for %s/%s #%d on %s?", r.Repo, r.Workflow, r.Run.ID, r.Run.HeadBranch),
			rerunFailedCmd(r.Repo, r.Run.ID),
		)
		return m, nil
	case "x":
		r, ok := m.overview.Selected()
		if !ok {
			return m, nil
		}
		if !isCancellable(r.Run) {
			m.setToast(toastError, "run cannot be cancelled")
			return m, nil
		}
		m.confirm.Open(
			fmt.Sprintf("Cancel %s/%s #%d on %s?", r.Repo, r.Workflow, r.Run.ID, r.Run.HeadBranch),
			cancelRunCmd(r.Repo, r.Run.ID),
		)
		return m, nil
	case "d":
		r, ok := m.overview.Selected()
		if !ok {
			return m, nil
		}
		if r.Run.Status != "completed" {
			m.setToast(toastError, "artifacts available only for completed runs")
			return m, nil
		}
		dest := defaultArtifactDir(r.Repo, r.Run.ID)
		repo := r.Repo
		runID := r.Run.ID
		m.prompt.Open("download to:", dest, func(chosen string) tea.Cmd {
			return downloadArtifactsCmd(repo, runID, chosen)
		})
		return m, nil
	case "enter":
		return m.openLogViewer()
	case "a":
		m.filter.status = statusActive
		m.overview.SetFilter(m.filter)
		m.syncDetailSelection()
		return m, nil
	case "f":
		m.filter.status = statusFailed
		m.overview.SetFilter(m.filter)
		m.syncDetailSelection()
		return m, nil
	case "A":
		m.filter = filterState{status: statusAll}
		m.overview.SetFilter(m.filter)
		m.syncDetailSelection()
		return m, nil
	case "/":
		m.mode = modeFilter
		m.filterInput.Focus()
		m.filterInput.SetValue(m.filter.fuzzy)
		return m, nil
	case "esc":
		if m.filter.fuzzy != "" || m.filter.status != statusAll {
			m.filter = filterState{status: statusAll}
			m.filterInput.SetValue("")
			m.overview.SetFilter(m.filter)
			m.syncDetailSelection()
		}
		return m, nil
	}

	prev := m.overview.SelectedID()
	_, _ = m.overview.Update(msg)
	if m.overview.SelectedID() != prev {
		m.syncDetailSelection()
		if id := m.overview.SelectedID(); id != 0 {
			return m, scheduleLogTailCmd(id)
		}
	}
	return m, nil
}

func (m model) openLogViewer() (tea.Model, tea.Cmd) {
	r, ok := m.overview.Selected()
	if !ok {
		return m, nil
	}
	job, _, _ := pickJobForTail(r)
	if job == 0 {
		return m, nil
	}
	m.logviewer.Open(fmt.Sprintf("%s/%s #%d", r.Repo, r.Workflow, r.Run.ID), m.width, m.height-2)
	m.mode = modeLogs
	repo := r.Repo
	runID := r.Run.ID
	return m, func() tea.Msg {
		lines, err := gh.FetchJobLogs(repo, job)
		return logViewerLoadedMsg{runID: runID, lines: lines, err: err}
	}
}

func (m model) View() string {
	header := titleStyle.Render("  GitHub Actions Monitor")
	if m.rateLimited {
		remaining := time.Until(m.rateLimitReset)
		header += runningStyle.Render(fmt.Sprintf("  [rate limited, resets in %s]", formatDuration(remaining)))
	}
	if m.err != nil {
		header += "  " + failureStyle.Render(fmt.Sprintf("error: %s", m.err))
	}

	if m.mode == modeLogs {
		return m.logviewer.View(m.width, m.height)
	}
	if m.help.IsOpen() {
		return m.help.View(m.width, m.height)
	}
	if m.confirm.IsOpen() {
		return m.confirm.View(m.width)
	}

	overview := m.overview.View(m.spinnerIndex)
	detail := m.detail.View()

	leftInner := overviewWidth(m.width) - 4 // borders + padding
	rightInner := detailWidth(m.width) - 4
	if leftInner < 10 {
		leftInner = 10
	}
	if rightInner < 10 {
		rightInner = 10
	}

	left := activePaneBorder.Width(leftInner).Render(overview)
	right := paneBorderStyle.Width(rightInner).Render(detail)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	status := dimStyle.Render("  j/k move  enter logs  r refresh  a/f/A filter  / search  ? help  q quit")
	if m.mode == modeFilter {
		status = "  " + m.filterInput.View()
	}
	if m.prompt.IsOpen() {
		status = m.prompt.View()
	}
	if m.toast != nil && time.Now().Before(m.toast.expires) {
		status = "  " + renderToast(*m.toast)
	}

	return header + "\n" + body + "\n" + status
}

func renderToast(t toast) string {
	switch t.level {
	case toastSuccess:
		return toastSuccessStyle.Render(t.text)
	case toastError:
		return toastErrorStyle.Render(t.text)
	default:
		return toastInfoStyle.Render(t.text)
	}
}

func (m *model) setToast(level toastLevel, text string) {
	m.toast = &toast{text: text, level: level, expires: time.Now().Add(toastTTL)}
}

func (m *model) allRuns() []gh.RunInfo {
	var out []gh.RunInfo
	for _, st := range m.watches {
		out = append(out, st.runs...)
	}
	return out
}

func (m *model) syncDetailSelection() {
	if r, ok := m.overview.Selected(); ok {
		m.detail.SetRun(&r)
	} else {
		m.detail.SetRun(nil)
	}
}

func (m model) fetchLogTailCmd(runID int64) tea.Cmd {
	r, ok := m.overview.Selected()
	if !ok || r.Run.ID != runID {
		return nil
	}
	job, step, updatedAt := pickJobForTail(r)
	if job == 0 {
		return nil
	}
	repo := r.Repo
	cache := m.logCache
	return func() tea.Msg {
		lines, err := gh.FetchLogTail(cache, repo, job, step, updatedAt, logTailLines)
		return logTailResultMsg{runID: runID, step: step, lines: lines, err: err}
	}
}

// pickJobForTail returns (jobID, stepName, updatedAt) for the log tail fetch.
// Prefers in-progress jobs; falls back to whichever job is attached
// (failed jobs in completed runs).
func pickJobForTail(r gh.RunInfo) (int64, string, time.Time) {
	for _, j := range r.Jobs {
		if j.Status == "in_progress" {
			return j.ID, j.CurrentStep, latestJobUpdatedAt(j)
		}
	}
	for _, j := range r.Jobs {
		return j.ID, j.CurrentStep, latestJobUpdatedAt(j)
	}
	return 0, "", time.Time{}
}

func latestJobUpdatedAt(j gh.JobInfo) time.Time {
	if !j.UpdatedAt.IsZero() {
		return j.UpdatedAt
	}
	return j.StartedAt
}

func scheduleLogTailCmd(runID int64) tea.Cmd {
	return tea.Tick(logTailSettle, func(t time.Time) tea.Msg {
		return logTailDueMsg{runID: runID}
	})
}

func overviewWidth(w int) int {
	if w < 60 {
		return w
	}
	return w * 55 / 100
}

func detailWidth(w int) int {
	if w < 60 {
		return 0
	}
	return w - overviewWidth(w)
}

func openURL(url string) {
	_ = exec.Command("open", url).Start()
}

// isRerunnable returns true for terminal, non-successful runs. GitHub will
// reject rerun on in-progress runs; this is a client-side pre-check so we can
// surface a friendlier toast than the raw API error.
func isRerunnable(run gh.WorkflowRun) bool {
	if run.Status != "completed" {
		return false
	}
	switch run.Conclusion {
	case "failure", "cancelled", "timed_out", "startup_failure":
		return true
	}
	return false
}

func isCancellable(run gh.WorkflowRun) bool {
	switch run.Status {
	case "in_progress", "queued", "waiting", "pending", "requested":
		return true
	}
	return false
}

func rerunFailedCmd(repo string, runID int64) tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg{kind: actionRerunFailed, repo: repo, err: gh.RerunFailedJobs(repo, runID)}
	}
}

func cancelRunCmd(repo string, runID int64) tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg{kind: actionCancel, repo: repo, err: gh.CancelRun(repo, runID)}
	}
}

// defaultArtifactDir builds a predictable, sibling-friendly path.
// Slashes in "org/repo" are replaced with "-" so it stays a single directory.
func defaultArtifactDir(repo string, runID int64) string {
	safe := strings.ReplaceAll(repo, "/", "-")
	return fmt.Sprintf("./artifacts/%s-%d", safe, runID)
}

func downloadArtifactsCmd(repo string, runID int64, dest string) tea.Cmd {
	return func() tea.Msg {
		path, err := gh.DownloadArtifacts(repo, runID, dest)
		return downloadResultMsg{dest: path, err: err}
	}
}

func isActive(runs []gh.RunInfo) bool {
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

func renderJobLine(job gh.JobInfo, _ int) string {
	switch job.Status {
	case "waiting", "queued", "pending":
		return waitingStyle.Render(fmt.Sprintf("%s  %s", job.Name, job.Status))
	default:
		if job.CurrentStep != "" {
			return fmt.Sprintf("%s  %s", runningStyle.Render(job.Name), stepStyle.Render(job.CurrentStep))
		}
		return runningStyle.Render(job.Name)
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
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func repoFetchCmd(repo string, wfs []string, bc *gh.BranchCache, jc *gh.JobCache) tea.Cmd {
	return func() tea.Msg {
		results, err := gh.FetchRepoRuns(repo, wfs, bc, jc)
		if err != nil && errors.Is(err, gh.ErrRateLimit) {
			return repoFetchResultMsg{repo: repo, rateLimit: true}
		}
		return repoFetchResultMsg{repo: repo, results: results, err: err}
	}
}

func scheduleRateLimitRetry(resetAt time.Time) tea.Cmd {
	wait := time.Until(resetAt) + 2*time.Second
	if wait < 10*time.Second {
		wait = 10 * time.Second
	}
	return tea.Tick(wait, func(t time.Time) tea.Msg { return rateLimitRetryMsg{} })
}
