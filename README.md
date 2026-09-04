# lazy-actions

A lazygit-style terminal UI for monitoring GitHub Actions workflow runs across multiple repositories.

Watch every workflow you care about in a single pane of glass: running jobs with live step progress, recent failures with inline log tails, fullscreen log viewer with search — all driven by the keyboard.

## Screenshots

![Overview: run list with detail pane](docs/overview.png)

<!-- future shots go in docs/, e.g. docs/logviewer.png for the fullscreen log viewer -->

## Features

- **At-a-glance overview** of every watched workflow run across every repo, grouped `Active` / `Recent`, sorted by recency.
- **Passive detail pane** follows the cursor: metadata, jobs, and the last ~10 lines of the active-or-failed step.
- **Fullscreen log viewer** (`enter`) with `/` search, `n/N` navigation, and colorized GitHub Actions output (`##[warning]`, `##[error]`, `##[group]`, etc.).
- **Actions**: re-run failed jobs (`F`), cancel (`x`), download artifacts (`d`), copy URL (`y`) / SHA (`Y`).
- **Filters**: status (`a`/`f`/`A`) and fuzzy text (`/` over repo/workflow/branch).
- **Adaptive polling**: 10 s while anything is running (or finished in the last 60 s), 180 s when idle. Rate-limit aware. `r` forces an immediate refresh; successful `F`/`x` auto-refresh the affected repo.
- **macOS notifications** on run state transitions (started / passed / failed / cancelled).

## Prerequisites

