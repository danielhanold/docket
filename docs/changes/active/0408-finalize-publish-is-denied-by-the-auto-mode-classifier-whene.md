---
id: 408
slug: 'finalize-publish-is-denied-by-the-auto-mode-classifier-whene'
title: 'Finalize publish is denied by the auto-mode classifier whenever the gate rebases'
status: 'proposed'
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
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-finalize-publish-is-denied-by-the-auto-mode-classifier-whene-design.md) |
| ADRs | [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md), [ADR-0105](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0105-finalize-s-local-gate-continuation-is-persisted-in-the-owned.md) |
<!-- docket:artifacts:end -->

## Why

Change 0404's finalize on 2026-09-06 passed its rebase/test gate, then Claude Code 2.1.260 denied publication through the Go binary in the finalize child, its prescribed retry, and the parent auto-mode session. The run needed a human command to proceed. This interrupts unattended close-out and can waste a completed green gate.

The original report overstates the cause and frequency. Archived change 0100 already recorded a plain Git force-with-lease denial on 2026-07-19. Conversely, change 0403 successfully rebased and published through the Go binary on 2026-09-04 under Claude Code 2.1.259; its old head is not an ancestor of the published head, so it required a real rewrite. Go opacity, a Claude version change, effective policy, and session context remain competing explanations.

The human specifically identified the 2.1.260 changelog as a lead. The linked spec records its permission-related changes and their limits. The original title is the initial failure hypothesis, not an established universal behavior. A controlled comparison must precede selection of a permission rule or runtime refactor.

## What changes

Make the first gate a bounded controlled comparison of equivalent direct-Git and Go publication across Claude Code 2.1.259, 2.1.260, and the installed current version. Prove each trial requires an actual history rewrite, preserve the exact old-value lease, and record effective policy, model, session context, and observed external effects.

Deliver a reproducible evidence report and a concrete remedy recommendation, then return to the human before changing permissions or redesigning the publisher. No autoMode.allow prerequisite or split publisher is preselected. An inconclusive or unavailable trial is reported honestly and does not certify a fix.

The selected repair must preserve still-valid green gate evidence across a denied publish, provide a concrete durable resume path, and retain remote lease checks, current identity/base checks, PR evidence convergence, and repair sign-off. Detailed experimental controls, interpretation rules, and the explicit post-investigation decision gate live in the spec.

## Out of scope

Changing Claude Code, branch protection, merge methods, bot approvals, or unrelated gate scheduling. Adding broad permissions or changing user settings as part of the baseline. Treating a renamed command, alternate tool after denial, no-op push, or human shell execution as proof that auto-mode publication works. Building a new publisher or recovery subsystem before the controlled comparison supports and the human selects that design.
