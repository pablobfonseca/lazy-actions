package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pablobfonseca/lazy-actions/internal/config"
	"github.com/pablobfonseca/lazy-actions/internal/gh"
)

// watchKey identifies a watched repo/workflow pair.
type watchKey struct {
	repo, workflow string
}

type runState struct {
	status     string // "in_progress", "waiting", "completed"
	conclusion string // "", "success", "failure", "cancelled"
}

type notification struct {
	title    string
	subtitle string
	message  string
	sound    string // empty = silent
	openURL  string // opened when the user clicks the notification
	actions  []notificationAction

	// interactive keeps the desktop notificli process alive for click
	// handling (-url/-actions); notificli blocks until dismissal whenever
	// either flag is present, so only failures opt in.
	interactive bool
}

// notificationAction is a button shown in the notification's dropdown.
// Clicking it opens the associated URL.
type notificationAction struct {
	label string
	url   string
}

// mobileChannel is an away-from-screen delivery channel (brrr.now push,
// Telegram, ...). Every configured channel receives a copy of the notification.
type mobileChannel interface {
	Configured() bool
	Send(notification)
}

// NotifyTracker detects run state transitions and sends desktop notifications.
// Workflows opted into mobile delivery additionally fan out to every configured
// mobile channel when a run completes.
type NotifyTracker struct {
	mu          sync.Mutex
	states      map[int64]runState // run ID -> last known state
	initialized map[watchKey]bool  // skip notifications on first fetch per watch
	notifierBin string             // path to notificli; empty falls back to osascript
	mobile      map[watchKey]bool  // workflows opted into mobile push notifications
	rules       map[watchKey]config.NotifyRules
	channels    []mobileChannel
	statePath   string // where the mobile selection is persisted
}

func NewNotifyTracker(cfg config.Config) *NotifyTracker {
	bin, _ := exec.LookPath("notificli")
	// First entry wins on duplicate repo/workflow pairs, matching the dedupe in tui.New.
	rules := make(map[watchKey]config.NotifyRules)
	for _, w := range cfg.Watches {
		for _, wf := range w.Workflows {
			key := watchKey{w.Repo, wf}
			if _, ok := rules[key]; !ok {
				rules[key] = w.Notify
			}
		}
	}
	nt := &NotifyTracker{
		states:      make(map[int64]runState),
		initialized: make(map[watchKey]bool),
		notifierBin: bin,
		mobile:      make(map[watchKey]bool),
		channels:    []mobileChannel{NewMobileNotifier(), NewTelegramNotifier()},
		statePath:   mobileStatePath(),
		rules:       rules,
	}
	nt.loadMobileState()
	return nt
}

// CheckAndNotify compares new runs against tracked state and sends notifications.
// Returns immediately; notifications are sent asynchronously.
func (nt *NotifyTracker) CheckAndNotify(repo, workflow string, runs []gh.RunInfo) {
	key := watchKey{repo, workflow}

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

		if !ruleAllows(nt.rules[key], newState, time.Now()) {
			continue
		}

		if n, ok := buildNotification(r, prev, tracked, newState); ok {
			go nt.send(n)
			if newState.status == "completed" && nt.mobile[key] {
				for _, ch := range nt.channels {
					if ch.Configured() {
						go ch.Send(n)
					}
				}
			}
		}
	}
}

