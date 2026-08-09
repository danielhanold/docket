---
id: 275
slug: run-gate-has-no-runnable-path-for-slash-command-or-backgroun
title: 'Run gate has no runnable path for slash-command or backgrounded implement-next dispatch'
status: proposed
priority: high
type: fix
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: []
discovered_from: [271]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

The run gate promoted into `AGENTS.md` ("Run gate — verify a dispatched implement-next run before
you report it") assumes one dispatch shape: the session takes a `verify-run --in-progress-ids`
snapshot, dispatches **foreground**, blocks on the return, re-snapshots, and diffs the two to
identify which change the run claimed.

When the run is launched as a backgrounded slash command — `/docket-implement-next 271`, which is
how a human actually starts one — that shape is unreachable. The dispatch happens within the same
user turn that requests it, so there is no point at which the session can take the before-snapshot,
and the run is not foreground. Steps 1–3 are structurally unrunnable on that path.

Observed live on change 0271 (2026-08-09), the first session to load the gate after 0242 created
docket's Claude surface. Only step 4 (`verify-run <id>`) could be run, and only because the agent's
report happened to name the id. Had the run died before reporting, the session would have had no id
to verify and no snapshot diff to recover one from — precisely the silent-failure case the gate
exists to catch.

## What

Define the gate's behavior for slash-command / backgrounded dispatch. Candidate directions, not a
decision:

- Have the session take the before-snapshot when it *observes* a dispatch it did not itself make,
  and treat a missing before-snapshot as a named, degraded mode rather than an unstated gap.
- Give the identification half a fallback that does not depend on the agent's prose — e.g. derive
  the claimed id from metadata written at claim time rather than from a two-snapshot diff.
- Or scope the gate explicitly to foreground dispatch and state what the human should run instead
  on the slash-command path.

Also worth settling: the gate says verify *before* you report, and a session that receives the
agent's completion notification has already been handed the report. The ordering obligation needs
wording that survives that.
