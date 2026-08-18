---
id: 328
slug: de-flake-testrecovermarkscleanlyabandonedownedrun-under-full
title: 'De-flake TestRecoverMarksCleanlyAbandonedOwnedRun under full-suite load'
status: proposed
priority: medium
type: fix
created: 2026-08-18
updated: 2026-08-18
depends_on: []
stacked_on:
related: []
discovered_from: [325]
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

`internal/process/recover_test.go`'s `TestRecoverMarksCleanlyAbandonedOwnedRun` is a
load-sensitive flake in the same gate-run/supervisor family as change 0325's barrier flake.
Under full parallel-suite contention it fails with `Marked:0` because the owned run wrote its
own durable terminal record before the test's recover logic ran, so recover correctly declined
to mark it — but the test expected `Marked:1`. Surfaced during change 0325's finalize merge gate
(the whole `go test ./...` file `test_go_toolchain` reddened, at 150s / OVER BUDGET); the test
passes 5/5 in isolation. It is a distinct flake from 0325's `--stop` barrier waits and needs its
own fix.

## What changes

Make `TestRecoverMarksCleanlyAbandonedOwnedRun` deterministic under load — e.g. drive the recover
precondition through an explicit synchronization point rather than assuming the owned run has not
yet written its terminal record, or otherwise remove the timing assumption. Never weaken the
assertion. Confirm with a concurrent-stress run (like 0325's 8-copy check).

## Out of scope

Other gate-run/supervisor timing tests not proven flaky here.
