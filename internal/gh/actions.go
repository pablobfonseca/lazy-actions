package gh

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ErrNotRerunnable is returned when a run cannot be re-run (still running,
// already re-running, or no failed jobs for rerun-failed).
var ErrNotRerunnable = errors.New("run is not re-runnable")

// ErrNotCancellable is returned when a run is already in a terminal state.
var ErrNotCancellable = errors.New("run is not cancellable")

// ghAPIPost issues `gh api -X POST <endpoint>` with no request body.
func ghAPIPost(endpoint string) error {
	return ghAPIPostJSON(endpoint, nil)
}

func ghAPIPostArgs(endpoint string, body []byte) []string {
	args := []string{"api", "-X", "POST", endpoint}
	if body != nil {
		args = append(args, "--input", "-")
	}
	return args
}

// ghAPIPostJSON issues `gh api -X POST <endpoint>` and normalizes error shape.
// Empty 204 responses are success; GitHub returns JSON error bodies on failure.
// A nil body sends no body at all. A non-nil body is piped through stdin rather
// than -f/-F so the request is exactly the bytes encoding/json produced.
func ghAPIPostJSON(endpoint string, body []byte) error {
	cmd := exec.Command("gh", ghAPIPostArgs(endpoint, body)...)
	if body != nil {
		cmd.Stdin = bytes.NewReader(body)
	}
	if _, err := cmd.Output(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "rate limit") {
				return fmt.Errorf("%w: %s", ErrRateLimit, endpoint)
			}
			return fmt.Errorf("gh api POST %s: %s", endpoint, stderr)
		}
		return fmt.Errorf("gh api POST %s: %w", endpoint, err)
	}
	return nil
}

// RerunFailedJobs re-runs only the failed jobs of a completed run.
// Endpoint: POST /repos/{repo}/actions/runs/{runID}/rerun-failed-jobs
func RerunFailedJobs(repo string, runID int64) error {
	endpoint := fmt.Sprintf("repos/%s/actions/runs/%d/rerun-failed-jobs", repo, runID)
	err := ghAPIPost(endpoint)
	if err != nil && strings.Contains(err.Error(), "Unable to re-run") {
		return fmt.Errorf("%w: %s/%d", ErrNotRerunnable, repo, runID)
	}
	return err
}

// CancelRun cancels an in-progress run.
// Endpoint: POST /repos/{repo}/actions/runs/{runID}/cancel
func CancelRun(repo string, runID int64) error {
	endpoint := fmt.Sprintf("repos/%s/actions/runs/%d/cancel", repo, runID)
	err := ghAPIPost(endpoint)
	if err != nil && strings.Contains(err.Error(), "Cannot cancel a workflow run") {
		return fmt.Errorf("%w: %s/%d", ErrNotCancellable, repo, runID)
	}
	return err
}

type DeploymentEnvironment struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// PendingDeployment is one environment gate holding a run at status "waiting".
type PendingDeployment struct {
	Environment           DeploymentEnvironment `json:"environment"`
	WaitTimer             int                   `json:"wait_timer"`
	WaitTimerStartedAt    *time.Time            `json:"wait_timer_started_at"`
	CurrentUserCanApprove bool                  `json:"current_user_can_approve"`
}

func pendingDeploymentsEndpoint(repo string, runID int64) string {
	return fmt.Sprintf("repos/%s/actions/runs/%d/pending_deployments", repo, runID)
}

// FetchPendingDeployments lists the environment gates a waiting run is held on.
// Endpoint: GET /repos/{repo}/actions/runs/{runID}/pending_deployments
// An empty slice is a valid answer: the run is not, or is no longer, gated.
func FetchPendingDeployments(repo string, runID int64) ([]PendingDeployment, error) {
	data, err := ghAPI(pendingDeploymentsEndpoint(repo, runID))
	if err != nil {
		return nil, err
	}
	return parsePendingDeployments(data)
}

func parsePendingDeployments(data []byte) ([]PendingDeployment, error) {
	var pending []PendingDeployment
	if err := json.Unmarshal(data, &pending); err != nil {
		return nil, fmt.Errorf("parse pending deployments: %w", err)
	}
	return pending, nil
}

// ApprovePendingDeployments approves the given environment gates for a run.
// Endpoint: POST /repos/{repo}/actions/runs/{runID}/pending_deployments
func ApprovePendingDeployments(repo string, runID int64, environmentIDs []int64) error {
	body, err := approvalRequestBody(environmentIDs)
	if err != nil {
		return err
	}
	return ghAPIPostJSON(pendingDeploymentsEndpoint(repo, runID), body)
}

func approvalRequestBody(environmentIDs []int64) ([]byte, error) {
	if len(environmentIDs) == 0 {
		return nil, errors.New("no environments to approve")
	}
	return json.Marshal(struct {
		EnvironmentIDs []int64 `json:"environment_ids"`
		State          string  `json:"state"`
		Comment        string  `json:"comment"`
	}{EnvironmentIDs: environmentIDs, State: "approved", Comment: ""})
}

// DownloadArtifacts downloads all artifacts for a run into destDir using the
// `gh run download` subcommand. destDir is created by gh if it doesn't exist.
// Returns the destination directory on success.
//
// Shells out to `gh run download <runID> -R <repo> -D <destDir>` rather than
// the raw artifacts API because gh handles the two-step download-URL dance,
// zip extraction, and per-artifact subdirectories for free.
func DownloadArtifacts(repo string, runID int64, destDir string) (string, error) {
	cmd := exec.Command("gh", "run", "download",
		fmt.Sprintf("%d", runID),
		"-R", repo,
		"-D", destDir,
	)
	if _, err := cmd.Output(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "no artifacts") {
				return "", fmt.Errorf("no artifacts for %s run %d", repo, runID)
			}
			return "", fmt.Errorf("gh run download: %s", strings.TrimSpace(stderr))
		}
		return "", fmt.Errorf("gh run download: %w", err)
	}
	return destDir, nil
}
