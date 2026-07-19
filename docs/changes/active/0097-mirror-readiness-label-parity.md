---
id: 97
slug: mirror-readiness-label-parity
title: GitHub mirror readiness parity — readiness labels stop at `proposed`
status: proposed
priority: low
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [87]
discovered_from: [87]
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

`scripts/github-mirror.sh`'s `readiness_label` early-returns for any change that is not
`proposed`, so the `finalize-blocked` readiness state introduced by change 0087 never becomes a
`docket:readiness/` label. The inline board renders the cell and the digest surfaces it; the
mirror does not.

This is **not a regression** — the mirror never showed readiness for `implemented` changes. But
docket now has three projections of the same state (board, digest, mirror) and they disagree,
which is the kind of drift that gets noticed at the worst time: a maintainer watching the GitHub
Projects board sees nothing while the inline board says `finalize blocked — needs you`.

## What changes

Decide whether the mirror should carry readiness for non-`proposed` changes at all, and if so,
extend `readiness_label` past its `proposed` early-return so `finalize-blocked` maps to a label.
The real question is scope: only `finalize-blocked`, or every readiness state the board can render
for a non-`proposed` change.

Whatever is decided, the outcome should be a stated rule about which projections owe readiness —
not a one-off patch for a single state, or this recurs at the next readiness value.

## Out of scope

- Making the mirror two-way, or reading anything back from GitHub.
- Changing the inline board or digest readiness rendering (both already correct).

## Open questions

- Should the mirror mirror *all* readiness states for non-`proposed` changes, or is `proposed`-only
  a deliberate design that `finalize-blocked` should respect instead?
- Is a label the right surface for a state that is inherently transient?

## Reconcile log
