# Supervisor findings for change 412 — deferred

Owner: [change 412](../active/0412-forked-implement-next-build-agent-still-backgrounds-the-gate.md).
Related compatibility work: [change 432](../active/0432-complete-native-codex-runner.md).
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
