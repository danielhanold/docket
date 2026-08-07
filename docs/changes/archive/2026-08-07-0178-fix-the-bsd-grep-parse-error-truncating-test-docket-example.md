---
id: 178
slug: fix-the-bsd-grep-parse-error-truncating-test-docket-example
title: Fix the BSD-grep parse error truncating test_docket_example_yml.sh
status: killed
priority: medium
type: fix
created: 2026-07-31
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [168]
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

`tests/test_docket_example_yml.sh` hits a **bash parse error** when BSD grep precedes ugrep on
`PATH`, which truncates the file to roughly half its asserts. Surfaced during change 0168's build
and reproduced on the unmodified `origin/main`, so it is pre-existing and was deliberately left
unfixed there.

The consequence is worse than a portability annoyance: the file does not fail loudly, it stops
early, so on any machine with a BSD-grep-first `PATH` roughly half of the `.docket.example.yml`
invariants silently stop being checked while the run still reports success. That is a guard
population quietly halving itself with no signal.

This is narrower than change 0150, which owns the general design question of pinning or reporting
the resolved shell toolchain across the whole suite. This one is a concrete parse error in a single
test file that can be fixed on its own, ahead of and independent of that design.

## What changes

- Reproduce the parse error with BSD grep ahead of ugrep on `PATH`.
- Fix the construct in `tests/test_docket_example_yml.sh` so the file parses and runs completely
  under both greps.
- Confirm the full assert population runs under both — a count, not an exit code, since the defect's
  signature is a silently shortened run rather than a red one.

## Out of scope

- The suite-wide toolchain pinning/reporting design (change 0150).
- Auditing other test files for the same construct beyond a quick grep for it.

## Open questions

- Is the same construct present in any other test file?

## Why killed

Consolidated into #0246 at the 2026-08-07 backlog triage: sequenced first there — the BSD-grep truncation must be fixed before #0187's guards (which sit in the possibly-skipped half of the file) can be trusted.
