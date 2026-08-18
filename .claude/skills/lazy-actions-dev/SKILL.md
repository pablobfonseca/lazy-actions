---
name: lazy-actions-dev
description: Architecture map, coding conventions, and verification commands for the lazy-actions Go codebase. MUST be read before writing, modifying, or reviewing any Go code in this repo — features, bugfixes, refactors, and test work all start here. Also use when asked how the codebase is structured or how to verify a change.
---

# lazy-actions development guide

lazy-actions is a Bubble Tea TUI that watches GitHub Actions workflow runs and sends desktop/mobile notifications. Single flat `package main`, module `github.com/jdelia/gh-action-monitor`, Go 1.25.

## Architecture map

| File | Owns | Key entry points |
|------|------|------------------|
| `main.go` | Startup, config resolution | `main`, `resolveConfig` (saved config wins; autodetect is fallback-only, never merged) |
| `config.go` | Saved config schema + loading | `Config`, `WatchEntry`, `loadConfig` (cwd then `~/.config/gh-action-monitor/`) |
| `autodetect.go` | Zero-config detection | `DetectFromCWD` — parses git remote origin (SSH/HTTPS/ssh://) + globs `.github/workflows/*.y{a,}ml` |
| `github.go` | All GitHub API access | `fetchRepoRuns` (REST via `gh api`), `batchBranchExists` (GraphQL), `BranchCache`, `JobCache`, rate-limit reset detection |
| `ui.go` | Bubble Tea model (Elm architecture) | `newModel`, `Update`, `View`, tick/fetch commands, rate-limit retry scheduling |
| `notify.go` | Notification decisions + desktop delivery | `NotifyTracker.CheckAndNotify`, `buildNotification` (state-transition logic), notificli args, mobile-toggle persistence |
| `mobile.go` | ntfy-style mobile channel + `.env` loading | `MobileNotifier`, `loadDotEnv` |
| `telegram.go` | Telegram channel | `TelegramNotifier` (env-configured: `Configured()` gates sending) |

Data flow: `resolveConfig` → `newModel` → per-repo `repoFetchCmd` (async tea.Cmd) → `fetchRepoRuns` → `Update` stores runs → `View` renders; `NotifyTracker.CheckAndNotify` diffs run states and fans out to configured notifiers.

## Conventions

- Flat package: new code goes in an existing file when it fits that file's responsibility; a new file only for a genuinely new concern (e.g. a new notification channel gets its own file, like `telegram.go`).
- No comments except constraints the code can't express. Existing code is nearly comment-free; keep it that way.
- Notification channels follow the notifier pattern: `NewXNotifier()` reading env vars, `Configured() bool`, `Send(n notification)`. `NotifyTracker.send` fans out to whichever notifiers are configured. Add new channels by that shape.
- External processes (`gh`, `notificli`, `osascript`) are invoked via `os/exec`; there is no GitHub SDK dependency. Keep it that way — `gh` handles auth.
- Errors at startup print to stderr and exit; errors during operation degrade gracefully (skip the repo, show rate-limit state) rather than crashing the TUI.
- Caching: check `BranchCache`/`JobCache` before adding API calls in the fetch path; GitHub rate limits are the scarce resource.

## Testing conventions

- Table-driven tests; isolation via `t.TempDir()`, `t.Setenv()`, `t.Chdir()` (see `autodetect_test.go`, `main_test.go` — they build real temp git repos).
- Testable without mocks: config parsing, autodetect, `buildNotification` state transitions, formatting helpers. Live API (`github.go` fetchers) and TUI rendering are not unit-tested; they get boundary review instead.

## Verification commands

Run all four; there is no CI, so local checks are the only gate:

```bash
go test ./...
gofmt -l .          # must print nothing
go vet ./...
go build -o /dev/null .
```

To exercise the TUI manually: `go build -o lazyactions . && ./lazyactions` inside a repo with workflows (needs `gh` authenticated). It runs in the alt screen; `q` quits.

## Gotchas

- `.env` in the repo root holds real tokens — never print or commit its contents.
- `.gitignore` excludes `config.yaml`, `.env`, and built binaries (`gh-action-monitor`, `lazyactions`); don't commit binaries after test builds.
- Module path (`jdelia/gh-action-monitor`) predates the repo name (`pablobfonseca/lazy-actions`); derive the GitHub slug from `git remote get-url origin`, never from go.mod.
