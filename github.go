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
	HTMLURL    string    `json:"html_url"`
	Conclusion string    `json:"conclusion"`
	Steps      []JobStep `json:"steps"`
}

type JobsResponse struct {
	Jobs []Job `json:"jobs"`
}

// RunInfo is the combined view of a workflow run with its current step.
type RunInfo struct {
	Run         WorkflowRun
	CurrentStep string
	JobURL      string // direct link to the relevant job
	Repo        string
	Workflow    string
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

// fetchRuns returns in-progress and the latest completed run for a repo/workflow.
func fetchRuns(repo, workflow string) ([]RunInfo, error) {
	endpoint := fmt.Sprintf("repos/%s/actions/workflows/%s/runs?per_page=5&status=in_progress", repo, workflow)
	data, err := ghAPI(endpoint)
	if err != nil {
		return nil, err
	}

	var resp WorkflowRunsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing runs: %w", err)
	}

	var results []RunInfo
	for _, run := range resp.WorkflowRuns {
		step, jobURL := fetchActiveJob(repo, run.ID)
		results = append(results, RunInfo{
			Run:         run,
			CurrentStep: step,
			JobURL:      jobURL,
			Repo:        repo,
			Workflow:    workflow,
		})
	}

	// Fetch latest completed run
	completedEndpoint := fmt.Sprintf("repos/%s/actions/workflows/%s/runs?per_page=1&status=completed", repo, workflow)
	completedData, err := ghAPI(completedEndpoint)
	if err == nil {
		var completedResp WorkflowRunsResponse
		if err := json.Unmarshal(completedData, &completedResp); err == nil && len(completedResp.WorkflowRuns) > 0 {
			run := completedResp.WorkflowRuns[0]
			jobURL := ""
			if run.Conclusion == "failure" {
				_, jobURL = fetchFailedJob(repo, run.ID)
			}
			results = append(results, RunInfo{
				Run:      run,
				JobURL:   jobURL,
				Repo:     repo,
				Workflow: workflow,
			})
		}
	}

	return results, nil
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

// fetchActiveJob returns the current step name and job URL for an in-progress run.
func fetchActiveJob(repo string, runID int64) (string, string) {
	jobs, err := fetchJobs(repo, runID)
	if err != nil {
		return "", ""
	}

	for _, job := range jobs {
		for _, step := range job.Steps {
			if step.Status == "in_progress" {
				return step.Name, job.HTMLURL
			}
		}
	}

	// Fallback: last completed step
	for i := len(jobs) - 1; i >= 0; i-- {
		for j := len(jobs[i].Steps) - 1; j >= 0; j-- {
			if jobs[i].Steps[j].Status == "completed" {
				return jobs[i].Steps[j].Name, jobs[i].HTMLURL
			}
		}
	}

	return "", ""
}

// fetchFailedJob returns the name and URL of the first failed job.
func fetchFailedJob(repo string, runID int64) (string, string) {
	jobs, err := fetchJobs(repo, runID)
	if err != nil {
		return "", ""
	}

	for _, job := range jobs {
		if job.Conclusion == "failure" {
			return job.Name, job.HTMLURL
		}
	}

	return "", ""
}

