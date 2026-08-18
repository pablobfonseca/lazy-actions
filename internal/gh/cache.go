package gh

import (
	"sync"
	"time"
)

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
// Entries are never pruned; growth is bounded in practice by the
// 24-hour cutoff that the polling layer applies to completed runs.
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
