---
name: lazy-actions-workflow
description: Orchestrates code changes to lazy-actions with a develop-then-verify agent pipeline. Use for ANY feature, bugfix, refactor, or enhancement request in this repo ("add X", "fix X", "improve X", "support X"), and for follow-ups — "re-run", "update", "redo", "fix what QA found", "improve the previous result", or partial reruns ("just re-verify", "just fix the notification part"). Simple questions about the code do not need this; answer those directly.
---

# lazy-actions workflow orchestrator

Coordinates the `feature-dev` and `qa-verifier` agents (defined in `.claude/agents/`) for code changes to this repo.

**Execution mode: sub-agent.** Two agents in a strict pipeline; results pass through the orchestrator via return values + workspace files. No agent team.

## Phase 0: context check

Determine the run mode before anything else:

- `_workspace/` exists AND the request is a partial fix/re-verify → **partial rerun**: invoke only the needed agent, pointing it at the existing artifacts.
- `_workspace/` exists AND the request is new work → **new run**: move `_workspace/` to `_workspace_prev/` (overwrite an older `_workspace_prev/`), then proceed.
- No `_workspace/` → **initial run**.

## Phase 1: scope

Restate the request as a verifiable goal (what observable behavior proves it done). Identify likely files from the architecture map in the `lazy-actions-dev` skill. If the request implies multiple independent modules, split into tasks so QA can run incrementally per module. Trivial one-liners (typo, string tweak) skip the pipeline: edit directly, run the verification commands yourself, done.

## Phase 2: implement

For each task, invoke feature-dev:

```
Agent(subagent_type: "feature-dev", model: "opus",
      prompt: <verifiable goal, expected files, workspace artifact path,
               prior QA findings path if rework>)
```

Independent tasks may run in parallel (`run_in_background: true`); tasks touching the same files run sequentially.

## Phase 3: verify (incremental)

After **each** task completes (not once at the end), invoke qa-verifier:

```
Agent(subagent_type: "qa-verifier", model: "opus",
      prompt: <the task's goal, feature-dev artifact path, changed files>)
```

- PASS → next task.
- FAIL → send blockers back to feature-dev (one rework pass), then re-verify. Second consecutive FAIL on the same task → stop and report to the user with both artifacts; do not loop further.

## Phase 4: report

Summarize for the user: what changed (files), what was verified and how (command results from the QA artifact), open warns/nits, anything skipped. Never claim done on a FAIL or an unverified change.

## Data protocol

- Intermediate artifacts: `_workspace/{NN}_{agent}_{task-slug}.md`, NN ordered by pipeline position (01 dev, 02 qa, 03 dev-rework, …).
- `_workspace/` is gitignored and preserved after the run for audit; only code changes land in the repo proper.
- Agent return messages carry summaries/verdicts; files carry detail.

## Error handling

- Agent dies or returns unusable output → retry once with the same prompt; on second failure report the gap to the user, do not fabricate the missing result.
- feature-dev reports blocked (out-of-scope failure, ambiguous requirement it can't default) → surface to the user rather than widening scope.
- Conflicting findings between agents → present both with attribution; don't discard either.

## Test scenarios

**Normal flow:** "Add a Slack notification channel" → Phase 1 scopes it to a new `slack.go` following the notifier pattern + `notify.go` fan-out wiring → feature-dev implements with tests → qa-verifier runs the four commands and cross-checks the `notification` struct against all four senders → PASS → report.

**Error flow:** "Fix the rate-limit retry" → feature-dev changes `ui.go` scheduling → qa-verifier FAILs: `go vet` flags an unused variable and the retry ignores `resetAt` zero value → blockers go back to feature-dev → rework → re-verify PASS → report includes the rework cycle.
