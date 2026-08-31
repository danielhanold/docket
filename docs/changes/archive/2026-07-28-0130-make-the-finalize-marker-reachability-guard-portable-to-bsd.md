---
id: 130
slug: make-the-finalize-marker-reachability-guard-portable-to-bsd
title: Make the finalize marker reachability guard portable to BSD grep
status: done
priority: medium
created: 2026-07-22
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [116]
adrs: []
spec: docs/superpowers/specs/2026-07-27-bsd-grep-interval-portability-design.md
plan: docs/superpowers/plans/2026-07-28-bsd-grep-interval-portability.md
results: docs/results/2026-07-28-make-the-finalize-marker-reachability-guard-portable-to-bsd-results.md
trivial: false
auto_groomable: true
branch: feat/make-the-finalize-marker-reachability-guard-portable-to-bsd
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/133
blocked_by:
reconciled: true
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-27-bsd-grep-interval-portability-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-27-bsd-grep-interval-portability-design.md) |
| Plan | [2026-07-28-bsd-grep-interval-portability.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-28-bsd-grep-interval-portability.md) |
| Results | [2026-07-28-make-the-finalize-marker-reachability-guard-portable-to-bsd-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-28-make-the-finalize-marker-reachability-guard-portable-to-bsd-results.md) |
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

## Reconcile log

### 2026-07-28

Re-validated the change and its spec against `origin/main` at reconcile time. Every measured fact
the design rests on still holds — nothing dropped, no scope adjustment needed.

- **The bug is still live and still singular.** A tracked-file scan of `origin/main` under
  `/usr/bin/grep` finds exactly **one** ERE interval above 255 outside `docs/`:
  `tests/test_finalize_disposition.sh:186`, unchanged and still reading
  `"where the reason surfaces.{0,600}appends the .{0,4}## Finalize blocked"`.
- **The 255 bound is genuinely unreachable by shrinking.** The matched prose is still a single line
  — `skills/docket-finalize-change/SKILL.md:168` — with both anchors (`Where the reason surfaces.`
  … `and appends the \`## Finalize blocked\` marker`) intact and separated by more than 255
  characters. A1's unbounded within-line `.*` remains the right shape; the mutation proof in the
  spec's §5 is still the thing that shows the assertion bites.
- **The guard file is a fresh path.** `tests/test_grep_portability.sh` does not exist on
  `origin/main`; no other test claims the grep-portability invariant. No file-collision coupling
  with any open branch.
- **A5's `docs/` exclusion is still load-bearing and still sized as designed.** The tracked
  population is 643 paths, 164 of them outside `docs/` — a healthy scan population for the
  collapse sentinel, and confirmation that excluding `docs/` does not gut the walk.
- **A6 holds.** `depends_on` stays empty; the reconcile found no dependency or file collision that
  would gate this change.
- **A8 holds so far.** No new ADR-worthy trade-off surfaced at reconcile; step 6 re-evaluates if
  the build argues one.

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
