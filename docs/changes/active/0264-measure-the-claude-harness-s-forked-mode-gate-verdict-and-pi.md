---
id: 264
slug: measure-the-claude-harness-s-forked-mode-gate-verdict-and-pi
title: 'Measure the claude harness''s forked-mode gate verdict and pin a surviving launch shape'
status: proposed
priority: medium
type: docs
created: 2026-08-08
updated: 2026-08-18
depends_on: []
related: [315]
discovered_from: [200]
adrs: []
spec: docs/superpowers/specs/2026-08-09-claude-forked-mode-gate-verdict-design.md
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
| Spec | [2026-08-09-claude-forked-mode-gate-verdict-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-claude-forked-mode-gate-verdict-design.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while running change 0200's build gate. `docket-build`'s *Gate execution
posture* directs a dispatched build role to start the suite so it outlives the initiating
foreground call, then observe a durable result artifact by blocking. Following that literally with
the reference's own recommended mitigation — `nohup setsid <runner> … >/dev/null 2>&1 </dev/null &`
plus `disown`, every stream redirected to a durable location — the gate **never started at all**:
no output file was ever created, no `run-tests.sh` process existed a few seconds later, and the
durable status file stayed absent. The run only completed once it was relaunched through the
harness's own native background mechanism. Per the reference's own rule ("a run in which the gate
never started is inconclusive, not `incompatible`"), this is an inconclusive probe, not a verdict —
which is exactly why it needs a real one.

**Opportunity** — `references/gate-execution.md` records claude's verdict as
`supported — interactive session, two foreground calls; forked mode unmeasured`, and forked/
dispatched mode is **docket's own default path**: this role is invoked inside
`docket-implement-next` Step 5, which is itself dispatched. So the one mode docket actually runs in
is the one mode no verdict covers, and the first agent to hit it has to improvise a launch shape
mid-gate. What does not exist today is a measured forked-mode verdict plus a launch shape known to
survive that mode.

**Independent value** — stands with change 0200 fully reverted. It is a property of docket's gate
contract and its harness evidence, not of anything 0200 touched. Every future `docket-build` run on
this harness — every change, not this one — starts a gate the same way.

**Boundary** — re-probe the `claude` harness in **forked/dispatched** mode using the reference's
one-variable-per-run ladder; record the verdict, the launch shape that survives, and the shape that
does not, in `references/gate-execution.md` and its evidence file. If a launch shape that survives
forked mode is found, name it in the reference the way the codex row already names its
race-free-new-session condition. Stops there: no change to the six required capabilities, no change
to the posture clauses in `docket-build/SKILL.md`, and no re-probe of cursor, codex, or opencode.

## What changes

Groomed 2026-08-09 (auto-groom; design in the linked spec). The build runs a forked-mode probe of
the claude harness — a dispatched-subagent operationalization of forked mode, using 0223's stand-in
gate properties and a one-variable-per-run ladder led by a launcher-liveness control (the 0200
trigger is explainable by macOS's missing `setsid(1)`) — and records the result in
`skills/docket-build/references/gate-execution.md` (claude verdict line: dispatched-mode scope
appended with its own version, interactive scope preserved verbatim) and
`gate-execution-evidence.md` (a new forked/dispatched-mode section). The harness-native background
launch is measured as a secondary shape. A never-started outcome stays "forked mode unmeasured"
with a dated inconclusive record; only started-then-killed across all shapes may record
`incompatible`.

## Out of scope

Re-probing cursor/codex/opencode or claude's interactive mode; the headless `claude -p` variant
(recorded unobtainable); any change to the six capabilities, `docket-build/SKILL.md`, or the
observation-budget machinery.

**Reason for deferral** — 0200 is a board-checks hardening change; its branch touches
`scripts/board-checks.sh`, that script's tests and contract, and one convention paragraph. Probing
a harness's process-teardown behavior and rewriting a per-harness evidence table shares no file and
no reasoning with it, and folding it in would expand the branch past the scope its spec, plan, and
review were all conducted against.
