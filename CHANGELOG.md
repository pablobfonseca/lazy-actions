# Changelog

All notable changes to this project are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

GitHub release notes are generated from commit subjects by goreleaser. This file
is the curated record, and it is where behavior changes worth knowing about
before you upgrade are called out.

## [Unreleased]

## [0.2.0] - 2026-09-04

### Added

- `lazyactions --version` (also `-v` and `version`) prints the build version,
  which now also appears in the TUI header. Release builds carry the tag;
  local `make build` uses `git describe`.
- Approve pending deployment reviews from the run list. `p` on a run held at
  `waiting` on an environment protection rule looks up its pending
  deployments, names the environments you can clear in a confirm modal, and
  approves them in one request. Approving only; rejecting stays in the browser.
  The lookup runs on the keypress rather than in the poll loop, to avoid
  spending rate limit on every waiting run every cycle.
- Runs awaiting approval are now visually distinct. They previously rendered
  exactly like an in-progress run, so nothing indicated the new key applied.

### Changed

- **A broken config file now fails startup instead of being ignored.** An
  unreadable or unparseable `config.yaml` previously fell back to
  auto-detection, so `lazyactions` started watching only the current
  directory's repo and exited 0, silently dropping the entire saved watch
  list. Read and parse failures now print and exit 1.
- `./config.yaml` is treated as a name this tool does not own. A document
  there without a top-level `watches` key, or that is not a mapping at all, is
  skipped and the search continues to
  `~/.config/gh-action-monitor/config.yaml`, so an unrelated project's
  `config.yaml` no longer breaks startup. The `~/.config` path is validated
  strictly in every case. One consequence worth knowing: a misspelled
  top-level key in `./config.yaml` (`watch:` for `watches:`) falls through
  silently rather than erroring, because at an unnamespaced path a typo and
  another tool's file are indistinguishable.
- `mobile_idle_minutes` is validated at load. Values above 1440 (24h) are
  rejected, where previously a large enough value overflowed the internal
  duration and suppressed mobile notifications permanently. Values written as
  floats are rejected instead of being truncated (`2.5` silently became `2`).
- README's configuration section rewritten. It stated that saved config always
  wins and auto-detection is never merged in, which is the opposite of the
  actual behavior; it also had the search order backwards.

### Fixed

- Telegram notifications with an action carrying an empty URL are no longer
  lost. Telegram rejects an entire message when an inline keyboard button has
  an empty URL, so "a notification without a link" became "no notification at
  all". Such buttons are now skipped, and the keyboard is omitted when none
  remain.
- Fix-with-Claude (`C`) no longer leaves its fetched job logs behind. The temp
  file is removed once the Claude session exits.
- Fix-with-Claude no longer crashes on a failed run with no jobs. That path
  produced a message with neither an error nor a run and was then
  dereferenced; it was unreachable in practice, guarded only at the call site.

### Security

- Fix-with-Claude treats GitHub-controlled text as untrusted input. The prompt
  handed to the child Claude session interpolated the repository, workflow,
  branch, run URL and job names directly into its instructions, and pointed it
  at fetched CI logs. Anyone who can name a branch or a job in a watched
  repository could therefore write text into that prompt. Those values now sit
  inside a delimited block marked as untrusted data, with an explicit
  instruction never to follow instructions found inside it. This is
  defense-in-depth, not a guarantee: it reduces the surface, and the log file
  contents remain attacker-influenced by nature. There was, and is, no command
  injection here; the prompt is a single argument and no shell is involved.

## [0.1.0] - 2026-08-19

First tagged release. Homebrew cask via `pablobfonseca/homebrew-tap`:

```sh
brew install pablobfonseca/tap/lazyactions
```

### Added

- Bubble Tea TUI watching GitHub Actions runs across multiple repositories,
  with a runs list, detail pane, and live log tail.
- Fullscreen log viewer with search and highlights, `##[group]` folding, and
  colorized output.
- Run actions: re-run failed jobs, cancel (with confirmation), download
  artifacts, copy run URL and commit SHA.
- Status and fuzzy filters over the runs list.
- Desktop notifications via notificli, with interactive actions on failures
  only, plus mobile channels via brrr.now and Telegram (inline URL buttons).
- Per-watch notification rules: `only: failures` and local-time `quiet`
  windows, honored for desktop and mobile alike.
- Away-from-screen detection: `mobile_idle_minutes` gates the mobile fan-out
  on macOS idle time, failing open on every error path.
- Zero-config auto-detection of the repository and workflows from the current
  directory.
- Fix-with-Claude: `C` on a failed run fetches the failed job logs and opens an
  interactive Claude session primed with the run's context.

[Unreleased]: https://github.com/pablobfonseca/lazy-actions/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/pablobfonseca/lazy-actions/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/pablobfonseca/lazy-actions/releases/tag/v0.1.0
