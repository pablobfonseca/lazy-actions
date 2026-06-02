package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type runState struct {
	status     string // "in_progress", "waiting", "completed"
	conclusion string // "", "success", "failure", "cancelled"
}

type notification struct {
	title    string
	subtitle string
	message  string
	sound    string // empty = silent
	group    string // dedup key — new notifications with same group replace old ones
	openURL  string // opened when the user clicks the notification
}

// NotifyTracker detects run state transitions and sends desktop notifications.
// Workflows opted into mobile delivery additionally trigger a brrr.now push when
// a run completes.
type NotifyTracker struct {
	mu          sync.Mutex
	states      map[int64]runState // run ID -> last known state
	initialized map[watchKey]bool  // skip notifications on first fetch per watch
	groups      map[string]bool    // groups we've sent — used by ClearAll on shutdown
	notifierBin string             // path to terminal-notifier; empty falls back to osascript
	mobile      map[watchKey]bool  // workflows opted into mobile push notifications
	mobileNotif *MobileNotifier
	statePath   string // where the mobile selection is persisted
}

func NewNotifyTracker() *NotifyTracker {
	bin, _ := exec.LookPath("terminal-notifier")
	nt := &NotifyTracker{
		states:      make(map[int64]runState),
		initialized: make(map[watchKey]bool),
		groups:      make(map[string]bool),
		notifierBin: bin,
		mobile:      make(map[watchKey]bool),
		mobileNotif: NewMobileNotifier(),
		statePath:   mobileStatePath(),
	}
	nt.loadMobileState()
	return nt
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

		if firstFetch {
			continue
		}

		if tracked && prev.status == newState.status && prev.conclusion == newState.conclusion {
			continue
		}

		if n, ok := buildNotification(r, prev, tracked, newState); ok {
			go nt.send(n)
			if newState.status == "completed" && nt.mobile[key] {
				go nt.mobileNotif.Send(n)
			}
		}
	}
}

// buildNotification returns the notification for a state transition, or false if no notification applies.
func buildNotification(r RunInfo, prev runState, tracked bool, ns runState) (notification, bool) {
	subtitle := fmt.Sprintf("%s · %s", r.Repo, r.Run.HeadBranch)
	// Grouping by run ID means the final state (passed/failed) replaces the
	// "started" toast if it's still on screen — only one toast per run lingers.
	group := fmt.Sprintf("ghmon-%d", r.Run.ID)

	switch {
	case ns.status == "in_progress" && (!tracked || prev.status != "in_progress"):
		return notification{
			title:    "▶ " + r.Workflow,
			subtitle: subtitle,
			message:  "Started",
			group:    group,
			openURL:  r.Run.HTMLURL,
		}, true

	case ns.status == "completed" && ns.conclusion == "success":
		return notification{
			title:    "✓ " + r.Workflow,
			subtitle: subtitle,
			message:  "Passed",
			group:    group,
			openURL:  r.Run.HTMLURL,
		}, true

	case ns.status == "completed" && ns.conclusion == "failure":
		message := "Failed"
		openURL := r.Run.HTMLURL
		if len(r.Jobs) > 0 {
			names := make([]string, 0, len(r.Jobs))
			for _, j := range r.Jobs {
				names = append(names, j.Name)
			}
			message = "Failed: " + strings.Join(names, ", ")
			openURL = r.Jobs[0].URL
		}
		return notification{
			title:    "✗ " + r.Workflow,
			subtitle: subtitle,
			message:  message,
			sound:    "Basso",
			group:    group,
			openURL:  openURL,
		}, true

	case ns.status == "completed" && ns.conclusion == "cancelled":
		return notification{
			title:    "⊘ " + r.Workflow,
			subtitle: subtitle,
			message:  "Cancelled",
			group:    group,
			openURL:  r.Run.HTMLURL,
		}, true
	}

	return notification{}, false
}

