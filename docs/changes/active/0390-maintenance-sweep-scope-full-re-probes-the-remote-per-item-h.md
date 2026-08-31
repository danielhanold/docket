---
id: 390
slug: 'maintenance-sweep-scope-full-re-probes-the-remote-per-item-h'
title: 'maintenance sweep --scope full re-probes the remote per item, hanging the sweep'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: [389]
discovered_from: [389]
adrs: [101]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0101](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0101-maintenance-sweep-scope-defer-historical-cleanup-out-of-impl.md) |
<!-- docket:artifacts:end -->

## Why

`docket maintenance sweep --scope full` effectively hangs — it runs for many minutes with no streamed output and no observed completion (killed after 1m25s in testing), which makes the sweep's merge-closeout, historical cleanup, and health-check pass unusable and blocks docket-status's full maintenance path. Root cause is confirmed and is NOT a deadlock: bare `git fetch`/`ls-remote`/`ssh` to GitHub each return in ~0.5s. The sweep pins the operational context once at the top (maintenance.go:254), then RE-PINS it for every worklist item via sweepReloadVersion/sweepReloadPresent (maintenance.go:449,500,508) -> reader.PinContext -> loadOperationalContext -> gatherRepoFacts (repository_facts.go:210,225,242), and each re-pin re-runs three GitHub network round-trips (FetchBranch(default), FetchBranch(integration), ProbeRemoteBranch(metadata) ls-remote) whose answers were already known from the first pin. The dispatched op re-pins once more. With hundreds of terminal records swept under full scope, that is O(items x network round-trips) sequential GitHub calls. It is aggravated by gitcli defaultNetworkTimeout = 5 minutes (client.go:12), so a single stalled probe silently costs up to 5 minutes; a goroutine dump caught the process parked in exactly one such ProbeRemoteBranch read.

## What changes

Two changes, both scoped to the sweep path. (1) Stop the per-item reload from re-probing the remote: reuse the remote branch facts already captured by the sweep's initial pin so the per-item fresh-authority reload re-reads only the metadata (change blob versions on origin/docket, which genuinely can move) and takes the remote tips as given via gatherRepoFacts's existing defaultTip/integrationTip short-circuit. Remote tips are the authority the reload does NOT protect, so reusing them preserves the fresh-authority guarantee the reload exists for. (2) Cap the sweep's per-op network timeout well below the 5-minute default so any genuine network stall fails fast and visibly instead of hanging. (1) removes the volume; (2) bounds any single stall.

## Out of scope

Changing --scope implementation (ADR-0101 / change 389 already keeps historical cleanup off the implement-next startup hot path). The separate internal/app suite wall-clock work. The content of the health-check pass. Any change to what the sweep decides to close out, clean up, or reclaim — this is purely about how per-item fresh authority is reloaded and how long a network probe may block.
