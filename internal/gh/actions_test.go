package gh

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestParsePendingDeployments(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    []PendingDeployment
		wantErr bool
	}{
		{
			name: "two environments with mixed approval rights",
			body: `[
				{
					"environment": {"id": 161088068, "node_id": "MDExOkVudg==", "name": "staging", "url": "https://api.github.com/x", "html_url": "https://github.com/x"},
					"wait_timer": 30,
					"wait_timer_started_at": "2020-11-23T22:00:40Z",
					"current_user_can_approve": true,
					"reviewers": [{"type": "User", "reviewer": {"login": "octocat"}}]
				},
				{
					"environment": {"id": 161088069, "name": "production"},
					"wait_timer": 0,
					"wait_timer_started_at": null,
					"current_user_can_approve": false,
					"reviewers": []
				}
			]`,
			want: []PendingDeployment{
				{
					Environment:           DeploymentEnvironment{ID: 161088068, Name: "staging"},
					WaitTimer:             30,
					WaitTimerStartedAt:    timePtr(time.Date(2020, 11, 23, 22, 0, 40, 0, time.UTC)),
					CurrentUserCanApprove: true,
				},
				{Environment: DeploymentEnvironment{ID: 161088069, Name: "production"}, CurrentUserCanApprove: false},
			},
		},
		{
			name: "timer-held gate the user cannot approve",
			body: `[
				{
					"environment": {"id": 161088070, "name": "timer-test"},
					"wait_timer": 30,
					"wait_timer_started_at": "2026-09-04T10:09:00Z",
					"current_user_can_approve": false,
					"reviewers": []
				}
			]`,
			want: []PendingDeployment{
				{
					Environment:        DeploymentEnvironment{ID: 161088070, Name: "timer-test"},
					WaitTimer:          30,
					WaitTimerStartedAt: timePtr(time.Date(2026, 9, 4, 10, 9, 0, 0, time.UTC)),
				},
			},
		},
		{
			name: "empty array",
			body: `[]`,
			want: []PendingDeployment{},
		},
		{
			name:    "malformed body",
			body:    `{"message": "Not Found"}`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePendingDeployments([]byte(tc.body))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parsePendingDeployments: got nil err, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePendingDeployments: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d deployments, want %d", len(got), len(tc.want))
			}
			for i := range tc.want {
				g, w := got[i], tc.want[i]
				if g.Environment != w.Environment || g.WaitTimer != w.WaitTimer || g.CurrentUserCanApprove != w.CurrentUserCanApprove {
					t.Errorf("deployment %d: got %+v, want %+v", i, g, w)
				}
				if !sameTime(g.WaitTimerStartedAt, w.WaitTimerStartedAt) {
					t.Errorf("deployment %d: wait_timer_started_at: got %v, want %v", i, g.WaitTimerStartedAt, w.WaitTimerStartedAt)
				}
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func TestApprovalRequestBody(t *testing.T) {
	body, err := approvalRequestBody([]int64{161088068, 161088069})
	if err != nil {
		t.Fatalf("approvalRequestBody: %v", err)
	}

	var decoded struct {
		EnvironmentIDs []json.Number `json:"environment_ids"`
		State          string        `json:"state"`
		Comment        *string       `json:"comment"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		t.Fatalf("decode body %s: %v", body, err)
	}

	if len(decoded.EnvironmentIDs) != 2 || decoded.EnvironmentIDs[0].String() != "161088068" || decoded.EnvironmentIDs[1].String() != "161088069" {
		t.Errorf("got environment_ids %v, want [161088068 161088069] as numbers", decoded.EnvironmentIDs)
	}
	if decoded.State != "approved" {
		t.Errorf("got state %q, want %q", decoded.State, "approved")
	}
	if decoded.Comment == nil {
		t.Error("comment absent from body, want an empty string")
	} else if *decoded.Comment != "" {
		t.Errorf("got comment %q, want empty", *decoded.Comment)
	}
}

func TestApprovalRequestBodyRejectsEmpty(t *testing.T) {
	if _, err := approvalRequestBody(nil); err == nil {
		t.Error("approvalRequestBody(nil): got nil err, want an error")
	}
}

func TestGhAPIPostArgs(t *testing.T) {
	cases := []struct {
		name string
		body []byte
		want []string
	}{
		{
			name: "nil body sends no input flag",
			body: nil,
			want: []string{"api", "-X", "POST", "repos/o/r/actions/runs/1/cancel"},
		},
		{
			name: "body is piped through stdin",
			body: []byte(`{"state":"approved"}`),
			want: []string{"api", "-X", "POST", "repos/o/r/actions/runs/1/cancel", "--input", "-"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ghAPIPostArgs("repos/o/r/actions/runs/1/cancel", tc.body)
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			}
		})
	}
}
