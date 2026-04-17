package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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
	Path         string    `json:"path"`
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
	cmd := exec.Command("gh", "api", endpoint)
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

// fetchRepoRuns fetches all runs for a repo in 3 API calls (in_progress, waiting, completed)
// instead of 3 calls per workflow. Results are filtered to watched workflows by path.
func fetchRepoRuns(repo string, workflows []string, branchCache *BranchCache, jobCache *JobCache) (map[string][]RunInfo, error) {
	// Map workflow file paths to config names for filtering
	watchedPaths := make(map[string]string, len(workflows))
	for _, wf := range workflows {
		watchedPaths[".github/workflows/"+wf] = wf
	}

	// Fetch in_progress, waiting, and completed in parallel (3 calls total per repo)
	type statusResult struct {
		status string
		runs   []WorkflowRun
		err    error
	}
	ch := make(chan statusResult, 3)

	for _, status := range []string{"in_progress", "waiting", "completed"} {
		go func(s string) {
			perPage := 20
			if s == "completed" {
				// Need enough to cover all watched workflows across branches
				perPage = 100
			}
			endpoint := fmt.Sprintf("repos/%s/actions/runs?per_page=%d&status=%s", repo, perPage, s)
			data, err := ghAPI(endpoint)
			if err != nil {
				ch <- statusResult{status: s, err: err}
				return
			}
			var resp WorkflowRunsResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				ch <- statusResult{status: s, err: fmt.Errorf("parsing runs: %w", err)}
				return
			}
			ch <- statusResult{status: s, runs: resp.WorkflowRuns}
		}(status)
	}

	var activeRuns, completedRuns []WorkflowRun
	for range 3 {
		res := <-ch
		if res.err != nil {
			return nil, res.err
		}
		if res.status == "completed" {
			completedRuns = res.runs
		} else {
			activeRuns = append(activeRuns, res.runs...)
		}
	}

	// Filter runs to only watched workflows
	filterByWorkflow := func(runs []WorkflowRun) map[string][]WorkflowRun {
		byWf := make(map[string][]WorkflowRun)
		for _, run := range runs {
			if wf, ok := watchedPaths[run.Path]; ok {
				byWf[wf] = append(byWf[wf], run)
			}
		}
		return byWf
	}

	activeByWf := filterByWorkflow(activeRuns)
	completedByWf := filterByWorkflow(completedRuns)

	results := make(map[string][]RunInfo)

	// Process active runs with job caching
	for wf, runs := range activeByWf {
		if len(runs) > 5 {
			runs = runs[:5]
		}
		for _, run := range runs {
			var jobs []JobInfo
			if cached, ok := jobCache.Get(run.ID, run.UpdatedAt); ok {
				jobs = cached
			} else {
				jobs = fetchActiveJobs(repo, run.ID)
				jobCache.Set(run.ID, run.UpdatedAt, jobs)
			}
			results[wf] = append(results[wf], RunInfo{
				Run:      run,
				Jobs:     jobs,
				Repo:     repo,
				Workflow: wf,
			})
		}
	}

	// Process completed runs per workflow (latest per branch within 24h)
	cutoff := time.Now().Add(-24 * time.Hour)
	for wf, runs := range completedByWf {
		latestPerBranch := make(map[string]WorkflowRun)
		for _, run := range runs {
			if run.RunStartedAt.Before(cutoff) {
				continue
			}
			if _, exists := latestPerBranch[run.HeadBranch]; !exists {
				latestPerBranch[run.HeadBranch] = run
			}
		}

		// Branches with active runs definitely exist
		activeBranches := make(map[string]bool)
		for _, run := range activeByWf[wf] {
			activeBranches[run.HeadBranch] = true
		}

		var branchesToCheck []string
		for branch := range latestPerBranch {
			if !activeBranches[branch] {
				branchesToCheck = append(branchesToCheck, branch)
			}
		}

		cached, missing := branchCache.Lookup(repo, branchesToCheck)
		if len(missing) > 0 {
			fresh := batchBranchExists(repo, missing)
			branchCache.Store(repo, fresh)
			for b, exists := range fresh {
				cached[b] = exists
			}
		}

		var recentRuns []WorkflowRun
		for branch, run := range latestPerBranch {
			if activeBranches[branch] || cached[branch] {
				recentRuns = append(recentRuns, run)
			}
		}

		// Fallback: latest completed run for this workflow
		if len(recentRuns) == 0 && len(runs) > 0 {
			recentRuns = []WorkflowRun{runs[0]}
		}

		for _, run := range recentRuns {
			var jobs []JobInfo
			if run.Conclusion == "failure" {
				if cachedJobs, ok := jobCache.Get(run.ID, run.UpdatedAt); ok {
					jobs = cachedJobs
				} else {
					jobs = fetchFailedJobs(repo, run.ID)
					jobCache.Set(run.ID, run.UpdatedAt, jobs)
				}
			}
			results[wf] = append(results[wf], RunInfo{
				Run:      run,
				Jobs:     jobs,
				Repo:     repo,
				Workflow: wf,
			})
		}
	}

	return results, nil
}

