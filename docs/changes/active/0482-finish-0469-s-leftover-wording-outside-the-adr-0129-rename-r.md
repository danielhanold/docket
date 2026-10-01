---
id: 482
slug: 'finish-0469-s-leftover-wording-outside-the-adr-0129-rename-r'
title: 'Finish 0469''s leftover wording outside the ADR-0129 rename rows'
status: 'proposed'
priority: 'low'
type: 'refactor'
created: '2026-10-01'
updated: '2026-10-01'
depends_on: [469]
stacked_on:
related: []
discovered_from: [469]
adrs: [129]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0129](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0129-collision-free-docket-vocabulary.md) |
<!-- docket:artifacts:end -->

## Why

Change 0469 replaced opaque docket terms per the ADR-0129 rename rows, but its build found wording that no row covers and therefore left it alone: three "repair" message strings in `change relink`, and "Step 0" in some Go comments, one shell test, and the `repository prepare` CLI help text. These leave the old vocabulary visible next to the new names. Reported in 0469's run report as a follow-up.

## What changes

Trace each leftover occurrence from a whole-repo grep, decide the replacement wording (adding ADR-0129 rows via an `## Update` if a new rename is warranted), and update the `change relink` messages, the Go comments, the shell test, and the `repository prepare` help text, plus any tests that assert those strings.

## Out of scope

Splitting the overloaded gate-drive halt tokens (separate change); point-in-time records (results files, archived changes, specs, Accepted ADR bodies), which keep their historical wording.
