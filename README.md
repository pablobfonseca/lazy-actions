# lazy-actions

A lazygit-style terminal UI for monitoring GitHub Actions workflow runs across multiple repositories.

Watch every workflow you care about in a single pane of glass: running jobs with live step progress, recent failures with inline log tails, fullscreen log viewer with search — all driven by the keyboard.

## Features

- **At-a-glance overview** of every watched workflow run across every repo, grouped `Active` / `Recent`, sorted by recency.
- **Passive detail pane** follows the cursor: metadata, jobs, and the last ~10 lines of the active-or-failed step.
- **Fullscreen log viewer** (`enter`) with `/` search, `n/N` navigation, and colorized GitHub Actions output (`##[warning]`, `##[error]`, `##[group]`, etc.).
- **Filters**: status (`a`/`f`/`A`) and fuzzy text (`/` over repo/workflow/branch).
- **Macros polling**: adaptive 10 s when anything is active, 180 s when idle. Rate-limit aware.
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
├── gh/         GitHub API client + caches
├── notify/     macOS notifications
└── tui/        Bubble Tea components
    ├── model.go                    root model + mode routing
    ├── overview.go / overview_test runs list + cursor
    ├── detail.go                   metadata + jobs + log tail
    ├── logviewer.go                fullscreen logs + search
    ├── help.go / confirm.go        modals
    ├── filter.go / filter_test     status + fuzzy matching
    ├── colorize.go                 log line coloring
    ├── keys.go                     centralized keymap
    └── styles.go                   lipgloss styles
```

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles).

## Roadmap

- **Phase 2** — Actions: rerun (with confirmation), cancel, download artifacts, copy URL/SHA.
- **Phase 3** — Local dry-runs via [`act`](https://github.com/nektos/act).
