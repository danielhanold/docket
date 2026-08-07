---
id: 236
slug: suppressed-execution-handoff-still-ends-run-at-plan
title: A suppressed execution hand-off still ends the run at the plan — 0113 recurrence
status: proposed
priority: high
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: [96, 113, 109]
discovered_from: [113]
adrs: [24, 44]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0044](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md) |
<!-- docket:artifacts:end -->

## Why

Change 0096 stopped an autonomous run from *asking* the plan skill's interactive
execution-hand-off question. Change 0113 was supposed to stop a *suppressed* hand-off from
silently ending the run, by making step completion verifiable rather than narrated. Both are
`done` and merged (PR #102, PR #154). The failure recurred anyway.

On 2026-08-07, `/docket-implement-next 231` was dispatched as a forked subagent
(agentId `a735ab380e777c9e1`). It wrote the plan and returned, verbatim:

> Plan complete and saved. Per the directive I'm stopping at the plan file rather than offering
> an execution choice — execution is already resolved.

It then reported task-completed with no build commits, no build on the branch, and no PR. So the
suppression half worked — the hand-off question was never posed — but the agent read suppression
as authorization to **end the autonomous run at the plan**, which is precisely the mode 0113 set
out to make impossible. The run only continued after a human resumed the agent with an explicit
"execute the plan" message; that resume then built normally, which suggests nothing was blocking
the build itself.

This is the third instance in the 0096 → 0113 family (0109 is the fourth data point), so it is a
recurrence of a fixed bug rather than a new class. It matters because it converts an autonomous
drainer into a half-run that reports success: the caller sees `completed`, the board shows the
change still `in-progress`, and only a human reading the transcript notices the gap.

## What changes

Close the plan→build boundary so that a run which suppresses the hand-off cannot also stop there.
Whatever verifiable-step-completion machinery 0113 installed needs to either cover this boundary
or actually detect this stop as a false completion — grooming should determine which.

## Out of scope

- Re-litigating 0096's suppression design; suppression worked correctly here.
- Patching the vendored `superpowers:writing-plans` plugin (established non-goal in this family).
- Change 231's own content — it is unrelated and was merely the carrier for this observation.

## Open questions

- Does 0113's verifiable-step-completion gate simply not cover the plan→build boundary, or does it
  cover it and fail to fire?
- Is the fork's stop observable to the caller at all, given a forked skill's completion report is
  unreliable in both directions? If not, the fix may belong on the caller's verification side
  rather than in the worker's step ritual.
- Do the three prior fixes share a root cause that a fourth point-fix would also miss?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
