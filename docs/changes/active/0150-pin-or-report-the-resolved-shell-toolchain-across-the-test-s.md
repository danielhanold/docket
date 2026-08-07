---
id: 150
slug: pin-or-report-the-resolved-shell-toolchain-across-the-test-s
title: Pin or report the resolved shell toolchain across the test suite
status: proposed
priority: medium
type: chore
created: 2026-07-28
updated: 2026-08-07
depends_on: []
related: [151]
discovered_from: [130]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

Change 0130 fixed a portability bug that was invisible on the maintainer's machine: a test asserted
with an ERE interval bound above 255, which BSD grep rejects, but PATH `grep` there resolves to
`ugrep 7.5.0`, which accepts it. The suite ran green while the bug was real. That failure mode is
not specific to grep bounds — it is the general shape of a portability suite silently exercising a
different tool than the one it targets.

0130 deliberately scoped this out (its spec's A4): it built a static source-level guard plus an
informational line in the one new test naming the resolved `grep`, and left toolchain reporting
across the rest of the suite for a separate design.

**Re-scoped 2026-08-07 (triage).** This stub's original framing — and its 2026-07-28 auto-groom
abstain — rested on "there is no suite runner and no `tests/lib/`". Both are now false: change
0227 shipped `scripts/run-tests.sh` (the parallel runner, now also `finalize.test_command`), and
`tests/lib/` exists (`sync_agents_common.sh`). The single seam the design was missing is exactly
where a toolchain report obviously lives. The abstain's other conclusions were verified and are
carried forward as settled here:

- **Report, never pin.** A global PATH pin generalizes 0130's recorded false-failure hazard and
  makes the maintainer's machine less representative. Do not assert the resolved `grep` *is*
  `/usr/bin/grep` — that is a pin wearing an assert.
- **No per-site absolute-path guard.** Measured: the "convention" was 7 absolute sites in 2 files
  with ~169 day-one violations in-scope, and the guard could never see the very file whose bug
  motivated it. Dropped for good.
- **CI-matrix testing is out of scope** — there is no CI; standing one up is its own change.

## What changes

- A `tests/lib/toolchain-report.sh` helper printing the resolved path and version of `grep`,
  `sed`, `awk`, `date`, `readlink` — gating nothing, permanently. Lift the implementation from
  0130's existing block in `tests/test_grep_portability.sh` (:87-93) verbatim, including its
  capture-then-here-string SIGPIPE discipline, and replace that block with a call to the helper
  so there is one implementation.
- Emit the report once per suite run from `scripts/run-tests.sh` (the seam that now exists), so
  every gate log records which toolchain actually ran.

## Out of scope

- Re-opening change 0130's static bound guard, which stands on its own.
- Any pin (PATH or per-site) — settled above.
- Mandating that all test files source the helper.

