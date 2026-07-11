---
id: 58
slug: docket-status-orchestrator
title: docket-status orchestrator — collapse the status pass into one script call
status: implemented
priority: high
created: 2026-07-10
updated: 2026-07-11
depends_on: [53]
related: [53, 54, 55]
adrs: [12, 21]
spec: docs/superpowers/specs/2026-07-10-docket-status-orchestrator-design.md
plan: docs/superpowers/plans/2026-07-11-docket-status-orchestrator.md
results: docs/results/2026-07-11-docket-status-orchestrator-results.md
trivial: false
auto_groomable:
branch: feat/docket-status-orchestrator
pr: https://github.com/danielhanold/docket/pull/65
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-10-docket-status-orchestrator-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-docket-status-orchestrator-design.md) |
| Plan | [2026-07-11-docket-status-orchestrator.md](https://github.com/danielhanold/docket/blob/feat/docket-status-orchestrator/docs/superpowers/plans/2026-07-11-docket-status-orchestrator.md) |
| Results | [2026-07-11-docket-status-orchestrator-results.md](https://github.com/danielhanold/docket/blob/feat/docket-status-orchestrator/docs/results/2026-07-11-docket-status-orchestrator-results.md) |
| PR | [#65](https://github.com/danielhanold/docket/pull/65) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0021](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0021-pipeline-script-authored-mechanical-commits.md) |
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

- 2026-07-11 — Reconciled at claim. `depends_on: [53]` satisfied (#53 done, archived; the slimmed
  `docket-status` SKILL.md body is live on origin/docket). All shared scripts the orchestrator
  sequences exist (`archive-change.sh`, `render-change-links.sh`, `terminal-publish.sh`,
  `cleanup-feature-branch.sh`, `board-checks.sh`, `render-board.sh`, `sync-integration-branch.sh`,
  `github-mirror.sh`, `docket-config.sh`); no `docket-status.sh` present yet. Related #54/#55 still
  `proposed` — no overlap (they slim other skill bodies; this adds the orchestrator + rewrites the
  status skill). Test harness confirmed at `tests/` (script-test pattern, e.g. `test_render_board.sh`).
  Scope unchanged; no drops or additions.
