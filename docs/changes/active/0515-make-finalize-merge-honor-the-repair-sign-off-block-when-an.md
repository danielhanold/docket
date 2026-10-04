---
id: 515
slug: 'make-finalize-merge-honor-the-repair-sign-off-block-when-an'
title: 'Retire the finalize repair sign-off so a green repair merges'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [502, 517, 520]
discovered_from: [502]
adrs: [8, 10, 11, 43, 139]
spec: 'docs/superpowers/specs/2026-10-04-make-finalize-merge-honor-the-repair-sign-off-block-when-an-design.md'
plan: 'docs/superpowers/plans/2026-10-04-make-finalize-merge-honor-the-repair-sign-off-block-when-an.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/make-finalize-merge-honor-the-repair-sign-off-block-when-an'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T17:00:04Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-make-finalize-merge-honor-the-repair-sign-off-block-when-an-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-make-finalize-merge-honor-the-repair-sign-off-block-when-an-design.md) |
| Plan | [2026-10-04-make-finalize-merge-honor-the-repair-sign-off-block-when-an.md](https://github.com/danielhanold/docket/blob/fix/make-finalize-merge-honor-the-repair-sign-off-block-when-an/docs/superpowers/plans/2026-10-04-make-finalize-merge-honor-the-repair-sign-off-block-when-an.md) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0010](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0010-finalize-merge-gate-split-agents.md), [ADR-0011](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0011-finalize-consent-model.md), [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md), [ADR-0139](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0139-finalize-adds-no-human-gate-of-its-own.md) |
<!-- docket:artifacts:end -->

## Why

When finalize's integration repair turns a red rebased suite green, the finalize skill still refuses to merge. An autonomous run records a `repair-needs-signoff` block and halts until a human runs `finalize clear-block` (0444 hit this). An attended run prompts for a go-ahead instead. The rule comes from ADR-0010, which assumed the human had approved the PR before the repair. Docket's recommended setup requires no PR approval at all, so the stop makes a human wait on a small fix that keeps every test, when the larger change it repairs needed nobody.

The rule also lives only in the skill text. The binary never refuses: the CLI always sends `ExplicitID: true`, and the selection skip is a placeholder that was never connected. Meanwhile the docs promise both a sign-off and a skip. Found while building change 0502.

## What changes

- A repair that turns the rebased suite green publishes and merges like any other green change, on autonomous and attended runs alike. There is no block, no prompt, and no `clear-block`.
- The repair stays visible. The run report and the archived record's closeout notes name what broke and the repair commits. A refused note never stops closeout.
- Approval stays the repository's policy. When branch protection requires approvals and dismisses stale approvals on new commits, the repair push removes the approval and the merge waits for a fresh one. That GitHub setting is off by default.
- Remove the binary's never-active blocked-note code (`finalizeBlockedMap`, the `finalize-blocked` skip, the merge's marker term). A `## Finalize blocked` note becomes visible only, and the next run retries the change.
- A new ADR records that finalize adds no human gate of its own. It reverses ADR-0010's sign-off rule; the rest of ADR-0010 stands.
- The finalize skill, the repair agent's instructions, and the docs describe this. Tests pin that a change with a note is still selected and merged, and that the end-to-end repair path merges without a block.

## Out of scope

- `finalize.block` / `finalize.clear-block` and the other abort points.
- The integration-repair ladder and its budget.
- Approval semantics (`require_pr_approval`, explicit id, `--admin`), and the separate gap where allowlist members get no override note.
- The board's "finalize blocked — needs you" wording.

## Reconcile log

### 2026-10-04

2026-10-04 — Re-checked against main at 84761645a. `finalizeBlockedMap`, `skipFinalizeBlocked`, the merge's `in.explicitID || !in.finalizeBlocked` term, and every `repair-needs-signoff` site in skills, agents, docs, goldens, the embedded tree, and tests are still present as the spec describes. 0517 and 0520 are now done, so the related-work note is updated. Merged plans under `docs/superpowers/plans/` that mention the token are frozen build records and stay untouched. Scope unchanged.
