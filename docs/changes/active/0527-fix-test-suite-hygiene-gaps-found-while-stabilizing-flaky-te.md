---
id: 527
slug: 'fix-test-suite-hygiene-gaps-found-while-stabilizing-flaky-te'
title: 'Fix test-suite hygiene gaps found while stabilizing flaky tests'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [273]
discovered_from: [507]
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

Change 507 (flaky-test stabilization) surfaced several unrelated test-suite defects that no existing change covers. Each lets a test silently stop running, fail for environmental reasons, or fail falsely.

## What changes

- The e2e test-count check misses a test renamed to lower case: the test silently stops running and the file stays green. Make the check catch it.
- `TestNoGitGuardShadowsGitOnPath` fails when `TMPDIR` ends in `/` because it compares paths as strings. Compare cleaned paths.
- `TestAgentEnterCLIPreservesRequestAndReceipt` showed a one-off EOF in the same hand-run measurement; investigate whether it is flaky.
- `TestProcessExitSitesAreAllowlisted` fails falsely when run from the main checkout because it walks into `.worktrees/`. Skip that directory.

## Out of scope

The runner's solo re-check not detecting another worktree's concurrent suite (tracked separately). Budget-table re-seeding (#273).
