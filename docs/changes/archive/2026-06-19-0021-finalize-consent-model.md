---
id: 21
slug: finalize-consent-model
title: Finalize consent model — ambiguity-only prompt + require_pr_approval policy gate
status: done
priority: medium
created: 2026-06-17
updated: 2026-06-19
depends_on: []
related: [15, 19]
adrs: [10, 11]
spec: docs/superpowers/specs/2026-06-17-finalize-consent-model-design.md
plan: docs/superpowers/plans/2026-06-17-finalize-consent-model.md
results: docs/results/2026-06-17-finalize-consent-model-results.md
trivial: false
auto_groomable:
branch: feat/finalize-consent-model
pr: https://github.com/danielhanold/docket/pull/34
blocked_by:
reconciled: true
---

## Why

`docket-finalize-change`'s no-arg (auto-detect) path prompts before merging a mergeable-but-unmerged
PR. The prompt guards a real risk — a no-arg run can match several `implemented` changes, and it
shouldn't blanket-merge everything it finds. But in the common case of **one** obvious target, the
prompt is pure friction: the human invoked finalize deliberately, so being asked to re-confirm
interrupts the flow.

The friction is sharpest on docket's **primary use case, a single human**, who pushes their own PRs
and so cannot approve them on GitHub at all — the PR is always unapproved, yet the human running
finalize *is* the authorization.

## What changes

Two changes to `docket-finalize-change` (markdown/prose behavior + one config knob):

- **Prompt only when ambiguous.** No-arg finalize runs the full flow (gate + merge + finalize) with
  **no prompt** when exactly one eligible candidate exists; it prompts **only when >1** would be
  merged (the blast-radius guard stays exactly there).
- **`finalize.require_pr_approval` (default `false`).** A repo-level policy gate nested beside
  `gate:` — `gate` validates correctness, this validates human sign-off. `false` (default,
  single-human friendly) ⇒ approval never blocks the auto-detect path. `true` ⇒ the auto-detect path
  refuses to merge an unapproved PR, surfacing it instead.
- **Explicit id is unchanged by default and always overrides the approval gate.** Passing an explicit
  id is itself the human authorization, so it proceeds even on an unapproved PR under
  `require_pr_approval: true`; the rebase-retest correctness gate still runs regardless.

Principle: `require_pr_approval` ensures a human authorized the merge — on the auto-detect path that
proof is a GitHub approval; an explicit id is that proof by another means.

Full matrix, config placement, and the test/ADR scope are in the linked spec.

## Out of scope

- Changing the rebase-retest `gate` behavior or its CI logic (the gate owns correctness + CI).
- CI-state selection logic — CI is the gate's concern, not selection's.
- A `--yes`/`all` bypass flag (ambiguity-only prompting removes the need; multi-target still confirms).
- The kill paths and terminal-publish.

## Open questions

None — consent model, knob name/placement/default, the readiness bar, and the explicit-id override
were all settled in the 2026-06-17 brainstorm (see spec §8).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- 2026-06-17 — Reconciled against `origin/main` (tip `c02c119`). Verified every spec premise still
  holds against current code: `skills/docket-finalize-change/SKILL.md` **Selection** section still
  carries the old unconditional "PROMPT before merging — merging is a deliberate act" language (the
  thing this change replaces with ambiguity-only prompting); the **rebase-retest merge gate** config
  block exposes exactly `gate:` + `test_command:` (the nesting spot for `require_pr_approval:`); and
  `.docket.yml`'s `finalize:` block has `gate: local` with no approval knob yet. Related changes
  unchanged since the brainstorm: 0015 (rebase-retest gate, ADR-0010) is `done` and is the gate this
  layers on; 0019 (finalize ci/both functional test) is still `proposed`/needs-brainstorm and
  non-overlapping. `tests/test_finalize_gate.sh` exists with an awk-based `gate_of()` parser to extend
  for the new knob. No scope dropped, no new constraints — spec and body carried into build as written.
