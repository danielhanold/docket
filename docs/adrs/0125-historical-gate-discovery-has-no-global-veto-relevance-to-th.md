---
id: 125
slug: 'historical-gate-discovery-has-no-global-veto-relevance-to-th'
title: 'Historical gate discovery has no global veto — relevance to the requested worktree or run decides whether uncertainty blocks'
status: 'Accepted'
date: '2026-09-23'
supersedes: []
reverses: []
relates_to: [87, 95, 118, 120, 124]
change:
---

## Context

Change 0446: an orphaned HALTED gate drive (or any unrelated, unmatchable, unreadable, or obsolete historical gate record) blocked every new worktree's first admission, because first-admission legacy inventory, the epoch launch census, and cancellation/resume/closeout accounting treated any uncertain historical record as possible ownership of the requested worktree. ADR-0118 scoped the admission inventory worktree-wide and allowed raw-run release only on explicit stop; ADR-0120 treated an unresolvable historical path as possible ownership of every worktree. Together these gave one stale record a global veto over all new drives.

## Decision

1. Relevance decides. First-admission legacy inventory, the epoch launch census, and cancellation/resume/closeout accounting treat historical gate records as diagnostic unless positively bound to the requested canonical worktree or run — via the current slot, scope, epoch, reservation token, or participant link. Unrelated, unmatchable, unreadable-unreferenced, or obsolete records never veto a new drive; they stay visible through history cleanup diagnostics.
2. Authoritative local uncertainty stays protected. A corrupt or missing record positively named by the target's current slot, scope, or epoch still refuses with an exact locator. A probe error is never treated as clean absence.
3. Proven-finished incumbents may be settled. Admission may settle a proven-finished incumbent (a PASSED/FAILED drive, or a raw run with positive teardown proof via ClassifyRun) through the existing exact-token CAS, and may retire a released slot whose recorded run epoch is settled (completed, cancelled, or superseded). This deliberately narrows ADR-0118's explicit-stop-only raw release. HALTED alone is never release proof.
4. Worktree owner resolution is deterministic: an active owner wins over fenced predecessors; ambiguity is a typed refusal; a slot-named epoch that cannot be resolved fails closed.
5. Slots remain addressable by stored identity after the worktree path is removed.

Partial supersession: this decision supersedes only the conflicting clauses of ADR-0118 (worktree-wide admission inventory scope; raw explicit-stop-only release) and ADR-0120 (an unresolvable historical path as possible ownership of every worktree). All other guarantees of ADR-0118 and ADR-0120 remain in force, so neither is marked Superseded. ADR-0087/ADR-0095 process-evidence rules and ADR-0124 observation-only closeout remain binding.

## Consequences

A stale or orphaned historical record no longer blocks unrelated worktrees; new drives proceed and the record is surfaced as a cleanup diagnostic instead. Safety for the target itself is preserved: records the target's own slot/scope/epoch names still fail closed with a locator, and probe errors never read as absence. Admission gains a narrow settle/retire path for provably finished incumbents, reusing the exact-token CAS rather than a new write path. Cost: relevance binding must be computed precisely; a missed binding link would under-protect, so binding is positive-evidence only. Readers of ADR-0118/0120 must consult this ADR for the narrowed clauses.

## Alternatives considered

Keep the global veto and require manual history cleanup before any new drive (rejected: one orphan halts all work). Fully supersede ADR-0118/0120 (rejected: most of their guarantees still hold). Treat HALTED as release proof (rejected: a halted drive may still own live work needing a human). Treat probe errors as absence (rejected: silently unsafe).
