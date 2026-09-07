---
id: 112
slug: 'a-completed-gate-publish-checkpoint-is-persisted-in-the-owne'
title: 'A completed-gate publish checkpoint is persisted in the owned rebase receipt'
status: 'Accepted'
date: '2026-09-07'
supersedes: []
reverses: []
relates_to: [105, 98]
change: 408
---

## Context

A finalize that performs a real rebase runs the local gate on the rewritten head and records green evidence, then publishes with a force-with-lease push. When the publish is denied (most sharply by a host permission classifier, so the binary never runs), nothing publish-specific persisted: on resume, recoverFromReceipt sees the local head is no longer the receipt's OrigHead, so composeLocalGate re-runs the full suite for a head it already certified. ADR-0105 persists the gate's LIVE continuation (drive id + owner generation) so a WAITING drive resumes; it does not persist COMPLETED evidence, and its own consequence note named this residue.

## Decision

When the local gate reaches PASSED for a real rebase, the owned rebase receipt additionally records a completed-gate publish checkpoint: the tested head, the effective base head, the byte-exact resolved finalize.test_command, the gate policy, the open PR number, and the green evidence block — written in the same atomic receipt write that clears the continuation pair, all-or-none, mutually exclusive with a live pair, keeping the receipt ==-comparable. On a resume with no rebase in progress and the head descending the base, a checkpoint whose every recorded identity still matches current reality (and whose evidence re-verifies green for the exact current head) skips the suite and returns the recorded evidence for publication; any mismatch clears the checkpoint and the gate re-runs. The identity and moved-base refusals stay ahead of any reuse. Host-denial detection stays a skill/harness concern — no Go primitive distinguishes a denial from a Go result, because a denied binary never ran.

## Consequences

A denied publish costs a resume, not a suite re-run; stale evidence can never publish (every conjunct is equality-checked and the evidence is re-verified); the receipt gains six optional scalar fields every existing equality gate (PublishRewrite's decide-and-act copy check, the write round-trip guard) covers automatically; a checkpoint that cannot be persisted degrades to today's behavior (the gate re-runs), fail-closed.

## Alternatives considered

Extend ADR-0105's live continuation pair to also encode a completed state — rejected: a WAITING drive and a finished gate have different identity requirements, and overloading the pair would make the mutually-exclusive invariant unstateable. Detect the host denial in Go and branch on it — rejected: a denied binary never runs, so no Go primitive can observe the denial. Cache gate results keyed by head outside the receipt — rejected: the receipt is already the owner-held authorization surface (ADR-0098), and a second store would need its own ownership and atomicity story.
