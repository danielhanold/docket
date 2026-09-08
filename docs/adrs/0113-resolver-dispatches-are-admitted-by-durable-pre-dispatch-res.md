---
id: 113
slug: 'resolver-dispatches-are-admitted-by-durable-pre-dispatch-res'
title: 'Resolver dispatches are admitted by durable pre-dispatch reservation'
status: 'Accepted'
date: '2026-09-08'
supersedes: []
reverses: []
relates_to: [10, 19, 105]
change: 349
---

## Context

Finalize's rebase-resolver dispatch cap was a skill-memory counter ("at most 2 attempts") held only in the running agent's context. A resumed or restarted session could not recover it, and it could not truly bound native dispatches: a check performed at continue time runs after the child has already executed, so the dispatch it was meant to admit has already happened. Change 334 exposed the practical cost from the other direction — three successive conflicting commits exceeded the hard-coded ceiling even though every conflict was mechanically resolvable, so a healthy rebase was blocked by a number nobody could tune.

Two problems therefore had to be solved together: the ceiling had to become configurable per repository, and its enforcement had to move out of agent memory into durable state that survives interruption, restart, and a lost response.

## Decision

Make the ceiling a configurable positive integer `finalize.resolver_max_attempts` (default 3, minimum 1), resolved through the normal repo-local / repo-committed / global / built-in layers and classified like `finalize.gate` under ADR-0019.

Enforce it durably in Go via a versioned resolver-budget group on the owned `RebaseReceipt` — limit, used, reservation token, stopped commit, continuation-started — snapshotted when a fresh owned rebase begins.

Admit every native `docket-rebase-resolver` dispatch through a new `finalize.resolver-reserve` operation. Under the per-workspace operation lock it checks capacity BEFORE incrementing and returns `reserved` only after the durable receipt write and its round-trip verification both succeed.

`FinalizeRebaseContinue` verifies the reservation before staging or continuing — the token plus the live stopped commit, so identical conflicted paths never rescue a stale reservation — marks continuation-started durably before the git mutation, and consumes or clears the reservation only once the git outcome has been reconciled.

## Consequences

Reserve-before-dispatch lives in Go, because a continue-time check cannot bound dispatches the child has already performed; the admission decision must precede the launch.

Consumption is deliberately conservative: a durably reserved dispatch opportunity is never refunded. A failed launch or a lost success response spends the opportunity, so the configured bound cannot be overrun across interruptions and restarts. The cost is that a genuinely wasted reservation is not recovered.

Legacy receipts — those whose whole budget group is absent — are recognized as legacy and never freshly budgeted. New reservation and new continuation refuse with `resolver-budget-unavailable`, while owned abort stays available so such a rebase is not stranded.

The limit is snapshotted per receipt, so a config change made mid-attempt applies only to the next fresh attempt rather than retroactively widening or narrowing a rebase already under way.

The reservation lives inside the existing receipt — no separate ledger, no second source of truth to reconcile. Post-continuation exhaustion reuses the existing rebase `blocked` disposition with reason `resolver-budget-exhausted`; only the reservation result itself uses the `exhausted` disposition, so the rebase vocabulary does not grow.

## Alternatives considered

Keep the skill-memory counter and merely make its number configurable. Rejected: the counter is unrecoverable across a resumed session and, being read after the child ran, cannot bound dispatches at all — configurability without durability would have left change 334's failure mode intact in a new form.

Check the budget at continue time only. Rejected for the same ordering reason: by then the dispatch has already been spent, so the check reports an overrun instead of preventing one.

Refund a reservation when a dispatch is observed to have failed. Rejected: the observation itself is unreliable exactly when it matters (a lost response is indistinguishable from a slow one), and an optimistic refund reintroduces the overrun the reservation exists to prevent.

Store the budget in a separate ledger keyed by workspace. Rejected: it duplicates lifetime and cleanup rules the `RebaseReceipt` already owns, and creates a second thing that can drift from the receipt.

Treat a legacy receipt as freshly budgeted. Rejected: it would silently grant a full new allowance to a rebase already partway through an unknown number of dispatches; refusing new work while keeping abort available is the safe reading.

Add new rebase dispositions for exhaustion. Rejected in favour of the existing `blocked` disposition with a reason token, keeping the rebase vocabulary fixed.
