package notify

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pablobfonseca/lazy-actions/internal/config"
	"github.com/pablobfonseca/lazy-actions/internal/gh"
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

func TestBuildNotificationInteractive(t *testing.T) {
	run := func(status, conclusion string) gh.RunInfo {
		r := gh.RunInfo{Repo: "o/r", Workflow: "ci.yml"}
		r.Run.ID = 1
		r.Run.Status = status
		r.Run.Conclusion = conclusion
		r.Run.HTMLURL = "https://example.com/run/1"
		r.Run.HeadBranch = "main"
		return r
	}
	tests := []struct {
		name            string
		status, concl   string
		wantInteractive bool
	}{
		{"started", "in_progress", "", false},
		{"success", "completed", "success", false},
		{"cancelled", "completed", "cancelled", false},
		{"failure", "completed", "failure", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := runState{status: tt.status, conclusion: tt.concl}
			n, ok := buildNotification(run(tt.status, tt.concl), runState{}, false, ns)
			if !ok {
				t.Fatal("no notification built")
			}
			if n.interactive != tt.wantInteractive {
				t.Errorf("interactive = %v, want %v", n.interactive, tt.wantInteractive)
			}
			if n.openURL == "" || len(n.actions) == 0 {
				t.Error("openURL/actions must stay populated for mobile channels")
			}
		})
	}
}

func TestNotifiCliArgsInteractivity(t *testing.T) {
	base := notification{
		title:    "t",
		subtitle: "s",
		message:  "m",
		openURL:  "https://example.com/run/1",
		actions:  []notificationAction{{"View Run", "https://example.com/run/1"}},
	}

	joined := func(n notification) string { return strings.Join(notifiCliArgs(n), " ") }

	inter := base
	inter.interactive = true
	got := joined(inter)
	if !strings.Contains(got, "-url") || !strings.Contains(got, "-actions") {
		t.Errorf("interactive args missing -url/-actions: %q", got)
	}

	got = joined(base)
	if strings.Contains(got, "-url") || strings.Contains(got, "-actions") {
		t.Errorf("non-interactive args must omit -url/-actions: %q", got)
	}
	if !strings.Contains(got, "-title") || !strings.Contains(got, "-message") || !strings.Contains(got, "-subtitle") {
		t.Errorf("non-interactive args lost basic fields: %q", got)
	}
}

func TestParseHIDIdleNanos(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    int64
		wantErr bool
	}{
		{"real ioreg line", `    | | |   "HIDIdleTime" = 423260208`, 423260208, false},
		{"embedded in dump", "junk\n  \"HIDIdleTime\" = 90000000000\nmore", 90000000000, false},
		{"missing", "no idle info here", 0, true},
		{"empty", "", 0, true},
		{"scientific notation", `    "HIDIdleTime" = 1.5e9`, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHIDIdleNanos([]byte(tt.out))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("nanos = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSystemIdleLive(t *testing.T) {
	d, err := systemIdle()
	if err != nil {
		t.Fatalf("systemIdle() on macOS: %v", err)
	}
	if d < 0 || d > 24*time.Hour {
		t.Errorf("implausible idle duration %v", d)
	}
}

func TestAwayFromScreen(t *testing.T) {
	tests := []struct {
		name    string
		idleMin time.Duration
		stub    func(*testing.T) func() (time.Duration, error)
		want    bool
	}{
		{
			name:    "zero threshold short-circuits",
			idleMin: 0,
			stub: func(t *testing.T) func() (time.Duration, error) {
				return func() (time.Duration, error) {
					t.Error("systemIdleFn called despite zero threshold")
					return 0, nil
				}
			},
			want: true,
		},
		{
			name:    "negative threshold short-circuits",
			idleMin: -5 * time.Minute,
			stub: func(t *testing.T) func() (time.Duration, error) {
				return func() (time.Duration, error) {
					t.Error("systemIdleFn called despite negative threshold")
					return 0, nil
				}
			},
			want: true,
		},
		{
			name:    "ioreg error fails open",
			idleMin: 5 * time.Minute,
			stub: func(t *testing.T) func() (time.Duration, error) {
				return func() (time.Duration, error) {
					return 0, errors.New("ioreg exploded")
				}
			},
			want: true,
		},
		{
			name:    "idle at threshold",
			idleMin: 5 * time.Minute,
			stub: func(t *testing.T) func() (time.Duration, error) {
				return func() (time.Duration, error) {
					return 5 * time.Minute, nil
				}
			},
			want: true,
		},
		{
			name:    "idle below threshold",
			idleMin: 5 * time.Minute,
			stub: func(t *testing.T) func() (time.Duration, error) {
				return func() (time.Duration, error) {
					return 4*time.Minute + 59*time.Second, nil
				}
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := systemIdleFn
			t.Cleanup(func() { systemIdleFn = orig })
			systemIdleFn = tt.stub(t)

			nt := &NotifyTracker{mobileIdleMin: tt.idleMin}
			if got := nt.awayFromScreen(); got != tt.want {
				t.Errorf("awayFromScreen() = %v, want %v", got, tt.want)
			}
		})
	}
}
