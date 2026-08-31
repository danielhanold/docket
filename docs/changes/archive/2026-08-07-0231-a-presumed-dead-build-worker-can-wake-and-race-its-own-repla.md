---
id: 231
slug: a-presumed-dead-build-worker-can-wake-and-race-its-own-repla
title: A presumed-dead build worker can wake and race its own replacement in one worktree
status: done
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: [223]
related: [223, 224, 232]
discovered_from: [223]
adrs: []
spec: docs/superpowers/specs/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-design.md
plan: docs/superpowers/plans/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-plan.md
results: docs/results/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-results.md
trivial: false
auto_groomable: true
branch: feat/a-presumed-dead-build-worker-can-wake-and-race-its-own-repla
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/170
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-design.md) |
| Plan | [2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-plan.md) |
| Results | [2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-results.md) |
<!-- docket:artifacts:end -->

## Why

A build worker that stalls is indistinguishable from a build worker that is slow, and docket has no
protocol for resolving the difference. On change 0223 this produced a real double-write.

Task 5's worker backgrounded the full suite and yielded to await a completion event that never
reaches a subagent. After ~15 minutes with no commit, the controller followed the only available
posture — treat it as failed, discard its uncommitted tree, re-dispatch fresh. The original worker
was not actually stopped. It woke, wrote its own citation and its own assert group into the same two
files the replacement worker was editing, and committed (`1be89816`), sweeping the replacement's
appended block in: the test file shipped with assert group (8) duplicated, seven asserts repeated
under identical descriptions. It then amended (`e1e64002`) to de-duplicate. The net state happened
to be correct; nothing in the system made that likely.

Three separate hazards compose here:

- **No liveness check before discard.** `docket-build`'s halting conditions and
  `docket-implement-next`'s fix loop both assume a returned worker is a finished worker. Neither has
  a rule for "presumed dead, actually running."
- **No worktree write lease.** Two workers sharing one worktree is forbidden by prose
  ("never dispatch two workers concurrently") and by nothing else. The prose binds the controller
  that dispatches deliberately; it does not bind a controller that dispatches believing the first
  worker is gone.
- **A resumed worker may amend.** The amend rewrote a commit another agent's work was already
  inside. `docket-build` forbids escalating onto a stray commit but says nothing about a worker
  amending a commit it authored after a rival has written to the same files.

## What changes

The settled design is in the spec. Four prose edits plus their guards:

- **`skills/docket-build/SKILL.md`** — extend the existing *A worker return is malformed or
  unverifiable* halting bullet with the sibling prohibition: never discard the worktree and dispatch
  a fresh worker for that task. The trigger is the observable event — control returned without a
  schema-valid outcome — never elapsed time.
- **`skills/docket-build/SKILL.md` § *Dispatching a task*** — one sentence extending "never dispatch
  two workers concurrently" to a controller that *believes* the first worker is gone.
- **`skills/docket-build-task/SKILL.md` § *Scope*** — widen "never rewrite, amend, or revert earlier
  task commits" to **any** commit, including one this worker just made; correct by adding a commit.
- **`skills/docket-implement-next/references/fix-loop.md`** — the same prohibition in the fix loop's
  own disposition vocabulary (abort-and-report, `claimed_at` refreshed), because Step 6 dispatches
  workers itself and never loads `docket-build`.
- **Guards** in `tests/test_docket_build.sh` over all three prose surfaces, mutation-tested.

Depends on change 0223, which rewrote the same *Halting conditions* list and introduced the
false-completion rule this design reasons from. 0223 reached `done` on 2026-08-07 (PR #166 merged as
`fd4d14f4`), so that text is on the integration branch and this dependency is satisfied.

## Out of scope

- The yield defect that caused the stall (change 0223).
- Reducing suite runtime (change 0227).
- `docket-finalize-change`'s resolver/repair dispatches — no discard-and-re-dispatch path exists
  there.
- Detection of a stray post-acceptance commit; the prohibition makes that state unreachable.

## Open questions

