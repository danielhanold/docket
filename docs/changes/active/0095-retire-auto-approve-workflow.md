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
adrs: [42, 43]
spec: docs/superpowers/specs/2026-07-18-retire-auto-approve-workflow-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: feat/retire-auto-approve-workflow
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-18-retire-auto-approve-workflow-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-18-retire-auto-approve-workflow-design.md) |
| ADRs | [ADR-0042](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0042-auto-approve-consent-model.md), [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md) |
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

### 2026-07-18 — reconciled against `origin/main` @ 0bd8c2f

**Verdict: no scope change.** The spec (2026-07-18) was written hours before this pass and
survives verification intact. Nothing in the removal work-list has been done elsewhere.

Verified against current code:

- **The 0062 footprint is fully live.** `git grep` over `origin/main` confirms every item in
  spec §4 still present: `.github/workflows/docket-approve.yml` +
  `scripts/templates/docket-approve.yml`, `scripts/setup-auto-approve.sh`/`.md`,
  `docs/auto-approve-setup.md`, the `setup-auto-approve` facade op (dispatch comment +
  `WRAPPED_OPS` + `scripts/docket.md` contract row), `FINALIZE_AUTO_APPROVE` in
  `scripts/docket-config.sh`/`.md`, and the finalize skill's step 6.
- **This repo's own `.docket.yml` carries `auto_approve: true`** (line 25, with its comment
  block) — the removal must drop the key here too, not just from the documented example.
- **`README.md` already has an `auto_approve` section** at §"Headless / autonomous finalize
  merge auto-approve (opt-in)" (~line 580) pointing at `docs/auto-approve-setup.md`. That
  section is *replaced* by the spec §6 content, not merely appended to — the deleted
  setup doc must not be left as a dangling link.
- **ADR-0042 is still `Accepted`** (`relates_to: [11]`, `change: 62`); the index row in
  `docs/adrs/README.md` renders it Accepted. The highest ADR id on `metadata_branch` is
  0042, so the new reversing ADR mints as **ADR-0043**.
- **Test footprint matches** spec §4: delete `test_auto_approve_docs.sh`,
  `test_docket_approve_template.sh`, `test_setup_auto_approve.sh`; prune
  `test_docket_config.sh` (12 refs), `test_finalize_gate.sh` (8), `test_docket_facade.sh` (4).
- **Branch-protection premise confirmed.** main now requires a PR with **0** required
  approvals; the 2026-07-18 finalize of change 0088 merged PR #100 via plain
  `gh pr merge --rebase` with no `--admin` and no bot — the empirical basis for the reversal.
- **No collision with related work.** #0086 (attended finalize-merge-path) and #0087
  (headless finalize driver) are both still `proposed` needs-brainstorm stubs — nothing
  built against the surfaces this change edits.

Folded in (build-time, beyond the spec's explicit list): `docs/adrs/README.md` must be
re-rendered for the reversed status, and the `.docket.yml` key removal should be checked
against `tests/test_config_example.sh` (ADR-0039 config-example-mirrors-defaults guard),
which does not reference `auto_approve` today but does validate the example's shape.
