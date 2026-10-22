---
id: 575
slug: 'retire-claim-reclaim-and-its-lease'
title: 'Retire claim reclaim and its lease'
status: 'in-progress'
priority: 'medium'
type: 'refactor'
deliverable: 'pr'
created: '2026-10-20'
updated: '2026-10-22'
depends_on: [360]
stacked_on:
related: [89, 268, 316, 368, 568]
discovered_from: [360]
adrs: [59, 84, 101, 111, 159, 160]
spec: 'docs/superpowers/specs/2026-10-22-retire-claim-reclaim-and-its-lease-design.md'
plan: 'docs/superpowers/plans/2026-10-22-retire-claim-reclaim-and-its-lease.md'
results:
auto_groomable:
hold:
branch_prefix:
branch: 'refactor/retire-claim-reclaim-and-its-lease'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-22T10:29:53Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-22-retire-claim-reclaim-and-its-lease-design.md](../../superpowers/specs/2026-10-22-retire-claim-reclaim-and-its-lease-design.md) |
| Plan | [2026-10-22-retire-claim-reclaim-and-its-lease.md](../../superpowers/plans/2026-10-22-retire-claim-reclaim-and-its-lease.md) |
| ADRs | [ADR-0059](../../adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0084](../../adrs/0084-re-dispatch-permission-gated-on-attribution-capability-not-launch-shape.md), [ADR-0101](../../adrs/0101-maintenance-sweep-scope-defer-historical-cleanup-out-of-impl.md), [ADR-0111](../../adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md), [ADR-0159](../../adrs/0159-claim-lease-stamped-only-at-claim-reconcile-and-resume-halte.md), [ADR-0160](../../adrs/0160-no-claim-lease-and-no-reclaim-stranded-runs-recover-only-by.md) |
<!-- docket:artifacts:end -->

## Why

Claim reclaim returns an abandoned `in-progress` change to `proposed` when three things hold: its `claimed_at` is older than `reclaim.lease_ttl` (72h by default), no feature branch exists locally or remotely, and no live workspace exists. It runs when typed (`docket change reclaim`), or from the maintenance sweep when `reclaim.auto: true` (off by default). It arrived with #89 (2026-07-17), borrowed from a competitor's lease-and-reclaim feature to self-heal crashed implement-next runs, and was ported to Go in #316.

It has never fired. Traced while grooming #360 and #575 (2026-10-20/21):

- No change record has ever carried a `## Reclaim log`: not in docket since #89, and not in the other local repositories using docket.
- No saved session ever ran a real `docket change reclaim`, and `reclaim.auto` is set in no config layer.
- The sweep has reported a stale claim for exactly one change (#422). It had halted with a feature branch, so reclaim would have refused it anyway, and the board already showed it as halted.

Its one window is tiny and already covered. On the building clone, the local branch blocks reclaim from `workspace.prepare` onward, minutes after the claim. A run that dies in those minutes is recovered by resuming the change by id (which works without a workspace since #368), with the run tracker's verdict driving any retry.

From another clone it is a risk rather than a help. A live run's branch is invisible there until publish, so a reclaim after the TTL could hand back a change that is still being built (ADR-0159 records this as an accepted risk).

The lease it reads has no other job. Nothing but reclaim reads `claimed_at`, yet claim, reconcile and resume-halted stamp it, and validation insists on it. The docs also describe reclaim wrongly: "crashed before pushing a branch" (a local branch or workspace blocks it too), and "the reclaim lease self-heals an abandoned claim" for halts (every such halt has a branch).

Removing both simplifies docket. One lifecycle edge, one operation, a sweep pass, a frontmatter field, two config keys and a body section go, along with their docs and tests.

## What changes

Retire claim reclaim and the claim lease end to end (settled at grooming; the spec has the full map):

- Remove `change.reclaim` (CLI command, capability-catalog entry, schema, dispositions vocabulary), the maintenance sweep's reclaim pass with its reasons and findings, and the domain lease and reclaim code.
- Stop writing `claimed_at`. Claim, reconcile and resume-halted no longer stamp it. Kill, done and complete no longer clear it. Validation no longer requires it. The archived-stamp check and its `drop-final-claimed-at` repair go. The claim result and receipt drop `claimed_at` and `lease`. Old records keep their line, and the decoder already ignores it.
- Keep `reclaim.lease_ttl` and `reclaim.auto` as obsolete tombstones, the same path as `finalize.skip_results_only_delta`. An old config loads in every layer with one warning per key ("… it is ignored — remove reclaim.<key> from <file>") and never blocks.
- Drop the dead `reclaim_auto` and `reclaim_lease_ttl_hours` fields from `context.finalize`, and the unused domain TTL field.
- Remove the `in-progress → proposed` edge, the *Reclaim edge* rule, the `## Reclaim log` section and the `claimed_at` manifest line from the convention. Fix the `halt` fallback's "self-heals" claim. Update implement-next, docket-status, the guides, the concepts and reference docs, and the comparison playbook, so a stranded run points only at resume-by-id and the run tracker.
- Reword `docket maintenance`'s help text, which uses "reclaim" in another sense.
- Record an ADR that supersedes ADR-0159.

## Out of scope

- Resume, `change.resume-halted`, the run tracker, and `## Run halted`: these are the recovery paths that stay. Claim's removal of a leftover `## Run halted` and the run tracker's `claim-replaced` verdict stay as defensive checks for older records.
- A replacement stale-claim signal: the one stale claim the sweep ever reported was already flagged as halted.
- A new way back from `in-progress` to `proposed` (#568 already refuses deferring an in-progress change): a stranded change is resumed, blocked, or killed.
- A `schema_version` bump.
- Other uses of the word "reclaim": `docket install collect`'s version trees, the transaction cleanup's abandoned worktrees, and gate-drive claims.
- Rewriting archived records, specs, plans, results, release notes, Accepted ADR bodies, learnings war stories, or frozen fixtures that mention reclaim or `claimed_at`.

## Reconcile log

### 2026-10-22

Unchanged against main 657c48a7e: #360 (dependency) landed in 42ddcb472 as the spec assumes, and the reclaim command, lease_ttl/reclaim.auto keys, and sweep reclaim leg are all still present on main.
