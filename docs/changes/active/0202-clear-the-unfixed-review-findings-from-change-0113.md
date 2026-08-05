---
id: 202
slug: clear-the-unfixed-review-findings-from-change-0113
title: Clear the unfixed review findings from change 0113
status: proposed
priority: high
type: chore
created: 2026-08-03
updated: 2026-08-05
depends_on: []
related: [113, 211]
discovered_from: [113]
adrs: []
spec: docs/superpowers/specs/2026-08-05-clear-the-unfixed-review-findings-from-change-0113-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-05-clear-the-unfixed-review-findings-from-change-0113-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-05-clear-the-unfixed-review-findings-from-change-0113-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0113 (the `aborted-run` check) was reviewed at the deep rung and merged with five
non-blocking findings left unfixed by deliberate merge-time judgment. Each is small, real, and
independent of the change's own correctness, so they belong in their own pass rather than a hotfix.

Three of them are the change's own thesis turned back on itself — a guard that proves fewer of its
legs than it claims — which is exactly the class 0113 exists to close, so leaving them unrecorded
would be the worse outcome.

## What changes

- `tests/test_docket_status.sh` — the caller-side `--results-dir` wiring has no test, so deleting
  the argument from `scripts/docket-status.sh`'s `health_checks` reddens nothing. Every other
  argument is asserted against the mocked `board-checks.sh` (`--adrs-dir` was added with the
  explicit rationale "a gate that is only ever tested open is a gate nothing proves is closed").
  The `${RESULTS_DIR:-docs/results}` fallback makes the regression silent: on a repo configuring a
  non-default `results_dir`, leg A's results arm would scan a nonexistent directory forever with a
  green suite. Add the assert, ideally pinning the resolved value rather than the fallback.
- `tests/test_board_checks.sh` — only one of `aborted-run`'s four anchored reads is pinned.
  Mutation D mutates `fm_field "$f" plan` alone and fixtures 205/223 supply body prose for `plan:`
  alone, so swapping `fm_field "$f" results` to `field` yields the same silent false negative with
  a green suite. Add the mirror fixture (frontmatter omits `results:`, body opens a `results:`
  line, branch carries an unrecorded results file) and a second arm on mutation D. `branch` and
  `claimed_at` are lower-risk — both are written by the Step-2 claim — and can stay unpinned.
- `tests/test_board_checks.sh` — mutation A's comment claims "the healthy-field fixture 221
  (plan: SET) starts misfiring. Both directions." Fixture 221's branch carries no branch-only plan
  file, so the second conjunct fails and the misfire direction is unreachable with these fixtures.
  Mutation E's "stale-in-progress must stay unaffected" has the same shape — stated in prose,
  asserted nowhere. Either make each claim assertable or drop it.
- `scripts/board-checks.sh` — `branch_only_artifact` reads `ls-tree -r --name-only`, which
  C-quotes any path containing a quote, a backslash, a control character, or (under the default
  `core.quotePath=true`) a non-ASCII byte. `git_has` then fails on the literal quoted string and
  the function reports an inherited file as branch-only — a false positive in a check whose
  credibility is its whole value. Fix is `-z`, consumed with `while IFS= read -r -d ''`. **The
  capture-then-here-string shape cannot be kept** — command substitution strips NUL bytes — so the
  spec settles on a process-substituted redirect instead.
- `tests/test_skill_size_budgets.sh` — the 0113 budget-rationale comment was reported to omit the
  measured actual and margin. **Superseded by the spec:** a later merge (0201's slim, re-measured)
  already restored them — the comment now records `3728 words -> 3800 (72 words of margin)` and
  `139 actual, 145 budget`. The build verifies and makes no edit; the figures quoted in the original
  finding (`4013 -> 4050`, `147 for 143`) are pre-rebase and must not be restored.

Each fix's settled shape — the NUL-delimited `ls-tree` rewrite, the paired
sanity/inherited fixtures and the both-halves mutation, mutation D2 for the `results` read, the
non-default `--results-dir` pin, and ARM fixtures 224-226 — is in the spec.

## Out of scope

- The dangling `git-state postcondition` clause in `docket-implement-next` §5 and Step 4's missing
  rider — that is a normative design question, captured separately.
- Any change to `aborted-run`'s predicates or its 12h window.
