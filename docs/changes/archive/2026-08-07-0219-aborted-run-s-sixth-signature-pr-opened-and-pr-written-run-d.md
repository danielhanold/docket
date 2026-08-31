---
id: 219
slug: aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d
title: aborted-run's Step 7 seam — a fourth git-only leg, plus GitHub enrichment for leg C
status: done
priority: high
type: fix
created: 2026-08-05
updated: 2026-08-07
claimed_at: 
depends_on: [211]
related: [200, 222]
discovered_from: [211]
adrs: [72]
spec: docs/superpowers/specs/2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-design.md
plan: docs/superpowers/plans/2026-08-07-aborted-run-step-7-seam.md
results: docs/results/2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-results.md
trivial: false
auto_groomable: false
branch: feat/aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d
pr: https://github.com/danielhanold/docket/pull/171
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-design.md) |
| Plan | [2026-08-07-aborted-run-step-7-seam.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-aborted-run-step-7-seam.md) |
| Results | [2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d-results.md) |
| ADRs | [ADR-0072](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0072-leg-c-predicate-duplicated-by-value-across-two-scripts.md) |
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

## Reconcile log

### 2026-08-07

Re-read the change, its spec, `related: [200, 222]`, and the current code on `origin/main`
(035e8eba). **The design holds unchanged — no scope adjustment needed.**

Verified against current code, every anchor the spec names still exists and still reads as
described:

- `scripts/board-checks.sh` — the `aborted-run` block gates on `status: in-progress` (line ~414);
  leg C's `[ -z "$(fm_field "$f" pr)" ]` short-circuit is at line ~462 and is the read leg D must
  share via the hoisted `ar_pr`. The stale preamble comment still reads "Two INDEPENDENT legs;
  either emits, and both can emit on one change" (line ~401) — both halves stale since 0211 made it
  three, exactly as the spec states.
- `ABORTED_RUN_STALE_SECS` (12h, line 172) and `ABORTED_RUN_IDLE_SECS` (2h, line 189) both remain
  hardcoded with no config knob, so the spec's "reuse leg C's floor, do not introduce a second magic
  number" reasoning still stands on its stated precedent.
- `scripts/docket-status.sh` — `detect_merged` is at line 483 and still carries both the batched
  sweep and the documented "per-change `gh pr list` fallback only for changes with no `pr:` set",
  plus the `sweep-skipped <reason>` / `return 0` posture at lines 514, 518, 535. The enrichment leg's
  reuse target is intact.
- `scripts/board-checks.md` — the `## Not covered` paragraph is at lines 269–271 and still defers
  the case citing the git-only contract, the exact text the spec directs be rewritten rather than
  deleted.

Coupling re-checked: `related: [200, 222]` are both still `proposed` and unbuilt, so neither has
landed a conflicting edit to `board-checks.sh` or its suite. The expected rebase compose is a future
concern, not a present one. `depends_on: [211]` is satisfied (`done`), and 0211's leg C is present
in the code as described.

Confirmed the spec's test guidance is still live: `tests/test_board_checks.sh` fixture ids must not
be hardcoded.

Auto-capture: the discovery pass surfaced no adjacent work clearing the six admission gates. Both
code legs and all four doc repairs sit inside this change's own declared scope; nothing minted.
