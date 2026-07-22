# Single-source the remaining duplicated board vocabularies — results

Change: #116 · Branch: feat/single-source-the-remaining-duplicated-board-vocabularies · PR: https://github.com/danielhanold/docket/pull/120 · Plan: docs/superpowers/plans/2026-07-22-single-source-board-vocabularies.md · ADRs: 55

## Findings

- ADR-0055 records the exhaustive-mapping guard: mappings over a named vocabulary array must pin both cardinality and set equality, while sparse/default mappings stay sparse.
- The helper tests explicitly assert that every function exists before exercising negative predicates. This prevents a missing command from making a negated assertion pass vacuously.
- Mutation checks removed and added vocabulary members and mapping arms in both directions. Each mutation reddened the intended guard before the implementation was restored.
- All change-116-focused suites pass. The full-suite gate also exposed two unchanged baseline defects, reproduced on the exact `origin/main` SHA with the feature worktree's macOS/BSD toolchain and captured as follow-ups below; `test_docket_status.sh` requires `GIT_EDITOR=true` in non-interactive runs because its rebase-conflict fixture otherwise launches an editor.

## Follow-ups

- #129 — Fix the pipefail-unsafe plain-format config assertion.
- #130 — Make the finalize marker-reachability guard portable to BSD grep.

## Plan deviations

- The configured build and review skills normally delegate to subagents. The user's explicit inline-only constraint took precedence, so implementation, TDD, mutation testing, and whole-branch review were performed inline in this session.
