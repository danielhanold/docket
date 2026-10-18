---
id: 558
slug: 'name-the-serial-confirmation-not-counted-budget-line-in-agen'
title: 'Point the budget-report rule in AGENTS.md at tests/README.md instead of listing the lines'
status: 'proposed'
priority: 'low'
type: 'docs'
created: '2026-10-17'
updated: '2026-10-18'
depends_on: []
stacked_on:
related: [528, 557]
discovered_from: [528]
adrs: []
spec: 'docs/superpowers/specs/2026-10-18-name-the-serial-confirmation-not-counted-budget-line-in-agen-design.md'
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
| Spec | [2026-10-18-name-the-serial-confirmation-not-counted-budget-line-in-agen-design.md](../../superpowers/specs/2026-10-18-name-the-serial-confirmation-not-counted-budget-line-in-agen-design.md) |
<!-- docket:artifacts:end -->

## Why

AGENTS.md tells every agent to read the test suite's budget report even on a green run, since no budget line fails the run. The same bullet also keeps a hand-written list of what each line means, and that list has fallen behind twice: it leaves out `OVER BUDGET:` (a real breach from a single-job run or a `serial` budget row) and `SERIAL CONFIRMATION NOT COUNTED:` (a set-aside re-check that needs nothing). `tests/README.md` already explains every line. Only the reminder has to sit in always-loaded context. The meanings are reference an agent can look up once it has been told to read the report.

## What changes

Shrink the AGENTS.md budget-report bullet to the reminder plus a pointer to `tests/README.md`, with no line names. Make the README's budget paragraph say outright, for each of the five lines, whether it is a screening note, a breach to act on, or needs nothing. Cut the same copied list in the `tests/runtime-budgets.tsv` header comment to a pointer. Add the missing `OVER BUDGET:` line to the glossary's budget entry.

## Out of scope

Any change to the runner or its output. `.github/workflows/release-candidate.yml`, which already classifies the lines correctly. Point-in-time plans, specs, results, and release records that quote the old bullet. A guard test on AGENTS.md wording.
