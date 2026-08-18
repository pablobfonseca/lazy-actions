package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pablobfonseca/lazy-actions/internal/gh"
)

type claudeLogsMsg struct {
	run     *gh.RunInfo
	logPath string
	err     error
}

type claudeDoneMsg struct{ err error }

func canFixWithClaude(r gh.RunInfo) bool {
	return r.Run.Status == "completed" && r.Run.Conclusion == "failure" && len(r.Jobs) > 0
}

func prepareClaudeCmd(r gh.RunInfo) tea.Cmd {
	return func() tea.Msg {
		if _, err := exec.LookPath("claude"); err != nil {
			return claudeLogsMsg{err: fmt.Errorf("claude CLI not found in PATH")}
		}

		var buf strings.Builder
		var firstErr error
		fetched := 0
		for _, job := range r.Jobs {
			lines, err := gh.FetchJobLogs(r.Repo, job.ID)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			fmt.Fprintf(&buf, "=== job: %s ===\n", job.Name)
			for _, line := range lines {
				buf.WriteString(line)
				buf.WriteString("\n")
			}
			fetched++
		}
		if fetched == 0 {
			return claudeLogsMsg{err: firstErr}
		}

		f, err := os.CreateTemp("", "lazyactions-failed-run-*.log")
		if err != nil {
			return claudeLogsMsg{err: err}
		}
		if _, err := f.WriteString(buf.String()); err != nil {
			f.Close()
			return claudeLogsMsg{err: err}
		}
		if err := f.Close(); err != nil {
			return claudeLogsMsg{err: err}
		}

		return claudeLogsMsg{run: &r, logPath: f.Name()}
	}
}

func buildClaudePrompt(r gh.RunInfo, logPath string) string {
	names := make([]string, 0, len(r.Jobs))
	for _, job := range r.Jobs {
		names = append(names, job.Name)
	}
	return fmt.Sprintf(`The GitHub Actions run for %s (workflow %s, branch %s) failed.
Run URL: %s
Failed job(s): %s
The full job logs are in %s — read them first.
Find the cause of the failure and fix it in this repository, then explain what you changed.`,
		r.Repo, r.Workflow, r.Run.HeadBranch, r.Run.HTMLURL, strings.Join(names, ", "), logPath)
}

func execClaudeCmd(r gh.RunInfo, logPath string) tea.Cmd {
	return tea.ExecProcess(exec.Command("claude", buildClaudePrompt(r, logPath)), func(err error) tea.Msg {
		return claudeDoneMsg{err: err}
	})
}
