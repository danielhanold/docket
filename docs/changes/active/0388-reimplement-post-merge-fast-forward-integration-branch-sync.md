---
id: 388
slug: 'reimplement-post-merge-fast-forward-integration-branch-sync'
title: 'Reimplement post-merge fast-forward integration-branch sync as a native Go verb'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-08-31'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [29, 41, 364, 370, 389]
discovered_from: [370]
adrs: [99, 101, 104, 109]
spec: 'docs/superpowers/specs/2026-09-07-reimplement-post-merge-fast-forward-integration-branch-sync-design.md'
plan: 'docs/superpowers/plans/2026-09-07-reimplement-post-merge-fast-forward-integration-branch-sync.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/reimplement-post-merge-fast-forward-integration-branch-sync'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-07T01:55:06Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-reimplement-post-merge-fast-forward-integration-branch-sync-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-reimplement-post-merge-fast-forward-integration-branch-sync-design.md) |
| Plan | [2026-09-07-reimplement-post-merge-fast-forward-integration-branch-sync.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-07-reimplement-post-merge-fast-forward-integration-branch-sync.md) |
| ADRs | [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md), [ADR-0101](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0101-maintenance-sweep-scope-defer-historical-cleanup-out-of-impl.md), [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md), [ADR-0109](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0109-docket-schema-is-a-separate-reflected-payload-schema-surface.md) |
<!-- docket:artifacts:end -->

## Why

Change 0370 removed the Bash post-merge integration sync without a native replacement. A repository's local integration checkout can therefore remain behind after finalize or maintenance has recovered a merged PR. This causes stale source and, when Docket is used on itself, stale skill files for later sessions.

## What changes

- Add the native repository.sync-integration operation, reusing the clean-fast-forward Git primitive delivered by change 0364.
- Resolve the consuming repository's primary checkout and advance only a clean tree already on its configured integration branch, using a freshly fetched, pinned target.
- Report explicit skips for unsafe checkout states and visible failures for unknown or failed Git observations.
- Run the shared operation once after finalize's closeout/cleanup suffix and once at the end of either maintenance scope, preserving completed lifecycle outcomes when sync cannot finish.
- Ship capability/schema coverage, regression tests, and consistent workflow documentation.

## Out of scope

Bash restoration; non-fast-forward updates, branch switching, resets, stashing, or user-edit recovery; background scheduling and retry queues; new lifecycle states; automatic binary reinstall or updating a separate Docket installation; terminal publication; live-session skill reload; and restoring main-mode compatibility. ADR-0099's existing legacy-repository refusal applies.

## Reconcile log

### 2026-09-07

2026-09-07: Reconciled against current main (effc9a6d). Design is current — spec was authored 2026-09-07 against this exact base. Confirmed the primitives the spec depends on are present: gitcli.Client.FastForwardWorktree (internal/gitcli/fastforward.go, from change 0364) and Repository.PrimaryWorktree. No native repository.sync-integration operation exists yet, so this is a clean addition, not a re-do. Related changes 0029/0041/0364/0370/0389 all done; no unmet dependency or stacking. Scope, ADR references (0099/0101/0104/0109), and out-of-scope boundaries hold as written. No section or relation changes required.
