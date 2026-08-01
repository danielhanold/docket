---
id: 170
slug: lean-whole-branch-review-skill
title: Lean Docket-owned whole-branch review skill
status: in-progress
priority: medium
type: feat
created: 2026-07-30
updated: 2026-08-01
depends_on: [167, 184]
related: [137]
discovered_from: [167]
adrs: []
spec: docs/superpowers/specs/2026-08-01-lean-whole-branch-review-skill-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/lean-whole-branch-review-skill
claimed_at: 2026-08-01T19:20:15Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-lean-whole-branch-review-skill-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-lean-whole-branch-review-skill-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0167 removes SDD's per-task and final reviews but deliberately preserves Docket Step 6 as
one independent whole-branch review. The current default still delegates that boundary to
Superpowers, leaving its model, effort, recursion, and output cost outside Docket's explicit
control — and its "All tests passing?" checklist re-runs the ~10-minute full suite on the exact
branch state the build gate just tested. With finalize's post-rebase run, one change pays for
three full-suite executions (~30 minutes); only the build gate's and finalize's runs have
distinct justifications, and finalize's only when the base actually moved.

## What changes

Ship `docket-review` — a Docket-owned `skills.review` replacement — plus the suite-once
evidence chain it consumes:

- One bounded, **read-only** whole-branch reviewer, dispatched foreground from
  `docket-implement-next` Step 6 via one of **three pinned rung wrappers**
  (lean / standard / deep; Claude: sonnet-5/high, opus-5/medium, opus-5/high), selected
  deterministically as "one above the build" — from the highest profile the build routed or
  escalated to, with an optional diff-size bump. It returns severity-tiered findings
  (blocker / important / minor) and never fixes, never dispatches subagents, and **never runs
  the test suite**.
- The build gate stays the suite's sole implementation-phase home and records a
  **build-evidence block** (command, result, HEAD SHA, timestamp). The reviewer verifies it
  instead of re-running; Step 7 writes it durably into the PR body.
- Controller triage: blockers → one synthetic fix task through the existing
  `docket-build-task` contract, then one suite re-run refreshing the evidence; important/minor
  → PR body for merge-time judgment; follow-ups → existing auto-capture. No re-review round.
- `docket-finalize-change`'s local gate **skips** its post-rebase suite run only when the
  rebase was a no-op and the PR's evidence block is green at the exact branch HEAD; any doubt
  runs the suite. Net: one run clean-path, two worst-case, never three.
- Shipped default binding unchanged (`superpowers:requesting-code-review`); this repo dogfoods
  `docket-review` via `.docket.yml`. README documents the suite-placement rationale.

Design detail, diagram, schema, and the full ripple list live in the linked spec.

## Out of scope

- Per-task review.
- Implementing findings inside the reviewer.
- Changing the build profiles, rubric, or TDD contract from changes 0167/0184.
- A second review round after blocker fixes.
