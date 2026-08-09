---
id: 257
slug: clear-the-residual-review-findings-from-0193-and-0201
title: 'Clear the residual review findings from 0193 and 0201'
status: proposed
priority: low
type: chore
created: 2026-08-07
updated: 2026-08-09
depends_on: []
related: [253, 260]
discovered_from: [197, 204]
adrs: []
spec: docs/superpowers/specs/2026-08-07-clear-the-residual-review-findings-from-0193-and-0201-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-clear-the-residual-review-findings-from-0193-and-0201-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-clear-the-residual-review-findings-from-0193-and-0201-design.md) |
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

Settled by the linked design spec (groomed 2026-08-07, critic-gated), eight edits E1–E8: two README roster cells ("opt-in" → "shipped default"), the convention sketch comment ("superpowers default" → "shipped default", live surface only), a non-vacuity companion assert through a factored shared extractor in the review test, removal of the two strictly-entailed `!=` asserts in the config test, the opt-back-in guard anchored to the `### docket-build` README section (0253-compatible shape), the anti-deadlock override rationale restated with its rule in `references/gate-failure.md`, a whitespace-class + read-back clause on AGENTS.md's anchoring bullet, and the bounded rationale-loss sweep of the four 0201-compressed SKILL.mds.

Coupling: `related: [253]` — 0253 rewrites prose-anchored guards in the same two test files and its build-time site re-derivation would sweep the guard E5 anchors; no ordering constraint, whichever lands second reconciles. 0249/0224/0172 collisions on the same files are plain append/textual adjacency, left in prose (mirroring 0249's own spec).

## Out of scope

- 0200's board-checks bundle (separate, larger, re-scoped).
- Any new guard machinery beyond the anchoring fixes named; helper hoisting (0252); guard-pattern policy (0253); producer-pipe hygiene (0172).
