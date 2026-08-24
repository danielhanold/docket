---
id: 342
slug: 'harden-autonomous-build-implement-agents-against-the-suite-y'
title: 'Harden autonomous build/implement agents against the suite-yield deadlock (ADR-0024)'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-08-24'
updated: '2026-08-24'
depends_on: []
stacked_on:
related: [223, 231, 282, 285, 314, 315, 341]
discovered_from: [339]
adrs: [24, 95]
spec: docs/superpowers/specs/2026-08-24-resumable-native-gate-driver-design.md
plan:
results:
trivial: false
auto_groomable:
branch: 'feat/harden-autonomous-build-implement-agents-against-the-suite-y'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-24T22:03:00Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-24-resumable-native-gate-driver-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-24-resumable-native-gate-driver-design.md) |
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md) |
<!-- docket:artifacts:end -->

## Why

Changes 0339 and 0341 exposed the same architectural deadlock. A forked build worker could not keep
one foreground command alive for the full suite, so it backgrounded the suite and yielded while
expecting a completion notification. ADR-0024 establishes that a forked agent has no such channel.
The agent therefore returned stale waiting prose, and repeated resumptions created more controllers,
workers, and monitors without proving that the previous process tree had stopped.

The native Go supervisor from change 0314 solved the process-level problem that change 0285 could
not solve portably in Bash: genuine sessions on Darwin and Linux, durable terminal state, exact exit
status, and ownership-proven stop/recovery. The workflow layer still drives that supervisor with a
single Bash polling loop that can occupy one foreground call for the entire 30-minute observation
budget. Finalize independently repeats the same shape in a synchronous Go polling loop. Once the
suite exceeds the harness's foreground-call ceiling, Go is supervising the right process while the
agent still loses the continuation.

The self-matching `pgrep` monitor in the 0339 incident made one wait permanent, but it was a symptom
of allowing workflow agents to invent liveness loops. The root fix is a native continuation protocol,
not a more careful process-name pattern.

## What changes

Add a high-level native Go gate driver above the existing raw supervisor primitives. One logical drive
launches one suite, persists its original deadline and execution identity, waits in short synchronous
slices, classifies terminal state, stops unsafe work, and permits at most one ownership-proven
relaunch without resetting the deadline. Workflow callers consume its typed `WAITING`, `PASSED`,
`FAILED`, and `HALTED` outcomes; they do not write polling loops, parse observation JSON, or use raw
gate verbs directly.

Make waiting a durable, local continuation rather than shared change status. A worker that must
unwind creates an explicit handoff recording the exact branch, HEAD, staged and unstaged changes,
and untracked task files. The nearest live owner claims that handoff atomically, continues the same
gate, and dispatches a fresh worker for the same task when agent judgment is needed again. Waiting
normally stops at the build controller; only an exceptional outer unwind reaches implement-next or
the top-level run gate. `verify-run` reports `run-waiting` only when the claim, handoff, driver, raw
gate, branch, and worktree receipts all agree.

Migrate every workflow gate caller in the same change: build task workers, the build controller's
final gate, implement-next's evidence re-mint and re-gates, and finalize's local gate. Retire the
Bash caller loop, its workflow `jq` dependency, and finalize's 30-minute in-process polling loop.
Keep `launch`, `observe`, `stop`, `recover`, and `cleanup` as primitive/operator APIs, and enforce
the boundary with a whole-repository, mutation-tested structural guard.

Passing the gate remains only a phase result. Implement-next is complete only after evidence,
review, branch publication, PR publication, and `mark-implemented` all succeed.

## Out of scope

Making the suite faster; converting AI controllers or task workers into Go processes; adding a Go
daemon or notification service; creating a shared `waiting` change status; making WIP commits for
handoff; redesigning delegated runner dispatch beyond teaching its verdict consumer about
`run-waiting`; changing the native supervisor's process/session guarantees; or revisiting the
already-completed builds that exposed the defect.

## Evidence

Occurrences of the wedge in the wild, newest first. The pattern is recurring rather than a one-off,
and became reliably visible as the suite approached the foreground-call ceiling.

- **2026-08-24 — change 0341** (`artifact-table-links-render-as-bare-code-spans…`). An
  autonomous run repeated the same yield-loop after its code tasks had committed green. The feature
  HEAD stopped moving before metadata, the build gate, branch publication, and PR publication.
  Human recovery had to terminate the redundant agents and finish the remaining phases inline.
- **2026-08-24 — change 0339** (`retire the gate-run.sh launch/liveness/stop facade`). A suite wait
  used `pgrep -f` against a command line containing its own pattern, then repeated background/yield
  resumptions multiplied the orchestrators and process monitors. Git inspection, rather than agent
  completion prose, was required to discover that the workflow had never reached publication.

## Reconcile log

### 2026-08-24

**2026-08-24** — Reconciled against current `docket` reality before planning. Confirmed the change is unstarted and its premise still holds: `docket gate` exposes only the raw `launch`/`observe`/`stop`/`recover`/`cleanup` primitives (no `drive` verb), and no `run-waiting` verdict exists anywhere in the tree. The two caller loops the spec targets are present as written — `docket-build`'s Bash caller loop (embedded `skills/docket-build/references/gate-caller-loop.md`, mirrored under `~/.claude/skills/docket-build`) and finalize's in-process Go polling loop (`internal/app`). Related work is already landed and reflected: the native supervisor (0314/ADR-0095) lives in `internal/app/gate_supervisor.go` + `internal/app/gate.go`, and the incident forensics (0339/0341) match the spec's Evidence section. No scope drift, no work done elsewhere, no new external constraints — proposal, spec, and relations (adrs 24, 95; the new structured-waiting ADR is minted in the review phase) carry forward unchanged.
