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
reconciled: true
claimed_at: 2026-07-19T12:21:40Z
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

### 2026-07-19 — reconcile (implement-next)

Verified the design against current `main`/`docket`:

- `scripts/board-checks.sh` is structurally as the spec assumes — the per-file `FILES` walk over
  `active/` + `archive/`, the `emit`/`FINDINGS` accumulator, the `GIT`/`NOW` mock seams, and the
  `merge-gate-stall` / `stale-in-progress` precedents (the latter hardcodes its `3*86400` branch-idle
  horizon, the exact model A4 mirrors). No drift.
- `scripts/lib/docket-frontmatter.sh` provides `finalize_blocked FILE` (whole-line `has_section`
  match) and `iso_to_epoch`, both as the spec relies on.
- Related #0087 (headless-finalize-driver — introduced the `## Finalize blocked` marker) is `done`
  (archived 2026-07-19); the board cell `finalize blocked — needs you` is live in `render-board.sh`
  (via `finalize_blocked()`). `depends_on: []` satisfied; no design-ahead gating.
- Marker-age signal: change file's last-commit timestamp via
  `git -C "$CHANGES_DIR" log -1 --format=%ct -- "$f"`. `$CHANGES_DIR` is always an absolute path in
  real callers (docket-status) and in the test harness, so the absolute `$f` pathspec resolves.
- Scope confirmed unchanged: add check-id `stale-finalize-blocked` to `board-checks.sh` + document it
  in `scripts/board-checks.md`; extend `tests/test_board_checks.sh`. No `.docket.yml` knob (A4). The
  docket-status SKILL enumeration is a curated subset (already omits merged-orphan/unknown-commit-ref),
  so per precedent it is not extended.
