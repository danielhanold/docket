---
id: 496
slug: 'drop-final-claimed-at-is-reported-repairable-but-nothing-can'
title: 'drop-final-claimed-at is reported repairable but nothing can repair it'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-03'
depends_on: []
stacked_on:
related: []
discovered_from: [491]
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

`docket repository check` reports 268 `drop-final-claimed-at` warnings, each marked `repairable: true` with the remedy "Apply the previewed mechanical repair". On an already-migrated repository no command applies that repair. `migrate --repair-frontmatter --yes` goes to `migrateHealthyRepair`, which repairs only derived views (board, artifact-links blocks, ADR index). The frontmatter repair corpus (`gatherMigrationRepairs`) runs only on the legacy-migration path. Observed 2026-10-03: the run printed `repository already migrated` and changed nothing. The count also keeps growing. `planClaimedAt` (`internal/reposetup/repair.go`) flags any `claimed_at` key on a final archived record, even an empty one, and closeout leaves an empty `claimed_at:` key. 0491's closeout added the 268th finding this way. The warning is now permanent noise that hides real drift, and its remedy points at a repair that doesn't exist.

## What changes

Make the finding and its remedy agree, then clear the backlog. Grooming decides the shape: whether `planClaimedAt` should ignore an empty `claimed_at:`, whether closeout should drop the key instead of blanking it, and whether the healthy-repository `migrate --repair-frontmatter` path should also apply the frontmatter repairs. Whatever the choice, a repairable finding must name a command that repairs it, and the existing findings must be clearable.

## Out of scope

Other frontmatter repair codes, unless the same healthy-path gap blocks them. Any new blocking gate: the finding stays a visibility-only warning.
