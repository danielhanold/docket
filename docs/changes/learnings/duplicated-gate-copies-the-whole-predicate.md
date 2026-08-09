---
slug: duplicated-gate-copies-the-whole-predicate
hook: "When a second site must AGREE with an existing gate, copy the whole predicate — copying only its threshold leaves a gate that agrees on the easy inputs and diverges on exactly the ones the original was written to exclude."
topics: [design, duplication, gates]
changes: [219, 269]
created: 2026-08-07
updated: 2026-08-09
promotion_state: candidate
promoted_to:
---

## Apply
A gate is rarely its constant. It is a constant *plus* the guards that decide which inputs the
constant is even allowed to judge — ref resolution, ahead-of-base checks, count gates, the
distinctions it draws before answering. When a second site is specified to reach the **same verdict**
as an existing one, the reviewable unit is the whole predicate, and the tempting reading of the spec
("duplicate the threshold") is the one that ships a divergent gate.

The failure is asymmetric and that is what makes it survive review: the two sites agree on every
ordinary input and diverge precisely on the states the original's extra guards existed to exclude.
So the copy fires on the pathological case the original deliberately stays silent about — the exact
inversion of the intent — while every hand-check of a normal input confirms "they agree."

Two tells that the duplication has not been reviewed as a unit:

- **The claim of agreement is load-bearing in prose.** If a spec, a code comment, or a script
  contract sells "the two always agree" as a property a consumer relies on, that sentence is a
  specification and needs a discriminating test, not a reading. Prose asserting agreement is the
  single strongest hint that agreement was assumed rather than established
  ([[verify-the-claim]]).
- **Every fixture makes the two sites agree.** A fixture population built from well-formed inputs
  cannot separate a whole-predicate copy from a constant-only one, so the suite is green under both
  ([[green-suite-untested-branch]]). Build the input the original's extra guards *exclude* and
  assert the copy is silent there.

Also check what the copy **asserts in its output**, not only what it computes: a message that names
a state ("pushed", "merged", "claimed") is a claim, and the copy has to actually probe it. Copying a
predicate that never needed the probe, into a site whose wording promises it, ships a confident
sentence with nothing behind it.

When duplication by value is the accepted design — the second site must stay independent of the
first, offline or dependency-free — the drift cost is real and unguarded by construction; name the
twin in prose at each site and consider a correspondence test over the shared constants and
shapes ([[correspondence-guard-runs-one-way]]).

## War story
- 2026-08-07 (#219, PR #171) — `board-checks.sh`'s new `detect_orphan_pr` was specified to reach the
  same verdict as `docket-status.sh`'s leg C, with the spec, a code comment, and `docket-status.md`
  all selling "the two findings always agree" as load-bearing. The first implementation copied leg
  C's 2h idle floor **and nothing else** — dropping the ahead-of-both-show-ref-verified-bases guard.
  For a claimed change whose run died before its first commit, `git log -1` returned the *base*
  commit's date, comfortably past 2h, so the new leg fired on the 0109 "stopped with nothing built"
  signature that leg C exists to stay silent about. It also asserted the word "pushed" without ever
  probing a remote ref. Neither defect was reachable by the fixtures: the test helper gave every
  branch a real own-commit, and no fixture had a remote at all, so "is pushed" was asserted true
  against branches that were never pushed. Fixed by mirroring the whole predicate — the leg now
  emits three messages, the new arm being `<branch> was never pushed … the run stopped before
  pushing it`. The duplication itself was then recorded as **ADR-0072** with its accepted cost
  stated: nothing links the two implementations, so a future retune of leg C's floor or its base
  handling breaks the agreement silently and no test will say so.
- 2026-08-09 (#269, PR #187) — `sync-agents.sh` grew a new `runner_config_error` gate beside the
  existing `validate_runner_config`. The sibling judges only the **resolved candidate set** — the
  runners some agent actually references; the new gate walked **every registered runner across all
  three config layers**, unconditionally. Same verdict on every ordinary input, and divergent on
  exactly the state the sibling scopes itself to avoid: a typo'd `shim_model` in the user-level
  `~/.config/docket/config.yml`, for a runner no agent in the repo names, would have hard-failed
  `sync-agents.sh` in **every repo on the machine** — a machine-wide outage from a key nothing in
  the repo reads. Caught at whole-branch review, not by the suite, because every fixture configured
  runners it also referenced. Fixed by scoping the gate to the resolved candidate set and
  **extracting** the population primitive rather than copying it, so the two sites cannot drift.
