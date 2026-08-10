---
id: 286
slug: gate-run-observe-poll-loops-strip-state-prefix
title: 'Caller-authored gate-run --observe poll loops strip the state= prefix and never terminate'
status: proposed
priority: high
type: fix
created: 2026-08-10
updated: 2026-08-10
depends_on: []
related: [282, 284, 277]
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-design.md
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
| Spec | [2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-design.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — observed live during a `docket-implement-next` build of an external consumer
(`cet-terraform` change 10). The full-suite gate was launched with
`docket.sh gate-run --launch`, finished cleanly (`terminal` recorded `kind=exit code=0`), and
`--observe` correctly reported `state=passed`. The caller's poll loop nevertheless treated every
observation as non-terminal and would have burned the full `GATE_OBSERVATION_BUDGET` (30 minutes)
had a human / parent agent not killed it.

**Defect** — `gate-run.md` contract: `--observe` prints `state=<state>` (and optionally
`cause=<cause>`). The caller owns the polling loop. A live agent-authored loop did:

```bash
state=$(echo "$out" | head -1 | awk '{print $1}')
case "$state" in
  passed|failed|died|stopped|unavailable) … ;;
  running) ;;
  *) echo "UNKNOWN: $out" ;;
esac
```

`awk '{print $1}'` yields `state=passed` / `state=running`, which match neither arm. Every
iteration hits `UNKNOWN`, sleeps, and repeats — a finished gate looks indefinitely unfinished.

**Not the same as 0284.** Change 0284 is a helper-side false-`running` on
`runner-dispatch --observe` (sentinel-only liveness). This is a **caller-side** mismatch against
`gate-run`'s already-correct `state=` vocabulary: the helper told the truth; the loop could not
read it. Related to change 0282 only as the contract that deliberately puts the poll in the
caller's hands and therefore depends on callers keying on the exact printed form.

**Cost** — worst case: a green suite is treated as unfinished until budget exhaustion, then fail-closed
halt / abort of an otherwise successful build. Measured instance: poll killed after the gate had
already passed; build continued only because a later direct `--observe` was issued outside the
broken loop.

## What changes

Settled design (2026-08-10 auto-groom; detail in the linked spec). Teaching fix, no behavior
change to `gate-run.sh`:

- **Canonical poll loop in `gate-run.md`** — a new *The caller's loop* subsection with one
  copy-paste-correct example: capture-then-match with the exit status neutralized (`|| true`,
  because `--observe` exits 1 on `unavailable` and callers key on the report line, never the exit
  code), `case` arms prefix-matched on the full printed form (`state=passed*`, `state=died*`, …),
  only `state=running*` retried, a fail-closed unknown-line arm (treat as `unavailable`, stop
  polling), bounded by `GATE_OBSERVATION_BUDGET` — plus a one-line anti-pattern note: never
  re-tokenize the report line and match bare state names.
- **`skills/docket-build/SKILL.md` § Gate execution posture** — one sentence: reuse the contract's
  canonical loop verbatim and key each arm on the full `state=<name>` printed form; a bare-token
  loop never terminates.
- **Guards** — the doc example is executable surface: a test extracts the fenced loop and proves,
  against stubbed observe outputs, correct disposition on every terminal state and retry only on
  `running`. Two mutation keys: bare-`passed` pattern reddens as a wrong terminal disposition;
  a retry-`*)` arm (the observed defect) reddens on non-termination under a fixture-local budget.
  A prose sentinel binds the SKILL.md sentence to its `state=` claim.

## Out of scope

- Changing the six-state vocabulary or the `state=` print format itself (callers and tests already
  key on it).
- A helper-owned `--wait` verb — rejected for now: "the helper never polls for the caller" is a
  stated contract invariant and reversing it is a human/ADR-level decision (spec assumption 1).
- `runner-dispatch --observe` liveness and the delegation loop (0284, 0277).
- The gate's own record / liveness predicate (0282).
- Budget values (0273).

## Open questions

None — resolved at groom: teaching + canonical example, not `--wait` (spec assumption 1); the
"caller owns the loop" contract stands unchanged.
