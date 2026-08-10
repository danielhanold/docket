---
id: 284
slug: runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi
title: 'runner-dispatch --observe is sentinel-only: adopt 0282''s identity-checked liveness probe'
status: proposed
priority: high
type: fix
created: 2026-08-10
updated: 2026-08-10
depends_on: []
related: [208, 270, 277]
discovered_from: [282]
adrs: []
spec: docs/superpowers/specs/2026-08-10-runner-dispatch-observe-liveness-probe-design.md
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
| Spec | [2026-08-10-runner-dispatch-observe-liveness-probe-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-10-runner-dispatch-observe-liveness-probe-design.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while reconciling change 0282 (the liveness-keyed launch-and-wait contract
for long-running child processes). 0282 excludes `runner-dispatch.sh` from its site rewiring as a
conscious decision — its surface is agent-dispatch-specific and change 0277 is actively reworking
it — and records the excluded gap as a named residual it is obliged to file rather than absorb.
Re-verified in current source at that reconcile: the only `kill -0` in `runner-dispatch.sh` is in
its give-up path, and `runner-dispatch.md` still states "The sentinel is the *only* source of
liveness — the facade never [probes the process]".

**Opportunity** — `runner-dispatch --observe` has no process-liveness probe at all. Its predicate
is "no sentinel ⇒ still running", so a delegated agent whose process died without writing its
sentinel reads as `running` for the entire `DELEGATION_OBSERVATION_BUDGET` (default 60 minutes)
before the budget bound fires. That is the same marker-keyed-versus-liveness-keyed defect 0282
exists to remove, in the dispatch lifecycle instead of the gate lifecycle, and at ten times the
worst-case latency. 0282 ships the correct predicate — an identity-checked process-group probe
with the terminal record outranking liveness on both sides — but deliberately does not apply it
here.

**Independent value** — stands with 0282 fully reverted: `runner-dispatch`'s dead-child latency
gap is a defect in its own contract, measured against its own documented budget, and closing it
would cut the worst-case detection time for a dead delegated agent from the observation budget to
one observation interval. The value is also independent in the other direction: if 0282 lands, the
gap is the one remaining place in the repo where a wait is still marker-keyed by design.

**Narrower than the stub first read** — `runner-dispatch.sh` **already owns** the identity
conjuncts, inside `terminate_dispatch`, where they gate the group kill on the give-up path. The gap
is not that no such check exists here; it is that the check is consulted only when the facade is
about to *signal*, never as a *verdict input* one lifecycle phase earlier.

**Boundary** — `runner-dispatch.sh`'s `--observe` verb and its `runner-dispatch.md` contract
section on liveness, plus the tests covering them, plus a new `scripts/lib/docket-liveness.sh` that
`gate-run.sh` is refactored onto so the predicate has one definition rather than two. In scope:
the identity-checked probe, the record-outranks-liveness read ordering, and a git-decided
disposition for a child that died without a sentinel — with the mutation tests that redden if any
is dropped. Deliberately out of scope: the observation budget's value (change 0273), reaping
orphaned children (reversing the refuse-to-signal-unprovable-ownership rule is an ADR-level
decision), the dispatch directory layout, the harness adapters, the brief-file format, the
`--launch` verb's detachment mechanism (ADR-0080 measured it and it is not in question), and
anything about the sentinel's role as the source of *correctness* — this is about liveness only,
and correctness still comes from git via `verify-run.sh`.

**Sequencing — no dependency; the stub's caution is superseded.** The stub proposed waiting for
change 0277's rework of the same file. Reading 0277's spec settles it the other way: 0277 declares
*"any change to launch/observe semantics, the run gate, or detachment"* **out of scope**, and the
other two active changes on this file — 0208 (input gates) and 0270 (config resolution) — sit on
the input-validation side. None touches the observe leg. 0277's own assumption 8 set the house
precedent for exactly this: file collisions on `runner-dispatch.sh` are recorded as `related:` and
reconciled at rebase by intent. Recorded as `related: [208, 270, 277]`; `depends_on:` stays empty,
so a `high`-priority fix does not queue behind a `medium` one.
