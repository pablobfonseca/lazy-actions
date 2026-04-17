package main

import (
	"testing"
	"time"
)

func fixtureRuns() []RunInfo {
	return []RunInfo{
		{Repo: "acme/web", Workflow: "ci.yml", Run: WorkflowRun{ID: 1, Status: "in_progress", HeadBranch: "feat/login"}},
		{Repo: "acme/web", Workflow: "ci.yml", Run: WorkflowRun{ID: 2, Status: "completed", Conclusion: "success", HeadBranch: "main"}},
		{Repo: "acme/api", Workflow: "deploy.yml", Run: WorkflowRun{ID: 3, Status: "completed", Conclusion: "failure", HeadBranch: "fix/x", RunStartedAt: time.Now()}},
		{Repo: "acme/api", Workflow: "ci.yml", Run: WorkflowRun{ID: 4, Status: "waiting", HeadBranch: "feat/big"}},
	}
}

func TestStatusFilterActive(t *testing.T) {
	got := applyFilters(fixtureRuns(), filterState{status: statusActive})
	ids := runIDs(got)
	want := []int64{1, 4}
	if !equalIDs(ids, want) {
		t.Errorf("statusActive: got %v, want %v", ids, want)
	}
}

func TestStatusFilterFailed(t *testing.T) {
	got := applyFilters(fixtureRuns(), filterState{status: statusFailed})
	ids := runIDs(got)
	want := []int64{3}
	if !equalIDs(ids, want) {
		t.Errorf("statusFailed: got %v, want %v", ids, want)
	}
}

func TestStatusFilterAll(t *testing.T) {
	got := applyFilters(fixtureRuns(), filterState{status: statusAll})
	if len(got) != 4 {
		t.Errorf("statusAll: got %d runs, want 4", len(got))
	}
}

func runIDs(runs []RunInfo) []int64 {
	out := make([]int64, len(runs))
	for i, r := range runs {
		out[i] = r.Run.ID
	}
	return out
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
