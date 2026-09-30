---
id: 474
slug: 'rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle'
title: 'Rename re-arm to re-enable and retire the lifecycle ''terminal'' and non-run ''fence'' names'
status: 'implemented'
priority: 'medium'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-30'
depends_on: [468]
stacked_on:
related: [468, 471, 472, 473, 477]
discovered_from: []
adrs: [129]
spec: 'docs/superpowers/specs/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle-design.md'
plan: 'docs/superpowers/plans/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle.md'
results: 'docs/results/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle'
pr: 'https://github.com/danielhanold/docket/pull/356'
blocked_by:
reconciled: true
claimed_at: '2026-09-30T14:57:21Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle-design.md) |
| Plan | [2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle.md](https://github.com/danielhanold/docket/blob/refactor/rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle/docs/superpowers/plans/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle.md) |
| Results | [2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle-results.md](https://github.com/danielhanold/docket/blob/refactor/rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle/docs/results/2026-09-30-rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle-results.md) |
| ADRs | [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0468 (ADR-0129) settled a collision-free docket vocabulary. This is its family (d), the last family change: 0471, 0472, 0473 and 0477 have merged. Three words still collide. "Re-arm" (auto-groom) shares a verb with the old run-tracker arming. "Terminal" names change-lifecycle end states as well as a finished process. "Fence" names the config shared-setting guard and four other internal checks as well as the run fence.

Grooming (2026-09-30) traced the code. Rows 53–59 are complete as wire tokens, but the rules they apply are still broken around them. The main gaps: `Status.Terminal()`, CLI help "a change's terminal half", commit subjects "terminal backlinks retargeted to archive", the skill file `terminal-close-out.md`, the config guard's `applyFence`/`scopeRepoFenced`, and `fenceBoardSurface`. ADR-0129 was extended in place with rows 59a–59c to cover them.

## What changes

Hard-cut ADR-0129 rows 53–59 and 59a–59c in one PR, with no aliases:

- **Re-enable:** `change.groom` `outcome: rearm` → `re-enable`, `nothing-to-rearm` → `nothing-to-re-enable`, and the "re-arm" concept → "re-enable" in groom-next, the convention, auto-groom, CLI help, result text and the glossary.
- **Shared-setting guard:** `fenced-setting-ignored` → `shared-setting-ignored`, plus the config package's fence-named Go identifiers and comments.
- **Final:** seven lifecycle codes change "terminal" to "final" (`skipped-final` kept as written although it also covers stacked-merged children). Every other change/ADR-lifecycle "terminal" in maintained source becomes "final" too: Go identifiers (`Status.Final()`), messages, commit subjects, skill text and comments. "Terminal half" becomes "closing half", and `references/terminal-close-out.md` → `close-out.md`.
- **Other fences:** the board-surface, learnings, capability, owned-section and owned-ref checks drop "fence" (`resolveBoardSurface`, `requireLearningsEnabled`, `haltPreflight`, test renames), so "fence" means only the run fence.
- **Seal:** append the retired tokens and the old filename to the retired-vocabulary table (non-vacuity plus a hand mutation). Results records a whole-repo grep that classifies every remaining `terminal`/`fence`/`re-arm` hit as a documented keep or a point-in-time record.
- **Landing:** no drain (nothing persisted carries these tokens). The results human action tells consumer repos to re-run `docket install`.

The full inventory, the kept senses, the seal rows and the tests are in the spec.

## Out of scope

- Any alias, deprecation window or dual spelling: old spellings are hard-cut (ADR-0129 Decision 2).
- Renaming config keys (`terminal_publish`), agent names, frontmatter fields or persisted storage (Decisions 3, 9).
- The process-level "terminal", Markdown code fences and `---` frontmatter fences (kept, listed in ADR-0129).
- Editing point-in-time records: archived changes, results, specs, plans, Accepted ADRs and frozen testdata corpora.
- Behavior changes of any kind; rows owned by other families or by change 0469.

## Reconcile log

### 2026-09-30

Reconciled at claim against origin/main: 0468, 0471, 0472, 0473 and 0477 are merged; a grep of origin/main confirms every family-(d) site in the spec inventory (GroomRearm, fenced-setting-ignored, skipped-terminal, fenceBoardSurface, terminal-close-out.md) is still present. No scope change; the spec stands as written, with the inventory re-derived by whole-repo grep at build time.
