<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0249 — Build-worker contract: gate-execution pointer and staging discipline](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0249-build-worker-contract-gate-execution-pointer-and-staging-dis.md)**
<!-- docket:backlink:end -->

# Build-worker contract: gate-execution pointer and staging discipline — design

**Change:** 0249 · **Date:** 2026-08-07 · **Groomed:** autonomously (default-biased self-brainstorm,
critic-gated). Consolidates the killed #0232 (gate posture never reaches workers) and #0238 (staging
unconstrained), per the 2026-08-07 triage.

## Problem

Two verified holes in `skills/docket-build-task/SKILL.md`, both mechanisms of the change-0223
incident:

1. The gate execution posture (0223) lives in `skills/docket-build/SKILL.md` §*Gate execution
   posture* and `skills/docket-build/references/gate-execution.md` — the one place workers never
   read: a worker is dispatched with its task, not its controller's SKILL.md. Workers routinely run
   the full suite as their honest focused verification; on 0223's own build four workers hit the
   foreground ceiling and three stalled, each re-inventing the same wrong answer (background the
   suite, yield, wait for an event a subagent cannot receive).
2. Nothing constrains **staging**. `## Scope` forbids *editing* unrelated work; nothing forbids
   `git add -A` / `git add .` / `git commit -a`, so a worker can sweep another agent's or a human's
   dirty paths into its one commit — exactly how the 0223 double-write started (the woken worker's
   *first* commit swept the replacement's uncommitted files in; 0231 closed only the amend half).

## Design

### Edit 1 — gate-execution pointer (worker contract, `## The cycle`)

One short paragraph appended to `## The cycle`, immediately after the numbered steps (beside step
4's "focused, not the whole suite" note, which is where the tension lives):

- When the narrowest honest verification is still a run that may outlast a single foreground call
  (on this repo, often the full suite), execute it under the gate-execution capabilities in
  [`skills/docket-build/references/gate-execution.md`](../../skills/docket-build/references/gate-execution.md)
  — read it before starting such a run.
- The worker-shaped consequence stated inline, in harness-neutral words: you are a **dispatched
  worker with no resumption channel — never yield to await the run**; observe by blocking (short
  foreground reads of the durable result), the observation is **finite**, and a run still
  unfinished when you stop observing is **not green** — treat it as unverified and fail closed,
  never infer success. (The same conduct `docket-build` clauses 4–6 state for its gate, without
  importing `GATE_OBSERVATION_BUDGET` or the controller's halt vocabulary.)

Pointer, not restatement: the six capabilities, the mitigation, and the per-harness verdicts stay
quarantined in the reference file (house policy, change 0154; learnings:
`restatement-accumulates-its-own-guards`). The pointer targets the **reference file**, not
`docket-build`'s §*Gate execution posture*: the reference is harness-neutral capability text,
whereas the SKILL.md section is written in controller vocabulary (`GATE_OBSERVATION_BUDGET`,
*Halting conditions*, the `halted` BUILD outcome) that a worker must not import — the exact hazard
the size-budget ledger records against pointing workers at controller prose.

### Edit 2 — staging discipline (worker contract, `## Scope`)

One new bullet in `## Scope`, placed adjacent to the existing escalated-worker bullet so the
carve-out stays local:

- **Stage by explicit path, only paths your task changed.** Never `git add -A`, `git add .`, or
  `git commit -a`: the worktree is shared, and a sweep puts work that is not yours into your commit.
- **Observability, defined honestly:** "what your task changed" is defined by the **task contract**,
  not by diffing `git status` — a derived file your task's own command regenerates is yours to
  stage; a dirty path you cannot attribute to your task is not yours — leave it in place and name it
  in `NOTES`.
- **Escalation carve-out, bounded by the task boundary:** an escalated worker is dispatched into a
  worktree already holding the weaker worker's uncommitted changes and is required to account for
  every one of them; an inherited path it revised, replaced, or deliberately kept **within the
  task's scope** is one of its task's paths and is staged normally. An inherited path **outside**
  the task boundary is accounted for but not staged — it takes the leave-and-report posture above
  ("Implement only that task" binds the escalated worker equally). The existing "never discard them
  blindly" bullet is untouched.

### Edit 3 — guards (`tests/test_docket_build.sh`, one change-0249 banner)

Appended under this file's banner-per-change discipline, reusing the existing `## Scope` extractor
(`worker_scope_flat`) and its non-vacuity companion; every assert independently mutation-tested
(remove/reverse the clause, watch it redden) per `AGENTS.md` — an assert never seen red is
decoration. Asserts, keyed on shape not spellings:

1. Gate pointer present in the worker body: the literal path
   `docket-build/references/gate-execution.md` appears, **and** the word-anchored never-yield
   negation (`\b(never|do not)\b … yield`) appears near it — presence of the path alone would
   survive a rewrite that inverts the consequence.
2. Staging rule: word-anchored negation over the sweep forms
   (`\b(never|do not)\b … (git add -A|git add \.|commit -a)`), extracted from the Scope block, not
   file-wide.
3. The explicit-path positive rule: phrase match on "Stage by explicit path" (flattened, per
   `flat()`).
4. The task-contract-not-git-status definition: phrase anchored on "task contract, not" — the
   observability half is the part a lazy rewrite drops first.
5. The escalation carve-out survives the new prohibition: phrase anchored in the Scope block,
   companion to the existing 0231 assert `"You may revise or replace them"` (which must stay
   green untouched).

### Edit 4 — size-budget raise (`tests/test_skill_size_budgets.sh`, same diff)

`skills/docket-build-task/SKILL.md` measures **122 lines / 1087 words** against row `130 1150` —
8 lines / 63 words of headroom; the two edits (~10 lines / ~150 words) do not fit. Raise per the
row's documented rule from the **measured post-edit actual, in-diff** — follow the rule (including
its within-25-words take-the-multiple-after clause), not any pre-build estimate.
The required in-diff rationale: `skills/docket-build-task/` has **no** `references/` tree, and the
candidate home `skills/docket-build/references/gate-execution.md` already holds everything
extractable — this change adds only the pointer and two normative Scope rules that must bind via
wrapper preload (`agents/docket-build-*.md` carry `skills: [docket-build-task]`), so nothing here
can move out. 0224 is concurrently raising the **controller** row (`docket-build/SKILL.md`
325/3000); different row, no conflict.

## Assumptions

**A1 — Pointer target is the reference file, whole file, not a clause excerpt.** Stub default
accepted. Rejected: pointing at `docket-build` §*Gate execution posture* (imports controller
vocabulary — `halted`, `GATE_OBSERVATION_BUDGET` — into a contract whose reader never runs a
controller; the size-budget ledger already records this exact hazard); restating only
"split-never-yield" (restatement is the 0154 anti-pattern, and the split answer is incident lore
while the reference's detach-plus-durable-artifact mitigation is the measured one). *Revised at the
critic gate:* the inline consequence was widened from never-yield alone to never-yield + finite
observation + fail-closed-on-unfinished — the reference states harness capabilities, not agent
conduct, so a bare "observe by blocking" would have converted the measured yield-and-stall failure
into an unbounded silent block; the added words stay harness-neutral (no
`GATE_OBSERVATION_BUDGET`, no controller halt vocabulary).

**A2 — Durable contract rule, not a per-dispatch prompt line.** 0232's open question, resolved
against the dispatch-prompt option: a per-dispatch instruction is unguardable (no file to grep),
depends on every controller remembering it, and dies with the prompt. The contract is preloaded
into all four profile agents — the durable channel already exists.

**A3 — The fix-loop workers need no separate edit.** 0232 asked whether `docket-implement-next`'s
Step-6 fix loop needs the same pointer. Verified: every fix task "runs the `docket-build-task`
contract" (its SKILL.md, Step 6), so both edits reach them through the same preloaded body. Zero
extra surface.

**A4 — Staging rule lives in `## Scope`, not `## The commit`.** 0238 left this open. Chosen:
Scope — staging is conduct in the shared worktree (what you may touch), the 0231 guards already
extract and pin the Scope block, and the escalation carve-out must sit beside the existing
escalated-worker bullet it qualifies. Rejected: `## The commit` (that section governs cardinality
and when a commit may exist; splitting the carve-out from the escalation bullet would put the rule
and its exception in different sections).

**A5 — The escalation carve-out is decidable from available context.** The stub flagged this as the
one clause the critic might push to a human. Decided autonomously — and the critic concurred —
because the carve-out's *permission* is already specified by merged, human-reviewed 0231 contract
text ("inspect and account for every one of them... revise or replace"); this change only names its
staging consequence. *Revised at the critic gate:* the first draft ("paths it revised, replaced, or
deliberately kept are its task's paths") re-licensed the forbidden sweep for inherited out-of-task
strays; the settled wording bounds the carve-out by the task boundary — accounted-for paths
**within the task's scope** are the task's paths, inherited paths outside it take the
leave-and-report posture ("Implement only that task" binds the escalated worker equally; both
bounding texts are live contract prose). Rejected: a blanket "escalated workers are exempt from
staging discipline" (re-licenses `git add -A` for exactly the worker most likely to be in a dirty
shared tree); enumerating inherited paths in the return schema (schema change, three consumers,
out of scope).

**A6 — "What your task changed" is defined by the task contract, not `git status` diffing.** Stub
default accepted. A pure git-observational definition breaks on derived files a task legitimately
regenerates and on pre-existing dirt; the task-contract definition keeps the rule
worker-observable — the same observability bar 0231's spec applied to each of its edits. The
unattributable-path posture is leave-and-report (NOTES), never stage, never clean up: cleaning is
exactly the sweep this change forbids.

**A7 — Guards extend the existing file under a 0249 banner; the 0231 asserts must stay green.**
Matches every prior wave in `tests/test_docket_build.sh`. The carve-out assert is written as a
companion to (not a replacement of) 0231's `"You may revise or replace them"` pin.

**A8 — Mechanical enforcement stays out.** Stub's Out-of-scope accepted: prose + guard only, the
same landing 0231 took. A pre-commit hook or `git add` wrapper is a new enforcement surface with
its own harness matrix — a separate change if the prose rule is ever observed violated.

**A9 — Dependency state: `depends_on: [224]` is `implemented`, PR #174 open, NOT merged
(recorded 2026-08-07).** Designed ahead. Verified against 0224's branch diff
(`main...feat/the-build-gate-contract-never-says-green-red-is-the-exit-code`): it touches
`skills/docket-build/SKILL.md`, `tests/test_docket_build.sh`, and `tests/test_skill_size_budgets.sh`
— **not** `skills/docket-build-task/SKILL.md`. So the collision surface is append-adjacency in the
two test files plus adjacent budget-table rows. Build 0249 only after #174 merges (the manifest
gate already enforces this); at build time re-measure the worker file and re-verify the guard
file's tail before appending the banner.

**A10 — Couplings written to frontmatter.** `depends_on: [224]` stands (same test files in
flight). `related: [231, 253]` — *widened at the critic gate from `[231]`*: 0231 authored the Scope
bullets this change extends and its guards pin phrases these edits must not break; 0253 would
settle/rewrite the very prose-anchored guard house pattern (`flat()`, proximity negations) that
Edit 3's asserts reuse, on the same file. Considered and left in prose: 0257 (edits other guards in
the same file — plain append-adjacency, the class `depends_on: [224]` already illustrates without
an ordering constraint) and 0172 (rewrites `fmv()` there, which none of Edit 3's asserts touch).
0223 stays prose-only context (done and archived).

## Out of scope

- Restating any gate capability or per-harness verdict outside the reference file.
- Any edit to `docket-build/SKILL.md`, `docket-implement-next/SKILL.md`, or the reference files.
- Detection of a stray post-acceptance commit (0231 spec A9 declined it deliberately).
- Hooks/wrappers enforcing staging scope (A8).
- Suite runtime (0227, landed) and the return-schema shape.

## Risks

- The never-yield inline line brushes restatement of `docket-build` clause 4; kept to one clause
  because it must bind workers that skip the pointer read. If 0154's audit later objects, the
  pointer alone remains and the guard's assert 1 is loosened in that change, not this one.
- Post-0224 measurements (budget row, guard-file tail) may drift before this builds; A9's re-measure
  instruction absorbs that.
