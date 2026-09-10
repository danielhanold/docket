---
id: 117
slug: 'sequential-test-drives-within-one-worker-recovery-scope'
title: 'Sequential test drives within one worker recovery scope'
status: 'Accepted'
date: '2026-09-10'
supersedes: []
reverses: []
relates_to: [107]
change: 405
---

## Context

A build-task recovery scope was one test execution: a single BoundDriveID was permanently bound to the scope at its first start, so a worker could not run baseline, RED, GREEN, and verification as separate drives under one parent-issued scope — the second scoped start was rejected. ADR-0107 established event-authorized direct-parent takeover and the one-live-drive rule, and those properties must survive any change to the scope lifecycle.

## Decision

A recovery scope represents one parent/child dispatch, not one test execution. It serializes a *sequence* of task-owned drives through three mechanisms: (1) a durable pre-launch reservation persisted under the scope CAS before any process launch, so an interrupted launch is always resolvable from durable state rather than inferred; (2) an explicit predecessor-acknowledgement receipt — the previous drive id plus its owner generation, presented both-or-neither — that each successor start must present to acknowledge exactly the PASSED/FAILED result it received, retiring the predecessor's recovery authority as the journaled second half of one logical transition; and (3) a cataloged terminal-acknowledgement operation, gate.drive.acknowledge, that consumes the final result and closes the scope. At most one current execution or launch reservation exists per scope at any time. A new test is always a new drive — never a relaunch of a prior drive, and never reuse of prior evidence. Typed OwnershipErrorKind reasons (stale-predecessor, predecessor-not-reusable, unresolved-launch-transition, scope-busy, plus the existing scope kinds) replace the generic invalid-request diagnostic. Lock order is scope lock before drive lock within one logical transition, and every transition revalidates its target after acquiring authority.

## Consequences

Workers run their full local test cycle — baseline, RED, GREEN, verification — under one parent-issued scope without re-obtaining parent authority per test. Recovery, takeover, and enumeration resolve only the scope's current drive: acknowledged predecessors drop out because their owner generation is cleared, so a retired drive can never be taken over or relaunched. Ambiguous launch or persistence fails closed with no automatic second launch — a duplicated test process is never a possible outcome of an interrupted start. The scope record schema bumps to v2; a v1 or unknown record fails closed rather than being interpreted. ADR-0107's one-live-drive rule is preserved and now read as "at most one current execution or launch reservation per scope", and its event-authorized direct-parent takeover is unchanged. The cost is added durable state (the launch reservation plus the pending-acknowledgement journal) and one more cataloged operation to keep in the catalog and its schema surface.

## Alternatives considered

Rebinding the scope to a fresh drive id on each start without an acknowledgement receipt was rejected: nothing would prove which result the successor observed, so a stale predecessor could still be recovered or taken over. Issuing a new parent scope per test was rejected: it pushes per-test round trips into the parent, defeating the point of a worker-owned recovery scope and multiplying the takeover surface. Reusing one drive record across tests (relaunch semantics) was rejected because it conflates distinct test executions and their evidence, making a PASSED/FAILED result unattributable to a specific run. Inferring an interrupted launch's outcome from process state instead of a durable pre-launch reservation was rejected as unsound — it can neither distinguish a launch that never happened from one whose record was lost, nor avoid a second launch.
