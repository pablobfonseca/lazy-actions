package notify

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/upscopeio/lazy-actions/internal/gh"
)

type runState struct {
	status     string
	conclusion string
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
	initialized map[string]bool    // "repo/workflow" -> has seen first fetch
}

func NewNotifyTracker() *NotifyTracker {
	return &NotifyTracker{
		states:      make(map[int64]runState),
		initialized: make(map[string]bool),
	}
}

// CheckAndNotify compares new runs against tracked state and sends notifications.
// Returns immediately; notifications are sent asynchronously.
func (nt *NotifyTracker) CheckAndNotify(repo, workflow string, runs []gh.RunInfo) {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	key := repo + "/" + workflow
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

		if firstFetch {
			continue
		}

		if tracked && prev.status == newState.status && prev.conclusion == newState.conclusion {
			continue
		}

		label := fmt.Sprintf("%s — %s / %s", r.Repo, r.Workflow, r.Run.HeadBranch)

		switch {
		case newState.status == "in_progress" && (!tracked || prev.status != "in_progress"):
			go sendNotification(notification{title: "▶ Started", message: label})
		case newState.status == "completed" && newState.conclusion == "success":
			go sendNotification(notification{title: "✓ Passed", message: label})
		case newState.status == "completed" && newState.conclusion == "failure":
			go sendNotification(notification{title: "✗ Failed", message: label, sound: true})
		case newState.status == "completed" && newState.conclusion == "cancelled":
			go sendNotification(notification{title: "⊘ Cancelled", message: label})
		}
	}
}

func sendNotification(n notification) {
	script := fmt.Sprintf(`display notification %q with title %q`, n.message, n.title)
	if n.sound {
		script += ` sound name "Basso"`
	}
	exec.Command("osascript", "-e", script).Run()
}
