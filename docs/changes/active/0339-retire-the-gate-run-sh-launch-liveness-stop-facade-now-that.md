---
id: 339
slug: retire-the-gate-run-sh-launch-liveness-stop-facade-now-that
title: 'Retire the gate-run.sh launch/liveness/stop facade now that the native Go-v1 gate is canonical (collapse the shared docket-liveness.sh seam with runner-dispatch.sh)'
status: proposed
priority: medium
type: refactor
created: 2026-08-22
updated: 2026-08-22
depends_on: []
stacked_on:
related: []
discovered_from: [338]
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

**Discovered from change 0338** (the gate-observe two-serialization seam). 0338 converges the gate's
`observe` **output format** on native protocol-v1 JSON and retires the shell `state=<name>` text
contract, but it deliberately draws its boundary at the observe *format* and the one caller loop.
It leaves a larger cleanup unaddressed and explicitly out of its scope: the `gate-run.sh` shell
facade still exists as a second implementation of gate launch/observe/stop alongside the landed Go-v1
native gate (`docket gate launch|observe|stop`, `internal/app/gate.go`).

The facade is not only an observe-format emitter. It also carries the **detached-launch and
liveness-checking machinery** it shares with `runner-dispatch.sh` through the extracted predicate in
`scripts/lib/docket-liveness.sh` (`docket_group_alive_and_ours`). So once 0338 lands and the observe
format is JSON, `gate-run.sh` and the native Go gate are two launch/liveness paths for one job,
reconciled by convention rather than by a single implementation. That duplication is exactly the
shape that drifted before (the liveness predicate had already diverged between `gate-run.sh` and
`runner-dispatch.sh` on the empty-token conjunct, which is why change 0284 extracted the shared lib).

## What changes

Decide the disposition of the `gate-run.sh` facade now that the native Go-v1 gate is the canonical
observe/launch/stop supervisor, and collapse the duplication rather than leaving two launch/liveness
paths held together by convention. Open questions for the brainstorm:

- Retire `gate-run.sh`'s launch/observe/stop entirely in favor of `docket gate …`, or keep the shell
  facade as a thin wrapper over the Go binary?
- What becomes of `scripts/lib/docket-liveness.sh` and its second consumer `runner-dispatch.sh` —
  does the native gate subsume the shared predicate, or does `runner-dispatch.sh` keep the shell
  liveness path?
- Migration/compat: any caller still invoking `docket.sh gate-run` (the `WRAPPED_OPS` entry, the
  `docket-build` posture) must be moved before the facade is removed.

**Sequenced after 0338** — this only makes sense once the observe format has converged on JSON, so
it depends on 0338. Boundary: the launch/liveness/stop facade seam and its shared lib, NOT the
observe-format seam 0338 owns.
