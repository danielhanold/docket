---
id: 286
slug: gate-run-observe-poll-loops-strip-state-prefix
title: 'Caller-authored gate-run --observe poll loops strip the state= prefix and never terminate'
status: implemented
priority: high
type: fix
created: 2026-08-10
updated: 2026-08-10
depends_on: []
related: [282, 284, 277]
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-design.md
plan: docs/superpowers/plans/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix.md
results: docs/results/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-results.md
trivial: false
auto_groomable: true
branch: feat/gate-run-observe-poll-loops-strip-state-prefix
pr: https://github.com/danielhanold/docket/pull/192
blocked_by:
claimed_at: 2026-08-10T22:27:21Z
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-design.md) |
| Plan | [2026-08-10-gate-run-observe-poll-loops-strip-state-prefix.md](https://github.com/danielhanold/docket/blob/feat/gate-run-observe-poll-loops-strip-state-prefix/docs/superpowers/plans/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix.md) |
| Results | [2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-results.md](https://github.com/danielhanold/docket/blob/feat/gate-run-observe-poll-loops-strip-state-prefix/docs/results/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix-results.md) |
| PR | [#192](https://github.com/danielhanold/docket/pull/192) |
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

## Reconcile log

- **2026-08-10** — Reconciled at claim. Checked `origin/main` (tip `394e2c48`, change 0282's
  terminal publish): `scripts/gate-run.md` still carries `## Usage`, `### --observe`, the six-state
  table and the "Only `running` is retryable" rule, and has **no** *The caller's loop* subsection —
  the gap the spec names is still open. `skills/docket-build/SKILL.md` § *Gate execution posture*
  still ends its state-keying paragraph at "**Key the wait on the state each observation reports**…
  The six states and their retryability are `gate-run.md`'s contract" with no sentence about the
  printed `state=<name>` form — also still open. Dependency reality: 0282 is `done` and merged (the
  helper and its contract are on `main`); 0284 and 0277 are still `proposed`, so the
  `runner-dispatch --observe` surface is untouched and the spec's assumption 6 (leave it alone)
  holds unchanged. No `depends_on`, nothing to wait on. Scope, assumptions, and the three touched
  files stand as specced; one refinement for the builder, already anticipated by the spec's *Files
  touched* parenthetical — SKILL.md prose guards for this file live in
  `tests/test_gate_execution_posture.sh`, which is where the sentence sentinel belongs, while the
  executable-example assert belongs in `tests/test_gate_run.sh`. Auto-capture: enabled, nothing
  surfaced clearing the six admission gates; the two health warnings the Step-0 status pass reported
  (change 189's `|` in its title, change 44's unquoted `blocked_by:`) are pre-existing backlog
  hygiene on other changes, not discoveries of this pass, and are reported rather than minted.
