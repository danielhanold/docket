---
id: 212
slug: an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
title: An inlined role skill's terminal stop ends the whole run — scope docket-build's stop and enforce the run disposition
status: done
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: [96, 113, 154, 203, 211]
discovered_from: [113]
adrs: [69]
spec: docs/superpowers/specs/2026-08-05-inlined-role-terminal-stop-scoping-design.md
plan: docs/superpowers/plans/2026-08-05-inlined-role-terminal-stop-scoping.md
results: docs/results/2026-08-05-an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco-results.md
trivial: false
auto_groomable: true
branch: feat/an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/161
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-05-inlined-role-terminal-stop-scoping-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-05-inlined-role-terminal-stop-scoping-design.md) |
| Plan | [2026-08-05-inlined-role-terminal-stop-scoping.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-05-inlined-role-terminal-stop-scoping.md) |
| Results | [2026-08-05-an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-05-an-inlined-role-skill-s-terminal-stop-ends-the-whole-run-sco-results.md) |
| ADRs | [ADR-0069](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0069-mode-conditioned-clause-discriminates-on-provenance.md) |
<!-- docket:artifacts:end -->

## Why

On 2026-08-05 the `docket-implement-next` fork building change 0206 ran Steps 0–5 in full — four
build commits, clean tree, 78-file suite green, `plan:` recorded — then ended its turn at the
**Step 5/6 boundary**, leaving the branch unpushed with no review and no PR.

Its closing report is the diagnosis:

> **Build disposition: complete — the plan is executed.** Review is not mine; stopping here.

That is verbatim `docket-build`'s own output contract. `skills/docket-build/SKILL.md:11` reads *"Then
you stop — review is not yours."* The fork invoked `docket-build` **inline via the Skill tool** and
then dispatched the profile agents itself, so the build role's terminal instruction was loaded into
the same context as the driver's step sequence — and outranked it. The run adopted the sub-role's
scope boundary as its own terminal boundary.

**This is 0096/0113's class with a different sub-skill.** 0096 and 0113 both diagnosed an invoked
skill's hand-off language ending the caller's run, and 0113 hardened the Step-4 call site against
`superpowers:writing-plans`. Nobody swept the *build* skill for the same shape. `docket-build`'s stop
sentence is correct for a dispatched build role and hazardous for an inlined one, and the skill
cannot know which it is.

**A second, independently checkable tell.** `docket-implement-next` requires every run to end by
declaring exactly one of four **run** dispositions — `advanced`, `contended`, `drained`, `halted` —
so a driver keys on the outcome instead of parsing prose. This run declared a **build** disposition.
The wrong disposition vocabulary in the final report is, on its own, proof the run never reached its
terminal step, and it would have caught all four observed incidents (0109, 0194 twice, 0206), each of
which closed with a step-scoped or invented disposition rather than one of the four.

**Why 0113's Step 5 rider did not prevent it.** Step 5 already carries 0113's language — *"Proceed
through the build — the deliverable is the executed plan, never the decision about how to execute
it"* and *"the step is not complete until its git-state postcondition holds."* Both were satisfied.
The build ran, the plan was executed, and Step 5's postcondition held. The rider guards **within** a
step; the failure was the **transition** out of it. No obligation anywhere states that the run is not
over until a run disposition is declared and its git state proven.

## What changes

Two prose levers, both aimed at the step-to-step transition rather than at any one step's contents.
Settled by the linked spec (auto-groom, 2026-08-05).

**Lever 1 — scope the stop to the role, and sweep for the class.** The hazardous construct is a
**second-person directive in a body that can be loaded into a caller's context**: terminal stops
("Then you stop — review is not yours") and, worse, second-person prohibitions (`docket-review`'s
`## Conduct` never-writes / never-commits / never-dispatches, which inline-loaded at Step 6 would
forbid the driver's own dispatches and Step 7's metadata writes). The sweep criterion is
**loadability, not role-skill membership** — `docket-adr` and `docket-status` are not role skills but
are run inline under the convention's Tier A, and `docket-build-task` reaches a caller's context by
wrapper preload. Six files get a per-file verdict (edit or recorded no-hazard):
`docket-build`, `docket-review`, `docket-adr`, `docket-brainstorm`, `docket-status`,
`docket-build-task`. The clause is **two-sided and conditioned on invocation mode** — a wrapper
injects the same body, so a one-sided "the caller continues" would be read by dispatched subagents
whose turn genuinely does end. `docket-brainstorm` Step 3 is the house pattern: it stops, then names
the owner of the next step. A **positive-presence**, proximity-scoped guard test in the
0194/0198/0199 style backs it; a negative vocabulary grep is rejected as paraphrase-evadable.

**Lever 2 — bind the run disposition.** `docket-implement-next`'s *Terminal disposition* section
gains an obligation on the **agent**, not only guidance to a driver: the run does not end until
exactly one of `advanced` / `contended` / `drained` / `halted` is declared, and a final report
declaring any other disposition vocabulary is by construction an aborted run. `advanced` is claimable
only when Step 7's postcondition holds — stated **by pointer**, never defined here.

