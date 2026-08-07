<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0231 — A presumed-dead build worker can wake and race its own replacement in one worktree](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0231-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla.md)**
<!-- docket:backlink:end -->

# Design — A presumed-dead build worker can wake and race its own replacement in one worktree

Change 0231 · autonomous groom (docket-auto-groom) · 2026-08-07

## Problem

A dispatched build worker that has not returned is indistinguishable, from the controller's
position, from one that is slow. On change 0223 the controller treated a ~15-minute silence as
death, discarded the worker's uncommitted tree, and re-dispatched a fresh worker into the **same
feature worktree**. The first worker was never stopped. It woke, wrote into the two files the
replacement was editing, and committed — sweeping the replacement's work into its own commit — then
amended to de-duplicate. The end state was accidentally correct.

Nothing in `docket-build`, `docket-build-task`, or `docket-implement-next`'s fix loop forbids the
move that opened the race: **discarding a dispatched worker's tree and dispatching a fresh worker
for the same task.**

## Decision (summary)

**Never discard-and-re-dispatch.** Whatever a controller believes about a worker that did not
return cleanly, it may not discard that worker's tree and dispatch a replacement for the same task:
it halts, preserving the worktree exactly as it stands.

The rule attaches to an event the controller can actually observe — **control returns without a
schema-valid outcome** — never to a notion of elapsed patience. A `docket-build` controller
dispatches **foreground** and blocks; change 0223 hardens that ("may never yield: it observes by
blocking instead"). A blocked controller has no clock and observes no silence, so "the worker is
dead" is not a state it can enter. This is the same epistemics docket already commits to: an
unobserved result establishes nothing (learning `capability-absence-needs-a-failed-attempt`), and
a caller may not read a completion signal — or its absence — as a fact about a child (ADR-0024).

The delta over today's contract is therefore narrow, and deliberately so: the existing halting
bullets already fire on the observable events. What is missing is the explicit prohibition on the
recovery move, and one worker-side rule about amending.

## What changes

1. **`skills/docket-build/SKILL.md` — extend the existing *A worker return is malformed or
   unverifiable* halting bullet** (do not mint a parallel condition with an unobservable trigger).
   Today it ends "Never re-dispatch a task to repair its own return." Add the sibling prohibition:
   never discard the worktree and dispatch a fresh worker for that task either — a worker you did
   not observe return cleanly may still be running, and a woken worker writes into the same
   worktree as its replacement. Halt, naming the task and the worktree, and leave the worktree
   untouched.

2. **`skills/docket-build/SKILL.md` § *Dispatching a task*** — one sentence beside the existing
   "never dispatch two workers concurrently": the rule binds a controller that **believes** the
   first worker is gone exactly as it binds one dispatching deliberately. That belief is the case
   the current wording does not reach.

3. **`skills/docket-build-task/SKILL.md` § *Scope*** — widen the existing line "Never rewrite,
   amend, or revert earlier task commits" to **any** commit, including one this worker just made:
   correct by adding a commit, never by amending. This is worker-observable and it binds the woken
   worker, which the post-return phrasing considered and rejected in A4 does not.

4. **`skills/docket-implement-next/references/fix-loop.md`** — one sentence stating the same
   prohibition in the fix loop's **own** disposition vocabulary (abort-and-report; the change stays
   `in-progress` with `claimed_at` refreshed). Not a bare pointer: the fix loop's controller is
   `docket-implement-next` Step 6, which dispatches `docket-build-task` workers directly and never
   loads `docket-build`'s SKILL.md, so importing `docket-build`'s `halted` build-outcome
   disposition would be wrong.

5. **Guard tests** in `tests/test_docket_build.sh` covering all three prose edits (1/2, 3, and 4 —
   that file already knows the `fix-loop.md` path), mutation-tested per `AGENTS.md`.

## Assumptions

Every decision below was defaulted autonomously; each records the rejected alternatives. Two trades
are called out explicitly for the human reading this later: **A4** removes ordinary amend-cleanup
from workers, and **A6** places this change behind a human merge of PR #166.

**A1 — Posture: prohibit the recovery move, keyed on an observable event.**
Options: (a) liveness probe before discard; (b) worktree write lease a second worker refuses to
cross; (c) make discard-and-re-dispatch illegal.
Chose **(c)**. (a) is unimplementable at contract altitude — there is no harness-neutral way for a
controller to observe whether a dispatched subagent is still running, and a foreground controller is
blocked anyway. (b) builds machinery (a lease file, its staleness policy, its interrupted-run
self-heal — the `transient-resource-lifecycle` learning) to make safe a state that must not be
entered, and cannot bind the woken worker, which was dispatched before the lease existed.
**The trigger is "control returned without a schema-valid outcome", never elapsed time** — an
undefined patience threshold in a normative contract would be unactionable and would invite exactly
the improvisation this change closes. Cost accepted: a genuinely dead worker now halts the run
instead of self-healing; the halt preserves the worktree and the change stays `in-progress`.

**A2 — Prose rule, not a mechanism.** No new field, status, file, or script. Rejected: a dispatch
heartbeat in the checkpoint ledger — `BUILD_CHECKPOINT` defaults to `false`, so anything keyed on it
is absent on the default path. The stronger reason: no mechanism can bind a worker already
dispatched.

