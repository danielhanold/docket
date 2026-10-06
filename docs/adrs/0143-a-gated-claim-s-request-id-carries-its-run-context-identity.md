---
id: 143
slug: 'a-gated-claim-s-request-id-carries-its-run-context-identity'
title: 'A gated claim''s request id carries its run-context identity'
status: 'Accepted'
date: '2026-10-06'
supersedes: []
reverses: []
relates_to: [142]
change: 531
---

## Context

claimRequestID was `claim-<id>-<revision>` for every claim, while the claim digest (change 0407) also binds the run-context hash. Two dispatches with distinct run contexts racing to claim one change therefore shared a request id. The lease-push CAS let exactly one win, but the loser's retry found the winner's receipt with a different digest and failed `invalid-input` (`request-id-reused`) instead of `contended`, which drivers continue past.

## Decision

A keyed (gated) claim's request id is `claim-<id>-<rev>-<first 16 hex of the run-context hash>`; an ungated claim keeps `claim-<id>-<rev>`. A racing loser with a different run context then hits the ordinary exact-revision check and reports `contended`. The same claim retried with the same context replays as already-claimed. The digest still binds the full run-context hash, so a prefix collision falls back to refusal, never cross-context replay. Sixteen hex characters keep the id within the 128-byte request-id limit for 64-hex revisions.

## Consequences

Concurrent gated claims of one change resolve to `contended` for the loser, which drivers continue past. Idempotent same-context retries are unchanged. During an upgrade, a gated claim reserved by an older binary and retried by the new one gets a run-context-conflict refusal that writes nothing.

## Alternatives considered

Relabel `request-id-reused` as `contended` on claims: rejected, because it turns a typed engine invalid-input failure into contention by matching stage/kind, loses ContendedPaths, and weakens the verdict continuity check. Put the full run-context hash in the id: rejected, because the id reaches 141 bytes, over the 128-byte limit.
