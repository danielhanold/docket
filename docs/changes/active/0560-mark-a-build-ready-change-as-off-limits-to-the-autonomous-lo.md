---
id: 560
slug: 'mark-a-build-ready-change-as-off-limits-to-the-autonomous-lo'
title: 'Mark a build-ready change as off-limits to the autonomous loop'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-17'
updated: '2026-10-17'
depends_on: []
stacked_on:
related: [559]
discovered_from: [559]
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

Some changes need a person for one or more steps: signing in somewhere, a manual check, a decision only a human can make. They are groomed build-ready like any other change (a spec or the trivial flag, and their dependencies done), so an autonomous loop draining the backlog cannot tell them apart. It claims such a change, works until it reaches the human step, and halts. A halt stops the loop today, so one such change ends the whole drain.

#559 accepts this gap: until this change ships, a person who runs a loop lists change ids and checks that none of them needs a human.

## What changes

Add a field that marks a build-ready change as needing a human, so autonomous selection skips it while an explicit, attended run of that change still takes it. Make the field editable through `change.groom` from the start.

To settle when grooming:

- The field's name and values.
- Which runs skip it: a queue-wide drain, an id-set drain, or both. A run naming a single change id should still take it.
- Whether a loop should continue past a halt that concerns only one change instead of stopping. Today every `halted` outcome stops the driver, because some halts (broken configuration, an agent that cannot be started) would halt every remaining change the same way. Telling the two kinds apart needs a typed halt reason the driver can read, and `docket-finalize-change` shares the same continue/stop rule.
- Where the loop logic lives: in the `docket-implement-next` skill body, or in a reference file the skill reads only when it applies (progressive disclosure), so the common path's prompt does not grow.

## Out of scope

How a run behaves when it reaches a human step: #559 settles that (the run does what it can and halts at the step).
