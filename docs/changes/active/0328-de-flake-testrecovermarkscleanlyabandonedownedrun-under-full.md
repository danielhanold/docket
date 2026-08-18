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
trivial: true
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

Groomed 2026-08-18 (trivial verdict — test-side setup-retry, no production changes):

1. Diagnose the exact race first under a concurrent-stress run (0325's 8-copy technique) to
   confirm which launch window loses — `Launch` returns at the supervisor's "established"
   handshake, before the child is spawned, so the group SIGKILL may land in an unexpected window.
2. Assert the abandoned precondition instead of assuming it: after the existing lock-release and
   group-gone waits, verify no `terminal.json` exists in the run dir before calling `Recover`.
3. If the precondition check fails (a terminal record snuck in), re-drive the setup with a bounded
   retry — discard that run, launch fresh, re-kill. The `Marked==1` assertion is never weakened: a
   run provably lacking a terminal record with its group gone must be marked at any load.
4. Evidence: the multi-copy stress run green, plus the full suite.

## Out of scope

- Other gate-run/supervisor timing tests not proven flaky here.
- Production-side synchronization hooks in the launch/supervisor path — considered and rejected as
  invasive for a test-only problem.
