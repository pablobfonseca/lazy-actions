package tui

import (
	"strings"
	"testing"

	"github.com/pablobfonseca/lazy-actions/internal/gh"
)

func TestCanFixWithClaude(t *testing.T) {
	cases := []struct {
		name string
		run  gh.RunInfo
		want bool
	}{
		{
			name: "completed failure with a job",
			run:  gh.RunInfo{Run: gh.WorkflowRun{Status: "completed", Conclusion: "failure"}, Jobs: []gh.JobInfo{{ID: 1, Name: "build"}}},
			want: true,
		},
		{
			name: "completed failure without jobs",
			run:  gh.RunInfo{Run: gh.WorkflowRun{Status: "completed", Conclusion: "failure"}},
			want: false,
		},
		{
			name: "completed success",
			run:  gh.RunInfo{Run: gh.WorkflowRun{Status: "completed", Conclusion: "success"}, Jobs: []gh.JobInfo{{ID: 1, Name: "build"}}},
			want: false,
		},
		{
			name: "in progress",
			run:  gh.RunInfo{Run: gh.WorkflowRun{Status: "in_progress"}, Jobs: []gh.JobInfo{{ID: 1, Name: "build"}}},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canFixWithClaude(tc.run); got != tc.want {
				t.Errorf("canFixWithClaude: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildClaudePrompt(t *testing.T) {
	run := gh.RunInfo{
		Repo:     "owner/repo",
		Workflow: "ci.yml",
		Run: gh.WorkflowRun{
			HeadBranch: "main",
			HTMLURL:    "https://example.com/run/1",
		},
		Jobs: []gh.JobInfo{{ID: 1, Name: "build"}, {ID: 2, Name: "check"}},
	}

	prompt := buildClaudePrompt(run, "/tmp/x.log")

	for _, want := range []string{"owner/repo", "ci.yml", "main", "https://example.com/run/1", "build, check", "/tmp/x.log"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
