---
id: 150
slug: pin-or-report-the-resolved-shell-toolchain-across-the-test-s
title: Pin or report the resolved shell toolchain across the test suite
status: proposed
priority: medium
type: chore
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [130]
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

Change 0130 fixed a portability bug that was invisible on the maintainer's machine: a test asserted
with an ERE interval bound above 255, which BSD grep rejects, but PATH `grep` there resolves to
`ugrep 7.5.0`, which accepts it. The suite ran green while the bug was real. That failure mode is
not specific to grep bounds — it is the general shape of a portability suite silently exercising a
different tool than the one it targets.

0130 deliberately scoped this out (its spec's A4): it built a static source-level guard plus an
informational line in the one new test naming the resolved `grep`, and left toolchain pinning or
reporting across the rest of the suite for a separate design. There is no suite runner today —
each of the ~63 test files is invoked on its own — so there is no single seam where a resolved
toolchain could be pinned, reported, or asserted.

## What changes

To be designed. The shape of the problem:

- Decide whether the suite should **pin** a toolchain (force `/usr/bin/grep` and friends to resolve
  first for portability-sensitive assertions), **report** one (each run prints which `grep`, `sed`,
  `awk` it actually used), or both.
- Decide where that lives given there is no suite runner — a shared `tests/lib` prelude sourced by
  each file, a thin runner introduced for the purpose, or a CI-only PATH posture.
- Consider the sibling tools with the same GNU-vs-BSD divergence surface (`sed -i`, `awk`,
  `date`, `readlink`), not only `grep`.

## Out of scope

- Re-opening change 0130's static bound guard, which stands on its own.

## Open questions

- Is a pin desirable at all, or does it mask the divergence the suite should be surfacing?
- Should CI run the suite under both a GNU and a BSD toolchain rather than pinning either?
