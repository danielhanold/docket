---
id: 566
slug: 'stop-superpowers-brainstorming-s-builder-check-subagent-in-d'
title: 'Stop superpowers brainstorming''s builder-check subagent in docket grooming'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-10-17'
updated: '2026-10-19'
depends_on: []
stacked_on:
related: [563]
discovered_from: [563]
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

The current `superpowers:brainstorming` skill (7.0.0) ends every written design with a "builder check": it dispatches a fresh general-purpose subagent that reads the spec as a would-be planner and reports its open questions, then comes back to the human with another round of questions. `docket-groom-next` and `docket-new-change` invoke that skill for their brainstorm role, so every interactive groom now pays for it. The subagent is not a docket agent and is not model-pinned: it runs at the parent's model and effort. Docket already reviews the spec downstream: the plan-writer reads it at plan time, and the implementer's reconcile pass re-validates it against current reality before building. So the builder check duplicates work docket does later, at full parent cost, and adds a question round to a human-attended session. More broadly, the brainstorming skill now carries steps docket does not need (builder check, visual companion, spike offers, its own spec location and size-the-work step) — superpowers is doing too much for docket's grooming. Seen while grooming change 563.

## What changes

Stop the builder-check dispatch when docket's interactive workflows run the brainstorm role, without patching the vendored plugin. The likely shape is a call-site direction in `docket-groom-next` and `docket-new-change` (phrased by shape, not by citing a vendored heading) telling the brainstorm to end at the settled design with no reviewer dispatch. Treat that as a hypothesis: while grooming, also weigh whether the other superpowers steps docket does not use should be suppressed the same way, or whether the brainstorm role should move to a docket-owned skill (`docket-brainstorm` exists for the consultant flow).

## Out of scope

Patching or forking the superpowers plugin. The plan-time and reconcile reviews themselves. The autonomous `docket-auto-groom` critic gate.
