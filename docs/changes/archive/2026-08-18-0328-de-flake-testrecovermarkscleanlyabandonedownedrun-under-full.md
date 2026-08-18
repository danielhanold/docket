---
id: 328
slug: de-flake-testrecovermarkscleanlyabandonedownedrun-under-full
title: 'De-flake TestRecoverMarksCleanlyAbandonedOwnedRun under full-suite load'
status: done
priority: medium
type: fix
created: 2026-08-18
updated: 2026-08-18
claimed_at: 
depends_on: []
stacked_on:
related: []
discovered_from: [325]
adrs: []
spec:
plan: docs/superpowers/plans/2026-08-18-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full.md
results: docs/results/2026-08-18-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full-results.md
trivial: true
auto_groomable:
branch: feat/de-flake-testrecovermarkscleanlyabandonedownedrun-under-full
pr: https://github.com/danielhanold/docket/pull/219
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-08-18-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full.md](https://github.com/danielhanold/docket/blob/feat/de-flake-testrecovermarkscleanlyabandonedownedrun-under-full/docs/superpowers/plans/2026-08-18-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full.md) |
| Results | [2026-08-18-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full-results.md](https://github.com/danielhanold/docket/blob/feat/de-flake-testrecovermarkscleanlyabandonedownedrun-under-full/docs/results/2026-08-18-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full-results.md) |
| PR | [#219](https://github.com/danielhanold/docket/pull/219) |
<!-- docket:artifacts:end -->

## Why

`internal/process/recover_test.go`'s `TestRecoverMarksCleanlyAbandonedOwnedRun` is a
load-sensitive flake in the same gate-run/supervisor family as change 0325's barrier flake.
Under full parallel-suite contention it fails with `Marked:0` — recover declined to mark the run,
but the test expected `Marked:1`. The grooming attributed this to the owned run writing its own
durable terminal record; reconcile found that unproven and at least three other `classifyRun`
paths that produce the same `Marked:0` (see the reconcile log). Surfaced during change 0325's finalize merge gate
(the whole `go test ./...` file `test_go_toolchain` reddened, at 150s / OVER BUDGET); the test
passes 5/5 in isolation. It is a distinct flake from 0325's `--stop` barrier waits and needs its
own fix.

## What changes

Groomed 2026-08-18 (trivial verdict — test-side setup-retry, no production changes):

1. Diagnose the exact race first under a concurrent-stress run (0325's 8-copy technique) to
   confirm which launch window loses — `Launch` returns at the supervisor's "established"
   handshake, before the child is spawned, so the group SIGKILL may land in an unexpected window.
2. Assert the abandoned precondition instead of assuming it: after the existing lock-release and
   group-gone waits, verify the run dir carries **none** of the durable records that outrank the
   group probe in `classifyRun` — `terminal.json`, `stopped.json`, `abandoned.json` — and that
   `recoverGroupProbe` still answers `probeAbsent`. Reconcile widened this from the
   `terminal.json`-only check the grooming assumed; see the reconcile log.
3. If the precondition check fails, re-drive the setup with a bounded retry — discard that run,
   launch fresh, re-kill. The `Marked==1` assertion is never weakened: a run provably lacking every
   durable verdict, with its group provably absent, must be marked at any load.
4. Evidence: the multi-copy stress run green, plus the full suite.

## Out of scope

- Other gate-run/supervisor timing tests not proven flaky here.
- Production-side synchronization hooks in the launch/supervisor path — considered and rejected as
  invasive for a test-only problem.

## Reconcile log

### 2026-08-18 — reconciled against current `main`; scope intact, step 2 widened

Premise re-verified, one correction:

- **Still real.** `internal/process/recover_test.go` is untouched since change 0314
  (`b0624a39`, `12bcbac6`). Change 0325 is `done` (PR #218) and de-flaked a *different* test, so
  it did not incidentally fix this one. `TestRecoverMarksCleanlyAbandonedOwnedRun` stands exactly
  as described.
- **The grooming's stated root cause is unproven.** `## Why` claimed the run "wrote its own
  durable terminal record". The helper's `sleep` mode blocks for `time.Hour` with default signal
  disposition (`main_test.go`, `case "sleep"`), so the child never exits on its own, and the
  group SIGKILL leaves the supervisor no opportunity to write `terminal.json` — a SIGKILLed
  supervisor writes nothing. The terminal-record story does not survive reading the helper.
- **`Marked:0` is reachable four ways**, per `classifyRun` in `internal/process/recover.go`:
  a durable `terminal.json` (`terminal`); an unprovable or held live lock
  (`needs-inspection` / `live`); a pre-existing `stopped.json` / `abandoned.json`; and
  `recoverGroupProbe(m.PGID)` answering `probeLive`/`probeUnknown` (`needs-inspection`) — the
  last being the most plausible under full-suite load, where the recorded PGID can be recycled to
  an unrelated process between the test's `groupAlive` wait and recover's own re-probe.
- **Consequence for the plan.** Step 2 as groomed asserted only `terminal.json`'s absence, which
  would leave three of the four paths still flaky and the test still red under load. Widened to
  assert every durable verdict absent *and* the group still `probeAbsent` at the moment of the
  check. Step 1's diagnose-first instruction is unchanged and now load-bearing: the stress run
  decides which path actually fires, and the fix follows the evidence.

Scope, trivial verdict, and out-of-scope list unchanged. No production-code change; still
test-only. No new dependencies; `depends_on` stays empty.
