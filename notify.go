package main

import (
	"fmt"
	"os/exec"
	"sync"
)

type runState struct {
	status     string // "in_progress", "waiting", "completed"
	conclusion string // "", "success", "failure", "cancelled"
}

type notification struct {
	title   string
	message string
	sound   bool
}

// NotifyTracker detects run state transitions and sends desktop notifications.
type NotifyTracker struct {
	mu          sync.Mutex
	states      map[int64]runState // run ID -> last known state
	initialized map[watchKey]bool  // skip notifications on first fetch per watch
}

func NewNotifyTracker() *NotifyTracker {
	return &NotifyTracker{
		states:      make(map[int64]runState),
		initialized: make(map[watchKey]bool),
	}
}

// CheckAndNotify compares new runs against tracked state and sends notifications.
// Returns immediately; notifications are sent asynchronously.
func (nt *NotifyTracker) CheckAndNotify(key watchKey, runs []RunInfo) {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	firstFetch := !nt.initialized[key]
	nt.initialized[key] = true

	for _, r := range runs {
		id := r.Run.ID
		newState := runState{
			status:     r.Run.Status,
			conclusion: r.Run.Conclusion,
		}

		prev, tracked := nt.states[id]
		nt.states[id] = newState

		if firstFetch || !tracked {
			continue
		}

		if prev.status == newState.status && prev.conclusion == newState.conclusion {
			continue
		}

		label := fmt.Sprintf("%s — %s", r.Workflow, r.Run.HeadBranch)

		switch {
		case newState.status == "in_progress" && prev.status != "in_progress":
			go sendNotification(notification{
				title:   "▶ Started",
				message: label,
			})
		case newState.status == "completed" && newState.conclusion == "success":
			go sendNotification(notification{
				title:   "✓ Passed",
				message: label,
			})
		case newState.status == "completed" && newState.conclusion == "failure":
			go sendNotification(notification{
				title:   "✗ Failed",
				message: label,
				sound:   true,
			})
		case newState.status == "completed" && newState.conclusion == "cancelled":
			go sendNotification(notification{
				title:   "⊘ Cancelled",
				message: label,
			})
		}
	}
}

func sendNotification(n notification) {
	script := fmt.Sprintf(`display notification %q with title %q`, n.message, n.title)
	if n.sound {
		script += ` sound name "Basso"`
	}
	// Errors are intentionally ignored — notifications are best-effort
	exec.Command("osascript", "-e", script).Run()
}
