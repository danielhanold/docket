---
id: 150
slug: pin-or-report-the-resolved-shell-toolchain-across-the-test-s
title: Pin or report the resolved shell toolchain across the test suite
status: proposed
priority: low
type: chore
created: 2026-07-28
updated: 2026-08-09
depends_on: []
related: [151, 227]
discovered_from: [130]
adrs: []
spec: docs/superpowers/specs/2026-08-07-pin-or-report-the-resolved-shell-toolchain-across-the-test-s-design.md
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
| Spec | [2026-08-07-pin-or-report-the-resolved-shell-toolchain-across-the-test-s-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-pin-or-report-the-resolved-shell-toolchain-across-the-test-s-design.md) |
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

Groomed 2026-08-07 (auto-groom); the linked spec settles the design — 7 assumptions, all
critic-confirmed sound.

- A `tests/lib/toolchain-report.sh` sourceable helper (`toolchain_report()`) printing the resolved
  path and best-effort version of `grep`, `sed`, `awk`, `date`, `readlink` — gating nothing,
  permanently. Implementation lifted from 0130's block in `tests/test_grep_portability.sh`
  (:87-93), generalized to a loop, keeping its capture-then-here-string SIGPIPE discipline; that
  block becomes a source-and-call plus a structural smoke assert, so there is one implementation.
- `scripts/run-tests.sh` emits the report once per suite run on stdout, after arg/target
  validation and before the launch loop, plus a runner-owned `test bash` line naming `$TEST_BASH`
  (which `command -v bash` could misreport). Gate logs thereby record which toolchain actually
  ran; the report is `-j`-independent, so the stdout byte-stability contract holds.
- Prose amendments only where the header falsifies them: the interruption comments/docs claiming
  an interrupted run "has printed nothing but the stderr ticker" (runner, run-tests.md,
  test_run_tests.sh) — no assert keys on them.

## Out of scope

- Re-opening change 0130's static bound guard, which stands on its own.
- Any pin (PATH or per-site) — settled above.
- Mandating that all test files source the helper.