**A3 — Stated in each controller's own vocabulary, not pointed at.** `docket-build` owns the
controller rule; `fix-loop.md` gets one sentence with **its** disposition (abort-and-report), because
Step 6's fix loop dispatches workers itself and never loads `docket-build`. Rejected: a bare pointer
(imports the wrong disposition) and a new shared reference file on the `references/task-routing.md`
precedent (two sentences do not earn a file, and every skill here is near its size budget). The
duplication is one sentence in two different vocabularies, which is the shape the `fix-loop.md`
owner already uses for shared rules.

**A4 — Widen the worker's amend prohibition to its own commits.** The stub's incident had the woken
worker commit and then amend **inside its own turn**, so a "never amend after emitting your return"
clause would not have reached it — and "after a rival has written to the same files" is not
worker-observable (the same objection that kills the lease in A1). Widening the existing *Scope*
line to any commit, including its own, is observable, absolute, and binds the woken worker.
**Trade, stated:** a worker loses ordinary amend-cleanup and must correct by adding a commit. The
existing escape ("if the task text prescribes more than one commit, the plan wins") is unaffected,
as is the *Scope of these prohibitions* sentence about self-invocation. Rejected: deferring the
amend half to its own stub — a one-line edit with a whole build, review, and PR around it.

**A5 — `docket-rebase-resolver` / `docket-integration-repair` are out of scope.** Verified: both are
foreground single-dispatch agents in the feature worktree, and `docket-finalize-change` has no
discard-and-re-dispatch path (conflict → abort-and-report; red after at most two attempts →
abort-and-report). The hazard is not reachable there today. Do **not** claim that a rule stated in
`docket-build` "reads onto" finalize — finalize neither loads nor references `docket-build`; that
sentence must not reach the built text. Rejected: adding a fourth pointer into
`docket-finalize-change` — restatement against a path that does not exist, on a different review
surface.

**A6 — `depends_on: [223]`, written into the manifest.** 0223's branch rewrites the exact
*Halting conditions* list this change appends to (+57 lines in that file) and introduces the
false-completion rule this design reasons from; building before it merges means editing text that is
not on the integration branch. **Consequence, stated:** a dependency is satisfied only at `done`, and
0223 is `implemented` with PR #166 open at the human merge gate, so 0231 is not build-ready until a
human merges it. That is the intended behavior, not a side effect. It is written into frontmatter,
not left in spec prose. 0224 is deliberately *not* a dependency despite touching the same file: it
edits § *The build gate*, a different section, and none of its text is a premise here.

**A7 — File-collision couplings as `related: [223, 224, 232]`.** Collisions, all additive:
`skills/docket-build/SKILL.md` (0223 § *Halting conditions*, 0224 § *The build gate*),
`skills/docket-build-task/SKILL.md` (0232 propagating the gate execution posture),
**`tests/test_docket_build.sh`** (0224 and 0232 will also append asserts there), and
**`tests/test_skill_size_budgets.sh`** (0223's branch already rewrites its rows and rationale
block). Per the `concurrent-edits-compose-at-rebase` learning, keep every edit additive and
reconcile by intent. Rejected: promoting any of these to `depends_on` — none of their text is a
premise of this one's.

**A8 — Guard tests in the existing `tests/test_docket_build.sh`, covering all three prose edits.**
That file already asserts over both the worker and the controller and already knows the
`fix-loop.md` path, so guarding edit 4 is cheap. Per `assert-detects-removal-not-replacement` the
assert must detect removal of the clause; per `phrase-grep-over-wrapped-prose` it must collapse
whitespace so a re-flow does not redden it. **Size budgets are a build hazard here and the builder
must plan for them:** on the post-0223 base `skills/docket-build/SKILL.md` runs ~312 lines against a
`320` row (~8 lines of headroom for a bullet extension plus a sentence), `docket-build-task/SKILL.md`
119/125, `fix-loop.md` 175/180. Raising a row in `tests/test_skill_size_budgets.sh` is permitted and
carries this repo's documented rationale-comment ritual (see the block above that table) — follow it
rather than compressing prose to fit. Rejected: a new test file — same contract surface.

**A9 — No detection, and none in scope.** If a woken worker commits after the controller has already
accepted the replacement's `COMPLETE`, **nothing in the contract detects it**: the stray commit
resolves and is an ancestor of HEAD, so the existing *A failed attempt left a commit* bullet — scoped
to the escalation decision on a `NEEDS_ESCALATION`/`BLOCKED` return — never fires. Do not claim
otherwise in the built text. That is accepted, because A1's prohibition makes the state unreachable
in the first place. Rejected: a controller-side check that the branch tip is still the SHA it
accepted before starting the next task — plausible, but it is new detection machinery for an
unreachable state and belongs in its own stub if the human wants belt-and-braces. Rejected also: a
de-duplication or reconciliation procedure, which would be authoring a repair for that same state.

## Out of scope

- The yield defect that caused the original stall (change 0223).
- Reducing suite runtime (change 0227).
- Any change to `docket-finalize-change`'s dispatches (A5).
- Detection of a stray post-acceptance commit (A9).

## Risks

- **A worker that did not return cleanly now halts the run** instead of being replaced. Accepted per
  A1; the halt preserves the worktree and the change stays `in-progress`, so a human resumes.
- The rule is prose, enforceable only by the guard tests and by review — docket's standing posture
  for contract rules, and the reason the guards are mutation-tested.
- Size-budget rows are tight on the post-0223 base (A8); expect to raise one with its rationale
  comment.
