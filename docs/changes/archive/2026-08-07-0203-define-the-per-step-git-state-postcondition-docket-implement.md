---
id: 203
slug: define-the-per-step-git-state-postcondition-docket-implement
title: Define the per-step git-state postcondition docket-implement-next now names but never states
status: done
priority: medium
type: docs
created: 2026-08-03
updated: 2026-08-07
depends_on: []
related: [113, 202, 211, 212]
discovered_from: [113]
adrs: []
spec: docs/superpowers/specs/2026-08-05-per-step-git-state-postcondition-design.md
plan: docs/superpowers/plans/2026-08-06-per-step-git-state-postcondition.md
results: docs/results/2026-08-06-define-the-per-step-git-state-postcondition-docket-implement-results.md
trivial: false
auto_groomable: true
branch: feat/define-the-per-step-git-state-postcondition-docket-implement
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/163
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-05-per-step-git-state-postcondition-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-05-per-step-git-state-postcondition-design.md) |
| Plan | [2026-08-06-per-step-git-state-postcondition.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-06-per-step-git-state-postcondition.md) |
| Results | [2026-08-06-define-the-per-step-git-state-postcondition-docket-implement-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-06-define-the-per-step-git-state-postcondition-docket-implement-results.md) |
<!-- docket:artifacts:end -->

## Why

Change 0113's prose rider added this clause to `docket-implement-next` §5:

> the step is not complete until its git-state postcondition holds

`git-state postcondition` occurs exactly once across `skills/` — in that new sentence. Step 5 states
no postcondition to hold, and neither does the *Terminal disposition* section nor
`docket-convention`. An agent that already misread this step is now told to satisfy a named
condition with no referent, which is weaker than the enumerated obligations around it.

The gap is worse where it matters most. Both incidents known when this stub was written stopped at
the **Step 4/5 seam** — Step 4 is where the plan file is written, committed, and `plan:` recorded —
and Step 4 received no rider at all. Its closest thing to a postcondition is the prose "Record the
plan path in `plan:` per the field-write rule", which is exactly the sentence two runs are known to
have narrated past.

0113's own thesis is that step completion must be **verifiable rather than narrated**. Its
deterministic half delivers that; its prose half left the narration in place and added an undefined
term. The change file itself already argued the check has to be "per-step and uniform" — that
reasoning was applied to the oracle and not to the prose.

**Reframed by the 0206 incident (2026-08-05).** That run completed Steps 0–5 — four build commits,
green suite, `plan:` recorded — and then ended at the **Step 5/6** boundary with the branch unpushed
and no PR. **Step 5's postcondition was fully satisfied at the moment it died.** So a postcondition
set that says only what each step must have *achieved* would have certified that run as healthy.
This stub's original premise — that stating the condition within each step is the fix — is therefore
insufficient on its own, and the settled design says so out loud rather than shipping six
independent green lights.

Surfaced by the deep-rung review of change 0113 and left unfixed by merge-time judgment, because
settling what a step's postcondition *is* for each of Steps 2 through 7 is a design decision, not a
cleanup.

## What changes

State a per-step git-state postcondition for `docket-implement-next` Steps 2–7 **once, as a table**
placed immediately before *Terminal disposition*, and repair the §5 clause by pointing at it. Full
design in the linked spec; the settled shape:

- **One table, not six inline riders** — per-site riders are the shape that skipped Step 4 in 0113.
- **A governing sentence**: the conditions are cumulative, and once a change is claimed (absent a
  `halted` disposition or a Step-3 kill) the only postcondition that also completes the *run* is
  Step 7's. This is a disclaimer, not a detector — it removes the false confidence the table would
  otherwise create. The transition out of a step stays 0212's (run-level disposition obligation) and
  0211's (`aborted-run` leg C, after the fact).
- **Step 4** gets a row like every other step — the uniformity *is* the treatment — stating the
  Step-3 SHA-compare, the plan file **and its backlink stamp** on `feat/<slug>`, and `plan:` landed
  on `metadata_branch`, verified by reading git and never by the sub-skill's report.
