package notify

import (
	"testing"
	"time"

	"github.com/pablobfonseca/lazy-actions/internal/config"
)

func TestRuleAllows(t *testing.T) {
	failures := config.NotifyRules{Only: "failures"}
	quietAllDay := config.NotifyRules{Quiet: []string{"00:00-23:59"}}
	noon := time.Date(2026, 8, 19, 12, 0, 0, 0, time.Local)

	tests := []struct {
		name  string
		rules config.NotifyRules
		state runState
		want  bool
	}{
		{"zero rules allow started", config.NotifyRules{}, runState{status: "in_progress"}, true},
		{"zero rules allow success", config.NotifyRules{}, runState{status: "completed", conclusion: "success"}, true},
		{"failures-only blocks started", failures, runState{status: "in_progress"}, false},
		{"failures-only blocks success", failures, runState{status: "completed", conclusion: "success"}, false},
		{"failures-only blocks cancelled", failures, runState{status: "completed", conclusion: "cancelled"}, false},
		{"failures-only allows failure", failures, runState{status: "completed", conclusion: "failure"}, true},
		{"quiet blocks failure", quietAllDay, runState{status: "completed", conclusion: "failure"}, false},
		{"quiet blocks success", quietAllDay, runState{status: "completed", conclusion: "success"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ruleAllows(tt.rules, tt.state, noon); got != tt.want {
				t.Errorf("ruleAllows() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewNotifyTrackerBuildsRules(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.Config{Watches: []config.WatchEntry{
		{Repo: "o/r", Workflows: []string{"ci.yml", "deploy.yml"}, Notify: config.NotifyRules{Only: "failures"}},
		{Repo: "o/other", Workflows: []string{"build.yml"}},
	}}
	nt := NewNotifyTracker(cfg)
	if !nt.rules[watchKey{"o/r", "ci.yml"}].FailuresOnly() {
		t.Error("rules for o/r ci.yml not failures-only")
	}
	if !nt.rules[watchKey{"o/r", "deploy.yml"}].FailuresOnly() {
		t.Error("rules for o/r deploy.yml not failures-only")
	}
	if nt.rules[watchKey{"o/other", "build.yml"}].FailuresOnly() {
		t.Error("rules for o/other build.yml unexpectedly failures-only")
	}
}
