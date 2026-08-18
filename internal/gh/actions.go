package gh

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNotRerunnable is returned when a run cannot be re-run (still running,
// already re-running, or no failed jobs for rerun-failed).
var ErrNotRerunnable = errors.New("run is not re-runnable")

// ErrNotCancellable is returned when a run is already in a terminal state.
var ErrNotCancellable = errors.New("run is not cancellable")

// ghAPIPost issues `gh api -X POST <endpoint>` and normalizes error shape.
// Empty 204 responses are success; GitHub returns JSON error bodies on failure.
func ghAPIPost(endpoint string) error {
	cmd := exec.Command("gh", "api", "-X", "POST", endpoint)
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
