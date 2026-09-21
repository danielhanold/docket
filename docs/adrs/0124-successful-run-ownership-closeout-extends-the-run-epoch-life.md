---
id: 124
slug: 'successful-run-ownership-closeout-extends-the-run-epoch-life'
title: 'Successful-run ownership closeout extends the run-epoch lifecycle with completing and completed'
status: 'Accepted'
date: '2026-09-21'
supersedes: []
reverses: []
relates_to: [118]
change: 441
---

## Context

ADR-0118 established worktree-wide gate admission and explicit human-cancellation authority: a live run epoch owns its worktree, and only an explicit human cancellation releases that ownership. That decision stands unchanged. It left one gap: a run that *succeeded* had no lifecycle path to release ownership. A verified-complete implementation run kept its worktree epoch live, so a subsequent standalone finalize gate over the same worktree either hit a stale-run-epoch refusal or required a human to cancel a run that had in fact finished successfully — encoding success as cancellation, which is both wrong in the record and unavailable to an autonomous flow.

Change 0441 closes that gap. The forces: ownership must stay fully exclusive while implementation has any outstanding work; release must be driven only by an authority that actually knows the run succeeded; the closeout must never be confusable with cancellation; and the accounting that proves nothing is still running must be at least as strict as cancellation's, while stopping nothing itself.

## Decision

Extend the existing run-epoch lifecycle with two new states, alongside (never replacing) the cancellation states ADR-0118 governs:

- `completing` — a durable success fence. The attributed, keyed RunGateVerdict has verified `run-complete`, but ownership accounting and retirement are unfinished. The epoch STILL owns its worktree: it admits no new registration, start, mutation, takeover, or relaunch.
- `completed` — successful closeout finished. Terminal. Excluded from ambient worktree-owner lookup, while explicit references to it stay revoked.

Rules:

1. **Only the attributed, keyed RunGateVerdict path drives successful closeout.** RunVerify stays read-only; unattributed and observe-mode verdicts never change ownership.
2. **Success is never encoded as cancellation.** `completing`/`completed` are distinct states from `cancelling`/`cancelled`. A cancelling, cancelled, superseded, or identity-mismatched run is never relabelled successful.
3. **Closeout accounting is observation-only.** It stops no process, signals nothing, and settles no never-launched reservation. It fails closed on any live, busy, pending, uncertain, or unreadable obligation. It reuses cancellation's accounting shapes through observation-only helpers over one shared inventory, so the two paths cannot drift.
4. **Exact native-participant terminal observation is persisted** in the existing participant records at the adapter boundary. Registration alone and the owner-complete marker cannot establish quiescence; a terminal *failure* is termination evidence too, and RunVerify independently decides implementation success.
5. **Explicit human cancellation wins from `completing`** — completion then loses, and does not report success. Cancelling an already-`completed` run is a no-op refusal, never a state regression.

This lets a standalone finalize gate reuse the same worktree after a verified-complete run without a stale-run-epoch refusal and without a human cancellation, while preserving full exclusion whenever implementation still has outstanding work.

## Consequences

- A standalone finalize gate can admit over a worktree whose prior implementation run verifiably completed, with no human in the loop and no cancellation record fabricated for a successful run.
- Exclusion is unchanged wherever it matters: from `completing` the epoch still owns the worktree, so a run with outstanding work can never be displaced.
- Record compatibility is additive: the run-epoch schema stays v1, with the new fields `omitempty`. Older records read forward unchanged.
- Missing historical terminal evidence means **unproven**, never implicitly complete — closeout fails closed, and a pre-existing epoch without persisted participant termination evidence simply does not close out automatically.
- Existing cancellation and resume behavior is preserved verbatim; ADR-0118's admission model and human-cancellation authority remain binding, and this ADR extends rather than narrows them.
- ADRs 0087, 0095, 0105, and 0111 remain binding and unmodified.
- Cost: two more lifecycle states and a second accounting entry point to keep in step with cancellation's — mitigated by routing both through one shared inventory and observation-only helpers rather than a parallel implementation.

## Alternatives considered

- **Reuse cancellation for successful runs** (mark the completed epoch `cancelled` so the worktree frees). Rejected: it falsifies the record, makes success indistinguishable from an abort in every downstream read, and would require an autonomous flow to invoke the explicitly human-only cancellation authority ADR-0118 reserves.
- **Require a human cancellation after every successful run.** Rejected: it puts a human in the loop on the happy path and gives the same false record as above.
- **Release ownership atomically at the verdict, with a single `completed` state and no fence.** Rejected: ownership accounting is not instantaneous, and a window with no owner would admit a competing registration while the prior run's obligations were still resolving. The `completing` fence exists precisely to keep exclusion unbroken across that window.
- **Let RunVerify (or an unattributed/observe-mode verdict) drive closeout.** Rejected: RunVerify is read-only by contract and an unattributed verdict cannot establish which run it speaks for; ownership must only change on an attributed, keyed authority.
- **Treat registration or the owner-complete marker as sufficient quiescence evidence.** Rejected: neither proves a native participant terminated, so closeout could release a worktree with live work attached. Exact persisted terminal observation — including terminal failure — is required, and its absence means unproven.
- **Have closeout stop or signal remaining participants.** Rejected: that duplicates cancellation's authority on a path that no human authorized. Closeout observes and fails closed instead.
