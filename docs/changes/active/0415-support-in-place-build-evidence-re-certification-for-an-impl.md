---
id: 415
slug: 'support-in-place-build-evidence-re-certification-for-an-impl'
title: 'Support in-place build-evidence re-certification for an implemented change'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-09-09'
updated: '2026-09-09'
depends_on: []
stacked_on:
related: []
discovered_from: [154]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

When a follow-up commit is added to an already-implemented change's feature branch (e.g. addressing review feedback on an open PR before merge), the recorded build-evidence goes stale and `docket run verify` drops to `evidence-unverified`. There is no supported in-place way to re-certify. `docket-implement-next` refuses an implemented change (`context.implementation` returns `invalid-state: not-ready-not-proposed`), so `--resume` cannot re-drive its evidence step, and `evidence.record` refuses without a gate `--run` dir (`invalid-input: missing-run-dir`). The only recovery today is `docket-finalize-change`'s re-gate at merge (which also refreshes the PR-body evidence block) or the merge sweep. The remaining alternative is to hand-drive `docket gate drive` plus `evidence.record --run <dir>` directly, which is lower-level, does not refresh the PR-body evidence block, and is exactly the gate hand-driving the convention warns against.

## What changes

Provide a supported path to re-run the build gate at the current head of an implemented change and re-record + verify build-evidence in place, refreshing the PR-body evidence block the way finalize does, without requiring a full finalize/merge and without hand-driving the gate. Design to settle at brainstorm: whether this is a dedicated re-certify operation, an `evidence`/`gate` verb that owns a gate run and the PR-body refresh, or a bounded `docket-implement-next` re-entry that accepts an implemented change purely to re-certify.

## Out of scope

The finalize re-gate path itself (already works and re-establishes evidence at merge). Changing the rule for when evidence is considered stale. The deferred results-only-delta skip optimization. Any change to how the PR-body evidence block is rendered.
