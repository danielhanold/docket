---
id: 177
slug: harden-the-0174-fixture-template-helpers-sticky-failure-ungu
title: Harden the 0174 fixture-template helpers (sticky failure, unguarded mktemp, destructive pre-clean, leaked root)
status: killed
priority: medium
type: chore
created: 2026-07-31
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [174]
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

Change 0174 replaced four per-assertion git-fixture builders with build-once-and-copy template
helpers in `test_docket_config.sh`, `test_docket_status.sh`, `test_board_checks.sh`, and
`test_closeout.sh`. Its review surfaced four robustness gaps in the new helper bodies, all judged
Minor and deliberately left unfixed as below the bar for a change whose scope was the speedup. They
are recorded in that change's results file, and each is a latent footgun rather than a live defect:

- **Template-build failure is now sticky.** Each builder assigns its file-scope global on the first
  line, so a partial failure leaves a non-empty path and every later call copies a broken template
  instead of retrying. The pre-0174 code rebuilt from scratch on every call, so this failure mode is
  new.
- **`new_repo`'s `root="$(mktemp -d …)"` is unguarded** in both files that define it; an empty
  `$root` would make the following `cp -R` write to `/origin.git`.
- **`mkrepo` gained an unconditional `rm -rf "$dir" "$bare"`.** Safe against all 114 current call
  sites (checked for duplication and prefix collision), but it silently destroys state for any
  future test that seeds `$dir` before calling it. The rationale lives in the plan, not in the code.
- **`test_closeout.sh` still leaks one template root** — its existing `trap … EXIT` near line 604
  would have been replaced by a new one, so no cleanup trap was added. `test_board_checks.sh` did
  get one, cutting its leak from 34 roots to zero.

## What changes

Harden the four helper bodies: clear or re-derive the template global on a failed build, guard the
`mktemp -d` results before any `cp -R`/`rm -rf`, make `mkrepo`'s destructive pre-clean explicit in
the code rather than only in the plan, and give `test_closeout.sh` a composing cleanup path that
does not clobber its existing `EXIT` trap.

## Out of scope

- Any change to what the tests assert, or to the template-copy design itself.
- The suite's invocation-bound cost (changes 0175 and 0176).

## Open questions

- Whether the four helpers should converge on a shared `tests/lib/` at this point, or stay
  independent as 0174 decided — hardening four near-identical bodies is the first real pressure on
  that call.

## Why killed

Consolidated into #0252 at the 2026-08-07 backlog triage: all four 0174-template robustness gaps verified and carried over; lands with the shared fixture-helper work.