Resolved at groom time (see the spec's `## Assumptions`):

- *Is a liveness probe available to a controller?* No — a foreground controller blocks and has no
  clock; presumed-dead is not an observable state. The rule keys on a return without a schema-valid
  outcome and forbids the recovery move.
- *Does the hazard reach `docket-rebase-resolver` / `docket-integration-repair`?* Not today — both
  are single foreground dispatches and finalize has no discard-and-re-dispatch path. Out of scope.

## Reconcile log

### 2026-08-07 — build claim (docket-implement-next)

Re-read the change, its spec, `related: [223, 224, 232]`, and the four target files on
`origin/main` (tip `f7fb123f`). **No scope change; design holds as written.**

- **Dependency satisfied, verified.** 0223 is archived `done` (PR #166, merged `fd4d14f4`). Its
  post-0223 *Halting conditions* list — including the *A worker return is malformed or
  unverifiable* bullet ending "Never re-dispatch a task to repair its own return." — is on the
  integration branch. The spec's A6 gate has been passed; its inline `> Resolved 2026-08-07` note
  is accurate.
- **Every anchor the spec names still exists verbatim.** `docket-build/SKILL.md` § *Dispatching a
  task* still reads "never dispatch two workers concurrently"; `docket-build-task/SKILL.md`
  § *Scope* still reads "Never rewrite, amend, or revert earlier task commits"; `fix-loop.md`
  already states abort-and-report with `claimed_at` refreshed in its own vocabulary, so edit 4
  lands beside existing prose rather than importing a foreign disposition.
- **A8's size-budget hazard re-measured on the current base and it is real.** Actual/budget:
  `docket-build/SKILL.md` 312/320 lines, 2876/2950 words; `docket-build-task/SKILL.md` 119/125,
  1051/1100; `fix-loop.md` 175/180, 1779/1850. The spec's figures were exact. Headroom is 5-8
  lines per file, so at least one row in `tests/test_skill_size_budgets.sh` will likely need the
  documented rationale-comment raise rather than prose compression.
- **Collision partners are all still unbuilt**, so nothing has landed ahead of this change:
  0224 and 0232 are `proposed` needs-brainstorm; 0234 is `proposed`. Change 0190 is `in-progress`
  under a concurrent agent and touches `docket-build`'s gate/evidence surface — unmerged, so this
  branch cuts clean from `origin/main`; keep every edit additive and reconcile by intent at rebase
  per the `concurrent-edits-compose-at-rebase` learning.
- **Auto-capture: nothing minted.** The one adjacent idea this pass surfaced — A9's belt-and-braces
  controller check that the branch tip is still the accepted SHA — is explicitly deferred by the
  spec to a human's call ("belongs in its own stub if the human wants belt-and-braces") and guards
  a state A1's prohibition makes unreachable. It fails the materiality bar as discovered work; it
  is reported here rather than minted.

### 2026-08-07 — resume re-reconcile (integration branch advanced)

The run halted at the Step 6 fix loop and was resumed by a human. `origin/main` had advanced
`f7fb123f` → `8c9cf509` in the interim, so the resume-safety rule required a fresh pass.
**Design still holds; no scope change.**

- **All three target files are byte-unchanged** between the old and new tip —
  `skills/docket-build/SKILL.md`, `skills/docket-build-task/SKILL.md`, and
  `skills/docket-implement-next/references/fix-loop.md`. Every anchor this change edits is exactly
  where the first pass found it, so the built commits need no rework.
- **Change 0234 merged** (the gate-execution split) and is the only overlap. It moved
  `tests/test_skill_size_budgets.sh` and `tests/test_docket_review.sh`, and split
  `skills/docket-build/references/gate-execution.md` into a non-blocking evidence sibling. This
  branch touches none of the gate-execution surface. The budget rows are **disjoint** — 0234 raised
  `gate-execution.md` and added `gate-execution-evidence.md`; this change raises
  `docket-build/SKILL.md`, `docket-build-task/SKILL.md`, and `fix-loop.md` — so the table composes
  at rebase per `concurrent-edits-compose-at-rebase`.
- **0234's merge strengthened the case for review finding 1.** `tests/test_docket_review.sh` now
  carries roughly thirty `fix-loop:`-prefixed asserts over `fix-loop.md`, with its own `FIX`
  variable and non-vacuity floor — so it is unambiguously that file's guard owner, and the six new
  fix-loop guards this change added to `tests/test_docket_build.sh` belong there instead.
- **Changes 0226, 0211, 0217, 0173 also landed.** 0226 reframed the convention's auto-capture
  section as gated capability discovery; that reframe governs this run's own capture decisions and
  was read before triaging the review findings. None of it touches this change's scope.
- **Auto-capture: still nothing minted** from this pass. The two gaps this run's own halt exposed —
  the fix loop naming no disposition for a malformed fix-worker return, and no sanctioned way for a
  later run to clear an abandoned worker's staged files — are real and are reported to the human in
  the run report, but both are contract-design questions about the very rules this change is
  editing. Filing them from inside this branch would pre-empt the human's groom gate on work whose
  boundary is not yet settled, so they fail admission gate 4 (clear, defensible boundary) and are
  reported rather than minted.
