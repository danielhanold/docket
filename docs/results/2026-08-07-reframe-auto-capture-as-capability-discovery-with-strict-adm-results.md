<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0226 — Reframe auto-capture as capability discovery with strict admission gates](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0226-reframe-auto-capture-as-capability-discovery-with-strict-adm.md)**
<!-- docket:backlink:end -->

# Reframe auto-capture as capability discovery with strict admission gates — results
Change: #226 · Branch: feat/reframe-auto-capture-as-capability-discovery-with-strict-adm · PR: <url> · Plan: docs/superpowers/plans/2026-08-07-auto-capture-capability-discovery.md · ADRs: none

## Findings

**The open question resolved by measurement, then reopened by a fix.** The change asked whether the
convention summary's reframe fit inside its remaining 6 lines / 46 words. It did — the rewrite
landed at 340/5837 against `345 5850`, so Task 2 raised nothing. Review then correctly called the
13-word margin a defect in its own right (the BUDGETS block's own near-zero rule is 25 words), and
the Finding-1 fix raised the row to `345 5900` at a measured 341/5853. Both facts are worth keeping:
the reframe *was* byte-neutral as designed, and byte-neutral was not good enough once the file sat
that close to its ceiling.

**A plan-specified guard was decoration.** The plan's site-C assert was
`grep -qiE "unavailable|\*\*no\*\*" <<<"$c_row"`. The routing row's own *Branch + fix loop* column
independently says `**no**`, so the alternation stayed satisfied with the fix-in-branch exemption
deleted — the mutation proved it. Split into two cell-scoped asserts using `[^|]` to keep each
pattern inside the cell that owns its claim. This is the `assert-detects-removal-not-replacement`
learning firing on a guard that was *written* by a plan rather than by a worker, which is a slightly
new shape of it: plan-supplied test code is unverified code.

**Stacked ERE gaps hang instead of failing — a hazard the suite does not detect.** A guard written
as `a[^.]{0,160}b[^.]{0,60}c` backtracks catastrophically under ugrep on *non-matching* input, which
is exactly the mutated file the assert exists to catch. It hung for minutes rather than reddening.
Both bounds are under 255, so `tests/test_grep_portability.sh` — which guards the BSD ceiling — has
nothing to say about it. Every pattern this branch adds now uses a single bounded gap. Captured as
change #233 because two other test files carry the shape.

**A generic `/^## /` awk terminator cut a section short.** `## What a captured discovery says`
embeds a fenced example whose first line is the literal `## Why` that `mint-stub.sh` requires, so
the plan's generic terminator ended the slice before the five fields it exists to guard. Fixed with
a named terminator (`/^## Per discovery/`), and review then correctly asked for an existence assert
on that terminator, since a rename would silently widen the slice to end-of-file with every assert
still green.

**Two guards passed green through the very rewrite they existed to demand.** The `admission gate`
assert was satisfied by the pre-change summary's unrelated "waits at the human's groom **gate**",
and the `mint-stub` contract assert was satisfied by the bare token while the rule-bearing
"start with `## Why`" clause could be rewritten to say the opposite. Both tightened.

**Review's most valuable finding was about reachability, not content.** The reframe's headline
addition is an instruction to actively search for capability — but the only trigger loading the
reference fired *after* something had already surfaced, so the six categories were read only by a
reader who no longer needed them. Every test asserted presence; none asserted reachability. The
convention's drill-down trigger now fires at each mint site on arrival. This is the
`specified-but-unreachable` learning, and it is worth noting that a fully green suite and a
faithful-to-spec implementation coexisted with the change's central instruction being dead prose.

No decision in this build was non-obvious enough to warrant an ADR: the two design calls (scoping
the gates to sites A and B, widening the drill-down trigger) both implement acceptance criteria the
spec already stated.

## Follow-ups

- **#233 — Guard against stacked-gap ERE patterns that hang instead of failing** (auto-captured,
  `chore`). `tests/test_dispatch_capability.sh` and `tests/test_docket_build.sh` both carry the
  stacked-gap shape and are outside this change's diff.
- `skills/docket-convention/references/auto-capture.md` sits at 123/1202 against `130 1250` — 7
  lines and 48 words of margin. Comfortable, but this file has now been raised four times (0201,
  0218, a 0218 review fix, and this change); the next raise is worth pausing on rather than taking
  reflexively.
- The plan's Step 6/7 restore idiom `git checkout -- <file>` is wrong while the edit is uncommitted
  — it silently reverted the rewritten reference to HEAD mid-mutation-test and produced a
  meaningless 40-failure reading. Later tasks used a `cp` backup. Worth carrying into how mutation
  steps are written in future plans.