- Go 1.25+
- The [GitHub CLI](https://cli.github.com/) (`gh`), authenticated:
  ```
  gh auth login
  ```
  `lazy-actions` shells out to `gh api` for every request, so whatever authorization `gh` has is what the TUI uses.
- macOS for desktop notifications (the rest works everywhere Go runs).

## Install

### Homebrew

```sh
brew install pablobfonseca/tap/lazyactions
```

### From source

```sh
go install github.com/pablobfonseca/lazy-actions@latest
# or from a checkout:
make build      # produces ./bin/lazyactions
make install    # copies it to ~/.local/bin/lazyactions
```

Check what you have with `lazyactions --version`. Homebrew and `make build`
report the tag; `go install` reports `dev`, since it does not set the build
flag that stamps the version in.

See [CHANGELOG.md](CHANGELOG.md) for what changed between releases.

## Configuration

`lazy-actions` resolves what to watch from two places:

1. **Saved config**: `./config.yaml` in the current directory (legacy), falling back to `~/.config/gh-action-monitor/config.yaml`.
2. **Auto-detect** from the current directory: `git remote get-url origin` → `owner/repo`, plus every `.github/workflows/*.{yml,yaml}` file. Both are required; a repo with no workflow files is not detectable.

Both sources are always merged, saved watches first:

- Saved config and CWD repo → the detected entry is appended to the saved watches. If the detected repo is already in the config, the saved entry wins and is used untouched (your `workflows` and `notify` rules are kept, nothing is appended).
- Saved config, CWD is not a GitHub repo → the saved watches are used as-is.
- No config file, but CWD is a GitHub repo → the detected entry is the whole config.
- No config file and no CWD repo → startup fails with both errors.
- Config file present but unreadable, unparseable, or invalid → startup fails with that error, so a typo *inside* your watch list can never silently drop it. A missing or misspelled top-level `watches` key in the legacy `./config.yaml` is the one exception, described below.

`./config.yaml` is not a name this tool owns, so a document there that is not a mapping with a `watches` key (another tool's settings, a bare list, an empty file) is skipped and the search continues; one that *has* a `watches` key is validated strictly. `~/.config/gh-action-monitor/config.yaml` is always validated strictly, a missing `watches` key included.

### Saved config format

```yaml
mobile_idle_minutes: 5 # optional: mobile pushes only after N min without keyboard/mouse input
watches:
  - repo: "your-org/your-repo"
    workflows:
      - "ci.yml"
      - "deploy.yml"
    notify:
      only: failures # optional: "all" (default) or "failures"
      quiet: # optional: local-time windows with no notifications
        - "22:00-08:00"
  - repo: "your-org/other-repo"
    workflows:
      - "build.yml"
```

Workflow names are the file names under `.github/workflows/` in each repo.

Per-watch `notify` rules are optional. `only: failures` delivers only failure notifications (started/success/cancelled are suppressed, on desktop and mobile alike). `quiet` windows are `HH:MM-HH:MM` in local time, may wrap midnight, and silence all notifications for that watch while active; transitions that happen during a quiet window are dropped, not queued.

`mobile_idle_minutes` gates the mobile channels (brrr.now, Telegram) on away-from-screen detection: pushes are sent only when macOS reports at least that many minutes without keyboard or mouse input (`HIDIdleTime`). `0` or omitted sends them regardless. Desktop notifications are never gated. The value must be a whole number of minutes between `0` and `1440`; anything else fails at startup.

## Keybindings

| Key            | Context     | Action                                  |
| -------------- | ----------- | --------------------------------------- |
| `j` / `k` / ↓↑ | normal      | Move cursor                             |
| `g` / `G`      | normal      | First / last run                        |
| `enter`        | normal      | Open fullscreen log viewer              |
| `o`            | normal      | Open run URL in browser                 |
| `r`            | normal      | Refresh all repos now                   |
| `F`            | normal      | Re-run failed jobs (confirm)            |
| `x`            | normal      | Cancel run (confirm)                    |
| `p`            | normal      | Approve pending deployment (confirm)    |
| `d`            | normal      | Download artifacts (prompts for path)   |
| `y` / `Y`      | normal      | Copy run URL / commit SHA               |
| `n`            | normal      | Toggle mobile notifications for workflow |
| `C`            | normal      | Fix failed run with Claude (opens `claude` primed with the failure logs) |
| `a` / `f` / `A`| normal      | Filter: active / failed / all           |
| `/`            | normal      | Fuzzy filter prompt                     |
| `esc`          | normal      | Clear filter                            |
| `?`            | normal      | Toggle help modal                       |
| `q` / `ctrl-c` | any         | Quit                                    |
| `/`            | log viewer  | Search                                  |
| `n` / `N`      | log viewer  | Next / previous match                   |
| `g` / `G`      | log viewer  | Top / bottom                            |
| `esc`          | modal       | Close modal                             |

## Notifications

**Desktop** — every run state transition (started / passed / failed / cancelled) fires a native
notification via [NotifiCLI](https://github.com/pablobfonseca/notificli) when it is on `PATH`,
falling back to `osascript`. Failure notifications carry **View Run** and **View Failed Job**
actions that open the relevant GitHub page; started / passed / cancelled notifications are
fire-and-forget, so they leave no process waiting on a dismissal.

**Mobile** — completed runs additionally fan out to any configured away-from-screen channel:

| Channel                         | Environment variables                      |
| ------------------------------- | ------------------------------------------ |
| [brrr.now](https://brrr.now)    | `BRRR_TOKEN`                               |
| Telegram                        | `TELEGRAM_BOT_TOKEN` + `TELEGRAM_CHAT_ID`  |

Variables are read from `./.env` or `~/.config/gh-action-monitor/.env` at startup (existing
environment variables are never overridden).

Mobile delivery is **opt-in per workflow**: press `n` on a selected run to toggle it. Opted-in
workflows show a 🔔 next to the workflow name, and the selection is persisted to
`~/.config/gh-action-monitor/mobile-notify.json` so it survives restarts.

## Development

```sh
make check      # fmt + vet + test
make run        # run against local config
make help       # list available targets
```

### Releasing

Push a tag and GitHub Actions does the rest:

```sh
git tag v0.2.0 && git push origin v0.2.0
```

`.github/workflows/release.yml` runs the checks on macOS, then goreleaser builds
both darwin architectures, publishes the release, and updates the cask in
`pablobfonseca/homebrew-tap`. Move the `[Unreleased]` section of
[CHANGELOG.md](CHANGELOG.md) under the new version before tagging.

Publishing the cask writes to a second repository, which a workflow's built-in
token cannot reach, so the repo needs a `HOMEBREW_TAP_TOKEN` secret. Use a
fine-grained PAT scoped to `pablobfonseca/homebrew-tap` alone with
`Contents: Read and write`; a classic `repo`-scope token also works but grants write
to every repo you own. Set it before the first tag: without it the cask step fails,
and whether the release itself survives that failure is untested.

`make release` does the same thing from a local checkout, using your `gh` token
for both. It is the fallback for when Actions is unavailable; tagging is the
normal path.

## Project Layout

```
main.go        entry point + resolveConfig fallback
dotenv.go      .env loader
internal/
├── config/     YAML config loading + repo auto-detect
├── gh/         GitHub API client + caches (reads in api.go, writes in actions.go, logs in logs.go)
├── clip/       pbcopy wrapper
├── notify/     desktop (NotifiCLI/osascript) + mobile (brrr.now, Telegram) notifications
└── tui/        Bubble Tea components
    ├── model.go                    root model + mode routing
    ├── overview.go / overview_test runs list + cursor
    ├── detail.go                   metadata + jobs + log tail
    ├── logviewer.go                fullscreen logs + search
    ├── help.go / confirm.go        modals
    ├── prompt.go                   inline one-line prompt
    ├── filter.go / filter_test     status + fuzzy matching
    ├── colorize.go                 log line coloring
    ├── keys.go                     centralized keymap
    ├── loggroups.go                ##[group] folding
    └── styles.go                   lipgloss styles
```

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles).

## Roadmap

- **Phase 3** — Local dry-runs via [`act`](https://github.com/nektos/act).
