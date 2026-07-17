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
spec: docs/superpowers/specs/2026-07-17-claim-leases-reclaim-script-design.md
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
| Spec | [2026-07-17-claim-leases-reclaim-script-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-17-claim-leases-reclaim-script-design.md) |
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

Design settled at auto-groom (2026-07-17) — see the linked spec for the full rationale and the
`## Assumptions` audit trail. At proposal altitude:

- A single `claimed_at:` UTC-8601 manifest field, stamped by `docket-implement-next`'s existing
  claim commit and re-stamped at its later phase-boundary metadata commits (a zero-cost
  poor-man's heartbeat); cleared when the change leaves `in-progress`. No claim identity —
  reclaim keys on elapsed time, and concurrency is already settled by final-push CAS.
- A config-layered `reclaim:` block: `lease_ttl` (generous default, ≥ the existing 3-day
  stale-in-progress window; proposed 72h) and `auto` (default `false`), resolved like
  `finalize:` / `learnings:` and shipped end-to-end (sample `.docket.yml` + README).
- **A deterministic reclaim script** `scripts/reclaim-claims.sh` (the binding capture constraint:
  a script per the ADR-0012 script-vs-model boundary, never model prose), reached via
  `docket.sh reclaim`. It reclaims an `in-progress` change **only when its lease is expired AND it
  has no existing feature branch** (the crashed-before-push blind spot the current check misses —
  the one case that is provably collision-free and orphan-free): appends a dated `## Reclaim log`
  entry, flips `status:` back to `proposed`, clears `branch:`/`claimed_at:`, resets
  `reconciled: false`, and commits + pushes under the standard CAS/re-read discipline.
- `stale-in-progress` (in `board-checks.sh`) upgrades to also key on `claimed_at:`+TTL (catching
  the no-branch case), and `docket-status` prints a state-valid recommended reclaim command.
  **Mutation is opt-in**: the default `docket-status` sweep stays warn-only (ADR-0012 "scripts
  never mutate autonomously"); reclaim runs only under `reclaim.auto: true` or an explicit
  `docket.sh reclaim`.
- A sanctioned `in-progress → proposed` reverse edge added to the convention's seven-state
  lifecycle, plus the new `## Reclaim log` body section.

## Out of scope

- A heartbeat daemon. docket has no resident process; the lease re-stamp rides existing
  status-transition commits.
- Killing or rewinding the crashed run's feature branch — reclaim touches metadata only.
- **Reclaiming an expired change that HAS a pushed feature branch** — it may carry real work;
  auto-reclaim there would orphan the branch and collide with the next claimant's worktree
  creation. It stays flagged for a human; branch **adopt-or-supersede** is a recommended
  **follow-up change**, not this one.
- Loop continuation semantics (#0088) and parallel drain (#0008), which this makes safer but
  does not implement.

## Open questions

Resolved at auto-groom (2026-07-17); detail in the linked spec's §7 Assumptions:
- TTL default/unit and phase-boundary refresh → single generous config default (proposed 72h) +
  re-stamp at existing phase-boundary commits (spec §7-D, §3).
- Reclaim-to-`proposed` vs a marker; is reconcile enough to absorb a half-built attempt →
  `proposed` with `spec:` intact and no new status; the half-built case is designed out by the
  no-branch narrowing (only the pre-branch window is ever reclaimed), so there is no branch to
  absorb (spec §7-B, §7-C).
- Runs unconditionally vs gated → gated; mutation is opt-in (`reclaim.auto`, default off), default
  pass stays warn-only (spec §7-E).

## Reconcile log
