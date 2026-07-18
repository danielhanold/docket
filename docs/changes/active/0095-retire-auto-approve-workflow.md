---
id: 95
slug: retire-auto-approve-workflow
title: Retire the auto-approve workflow — document the classifier and the single-maintainer branch-protection solution
status: in-progress
priority: high
created: 2026-07-18
updated: 2026-07-18
depends_on: []
related: [15, 21, 62, 86]
adrs: [42]
spec: docs/superpowers/specs/2026-07-18-retire-auto-approve-workflow-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: feat/retire-auto-approve-workflow
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-18-retire-auto-approve-workflow-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-18-retire-auto-approve-workflow-design.md) |
| ADRs | [ADR-0042](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0042-auto-approve-consent-model.md) |
<!-- docket:artifacts:end -->

## Why

Changes 0015, 0021, and 0062 all aimed at one goal: run `docket-finalize-change` in one
swoop — gate, merge, close out — without human intervention or a blocking wall. Change
0062's mechanism for the merge-authorization half was a repo-installed GitHub Actions
workflow (`docket-approve.yml`) that bot-approves the PR so branch protection is satisfied
without `--admin` (ADR-0042).

**That workflow is a full failure.** Claude Code's auto-mode classifier soft-denies the
`gh workflow run docket-approve.yml` dispatch the finalize gate must issue — reproduced on
the 2026-07-18 finalize of change 0088, where the dispatch was denied outright. A bot
chain whose first step is blocked can never complete, so the capability ADR-0042 promised
does not exist on Claude Code as run. (ADR-0042 itself pinned that classifier behavior is
version/mode-scoped; this is that version-dependence landing on the failure side.)

**A far simpler solution was found empirically.** Setting this repo's branch protection to
require a pull request but require **zero** approvals lets a plain `gh pr merge --rebase`
satisfy protection with no `--admin`, no bot, and nothing for the classifier to deny —
which is exactly why the 0088 finalize was the first end-to-end run-through that worked.
For a single-maintainer repo (docket's primary use case, where you cannot approve your own
PR) this is strictly better than the bot workflow. The dead machinery should go, the
reversal should be recorded, and the classifier behavior + the working recipe should be
documented so the next maintainer does not re-derive this the hard way.

## What changes

- **Decommission change 0062's `auto_approve` subsystem** in full: the installed workflow
  and its template, `setup-auto-approve.sh`/`.md`, `docs/auto-approve-setup.md`, the
  `finalize.auto_approve` config knob and its `FINALIZE_AUTO_APPROVE` resolver, the
  `setup-auto-approve` facade op, the finalize gate's step-6 (Approve) prose, and the
  associated tests (delete three, prune three).
- **Author a new Accepted ADR that reverses ADR-0042**, recording the branch-protection
  configuration (require PR, 0 required approvals) as the supported single-maintainer
  hands-off merge path; flip ADR-0042 to `Reversed by ADR-NN`.
- **Document in `README.md`:** the Claude Code auto-mode classifier behavior (as *why the
  bot approach failed*), and the single-maintainer branch-protection recipe.
- **Preserve the human-approval path** for approval-required repos: a real human/co-maintainer
  approval on GitHub satisfies both branch protection and `require_pr_approval: true`, and
  finalize merges without `--admin`. Removing `auto_approve` must not break this.

Full design, the removal work-list, the new ADR shape, and the README structure are in the
linked spec.

## Out of scope

- **`finalize.gate`** (the rebase-retest correctness gate, 0015) — kept intact; unrelated
  to approvals.
- **`require_pr_approval`** (the human-authorization policy gate, 0021 / ADR-0011) — kept
  intact; removing `auto_approve` restores its clean "a human authorized the merge" meaning.
- Reversing or editing ADR-0011 (it stands).
- The finalize driver/loop (#0087) and the attended finalize-merge-path work (#0086).
- The `--admin` attended/explicit-id escape hatch — remains available.

## Open questions

_None — resolved during the 2026-07-18 brainstorm. The new ADR's id is minted at build
time; the `auto_approve` removal must preserve the Selection matrix's "approved ⇒ eligible"
behavior (spec §6.3)._

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
