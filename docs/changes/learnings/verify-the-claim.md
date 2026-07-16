---
slug: verify-the-claim
hook: "A document asserting a fact about another artifact is not an oracle — verify it against the artifact or the RUNNING CODE before acting on it."
topics: [process, review, spec]
changes: [12, 21, 47, 65, 67, 74]
created: 2026-06-12
updated: 2026-07-16
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
- 2026-06-12 → 2026-07-16 (#12 PR #7; #21 PR #34; #47 PR #55; #65 PR #74; #74 PR #82; #67 PR #91 —
  merged, one verify-the-claim family) — A document that asserts a fact about another artifact — a code
  review, a spec, teaching prose — is not an oracle, and it has been flatly false five times. (a) A review
  finding cited a sentence that did not exist in the reviewed file. (b) A spec's stated *rationale*
  for a scope boundary was wrong (it claimed the convention's `.docket.yml` example "does not
  enumerate `finalize:`" — it does), though the boundary itself was sound on other grounds. (c) #74's
  spec claimed `docket.sh bootstrap` in a `STOP_MIGRATE`-shaped repo "exits non-zero and writes
  nothing"; against the real resolver it exits **0**, emits `BOOTSTRAP=STOP_MIGRATE`, and writes
  nothing — so an assert written FROM the spec would have pinned fiction and gone green doing it.
  (d) Prose restating a fact owned by another file drifts, and no sentinel can catch it: #65's README
  asserted which model tier each built-in agent runs at and shipped factually FALSE, with every grep
  green, because a doc sentinel proves a sentence still EXISTS and can never prove it is still TRUE.
  (e) **A CODE COMMENT is a claim too, and #67 broke this rule inside the change that ships it.** A
  fix's own comment warned against a "naive two-pass" unescape and explained why the single-pass was
  required; review swapped the single-pass *for* the two-pass it warns against and the suite stayed
  green. The comment's scenario was unreachable — `_dq_unescape_dquote` is called with the closing
  delimiter already stripped — so for well-formed YAML the two passes are provably equivalent and the
  comment was asserting a danger that could not occur. The single-pass is still correct (it degrades
  predictably on *malformed* input), but the justification was decoration and the fixture set could
  not tell the two apart. Fixed by adding the one discriminating fixture (`"path C:\\" and more"` — a
  bare unescaped quote inside a double-quoted scalar) and correcting the comment. Treat a comment
  explaining why code is correct as an unverified claim: find the input that distinguishes it from the
  alternative it rejects, or delete the claim.
