package tui

import (
	"testing"
	"time"
)

func TestActiveRepoErrors_PrunesExpired(t *testing.T) {
	now := time.Now()
	m := &model{
		repoErrors: map[string]repoError{
			"org/fresh": {repo: "org/fresh", msg: "timeout", expires: now.Add(10 * time.Second)},
			"org/stale": {repo: "org/stale", msg: "old", expires: now.Add(-1 * time.Second)},
		},
	}

	got := m.activeRepoErrors()

	if len(got) != 1 {
		t.Fatalf("want 1 active error, got %d", len(got))
	}
	if got[0].repo != "org/fresh" {
		t.Errorf("want org/fresh, got %s", got[0].repo)
	}
	if _, stillThere := m.repoErrors["org/stale"]; stillThere {
		t.Error("expected stale entry to be pruned from map")
	}
	if _, stillThere := m.repoErrors["org/fresh"]; !stillThere {
		t.Error("expected fresh entry to remain in map")
	}
}

func TestTruncateMsg(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"short", 10, "short"},
		{"exactly-ten", 11, "exactly-ten"},
		{"this is a longer message", 10, "this is a…"},
		{"日本語テスト", 4, "日本語…"},
	}
	for _, tc := range tests {
		if got := truncateMsg(tc.in, tc.max); got != tc.want {
			t.Errorf("truncateMsg(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
		}
	}
}
