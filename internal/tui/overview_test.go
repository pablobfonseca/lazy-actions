package tui

import (
	"testing"

	"github.com/upscopeio/lazy-actions/internal/gh"
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