- **No new enforcement**: no new field, status, run record, or runtime mechanism. A step boundary is
  not a fact about git. The mechanical companion is a proximity-scoped prose-presence guard in
  `tests/test_loop_continuation.sh`.
- **Named residual**: on a clean-review path Step 6 produces no git state of its own, so its row
  reduces to Step 5's. Recorded, not papered over.

Word budget is a live constraint. **Re-measured 2026-08-06 (post-0212):**
`skills/docket-implement-next/SKILL.md` is 145 lines / 3844 words against a `150 / 3850` row — 5
lines and 6 words of margin, not the 6/72 the stub recorded pre-0212. Compress the touched region
first, re-measure, raise from the *measured* actual.

## Sequencing

0212 edits the adjacent section of the same file and the same budget row — a **semantic** conflict a
clean git merge will not resolve. Recommended order: **0212 first** (its pointer degrades gracefully
either way). Recorded as a recommendation, not `depends_on:`, because a hard gate would park this
change until 0212 reaches `done`. Whichever lands second re-measures the merged file and re-derives
the budget row.

**Resolved (2026-08-06):** 0212 landed first, as recommended. 0203 is therefore the second lander
and **owes the re-measure**: the budget row is now `150 3850` and the file measures 145/3844 — 5
lines and **6 words** of margin. A raise is now effectively certain, and its in-diff justification
must be written rather than assumed.

## Out of scope

- Reversing ADR-0044 or re-litigating call-site pre-specification.
- Changing the `aborted-run` check or its predicates — the deterministic oracle is not what is
  under-specified here.

## Reconcile log

### 2026-08-06 — claimed and reconciled against current `docket` + `main`

Re-verified every fact the spec's assumption 11 orders re-checked at build time. All hold, with
three updates:

1. **0212 landed** (`archive/2026-08-05-0212-…`), as the spec recommended. So did **0211**
   (`aborted-run` leg C exists in `scripts/board-checks.sh`) and **0202**. The sequencing question is
   settled and 0203 is the second lander that owes the post-rebase re-measure.
2. **The budget is much tighter than the spec recorded.** 0212 raised the row `145/3800 → 150/3850`
   and consumed the raise in its own diff. Measured now: **145 lines / 3844 words** — 5 lines, 6
   words. "Compress first, raise only if it still exceeds" is unchanged as a *sequence*, but the
   spec's implicit hope that compression alone might absorb the table is no longer plausible; plan
   for a raise with a written in-diff justification naming `references/edge-paths.md` and why the
   table cannot live there.
3. **0212's pointer is now concrete.** `SKILL.md` reads "`advanced` is claimable only when **Step
   7's postcondition** holds; that postcondition is Step 7's to state, not this section's", and
   `tests/test_loop_continuation.sh` asserts that pointer (`advanced.{0,80}Step 7|Step 7.{0,80}advanced`)
   while deliberately asserting no postcondition wording — the test comment names 0203 as the
   surface that supplies it. **That regex is a live constraint on any compression of that sentence:
   `Step 7` must stay within 80 characters of `advanced`.**

Verified unchanged: `git-state postcondition` still occurs **exactly once** across `skills/` (§5,
`SKILL.md:74`). `scripts/board-checks.sh` reads **neither** the build-evidence record **nor**
`## Reconcile log` presence — the spec's correction stands, so table rows 3, 5 and 6 do name state no
existing check consumes. `tests/test_loop_continuation.sh` (63 lines) still carries one `mktemp`
probe over one matcher and no file-exists/non-empty anchor, so the spec's instruction to borrow the
fuller anchor set from `tests/test_role_skill_self_description.sh` (file-exists + non-empty anchor, a
live presence assert through the same read, and a mutation probe **per new matcher**) applies as
written.

Scope unchanged; no design invalidation. Proceeding to plan.
