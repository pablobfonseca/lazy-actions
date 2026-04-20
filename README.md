# lazy-actions

A lazygit-style terminal UI for monitoring GitHub Actions workflow runs across multiple repositories.

Watch every workflow you care about in a single pane of glass: running jobs with live step progress, recent failures with inline log tails, fullscreen log viewer with search — all driven by the keyboard.

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

```sh
go install github.com/upscopeio/lazy-actions@latest
# or from a checkout:
make build      # produces ./bin/lazy-actions
make install    # to $GOBIN
```

## Configuration

Create `config.yaml` in the current directory or at `~/.config/lazy-actions/config.yaml`:

```yaml
watches:
  - repo: "your-org/your-repo"
    workflows:
      - "ci.yml"
      - "deploy.yml"
  - repo: "your-org/other-repo"
    workflows:
      - "build.yml"
```

Workflow names are the file names under `.github/workflows/` in each repo.

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
| `d`            | normal      | Download artifacts (prompts for path)   |
| `y` / `Y`      | normal      | Copy run URL / commit SHA               |
| `a` / `f` / `A`| normal      | Filter: active / failed / all           |
| `/`            | normal      | Fuzzy filter prompt                     |
| `esc`          | normal      | Clear filter                            |
| `?`            | normal      | Toggle help modal                       |
| `q` / `ctrl-c` | any         | Quit                                    |
| `/`            | log viewer  | Search                                  |
| `n` / `N`      | log viewer  | Next / previous match                   |
| `g` / `G`      | log viewer  | Top / bottom                            |
| `esc`          | modal       | Close modal                             |

## Development

```sh
make check      # fmt + vet + test
make run        # run against local config
make help       # list available targets
```

## Project Layout

```
main.go
internal/
├── config/     YAML config loading
├── gh/         GitHub API client + caches (reads in api.go, writes in actions.go)
├── clip/       pbcopy wrapper
├── notify/     macOS notifications
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
    └── styles.go                   lipgloss styles
```

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles).

## Roadmap

- **Phase 3** — Local dry-runs via [`act`](https://github.com/nektos/act).