func ghGraphQL(query string) ([]byte, error) {
	cmd := exec.Command("gh", "api", "graphql", "-f", "query="+query)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "rate limit") || strings.Contains(stderr, "403") {
				return nil, fmt.Errorf("%w: graphql", errRateLimit)
			}
			return nil, fmt.Errorf("gh api graphql: %s", stderr)
		}
		return nil, fmt.Errorf("gh api graphql: %w", err)
	}
	return out, nil
}

// batchBranchExists checks multiple branches in a single GraphQL query.
// Returns a map of branch name -> exists. On error, optimistically treats all as existing.
func batchBranchExists(repo string, branches []string) map[string]bool {
	result := make(map[string]bool, len(branches))
	if len(branches) == 0 {
		return result
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		for _, b := range branches {
			result[b] = true
		}
		return result
	}
	owner, name := parts[0], parts[1]

	var refs strings.Builder
	for i, branch := range branches {
		refs.WriteString("b")
		refs.WriteString(strconv.Itoa(i))
		refs.WriteString(": ref(qualifiedName: ")
		refs.WriteString(strconv.Quote("refs/heads/" + branch))
		refs.WriteString(") { id }\n")
	}

	query := fmt.Sprintf(`{ repository(owner: %s, name: %s) { %s } }`,
		strconv.Quote(owner), strconv.Quote(name), refs.String())

	out, err := ghGraphQL(query)
	if err != nil {
		// Optimistic fallback: treat all as existing
		for _, b := range branches {
			result[b] = true
		}
		return result
	}

	var resp struct {
		Data struct {
			Repository map[string]json.RawMessage `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		for _, b := range branches {
			result[b] = true
		}
		return result
	}

	for i, branch := range branches {
		alias := "b" + strconv.Itoa(i)
		raw, ok := resp.Data.Repository[alias]
		result[branch] = ok && string(raw) != "null"
	}
	return result
}

// BranchCache provides thread-safe cross-workflow caching of branch existence.
type BranchCache struct {
	mu    sync.Mutex
	cache map[string]map[string]bool // repo -> branch -> exists
}

func NewBranchCache() *BranchCache {
	return &BranchCache{cache: make(map[string]map[string]bool)}
}

// Lookup returns cached results and a list of branches not yet in the cache.
func (bc *BranchCache) Lookup(repo string, branches []string) (found map[string]bool, missing []string) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	repoBranches := bc.cache[repo]
	found = make(map[string]bool, len(branches))
	for _, b := range branches {
		if exists, ok := repoBranches[b]; ok {
			found[b] = exists
		} else {
			missing = append(missing, b)
		}
	}
	return
}

// Store saves branch existence results into the cache.
func (bc *BranchCache) Store(repo string, results map[string]bool) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.cache[repo] == nil {
		bc.cache[repo] = make(map[string]bool)
	}
	for branch, exists := range results {
		bc.cache[repo][branch] = exists
	}
}

// JobCache provides thread-safe caching of job details keyed by run ID and updated_at.
// Avoids redundant API calls when a run's state hasn't changed between polls.
type JobCache struct {
	mu    sync.Mutex
	cache map[int64]jobCacheEntry
}

type jobCacheEntry struct {
	updatedAt time.Time
	jobs      []JobInfo
}

func NewJobCache() *JobCache {
	return &JobCache{cache: make(map[int64]jobCacheEntry)}
}

func (jc *JobCache) Get(runID int64, updatedAt time.Time) ([]JobInfo, bool) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	entry, ok := jc.cache[runID]
	if ok && entry.updatedAt.Equal(updatedAt) {
		return entry.jobs, true
	}
	return nil, false
}

func (jc *JobCache) Set(runID int64, updatedAt time.Time, jobs []JobInfo) {
	jc.mu.Lock()
	defer jc.mu.Unlock()
	jc.cache[runID] = jobCacheEntry{updatedAt: updatedAt, jobs: jobs}
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