func (nt *NotifyTracker) send(n notification) {
	// Errors are intentionally ignored — notifications are best-effort.
	if nt.notifierBin == "" {
		exec.Command("osascript", "-e", osascriptScript(n)).Run()
		return
	}

	if n.group != "" {
		nt.mu.Lock()
		nt.groups[n.group] = true
		nt.mu.Unlock()
		// Explicit remove before send: -group only replaces still-visible toasts,
		// so this also clears the prior status from notification-center history.
		exec.Command(nt.notifierBin, "-remove", n.group).Run()
	}
	exec.Command(nt.notifierBin, terminalNotifierArgs(n)...).Run()
}

// ClearAll removes every notification this tracker has sent. Called on shutdown.
func (nt *NotifyTracker) ClearAll() {
	if nt.notifierBin == "" {
		return
	}
	nt.mu.Lock()
	groups := make([]string, 0, len(nt.groups))
	for g := range nt.groups {
		groups = append(groups, g)
	}
	nt.mu.Unlock()

	var wg sync.WaitGroup
	for _, g := range groups {
		wg.Add(1)
		go func(group string) {
			defer wg.Done()
			exec.Command(nt.notifierBin, "-remove", group).Run()
		}(g)
	}
	wg.Wait()
}

func terminalNotifierArgs(n notification) []string {
	args := []string{"-title", n.title, "-message", n.message}
	if n.subtitle != "" {
		args = append(args, "-subtitle", n.subtitle)
	}
	if n.sound != "" {
		args = append(args, "-sound", n.sound)
	}
	if n.group != "" {
		args = append(args, "-group", n.group)
	}
	if n.openURL != "" {
		args = append(args, "-open", n.openURL)
	}
	return args
}

// ToggleMobile flips mobile push delivery for a workflow and persists the change.
func (nt *NotifyTracker) ToggleMobile(key watchKey) {
	nt.mu.Lock()
	if nt.mobile[key] {
		delete(nt.mobile, key)
	} else {
		nt.mobile[key] = true
	}
	nt.mu.Unlock()
	nt.saveMobileState()
}

// IsMobileEnabled reports whether a workflow is opted into mobile push delivery.
func (nt *NotifyTracker) IsMobileEnabled(key watchKey) bool {
	nt.mu.Lock()
	defer nt.mu.Unlock()
	return nt.mobile[key]
}

// MobileConfigured reports whether a brrr.now token is available to send with.
func (nt *NotifyTracker) MobileConfigured() bool {
	return nt.mobileNotif.Configured()
}

// mobileSelection is the persisted form of a single opted-in workflow.
type mobileSelection struct {
	Repo     string `json:"repo"`
	Workflow string `json:"workflow"`
}

func mobileStatePath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gh-action-monitor", "mobile-notify.json")
}

func (nt *NotifyTracker) loadMobileState() {
	data, err := os.ReadFile(nt.statePath)
	if err != nil {
		return
	}
	var sel []mobileSelection
	if json.Unmarshal(data, &sel) != nil {
		return
	}
	for _, s := range sel {
		nt.mobile[watchKey{s.Repo, s.Workflow}] = true
	}
}

func (nt *NotifyTracker) saveMobileState() {
	nt.mu.Lock()
	sel := make([]mobileSelection, 0, len(nt.mobile))
	for k := range nt.mobile {
		sel = append(sel, mobileSelection{k.repo, k.workflow})
	}
	nt.mu.Unlock()

	data, err := json.MarshalIndent(sel, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(nt.statePath), 0o755) != nil {
		return
	}
	os.WriteFile(nt.statePath, data, 0o644)
}

func osascriptScript(n notification) string {
	script := fmt.Sprintf(`display notification %q with title %q`, n.message, n.title)
	if n.subtitle != "" {
		script += fmt.Sprintf(` subtitle %q`, n.subtitle)
	}
	if n.sound != "" {
		script += fmt.Sprintf(` sound name %q`, n.sound)
	}
	return script
}
