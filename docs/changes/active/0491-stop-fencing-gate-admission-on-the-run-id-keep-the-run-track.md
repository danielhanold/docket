---
id: 491
slug: 'stop-fencing-gate-admission-on-the-run-id-keep-the-run-track'
title: 'Retire the run id; the run key becomes the run tracker''s only handle'
status: 'in-progress'
priority: 'critical'
type: 'refactor'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: [490]
stacked_on:
related: [375, 422, 435, 437, 441, 443, 463, 467, 488, 489, 492, 493, 494]
discovered_from: []
adrs: [111, 118, 124, 128, 129, 132]
spec: 'docs/superpowers/specs/2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track-design.md'
plan: 'docs/superpowers/plans/2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track.md'
results: 'docs/results/2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/stop-fencing-gate-admission-on-the-run-id-keep-the-run-track'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-02T23:06:02Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track-design.md) |
| Plan | [2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track.md](https://github.com/danielhanold/docket/blob/refactor/stop-fencing-gate-admission-on-the-run-id-keep-the-run-track/docs/superpowers/plans/2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track.md) |
| Results | [2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track-results.md](https://github.com/danielhanold/docket/blob/refactor/stop-fencing-gate-admission-on-the-run-id-keep-the-run-track/docs/results/2026-10-02-stop-fencing-gate-admission-on-the-run-id-keep-the-run-track-results.md) |
| ADRs | [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md), [ADR-0128](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0128-resume-arms-mint-an-arm-time-epoch-that-run-cancel-can-cance.md), [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md), [ADR-0132](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md) |
<!-- docket:artifacts:end -->

## Why

The run tracker hands out three tokens per run: a run key, a run id, and a run context. The coordinator threads all three. The run id adds nothing the key doesn't give. The two are minted together and stay one-to-one, and a retry keeps both. Yet the id is what gate starts are fenced on.

`runLaunchGate` refuses with `stale-run-id` whenever the run's worktree is unbound or different, or the `--repo-dir` spelling differs, not only when a resume superseded the run. Every one of those refusals says "superseded by a resume", which sends agents looking for a resume that never happened. The token-threading bugs in 0463, 0467, and 0477 came from the same chain. Changes 0489 and 0490 removed the takeover and slot fences; this change finishes the job.

Two stuck states ride along.

- **A never-launched drive.** A `gate.drive.start` can be killed between writing its drive record and launching the suite. The leftover record makes `run.verdict` stop a successful run for good (`completion-unaccounted`, 0490 review finding F3). The only way out is `run.cancel`, which records the success as cancelled. The same record turns an early-stopped run's retry into `continuation-unverified`.
- **A missing run root.** It does the same to cancel, resume, and the verdict: `resolution-unresolved` forever.

## What changes

- **Retire the run id.** The run key becomes the run tracker's only handle:
  - `run.start` prints `run-started <key> <run-context>`;
  - cancel is `run.cancel --key <key> --reason <why>`;
  - `--run-id` is removed from `gate.drive.start` and `agent.enter`;
  - run records stop storing an id.
- **Delete the run check on gate starts** (`runLaunchGate`). A gate start is refused only by the worktree lock (`worktree-busy`).
- **The keyed `run.verdict` closes a proven never-launched drive.** It marks the drive HALTED `launch-abandoned` on both its success and run-incomplete paths, under the same per-drive lock cancel uses, and it stops nothing.
- **A missing run root counts as never launched,** for cancel and the verdict alike.
- **Names:**
  - `stale-run-id` becomes `run-superseded`.
  - `unknown-run-id` is retired; an unknown key reports `run-not-found`.
  - The non-cancel meaning of `run-id-mismatch` becomes `run-record-conflict`.
- **`agent.enter --run-key`** alone drives the dormant Codex lifecycle linkage. Nothing documents passing it.
- **`run.start`** declares `process-control`.
- **Prose and decisions:**
  - update the CLAUDE.md/AGENTS.md run-tracker block, the skills, the glossary, and the Codex clause;
  - fold in 0443's wording fix (killed 2026-10-02): the run-tracker block says an operation id such as `run.start` is not a command; look it up in `docket capabilities --json` and run that entry's `argv`;
  - record a new ADR, with Update notes on ADR-0124, ADR-0128, and ADR-0132;
  - amend ADR-0129's rename rows in place.

Accepted loss: an agent left over from a cancelled run could still start a suite in that worktree. The worktree lock keeps it to one suite, and a repeat `run.cancel` on the old key stops it.

## Out of scope

- Retiring the run tracker itself, or its attribution and retry model.
- The workflow-mutation fence beyond the rename, and the publish journal wedge (0494).
- Process-tree teardown (0492), and finalize's automatic relaunch with its reserved-relaunch accounting (0493 retires it).
- Launches that cannot be resolved either way: `resolution-unresolved` stays fail-closed.
- Deleting the dormant `agent.enter` lifecycle linkage.

## Reconcile log

### 2026-10-02

2026-10-02 — Reconciled against main at 756fea9fe, the exact commit the spec traced (after 0489 and 0490 landed; 0490 is done). No other change is in-progress or implemented, so nothing concurrent touches reconcile.go (0493 is still proposed). 0422 and 0443 were killed in the 2026-10-02 backlog review, as the spec already records. Scope, spec, and relations unchanged.
