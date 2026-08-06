---
id: 223
slug: the-build-gate-contract-never-states-an-execution-posture-for
title: The build gate contract never states an execution posture for a suite that outgrows a single foreground call
status: proposed
priority: high
type: docs
created: 2026-08-06
updated: 2026-08-06
depends_on: []
related: [66, 190, 224, 225]
discovered_from: [203]
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

`skills/docket-build/SKILL.md` § *The build gate* says to run the **whole suite once** and defines
green vs red, but says nothing about **how** the run is executed. That silence was survivable while
the suite fit comfortably inside one foreground call. It no longer does.

The suite (79 test files) now runs right at the **maximum** foreground timeout the Claude Code
harness allows, so the gate is nondeterministic at the boundary — there is no larger value to raise
it to. Observed on the change 0203 run (2026-08-06):

- one suite run completed foreground, while an **identical** run returned
  `Exit code 143 / Command timed out after 10m 0s`;
- an independent foreground run in the parent session died at exactly 10m with ~5 files left
  (last PASS: `test_sync_agents_opencode.sh`).

The 0203 implementer worked around it on its own — backgrounding each gate to a log file and
blocking on a sentinel — and passed three gates that way. That workaround is correct but
**unwritten**, so the next implementer re-derives it, or doesn't.

The workaround also has a consequence the contract must name. Parking on the suite makes the run
**yield**, and the yield surfaces to the dispatcher as a `status: completed` notification carrying
the agent's *stale pre-park text* ("suite still running, holding"). On 0203 that produced **three
false stops** on a single change: each looked like a crashed run and was only disproved by reading
git state. The convention already states the reciprocal rule for dispatched subagents — a caller
must not read a bare `completed` as proof the child finished (ADR-0024) — but nothing extends it to
the gate.

Note this is **not** an ADR-0024 violation and must not be written as one. ADR-0024's never-yield
rule governs **dispatched subagents**; backgrounding a **test command** and blocking on its result
is a different thing. The distinction is the crux of the design.

A prior rule of thumb — "run the full suite in ONE foreground call with the timeout at its maximum"
— circulated in operator memory from change 0053 and was **never written into this repo**. It is now
unsatisfiable and should not be adopted; the 0053 failure it guarded against (a subagent losing its
background log across a turn boundary) is avoided by writing to a stable path.

## What changes

State an **execution posture** for the gate in `skills/docket-build/SKILL.md` § *The build gate*:

- the gate must not assume the suite fits inside a single foreground call;
- it must be run so its result **survives a yield** (a durable result artifact, not a transient
  in-context return);
- completion is verified from **that artifact**, never from a caller-visible `completed` signal;
- reconcile this against the convention's *Composition* never-yield rule so the boundary between a
  dispatched subagent and a backgrounded test command is unambiguous.

Also name the downstream reach: `finalize.gate: local` re-runs the same suite and hits the same
wall.

**Harness-neutrality is a hard constraint.** The convention requires skill prose to be
harness-neutral and to never name product-specific syntax. "Bash tool", a literal timeout value, and
"Monitor" are all Claude Code specifics and cannot appear normatively — the rule has to be phrased
by **shape**. Finding a formulation that is both actionable and harness-neutral is the real design
work here.

## Out of scope

- Reducing suite runtime — that is change 0225.
- The green/red keying gap — that is change 0224.
- Any change to ADR-0024 or the subagent never-yield rule.

## Open questions

- Can the posture be stated normatively at all without a harness-specific escape hatch, or does it
  belong partly in a per-harness reference?
- Should this bind `finalize`'s gate too, or only the build gate?
- Is a guard test feasible for a rule of this shape, or is it prose-only?

## Reconcile log
