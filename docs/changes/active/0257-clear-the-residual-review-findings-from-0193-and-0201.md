---
id: 257
slug: clear-the-residual-review-findings-from-0193-and-0201
title: 'Clear the residual review findings from 0193 and 0201'
status: proposed
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [197, 204]
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
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Consolidates #0197 and #0204 (2026-08-07 triage): the two surviving small "clear the residue" changes of the review-finding-clearance class (their siblings 0200 and 0196 are larger and stay separate).

Verified 2026-08-07 — #0197's five findings from 0193's merge (PR #152), all present:

1. `README.md:653` and `:655` still label `docket-build`/`docket-review` "opt-in", contradicted by `README.md:759` ("Change 0193 ended that: … shipped cross-harness defaults").
2. `skills/docket-convention/SKILL.md:44` — the config sketch comment still says "unset key = the superpowers default shown", stale for build/review.
3. `tests/test_docket_review.sh:785` — an absence assert with no live non-vacuity companion through the same `awk` extractor.
4. `tests/test_docket_config.sh:461,463` — two `!=` asserts entailed by the `=` asserts directly above them (:458-459). Default disposition: remove.
5. `tests/test_docket_build.sh:694-695` — the README/SDD opt-back-in guard is a whole-file unanchored substring.

#0204's surviving items (item 2 — the auto-capture mint-site consequence — was already restored by 0226 into `skills/docket-convention/SKILL.md:276-278`; drop it):

1. The finalize explicit-id override kept its rule but lost the anti-deadlock rationale: `skills/docket-finalize-change/SKILL.md:164` has a different rationale, and `references/gate-failure.md`'s `## Finalize blocked` section (:26-31) has no occurrence of "deadlock"/"never be finalized". Restore the one-sentence why.
2. (absorbed 0214) `AGENTS.md:25-40`'s frontmatter-edit bullets omit the whitespace-class half — no `[[:blank:]]*`-not-`\s*` guidance, no read-back-after-write instruction. Add it (own bullet or a clause on the anchoring bullet — editorial default: a clause).
3. Sweep the other 0201-compressed files for the same rationale-loss class while there (bounded: the four files 0201 touched).

## What changes

The eight concrete edits above: two README cells, one convention sketch comment, one non-vacuity companion assert, two entailed-assert removals, one anchored guard, one restored rationale sentence, one AGENTS.md clause — plus the bounded 0201-file sweep.

## Out of scope

- 0200's board-checks bundle (separate, larger, re-scoped).
- Any new guard machinery beyond the anchoring fixes named.
