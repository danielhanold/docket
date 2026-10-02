---
id: 490
slug: 'replace-the-durable-worktree-admission-slot-with-a-superviso'
title: 'Replace the durable worktree admission slot with a supervisor-held kernel lock'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-10-02'
updated: '2026-10-02'
depends_on: [489]
stacked_on:
related: [375, 428, 435, 437, 439, 441, 446, 452, 453, 457]
discovered_from: []
adrs: [118, 120, 125]
spec:
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
| ADRs | [ADR-0118](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0118-worktree-wide-gate-admission-and-explicit-human-cancellation.md), [ADR-0120](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0120-historical-gate-drive-schemas-are-assessed-never-executed.md), [ADR-0125](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0125-historical-gate-discovery-has-no-global-veto-relevance-to-th.md) |
<!-- docket:artifacts:end -->

## Why

Change 0375 (ADR-0118) enforces "one live gate per worktree" with a durable JSON state machine under `<git-common-dir>/docket/gate-admission/`. Its states are reserved, executing, stopping, unresolved, and released.

The state machine fails closed. Any ambiguous outcome marks the slot `unresolved` or `stopping`: a lost launch response, an interrupted release, or a HALTED drive whose stop can't be proven. From then on every later start in that worktree is blocked, including the build suite and finalize, until someone recovers the slot. The file header says so directly: "the flock is a critical-section primitive here, never the lifetime guarantee — the persisted state … is the authority on whether the worktree is busy."

The fixes that followed form the longest chain in the repo:

- 0428: 989 legacy records vetoed admission.
- 0435, 0437.
- 0439: a slot left `executing` blocked finalize.rebase.
- 0441.
- 0446: one orphaned HALTED drive became a veto over every worktree.
- 0452, 0453.
- 0457: still open.

Together that is about five run halts plus several finalize and resume incidents.

The kernel already provides what the slot reconstructs by hand. The gate supervisor holds `live.lock` with flock for its whole lifetime (`internal/process/lock.go`, `supervisor.go`), and the kernel releases it when the process dies.

ADR-0118 rejected two alternatives:

- "A worktree flock alone": short-lived CLI calls release it while detached work survives.
- Releasing the slot on process death: rejected as fail-open.

Neither objection applies when the long-running supervisor holds the lock. The CLI's lifetime no longer matters. A release is not a guess about liveness; it is the kernel proving no holder remains.

Once task drives are gone (dependency 0489), only build-owned and finalize gates compete for a worktree, so contention is rare.

## What changes

- **The lock.** For the whole run, the supervisor holds an exclusive flock on a per-worktree lock file keyed by the canonical worktree root. If grooming finds it necessary, the supervised process tree holds it too. `worktree-busy` means the lock is held. A dead run frees it automatically, with no recovery step.
- **What to retire.** Retire everything that exists only to recover the durable slot: the persisted slot states (reserved/executing/stopping/unresolved/released), `launch-unconfirmed`, incumbent reconciliation, the legacy-history inventory on first admission, and `gate.history.cleanup`. Drive records stay as evidence.
- **ADRs.** Supersede ADR-0118, and ADR-0120/0125 as far as they cover slot recovery, with a new ADR.

Grooming must verify:

- the supervisor outlives its test process tree, or the tree inherits the lock fd;
- flock behaves the same across fork/exec on macOS and Linux;
- every linked worktree can reach the lock-file location;
- what `run.cancel` still needs once there is no slot to tear down.

Accepted loss: the `launch-unconfirmed` protection, and the durable record of who last held the slot. Drive records remain.

## Out of scope

- Run-id fencing of gate admission (the follow-up change).
- The build-owned drive protocol: start/advance, owner generation, handoff/claim.
- Fingerprinting, and the stored terminal result record.
- Finalize's rebase gate logic, apart from its admission call.
