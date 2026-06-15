---
id: 15
slug: finalize-rebase-retest-gate
title: finalize — rebase onto base + re-run tests before merge
status: proposed
priority: medium
created: 2026-06-15
updated: 2026-06-15
depends_on: []
related: [16]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Why

`docket-finalize-change` trusts the PR's own CI and merges via a merge commit
(`gh pr merge --merge`). It never rebases the feature branch onto
`origin/<integration_branch>` first, and its only test step is a parenthetical
*optional* — "verify the merge landed (optionally: tests green on the merged
result)". So the effective gate today is "the PR head was green when a human
approved it."

That leaves a real gap: a PR can be **behind base** and still pass its own CI,
yet produce a logically-broken integration branch once merged — a semantic
conflict git auto-merges cleanly (e.g. base renamed a symbol the PR still
calls). Nothing re-validates the *merged* result before it lands on the
integration branch. The same gap exists in `docket-status`'s bulk merge-sweep,
which shares finalize's archive path.

## What changes

Add a **rebase-onto-base + re-run-tests gate** to the close-out path, before the
merge is allowed to land: update the feature branch onto
`origin/<integration_branch>`, resolve conflicts, run the project's test
command on the rebased result, and only merge if it is green. On a conflict that
can't be auto-resolved, or a red suite, **abort-and-report** (no merge) — matching
the subagent abort-and-report semantics from change 0016.

## Out of scope

- Changing the merge *mode* (merge vs squash vs rebase-merge) — that stays the
  team's `gh` flag choice.
- Re-running tests for changes whose PR is already merged (the merge already
  landed; this gate is pre-merge only).
- The model/effort plumbing itself — that is change 0016; this change consumes it.

## Open questions

- **Two-subagent fan-out.** Split finalize's close-out into two subagents: a
  **rebase/conflict-resolver** (Opus — conflict resolution is real judgment) and
  a **close-out executor** (Sonnet — the existing procedural archive/publish/board
  flow). This generalizes the same "split by reasoning-load within one skill"
  pattern as auto-groom's designer/critic. Decide during grooming whether the
  split is worth the extra subagent hop or whether one Sonnet subagent that
  escalates to Opus only on conflict is simpler.
- Where the project's test command comes from (a `.docket.yml` knob? infer from
  repo? reuse whatever `docket-implement-next`'s build step already runs?).
- Does the bulk `docket-status` sweep get the same gate, or only the deliberate
  single-change finalize? (The sweep is unattended — a red re-test there should
  skip-and-flag, not block the whole sweep.)
- Interaction with change 0016: finalize-as-subagent can't pause to ask, so the
  rebase resolver must abort-and-report on anything ambiguous.

## Reconcile log
