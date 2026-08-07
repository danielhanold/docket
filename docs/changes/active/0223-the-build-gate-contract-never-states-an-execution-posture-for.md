---
id: 223
slug: the-build-gate-contract-never-states-an-execution-posture-for
title: The build gate contract never states an execution posture for a suite that outgrows a single foreground call
status: in-progress
priority: high
type: feat
created: 2026-08-06
updated: 2026-08-07
depends_on: []
related: [66, 190, 224, 227]
discovered_from: [203]
adrs: []
spec: docs/superpowers/specs/2026-08-06-the-build-gate-contract-never-states-an-execution-posture-for-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/the-build-gate-contract-never-states-an-execution-posture-for
pr:
blocked_by:
claimed_at: 2026-08-07T06:09:20Z
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-06-the-build-gate-contract-never-states-an-execution-posture-for-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-06-the-build-gate-contract-never-states-an-execution-posture-for-design.md) |
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

State a **gate execution posture** in `skills/docket-build/SKILL.md` § *The build gate*,
harness-neutral and stated by capability: the gate must not assume the suite fits inside a single
foreground call, must record its outcome to a durable result artifact, must establish completion
from that artifact rather than from any caller-visible completion signal, may yield while the gate
runs, and must observe within a finite budget and fail closed when it expires. A companion
false-completion rule covers the reciprocal error — a stale pre-yield report is not evidence of a
crashed run.

The contract lives in `docket-build` (the gate's owner); `docket-finalize-change`'s local gate
**cites it by reference** rather than restating it, mirroring how build already points at finalize's
single-source suite command.

Product-specific detail is quarantined in a new
`skills/docket-build/references/gate-execution.md` — six required capabilities, plus a per-harness
evidence section carrying a `supported` / `unverified` / `incompatible` verdict for every shipped
harness. That quarantine is what keeps the skill body neutral while the rule stays actionable.

Ship the observation budget as configuration end-to-end: a new top-level
`gate_observation_budget` (integer minutes, default 30), exported as `GATE_OBSERVATION_BUDGET`, with
resolver, example yml, README, and layer classification landing together. Classified global-able,
not coordination-fenced.

Guard the statically representable parts, mutation-tested — including a verdict recorded for every
harness in `HD_SHIPPED_HARNESSES`, so a fifth harness cannot silently ship undeclared.

Record the ADR-0024 boundary as a new ADR: the never-yield rule governs **dispatched subagents**,
not an external gate process observed by its owner. Without it the change reads as a violation of a
rule it does not touch.

Type flipped `docs` → `feat` at grooming: the configuration knob is real code.

## Out of scope

- Reducing suite runtime — that is change 0227 (supersedes the killed 0225).
- The green/red keying gap — that is change 0224.
- Any change to ADR-0024 or the subagent never-yield rule.

## Open questions

None outstanding. All four shipped harnesses were smoke-tested at grooming (2026-08-06) and all four
are **`supported`** — but three of the four break under a naive launch, by three different symptoms:

- **codex-cli 0.146.1** — gate **killed** before writing any output, while the launch command reports
  success. Requires new-session detach.
- **opencode 1.18.14** — caller **blocked** for the job's full duration (51s call, 45s job). Requires
  stream redirection.
- **cursor-agent 2026.01.23** — gate **killed** mid-run, call returns normally. Requires stream
  redirection; `nohup` alone is insufficient (proven by a disambiguating run).
- **claude** — documented background mechanism; no naive-launch failure.

One mitigation covers all four — detach into a new session, redirect every stream to a durable
location — which is also what produces the durable artifact. That convergence sharpened required
capabilities 1 and 2, and answers the stub's original question: the posture **can** be stated
normatively with no harness-specific escape hatch.

Verdicts are version-scoped; the implementer re-probes rather than inheriting them on faith.

## Reconcile log
