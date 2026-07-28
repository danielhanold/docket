---
id: 162
slug: restore-the-machine-local-ignored-advisory-for-a-committed-t
title: Restore the machine-local-ignored advisory for a committed too-deep runtime.bash
status: killed
priority: medium
type: fix
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [157]
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

Change 0157 (unit 0152) routed `ensure-docket-env.sh` through `docket-runtime.sh`'s validator. A
side effect noticed at close-out: a **committed** `.docket.yml` carrying a too-deep `runtime.bash`
block no longer trips the "machine-local — ignored" advisory. Nothing resolves wrongly — the value
is still rejected — but the operator loses the diagnostic that explains why their setting had no
effect, which is the whole point of the advisory.

## What changes

Restore the advisory for the committed-layer too-deep `runtime.bash` shape, and add a negative
fixture pinning it so the advisory cannot silently disappear again.

## Out of scope

Changing which shapes are accepted; only the advisory is at issue.

## Why killed

Duplicate — already captured as #160 by change 0157's own auto-capture.
