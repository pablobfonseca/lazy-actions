---
name: lazy-actions-workflow
description: Orchestrates code changes to lazy-actions with a develop-then-verify agent pipeline. Use for ANY feature, bugfix, refactor, or enhancement request in this repo ("add X", "fix X", "improve X", "support X"), and for follow-ups — "re-run", "update", "redo", "fix what QA found", "improve the previous result", or partial reruns ("just re-verify", "just fix the notification part", "add the changelog entry"). Simple questions about the code do not need this; answer those directly.
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

Every feature-dev prompt asks for a proposed `CHANGELOG.md` entry, returned as text in the report rather than written to the file. The `lazy-actions-dev` skill carries the test for whether one is warranted and how to phrase it; ask for the judgment too, so "no entry, internal refactor" is an answer you can check rather than a silence you cannot distinguish from an omission. You write the file yourself in Phase 4, because `CHANGELOG.md` collides across parallel agents and staging a shared file sweeps another agent's uncommitted lines into the wrong commit.

## Phase 3: verify (incremental)

After **each** task completes (not once at the end), invoke qa-verifier:

```
Agent(subagent_type: "qa-verifier", model: "opus",
      prompt: <the task's goal, feature-dev artifact path, changed files>)
```

- PASS → next task.
- FAIL → send blockers back to feature-dev (one rework pass), then re-verify. Second consecutive FAIL on the same task → stop and report to the user with both artifacts; do not loop further.

Ask qa-verifier to judge the proposed changelog entry against the diff it just verified: whether a warranted entry is missing, whether the entry claims more than the code does, and whether an accepted residual it found is reflected. QA reads the diff line by line, so it is the only participant positioned to catch an entry that describes the intent rather than the result. A wrong entry is a warning, not a blocker, unless it overstates a security fix or omits a behavior change a user would hit on upgrade.

## Phase 4: changelog

Apply the verified entries to `CHANGELOG.md` under `## [Unreleased]`, merging what the tasks returned into the existing sections rather than appending a block per task: a reader wants one Changed list, not one per agent. Skip only when every task judged no entry warranted, and say so in the report so the omission is visible. Then re-run `gofmt -l . && go vet ./... && go build ./... && go test ./...`; a changelog edit cannot break them, but committing without having run them once against the final tree is how an unrelated break ships.

## Phase 5: report

Summarize for the user: what changed (files), what was verified and how (command results from the QA artifact), the changelog entries applied or why none were, open warns/nits, anything skipped. Never claim done on a FAIL or an unverified change.

## Data protocol

- Intermediate artifacts: `_workspace/{NN}_{agent}_{task-slug}.md`, NN ordered by pipeline position (01 dev, 02 qa, 03 dev-rework, …).
- `_workspace/` is gitignored and preserved after the run for audit; only code changes land in the repo proper.
- Agent return messages carry summaries/verdicts; files carry detail.

## Error handling

- Agent dies or returns unusable output → retry once with the same prompt; on second failure report the gap to the user, do not fabricate the missing result.
- feature-dev reports blocked (out-of-scope failure, ambiguous requirement it can't default) → surface to the user rather than widening scope.
- Conflicting findings between agents → present both with attribution; don't discard either.

## Test scenarios

**Normal flow:** "Add a Slack notification channel" → Phase 1 scopes it to a new `slack.go` following the notifier pattern + `notify.go` fan-out wiring → feature-dev implements with tests and returns a proposed Added entry → qa-verifier runs the four commands, cross-checks the `notification` struct against all four senders, and confirms the entry matches the diff → PASS → Phase 4 merges the entry under `## [Unreleased]` → report.

**No-entry flow:** "Extract the run-status switch into a helper" → feature-dev returns "no entry warranted: internal refactor, identical behavior" → qa-verifier confirms the diff is behavior-preserving and agrees → Phase 4 skips `CHANGELOG.md` and the report says so, so the omission is visible rather than silent.

**Changelog-only rework:** QA finds the proposed entry claims a fix is complete while the diff leaves a documented residual → warning, not a blocker → the orchestrator rewrites the entry to name the residual before applying it; feature-dev is not re-invoked, since no code needs to change.

**Error flow:** "Fix the rate-limit retry" → feature-dev changes `ui.go` scheduling → qa-verifier FAILs: `go vet` flags an unused variable and the retry ignores `resetAt` zero value → blockers go back to feature-dev → rework → re-verify PASS → report includes the rework cycle.
