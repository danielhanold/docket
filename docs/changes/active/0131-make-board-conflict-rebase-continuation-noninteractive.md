---
id: 131
slug: make-board-conflict-rebase-continuation-noninteractive
title: Make board-conflict rebase continuation noninteractive
status: proposed
priority: medium
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: []
discovered_from: [128]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The full-suite board-conflict fixture reaches `git rebase --continue` after resolving a generated
`BOARD.md` conflict. In the current noninteractive agent environment, Git launches an editor and
the fixture blocks at `E303: Unable to open swap file for "[No Name]" ... Press ENTER` instead of
finishing. This prevents the whole shell-test corpus from reaching a result even when all earlier
assertions are green.

## What changes

Make the automated board-conflict rebase continuation reliably noninteractive without weakening
the fixture's assertions; add a regression witness that the conflict branch completes under the
same unattended environment.

## Out of scope

- Change 0128's fetch-diagnostic and harness-retry behavior.
- Broad editor configuration or user-level Git settings.
