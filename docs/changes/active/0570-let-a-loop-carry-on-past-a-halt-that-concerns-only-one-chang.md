---
id: 570
slug: 'let-a-loop-carry-on-past-a-halt-that-concerns-only-one-chang'
title: 'Let a loop carry on past a halt that concerns only one change'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-10-18'
updated: '2026-10-18'
depends_on: []
stacked_on:
related: [559, 560]
discovered_from: [560]
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

Today every `halted` disposition stops the driver, in both `docket-implement-next` and `docket-finalize-change`. Some halts concern only the change at hand: a design the reconcile pass finds invalidated, a build that cannot get its suite green, a results-artifact change that turns out to need code. Others would halt every remaining change the same way: broken configuration, an agent that cannot be started, a failed preflight. The driver cannot tell the two apart, so one change's halt ends an overnight drain.

#560 removes the most common planned cause: a change that needs a human is put on hold and no loop picks it. The other per-change halts still end the loop.

## What changes

Give a halt a reason a driver can read, so a loop carries on past a halt that concerns only one change and still stops on one that would recur.

To settle when grooming:

- The reason's shape and where it is recorded. `change.halt` takes only a free-text report today, and a halt before any claim is recorded nowhere: the run tracker reports it as `no-attributable-claim`, which it cannot tell apart from an empty queue.
- How every halt point in implement-next and finalize is classified, and the default for one nobody classified. Stopping is the safe default: a setup-wide halt misread as a one-change halt would claim and halt every remaining change, each needing a person to resume it.
- Whether a cap on consecutive halts is needed as a backstop.
- Whether finalize shares the rule.
- Where the loop logic lives: the skill body, or a reference file read only when it applies, so the common path's prompt does not grow.

## Out of scope

Keeping a change that needs a human away from the loop: #560's hold does that. How a run behaves when it reaches a human step: #559 settled that.
