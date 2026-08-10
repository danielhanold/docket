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
related: [282, 284]
discovered_from: []
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

## What

Needs brainstorm. Candidate directions (not settled):

- Ship a copy-paste-correct poll example in `gate-run.md` / `docket-build` gate posture that matches
  on `state=passed*` (etc.), not bare tokens — and make that the taught shape agents are told to
  reuse rather than invent.
- Add a `gate-run --wait` (or equivalent) that owns the budgeted poll internally, so agents never
  author the loop — tension with the current invariant "the helper never polls for the caller."
- Harden skill / shim teaching so invented loops that strip `state=` are an explicit anti-pattern.

## Out of scope

- Changing the six-state vocabulary or the `state=` print format itself (callers and tests already
  key on it).
- `runner-dispatch --observe` liveness (0284).
- The gate's own record / liveness predicate (0282).

## Open questions

- Prefer teaching + examples, a helper-owned `--wait`, or both?
- If `--wait` lands, does it replace or sit beside the "caller owns the loop" contract in
  `gate-run.md`?
