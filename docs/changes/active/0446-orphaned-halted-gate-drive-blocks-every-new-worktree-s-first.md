---
id: 446
slug: 'orphaned-halted-gate-drive-blocks-every-new-worktree-s-first'
title: 'Orphaned halted gate drive blocks every new worktree''s first gate admission'
status: 'proposed'
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
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-23-orphaned-halted-gate-drive-blocks-every-new-worktree-s-first-design.md) |
| ADRs | [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md), [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0120](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0120-historical-gate-drive-schemas-are-assessed-never-executed.md), [ADR-0124](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0124-successful-run-ownership-closeout-extends-the-run-epoch-life.md) |
<!-- docket:artifacts:end -->

## Why

Docket's own historical bookkeeping can prevent explicitly named independent changes from implementing, cancelling, resuming, or finalizing. Every fresh worktree's first admission scans the repository-wide drive registry, and an old record with deleted paths or unreadable evidence can veto the start. Cancellation and successful closeout similarly treat unresolvable historical drive linkage as an obligation of the run being closed. Current worktree-owner lookup can also select a superseded predecessor before its replacement.

The original worktree-admission design called for checking history for that canonical worktree. The implementation expanded uncertainty into a repository-wide blocker. The reported change-368 HALTED drive and change-444 cancellation failure are examples of this broader authority-boundary defect; a special case for one halt cause would leave the problem intact. The linked spec traces the existing implementation, prior fixes, and architectural decisions and defines end-to-end progress and safety requirements.

Named implementation also runs maintenance before selecting its requested change, so another change's failed closeout or reclaim can halt it before a drive starts. Metadata transactions then require the entire corpus to have no errors: one malformed unrelated change can block both a new claim and a final archive. The generated board assumes the same globally valid state. These are part of this change's progress requirement, not exclusions hidden behind a gate-only fix.

## What changes

- Establish relevance to the requested canonical worktree or run before treating historical uncertainty as a blocker. Keep historical cleanup diagnostic; unrelated, unmatchable, or obsolete records must not veto new drives.
- Reuse current slots, scopes, epochs, reservation tokens, and participant links to account for the target run's outstanding work. Preserve failures for positively identified current obligations, including missing or corrupt required records.
- Reconcile proven-finished incumbents through the existing admission/release machinery so leftover slot bookkeeping does not require a manual stop or second start. Preserve exact-token checks, launch claims, epoch fences, and admission-before-charging.
- Resolve the current worktree owner through existing replacement relationships instead of directory order; keep old explicit identities revoked and prevent predecessors from masking replacements. Preserve sufficient durable completion facts after optional scratch evidence disappears.
- For an explicitly named change, go from repository preparation to its authoritative context and existing claim/resume checks without running unrelated maintenance. Keep deliberate maintenance and no-ID implementation behavior; add no preflight flags or cleanup queue.
- Extend the existing metadata transaction's validation with a bounded in-memory subject set. Preserve/report unchanged errors in unrelated records while rejecting required-record defects, identity conflicts affecting the target, new errors, illegal evolution, and unapproved source changes. Keep complete corpus reads and exact-version/lease protections.
- Carry that rule through named workflow reads and writes, final archive/closeout, and canonical generated views. An unrenderable unrelated record remains visible as needing repair; it cannot stop the selected change's board update.
- Cover complete implementation and finalize workflows invoked by ID, alongside cancellation/resume, raw/scoped/scopeless gates, and same-worktree recovery. Test progress with the broken unrelated records still present, and paired refusals for defects actually affecting the target. Starting the gate alone does not satisfy this change.
- Ground the expanded design in changes 309/310/312 (metadata transactions and planning), 367 (canonical board), 389/397 and ADRs 101/106 (maintenance preflight), in addition to the runtime history linked above. Record the changed architecture clauses during implementation.
- After the implementation lands and is installed, verify change 444's existing recovery workflow while preserving its uncommitted work. Grooming itself does not resume that run.

## Out of scope

New orchestration systems, commands, force flags, persistent stores or schemas, retirement receipts, background cleanup, leases/TTLs, age cutoffs, retry allowances, storage relocation, general garbage collection, and a second liveness implementation. No automatic cancellation to make room, no blanket trust in HALTED, no dropping positively owned pending work, no manual gate-record editing, and no treating change status as process-death proof. Automatic finalize queue ordering and driver stop/continue behavior are excluded: this change's workflow guarantee is for explicit IDs. Shared infrastructure failures, real dependencies, and actual ownership conflicts remain valid blockers. Publication-journal reconciliation remains change 444's work. Implementation and implementation planning are separate from grooming.

