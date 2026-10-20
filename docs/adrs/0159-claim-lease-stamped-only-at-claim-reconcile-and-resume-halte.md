---
id: 159
slug: 'claim-lease-stamped-only-at-claim-reconcile-and-resume-halte'
title: 'Claim lease stamped only at claim, reconcile, and resume-halted; refresh-claim deleted'
status: 'Accepted'
date: '2026-10-20'
supersedes: []
reverses: []
relates_to: [158]
change: 360
---

## Context

implement-next paid a coordination tax of four change.refresh-claim calls per run to keep the claim lease (claimed_at) fresh. The lease's only consumer is reclaim, which already refuses while the feature branch (local or remote), a live workspace, or a gate run exists, and the feature branch exists from workspace.prepare onward. Design spec: .docket/docs/superpowers/specs/2026-10-20-cut-implement-next-coordination-tax-context-after-claim-sess-design.md.

## Decision

The claim lease (claimed_at) is stamped only by change.claim, change.reconcile, and change.resume-halted, and cleared on leaving in-progress. The explicit change.refresh-claim operation is deleted: its CLI command, capability catalog entry, schema, and install allow rule. implement-next makes no refresh calls.

## Consequences

Removes four metadata round-trips per run that protected nothing reclaim did not already guard. Accepted at grooming: from another clone the feature branch is invisible until workspace.publish, so a run could be reclaimed cross-clone only if it ran longer than reclaim.lease_ttl (72h default) between reconcile and publish and someone ran reclaim in that clone (reclaim.auto defaults off).

## Alternatives considered

Keep the refresh calls: rejected, four calls per run protecting nothing. Re-stamp the lease from change.attach-plan / change.attach-results: considered and declined.
