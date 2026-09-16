---
id: 415
slug: 'support-in-place-build-evidence-re-certification-for-an-impl'
title: 'Support in-place build-evidence re-certification for an implemented change'
status: 'implemented'
priority: 'medium'
type: 'feat'
created: '2026-09-09'
updated: '2026-09-16'
depends_on: []
stacked_on:
related: [374, 408]
discovered_from: [154]
adrs: [102]
spec: 'docs/superpowers/specs/2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl-design.md'
plan: 'docs/superpowers/plans/2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl.md'
results: 'docs/results/2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/support-in-place-build-evidence-re-certification-for-an-impl'
pr: 'https://github.com/danielhanold/docket/pull/307'
blocked_by:
reconciled: true
claimed_at: '2026-09-16T15:47:57Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl-design.md) |
| Plan | [2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl.md](https://github.com/danielhanold/docket/blob/feat/support-in-place-build-evidence-re-certification-for-an-impl/docs/superpowers/plans/2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl.md) |
| Results | [2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl-results.md](https://github.com/danielhanold/docket/blob/feat/support-in-place-build-evidence-re-certification-for-an-impl/docs/results/2026-09-16-support-in-place-build-evidence-re-certification-for-an-impl-results.md) |
| ADRs | [ADR-0102](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0102-build-and-finalize-own-independent-gate-and-test-command-con.md) |
<!-- docket:artifacts:end -->

## Why

When a follow-up commit is added to an already-implemented change's feature branch (e.g. addressing review feedback on an open PR before merge), the recorded build-evidence goes stale and `docket run verify` drops to `evidence-unverified`. There is no supported in-place way to re-certify. `docket-implement-next` refuses an implemented change (`context.implementation` returns `invalid-state: not-ready-not-proposed`), so `--resume` cannot re-drive its evidence step, and `evidence.record` refuses without a gate `--run` dir (`invalid-input: missing-run-dir`). The only recovery today is `docket-finalize-change`'s re-gate at merge (which also refreshes the PR-body evidence block) or the merge sweep. The remaining alternative is to hand-drive `docket gate drive` plus `evidence.record --run <dir>` directly, which is lower-level, does not refresh the PR-body evidence block, and is exactly the gate hand-driving the convention warns against.

## What changes

Add a supported evidence recertify command for an implemented change with an open PR. It reruns the configured build gate at the current published feature head, records and verifies canonical build evidence, and refreshes only the existing PR's evidence block. Reuse the existing gate and publication services; leave the change implemented and stop on failure without automatic repairs.

## Out of scope

The finalize re-gate path itself (already works and re-establishes evidence at merge). Changing the rule for when evidence is considered stale. The deferred results-only-delta skip optimization. Any change to how the PR-body evidence block is rendered.

## Reconcile log

### 2026-09-16

2026-09-16: Reconciled against current origin/docket. Design remains valid. Confirmed: related changes 374 and 408 are archived (terminal); no `evidence.recertify` command exists yet; the composed services the spec depends on are present in the codebase (internal/app/evidence_ops.go with EvidenceRecord/EvidenceVerify, internal/app/finalize_publish.go with FinalizePublish's PR evidence-block edit via internal/evidence Upsert, the build-owned gate driver, and workspace inspection). ADR-0102 (build and finalize own independent gate/test command) still governs — the command runs build.test_command only, with no finalize-command fallback. Scope unchanged: add evidence.recertify as an app-layer composition, reusing gate/evidence/GitHub services, leaving the change implemented and stopping on failure without automatic repairs.
