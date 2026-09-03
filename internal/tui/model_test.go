package tui

import (
	"testing"
	"time"

	"github.com/pablobfonseca/lazy-actions/internal/gh"
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

func TestIsApprovable(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{status: "waiting", want: true},
		{status: "in_progress", want: false},
		{status: "queued", want: false},
		{status: "pending", want: false},
		{status: "requested", want: false},
		{status: "completed", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			if got := isApprovable(gh.WorkflowRun{Status: tc.status}); got != tc.want {
				t.Errorf("isApprovable(%q): got %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

func TestApprovableEnvironments(t *testing.T) {
	cases := []struct {
		name       string
		pending    []gh.PendingDeployment
		wantIDs    []int64
		wantNames  []string
		wantReason string
	}{
		{
			name:       "no pending deployments",
			pending:    nil,
			wantReason: "no pending deployments for this run",
		},
		{
			name: "pending but user cannot approve any",
			pending: []gh.PendingDeployment{
				{Environment: gh.DeploymentEnvironment{ID: 1, Name: "staging"}},
				{Environment: gh.DeploymentEnvironment{ID: 2, Name: "production"}},
			},
			wantReason: "you are not a required reviewer for this run",
		},
		{
			name: "all approvable",
			pending: []gh.PendingDeployment{
				{Environment: gh.DeploymentEnvironment{ID: 1, Name: "staging"}, CurrentUserCanApprove: true},
				{Environment: gh.DeploymentEnvironment{ID: 2, Name: "production"}, CurrentUserCanApprove: true},
			},
			wantIDs:   []int64{1, 2},
			wantNames: []string{"staging", "production"},
		},
		{
			name: "only the approvable subset is returned",
			pending: []gh.PendingDeployment{
				{Environment: gh.DeploymentEnvironment{ID: 1, Name: "staging"}},
				{Environment: gh.DeploymentEnvironment{ID: 2, Name: "production"}, CurrentUserCanApprove: true},
			},
			wantIDs:   []int64{2},
			wantNames: []string{"production"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ids, names, reason := approvableEnvironments(tc.pending)
			if reason != tc.wantReason {
				t.Errorf("reason: got %q, want %q", reason, tc.wantReason)
			}
			if len(ids) != len(tc.wantIDs) || len(names) != len(tc.wantNames) {
				t.Fatalf("got ids %v names %v, want ids %v names %v", ids, names, tc.wantIDs, tc.wantNames)
			}
			for i := range tc.wantIDs {
				if ids[i] != tc.wantIDs[i] || names[i] != tc.wantNames[i] {
					t.Errorf("entry %d: got (%d, %q), want (%d, %q)", i, ids[i], names[i], tc.wantIDs[i], tc.wantNames[i])
				}
			}
		})
	}
}

func TestToastForApproveDeployment(t *testing.T) {
	level, text := toastForActionResult(actionResultMsg{kind: actionApproveDeployment})
	if level != toastSuccess || text != "deployment approved" {
		t.Errorf("got (%v, %q), want (toastSuccess, \"deployment approved\")", level, text)
	}

	level, text = toastForActionResult(actionResultMsg{kind: actionApproveDeployment, err: gh.ErrRateLimit})
	if level != toastError || text != "rate limited" {
		t.Errorf("rate limited: got (%v, %q), want (toastError, \"rate limited\")", level, text)
	}
}

func TestPendingDeploymentsMsgOnlyOpensConfirmInNormalMode(t *testing.T) {
	msg := pendingDeploymentsMsg{
		run:     gh.RunInfo{Repo: "org/repo", Workflow: "deploy.yml", Run: gh.WorkflowRun{ID: 7, Status: "waiting"}},
		pending: []gh.PendingDeployment{{Environment: gh.DeploymentEnvironment{ID: 1, Name: "production"}, CurrentUserCanApprove: true}},
	}

	tests := []struct {
		name       string
		mode       mode
		helpOpen   bool
		wantOpened bool
	}{
		{"normal mode opens the modal", modeNormal, false, true},
		{"log viewer drops the reply", modeLogs, false, false},
		{"filter mode drops the reply", modeFilter, false, false},
		{"help overlay drops the reply", modeNormal, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := model{mode: tc.mode, confirm: newConfirm(), help: newHelp()}
			if tc.helpOpen {
				m.help.Toggle()
			}

			m.Update(msg)

			if got := m.confirm.IsOpen(); got != tc.wantOpened {
				t.Errorf("confirm.IsOpen(): got %v, want %v", got, tc.wantOpened)
			}
		})
	}
}

func TestPendingDeploymentsMsgDoesNotReplaceAnOpenConfirm(t *testing.T) {
	m := model{mode: modeNormal, confirm: newConfirm(), help: newHelp()}
	m.confirm.Open("Cancel org/repo #1?", nil)

	m.Update(pendingDeploymentsMsg{
		run:     gh.RunInfo{Repo: "org/repo", Run: gh.WorkflowRun{ID: 7}},
		pending: []gh.PendingDeployment{{Environment: gh.DeploymentEnvironment{ID: 1, Name: "production"}, CurrentUserCanApprove: true}},
	})

	if m.confirm.Message != "Cancel org/repo #1?" {
		t.Errorf("approval prompt replaced a live confirm: %q", m.confirm.Message)
	}
}
