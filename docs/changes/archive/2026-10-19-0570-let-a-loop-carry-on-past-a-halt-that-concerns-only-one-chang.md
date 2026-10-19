---
id: 570
slug: 'let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang'
title: 'Let a loop carry on past a halt that concerns only one change'
status: 'done'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-10-18'
updated: '2026-10-19'
depends_on: []
stacked_on:
related: [559, 560]
discovered_from: [560]
adrs: [115, 148, 153, 154]
spec: 'docs/superpowers/specs/2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-design.md'
plan: 'docs/superpowers/plans/2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang.md'
results: 'docs/results/2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang'
pr: 'https://github.com/danielhanold/docket/pull/435'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-design.md](../../superpowers/specs/2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-design.md) |
| Spec (merged) | [2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-design.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/specs/2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-design.md) |
| Plan | [2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang.md](../../superpowers/plans/2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang.md) |
| Results | [2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-results.md](../../results/2026-10-19-let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang-results.md) |
| ADRs | [ADR-0115](../../adrs/0115-outer-run-gate-retry-budget-is-a-counted-config-snapshotted.md), [ADR-0148](../../adrs/0148-every-keyed-run-done-verdict-drives-the-successful-run-close.md), [ADR-0153](../../adrs/0153-a-hold-keeps-a-groomed-change-out-of-autonomous-build-select.md), [ADR-0154](../../adrs/0154-a-loop-carries-on-past-a-queue-picked-halt-up-to-a-halts-in.md) |
<!-- docket:artifacts:end -->

## Why

Today every `halted` disposition stops the driver, in both `docket-implement-next` and `docket-finalize-change`. Some halts concern only the change at hand: a design the reconcile pass finds invalidated, a build that cannot get its suite green, a results-artifact change that turns out to need code. Others would halt every remaining change the same way: broken configuration, an agent that cannot be started, a failed preflight. The driver cannot tell the two apart, so one change's halt ends an overnight drain.

#560 removes the most common planned cause: a change that needs a human is put on hold and no loop picks it. The other per-change halts still end the loop.

## What changes

Give a loop a way to carry on past a halt that concerns one change, and a backstop that stops it when halts keep coming.

- A tracked implement-next loop continues after a run that halted on a change the loop picked from the queue (the halt is already recorded as `## Run halted` and shown as "run halted — needs you"), while the count of halts in a row stays under the new `run.max_consecutive_halts` setting (default 2).
- The run tracker keeps that count per repository. Only halts on loop-picked changes add to it; a halt on a named change or a resume never touches it; any clean build resets it.
- At the limit the loop stops and the session asks the person whether to reset the count with a new `docket run reset-halts` command; the agent never resets it on its own.
- A new verdict line, `run-next`, tells a loop it may start its next run; named runs, resumes, retries, continuations, untracked runs and finalize print exactly today's lines.
- Docs state that docket supports one implement loop and one finalize loop at a time.

## Out of scope

Keeping a change that needs a human away from the loop: #560's hold does that. How a run behaves when it reaches a human step: #559 settled that.

Carrying a finalize loop past a blocked change: finalize keeps stopping on every block (a separate change). Typed halt reasons or classifying halt points. Any gate that prevents two loops of the same stage from running at once.

## Reconcile log

### 2026-10-19

2026-10-19 — Reconciled against main at 9ec9eb2b9 (#560 hold landed; #559 done). Spec groomed today; every cited symbol (ChangeClaimRequest, changeClaimReceipt, persistRunVerdict, acquireResumeLock, PruneRunTrackerRecords, the resumeAttributed method, runTrackerSchemaVersion 4, bindingSchemaVersion 1) still exists as described, and neither run.max_consecutive_halts nor reset-halts exists yet. No scope change.

## Build evidence

<!-- docket:build-evidence:start -->
command:  go run ./cmd/docket development test
result:   green
head_sha: a15eb5631e270dca47664c8ddab5217407bca0ef
ran_at:   2026-10-19T18:19:58Z
<!-- docket:build-evidence:end -->
