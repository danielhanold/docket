---
id: 410
slug: 'require-durable-results-artifacts-with-human-testing-and-coo'
title: 'Require durable results artifacts with human testing and coordinator findings'
status: 'in-progress'
priority: 'medium'
type: 'feat'
created: '2026-09-07'
updated: '2026-09-08'
depends_on: []
stacked_on:
related: [1, 170, 190, 218, 330, 360, 374]
discovered_from: []
adrs: [102]
spec: 'docs/superpowers/specs/2026-09-07-require-durable-results-artifacts-with-human-testing-and-coo-design.md'
plan: 'docs/superpowers/plans/2026-09-08-require-durable-results-artifacts-with-human-testing-and-coo.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'feat/require-durable-results-artifacts-with-human-testing-and-coo'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-08T20:08:30Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-require-durable-results-artifacts-with-human-testing-and-coo-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-require-durable-results-artifacts-with-human-testing-and-coo-design.md) |
| Plan | [2026-09-08-require-durable-results-artifacts-with-human-testing-and-coo.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-08-require-durable-results-artifacts-with-human-testing-and-coo.md) |
| ADRs | [ADR-0102](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0102-build-and-finalize-own-independent-gate-and-test-command-con.md) |
<!-- docket:artifacts:end -->

## Why

Results artifacts remain optional in Docket Go, so human functional verification instructions, implementation findings, and out-of-scope follow-ups can remain only in a coordinator session and disappear when it ends. Restore a dependable, linked handoff for every implemented change, including trivial changes, and preserve meaningful discoveries at checkpoints during implementation.

## What changes

Make results a required completion artifact and adopt the human-approved template: Outcome, Human testing, Verification performed, Findings and limitations, and Follow-ups. Omit sections without substantive content. Human testing contains only functional scenarios requiring human verification that automated tests do not cover; it never repeats automated checks or tells the human to rerun test suites.

Capture coordinator-known findings after build returns, after review and fixes, before planned pauses or halts when safe, and during final consolidation. Preserve earlier content on resume and make checkpoint persistence verifiable. Follow-ups retain evidence, impact, scope boundaries, and suggested next actions for human triage; link existing changes without automatically creating new ones.

Enforce required results through the shared Go completion checks and shared workflow instructions across Claude, Codex, Cursor, and OpenCode. Preserve exact-head test evidence, branch ownership, generated links, and frozen merged results. The linked specification defines the artifact format, checkpoint lifecycle, evidence sequencing, failure handling, compatibility, and acceptance tests.

## Out of scope

Implementing the feature in this grooming session; automatic follow-up change creation or learnings promotion; retrospective rewriting or backfilling of historical results; post-merge results edits; new lifecycle states, background autosave services, per-observation capture, transcript archives, or additional agent/review rounds; new results-only evidence permits or changes to finalize gate policy.

## Reconcile log

### 2026-09-08

2026-09-08 — Reconciled against current main (9e82cc47). The spec inspected main at 0d1a7e1b; the 27 intervening commits are terminal-backlink retargets with no material bearing on this change. Confirmed all named surfaces still exist and match the spec's description: skills/docket-implement-next/SKILL.md (Step 6.5 optional results, Step 7 optional-results postcondition), skills/docket-implement-next/results-template.md (still the three-section Verify/Findings/Follow-ups template to be replaced), internal/app/change_attach.go (ChangeAttachResults), internal/app/change_implemented.go (ChangeMarkImplemented), internal/app/run_verify.go (RunVerify), ADR-0102, and the four-harness adapter registry under internal/harness (claude, codex, cursor, opencode). No design invalidation, no scope change, no relations change; adrs already cites ADR-0102. Proceeding to plan and build the change as specified.
