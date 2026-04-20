package gh

import (
	"bufio"
	"fmt"
	"strings"
	"sync"
	"time"
)

// logCacheKey keys on the job and its updatedAt — the stored value is the
// full job log, so the current step name isn't relevant to identity.
type logCacheKey struct {
	jobID     int64
	updatedAt time.Time
}

type LogCache struct {
	mu    sync.Mutex
	cache map[logCacheKey][]string
}

func NewLogCache() *LogCache {
	return &LogCache{cache: make(map[logCacheKey][]string)}
}

func (c *LogCache) Get(key logCacheKey) ([]string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.cache[key]
	return v, ok
}

func (c *LogCache) Set(key logCacheKey, lines []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = lines
}

// FetchJobLogs returns the full log text for a single job.
// The GitHub API endpoint redirects to a plaintext download; gh follows it.
func FetchJobLogs(repo string, jobID int64) ([]string, error) {
	endpoint := fmt.Sprintf("repos/%s/actions/jobs/%d/logs", repo, jobID)
	data, err := ghAPI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching job logs: %w", err)
	}
	return splitLines(string(data)), nil
}

// FetchLogTail returns at most n trailing lines of the given job's logs.
// Uses the cache; on miss, calls FetchJobLogs and caches the full result.
func FetchLogTail(c *LogCache, repo string, jobID int64, updatedAt time.Time, n int) ([]string, error) {
	key := logCacheKey{jobID: jobID, updatedAt: updatedAt}
	if lines, ok := c.Get(key); ok {
		return tailN(lines, n), nil
	}
	lines, err := FetchJobLogs(repo, jobID)
	if err != nil {
		return nil, err
	}
	c.Set(key, lines)
	return tailN(lines, n), nil
}

func splitLines(s string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

func tailN(lines []string, n int) []string {
	if n >= len(lines) {
		return lines
	}
	return lines[len(lines)-n:]
}
