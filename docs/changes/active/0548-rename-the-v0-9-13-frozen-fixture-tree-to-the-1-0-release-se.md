---
id: 548
slug: 'rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se'
title: 'Name test fixture folders by what they hold'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-10-11'
updated: '2026-10-16'
depends_on: [546]
stacked_on:
related: [546, 554]
discovered_from: [546]
adrs: []
spec: 'docs/superpowers/specs/2026-10-15-rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se-design.md'
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
| Artifact | Link |
|---|---|
| Spec | [2026-10-15-rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se-design.md](../../superpowers/specs/2026-10-15-rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se-design.md) |
<!-- docket:artifacts:end -->

## Why

Tests run against sample files under `testdata/repositories/`, kept in folders named like releases: `v0.9.2` through `v0.9.13`. Only `v0.9.2` and `v0.9.3` are real releases. The other ten were "the next number", cut each time a shipped file changed, because tests kept byte copies of `agents/harness-defaults.yml` and `.docket.yml` and the "never edit a fixture, make a new folder" rule covered the copies too. Eight of those folders are read by nothing. Even the real names mislead: the `v0.9.2` configs are hand-written, and two of its five records came from later. A folder name tells a reader neither what it holds nor whether it may change, and "never edit" wrongly suggests a sample can never change.

## What changes

Name the folders for what they hold: `configs/`, `documents/`, `status-corpus/`. Each folder's `PROVENANCE.md` records where its data came from, and tagging a release creates nothing. These are samples of data docket must keep reading. A sample is never edited to make a failing test pass, and is removed only when docket deliberately drops support for it, with the reason recorded. Tests that need input that keeps up with docket build it themselves: they read `agents/harness-defaults.yml` and `.docket.yml` directly, and write the complete deferred-capability config inline. Delete `v0.9.4`–`v0.9.13` and every byte copy, rewrite `testdata/README.md` around the rule, add a guard against version- or date-named folders, and update the learnings finding that teaches the old cut-a-new-folder protocol.

## Out of scope

Changing any sample's contents or renaming individual fixtures. The legacy installer's embedded v0.9.2 floor (`internal/install/legacydata/`) and its goldens, `testdata/bash-upgrade/`, and package-local `testdata/` directories. Frozen build records and ADRs that name the old folders.
