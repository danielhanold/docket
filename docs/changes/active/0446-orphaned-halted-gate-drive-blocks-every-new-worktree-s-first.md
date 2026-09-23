---
id: 446
slug: 'orphaned-halted-gate-drive-blocks-every-new-worktree-s-first'
title: 'Orphaned halted gate drive blocks every new worktree''s first gate admission'
status: 'in-progress'
priority: 'critical'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-23'
depends_on: []
stacked_on:
related: [368, 375, 428, 435, 437, 439, 441, 444]
discovered_from: [444]
adrs: [87, 95, 118, 120, 124]
spec: 'docs/superpowers/specs/2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first-design.md'
plan: 'docs/superpowers/plans/2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/orphaned-halted-gate-drive-blocks-every-new-worktree-s-first'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-23T17:46:30Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first-design.md) |
| Plan | [2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first.md](https://github.com/danielhanold/docket/blob/fix/orphaned-halted-gate-drive-blocks-every-new-worktree-s-first/docs/superpowers/plans/2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first.md) |
| ADRs | [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0120](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0120-historical-gate-drive-schemas-are-assessed-never-executed.md), [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md) |
<!-- docket:artifacts:end -->

## Why

Docket's own historical gate bookkeeping can prevent explicitly named independent changes from starting a gate, cancelling, resuming, or entering finalize. Every fresh worktree's first admission scans the repository-wide drive registry, and an old record with deleted paths or unreadable evidence can veto the start. Cancellation and successful closeout similarly treat unresolvable historical drive linkage as an obligation of the run being closed. Current worktree-owner lookup can also select a cancelled, never-superseded predecessor ahead of a fresh run on the same path, wrongly fencing the live run.

The original worktree-admission design called for checking history for that canonical worktree. The implementation expanded uncertainty into a repository-wide blocker. The reported change-368 HALTED drive and change-444 cancellation failure are examples of this broader authority-boundary defect; a special case for one halt cause would leave the problem intact. The linked spec traces the existing implementation, prior fixes, and architectural decisions and defines end-to-end progress and safety requirements.

On 2026-09-23 the broader named-workflow scope was split into three critical changes, built in succession: this change (gate history and run bookkeeping), 448 (a named start skips unrelated maintenance), and 449 (scoped metadata validation and a tolerant board).

## What changes

- Establish relevance to the requested canonical worktree or run before treating historical uncertainty as a blocker. Keep historical cleanup diagnostic; unrelated, unmatchable, or obsolete records must not veto new drives.
- Reuse current slots, scopes, epochs, reservation tokens, and participant links to account for the target run's outstanding work. Preserve failures for positively identified current obligations, including missing or corrupt required records.
- Reconcile proven-finished incumbents through the existing admission/release machinery so leftover slot bookkeeping does not require a manual stop or second start. Preserve exact-token checks, launch claims, epoch fences, and admission-before-charging.
- Resolve the current worktree owner through existing replacement relationships instead of directory order; keep old explicit identities revoked and prevent predecessors from masking replacements. Preserve sufficient durable completion facts after optional scratch evidence disappears.
- Cover cancellation/resume, success closeout into finalize, raw/scoped/scopeless gates, and same-worktree recovery, with broken unrelated gate records still present, plus paired refusals for defects in the target's own runtime records.
- Record the changed ADR-0118/ADR-0120 clauses during implementation.
- After the implementation lands and is installed, verify change 444's existing recovery workflow while preserving its uncommitted work. Grooming itself does not resume that run.

## Out of scope

New orchestration systems, commands, force flags, persistent stores or schemas, retirement receipts, background cleanup, leases/TTLs, age cutoffs, retry allowances, storage relocation, general garbage collection, and a second liveness implementation. No automatic cancellation to make room, no blanket trust in HALTED, no dropping positively owned pending work, no manual gate-record editing, and no treating change status as process-death proof. Automatic finalize queue ordering and driver stop/continue behavior are excluded. Named-start maintenance (change 448) and metadata validation/board rendering (change 449) are separate follow-on changes. Shared infrastructure failures, real dependencies, and actual ownership conflicts remain valid blockers. Publication-journal reconciliation remains change 444's work. Implementation and implementation planning are separate from grooming.

## Reconcile log

### 2026-09-23

Reconciled against origin/main 442770e1 (change 0414 merged; no gate-admission, run-cancel, or gate-drive code changed since grooming on 2026-09-23). Related 444 is blocked; 448/449 wait on this change. Scope, spec, and relations stand unchanged.
