---
id: 516
slug: 'remove-stale-auto-groom-comments-and-fix-testskillhandoffsit'
title: 'Remove stale auto_groom comments and fix TestSkillHandoffSites'' ''cannot be invoked'' match'
status: 'done'
priority: 'low'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [502]
discovered_from: [502]
adrs: []
spec: 'docs/superpowers/specs/2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit-design.md'
plan: 'docs/superpowers/plans/2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit.md'
results: 'docs/results/2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/remove-stale-auto-groom-comments-and-fix-testskillhandoffsit'
pr: 'https://github.com/danielhanold/docket/pull/384'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit-design.md) |
| Plan | [2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit.md) |
| Results | [2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-10-04-remove-stale-auto-groom-comments-and-fix-testskillhandoffsit-results.md) |
<!-- docket:artifacts:end -->

## Why

Two small leftovers found while building change 0502. (1) Code comments still say an unset `auto_groomable` inherits a repository `auto_groom` setting, which no longer exists: `internal/domain/entities.go` (the `AutoGroomable` field comment and the comment above the optional-bool reader) and `internal/app/change_create.go` (the `AutoGroomable` request field comment). (2) `TestSkillHandoffSites` treats the phrase "cannot be invoked" as a skill invocation, so 0502 had to reword skill text to dodge the guard instead of the guard reading the sentence correctly.

## What changes

Rewrite the three stale comments (`internal/domain/entities.go` ×2, `internal/app/change_create.go`) to say what is true today: an unset or `false` `auto_groomable` means not auto-groomable, and only an explicit `true` opts a stub in.

Make `TestSkillHandoffSites` read a negation by its shape instead of requiring a bare `not`: a word ending in `not` (`not`, `cannot`), the `n't` contraction (`can't`, `won't`), or `never`. A line such as "when `docket-review` cannot be invoked" then counts as a mention, not an invocation. Extend the guard's self-check with these cases and mutation-test it both ways. Design detail is in the linked spec.

## Out of scope

Any behavior change to auto-groom selection. The `auto_groom` row in the config schema (a deferred setting) stays as it is. 0502's skill wording ("when the `docket-review` skill is missing") stays as written. The frozen fixture `internal/render/testdata/records/PROVENANCE.md`, which names deleted templates, stays as written.

## Reconcile log

### 2026-10-04

Reconciled against origin/main fc719ac6d. The three stale `auto_groom` comments are still present exactly as the spec names them (`internal/domain/entities.go` field comment and `AutoGroomable()` doc comment, `internal/app/change_create.go` request-field comment); a whole-repo grep found no other maintained site (remaining hits are frozen plans/results). `negatedInvokeRe` in `internal/repoguard/skill_handoff_sites_test.go` is unchanged from the spec's quoted pattern. Scope stands as written; no relation changes.
