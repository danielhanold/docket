---
id: 368
slug: resume-halted-preallocation-recovery
title: Recover a run halted before its workspace was allocated
status: proposed
priority: medium
type: fix
created: 2026-08-29
updated: 2026-08-29
depends_on: []
stacked_on:
related: [318, 366]
discovered_from: [318]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

A run that halts *before* it allocates its feature workspace — e.g. `docket-implement-next`
halting at the Step 3 reconcile assessment, which is exactly what change 0318's first run did —
leaves an `in-progress` change with a `## Run halted` marker, a live claim lease, and **no
workspace manifest** on disk. Neither sanctioned recovery command can clear that state:

- **`change resume-halted` refuses.** Its reprobe classifies an absent workspace manifest as
  `StateForeign` (`internal/workspace/inspect.go` returns foreign with detail "no workspace
  manifest"), and `resumeQuiescenceRefusal` maps `StateForeign` to `workspace-writer-active`
  (`internal/app/change_halt.go`). So resume treats a workspace that was *never created* as one
  whose writer "may still be live" and adopts nothing — even with `--acknowledge-quiescent`.
- **`change reclaim` is not applicable in time.** Reclaim *does* treat an absent workspace as
  clear, but it gates on a **strictly-expired** lease (default `reclaim.lease_ttl` = 72h). A fresh
  halt is minutes old, so reclaim is unavailable for ~3 days.

The result is an asymmetry: an absent/foreign workspace is "clear" for `reclaim` but "writer
active" for `resume-halted`. A change halted before allocation is therefore un-restartable through
the sanctioned CLI until its 72h lease expires.

The current manual workaround (found while restarting 0318) is to run `docket workspace prepare`
first — which allocates the branch/worktree/manifest and moves the state `foreign → ready` — and
only then `resume-halted`. That works, but it forces a human to conjure a workspace purely to get
past a quiescence guard, which is backwards: the safe, quiescent case (nothing was ever created)
is the one the guard blocks.

## What changes

- Make a run halted before workspace allocation recoverable through the sanctioned CLI without
  first hand-allocating a workspace it does not need.
- Resolve the `resume-halted` / `reclaim` asymmetry so an absent, never-allocated workspace reads
  as quiescent for both (a foreign manifest with an *identity mismatch* — a real other-owner
  workspace — must still block resume; the fix is scoped to genuine absence).
- Keep the destructive/adoption safety `resume-halted` and `reclaim` are guarding: this must not
  weaken the guard against adopting a workspace whose writer may actually be live.

## Out of scope

- Changing the 72h `reclaim.lease_ttl` default or making it per-invocation.
- Any change to how a run that *did* allocate a workspace is resumed (the `ready` / `cleaned` /
  `dirty-owned` paths are correct today).
- Reworking the halt marker mechanism or the gate facade's attribution.

## Open questions

- Fix at the classifier (distinguish "absent — never allocated" from "foreign — mismatched owner"
  as separate states) vs. at `resumeQuiescenceRefusal` (treat a proven-absent workspace as
  quiescent) vs. a dedicated `resume-halted --no-workspace` recovery? Prefer the smallest change
  that keeps the mismatched-owner case blocking.
- Should the fix also let `reclaim` proceed on a proven pre-allocation halt regardless of lease
  age, or is fixing `resume-halted` sufficient?
- Is there a regression test fixture for "halt at reconcile, before allocation" specifically?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
