---
id: 349
slug: configurable-finalize-resolver-dispatch-cap
title: Make the finalize rebase-resolver dispatch cap configurable
status: 'in-progress'
priority: medium
type: feat
created: 2026-08-26
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [291, 334, 392, 396, 399, 403]
discovered_from: [334]
adrs: [10, 19, 105, 109]
spec: 'docs/superpowers/specs/2026-09-07-configurable-finalize-resolver-dispatch-cap-design.md'
plan:
results:
trivial: false
auto_groomable:
branch: 'feat/configurable-finalize-resolver-dispatch-cap'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-07T14:52:45Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-configurable-finalize-resolver-dispatch-cap-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-configurable-finalize-resolver-dispatch-cap-design.md) |
| ADRs | [ADR-0010](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0010-finalize-merge-gate-split-agents.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md), [ADR-0109](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0109-docket-schema-is-a-separate-reflected-payload-schema-surface.md) |
<!-- docket:artifacts:end -->

## Why

Finalize currently allows only two conflict-resolver dispatches during one rebase. A branch with three successive conflicting commits therefore stops for a human even when every conflict is mechanically resolvable, as observed while finalizing change 334. The cap also lives in skill memory, so a resumed session has no durable record of attempts already spent.

Make the ceiling configurable and enforce admission in Go before each resolver dispatch. The default becomes three attempts, as selected during grooming on 2026-09-07. An owned rebase keeps its snapshotted limit and consumed attempts across interruptions.

## What changes

- Add `finalize.resolver_max_attempts` as a positive integer with default 3, resolved through normal repository, global, and machine-local configuration layers and exposed through typed Go context.
- Persist the per-rebase limit, used count, and outstanding resolver reservation in the owned receipt. Require a durable Go reservation before native dispatch and validate that reservation before continuing the rebase.
- Preserve the budget across process restarts and repeated calls; define conservative recovery for lost responses and legacy receipts without inventing an unused budget.
- Keep the existing halt, verified abort, and Finalize blocked path on exhaustion or a stuck resolver. Keep the resolver's editing-only charter and integration-repair's separate two-attempt limit.
- Update configuration documentation, skill and agent contracts, schema/capability surfaces, and generated assets together. Verify successive conflicts, configured limits, interrupted/concurrent calls, and stale-report rejection.

## Out of scope

- Unlimited or until-stuck operation, and changing the integration-repair budget or repair sign-off.
- Moving Git staging, continue, abort, or testing into the resolver agent.
- General agent scheduling/liveness infrastructure, unrelated wrapper cleanup, or changes to merge policy.
- Implementing sibling changes or writing an implementation plan during grooming.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
