package tui

import (
	"strings"

	"github.com/upscopeio/lazy-actions/internal/gh"
)

type filterStatus int

const (
	statusAll filterStatus = iota
	statusActive
	statusFailed
)

type filterState struct {
	status filterStatus
	fuzzy  string // Task 5 uses this
}

func applyFilters(runs []gh.RunInfo, f filterState) []gh.RunInfo {
	out := make([]gh.RunInfo, 0, len(runs))
	for _, r := range runs {
		if !matchStatus(r, f.status) {
			continue
		}
		if !matchFuzzy(r, f.fuzzy) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func matchStatus(r gh.RunInfo, s filterStatus) bool {
	switch s {
	case statusActive:
		return r.Run.Status == "in_progress" || r.Run.Status == "waiting"
	case statusFailed:
		return r.Run.Status == "completed" && r.Run.Conclusion == "failure"
	default:
		return true
	}
}

func matchFuzzy(r gh.RunInfo, needle string) bool {
	if needle == "" {
		return true
	}
	n := strings.ToLower(needle)
	return strings.Contains(strings.ToLower(r.Repo), n) ||
		strings.Contains(strings.ToLower(r.Workflow), n) ||
		strings.Contains(strings.ToLower(r.Run.HeadBranch), n)
}
