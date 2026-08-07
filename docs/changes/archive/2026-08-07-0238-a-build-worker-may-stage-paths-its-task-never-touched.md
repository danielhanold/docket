---
id: 238
slug: a-build-worker-may-stage-paths-its-task-never-touched
title: A build worker may stage paths its task never touched
status: killed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [231]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced by the whole-branch review of change 0231, which widened
`docket-build-task`'s amend prohibition to cover any commit including the worker's own. The
reviewer observed that 0231 closes the *amend* half of the change-0223 double-write and leaves the
*staging* half untouched.

**Opportunity** — the worker contract says nothing about **what a worker may put into its commit**.
`## Scope` forbids touching "unrelated user work", which reads as *edit*, not *stage*, and names
*user* work rather than another agent's or a concurrent process's. Nothing forbids `git add -A`,
`git add .`, or `git commit -a`, so a worker can sweep in every dirty path in the shared worktree —
including files it never wrote and never read. In the 0223 incident the damage was done by exactly
this: the woken worker's **first** commit swept the replacement's uncommitted files in; the amend
that followed was only the de-duplication.

**Independent value** — stands with change 0231 fully reverted. Staging discipline is ordinary
worker hygiene: it also covers a human's stray edits sitting in the worktree, a concurrent
process's output, and generated artifacts — none of which involve a presumed-dead worker at all.
It converts "the worker committed something nobody asked for" from an undetectable event into a
contract violation a guard can key on.

**Boundary** — one clause in `skills/docket-build-task/SKILL.md` (either `## Scope` or
`## The commit`) plus its guard in `tests/test_docket_build.sh`. It must carve out the **escalated
worker**, which is explicitly dispatched into a worktree already holding the weaker worker's
uncommitted changes and is told to "inspect and account for every one of them... revise or
replace". That carve-out is the design question this needs and is the reason it cannot be a
one-line edit. Deliberately leaves alone: detection of a stray post-acceptance commit (change
0231's spec, assumption A9, declines that on purpose), and any controller-side check.

**Reason for deferral** — change 0231's scope is four named prose edits settled at groom time, and
its spec argues each one by observability. A staging rule needs the same treatment: whether it
belongs in `## Scope` or `## The commit`, how it reads against the escalation inheritance, and
whether "stage only what your task changed" is even worker-observable when a task legitimately
regenerates a derived file. Authoring that inside 0231 would be shipping un-designed normative
contract text on a contract whose whole point is that its rules are argued before they bind.

## Why killed

Consolidated into #0249 at the 2026-08-07 backlog triage: the staging-discipline clause lands with #0232's gate-posture pointer in the same contract file.
