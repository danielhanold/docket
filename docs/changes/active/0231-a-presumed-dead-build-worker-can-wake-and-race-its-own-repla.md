---
id: 231
slug: a-presumed-dead-build-worker-can-wake-and-race-its-own-repla
title: A presumed-dead build worker can wake and race its own replacement in one worktree
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: [223]
related: [223, 224, 232]
discovered_from: [223]
adrs: []
spec: docs/superpowers/specs/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-design.md
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
| Spec | [2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-a-presumed-dead-build-worker-can-wake-and-race-its-own-repla-design.md) |
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
