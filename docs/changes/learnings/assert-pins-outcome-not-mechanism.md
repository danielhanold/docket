---
slug: assert-pins-outcome-not-mechanism
hook: "An assert on the outcome alone (\"it failed\") is satisfied by every unrelated way of failing — pin the mechanism with positive evidence the fixture is already collecting."
topics: [testing, guards, mutation]
changes: [228, 208]
created: 2026-08-07
updated: 2026-08-11
promotion_state: candidate
promoted_to:
---

## Apply
A guard that asserts only a coarse outcome — a non-zero exit, a non-empty stderr, "the command
failed" — passes for every reason the region can fail, not for the one the test is about. A failed
`cd`, a missing environment variable, a typo in the fixture, or a broken harness all satisfy it, so
the assert survives changes that destroy the behavior it was written to protect. It pins *that*
something went wrong; it never pins *what*.

Two moves:

1. **Assert the mechanism, not the outcome.** Look for the positive evidence the fixture already
   collects and is not asserting on — a runtime log, a recorded argv, a captured stdout. Pin that.
   `[ "$(cat "$runtime_log")" = "tests/test_*.sh" ]` says the literal glob reached the runtime and
   failed there; `[ "$status" -ne 0 ]` says only that the subshell was unhappy. Fixtures usually
   collect this evidence for isolation and then discard it — the assert is nearly free.

2. **Prefer the behavioral pin to the text-shape grep.** A guard that greps the source for a
   spelling (`! grep -qi -- "nullglob"`) can only ever enumerate the spellings its author imagined,
   false-reddens against a maintainer who hardens the code *correctly* in an unanticipated way, and
   drags in regex-portability hazards ([[shell-portability]], `grep` is ugrep locally). Once the
   mechanism is pinned behaviorally, *any* way of suppressing it — a shell option, a guard clause,
   an array rewrite — empties the evidence and reddens the assert regardless of how it is written.
   Delete the text guard rather than re-spelling it, and record why in a block comment so the next
   maintainer does not re-add it.

The check that catches this before review: name the assert's satisfying set out loud. If it
includes states you would not call a pass, the assert is on the outcome, not the mechanism.

Sibling to [[guards-are-code]] (a guard that never saw red is decoration) and
[[assert-detects-removal-not-replacement]] (assert the state you removed, not the one you added).

## War story
- 2026-08-07 (#228, PR #167) — the change added a failure accumulator to finalize's auto-detect
  suite loop and guarded it in `tests/test_configured_bash_finalize.sh`. The empty-suite case
  asserted `empty_status -ne 0`, which was satisfied by any failure inside the subshell — a failed
  `cd`, a broken fragment, an unset `DOCKET_BASH_PATH` — not specifically by the literal
  `tests/test_*.sh` glob reaching the runtime and failing there. The fixture was already writing
  `empty_runtime_log` purely for isolation and never asserting on it. Review swapped in
  `[ "$(cat "$empty_runtime_log")" = "tests/test_*.sh" ]` and, in the same pass, deleted the
  `! grep -qi -- "nullglob"` text guard as strictly weaker than the behavioral pin it now
  duplicated. Two asserts out, one mechanism pin in; net assert count unchanged.
- 2026-08-11 (#208, PR #195) — **The sharpening: a diagnostic literal pins a mechanism only if it
  is unique to that mechanism.** Hardening runner-dispatch added two independent gates over the
  same argument. The gate-1 assert did the right-looking thing — it pinned a literal out of gate 1's
  diagnostic rather than a bare non-zero status — yet the mutation probe found it stayed **green
  with gate 1 deleted**, because gate 2 fires on the same input and its diagnostic contains the
  same literal. Two independent gates whose behavior on the overlapping input is
  *indistinguishable* is a vacuity trap that no amount of reading finds: both asserts look
  mechanism-pinned, both are green, and the pair covers what looks like two properties while
  actually covering one. Only deleting gate 1 and watching nothing redden surfaces it. So the
  satisfying-set question has a second half — not just "what else could make this pass?" but "what
  else in this file *emits this same string*?" The fix is to pick evidence the sibling gate cannot
  produce (gate 1's own distinct wording, or an input gate 2 does not reject at all), and the
  general rule is that adding a gate beside an existing one obliges you to mutation-test **both**,
  because the new one can retroactively make the old one's assert vacuous. Two of the change's
  three vacuous asserts came from this same overlap shape. Kin to
  [[duplicated-gate-copies-the-whole-predicate]] and [[guards-are-code]].
