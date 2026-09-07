---
id: 410
slug: 'require-durable-results-artifacts-with-human-testing-and-coo'
title: 'Require durable results artifacts with human testing and coordinator findings'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-09-07'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [1, 170, 190, 218, 330, 360, 374]
discovered_from: []
adrs: [102]
spec: 'docs/superpowers/specs/2026-09-07-require-durable-results-artifacts-with-human-testing-and-coo-design.md'
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
| Spec | [2026-09-07-require-durable-results-artifacts-with-human-testing-and-coo-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-require-durable-results-artifacts-with-human-testing-and-coo-design.md) |
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
