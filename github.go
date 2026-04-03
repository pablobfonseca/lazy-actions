package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var errRateLimit = errors.New("GitHub API rate limit exceeded")

type RateLimitResponse struct {
	Resources struct {
		Core struct {
			Remaining int   `json:"remaining"`
			Reset     int64 `json:"reset"`
		} `json:"core"`
	} `json:"resources"`
}

// fetchRateLimitReset returns the time when the rate limit resets.
func fetchRateLimitReset() time.Time {
	// rate_limit endpoint itself doesn't count against the limit
	cmd := exec.Command("gh", "api", "rate_limit")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}
	}
	var resp RateLimitResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return time.Time{}
	}
	return time.Unix(resp.Resources.Core.Reset, 0)
}

type WorkflowRun struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	HeadBranch   string    `json:"head_branch"`
	RunStartedAt time.Time `json:"run_started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	HTMLURL      string    `json:"html_url"`
}

type WorkflowRunsResponse struct {
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}

type JobStep struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

type Job struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	HTMLURL    string    `json:"html_url"`
	Conclusion string    `json:"conclusion"`
	StartedAt  time.Time `json:"started_at"`
	Steps      []JobStep `json:"steps"`
}

type JobsResponse struct {
	Jobs []Job `json:"jobs"`
}

type JobInfo struct {
	Name        string
	Status      string // "in_progress", "queued", "waiting"
	CurrentStep string
	URL         string
	StartedAt   time.Time
}

// RunInfo is the combined view of a workflow run with its active/failed jobs.
type RunInfo struct {
	Run      WorkflowRun
	Jobs     []JobInfo // active jobs (in-progress) or the failed job (completed)
	Repo     string
	Workflow string
}

func ghAPI(endpoint string) ([]byte, error) {
	cmd := exec.Command("gh", "api", endpoint, "--cache", "0s")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "rate limit") || strings.Contains(stderr, "403") {
				return nil, fmt.Errorf("%w: %s", errRateLimit, endpoint)
			}
			return nil, fmt.Errorf("gh api %s: %s", endpoint, stderr)
		}
		return nil, fmt.Errorf("gh api %s: %w", endpoint, err)
	}
	return out, nil
}

// fetchRuns returns in-progress/waiting runs and completed runs (latest per branch within 24h).
func fetchRuns(repo, workflow string) ([]RunInfo, error) {
	// Fetch both in_progress and waiting runs in parallel
	type fetchResult struct {
		runs []WorkflowRun
		err  error
	}
	ch := make(chan fetchResult, 2)
	for _, status := range []string{"in_progress", "waiting"} {
		go func(s string) {
			endpoint := fmt.Sprintf("repos/%s/actions/workflows/%s/runs?per_page=5&status=%s", repo, workflow, s)
			data, err := ghAPI(endpoint)
			if err != nil {
				ch <- fetchResult{err: err}
				return
			}
			var resp WorkflowRunsResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				ch <- fetchResult{err: fmt.Errorf("parsing runs: %w", err)}
				return
			}
			ch <- fetchResult{runs: resp.WorkflowRuns}
		}(status)
	}

	var activeRuns []WorkflowRun
	for range 2 {
		res := <-ch
		if res.err != nil {
			return nil, res.err
		}
		activeRuns = append(activeRuns, res.runs...)
	}

	var results []RunInfo
	for _, run := range activeRuns {
		jobs := fetchActiveJobs(repo, run.ID)
		results = append(results, RunInfo{
			Run:      run,
			Jobs:     jobs,
			Repo:     repo,
			Workflow: workflow,
		})
	}

	// Fetch recent completed runs — enough to cover multiple branches
	completedEndpoint := fmt.Sprintf("repos/%s/actions/workflows/%s/runs?per_page=20&status=completed", repo, workflow)
	completedData, err := ghAPI(completedEndpoint)
	if err != nil {
		return results, nil
	}

	var completedResp WorkflowRunsResponse
	if err := json.Unmarshal(completedData, &completedResp); err != nil || len(completedResp.WorkflowRuns) == 0 {
		return results, nil
	}

	cutoff := time.Now().Add(-24 * time.Hour)

	// Keep latest run per branch within 24h
	latestPerBranch := make(map[string]WorkflowRun)
	for _, run := range completedResp.WorkflowRuns {
		if run.RunStartedAt.Before(cutoff) {
			continue
		}
		if _, exists := latestPerBranch[run.HeadBranch]; !exists {
			latestPerBranch[run.HeadBranch] = run
		}
	}

	// Branches with in-progress runs definitely exist
	activeBranches := make(map[string]bool)
	for _, r := range results {
		if r.Run.Status == "in_progress" || r.Run.Status == "waiting" {
			activeBranches[r.Run.HeadBranch] = true
		}
	}

	// Check branch existence in parallel
	type branchCheck struct {
		branch string
		run    WorkflowRun
		exists bool
	}
	branchCh := make(chan branchCheck, len(latestPerBranch))
	for branch, run := range latestPerBranch {
		go func(b string, r WorkflowRun) {
			exists := activeBranches[b] || branchExists(repo, b)
			branchCh <- branchCheck{b, r, exists}
		}(branch, run)
	}

	var recentRuns []WorkflowRun
	for range latestPerBranch {
		check := <-branchCh
		if check.exists {
			recentRuns = append(recentRuns, check.run)
		}
	}

	// If nothing matched, fall back to the overall latest completed run
	if len(recentRuns) == 0 {
		recentRuns = []WorkflowRun{completedResp.WorkflowRuns[0]}
	}

	for _, run := range recentRuns {
		var jobs []JobInfo
		if run.Conclusion == "failure" {
			jobs = fetchFailedJobs(repo, run.ID)
		}
		results = append(results, RunInfo{
			Run:      run,
			Jobs:     jobs,
			Repo:     repo,
			Workflow: workflow,
		})
	}

	return results, nil
}

func branchExists(repo, branch string) bool {
	endpoint := fmt.Sprintf("repos/%s/git/ref/heads/%s", repo, branch)
	_, err := ghAPI(endpoint)
	return err == nil
}

func fetchJobs(repo string, runID int64) ([]Job, error) {
	endpoint := fmt.Sprintf("repos/%s/actions/runs/%d/jobs?filter=latest", repo, runID)
	data, err := ghAPI(endpoint)
	if err != nil {
		return nil, err
	}
	var resp JobsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Jobs, nil
}

// fetchActiveJobs returns all in-progress, queued, or waiting jobs.
func fetchActiveJobs(repo string, runID int64) []JobInfo {
	jobs, err := fetchJobs(repo, runID)
	if err != nil {
		return nil
	}

	var result []JobInfo
	for _, job := range jobs {
		switch job.Status {
		case "waiting", "queued", "pending":
			result = append(result, JobInfo{
				Name:      job.Name,
				Status:    job.Status,
				URL:       job.HTMLURL,
				StartedAt: job.StartedAt,
			})
		case "in_progress":
			result = append(result, JobInfo{
				Name:        job.Name,
				Status:      job.Status,
				CurrentStep: currentStepForJob(job),
				URL:         job.HTMLURL,
				StartedAt:   job.StartedAt,
			})
		case "completed":
			// Skip completed jobs in an active run
		}
	}

	// Fallback: if no active jobs yet, show the last completed step
	if len(result) == 0 {
		for i := len(jobs) - 1; i >= 0; i-- {
			for j := len(jobs[i].Steps) - 1; j >= 0; j-- {
				if jobs[i].Steps[j].Status == "completed" {
					return []JobInfo{{
						Name:        jobs[i].Name,
						Status:      "in_progress",
						CurrentStep: jobs[i].Steps[j].Name,
						URL:         jobs[i].HTMLURL,
						StartedAt:   jobs[i].StartedAt,
					}}
				}
			}
		}
	}

	return result
}

func currentStepForJob(job Job) string {
	for _, step := range job.Steps {
		if step.Status == "in_progress" {
			return step.Name
		}
	}
	// Job is queued or waiting — show last completed step if any
	for i := len(job.Steps) - 1; i >= 0; i-- {
		if job.Steps[i].Status == "completed" {
			return job.Steps[i].Name
		}
	}
	return ""
}

// fetchFailedJobs returns all failed jobs for a completed run.
func fetchFailedJobs(repo string, runID int64) []JobInfo {
	jobs, err := fetchJobs(repo, runID)
	if err != nil {
		return nil
	}

	var result []JobInfo
	for _, job := range jobs {
		if job.Conclusion == "failure" {
			result = append(result, JobInfo{
				Name: job.Name,
				URL:  job.HTMLURL,
			})
		}
	}
	return result
}

