---
id: 514
slug: 'retire-the-saved-bash-upgrade-test-cases-when-stable-v1-0-0'
title: 'Retire the saved Bash upgrade test cases when stable v1.0.0 ships'
status: 'deferred'
priority: 'low'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: [511]
stacked_on:
related: [511, 366]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
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

Change 0511 adds saved v0.9.2 and v0.9.3 Bash state under `testdata/bash-upgrade/` and a test that runs the upgrade guide against it. They exist only to keep the guide true while the code changes during the pre-releases. Users never run them. The human decided to retire them once stable v1.0.0 ships.

## What changes

- Delete `testdata/bash-upgrade/` and the `TestBashUpgrade` test, plus its runtime budget row and anything else that only exists for it.
- Keep `docs/release/upgrading-from-bash.md` as a frozen document. Add one line saying it was last verified against v1.0.0.

## Out of scope

- Deleting or rewriting the guide itself.
- Any change to the installer's v0.9.2 takeover (ADR-0096).

## Why deferred

Parked on purpose until stable v1.0.0 ships (human decision, 2026-10-04). Until then the saved Bash cases and their test keep the upgrade guide (0511) honest through alpha.2 and alpha.3. Revive this once v1.0.0 is published.
