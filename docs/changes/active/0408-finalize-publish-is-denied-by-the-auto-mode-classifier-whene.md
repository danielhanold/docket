---
id: 408
slug: 'finalize-publish-is-denied-by-the-auto-mode-classifier-whene'
title: 'Finalize publish is denied by the auto-mode classifier whenever the gate rebases'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-09-06'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [100, 260, 316, 360, 396, 403, 404]
discovered_from: [404]
adrs: [43, 105]
spec: 'docs/superpowers/specs/2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/finalize-publish-is-denied-by-the-auto-mode-classifier-whene'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-07T15:02:34Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md) |
| ADRs | [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md), [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md) |
<!-- docket:artifacts:end -->

## Why

A finalize that rebases runs the local gate on the rebased head, records green evidence, and then force-with-lease publishes that head. If the publish is denied — most sharply by a host permission classifier, so the Go binary never runs — the branch is rebased and the gate is green, but nothing publish-specific persisted. On resume, `recoverFromReceipt` sees the local head is no longer the receipt's `OrigHead`, treats the gate as non-noop, and re-runs the full suite. A passed green gate is discarded by exactly the interruption a durable gate should survive.

The original alarmist framing — that the classifier denies every rebased publish — did not hold. The five most recent finalizes (0379, 0383, 0388, 0406, 0407) each really rebased and published on Claude Code 2.1.263 with no denial and no human command; recovered results show the same for 0403, 0384, and 0364. The one observed denial was 0404 on 2.1.260, two versions back. The defect worth fixing is not the version-specific, non-reproducing denial but the lost green gate whenever a publish is denied.

## What changes

Persist a completed-gate publish checkpoint in the owned rebase receipt when the local gate passes for the rebased head — tested head, effective base, resolved test command, gate policy, repo/change/PR identity, and green evidence. On a finalize resume with no rebase in progress and the head descending the base, reuse the checkpoint to skip the suite and go straight to publish when every recorded identity still matches; a moved head/base, a changed resolved command, or a changed policy invalidates it and the gate re-runs as today. Write-on-pass, clear-on-terminal-or-invalidation, mirroring ADR-0105's continuation discipline. Preserve `PublishRewrite`'s exact lease and response-loss behavior and `FinalizePublish`'s PR-body preservation. Record the checkpoint as a new ADR relating to 0105 and 0098.

## Out of scope

The retired historical-version comparison (the 2.1.259 / 2.1.260 / current matrix) and the live classifier acceptance activity. Any change to Claude Code, branch protection, merge method, or bot approvals. Any broad permission grant or user-settings change. A Go primitive distinguishing a host denial from a Go result — a host denial means the binary never ran, so that distinction lives in the finalize skill and harness. A split publisher or a general recovery subsystem beyond the receipt checkpoint.

## Reconcile log

### 2026-09-07

2026-09-07: Reconciled against current HEAD (main @ 0d1a7e1b). The rewritten spec ("Preserve a still-valid green gate across a denied finalize publish") matches the live code: internal/app/finalize_rebase.go still carries recoverFromReceipt (the noop := string(localHead) == rec.OrigHead derivation), composeLocalGate, and the pure gateDecision skip policy; internal/workspace/rebasereceipt.go's RebaseReceipt is the all-scalar ==-comparable effect record with the ADR-0105 GateDriveID/GateOwnerGeneration continuation pair the checkpoint must mirror; PublishRewrite (internal/workspace/rewrite.go) and FinalizePublish (internal/app/finalize_publish.go) hold the lease/response-loss and PR-body-preservation behavior the change must leave untouched. ADR-0105 and ADR-0098 both exist and are Accepted; the new completed-evidence checkpoint decision extends 0105 and refines 0098. No scope change; the investigation-first framing is already retired in the spec. Relations (related, adrs:[43,105], discovered_from:[404]) already reflect reality and are left unchanged. Building from scratch: no plan, no branch commits, no PR exist yet.
