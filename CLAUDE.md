# lazy-actions

Go Bubble Tea TUI that watches GitHub Actions runs and sends desktop/mobile notifications. See `.claude/skills/lazy-actions-dev/SKILL.md` for the architecture map and verification commands.

## Harness: lazy-actions development

**Goal:** every code change goes through implement → independent verify, since this repo has no CI.

**Trigger:** for any feature, bugfix, or refactor request, use the `lazy-actions-workflow` skill. Simple questions about the code can be answered directly.

**Change history:**
| Date | Change | Target | Reason |
|------|--------|--------|--------|
| 2026-08-18 | Initial harness: feature-dev + qa-verifier agents, dev guide + orchestrator skills | all | - |
