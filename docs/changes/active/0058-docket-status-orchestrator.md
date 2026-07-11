---
id: 58
slug: docket-status-orchestrator
title: docket-status orchestrator — collapse the status pass into one script call
status: in-progress
priority: high
created: 2026-07-10
updated: 2026-07-11
depends_on: [53]
related: [53, 54, 55]
adrs: [12]
spec: docs/superpowers/specs/2026-07-10-docket-status-orchestrator-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/docket-status-orchestrator
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-10-docket-status-orchestrator-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-docket-status-orchestrator-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

A `docket-status` run takes several minutes and costs ~$1 per run in Cursor. The cost is
dominated by **model round-trips**: every Bash call re-sends 20–30k tokens of context, a no-op
run is ~10–15 turns, and each swept change adds ~6–8 more. Changes #0053–#0055 slim the
tokens-per-turn; this change attacks the orthogonal turn-count dimension. ADR-0012 already names
the motivation — every status step except harvest-learnings and `blocked_by:` review is
mechanical, and every sweep sub-step is already a script; the model is an expensive glue layer
invoking them one turn at a time.

## What changes

- New `scripts/docket-status.sh` (+ contract): the full deterministic pipeline — config +
  bootstrap verdict, worktree sync, board render/commit/push per surface, **batched** sweep
  detection (one `gh` call for all `implemented` changes), sweep execution chaining the existing
  shared close-out scripts with the sweep's log-and-continue posture, `board-checks.sh`,
  integration sync — in ONE invocation, emitting one compact machine-parseable report.
- `--board-only` fast mode (sync + render + commit/push) for the interactive "just show me the
  board" case.
- One new ADR (relates to ADR-0012): deterministic pipelines may author formulaic templated
  commit messages and mutate state along an already-blessed script sequence; judgment-bearing
  prose stays model-authored.
- `docket-status` SKILL.md rewritten around the orchestrator: invoke, surface report, then
  judgment-only follow-ups (harvest-learnings, `blocked_by:` review, mint write-backs).
- Expected: no-op run ~2–3 turns (from 10–15); sweep run ~4–5 + harvest (from ~20–35+); cost and
  wall clock drop proportionally on any model/harness.

## Out of scope

- Per-harness model/effort re-pinning (`agents:` config).
- `docket-finalize-change`'s flow and the merge gate (#0054 owns its slimming).
- Convention/lifecycle semantics, board format, `render-board.sh` / `board-checks.sh` internals.
- Turn-count work on other skills (follow-up may reuse the pattern).

## Open questions

- Batched detection: one GraphQL aliased query vs. N `gh pr view` calls inside the script —
  decide at plan time.

## Reconcile log
