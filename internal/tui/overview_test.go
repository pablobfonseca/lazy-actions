package tui

import (
	"testing"

	"github.com/pablobfonseca/lazy-actions/internal/gh"
)

func TestOverviewCursorStableAfterInsert(t *testing.T) {
	ov := newOverview()
	runs := []gh.RunInfo{
		{Run: gh.WorkflowRun{ID: 10, Status: "in_progress"}},
		{Run: gh.WorkflowRun{ID: 20, Status: "in_progress"}},
	}
	ov.SetRuns(runs)
	ov.MoveCursor(1) // cursor on ID 20

	// New run inserted at the top (newer start time in real life)
	updated := []gh.RunInfo{
		{Run: gh.WorkflowRun{ID: 99, Status: "in_progress"}},
		{Run: gh.WorkflowRun{ID: 10, Status: "in_progress"}},
		{Run: gh.WorkflowRun{ID: 20, Status: "in_progress"}},
	}
	ov.SetRuns(updated)

	if got := ov.SelectedID(); got != 20 {
		t.Errorf("cursor moved: got run %d selected, want 20", got)
	}
}

func TestOverviewCursorFallsBackWhenSelectedGone(t *testing.T) {
	ov := newOverview()
	runs := []gh.RunInfo{
		{Run: gh.WorkflowRun{ID: 10, Status: "in_progress"}},
		{Run: gh.WorkflowRun{ID: 20, Status: "in_progress"}},
	}
	ov.SetRuns(runs)
	ov.MoveCursor(1) // ID 20

	ov.SetRuns([]gh.RunInfo{{Run: gh.WorkflowRun{ID: 10, Status: "in_progress"}}})
	if got := ov.SelectedID(); got != 10 {
		t.Errorf("fallback: got %d, want 10", got)
	}
}

func TestLineIconDistinguishesWaitingFromRunning(t *testing.T) {
	waitingIcon, _ := lineIconAndStyle(gh.RunInfo{Run: gh.WorkflowRun{Status: "waiting"}}, 0)
	runningIcon, _ := lineIconAndStyle(gh.RunInfo{Run: gh.WorkflowRun{Status: "in_progress"}}, 0)

	if waitingIcon == runningIcon {
		t.Errorf("waiting and in_progress share icon %q; a gated run is indistinguishable from a running one", waitingIcon)
	}
	if waitingIcon == "" {
		t.Error("waiting run has no icon")
	}
}
