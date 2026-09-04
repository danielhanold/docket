---
id: 405
slug: 'investigate-the-gate-drive-prepare-scope-gate-drive-start-ha'
title: 'Investigate the gate.drive.prepare-scope -> gate.drive.start handshake rejecting a build-task worker''s focused gate'
status: 'proposed'
priority: 'medium'
type: 'fix'
created: '2026-09-04'
updated: '2026-09-04'
depends_on: []
stacked_on:
related: []
discovered_from: [402]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

During the change-402 build, two build-task workers (Tasks 12 and 15's fix) reported that `docket gate drive start`, handed the exact scope-id and child-capability that `docket gate drive prepare-scope` had just minted for their task, returned `invalid-input` / `reason: invalid-request` on every attempt (they varied run-root, branch, and change-id). A deliberately bogus scope-id instead returns `not-found`, so the scope was recognized as real but its state refused a fresh `start` — the child-capability appears already consumed, or the scope is not in a startable state. The workers correctly fell back to running their fast, sub-second focused test (`go test ./internal/repoguard/`) directly in the foreground, so no build was harmed; the controller's own full-suite gate (driven through `gate.drive.start --owner build` at top level) worked without issue in the same run. But a prepare-scope handshake that a worker cannot start is either a real defect in the driver's scope/capability state machine or a contract ambiguity about who may `start` inside a prepared scope, and it matters because docket runs multiple implement-next loops concurrently: changes 261 and 404 were both in-progress alongside 402 when this was observed. If the recovery-scope handshake is unreliable, a real WAITING->handoff->claim recovery could silently fail to re-establish a drive under load. This is the run-gate / gate-driver subsystem introduced across the #271 / #342 / #345 / #359 work; the anomaly is at the prepare-scope <-> start seam, not the top-level owner-build path that this change exercised cleanly.

## What changes

Reproduce the handshake failure with a controlled probe: call `gate.drive.prepare-scope` for a (change-id, task-id, phase=build, branch, worktree) tuple, then immediately call `gate.drive.start` with the returned scope-id and child-capability, and capture the exact rejection. Determine the ROOT CAUSE and answer the two questions this discovery raised: (1) Is `prepare-scope` intended to be paired with `gate.drive.start --scope-id --child-cap` at all for a build-task worker's OWN focused gate, or is prepare-scope only for the parent/child RECOVERY-takeover path (parent keeps parent-cap, child inherits a drive the parent started) so that a fresh `start` inside a not-yet-started scope is simply an unsupported call the contract should reject more legibly? (2) Under concurrent builds, can the driver mis-identify or cross-bind which build/drive a scope belongs to — i.e. does the repo/execution fingerprint plus owner-generation actually keep two simultaneous build drives (e.g. for 261, 402, 404) from being confused, and does a scope minted for one (change,task,phase) tuple ever collide with another? Then fix accordingly: either correct the driver so a prepared scope accepts a legitimate first `start` (with a regression test that starts a drive inside a freshly-prepared scope and advances it to a terminal disposition), OR, if the pairing is genuinely unsupported, make the rejection a clear typed diagnostic naming the correct verb and tighten the caller-facing docs (docket-build's gate-caller-loop / gate-execution references and the build-task worker contract) so a worker never constructs the unsupported call. Include a concurrency test that runs two build drives against distinct worktrees at once and asserts each observes only its own terminal result.

## Out of scope

Reworking the run-gate attribution model or the driver's disposition vocabulary beyond what the root cause requires. Change 402's docs restructure (already implemented; this was only discovered during its build). The unrelated internal/app wall-clock finalize-blocker and any other pre-existing suite-timing findings.
