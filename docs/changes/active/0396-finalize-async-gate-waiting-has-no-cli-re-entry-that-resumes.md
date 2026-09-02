---
id: 396
slug: 'finalize-async-gate-waiting-has-no-cli-re-entry-that-resumes'
title: 'finalize async gate WAITING has no CLI re-entry that resumes the same drive'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-02'
updated: '2026-09-02'
depends_on: []
stacked_on:
related: [364]
discovered_from: [364]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

During finalize of change 0364, the async local gate returned disposition `waiting` from `finalize.rebase-continue` with a continuation (drive_id + generation). There is no CLI path that both advances THAT drive to terminal AND records finalize evidence through the finalize seam:

- `finalize.rebase` / `finalize.rebase-continue` never expose the continuation as a flag, so re-entering `finalize rebase` calls the gate seam's `Start` (fresh `os.MkdirTemp` run root) every time — minting a NEW drive and re-running the whole suite instead of resuming the live one. Repeated re-entry therefore never converges and spawns extra detached-supervisor suite runs under `/var/folders/.../docket-finalize-gate-*`.
- The generic `docket gate drive advance --drive-id <id> --owner-gen <gen>` DOES advance the same drive to PASSED, but it is not the finalize seam, so it mints no finalize evidence block; the evidence has to be recovered out-of-band via `evidence.record --run <raw_run_dir>`.

Net effect: the documented WAITING re-entry contract (skill step 3/4 and the memory note 'drive finalize rebase's continuation with docket gate drive advance, don't re-invoke rebase') is not actually expressible as a clean single operation. A caller either burns extra full suite runs (re-invoking rebase) or has to hand-stitch generic drive-advance + evidence.record. This lengthens every finalize whose gate takes more than one slice, and it is easy to get wrong (observed live on 0364: multiple orphaned gate drives, evidence minted manually).

Note: the JSON envelope of `finalize.rebase` truncates the attempt token in the returned `attempt` field (e.g. `20260902T095609Z-c3a5cebc`) while the on-disk receipt holds the full token (`20260902T095609Z-c3a5cebc1c9e`); `rebase-continue` requires the full token, so the truncated display caused an `attempt-token-mismatch` false stop. This should be fixed alongside — the surfaced token must be the one the continue op accepts.

## What changes

Give finalize a first-class, idempotent re-entry for a WAITING gate that resumes the SAME drive across slices AND mints evidence through the finalize seam on PASSED — e.g. a `finalize.rebase-continue` (or a dedicated `finalize.gate-advance`) that accepts the returned continuation (drive_id + generation), calls the finalize seam's `Advance` rather than `Start`, and on a terminal PASSED records the evidence block and returns `rebased`/`unchanged` with it. Re-running `finalize rebase` with no continuation must resume an existing live drive for the same change+phase rather than starting a fresh run root. Also surface the full, continue-accepted attempt token in the `finalize.rebase`/`rebase-continue` JSON envelope (stop truncating it).

## Out of scope

Changing the gate supervisor/driver mechanics themselves (gate.drive.* stays as-is); changing the two-agent resolver/repair flow; any change to build's gate.
