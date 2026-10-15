---
id: 548
slug: 'rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se'
title: 'Retire the fake-version test fixture folders'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-10-11'
updated: '2026-10-15'
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

Tests run against sample files under `testdata/repositories/`, kept in folders named like releases: `v0.9.2` through `v0.9.13`. Only `v0.9.2` and `v0.9.3` are real releases. The other ten were "the next number", cut each time a shipped file (the agent-defaults file, docket's own `.docket.yml`) changed, because tests kept a byte copy of those files and the "never edit a fixture, make a new folder" rule applied to the copies too. The names mislead (`v0.9.13` looks like a stale release in a 1.0 repository), eight of the folders are read by nothing, and every edit to a shipped file costs a new folder even when it changes nothing a test checks.

Renaming `v0.9.13` to a 1.0 name would repeat the mistake: it was cut after the `v1.0.0-rc.1` tag, and a folder is cut before anyone knows which release ships it.

## What changes

Keep the two real release samples (`v0.9.2`, `v0.9.3`) under their names, never edited; a release-named folder is created only when a test needs a sample from that release, never just because a release was tagged. Make tests read shipped files directly: the agent-defaults parity test reads `agents/harness-defaults.yml`, and the docket-self test copies `.docket.yml` into a temp directory at test time. Move the one hand-written fixture that must track docket as it is now (`deferred-active`) into `testdata/repositories/current/`, edited in place when it falls behind. Delete `v0.9.4`–`v0.9.13` and the stale first copies inside `v0.9.2`/`v0.9.3`, rewrite `testdata/README.md` to state the rules, add a folder-name guard test, and update the learnings finding that teaches the old cut-a-new-folder protocol.

## Out of scope

Editing or renaming the remaining release samples in `v0.9.2`/`v0.9.3`. The legacy installer's embedded v0.9.2 floor (`internal/install/legacydata/`), `testdata/bash-upgrade/`, and package-local `testdata/` directories. Frozen build records (old plans, specs, results, archived change records) that name the old folders.
