# lazy-actions

Go Bubble Tea TUI that watches GitHub Actions runs and sends desktop/mobile notifications. See `.claude/skills/lazy-actions-dev/SKILL.md` for the architecture map and verification commands.

## Harness: lazy-actions development

**Goal:** every code change goes through implement → independent verify. CI runs only on tag push (the release gate), so local checks remain the only gate for ordinary commits.

**Trigger:** for any feature, bugfix, or refactor request, use the `lazy-actions-workflow` skill. Simple questions about the code can be answered directly.

**Change history:**
| Date | Change | Target | Reason |
|------|--------|--------|--------|
| 2026-08-18 | Initial harness: feature-dev + qa-verifier agents, dev guide + orchestrator skills | all | - |
| 2026-09-03 | CHANGELOG.md entries required for user-visible changes: convention in dev skill, dev proposes / QA gates / orchestrator writes | skills + both agents | goreleaser's github-native release notes cannot flag behavior changes; q11 shipped an exit-code change with no user-facing record |
| 2026-09-03 | Fixed harness drift: qa-verifier boundary paths still named pre-`internal/` files, and it claimed the TUI cannot be exercised headlessly | agents/qa-verifier.md | agents rediscovered real paths each run; tmux drives the TUI fine and caught a live q19 guard |
| 2026-09-04 | Corrected the "no CI" premise in the harness goal and the dev skill: release.yml gates tags on macOS | CLAUDE.md + skills/lazy-actions-dev | the claim became false for tagged commits; local checks still gate everything else |
