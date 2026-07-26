---
id: 131
slug: make-board-conflict-rebase-continuation-noninteractive
title: Make board-conflict rebase continuation noninteractive
status: killed
priority: medium
created: 2026-07-22
updated: 2026-07-26
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
type: fix
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

## Why killed

Already fixed by change 0132 (`2e3789ca`, "Install configured Bash 4+ runtime", merged 2026-07-22 — the same day this stub was filed from 0128's results).

The stub described the full-suite board-conflict fixture blocking at `git rebase --continue`, where Git launches an editor and the run hangs on `E303: Unable to open swap file` instead of completing. 0132 added `GIT_EDITOR=true` to the fixture invocation at `tests/test_docket_status.sh:503`, making the continuation noninteractive — the fix this change asked for.

Verified 2026-07-26 by running `tests/test_docket_status.sh` to completion: all six board-conflict assertions pass, including `conflict run exits zero` (the blocking symptom) and the unconditional `conflict run reports the deterministic pushed outcome`. The suite reaches a result rather than hanging.

The stub's second ask — "add a regression witness that the conflict branch completes under the same unattended environment" — is satisfied by that same `conflict run exits zero` assertion, which is unconditional and would redden if the fixture ever hung again.

Killed rather than closed as done: no work was performed under this id. Recorded so the incidental fix is legible.
