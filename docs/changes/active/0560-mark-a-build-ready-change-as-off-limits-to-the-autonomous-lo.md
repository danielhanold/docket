---
id: 560
slug: 'mark-a-build-ready-change-as-off-limits-to-the-autonomous-lo'
title: 'Put a groomed change on hold so loops skip it'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-17'
updated: '2026-10-19'
depends_on: []
stacked_on:
related: [559, 570]
discovered_from: [559]
adrs: [6]
spec: 'docs/superpowers/specs/2026-10-18-mark-a-build-ready-change-as-off-limits-to-the-autonomous-lo-design.md'
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
| Spec | [2026-10-18-mark-a-build-ready-change-as-off-limits-to-the-autonomous-lo-design.md](../../superpowers/specs/2026-10-18-mark-a-build-ready-change-as-off-limits-to-the-autonomous-lo-design.md) |
| ADRs | [ADR-0006](../../adrs/0006-autonomous-grooming-bounds.md) |
<!-- docket:artifacts:end -->

## Why

Some groomed changes should not be built by an autonomous loop. One kind needs a person for some step: signing in somewhere, a manual check, a decision only the person can make. Another the person has groomed but does not want built yet, for any reason. Every build-ready change looks alike to a loop, so it claims such a change, works until the human step, and halts, and a halt ends the whole drain.

Today the only ways to keep one away are leaving it ungroomed on purpose (#547 does this) or deferring it. Deferring also blocks a run that names the change and stops grooming from revising it. #559 settled for a stop-gap: whoever starts a loop lists ids and checks that none needs a human.

## What changes

A change can carry a hold (`hold: true`). No autonomous build selection picks a held change: neither a drain over the whole backlog nor a drain over a list of ids. A run that names that one change still builds it.

The hold needs no reason. An optional one is written as a `## Why on hold` section and removed when the hold is lifted. Grooming (every exit except abstain) and change creation can set and lift the hold. When grooming settles on a spec that needs manual work, docket suggests a hold, and the auto-groomer sets one itself. The board shows a held change as `on hold`. implement-next learns one new skip reason, and an ADR records that build-ready no longer means a loop will build it. The spec holds the design.

## Out of scope

Grooming picks: a hold does not keep a stub out of groom-next's automatic pick or the auto-groomer's queue, which `auto_groomable` already controls; a hold on a stub takes effect once it is groomed. Finalize. Carrying on past a halt in a loop (#570). How a run behaves at a human step (#559 settled it). Showing the hold's reason on the board.
