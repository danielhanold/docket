---
id: 213
slug: settle-the-bash-4-4-mapfile-d-floor-inconsistency-between-te
title: Settle the bash 4.4 mapfile -d floor inconsistency between tests and shipped scripts
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

Change 0202's spec (Assumptions §1) rejected `mapfile -d '' < <(…)` for the `branch_only_artifact`
rewrite because `mapfile -d` requires bash 4.4, while this repo's **shipped-script** floor is bash
4.0 — the floor `ensure-docket-env.sh` enforces, and the one `scripts/docket-status.sh` codes to.

But test-side code already uses `mapfile -d`: `tests/test_grep_portability.sh:102`. That is an
unexplained pre-existing inconsistency, not a sanctioned carve-out. `ensure-docket-env.sh` validates
only `major >= 4`, so nothing actually guarantees 4.4 is present for the test suite either — the
tests are simply passing on a machine whose bash happens to be new enough. On a bash 4.0-4.3 host
the suite would fail with a confusing `mapfile: -d: invalid option` rather than a clear floor
diagnostic.

## What changes

Settle the question and make the answer enforced rather than incidental. Options to weigh at
brainstorm:

- Raise the validated floor to 4.4 in `ensure-docket-env.sh` if 4.4 is genuinely required, so the
  failure is a clear diagnostic at env-check time instead of a mid-suite option error.
- Or declare a documented two-floor policy (shipped scripts 4.0, tests 4.4) and enforce the test
  floor explicitly where the suite starts, so the carve-out is sanctioned and visible.
- Or drop `mapfile -d` from `tests/test_grep_portability.sh` in favor of the
  `while IFS= read -r -d ''` shape 0202 adopts for shipped code, keeping one floor everywhere.

Whichever is chosen, the rule should be stated once where a future author writing `mapfile -d` will
read it, so the next instance is a decision rather than a repeat of this drift.

## Out of scope

- `branch_only_artifact`'s own rewrite — change 0202 lands the `read -r -d ''` shape.
- Any change to which shell the suite selects at runtime (change 0150's territory).

## Why killed

Consolidated into change 0200 (board-checks and test-suite hardening bundle); scope carried over verbatim, nothing dropped.
