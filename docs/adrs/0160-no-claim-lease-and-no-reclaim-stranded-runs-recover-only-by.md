---
id: 160
slug: 'no-claim-lease-and-no-reclaim-stranded-runs-recover-only-by'
title: 'No claim lease and no reclaim — stranded runs recover only by resume-by-id'
status: 'Accepted'
date: '2026-10-22'
supersedes: [159]
reverses: []
relates_to: [59, 84, 101]
change: 575
---

## Context

ADR-0159 settled where the claim lease (`claimed_at`, evaluated against `reclaim.lease_ttl`) is stamped: at claim, reconcile, and resume-halted. The lease existed for one job: letting `change.reclaim` (typed, or from the maintenance sweep when `reclaim.auto: true`) return an `in-progress` change with no branch and no live workspace to `proposed` once the lease expired. Reclaim arrived with #89, borrowed from a competitor's lease-and-reclaim feature, and was ported to Go in #316.

It has never fired. No change record in this repository's full metadata history, or in the other local repositories using docket, has ever carried a `## Reclaim log`; no saved session ran a real reclaim; `reclaim.auto` is set in no config layer. In every saved session the sweep reported a stale claim for exactly one change, #422, six times, and #422 had halted with a feature branch, so reclaim would have refused it and the board already showed it halted. Its one real window is already covered: the building clone has a local branch from `workspace.prepare` onward, minutes after the claim, which blocks reclaim; a run that dies before then is recovered by resuming the change by id (workspace-free since #368), with the run tracker's verdict deciding any retry. From another clone reclaim is an active risk: a live run's branch is invisible there until `workspace.publish`, so a reclaim after the TTL could hand back a change still being built (ADR-0159 recorded this as accepted).

Nothing else reads `claimed_at`: not the board, `docket status`, `context.implementation`, `change.relink`, or the run tracker, which binds a dispatch to its claim through claim proofs (ADR-0111).

Three Accepted ADRs carry passages this decision makes stale; their bodies are not edited:
- ADR-0059's consequence "the existing reclaim lease self-heals an abandoned claim" no longer applies (it was never true in Go: every halt in that row happens after `workspace.prepare`, so a branch exists).
- ADR-0101's implementation-scope schedule no longer includes `reclaim.auto`-gated reclaims.
- ADR-0084's `claimed_at` attribution filters were already replaced by ADR-0111's claim proofs.

## Decision

Docket keeps no claim lease and no reclaim.

- `change.reclaim` and the maintenance sweep's reclaim pass are deleted; no operation, sweep pass, config key, body section, or doc returns an `in-progress` change to `proposed`.
- `claimed_at` is never written, cleared, required, or read. Claim, reconcile, and resume-halted stop stamping it; kill, done, and complete stop clearing it. An existing `claimed_at:` line in an older record (empty, stamped, or malformed) is left exactly as it is by every write and loads as ignored compatibility data with no finding. Validation no longer requires it (`branch:` stays required for in-progress, implemented, and stacked-merged), and the claim receipt and result drop `claimed_at` and `lease`.
- `reclaim.lease_ttl` and `reclaim.auto` become obsolete tombstones: in every config layer each emits one `obsolete-setting` warning and is ignored, never resolving into effective config and never blocking an operation.
- A stranded run recovers only by resuming the change by id (`run.start ... --resume <id>`), with the run tracker's verdict deciding any retry. Otherwise a stranded change is blocked or killed.

## Consequences

- Removes a mechanism that never fired in practice: no `## Reclaim log` was ever written, and #422, the only stale claim ever reported, had already halted with a branch and was already shown as halted.
- Removes the cross-clone risk ADR-0159 accepted: no clone can hand back a change another clone is still building before its branch is published.
- There is no path from `in-progress` back to `proposed` at all (#568 already refuses deferring an in-progress change). A stranded change is resumed, blocked, or killed.
- No replacement stale-claim signal. The sweep's skipped-reclaim entries were the only place a stale claim showed up; a crashed coordinator now surfaces at the next `run.start ... --resume <id>` as `resume-active-run`.
- Upgrade together: there is no compatibility step for older binaries. A pre-change binary's validation treats a missing `claimed_at` on an in-progress, implemented, or stacked-merged record as `change-state-incoherent`, so it flags records the new binary claimed and refuses their reconcile, mark-implemented, and finalize writes. Every machine sharing a repository's docket metadata must upgrade before its next claim; on a single machine there is no gap because the binary is rebuilt right after the merge.
- Old configs keep loading: a `reclaim:` block in any layer warns and never makes an operation refuse.

## Alternatives considered

- **Keep `claimed_at` as a "claimed on" date.** Rejected: git history already records the claim commit, and a field nothing reads keeps writer, validation, and repair code alive for no reader.
- **Leave the lease in place (stamped but unused, or with reclaim kept opt-in).** Rejected: it never fired, its only window is covered by resume-by-id, and from another clone it is a live hazard; a lease nothing evaluates is dead weight with wrong documentation attached.
- **Delete the config keys outright.** Rejected: `reclaim:` would become an `unknown-key` error, and every operation would fail with invalid configuration for any repository or machine still carrying the block.
- **A silent tombstone (accept and ignore with no warning).** Rejected: the user would never learn the setting does nothing; the existing retired-key path (`finalize.skip_results_only_delta`, `runtime.bash`, `metadata_branch`) warns once per key, and these follow it.
