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
related: []
discovered_from: [282]
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

**Boundary** — `runner-dispatch.sh`'s `--observe` verb and its `runner-dispatch.md` contract
section on liveness, plus the tests covering them. In scope: adopting the identity-checked liveness
probe and the record-outranks-liveness read ordering, with the mutation tests that redden if either
is dropped. Deliberately out of scope: the dispatch directory layout, the harness adapters, the
brief-file format, the `--launch` verb's detachment mechanism (ADR-0080 measured it and it is not
in question), and anything about the sentinel's role as the source of *correctness* — this is about
liveness only, and correctness still comes from git via `verify-run.sh`.

**Reason for deferral** — it cannot ride 0282's branch without expanding that branch's scope past
its own stated exclusion. `runner-dispatch.sh` is being concurrently reworked by change 0277;
touching it from 0282 would couple two unrelated lifecycles and double 0282's blast radius, which
is exactly the coupling 0282's spec assumption 1 rejected when it declined to extend
`runner-dispatch.sh` with a generic verb. The right sequencing is: 0282 ships and proves the
predicate, 0277 finishes its rework, then this change transplants the proven predicate.
