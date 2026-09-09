---
id: 416
slug: 'scoped-build-task-gate-starts-omit-prepared-scope-identity'
title: 'Scoped build-task gate starts omit prepared scope identity'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-09-09'
updated: '2026-09-09'
depends_on: []
stacked_on:
related: [359, 376, 405]
discovered_from: [323]
adrs: [107]
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
| ADRs | [ADR-0107](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0107-event-authorized-parent-takeover-extends-fingerprinted-gate.md) |
<!-- docket:artifacts:end -->

## Why

Change 0323 halted before its first implementation task because the dispatched build worker followed the maintained docket-build-task instructions and invoked gate.drive.start with the scope id, child capability, and run root, but without the change id, task id, phase, or branch pinned by gate.drive.prepare-scope. The driver correctly rejects those empty values through scopeIdentityMatch, and the CLI maps that ownership failure to invalid-input / invalid-request with no drive. The failure reproduces with the documented call shape; reusing the same scope while adding the four pinned identity fields produces an applied, passing drive. The source, embedded, and installed worker contracts contain the same omission, while the CLI regression always supplies every field and the existing workflow guard checks JSON capture but not scope-identity propagation. As written, every scoped build-task worker can halt before running its first test.

## What changes

Update the build controller's task dispatch payload to carry one explicit, complete start-ready scope tuple: change id, task id, phase, branch, worktree, scope id, child capability, and outer gate context when present. Update the build-task worker contract so every task-owned gate.drive.start passes that tuple unchanged, including the required identity flags and gate context. Add a whole-repository syntactic guard over scoped task-start instruction sites that derives the required identity shape and fails when any required field is omitted; mutation-test each protected field so the guard cannot pass vacuously. Regenerate embedded skill assets through the existing generator and verify the installed/generated surfaces remain aligned. Acceptance requires a worker-following round trip to bind a drive and the old omitted-field shape to remain fail-closed. This is a bounded caller-contract repair: it changes no CLI field, protocol, driver transition, or authorization rule, so no separate architecture decision is needed.

## Out of scope

Changing the gate driver to infer or hydrate omitted scope identity; weakening scopeIdentityMatch; changing capability, handoff, takeover, or credential-redaction semantics; redesigning the invalid-request error vocabulary; resolving broader concurrent-scope or duplicate-drive questions tracked by change 0405; and resuming or implementing change 0323.
