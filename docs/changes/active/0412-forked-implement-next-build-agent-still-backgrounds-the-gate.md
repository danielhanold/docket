---
id: 412
slug: 'forked-implement-next-build-agent-still-backgrounds-the-gate'
title: 'Forked implement-next/build agent still backgrounds the gate driver and yields (recurring suite-gate yield-wedge)'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-08'
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [405, 345, 342, 359, 323, 375]
discovered_from: [349, 323]
adrs: [24]
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
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

The forked-agent suite-gate yield-wedge has recurred through two distinct paths despite the merged
contract and driver work intended to prevent it.

During change 0349, the forked implement-next/build agent deliberately launched gate-drive advance
loops in the background and then yielded while waiting for completion notifications it could not
receive. Caller-visible completion notifications arrived while suite gates were still in flight,
and the parent had to recover terminal state manually. On the final recurrence, the gate passed but
the forked agent stopped before pushing the branch and opening the PR. This is the failure family
ADR-0024 and changes 0223, 0232, 0271, 0342, and 0359 were meant to close: a forked agent has no
notification channel, so background-and-yield cannot be its continuation mechanism.

Change 0323 exposed a different trigger with the same unsafe result. Its Task 7 worker correctly
invoked `gate.drive.start` in the foreground for a required mutation test, but the worker's nested
shell call used a 30-second harness yield while Docket's `productionSlice` was also 30 seconds. At
30.2 seconds the shell wrapper returned an empty `output` plus the gate command and test were still
live. The wrapper emitted only that empty `output`, did not preserve the returned live shell-session
identity, and concluded that the native gate had produced no protocol response. Approximately 2.6
seconds later the original command completed normally with a valid `WAITING` document for drive
`505277339b6d5b203a18447822bbfe1f`, including its ownership generation. The worker never consumed
that response and returned `BLOCKED`. Parent takeover later advanced the same drive to `FAILED`,
which was the expected red result of the deliberately removed `uninstall` capability annotation,
not a native-driver failure. The run halted with the worker's Task 7 edits preserved.

The immediate defect in 0323 was loss of the yielded shell-session identity, but the architectural
problem is broader: an LLM agent currently participates in the gate's observation cadence. Each
slice boundary wakes a model, spends tokens, and makes correctness depend on the interaction between
Docket's observation duration and a harness's independently versioned foreground-call behavior.
Shortening `productionSlice` to 20 or 25 seconds would create timing headroom, but it would not be a
correctness boundary and would increase model wake-ups for long suites. More prose telling agents to
remember a session is also insufficient on its own; this failure family has repeatedly survived
instruction-only mitigations.

Move repetitive gate observation into a Docket-owned supervisor process. The supervisor should
advance the native process to a durable terminal disposition without an agent making one call per
slice. The workflow owner still controls who may consume, hand off, or acknowledge the result, but
agent ownership should no longer be what causes process observation to happen. The common case then
costs one model activation to start the gate and one to consume its terminal result, rather than one
activation every observation slice.

## What changes

- Introduce a Docket-owned autonomous gate supervisor above the existing raw process supervisor and
  gate-drive state machine. Starting a drive should validate the execution identity, allocate the
  durable drive, launch the test plus its supervisor, and return the drive identity promptly rather
  than spending a full observation slice in the initiating agent call.
- Make the supervisor, not an LLM caller, own the repetitive observation cadence. It should observe
  the detached native run, enforce the persisted deadline, apply the existing bounded relaunch
  policy where eligible, distinguish `WAITING`/`PASSED`/`FAILED`/`HALTED`, and write the authoritative
  durable drive record.
- Separate monitoring authority from workflow-consumption ownership. The supervisor may advance
  process state to a terminal record, while the current workflow owner's opaque generation remains
  required to consume, hand off, claim, take over, or acknowledge that result. A stale or
  superseded owner must gain no authority merely because the supervisor finished.
- Recompute the repository/execution fingerprint before accepting a terminal pass. Identity drift,
  ambiguous ownership, supervisor-state corruption, or an exhausted deadline must retain their
  existing fail-closed `HALTED` meaning; only a completed red suite is `FAILED`.
- Define one durable collection/await path for the workflow owner. A harness may return a live
  task/session identity while that collection command is still running; that identity is a
  nonterminal result and must be retained and drained through the harness-native wait mechanism
  before output is parsed or declared missing. Empty output while the session is live is never a
  gate response and never permission to rerun `start`.
