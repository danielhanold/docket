---
id: 338
slug: gate-execution-terminal-sentinel-has-no-format-contract-poll
title: 'Gate-execution terminal sentinel has no format contract — poll grepping JSON never matches the plain-text state: line and spins forever'
status: proposed
priority: medium
type: fix
created: 2026-08-22
updated: 2026-08-22
depends_on: []
stacked_on:
related: []
discovered_from: [337]
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

**Trigger** — observed live during the change 0337 `docket-implement-next` run (2026-08-22). The
dispatched run reached the build gate, started the suite correctly, and then its
gate-observation poll spun indefinitely: the run notified its parent three times with "still
waiting / blocking on the terminal notification" while making no new commits, reading exactly like
the ADR-0024 self-notification deadlock. It was not that deadlock. Child-agent notifications
(status, plan-writer, the four build workers, the reviewer) all arrived and were acted on, and the
gate itself was healthy and passed twice. The poll matched **nothing** because it grepped for a
JSON-shaped terminal sentinel (`"state":"..."`) while the actual observation surface emits the
state as a **plain-text** line (`state: running`, `state: <terminal>`). The pattern never matched,
so the loop never exited, and the run only advanced after a human resumed it and told it to stop
arming the poll.

**Root cause** — `docket-build`'s *Gate execution posture* (`skills/docket-build/SKILL.md`, and its
`references/gate-execution.md` Capability 5) requires the gate to "record an unambiguous terminal
result" and defines the four-state vocabulary a caller keys on, but it does **not** specify the
serialization/format of that terminal sentinel. With the format left implicit, the worker that
improvises the poll and the surface that emits the state can drift on shape (JSON vs. plain-text
`key: value`), and the mismatch is silent: a poll that never matches is indistinguishable from a
gate that never finishes. The failure mode masquerades as a wedge / false completion and costs
human resumes to break.

**Distinct from adjacent work** — this is NOT change 0264 (which measures the *forked-mode launch
shape* that lets the gate survive being detached) and NOT ADR-0024 (unreceivable self-notification).
Both of those were the leading hypotheses during the incident and both were wrong. The defect here
is a missing **format contract** for the terminal-state sentinel that the observation poll parses:
the state vocabulary is specified, the state *shape* is not.

## What changes

Pin the terminal-state sentinel's format as part of the gate-execution contract so the emitting
surface and the observing poll cannot drift:

- Specify the exact serialization of the terminal sentinel (the `state:` signal the poll keys on)
  in `skills/docket-build/references/gate-execution.md` alongside the existing four-state vocabulary
  — one canonical shape, named, so both sides parse the same thing.
- Make the observation poll's match key on that specified shape (and, ideally, fail loudly on an
  **unrecognized** state line rather than treating "no match yet" as "still running" forever — an
  unparseable sentinel is a defect, not a poll-again condition).
- Add a guard/regression that a plain-text `state: <terminal>` line is recognized as terminal, so a
  future reshape of the emitter (e.g. to JSON) reddens a test instead of silently re-introducing the
  infinite poll.

Boundary: contract + poll-parser + guard only. No change to the four-state vocabulary itself, no
change to the launch-shape question owned by 0264, and no change to the notification mechanics.
