---
id: 128
slug: 'resume-arms-mint-an-arm-time-epoch-that-run-cancel-can-cance'
title: 'Resume arms mint an arm-time epoch that run.cancel can cancel without a claim binding'
status: 'Accepted'
date: '2026-09-28'
supersedes: []
reverses: []
relates_to: [111, 118]
change: 463
---

## Context

A `run.gate-before implement-next --resume <id>` for a change with no prior run epoch minted no epoch, so the armed line carried two tokens and parents misread the dispatch context as the epoch (seen on change 0382). Change 0463 makes that resume mint an epoch and bind it to the change id and the verified feature worktree at arm time. A resume arm never gets a claim binding, because change.claim requires a proposed change. run.cancel required a confirmed claim binding (ADR-0111's attribution proof, ADR-0118's human-cancellation authority keyed by gate key plus epoch), so it could never cancel that epoch, and every later resume of the change was stuck behind resume-active-run.

## Decision

1. run.cancel accepts a second ownership proof beside ADR-0111's confirmed claim binding: the resume-verified shape, a gate record with AttributedID set, BoundRequestID empty, and no claim-binding file at all (predicate `GateRecord.resumeAttributed` in internal/app/rungate_store.go, used by runCancel, resolveGateOwnership, and ChangeClaim). The attributed id must still equal the epoch's change id when the epoch names one. An unconfirmed reservation still refuses claim-unconfirmed; the key, epoch, repository, and parent-held authority checks are unchanged. Epochs from the existing resume-after-cancel replacement path, which also never had a claim binding, become cancellable too.

2. change.claim refuses a resume arm's dispatch context as gate-context-conflict before reserving anything, so a stray claim cannot leave an unconfirmed reservation (claim-unconfirmed) or a claim of another change (claim-mismatch) that would make the resume epoch uncancellable again.

3. Resume arms of one change are serialized by a per-change resume lock under `<git-common-dir>/docket/rungate-resume/<change-id>`, held from the prior-epoch check through the epoch bind. Without it, 12 of 12 simultaneous arms each minted a live epoch in test.

4. The resume epoch is bound at arm time, deliberately. An earlier resume arm that was never dispatched blocks the next resume (resume-active-run) until it is cancelled with run.cancel, matching the resume-after-cancel path where a repeat arm refuses and points at the reserved replacement. Nothing records whether an agent is using an epoch, so the refusal names both remedies: cancel if never dispatched or its agent exited; run gate-verdict if still running.

This supersedes and reverses neither ADR-0111 nor ADR-0118: run.cancel stays keyed by gate key plus epoch, and the claim binding remains an accepted proof.

## Consequences

Resumed runs, including resume-after-cancel replacements, are now cancellable, so a resume can no longer strand a change behind resume-active-run permanently. The armed line for a resume always carries key, epoch, and dispatch context. Cost: an undispatched resume arm must be explicitly cancelled before the next resume, and ownership now has two accepted proofs that every ownership check (runCancel, resolveGateOwnership, ChangeClaim) must evaluate through the shared predicate. The per-change resume lock adds a lock directory under the git common dir.

## Alternatives considered

- Bind the resume epoch lazily at the child's first use: for a Claude Code dispatch the first epoch-carrying call is the child's first gate drive (after reconcile, planning, workspace prepare), so a second agent could be dispatched into the worktree for most of the run; the epoch launch gate is specified read-only (no epoch write on the liveness path); and it would need a cross-epoch lock at bind time anyway.
- Make a repeat resume arm reprint the same armed line: impossible, the dispatch context is never stored (only its hash).
- Let a new arm take over an epoch that looks unused: a just-dispatched agent that has made no docket call yet is indistinguishable from an abandoned arm.
- Check-after-bind re-scan with an id tie-break: an arm can finish its check and report armed before a racer binds, so a tie-break can crown the racer while the first is also armed; the lock closes the window instead.
- Keep a confirmed claim binding as the only cancel authority: resumed runs could never be cancelled.
