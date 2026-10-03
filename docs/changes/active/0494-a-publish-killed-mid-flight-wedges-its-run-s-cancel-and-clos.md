---
id: 494
slug: 'a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos'
title: 'A publish killed mid-flight wedges its run''s cancel and closeout'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-10-02'
updated: '2026-10-03'
depends_on: []
stacked_on:
related: [444, 491, 492]
discovered_from: [491]
adrs: [118, 124, 132, 133, 134]
spec: 'docs/superpowers/specs/2026-10-03-a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-03T08:55:47Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-03-a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-03-a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos-design.md) |
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md), [ADR-0132](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0132-worktree-admission-is-a-supervisor-held-kernel-lock.md), [ADR-0133](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0133-the-run-key-is-the-run-tracker-s-only-handle-gate-starts-car.md), [ADR-0134](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0134-a-dead-supervisor-s-suite-counts-as-gone-only-when-its-proce.md) |
<!-- docket:artifacts:end -->

## Why

Found while grooming 0491 (2026-10-02). Grooming on 2026-10-03 confirmed it by reading the code on main (1fc28e872, after 0491 and 0492 merged). Nothing has reproduced it, and no run record on this machine shows it.

Before `pr.publish` or `workspace.publish` does its remote work for a tracked run, it writes an `admitted` entry in the owning run's mutation journal. The same process rewrites the entry as `completed` or `uncertain` when the work returns. If the process dies in between, the entry stays `admitted` for good. That covers Ctrl-C, a coordinator interrupt, and a crash: the CLI installs no signal handler for these operations.

- **Nothing settles it.** 0444's retry match settles only `uncertain` entries.
- **The success closeout never finishes.** `run.verdict` prints `completion-unaccounted` on every call, and the run stays `completing`.
- **Cancel never finishes.** It stays `cancellation-pending`, and `run.start --resume` refuses the change.

An `uncertain` entry with no successful identical retry wedges the same way. Once cancel or the closeout fences the run, docket refuses every new publish, so that retry can never happen. 0444 left both cases to "human investigation", but no docket operation clears an entry. The only remedy is hand-editing the run record under `.git/docket/run-tracker/<key>/`.

## What changes

- **The publisher holds a lock.**
  - `pr.publish` and `workspace.publish` take a per-entry kernel lock before writing their `admitted` entry, and release it only after writing the outcome.
  - The operating system frees the lock when the process dies, so a free lock proves the publisher is gone.
  - A lock failure never refuses the publish; the entry then behaves exactly as today.
  - This is the pattern `live.lock` and the worktree lock (ADR-0132) already use.
- **An entry blocks only while its publisher may still be running.** Cancel, the death guardian, resume's check, and the success closeout share one rule:
  - `completed` is accounted;
  - `uncertain`, or `admitted` with a free lock, is accounted with a new informational finding, `mutation-abandoned:<op>`;
  - `admitted` with a held lock, a missing lock file, a probe error, or no lock still blocks as `mutation-pending`, as today.
- **The journal stays truthful.** Cancel and the keyed closeout rewrite a dead publisher's `admitted` entry as `uncertain`, so a later verified identical retry still settles it (`mutation-settled`).
- **Why it is safe.** `run-complete` already proves the branch and the PR live, and both publishes converge on a retry, so a resumed run adopts whatever landed.
- **Failure posture.** No new refusal, no new block, and no new signal or GitHub call. The finding never blocks.
- **Records and docs.**
  - A new ADR, with Update notes on ADR-0124 and ADR-0118.
  - A glossary entry and a run-tracker concept paragraph.
- **Tests.**
  - Killed, live, unprovable, retried, and no-retry cases.
  - A real SIGKILL test on macOS.
  - Mutation checks.
  - 0444's no-retry tests are rewritten to the new rule.

## Out of scope

- A Ctrl-C handler in the CLI. The lock covers every kind of death.
- Live Git or GitHub checks inside cancel, closeout, or resume.
- A force-clear command or flag.
- Rewriting existing journal entries, or deleting lock files.
- Metadata transactions (completed at admission), and `finalize.publish` (not journaled).
- The never-started test run and the missing run root, both fixed by 0491.

## Reconcile log

### 2026-10-03

2026-10-03 — Reconciled against origin/main 1fc28e872, the same commit the spec was re-checked at; no code drift since grooming. Confirmed the traced symbols still exist (admitWorkflowMutation, settleUncertainPublications, accountCompletionMutations, verifyTerminalRunQuiescence, process.TryExclusiveLock, probeFlock). Related 0444/0491/0492 are done and change nothing in the journal logic. Scope unchanged.
