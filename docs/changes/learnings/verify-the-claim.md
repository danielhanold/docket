---
slug: verify-the-claim
hook: "A document asserting a fact about another artifact is not an oracle — verify it against the artifact or the RUNNING CODE before acting on it."
topics: [process, review, spec]
changes: [12, 21, 47, 65, 74]
created: 2026-06-12
updated: 2026-07-14
promotion_state: retained
promoted_to:
---

## Apply
Verify a claim against the artifact or the RUNNING CODE before acting on it — byte-diff a
review's quoted sentence; RUN the command whose behavior a spec describes before encoding it in an
assert; and write prose asserting a tier, count, default, or behavior against the CODE (cite the
line), never against sibling prose that may already have drifted. Treat prose restating a
configurable value as a drift surface from the day it ships. When a claim is false but its
conclusion still defensible, keep the conclusion, write the test to the OBSERVED behavior, and
record the discrepancy in the results file — never silently override a spec's scope boundary
mid-build; leave the re-scope to the human. Reject false positives with evidence.

## War story
- 2026-06-12 → 2026-07-14 (#12 PR #7; #21 PR #34; #47 PR #55; #65 PR #74; #74 PR #82 — merged, one
  verify-the-claim family) — A document that asserts a fact about another artifact — a code review, a
  spec, teaching prose — is not an oracle, and it has been flatly false four times. (a) A review
  finding cited a sentence that did not exist in the reviewed file. (b) A spec's stated *rationale*
  for a scope boundary was wrong (it claimed the convention's `.docket.yml` example "does not
  enumerate `finalize:`" — it does), though the boundary itself was sound on other grounds. (c) #74's
  spec claimed `docket.sh bootstrap` in a `STOP_MIGRATE`-shaped repo "exits non-zero and writes
  nothing"; against the real resolver it exits **0**, emits `BOOTSTRAP=STOP_MIGRATE`, and writes
  nothing — so an assert written FROM the spec would have pinned fiction and gone green doing it.
  (d) Prose restating a fact owned by another file drifts, and no sentinel can catch it: #65's README
  asserted which model tier each built-in agent runs at and shipped factually FALSE, with every grep
  green, because a doc sentinel proves a sentence still EXISTS and can never prove it is still TRUE.
