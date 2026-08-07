---
id: 232
slug: the-gate-execution-posture-never-reaches-the-build-workers-t
title: The gate execution posture never reaches the build workers that also run the full suite
status: killed
priority: medium
type: docs
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [223]
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

Change 0223 stated a gate execution posture in `docket-build` — the controller's single full-suite
gate must survive a foreground boundary, record its outcome durably, establish completion from that
artifact, observe within a finite budget, and fail closed. It deliberately scoped itself to the
**gate**, and `docket-finalize-change` cites it by reference.

But the controller is not the only agent in the loop that runs the whole suite. Every
`docket-build-task` worker runs verification before it commits, and on this repo that verification
is routinely the full suite — the worker contract requires the change be verified, and for a
cross-cutting contract edit there is no smaller honest check. `docket-build-task` says nothing about
how to execute a run that outlasts a foreground call.

The cost was measured on change 0223's own build, before the posture existed to cite: **four
separate workers hit the ceiling and three stalled.** Each independently invented the same wrong
answer — background the suite, yield, wait for a completion event that a subagent has no channel to
receive. One had to be discarded and its task rebuilt from scratch. The working answer, also
re-derived independently each time, was to split the suite into two sub-ceiling foreground halves;
by the end of the run even that was not enough and the second half needed splitting again.

The posture now exists and is written down. It is written down in the one place the workers do not
read: a worker is dispatched with its task, not with its controller's SKILL.md.

## What changes

Propagate the execution posture to the worker contract. Almost certainly by reference rather than
restatement — `skills/docket-build/references/gate-execution.md` already holds the capabilities and
the per-harness evidence, and change 0154 exists to clean up restatement, so a pointer from
`docket-build-task` plus a line in its verification section is the likely shape.

Consider whether the fix-loop workers in `docket-implement-next` need the same pointer; they run the
same contract.

## Out of scope

- Re-stating the six capabilities anywhere. The quarantine file is the single source.
- Reducing suite runtime — that is change 0227.

## Open questions

- Does a worker need the whole posture, or only the "split, never yield" operational consequence?
- Should the controller pass the suite-execution instruction in the dispatch prompt instead, so the
  worker contract stays silent? That trades a durable rule for a per-dispatch one.

## Why killed

Consolidated into #0249 at the 2026-08-07 backlog triage: one-clause sibling of #0238 in the same worker contract file and guard file; depends on 0224's merge (same test file in flight).
