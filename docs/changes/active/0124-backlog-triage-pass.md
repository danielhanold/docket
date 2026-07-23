---
id: 124
slug: backlog-triage-pass
title: Backlog triage pass — kill, defer, or arm each needs-brainstorm stub
status: proposed
priority: medium
created: 2026-07-21
updated: 2026-07-21
depends_on: []
related: []
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
type: chore
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The needs-brainstorm queue is growing faster than grooming drains it. Measured 2026-07-21:
**17 stubs**, of which **6 were created that day** and **10 of 17 were auto-captured**
(`discovered_from:` set) rather than filed by hand. Four date to 2026-06-11 and have never been
touched.

Grooming them one at a time is the wrong instrument. Most of that queue does not need a design
conversation — it needs a *verdict*. Several stubs are plausibly dead (superseded by work that has
since merged, or duplicated by a sibling), and several others are mechanically obvious enough that
`docket-auto-groom` could carry them to build-ready with no human at all. Both outcomes are
cheaper than a grooming session, and both shrink the queue rather than merely advancing it.

`auto_groom` is `false` repo-wide, so today every stub waits on the maintainer personally. That is
the actual bottleneck, and it is a configuration choice rather than a fact.

## What changes

A single pass over every needs-brainstorm stub in `active/`, reaching one of three verdicts each:

- **Kill** — obsolete, superseded, or a duplicate. Drives the proposed-kill sub-path.
- **Defer** — right idea, wrong time. `status: deferred` plus a `## Why deferred`.
- **Arm** — mechanically clear enough to design without a human: commit `auto_groomable: true` so
  `docket-auto-groom` drains it.

Anything genuinely needing design judgment keeps its current state and stays in the human queue.

Input is `docket-status --digest-only` (shipped by #0094) — a write-free read that already emits
every stub with its status, readiness, and slug in selection order. No tooling is needed.

Arming is deliberate, not broad: `auto_groomable: true` must be **committed** before dispatch, and
arming N stubs drains all N in one autonomous run.

## Out of scope

- Changing the `auto_capture` knob or its materiality bar. The inflow rate is a real and separate
  question; this change addresses the standing queue, not the tap.
- Flipping `auto_groom` repo-wide. Per-stub arming is the deliberate unit here.
- Actually designing any stub. Arming hands that to `docket-auto-groom`; it does not do it.

## Open questions

- Which stubs are genuinely dead? The four dating to 2026-06-11 are the first candidates, but
  each needs a read against what has merged since.
- Is there a size threshold past which a repo-wide `auto_groom: true` beats per-stub arming?

## Notes

**This change is metadata-only and is NOT for `docket-implement-next`.** It produces no code and
no feature branch — its deliverable is a set of frontmatter and status edits on the metadata
branch. Sent through the autonomous implementer it would cut a `feat/` branch, find nothing to
build, and open an empty PR. It is deliberately left needs-brainstorm with `auto_groomable: false`
so that neither the implementer nor `docket-auto-groom` can pick it up; it is executed
interactively by a human, and reaches `killed` (not `done`) when the pass is complete.

Filed at the maintainer's explicit direction after the alternative — running the triage inline in
the grooming session that surfaced it — was offered and declined in favour of scheduling it.
