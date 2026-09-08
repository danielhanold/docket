---
id: 411
slug: 'steer-post-completion-durable-write-failures-to-rebase-conti'
title: 'Steer post-completion durable-write failures to rebase-continue, not abort'
status: 'proposed'
priority: 'low'
type: 'docs'
created: '2026-09-08'
updated: '2026-09-08'
depends_on: []
stacked_on:
related: [349]
discovered_from: [349]
adrs: [113]
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0113](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0113-resolver-dispatches-are-admitted-by-durable-pre-dispatch-res.md) |
<!-- docket:artifacts:end -->

## Why

A deep-review finding on change 0349 (PR #288) noted that the finalize resolver-reserve path and the docket-finalize-change abort-flow guidance both steer a stuck resolver to the abort flow (finalize.rebase-abort) even in the narrow window where the owned rebase already completed successfully and only the durable receipt/reservation write failed afterward. Aborting in that window discards a completed rebase — the merge the human already intended — instead of re-attempting only the failed durable write. The safer recovery is re-running finalize.rebase-continue, which reconciles an already-started continuation.

## What changes

Update the reconcile/reserve write-failure message(s) (e.g. in internal/app/finalize_reserve.go and the reconcile-write-failure path) and the docket-finalize-change SKILL abort-flow note (skills/docket-finalize-change/references/gate-failure.md) to call out this narrow 'rebase already completed, only the durable write failed' window and point a human at re-running finalize.rebase-continue rather than the generic abort flow.

## Out of scope

No behavior change to the finalize.rebase-continue or finalize.rebase-abort operations themselves; no change to the reservation admission logic decided in ADR-0113. Purely guidance and message wording so the correct recovery is discoverable in the narrow post-completion durable-write-failure window.
