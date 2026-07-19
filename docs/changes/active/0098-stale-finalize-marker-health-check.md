---
id: 98
slug: stale-finalize-marker-health-check
title: Health check for a stale `## Finalize blocked` marker
status: in-progress
priority: medium
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [87]
discovered_from: [87]
adrs: []
spec: docs/superpowers/specs/2026-07-19-stale-finalize-marker-health-check-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/stale-finalize-marker-health-check
pr:
blocked_by:
reconciled: false
claimed_at: 2026-07-19T12:19:14Z
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-19-stale-finalize-marker-health-check-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-19-stale-finalize-marker-health-check-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0087 added the `## Finalize blocked` marker and its board cell, but `scripts/board-checks.sh`
gained nothing. A marker whose underlying cause the human fixed — without merging, and without
re-running finalize with the change id named — sits on the board indefinitely with no advisory.

The marker's clearing rule fires on the paths finalize itself drives. It does not fire when the
human resolves the cause out of band, which is exactly the case a health check exists to catch.
`merge-gate-stall` is the obvious precedent: same shape (a state that is legitimate briefly and
suspicious once it persists), same surface.

## What changes

Add a new health check — check-id `stale-finalize-blocked` — to `scripts/board-checks.sh` that
flags an `implemented` change carrying the `## Finalize blocked` section whose marker has outlived a
fixed staleness horizon, surfaced through the same `docket-status` needs-you finding channel
`merge-gate-stall` uses. It advises only — it never mutates the change file and never auto-clears the
marker.

The check is **git-only and time-based**, honoring `board-checks.sh`'s core no-`gh`/no-network
invariant: marker age is the change file's last-commit timestamp (the bare heading is undated and its
in-body date is model-authored, so git's clock is the tamper-proof signal), and the horizon is a
hardcoded 72 h constant mirroring `stale-in-progress`'s own hardcoded branch-idle horizon — no new
config knob. Design settled in the linked spec; see its `## Assumptions` block for why the
cause-re-probing alternative (which would need a network probe) is rejected in favor of a pure
time signal, and why a still-blocked-but-old marker firing the advisory is acceptable.

## Out of scope

- Auto-clearing the marker. A health check advises; it does not mutate change files.
- Distinguishing "cause resolved, marker stale" from "cause still holds, genuinely blocked" — a
  git-only check structurally cannot probe live PR state; see the spec.
- Revisiting the clearing rule itself (tracked separately as change #0099).
- Mirror readiness parity (change #0097).

## Reconcile log
