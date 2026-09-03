---
id: 376
slug: gate-drive-start-human-output-omits-drive-id-generation
title: '`docket gate drive start` human-readable output omits drive_id/generation'
status: proposed
priority: medium
type: fix
created: 2026-08-30
updated: 2026-08-30
depends_on: []
stacked_on:
related: [375]
discovered_from: [372]
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

During the change-0372 build, the first `docket gate drive start` call's human-readable output did
not include the `drive_id`/`generation` (owner-gen) the operator needs to drive the gate. The
`--json` form carries them, but the default text output did not surface them. With no owner-gen in
hand, the natural next step was to re-run `start` to obtain it — which spawned a second concurrent
drive (#375). So this omission is the **trigger** for that more damaging failure: the interface
doesn't hand back the identity it just minted, and the only obvious recovery re-runs a
non-idempotent command.

## What changes

Ensure `docket gate drive start` surfaces the `drive_id` and `generation` it mints in its default
human-readable output — not only under `--json` — so an operator never has to re-run `start` to
recover them. Confirm the same for any sibling `gate drive` verbs whose identity a caller must
capture. Exact wording/format to be settled during brainstorm.

## Out of scope

- Making `start` idempotent / preventing the second concurrent drive — that fix is #375. This change
  removes the *reason* an operator re-runs `start`; #375 makes the re-run harmless if it still happens.

## Open questions

- Which fields does a caller actually need echoed (drive_id, generation, worktree, anything else)?
- Should the guidance be to always pass `--json` for capture, with the text output as a human
  convenience — or should the text output itself be reliably parseable?
- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Partially stale: `GateDriveResult.HumanText` already prints `drive_id`; the owner generation is omitted from human text by design (ownership credentials never appear in prose). Regroom toward the second open question: document `--json` as the capture channel in the gate-caller prose (`docket-build` references, `docket-implement-next` step 5), and check whether `drive_id` alone suffices once 0375 makes re-run safe.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
