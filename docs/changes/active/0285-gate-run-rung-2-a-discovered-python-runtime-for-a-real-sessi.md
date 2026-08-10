---
id: 285
slug: gate-run-rung-2-a-discovered-python-runtime-for-a-real-sessi
title: 'gate-run rung 2 — a discovered Python runtime for a real session and an exact child status'
status: proposed
priority: high
type: feat
created: 2026-08-10
updated: 2026-08-10
depends_on: [282]
related: [264, 284, 132]
discovered_from: [282]
adrs: [81]
spec: docs/superpowers/specs/2026-08-10-gate-run-rung-2-a-discovered-python-runtime-for-a-real-sessi-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-10-gate-run-rung-2-a-discovered-python-runtime-for-a-real-sessi-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-10-gate-run-rung-2-a-discovered-python-runtime-for-a-real-sessi-design.md) |
| ADRs | [ADR-0081](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0081-gate-run-contract-narrowed-per-platform-process-group-where-no-session-primitive-exists.md) |
<!-- docket:artifacts:end -->

## Why

Change 0282 shipped `scripts/gate-run.sh` with two *Named residuals*, and ADR-0081 recorded that
settling them was a human's decision, not a task's: *"superseding this ADR is the mechanism."* This
change takes that decision.

Both residuals are the same limitation twice — a POSIX shell cannot reach two kernel facilities:

1. **No session primitive on macOS.** `setsid(1)` is absent there, so gate-run's rung 1 is dead code
   on darwin and every macOS launch lands on rung 3 (own process group). The contract was narrowed
   honestly rather than claiming a session it does not deliver.
2. **The `129..192` floor.** A shell cannot tell a genuine `exit 143` from death by signal 15. The
   bias toward "killed" is deliberate and correct, but it is still a guess.

What tipped the decision was noticing the shape of the evidence. All four harness verdicts in
`gate-execution-evidence.md` were measured **on macOS**, and all four used `fork` + `POSIX::setsid` —
a shape gate-run cannot use, because it needs an interpreter. The disambiguating arms removed the
detach from a plain `fork`, leaving the child in the launcher's own group, so they compare *new
session* against *no separation at all* and never exercise the middle rung. The rung macOS actually
runs has **no in-harness measurement on any harness**; its only support is ADR-0080's synthetic
group-TERM test, which is faithful to Codex's stated mechanism but was not run inside a live turn.

So on macOS the choice is not "session versus group, pick a guarantee." It is *the shape with four
green harness measurements* against *the shape with none* — and the only thing standing between them
is an interpreter docket already has the machinery to discover.

That machinery is the second half of the reason this is cheap. `ensure-global-config.sh` already
discovers, validates, persists, and exports an absolute runtime path — it is a generic framework
with exactly one consumer today (`runtime.bash`). And ADR-0062 already drew the dependency boundary
narrow enough to permit this explicitly: *"This is not a claim that docket has no external
requirements at all — change 0132 established that docket validates a configured GNU Bash 4+
runtime. The rule bans an external YAML parser, nothing wider."*

## What changes

- A **`runtime.python`** config key alongside `runtime.bash`, discovered and validated by the
  existing framework, exported as `DOCKET_PYTHON_PATH`.
- Discovery that **rejects `/usr/bin/python3`** — Apple's Xcode shim, which opens a blocking GUI
  dialog on a machine without developer tools, so probing it would hang the install itself.
- A **functional** validator (fork, setsid, and both wait-status arms) rather than a version check,
  because a pyenv shim or half-built virtualenv passes every non-functional check and fails at exec.
- A **Python wrapper filling gate-run's vacant rung 2**, delivering a real new session and the true
  child status. The three verbs stay in Bash; only the wrapper moves, and only under rung 2.
- **Graceful degradation** to today's behavior when the runtime is absent or fails its probe.
- A new ADR **superseding ADR-0081**.

## Out of scope

- **The delegation boundary** — `runner-dispatch`'s `set -m` (ADR-0080) would benefit from the same
  upgrade, but it doubles the blast radius and change 0284 already touches that file. Follow-up.
- **Measuring the middle rung in a live harness** — change 0264 owns that, and it stays valuable
  whichever rung ships.
- **Making Python mandatory.** The fallback is the point.
- **Re-probing any of the four harness verdicts** in `gate-execution.md`.

## Open questions

- **Close-out will need `terminal-publish.sh --adr 81`.** Once ADR-0081 is superseded its status is
  no longer `Accepted`, and terminal-publish's Accepted gate silently skips it — leaving `main`
  inconsistent. Finalize will not catch this; it is a known, previously-hit failure for exactly this
  change shape.
- Whether the two wrapper implementations should eventually collapse into one, if the Python runtime
  proves universally present in practice. Deliberately not decided now.

## Reconcile log
