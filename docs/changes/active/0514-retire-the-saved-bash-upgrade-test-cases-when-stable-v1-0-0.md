---
id: 514
slug: 'retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0'
title: 'Retire the saved Bash upgrade test cases when stable v1.0.0 ships'
status: 'in-progress'
priority: 'low'
type: 'chore'
created: '2026-10-04'
updated: '2026-11-08'
depends_on: [511]
stacked_on:
related: [511, 366]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0-design.md'
plan: 'docs/superpowers/plans/2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0.md'
results: 'docs/results/2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0-results.md'
trivial: false
auto_groomable: false
branch_prefix:
branch: 'chore/retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0'
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0-design.md](../../superpowers/specs/2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0-design.md) |
| Plan | [2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0.md](../../superpowers/plans/2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0.md) |
| Results | [2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0-results.md](../../results/2026-11-08-retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0-results.md) |
<!-- docket:artifacts:end -->

## Why

Change 0511 adds saved v0.9.2 and v0.9.3 Bash state under `testdata/bash-upgrade/` and a test that runs the upgrade guide against it. They exist only to keep the guide true while the code changes during the pre-releases. Users never run them. The human decided to retire them once stable v1.0.0 ships.

## What changes

- Delete the saved Bash cases (`testdata/bash-upgrade/`), the `internal/bashupgrade/` package, its integration shard `tests/test_go_integration_bashupgrade.sh` and budget row, and the `testdata/README.md` section describing the cases.
- Keep the `dckt` alias test that lived in that package by moving it to `cmd/docket`, which already builds the binary.
- Edit `docs/release/upgrading-from-bash.md` so it describes only the upgrade: remove or rewrite every sentence about the retired test and drop its step markers. Steps, commands, and tables are unchanged.

## Out of scope

- Changing any upgrade step, command, or table in the guide.
- Any change to the installer's v0.9.2 takeover (ADR-0096).
- Editing point-in-time records (release decisions, archived specs/plans/results, the frozen export corpus) that mention the test.

## Reconcile log

### 2026-11-08

Unchanged against `main` 2ee250bdc: every file the spec names still exists and the maintained grep hits match the spec.
