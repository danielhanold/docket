---
id: 89
slug: claim-leases-reclaim-script
title: Claim leases + reclaim script — expired in-progress claims self-heal back to proposed
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [23, 88]
adrs: [1, 12]
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads claims
carry a **lease**: `bd claim` stamps a TTL, `bd heartbeat` refreshes it, and `bd reclaim`
"reverts in_progress issues with expired leases back to ready" (grace window defaulting to 2× the
lease TTL). Crashed agents therefore self-heal — their work returns to the queue without a human
noticing the crash first.

docket has the failure mode but not the healing. A crashed `docket-implement-next` leaves its
change `in-progress` forever; `docket-status` health-checks flag *stale claims* but nothing
recovers them. Worse, the documented recovery is a trap we've hit in practice: resuming
implement-next without an explicit id silently claims a *different* change, because Step-1
selection skips `in-progress`. A `claimed_at` timestamp + TTL in the manifest and a deterministic
reclaim pass make that class of problem disappear — and it's near-free in markdown frontmatter.

## What changes

- New manifest fields stamped at claim time (e.g. `claimed_at:` UTC timestamp; possibly a
  claim identity), written by `docket-implement-next`'s existing claim commit.
- A lease TTL knob (config-layered; default settled in brainstorm — builds legitimately run
  hours, so the TTL must be generous and/or refreshable).
- **A deterministic reclaim script** (constraint from capture: this is a script, not model
  prose — per the ADR-0012 script-vs-model boundary): sweeps `active/` for `in-progress`
  changes whose lease is expired, flips them back to `proposed`, clears `branch:`/claim fields,
  appends a dated note, and reports what it reclaimed. Invoked as a `docket-status` pass via the
  `docket.sh` facade.
- The stale-claim health check upgrades from "flag it" to "flag it + reclaim it (or recommend
  reclaim)" — posture decided in brainstorm.
- Reclaim must respect real work: a reclaimed change with an existing feature branch/partial
  commits needs its state surfaced (e.g. in the reconcile log) so the next claimant finds the
  prior branch rather than starting blind.

## Out of scope

- A heartbeat daemon. docket has no resident process; lease refresh, if any, rides existing
  status-transition commits.
- Killing or rewinding the crashed run's feature branch — reclaim touches metadata only.
- Loop continuation semantics (#0088) and parallel drain (#0008), which this makes safer but
  does not implement.

## Open questions

- TTL default and unit (hours? per-priority?), and whether implement-next refreshes the lease at
  phase boundaries (claim → plan → build → PR) as a poor-man's heartbeat.
- Reclaim-to-`proposed` vs a distinct marker (beads reverts to *ready*; docket's equivalent is
  `proposed` with spec intact — is the reconcile pass enough to absorb a half-built prior
  attempt?).
- Does the reclaim pass run inside `docket-status` unconditionally or behind a flag/config knob?

## Reconcile log