// buildNotification returns the notification for a state transition, or false if no notification applies.
func buildNotification(r gh.RunInfo, prev runState, tracked bool, ns runState) (notification, bool) {
	subtitle := fmt.Sprintf("%s · %s", r.Repo, r.Run.HeadBranch)
	viewRun := notificationAction{"View Run", r.Run.HTMLURL}

	switch {
	case ns.status == "in_progress" && (!tracked || prev.status != "in_progress"):
		return notification{
			title:    "▶ " + r.Workflow,
			subtitle: subtitle,
			message:  "Started",
			openURL:  r.Run.HTMLURL,
			actions:  []notificationAction{viewRun},
		}, true

	case ns.status == "completed" && ns.conclusion == "success":
		return notification{
			title:    "✓ " + r.Workflow,
			subtitle: subtitle,
			message:  "Passed",
			openURL:  r.Run.HTMLURL,
			actions:  []notificationAction{viewRun},
		}, true

	case ns.status == "completed" && ns.conclusion == "failure":
		message := "Failed"
		openURL := r.Run.HTMLURL
		actions := []notificationAction{viewRun}
		if len(r.Jobs) > 0 {
			names := make([]string, 0, len(r.Jobs))
			for _, j := range r.Jobs {
				names = append(names, j.Name)
			}
			message = "Failed: " + strings.Join(names, ", ")
			openURL = r.Jobs[0].URL
			actions = append(actions, notificationAction{"View Failed Job", r.Jobs[0].URL})
		}
		return notification{
			title:       "✗ " + r.Workflow,
			subtitle:    subtitle,
			message:     message,
			sound:       "Basso",
			openURL:     openURL,
			actions:     actions,
			interactive: true,
		}, true

	case ns.status == "completed" && ns.conclusion == "cancelled":
		return notification{
			title:    "⊘ " + r.Workflow,
			subtitle: subtitle,
			message:  "Cancelled",
			openURL:  r.Run.HTMLURL,
			actions:  []notificationAction{viewRun},
		}, true
	}

	return notification{}, false
}

func ruleAllows(r config.NotifyRules, ns runState, now time.Time) bool {
	if r.InQuiet(now) {
		return false
	}
	if r.FailuresOnly() && !(ns.status == "completed" && ns.conclusion == "failure") {
		return false
	}
	return true
}

func (nt *NotifyTracker) send(n notification) {
	// Errors are intentionally ignored — notifications are best-effort.
	if nt.notifierBin == "" {
		exec.Command("osascript", "-e", osascriptScript(n)).Run()
		return
	}

	if !n.interactive {
		exec.Command(nt.notifierBin, notifiCliArgs(n)...).Run()
		return
	}

	// For interactive (failure) notifications notificli blocks until the user
	// clicks the notification, picks a dropdown action, or dismisses it, then
	// prints the choice to stdout. A click on the body ("default") opens -url
	// itself; dropdown actions we open here.
	out, err := exec.Command(nt.notifierBin, notifiCliArgs(n)...).Output()
	if err != nil {
		return
	}
	choice := strings.TrimSpace(string(out))
	for _, a := range n.actions {
		if choice == a.label {
			exec.Command("open", a.url).Run()
			return
		}
	}
}

func notifiCliArgs(n notification) []string {
	args := []string{"-title", n.title, "-message", n.message}
	if n.subtitle != "" {
		args = append(args, "-subtitle", n.subtitle)
	}
	if n.sound != "" {
		args = append(args, "-sound", n.sound)
	}
	if n.interactive {
		if n.openURL != "" {
			args = append(args, "-url", n.openURL)
		}
		if len(n.actions) > 0 {
			labels := make([]string, 0, len(n.actions))
			for _, a := range n.actions {
				labels = append(labels, a.label)
			}
			args = append(args, "-actions", strings.Join(labels, ","))
		}
	}
	return args
}

// ToggleMobile flips mobile push delivery for a workflow and persists the change.
func (nt *NotifyTracker) ToggleMobile(repo, workflow string) {
	key := watchKey{repo, workflow}

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
func (nt *NotifyTracker) IsMobileEnabled(repo, workflow string) bool {
	nt.mu.Lock()
	defer nt.mu.Unlock()
	return nt.mobile[watchKey{repo, workflow}]
}

// MobileConfigured reports whether at least one mobile channel can send.
func (nt *NotifyTracker) MobileConfigured() bool {
	for _, ch := range nt.channels {
		if ch.Configured() {
			return true
		}
	}
	return false
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
