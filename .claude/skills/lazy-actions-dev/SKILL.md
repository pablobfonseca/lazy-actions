---
name: lazy-actions-dev
description: Architecture map, coding conventions, and verification commands for the lazy-actions Go codebase. MUST be read before writing, modifying, or reviewing any Go code in this repo — features, bugfixes, refactors, and test work all start here. Also use when asked how the codebase is structured or how to verify a change.
---

# lazy-actions development guide

lazy-actions is a Bubble Tea TUI that watches GitHub Actions workflow runs and sends desktop/mobile notifications. Composed packages under `internal/`, module `github.com/pablobfonseca/lazy-actions`, Go 1.25.

## Architecture map

Root (`package main`) is startup only; everything else lives under `internal/`.

| Path | Owns | Key entry points |
|------|------|------------------|
| `main.go` | Startup, config resolution | `main`, `resolveConfig` (saved config + CWD autodetect always merged; detected entry appended unless its repo is already saved) |
| `dotenv.go` | `.env` loading | `loadDotEnv` (`./.env` then `~/.config/gh-action-monitor/.env`; never overrides set vars) |
| `main_test.go` | `resolveConfig` behavior | builds real temp git repos + config files |
| `internal/config/config.go` | Saved config schema + loading | `Config`, `WatchEntry`, `Load` (cwd then `~/.config/gh-action-monitor/`) |
| `internal/config/autodetect.go` | Zero-config detection | `DetectFromCWD` — parses git remote origin (SSH/HTTPS/ssh://) + globs `.github/workflows/*.y{a,}ml` |
| `internal/gh/api.go` | GitHub API reads | `RunInfo`, `FetchRepoRuns` (REST via `gh api`), branch existence (GraphQL), rate-limit reset detection |
| `internal/gh/cache.go` | Fetch-path caches | `BranchCache`, `JobCache` |
| `internal/gh/actions.go` | GitHub API writes | re-run failed jobs, cancel run, download artifacts |
| `internal/gh/logs.go` | Run log retrieval + cache | `LogCache`, `FetchJobLogs`, `FetchLogTail` (detail pane + log viewer) |
| `internal/tui/model.go` | Root Bubble Tea model (Elm architecture) | `New`, `Update`, `View`, `handleKey`, mode routing, tick/fetch commands, rate-limit retry scheduling |
| `internal/tui/overview.go` | Runs list + cursor | `overviewModel`, `Selected`, `renderLine` (🔔 marker for mobile-enabled workflows) |
| `internal/tui/detail.go` | Right pane | metadata, jobs, live log tail |
| `internal/tui/logviewer.go` | Fullscreen log viewer | search + highlights, match cycling |
| `internal/tui/loggroups.go` | `##[group]` folding | group parsing, fold/unfold, error-group auto-expand |
| `internal/tui/colorize.go` | Log line coloring | timestamps, commands, errors, warnings, notices |
| `internal/tui/filter.go` | Filtering | status filter + fuzzy matching |
| `internal/tui/confirm.go`, `prompt.go`, `help.go` | Modals | confirm dialogs, inline one-line prompt, help overlay |
| `internal/tui/keys.go` | Centralized keymap | help sections + bindings |
| `internal/tui/styles.go` | lipgloss styles | `AdaptiveColor` pairs (Light/Dark) resolved once at startup |
| `internal/notify/notify.go` | Notification decisions + desktop delivery | `NotifyTracker.CheckAndNotify(repo, workflow, runs)`, `buildNotification` (state-transition logic), notificli args, `ToggleMobile`/`IsMobileEnabled` persistence |
| `internal/notify/mobile.go` | brrr.now mobile channel | `MobileNotifier` (`BRRR_TOKEN`) |
| `internal/notify/telegram.go` | Telegram channel | `TelegramNotifier` (`TELEGRAM_BOT_TOKEN` + `TELEGRAM_CHAT_ID`) |
| `internal/clip/clip.go` | Clipboard | `Copy` (pbcopy wrapper) |

Data flow: `resolveConfig` → `tui.New` → per-repo `repoFetchCmd` (async tea.Cmd) → `gh.FetchRepoRuns` → `Update` stores runs → `View` renders; `NotifyTracker.CheckAndNotify` diffs run states and fans out to configured notifiers.

## Conventions

- Package boundaries: `internal/gh` never imports `internal/tui`; the TUI consumes `gh.RunInfo` and calls `notify` through `NotifyTracker`. New code goes in an existing file when it fits that file's responsibility; a new file only for a genuinely new concern (e.g. a new notification channel gets its own file in `internal/notify`, like `telegram.go`).
- No comments except constraints the code can't express. Existing code is nearly comment-free; keep it that way.
- Notification channels follow the notifier pattern: `NewXNotifier()` reading env vars, `Configured() bool`, `Send(n notification)`. `NotifyTracker.send` fans out to whichever notifiers are configured. Add new channels by that shape.
- External processes (`gh`, `notificli`, `osascript`) are invoked via `os/exec`; there is no GitHub SDK dependency. Keep it that way — `gh` handles auth.
- Errors at startup print to stderr and exit; errors during operation degrade gracefully (skip the repo, show rate-limit state) rather than crashing the TUI.
- Caching: check `BranchCache`/`JobCache` before adding API calls in the fetch path; GitHub rate limits are the scarce resource.

## Changelog

`CHANGELOG.md` is the curated record. goreleaser generates GitHub release notes
from commit subjects (`changelog: use: github-native`), so the file exists to carry
what generated notes cannot: which changes alter behavior someone already depends on.
That is its whole job, and it is why the test below is about the reader, not about
whether you touched code.

**An entry is warranted when a user could notice the change without reading the diff:**

| Warrants an entry | Section | Does not warrant one |
|---|---|---|
| New or changed keybinding, flag, or command | Added / Changed | Internal refactor with identical behavior |
| Config schema, validation, or resolution change | Changed | Test-only changes, new test coverage |
| A failure mode a user hit, now fixed | Fixed | Comment moves, formatting, renames |
| Notification content, timing, or delivery | Changed / Fixed | Dependency bumps with no visible effect |
| Anything altering the trust boundary or an injection surface | Security | Workspace artifacts, harness edits |

When it is genuinely borderline, write the entry. A reader skipping one line costs
nothing; a silent behavior change costs them a debugging session.

**Behavior changes get the consequence, not the mechanism.** A reader scanning before
upgrading needs to know what will break, in their terms. "resolveConfig now uses
errors.Is on a sentinel" tells them nothing. "A broken config file now fails startup
instead of being ignored, where it previously dropped your whole watch list and exited
0" tells them whether to care. Name the old behavior, since that is the one they have.

**Entries go under `## [Unreleased]`**, in the Keep a Changelog sections already in the
file (Added, Changed, Fixed, Security). Do not invent sections, do not add a version
heading; tagging a release is a human decision and moving `[Unreleased]` under a new
version happens then. Prose style follows the repo's: no em-dashes, no filler.

**Accepted residuals belong in the entry.** If a fix leaves a known gap (q11 left a
typo'd key in the shared config path falling through silently), say so where the reader
will see it. A changelog that only lists wins teaches readers not to trust it.

**Parallel work: the orchestrator owns this file.** `CHANGELOG.md` is touched by nearly
every change, so concurrent agents collide on it, and a staged shared file silently
sweeps another agent's uncommitted lines into the wrong commit. When agents run in
parallel, each one returns its proposed entry as text in its report and the orchestrator
writes the file. A single agent working alone edits it directly.

## Testing conventions

- Table-driven tests; isolation via `t.TempDir()`, `t.Setenv()`, `t.Chdir()` (see `internal/config/autodetect_test.go`, `main_test.go` — they build real temp git repos).
- Testable without mocks: config parsing, autodetect, `internal/notify`'s `buildNotification` state transitions, formatting helpers. Live API (`internal/gh` fetchers) and TUI rendering are not unit-tested; they get boundary review instead.

## Verification commands

Run all four. CI runs only on tag push (`.github/workflows/release.yml` gates the release on macOS), so for ordinary commits these local checks are the only gate:

```bash
go test ./...
gofmt -l .          # must print nothing
go vet ./...
go build ./...
```

To exercise the TUI manually: `go build -o lazyactions . && ./lazyactions` inside a repo with workflows (needs `gh` authenticated). It runs in the alt screen; `q` quits.

## Gotchas

- `.env` in the repo root holds real tokens — never print or commit its contents.
- `.gitignore` excludes `config.yaml`, `.env`, and built binaries (`gh-action-monitor`, `lazyactions`); don't commit binaries after test builds.
- Config and state paths keep the old `gh-action-monitor` name (`~/.config/gh-action-monitor/{config.yaml,.env,mobile-notify.json}`) even though the module is `github.com/pablobfonseca/lazy-actions`; do not "fix" them, they are the user's live setup. Derive the GitHub slug from `git remote get-url origin`, never from go.mod.
