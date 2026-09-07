---
id: 407
slug: 'keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent'
title: 'Keyed gate-verdict misattributes its verdict to a concurrent loop''s change id under parallel implement-next runs'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-09-04'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [345, 405]
discovered_from: [403]
adrs: [75]
spec: 'docs/superpowers/specs/2026-09-07-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-07T02:03:16Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-keyed-gate-verdict-misattributes-its-verdict-to-a-concurrent-design.md) |
| ADRs | [ADR-0075](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0075-run-gate-attributes-a-claim-conservatively-and-reports-a-halt-with-its-own-exit-code.md) |
<!-- docket:artifacts:end -->

## Why

A keyed implement-next run can report a concurrent run's change as incomplete after its own change has reached an open PR. Observed on 2026-09-04 during changes 0403 and 0402: keyed verdicts named sibling changes 0261 and 0402 instead of their own completed work.

Source inspection during grooming identified the failure class: fresh gate records have no explicit association with the successful claim. Initial attribution filters the current in-progress set using a before-set and dispatch timestamp. Once the intended change becomes implemented it disappears from consideration, leaving concurrent claims as candidates. The code does not select the newest or highest-priority claim; it attributes exactly one surviving candidate and refuses multiple candidates. ADR-0075 explicitly accepts a concurrency residual in this inference.

The existing dispatch-context token links test drives to recovery scopes, but the claim operation does not record it. A durable association with the successful claim is needed so verdicts and recovery stay scoped to the dispatch's actual work.

## What changes

Carry the existing dispatch context into change.claim and record durable proof linking that dispatch to its successful claim transaction. Make keyed verdicts use this ownership proof across completion, retries, and continuations, with bind-once behavior and safe handling of interrupted or replayed claims. Missing or conflicting proof must stop without guessing or authorizing work on a sibling change. Preserve verified resume and existing retry and continuation safety gates.

Cover overlapping runs, completion before the first verdict, failed and interrupted claims, conflicting bindings, replacement claims, and context propagation. Update the maintained interfaces and instructions, and record a successor ADR for the stronger attribution mechanism. Detailed design and acceptance criteria are in the linked spec.

## Out of scope

Slash-command interception and creation of attribution context for change 0345; the test-drive prepare-scope/start handshake seam in change 0405; scheduling or serializing concurrent implement-next loops; the release-determinism failure in change 0406; and unrelated lifecycle or disposition redesign.

## Reconcile log

### 2026-09-07

2026-09-06 — Reconciled against current source. Diagnosis confirmed in HEAD: internal/app/rungate_before.go leaves fresh dispatches' AttributedID unset (attribution deferred to verdict time), and internal/app/rungate_verdict.go attributeGateClaim still infers ownership from the pre-hand-off in-progress set minus a before-set with a claimed_at window, so a completed intended change drops out and a lone surviving concurrent claim is misattributed — exactly the failure class Why/spec describe. RunGateBefore already mints a dispatch context (DispatchContext / ChildContextHash) but change.claim (internal/app/change_claim.go ChangeClaimRequest/Result) records no dispatch identity, so the bind-at-claim seam the spec calls for is still absent. Related siblings unchanged and out of scope: 345 (slash-command attribution, proposed), 405 (test-drive prepare-scope/start handshake, proposed), 406 (release-determinism flake, in-progress); discovery provenance 403 and 402 are archived done. ADR-0075 remains Accepted; this change will record a successor ADR superseding its snapshot/cardinality attribution mechanism while preserving its conservative safety principle. Scope, relations, and acceptance criteria stand as written; no adjustment required.
