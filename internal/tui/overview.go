package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pablobfonseca/lazy-actions/internal/gh"
)

type overviewModel struct {
	runs      []gh.RunInfo // full, unfiltered
	filter    filterState
	cursorID  int64 // selected run by ID (stable across updates)
	width     int
	sameOwner bool
	owner     string

	mobileEnabled func(repo, workflow string) bool
}

func newOverview() *overviewModel {
	return &overviewModel{}
}

// SetRuns replaces the backing run list, preserving the cursor by ID when possible.
func (o *overviewModel) SetRuns(runs []gh.RunInfo) {
	o.runs = runs
	o.refreshOwnerSameness()
	visible := o.visibleRuns()
	if o.cursorID != 0 {
		for _, r := range visible {
			if r.Run.ID == o.cursorID {
				return
			}
		}
	}
	if len(visible) > 0 {
		o.cursorID = visible[0].Run.ID
	} else {
		o.cursorID = 0
	}
}

func (o *overviewModel) SetFilter(f filterState) {
	o.filter = f
	o.SetRuns(o.runs) // re-validates cursor against visibility
}

func (o *overviewModel) SetSize(width int) { o.width = width }

func (o *overviewModel) SelectedID() int64 { return o.cursorID }

func (o *overviewModel) Selected() (gh.RunInfo, bool) {
	for _, r := range o.visibleRuns() {
		if r.Run.ID == o.cursorID {
			return r, true
		}
	}
	return gh.RunInfo{}, false
}

func (o *overviewModel) MoveCursor(delta int) {
	visible := o.visibleRuns()
	if len(visible) == 0 {
		o.cursorID = 0
		return
	}
	idx := 0
	for i, r := range visible {
		if r.Run.ID == o.cursorID {
			idx = i
			break
		}
	}
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx > len(visible)-1 {
		idx = len(visible) - 1
	}
	o.cursorID = visible[idx].Run.ID
}

func (o *overviewModel) CursorTop()    { o.moveToIndex(0) }
func (o *overviewModel) CursorBottom() { o.moveToIndex(-1) }

func (o *overviewModel) moveToIndex(idx int) {
	visible := o.visibleRuns()
	if len(visible) == 0 {
		return
	}
	if idx < 0 {
		idx = len(visible) - 1
	}
	o.cursorID = visible[idx].Run.ID
}

// visibleRuns applies filters and sorts Active→Recent, most-recent-first within each group.
func (o *overviewModel) visibleRuns() []gh.RunInfo {
	active, recent := o.visibleSections()
	return append(active, recent...)
}

// visibleSections returns the same runs as visibleRuns but pre-split into the
// two sections View() renders. Kept separate so View() doesn't re-run the
// filter and sort logic twice per render.
func (o *overviewModel) visibleSections() (active, recent []gh.RunInfo) {
	filtered := applyFilters(o.runs, o.filter)
	active, recent = splitActiveRecent(filtered)
	sort.SliceStable(active, func(i, j int) bool {
		return active[i].Run.RunStartedAt.After(active[j].Run.RunStartedAt)
	})
	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].Run.RunStartedAt.After(recent[j].Run.RunStartedAt)
	})
	return active, recent
}

func splitActiveRecent(runs []gh.RunInfo) (active, recent []gh.RunInfo) {
	for _, r := range runs {
		if r.Run.Status == "in_progress" || r.Run.Status == "waiting" ||
			time.Since(r.Run.RunStartedAt) < recentThreshold {
			active = append(active, r)
		} else {
			recent = append(recent, r)
		}
	}
	return
}

func (o *overviewModel) refreshOwnerSameness() {
	o.sameOwner = true
	o.owner = ""
	for _, r := range o.runs {
		owner, _, _ := strings.Cut(r.Repo, "/")
		if o.owner == "" {
			o.owner = owner
		} else if owner != o.owner {
			o.sameOwner = false
			o.owner = ""
			return
		}
	}
}

func (o *overviewModel) displayRepo(repo string) string {
	if o.sameOwner {
		if _, name, ok := strings.Cut(repo, "/"); ok {
			return name
		}
	}
	return repo
}

// Update handles navigation keys. Caller routes tea.KeyMsg only in normal mode.
func (o *overviewModel) Update(msg tea.KeyMsg) (*overviewModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		o.MoveCursor(1)
	case "k", "up":
		o.MoveCursor(-1)
	case "g":
		o.CursorTop()
	case "G":
		o.CursorBottom()
	}
	return o, nil
}

func (o *overviewModel) View(spinnerIndex int, initialLoading bool) string {
	active, recent := o.visibleSections()

	var b strings.Builder
	if len(active) > 0 {
		b.WriteString(sectionLabelStyle.Render("Active"))
		b.WriteString("\n")
		for _, r := range active {
			b.WriteString(o.renderLine(r, spinnerIndex))
			b.WriteString("\n")
		}
	}
	if len(recent) > 0 {
		if len(active) > 0 {
			b.WriteString("\n")
		}
		b.WriteString(sectionLabelStyle.Render("Recent"))
		b.WriteString("\n")
		for _, r := range recent {
			b.WriteString(o.renderLine(r, spinnerIndex))
			b.WriteString("\n")
		}
	}
	if len(active) == 0 && len(recent) == 0 {
		if initialLoading {
			b.WriteString("  ")
			b.WriteString(runningStyle.Render(spinnerFrames[spinnerIndex]))
			b.WriteString(" ")
			b.WriteString(waitingStyle.Render("Loading actions…"))
		} else {
			b.WriteString(dimStyle.Render("  no runs"))
		}
	}
	return b.String()
}

func (o *overviewModel) renderLine(r gh.RunInfo, spinnerIndex int) string {
	selected := r.Run.ID == o.cursorID
	icon, style := lineIconAndStyle(r, spinnerIndex)
	var elapsed string
	if r.Run.Status == "completed" {
		elapsed = formatDuration(r.Run.UpdatedAt.Sub(r.Run.RunStartedAt))
	} else {
		elapsed = formatDuration(time.Since(r.Run.RunStartedAt))
	}
	wf := r.Workflow
	if o.mobileEnabled != nil && o.mobileEnabled(r.Repo, r.Workflow) {
		wf += " 🔔"
	}
	line := fmt.Sprintf("%s %s  %s/%s · %s",
		style.Render(icon),
		style.Render(elapsed),
		o.displayRepo(r.Repo),
		wf,
		branchStyle.Render(r.Run.HeadBranch),
	)
	prefix := "  "
	if selected {
		prefix = selectedBarStyle.Render("▎ ")
		line = selectedRowStyle.Render(line)
	}
	return prefix + line
}

func lineIconAndStyle(r gh.RunInfo, spinnerIndex int) (string, lipgloss.Style) {
	switch {
	case r.Run.Status == "waiting":
		return "⊙", awaitingApprovalStyle
	case r.Run.Status == "in_progress":
		return spinnerFrames[spinnerIndex], runningStyle
	case r.Run.Status == "completed" && r.Run.Conclusion == "success":
		return "✓", successStyle
	case r.Run.Status == "completed" && r.Run.Conclusion == "failure":
		return "✗", failureStyle
	case r.Run.Status == "completed" && r.Run.Conclusion == "cancelled":
		return "⊘", dimStyle
	default:
		return "?", dimStyle
	}
}
