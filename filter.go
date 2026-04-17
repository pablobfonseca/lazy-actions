package main

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

func applyFilters(runs []RunInfo, f filterState) []RunInfo {
	out := make([]RunInfo, 0, len(runs))
	for _, r := range runs {
		if !matchStatus(r, f.status) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func matchStatus(r RunInfo, s filterStatus) bool {
	switch s {
	case statusActive:
		return r.Run.Status == "in_progress" || r.Run.Status == "waiting"
	case statusFailed:
		return r.Run.Status == "completed" && r.Run.Conclusion == "failure"
	default:
		return true
	}
}
