---
id: 216
slug: guard-the-capture-shape-constraint-in-branch-only-artifact-w
title: Guard the capture-shape constraint in branch_only_artifact with a mutation G
status: killed
priority: medium
type: chore
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: [200]
discovered_from: [202]
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

`branch_only_artifact` in `scripts/board-checks.sh` now depends on two properties, and only one of
them is guarded. Mutation F reverts `-z` and the `read -r -d ''` form, which reproduces the
C-quoting false positive change 0202 fixed. The spec also asked mutation F to revert the
capture-then-here-string shape; the build dropped that half, reasoning that C-quoting originates in
`ls-tree` and not in the consumption shape — true for reproducing that defect, and it leaves the
other property with no coverage at all.

A refactor to `boa_list="$(… git ls-tree -r -z …)"` + `done <<<"$boa_list"` keeps `-z`, keeps
`read -r -d ''`, passes `bash -n`, and makes `branch_only_artifact` return 1 for **every** input:
command substitution strips NUL bytes, so `read -d ''` hits EOF and the loop body never runs. Leg A
would go permanently, silently false-negative with a green suite. `board-checks.sh` carries an
explicit "do not simplify this back" comment with nothing enforcing it — decoration, by this repo's
own guard rule.

Raised as an `important` (non-blocking) finding at 0202's deep-rung review and left for merge-time
judgment.

## What changes

- `tests/test_board_checks.sh` — add a mutation G that rewrites the consumption to the capture
  shape while keeping `-z`, and asserts fixture 230 goes GREEN (i.e. the check stops firing), so
  the constraint the comment states is enforced by execution.

## Out of scope

- Changing `branch_only_artifact` itself; the current shape is correct.
- Mutation F's existing arm, which stays as-is.

## Open questions

- Whether the same capture-shape hazard exists at any other `-z` read in the scripts, which would
  make this a helper-level guard rather than a single mutation arm.

## Why killed

Consolidated into change 0200 (board-checks and test-suite hardening bundle); scope carried over verbatim, nothing dropped.
