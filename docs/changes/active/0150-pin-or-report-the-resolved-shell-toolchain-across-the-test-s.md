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
related: [151]
discovered_from: [130]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
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

## Auto-groom blocked

**2026-07-28** — autonomous grooming abstained. A full design was drafted; the critic **measured**
the population the design's central guard would cover and found it self-defeating, and the repair
is a maintainer scope call rather than a defaultable one.

### The undecidable decision

The draft's part 2 — a static guard requiring an absolute `/usr/bin/<tool>` at portability-sensitive
call sites — rested on the claim that the repo *already practises* that convention, so the guard
would merely formalise it. Measured across all 66 test files, the convention is **7 absolute call
sites in 2 files**, all `grep`; `sed`, `awk`, `date`, and `readlink` have **zero** anywhere in the
suite.

Scoping the guard to "files already using the absolute form for that tool" therefore yields:

| tool | in-scope files | bare command-word sites in them |
|---|---|---|
| grep | **2** of 66 | **~169** (`test_docket_config.sh` 86, `test_board_checks.sh` 83) |
| sed / awk / date / readlink | 0 | 0 |

Degenerate and huge at once: 3% of the suite in scope, four of five tools permanently empty, and
essentially every site inside the two in-scope files is a day-one violation. Worse, the file the
whole change descends from — `tests/test_grep_portability.sh` — has **no** absolute call site (its
`/usr/bin/grep` is in a comment), so the guard would never see the very file whose bug motivated it.

The three ways out are all maintainer decisions:

1. Rewrite ~169 call sites across the repo's two largest test files — a large mechanical change
   nobody asked for.
2. Design a computed "this call's *behavior* is the thing under test" predicate — what ADR-0050
   actually demands, and genuinely undesigned.
3. Drop the guard and ship the reporting half alone.

### What a human should supply

- The ruling among those three.
- If (1) or (2): confirmation that a large mechanical rewrite of `tests/test_docket_config.sh` and
  `tests/test_board_checks.sh` is acceptable, given three other active changes edit the former.

### Settled and verified, ready to re-use on re-arm

**Ground truth, all confirmed against the repo:**

- **66** test files, each invoked standalone. **No suite runner**, no `tests/lib/`.
- **There is no CI** — no `.github`, `.gitlab`, `circleci`, `travis`, `Makefile`, `justfile`, or
  `Taskfile` on either `main` or `docket`. `finalize.gate: local` with no `finalize.test_command`,
  so the merge gate auto-detects and runs the suite locally.

**Open question 2 is answered and should not be re-litigated.** "Should CI run the suite under both
a GNU and a BSD toolchain rather than pinning either?" is **not a question about the suite** — there
is no CI at all, so it is a proposal to stand up CI, an infrastructure and cost decision for the
maintainer and a separate change.

**Open question 1 is answered: report, never pin.** A global `PATH` prepend is rejected on change
0130's own recorded reasoning — its guard is deliberately a *static source scan*, not a behavioral
probe, because "on Linux `/usr/bin/grep` is GNU grep and accepts the bound, so a behavioral
assertion would be a platform-dependent false failure." A global pin generalises that false-failure
hazard across the suite while making the maintainer's machine *less* representative. Note honestly,
for whoever re-arms this: **per-site absolute-path discipline is itself a pin, statically enforced
instead of PATH-enforced** — the draft rejected pinning in one section and mandated it in another,
and any revived design must own that rather than repeat it.

**The reporting half is sound and buildable on its own:** a `tests/lib/toolchain-report.sh` sourced
by toolchain-sensitive files, printing the resolved path and version of `grep`, `sed`, `awk`,
`date`, `readlink`, gating nothing, permanently. Lift the implementation from 0130's existing block
verbatim — including its capture-then-here-string discipline, since a producer feeding an
early-exiting consumer under `pipefail` can take SIGPIPE and become an intermittent 141 — and
replace that block with a call to the helper so there is one implementation. Do **not** mandate all
66 files source it, and do **not** assert the resolved `grep` *is* `/usr/bin/grep`: that is a pin
wearing an assert, and it would redden exactly the contributors the report exists to make visible.

**No suite runner.** Introducing one would make it the de-facto entry point for 66 files and for
finalize's auto-detect — a structural change to how this repo is tested, decided as a side effect of
a portability report. If a runner is wanted, propose it on its own merits.

**Couplings.** `related: [151]` — change 0151 edits `tests/test_docket_config.sh` in the 0132/0126
region adjacent to the five absolute `grep` sites, which is the guard's single largest in-scope
file. Changes 0152/0153 concern the `runtime.bash` toolchain for docket's own *scripts*, a different
axis from the tests' PATH tools; do not conflate them.

### Recommendation

Keep, and lean toward option 3 — ship the reporting half, drop the guard. The reporting half is
cheap, non-gating, verified, and independently useful; the guard as conceived cannot prevent
recurrence and cannot be scoped without a decision the drain may not make.

