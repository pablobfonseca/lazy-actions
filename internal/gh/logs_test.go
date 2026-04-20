package gh

import (
	"testing"
	"time"
)

func TestLogCacheHitAndMiss(t *testing.T) {
	c := NewLogCache()
	key := logCacheKey{jobID: 1, updatedAt: time.Unix(100, 0)}
	if _, ok := c.Get(key); ok {
		t.Fatal("expected miss on empty cache")
	}
	c.Set(key, []string{"line a", "line b"})
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected hit after Set")
	}
	if len(got) != 2 || got[0] != "line a" {
		t.Errorf("got %v, want [line a line b]", got)
	}
}

func TestLogCacheInvalidatedByUpdatedAt(t *testing.T) {
	c := NewLogCache()
	k1 := logCacheKey{jobID: 1, updatedAt: time.Unix(100, 0)}
	k2 := logCacheKey{jobID: 1, updatedAt: time.Unix(200, 0)}
	c.Set(k1, []string{"old"})
	if _, ok := c.Get(k2); ok {
		t.Fatal("different updatedAt must miss")
	}
}

func TestLogCacheSharedAcrossSteps(t *testing.T) {
	c := NewLogCache()
	key := logCacheKey{jobID: 1, updatedAt: time.Unix(100, 0)}
	c.Set(key, []string{"full job log"})
	if _, ok := c.Get(key); !ok {
		t.Fatal("same job/updatedAt must hit regardless of current step")
	}
}

func TestLogCacheJobSeparation(t *testing.T) {
	c := NewLogCache()
	k1 := logCacheKey{jobID: 1, updatedAt: time.Unix(100, 0)}
	k2 := logCacheKey{jobID: 2, updatedAt: time.Unix(100, 0)}
	c.Set(k1, []string{"job 1"})
	if _, ok := c.Get(k2); ok {
		t.Fatal("different job must miss")
	}
}
