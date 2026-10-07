---
id: 482
slug: 'finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r'
title: 'Finish 0469''s leftover "repair" (relink) and "Step 0" (startup check) wording'
status: 'done'
priority: 'low'
type: 'refactor'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: [469]
stacked_on:
related: [481]
discovered_from: [469]
adrs: [129]
spec: 'docs/superpowers/specs/2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r-design.md'
plan: 'docs/superpowers/plans/2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r.md'
results: 'docs/results/2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r'
pr: 'https://github.com/danielhanold/docket/pull/361'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r-design.md](../../superpowers/specs/2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r-design.md) |
| Plan | [2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r.md) |
| Results | [2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-10-01-finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r-results.md) |
| ADRs | [ADR-0129](../../adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0469 delivered ADR-0129 family (e), but its build left two old spellings of row 80 and row 84 concepts in place. "repair" still names the `change relink` operation in three relink messages, the relink `--id` flag help ("whose recorded identity to repair"), Go comments and relink tests. "Step-0" / "Step 0" still names the shared startup check (`repository.prepare`) in Go comments, tests, one shell test header and the `repository prepare` help text. 0469's results reported these as covered by no rename row. The code trace at grooming (2026-10-01) showed they are rows 80 and 84's own concepts, left half done. They keep the retired vocabulary visible next to the new names, including in two user-visible help strings.

## What changes

Finish ADR-0129 rows 80 and 84 with a full sweep. Change "repair" to "relink" wherever it names the relink operation, and "Step-0" / "Step 0" to "startup check" wherever it names the shared startup check. That covers user-visible messages and help text, Go comments, and test wording, fixtures and one closure name: about 60 sites, enumerated in the linked spec. Every other meaning stays: finalize's and build's repair, the board's repair notice, implement-next's own "Step 0" step label, and the retirement guards that must spell the old token. No ADR edit, no new guard, no wire or behavior change.

## Out of scope

- Splitting the overloaded gate-drive halt tokens (change 0481).
- Any wire token, flag, schema key, operation id or behavior.
- Skill and agent-wrapper text (neither leftover sense appears there), so no `docket install` re-run.
- Point-in-time records (results files, archived changes, specs, Accepted ADR bodies), which keep their historical wording.

## Reconcile log

### 2026-10-01

2026-10-01 (implement-next): re-traced against origin/main 85bace7de (0469 merged). Every inventoried "repair" (relink) and "Step-0"/"Step 0" (startup check) site in the spec is still present unchanged; no intervening change touched them. 0481 (related) remains proposed and out of scope. Scope unchanged; build from the spec inventory, re-derived by whole-repo grep at build time.