**No new runtime mechanism for lever 2** — the stub's open question, settled. The final report is
model output: `board-checks.sh` is git-only by contract, `/loop` is vendored, and a wrapper cannot
read a subagent's final message. A git-readable run record was weighed and rejected — it duplicates
the `aborted-run` ledger (0113's incoherence leg plus 0211's leg C already cover all four observed
signatures) and the convention's Tier-C precedent is explicit that an abandoned run gets **no new
status, no new field**. Lever 2 is prose plus a prose-presence guard in
`tests/test_loop_continuation.sh`.

**Why this is not the fix that already failed.** 0113's rider was already driver-body prose at the
moment of action and still lost to the sub-skill's sentence. Lever 1 works by **removing the
conflicting instruction at its source** rather than out-ranking it; lever 2 is deliberately the
smaller half.

**Size budgets are a live constraint** on every swept file (re-measured 2026-08-05 at reconcile:
`docket-build` 262/2369 against 265/2400 — **3 lines, 31 words**; `docket-build-task` 111/959 against
115/1000 — 4 lines, 41 words). Sequence: compress the touched section, re-measure, raise only rows that
still exceed — and note that `skills/docket-build/` and `skills/docket-review/` have **no
`references/` tree**, so change 0201's raise-justification rule must argue against creating one
rather than name an existing file.

## Out of scope

- The deterministic oracle half — extending `aborted-run` with a built-but-not-delivered leg — which
  is change 0211.
- Defining the per-step git-state postconditions Step 5 names but never states; that is change 0203.
  0212 points at Step 7's postcondition and never defines it. The two collide on
  `skills/docket-implement-next/SKILL.md` and on a `tests/test_skill_size_budgets.sh` budget row —
  a **semantic** conflict a clean git merge does not resolve, so whichever lands second re-derives
  the row from the post-rebase actual.
- Editing any vendored skill body; the ADR-0044 call-site pre-specification is the remedy there.
- Dispatching the build role as a subagent instead of invoking it inline — the one option that
  removes the mechanism rather than counter-instructing it, weighed and recorded in the spec as a
  structural redesign outside a fix change.
- Reversing ADR-0044 (pre-specification at the call site) or re-litigating ADR-0024 (`context: fork`).

## Reconcile log

### 2026-08-05

Reconciled against `origin/main` @ `18195d91` and `origin/docket`. The design holds unchanged; the
only drift is measured numbers, now corrected in `## What changes`.

**Re-verified the per-file verdict table (the spec's own instruction to re-check it at build time).**
All six hazard readings still resolve on `origin/main`:

- `skills/docket-build/SKILL.md:11` — *"Then you stop — review is not yours."* present. **Edit.**
- `skills/docket-review/SKILL.md` — H1 line 6 ("read the branch, return findings, stop"), `## Conduct`
  lines 28–32 (never writes / never commits / never checks out / never dispatches / never runs the
  suite), and line 94's abort-and-report. **Edit** — and the prohibition class is the larger half, as
  the spec argues.
- `skills/docket-adr/SKILL.md` — no terminal stop and no second-person prohibition anywhere in the
  body. **No-hazard confirmed**, to be recorded as a verdict rather than skipped silently.
- `skills/docket-brainstorm/SKILL.md:65-66` — `STOP AT THE SPEC` immediately followed by the owner of
  the next step. **No-hazard confirmed**; it stays the house pattern the clause is modelled on.
- `skills/docket-status/SKILL.md:35` — *"surface the stderr diagnostic and stop rather than
  improvising a fix"* present and unscoped; the file is inlined at `docket-implement-next` Step 0
  under Tier A. **Verdict still open at build time**, as designed.
- `skills/docket-build-task/SKILL.md:86` — *"Return exactly one of three outcomes"* present, plus the
  prohibition class. **Verdict still open at build time.**

**Budget drift — the one substantive correction.** The spec measured `docket-build` at 260/2348
(5 lines, 52 words of margin). Change 0202 has landed since; the actual is now **262/2369 — 3 lines
and 31 words**. Thirty-one words will not absorb a two-sided mode-conditioned clause, so the spec's
"docket-build's 52-word margin may absorb the clause outright" is no longer a live possibility: plan
on compressing `## Output` / the line-11 sentence first, then raising the row from the *measured*
post-compression actual under the file's rounding rule. Every other row is unchanged from the spec's
table. `docket-adr` (78/1280 against 86/1408) and `docket-brainstorm` (76/622 against 84/692) have
room if a recorded no-hazard verdict turns out to want a line of prose.

**Dependencies and collisions unchanged.** 0203 and 0211 are both still `proposed` and unbuilt, so
0212 lands first on `skills/docket-implement-next/SKILL.md` and on the budget table; the re-measure
obligation transfers to whichever of them lands second, exactly as assumption 8 states. The guard
tests the change extends both exist on `origin/main`: `tests/test_role_skill_self_description.sh`
(the shape lever 1's new guard copies) and `tests/test_loop_continuation.sh` (lever 2's home).

No scope change, no kill, no invalidation.
