---
id: 15
slug: finalize-rebase-retest-gate
title: finalize — rebase onto base + re-run tests before merge
status: in-progress
priority: medium
created: 2026-06-15
updated: 2026-06-17
depends_on: []
related: [16, 17]
adrs: []
spec: docs/superpowers/specs/2026-06-17-finalize-rebase-retest-gate-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/finalize-rebase-retest-gate
pr:
blocked_by:
reconciled: false
---

## Why

`docket-finalize-change` merges an approved PR by trusting the PR's own CI —
green on the PR **head**. `gh pr merge --merge` blocks only *textual* conflicts.
So a PR that is **behind base** can pass its own CI yet produce a logically-broken
integration branch once merged — a **semantic** conflict git auto-merges cleanly
(e.g. base renamed a symbol the PR still calls). Nothing re-validates the *merged*
result before it lands. Finalize's only test step today is a parenthetical
*optional*, so the effective gate is "the PR head was green when a human approved
it."

## What changes

Add a **rebase-onto-base + re-run-tests gate** to finalize's merge step — the only
place docket itself performs a merge. Before the merge lands: rebase the feature
branch onto `origin/<integration_branch>`, validate the integrated result against
the repo's suite, and merge only if green.

- **Config** (`.docket.yml`): `finalize.gate` = `local` (default) · `ci` · `both` ·
  `off`. The test command is auto-detected (optional `test_command` override); `off`
  restores today's behavior.
- **Two pinned `opus/xhigh` subagents**, split at rebase-completion: a
  **`docket-rebase-resolver`** reconciles rebase conflicts *during* the rebase, and a
  **`docket-integration-repair`** owns red-test outcomes *after* it (root-cause +
  minimal fix, ≤2 attempts). Both abort-and-report when ambiguous or stuck.
- **Auto-authored repairs never merge unseen**: interactive finalize prompts with
  the diff before merging; autonomous finalize pushes the repair and aborts-and-reports.

Full design — the gate flow, the two-agent boundary, the sign-off rule, and the
abort-and-report set — is in the linked spec.

## Out of scope

- The merge *mode* (merge / squash / rebase-merge) — stays the team's `gh` flag.
- The `docket-status` sweep — it only archives already-merged PRs, so a pre-merge
  gate has nothing to act on there; the gate is finalize-only.
- Asserting the two agents' resolution/repair *quality* — governed by their
  `opus/xhigh` tier, not the test suite.

## Reconcile log
