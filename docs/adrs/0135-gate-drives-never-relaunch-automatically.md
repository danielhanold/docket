---
id: 135
slug: 'gate-drives-never-relaunch-automatically'
title: 'Gate drives never relaunch automatically'
status: 'Accepted'
date: '2026-10-03'
supersedes: []
reverses: []
relates_to: [87, 98, 107, 132]
change: 493
---

## Context

Gate drives carried a single automatic relaunch: when a drive's supervisor died, an idempotent drive could launch a replacement supervisor, with crash-window machinery (reserved relaunches, recovery of never-launched replacements, run-liveness checks) to make that safe. Change 0493 began as a fix to the launch census, which could miss a replacement supervisor behind a halted, unattributed drive. Tracing it showed the relaunch itself was the problem: across 216 drive records it never fired once, it was the sole source of the lost-replacement census bug class, it was the reason change 0492's item 3 existed, and its crash-window machinery made up a large share of the driver. Design section 7 of docs/superpowers/specs/2026-10-02-cancel-misses-a-replacement-supervisor-behind-a-halted-unatt-design.md records this decision.

## Decision

A supervisor death halts every gate drive. The drive is written HALTED with cause `supervisor-died`, or `uncertain-ownership` when the death cannot be proven. No drive relaunches automatically; a human re-runs the workflow. The relaunch machinery is deleted.

Two compatibility rules hold. The per-drive claim file keeps its on-disk name `relaunch.lock`, so binaries on either side of the change still exclude each other. Relaunch fields in old drive records are ignored on read.

## Consequences

The census only ever handles one launch per drive, so the lost-replacement bug class is gone by construction. The driver loses its crash-window relaunch paths. ADR-0132's problem fact 6 (the relaunch run checks cannot be reached in production) now holds by construction.

Accepted cost: on the rare occasion a supervisor dies during finalize, a human has to re-run the workflow, where before the drive might have relaunched itself. It never did in practice.

The `relaunch.lock` file name no longer describes what the file does; it is kept for cross-binary exclusion.

## Alternatives considered

Harden the launch census so it finds replacement supervisors (the original 0493 scope). Rejected: it keeps a mechanism that never fired and keeps its bug class alive.

Forbid the relaunch only for run-attributed drives. Rejected: it keeps all the crash-window machinery for a path that is still unused, and adds a split rule.
