---
id: 314
slug: native-process-supervisor-and-local-gate
title: 'Native process supervisor and local gate'
status: in-progress
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-17
depends_on: [304]
stacked_on:
related: [264, 285]
discovered_from: [303]
adrs: [80, 81, 87, 95]
spec: docs/superpowers/specs/2026-08-16-native-process-supervisor-and-local-gate-design.md
plan: docs/superpowers/plans/2026-08-16-native-process-supervisor-and-local-gate.md
results:
trivial: false
auto_groomable:
branch: feat/native-process-supervisor-and-local-gate
pr:
blocked_by:
reconciled: true
claimed_at: 2026-08-17T02:20:00Z
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-16-native-process-supervisor-and-local-gate-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-16-native-process-supervisor-and-local-gate-design.md) |
| Plan | [2026-08-16-native-process-supervisor-and-local-gate.md](https://github.com/danielhanold/docket/blob/feat/native-process-supervisor-and-local-gate/docs/superpowers/plans/2026-08-16-native-process-supervisor-and-local-gate.md) |
| ADRs | [ADR-0080](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0080-detached-delegation-execution-posture-launch-then-observe.md), [ADR-0081](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0081-gate-run-contract-narrowed-per-platform-process-group-where-no-session-primitive-exists.md), [ADR-0087](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0087-liveness-probe-non-zero-is-not-evidence-of-death.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md) |
<!-- docket:artifacts:end -->

## Why

The Bash gate helper cannot establish the required macOS session or distinguish a real high exit
code from signal death. Go can provide both natively while preserving durable observation and safe
group termination without another runtime or daemon.

## What changes

Add a repository-independent native process package and `docket gate` launch, observe, stop, and
recover operations. Re-execute the same binary as a per-run supervisor in a new Darwin/Linux
session; keep private durable run state and separate logs; record exact exit or signal status; gate
signals on a random ownership token, live lock, and current session/group identity; and mark only
proved abandoned owned runs during recovery.

## Out of scope

Behavior owned by changes 0305–0313; Python, Perl, shell wrappers, a second executable, a global
daemon, sockets, Windows, CI gate polling, or live-harness re-probes; workflow relaunch/repair and
the claim-to-implemented transition (0315); finalize recovery and physical cleanup (0316); release
packaging and harness acceptance (0317); and Bash removal or hard cutover (0318).

## Design decisions

The approved focused design is in the linked spec. The same `docket` executable becomes a narrow
per-run supervisor rather than shipping a helper binary or daemon. It publishes an addressable
session before starting the command, holds the run's live lock until an atomic exact terminal
record is durable, and treats clean absence separately from unprovable identity. Recovery records
abandonment but neither signals nor deletes ambiguous state.

Implementation records a new Accepted ADR superseding ADR-0081 with the native-session and exact
wait-status decision. ADR-0080's delegated-agent boundary, ADR-0087's liveness rule, and change
0264's harness evidence remain unchanged.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-17

Reconciled against current `main`/`docket` reality before planning. No scope change; spec kept as-is.

- Predecessor **0304** is `done` (archived). Its contracts (`app.Envelope`, `app.Result`, `app.ExitCode`, Cobra registration via `internal/cli`, Darwin/Linux target set) are present and consumed as designed.
- The sibling migration packages **0305–0313** have since landed in `internal/` (`config`, `document`, `domain`, `gitcli`, `githubcli`, `harness`, `install`, `render`, `repository`, `workspace`, `evidence`). The spec already anticipates them as independent siblings this change must not import; that boundary is unchanged and now enforceable by a real import-containment test.
- `internal/process` does **not** exist yet — no file/package collision; this change creates it fresh.
- No `gate` command group exists (grep matches were incidental substrings). `docket gate` is net-new.
- **ADR-0081** is `Accepted`; this change mints a new ADR superseding it (ADR-0081 stays on `adrs:` so the supersession + status flip land atomically at build).
- Auto-capture (enabled): no distinct beyond-scope follow-up surfaced during reconcile — downstream 0315–0318 already exist as stubs; nothing minted.
