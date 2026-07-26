---
id: 130
slug: make-the-finalize-marker-reachability-guard-portable-to-bsd
title: Make the finalize marker reachability guard portable to BSD grep
status: proposed
priority: medium
created: 2026-07-22
updated: 2026-07-26
depends_on: []
related: []
discovered_from: [116]
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
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`tests/test_finalize_disposition.sh` uses the ERE interval `.{0,600}` to prove that the Finalize
blocked marker write is reachable from the abort-and-report procedure. BSD grep rejects repetition
bounds above 255 with `maximum repetition exceeds 255`, so the assertion fails before examining
the unchanged finalize skill. This prevents a portable whole-suite green run on macOS.

## What changes

Replace the oversized interval with a portable structural extraction or bounded multi-stage check
that preserves the reachability claim. Mutation-test it by removing the procedure's marker-write
call and confirming only the intended guard reddens.

## Out of scope

- Changes to finalize behavior or the `## Finalize blocked` contract.
- Broad rewrites of other disposition assertions.

## Open questions

- None.

## Triage note (2026-07-26, change 0124)

**Confirmed still live, and it hides on a developer machine — do not conclude "already fixed" from
a green suite.** The offending interval is `tests/test_finalize_disposition.sh:186`
(`grep -Eqi "where the reason surfaces.{0,600}appends the .{0,4}## Finalize blocked"`). It is the
only interval above 255 anywhere in `tests/`.

Running `bash tests/test_finalize_disposition.sh` on the maintainer's machine **PASSES**, because
that machine's PATH resolves `grep` to `ugrep 7.5.0`, which accepts the bound. The system grep does
not:

```
$ /usr/bin/grep -Eqi "he.{0,600}lo"   # <<< "hello"
grep: maximum repetition exceeds 255
```

So the correct verification is `/usr/bin/grep`, not whatever `grep` resolves to. Any fix must be
mutation-tested against the system grep explicitly, and it is worth considering whether the suite
should pin or report which grep it ran under — a portability guard that silently tests a different
tool than the one it targets is the `guards-are-code` vacuity trap in a new costume.