- Preserve task-level handoff semantics for forked workers. A worker that cannot remain active until
  the terminal result must return only after creating a valid handoff. The parent controller claims
  the same drive, collects the supervisor's durable result, and resumes agent judgment from that
  result. No design may depend on a completion notification waking the forked worker.
- Make supervisor launch and recovery idempotent. There must be at most one authoritative supervisor
  and one test attempt for a drive. A repeated start, controller continuation, supervisor crash, or
  process restart must recover the existing durable identity or halt explicitly, never create a
  duplicate suite. Reconcile this requirement with change 0375's gate-start idempotency work rather
  than introducing a competing identity rule.
- Define supervisor lifecycle and cleanup: establishment must be acknowledged before the initiating
  command returns; terminal records must survive the launching shell; abandoned or crashed monitors
  must be detectable; cleanup must never erase the only terminal evidence before it is consumed.
- Retain the existing prohibition against agent-authored background loops, raw observe/sleep loops,
  and notification waits. The autonomous supervisor is a Docket process with a durable state
  contract, not permission for workflow agents to recreate background shell machinery.
- Keep the public gate outcome vocabulary unless the design proves a new state is necessary. Prefer
  evolving `gate.drive.start` plus a single collect/await operation over exposing raw process state
  or making callers parse supervisor internals.
- Add deterministic coverage for both known triggers:
  - a gate lasting beyond the harness's foreground-yield boundary returns promptly from start,
    completes under the supervisor, and is collected under the same drive identity;
  - a yielded collection call retains and drains its original live session instead of turning empty
    interim output into `BLOCKED`;
  - an intentionally backgrounded agent loop or completion report over a live drive is rejected;
  - worker-to-parent handoff consumes the supervisor's terminal result exactly once;
  - stale owners, duplicate starts, supervisor death, deadline expiry, and identity drift fail
    closed without launching a second test;
  - a terminal mutation-test failure remains trustworthy `FAILED` evidence and can continue the
    task's mutation workflow after restoration.
- Mutation-test every new structural guard: remove the session-draining requirement, reintroduce an
  agent-owned observation loop, or bypass the single-supervisor identity and prove the relevant
  guard turns red.
- Re-probe the supported harness modes and versions whose behavior the design relies on. The 0323
  Codex reproduction is a required acceptance case, but the workflow contract and supervisor
  semantics must remain harness-neutral.
- Measure the model-facing effect. A long suite should require model participation at start and
  terminal consumption, not one model activation per internal observation interval. Record any
  unavoidable harness-session polling separately from Docket gate-drive polling.

## Out of scope

Continuing implementation work after a test completes still requires an agent; this change moves
test monitoring, not coding or repair judgment, into a process. It does not add a general-purpose
harness notification service, weaken task/parent ownership boundaries, adopt another worker's
uncommitted files, or repair unrelated suite performance. It does not replace the raw process
supervisor's cross-platform session guarantee, change test-command selection, or redesign run-gate
attribution beyond what the autonomous supervisor requires. Change 0405's completed
prepare-scope/start lifecycle work and change 0375's in-progress start-idempotency work remain
separate, related inputs. Merely lowering `productionSlice` is not an acceptable complete fix.

## Open questions

- Should the autonomous monitor be a new per-drive supervisor process, an extension of the existing
  raw process supervisor, or a long-lived mode of the gate-drive CLI? Preserve the existing layer
  boundaries unless evidence shows that a new process boundary is required.
- Should `gate.drive.start` always return promptly with `WAITING`, or may it still return a terminal
  result for a test that finishes during establishment? Whichever answer is chosen must have one
  deterministic consumption contract.
- What is the single owner-facing collection surface: a blocking `await`, a short `collect`, or both?
  Avoid recreating agent-driven periodic polling under a new verb.
- How does a controller wait for or discover terminal supervisor state without repeated model
  activations on harnesses that cannot deliver a completion notification to a forked child?
- What durable lease or establishment record proves that exactly one supervisor owns a drive, and
  what recovery is safe if that supervisor dies while the detached test remains live?
- Which identity checks belong inside the autonomous monitor, and which remain mandatory only at
  owner transitions and terminal consumption?
- Does `productionSlice` remain as an internal supervisor observation bound, become event-driven,
  or disappear? Its value must no longer determine model wake-up cadence.
- Which current gate-drive APIs remain source-compatible during migration, and can the old
  agent-advanced path be removed rather than maintained as a second behavior?
- Which harness/version probes are required before merge, and how will the regression suite cover
  the 0323 lost-session race without depending on wall-clock flakiness?
