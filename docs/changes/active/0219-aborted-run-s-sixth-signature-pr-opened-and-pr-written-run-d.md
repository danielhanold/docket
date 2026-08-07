---
id: 219
slug: aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d
title: aborted-run's Step 7 seam — a fourth git-only leg, plus GitHub enrichment for leg C
status: in-progress
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-07
claimed_at: 2026-08-07T15:59:04Z
depends_on: [211]
related: [200, 222]
discovered_from: [211]
adrs: []
spec: docs/superpowers/specs/2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: feat/aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-design.md) |
<!-- docket:artifacts:end -->

## Why

`aborted-run` is the external, mechanical oracle for an autonomous run that stopped mid-step. Its
three legs (0113's A and B, 0211's C) leave the **Step 7 seam** — push, open PR, record
`status: implemented` + `pr:` — incompletely covered, in two different ways.

**A recorded PR with no status advance is invisible until 12h.** `docket-implement-next` writes
`status: implemented` and `pr:` in a single field-write, so a manifest carrying `pr:` while still
`in-progress` is an anomaly by construction. Leg A sees no incoherence (`plan:` and `results:` are
both recorded by then); leg C deliberately short-circuits on a non-empty `pr:` and exits with zero
git calls; only leg B catches it, at the same 12h lag 0211 exists to close.

**Leg C's `pr:`-unset finding is ambiguous and a human has to resolve it by hand.** For the state
0194's second stop produced — PR open on GitHub, manifest unwritten — leg C already fires, telling
the reader to "verify the PR exists." It cannot do better: `board-checks.sh` is git-only by contract
and shells no `gh`. But two very different situations produce that one finding — a PR that exists
and went unrecorded, versus a run that died before creating one — with two different remedies.

This change is therefore not "a fourth abort signature." It is one new git-only leg, plus the
enrichment that makes leg C's existing finding actionable.

## What changes

- **Leg D in `board-checks.sh`** — fires when `status: in-progress` and `pr:` is non-empty. Git-only;
  the script's contract is untouched. Time-free like leg A (the two fields are written in one
  stroke, so there is no healthy window to wait out), and sharing a single hoisted `pr:` read with
  leg C to protect that cost-sensitive path (change 0176).
- **A GitHub enrichment leg in `docket-status.sh`**, beside `detect_merged` — for `in-progress`
  changes with `pr:` unset past leg C's own 2h idle floor, resolve whether an open PR exists on
  `feat/<slug>`. Reuses `detect_merged`'s best-effort posture verbatim: any gh/network/parse failure
  emits `sweep-skipped <reason>` and returns 0.
- **Doc repairs** — rewrite (never delete) `board-checks.md`'s `## Not covered` paragraph to name
  the surviving residual, repoint its line 271 at `docket-status.sh`, and fix the `aborted-run`
  preamble comment still claiming "Two INDEPENDENT legs" (stale since 0211).

## Out of scope

- Retuning leg B's 12h horizon or leg C's 2h floor.
- Any status flip or claim release — the advisory posture holds.
- Relaxing `board-checks.sh`'s git-only contract. Rejected at grooming: the GitHub work goes where
  `gh` already lives, so the offline-safe check stays offline-safe.
