---
id: 420
slug: 'prevent-build-workers-from-assigning-zsh-s-read-only-status'
title: 'Prevent build workers from assigning zsh''s read-only status parameter'
status: 'proposed'
priority: 'critical'
type: 'fix'
created: '2026-09-10'
updated: '2026-09-10'
depends_on: []
stacked_on:
related: [263, 376, 416]
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

A resumed change 0323 build worker captured the first gate.drive.start response and then executed `status=$?`. In zsh, `status` is a read-only special parameter, so the shell aborted before the worker could parse the drive id and owner generation or make any TDD edits. The coordinator remained live without visible feature-worktree activity until a human intervened. The maintained agent contract requires JSON capture but does not give workers a shell-safe exit-code capture shape or mechanically prevent this reserved-name failure.

## What changes

Strengthen the maintained build-controller, build-task, and shared gate-caller instructions so gate responses and exit codes are captured with explicit shell-safe names such as `gate_reply` and `gate_rc`, compatible with both zsh and bash, and never assigned to zsh special parameters such as `status` or `pipestatus`. Add a whole-repository syntactic guard over maintained executable agent-facing workflow instructions that detects reserved-parameter assignments at gate-call capture sites, with a mutation test proving that inserting `status=$?` makes the guard fail. Regenerate embedded skill assets through the existing generator and keep source and distributed instruction surfaces byte-aligned.

## Out of scope

Changing gate-driver ownership, identity, continuation, or retry semantics; making gate starts idempotent; weakening fail-closed response validation; broadly linting immutable archives, accepted ADRs, historical plans, or unrelated shell code; and resuming or implementing change 0323 as part of this fix.
