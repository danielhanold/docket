---
id: 161
slug: enumerate-the-health-checks-failed-outcome-in-docket-status
title: Enumerate the health-checks-failed outcome in docket-status SKILL.md
status: proposed
priority: medium
type: docs
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [157]
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

Change 0157 (unit 0144) added a `health checks failed <exit>` line to `docket-status.sh`'s
`health_checks()`, but `skills/docket-status/SKILL.md`'s enumerated outcomes list was deliberately
left untouched because change 0145 owned that file at the time. 0145 has since landed, so the
skill's enumerated list is now missing an outcome the script really emits — a reader of the skill
cannot learn that the line exists.

## What changes

Add the `health checks failed <exit>` outcome to the enumerated outcomes list in
`skills/docket-status/SKILL.md`, matching the shape of the neighboring entries. Check
`scripts/docket-status.md` for the same gap while there.

## Out of scope

Any change to the script's behavior or the line's format.
