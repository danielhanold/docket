# Supervisor findings for change 412 — deferred

Owner: [change 412](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0412-forked-implement-next-build-agent-still-backgrounds-the-gate.md).
Related compatibility work: [change 432](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0432-complete-native-codex-runner.md).
This note does not authorize implementing 412 or make it a prerequisite for 432.

## Existing scope

412 proposes a Docket-owned supervisor for repetitive gate observation, durable result
collection, establishment/recovery, deadlines, and duplicate-execution prevention.
It separates monitoring from workflow-consumption ownership. It is not a general
implementation-workflow controller.

## Evidence from 431, 2026-09-16

Earlier acceptance reports describe foreground tool yields whose live session identity
or eventual protocol output was not consumed. That overlaps directly with 412's
start/collect and transport requirements.

The latest run reported:
- Scoped build passed at d0c75eb4938096dcfb331289ac371cc1b217684d.
- Results were committed and published at ed80a72a33ce535ce8c3ab639b6090cb3ba9abc8.
- Final certification returned WAITING; the continuation reported a missing single-use
  handoff token and no available scope parent capability.
- A durable halt was recorded at d9c7e3c32f930f006d06cd8b9c9e9248c4da181d.

This is a reported boundary failure, not a diagnosis of whether the token was never
produced, omitted, lost in transport, or misinterpreted. Change 432 must trace that
existing contract before suggesting architectural changes.

Local diagnostic:
 /Users/homer/dev/docket-0425-launch-kit/candidate/acceptance-active-validation/resume-431-scope/control/continuation-431-final-blocked.md

## Deferred design questions

Should durable terminal-result observation be repeatable without consuming a workflow
handoff credential? Which mutation/consumption operations still require exclusive authority?
Evaluate these within 412's design and shared simplification, not as unapproved changes
inside the Codex compatibility repair.

Record model activations for gate polling separately from harness-session collection;
a change that only renames polling does not establish supervisor success.

## Investigated final boundary, 2026-09-16

The [432 investigation](native-codex-runner/0432-codex-runner-handoff.md) corrects the latest run's causal
claim. Saved command and tool responses show a complete `gate.drive.handoff` document
with nonempty `drive.generation`, followed by a successful `run.gate-claim` response
with nonempty top-level `generation`. The agents misinterpreted those responses; the
final failure was not a lost live shell handle or an absent serialized token.

This particular failure does not establish that a supervisor is necessary for Codex
compatibility. It also does not invalidate 412's earlier yield/collection and polling
cost evidence. A future supervisor still needs callers that understand the authority
returned by collection and continuation; a durable terminal record alone does not
make the consuming workflow complete.

Keep separate measurements for process observation, harness-session collection,
receipt interpretation, and workflow completion. Preserve exclusive consumption and
stale-owner fencing while evaluating repeatable read-only observation. Any new collect
operation, changed cadence, or separation of monitoring authority remains 412 work.
No supervisor or acceptance run was launched during this investigation.
