---
id: 474
slug: 'rename-re-arm-to-re-enable-and-the-terminal-fence-lifecycle'
title: 'Rename re-arm to re-enable and the terminal/fence lifecycle codes'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-29'
updated: '2026-09-29'
depends_on: [468]
stacked_on:
related: [468]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0468 settled a collision-free docket vocabulary. "Re-arm" shares a verb with the unrelated run-tracker arming, "terminal" collides with the shell terminal for change-lifecycle end states, and "fence" names both the config guard and the run fence. This family applies 0468's rename table rows 53–59.

## What changes

Apply 0468 spec rows 53–59 as a hard cut (no aliases), following the spec's "Family changes" obligations:

- `change.groom` `outcome: rearm` → `re-enable` (the `groom_outcomes` vocabulary); `nothing-to-rearm` → `nothing-to-re-enable`; concept re-arm → re-enable.
- `fenced-setting-ignored` → `shared-setting-ignored`.
- Change-lifecycle `terminal` codes → `final`: `terminal-backlink-pending`, `terminal-notes-frozen`, `change-terminal-claim-stamp`, `drop-terminal-claimed-at`, `not-terminal`, `skipped-terminal`, `adr-update-after-terminal`.
- Go identifiers, tests, golden `schema` output, skills (docket-groom-next, docket-auto-groom, docket-convention), agents, embedded copies, docs, and the glossary entries for these rows.
- Add these rows' retired tokens to the retired-vocabulary table's absence seal (creating the table if this family lands first), mutation-tested.
- Process-level "terminal" tokens (`record-terminal`, `incumbent-nonterminal`, `terminal_receipt`, JSON `terminal`) stay unchanged (0468 Decision 6).

## Out of scope

- Any alias, deprecation-window, or dual-spelling support: old spellings are hard-cut (0468 Decision 2).
- Renaming persisted storage names, config keys, agent names, or frontmatter fields (0468 Decisions 3, 9).
- Editing point-in-time records (archived changes, results, specs, plans, Accepted ADRs).
- Rows owned by the other 0468 family changes or by change 0469.
