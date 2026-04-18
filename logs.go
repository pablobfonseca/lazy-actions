package main

import (
	"bufio"
	"fmt"
	"strings"
	"sync"
	"time"
)

type logCacheKey struct {
	jobID     int64
	step      string
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

// fetchJobLogs returns the full log text for a single job.
// The GitHub API endpoint redirects to a plaintext download; gh follows it.
func fetchJobLogs(repo string, jobID int64) ([]string, error) {
	endpoint := fmt.Sprintf("repos/%s/actions/jobs/%d/logs", repo, jobID)
	data, err := ghAPI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetching job logs: %w", err)
	}
	return splitLines(string(data)), nil
}

// fetchLogTail returns at most n trailing lines of the given job's logs.
// Uses the cache; on miss, calls fetchJobLogs and caches the full result.
func fetchLogTail(c *LogCache, repo string, jobID int64, step string, updatedAt time.Time, n int) ([]string, error) {
	key := logCacheKey{jobID: jobID, step: step, updatedAt: updatedAt}
	if lines, ok := c.Get(key); ok {
		return tailN(lines, n), nil
	}
	lines, err := fetchJobLogs(repo, jobID)
	if err != nil {
		return nil, err
	}
	c.Set(key, lines)
	return tailN(lines, n), nil
}

func splitLines(s string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // allow long log lines
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

// colorizeLogLine applies GitHub Actions-aware coloring:
// dim ISO-8601 timestamp prefix, then style the rest by ##[level] marker
// (warning, error, group, notice, debug, command). Plain lines pass through.
func colorizeLogLine(line string) string {
	rest := line
	var ts string
	if i := timestampEnd(line); i > 0 {
		ts = line[:i]
		rest = line[i:]
	}
	out := logTimestampStyle.Render(ts)

	switch {
	case strings.HasPrefix(rest, "##[error]"):
		out += logErrorStyle.Render(rest)
	case strings.HasPrefix(rest, "##[warning]"):
		out += logWarningStyle.Render(rest)
	case strings.HasPrefix(rest, "##[group]"), strings.HasPrefix(rest, "##[endgroup]"):
		out += logGroupStyle.Render(rest)
	case strings.HasPrefix(rest, "##[notice]"):
		out += logNoticeStyle.Render(rest)
	case strings.HasPrefix(rest, "##[debug]"):
		out += logDebugStyle.Render(rest)
	case strings.HasPrefix(rest, "##[command]"), strings.HasPrefix(rest, "[command]"):
		out += logCommandStyle.Render(rest)
	default:
		out += rest
	}
	return out
}

// timestampEnd returns the byte index where a leading ISO-8601 timestamp
// (e.g. "2026-04-06T14:38:50.2346676Z ") ends, or 0 if none found.
func timestampEnd(s string) int {
	if len(s) < 20 {
		return 0
	}
	for i, c := range []byte("0000-00-00T00:00:00") {
		if c == '0' {
			if s[i] < '0' || s[i] > '9' {
				return 0
			}
		} else if s[i] != c {
			return 0
		}
	}
	i := 19
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i >= len(s) || s[i] != 'Z' {
		return 0
	}
	i++
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}
