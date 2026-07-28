---
id: 130
slug: make-the-finalize-marker-reachability-guard-portable-to-bsd
title: Make the finalize marker reachability guard portable to BSD grep
status: in-progress
priority: medium
created: 2026-07-22
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [116]
adrs: []
spec: docs/superpowers/specs/2026-07-27-bsd-grep-interval-portability-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/make-the-finalize-marker-reachability-guard-portable-to-bsd
claimed_at: 2026-07-28T05:14:55Z
pr:
blocked_by:
reconciled: false
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-27-bsd-grep-interval-portability-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-27-bsd-grep-interval-portability-design.md) |
<!-- docket:artifacts:end -->

## Why

`tests/test_finalize_disposition.sh` uses the ERE interval `.{0,600}` to prove that the Finalize
blocked marker write is reachable from the abort-and-report procedure. BSD grep rejects repetition
bounds above 255 with `maximum repetition exceeds 255`, so the assertion fails before examining
the unchanged finalize skill. This prevents a portable whole-suite green run on macOS.

## What changes

Two parts, per the linked spec:

1. Replace the oversized interval at `tests/test_finalize_disposition.sh:186` with an unbounded
   **within-line** `.*`. `grep` is line-based, so the same-line scope and the anchor ordering — the
   properties the assertion actually rests on — survive; the numeric bound never was the constraint.
2. Add `tests/test_grep_portability.sh`, a repo-wide static guard that fails on any ERE interval
   literal with a bound above 255. Its population is **every tracked path** (`git ls-files`, anchored
   on the repo root, no extension filter) minus the `docs/` prefix — archived records and historical
   plans legitimately quote the defective pattern and are immutable. The guard carries no >255
   literal of its own (fixtures assembled at runtime) and asserts it is itself in the scanned,
   clean population.

Both halves are mutation-proofed: the repaired assertion by deleting the marker-write clause from
the finalize skill, and the guard by injecting a >255 literal into a real tracked non-`.sh` file
(must redden) and under `docs/` (must stay green).

**Verification runs `PATH=/usr/bin:$PATH`** so `grep` resolves to BSD grep — prepend, never replace,
since the suite also needs Homebrew `jq`/`gh`. A green run under the ambient PATH proves nothing:
this machine's `grep` is ugrep 7.5.0, which accepts the bound the fix exists to eliminate.

## Out of scope

- Changes to finalize behavior or the `## Finalize blocked` contract.
- Broad rewrites of other disposition assertions; rewriting the four existing `docs/` occurrences.
- Pinning or reporting the resolved `grep` across all 63 test files (recommended follow-up only).

## Open questions

- None — resolved at auto-groom (2026-07-27); see the spec's `## Assumptions` block.

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
