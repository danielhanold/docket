---
id: 295
slug: make-render-change-links-sh-genuinely-offline-safe-stop-re-r
title: 'Make render-change-links.sh genuinely offline-safe — stop re-resolving config with a network fetch'
status: 'killed'
priority: high
type: fix
created: 2026-08-11
updated: '2026-09-03'
depends_on: []
related: []
discovered_from: [118]
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

**Trigger** — surfaced while reconciling change 0118 (marking the sweep's skipped-publish leg). The
spec's Decision section had to accept a correlated residual it could not remove: the motivating
transient cause of a renderer failure is a network blip, and pushing the marker that records it
needs the same network that just failed.

**Opportunity** — `render-change-links.sh` is documented as **offline-safe** (the convention's
derived-view script family names it so), but it unconditionally resolves config through
`docket-config.sh --export`, which runs `git fetch origin` and dies on fetch failure. So the
renderer is network-dependent in fact and offline-safe only in prose. Nothing today lets a caller
that has *already resolved config* hand those values to the renderer instead of paying a second,
redundant fetch.

**Independent value** — stands entirely without 0118. Every caller of the renderer benefits: the
sweep's artifacts refresh, `render-change-links` after each field write, and close-out. It removes
a whole class of spurious failure (a network blip firing a close-out failure branch), and it makes
the doc's offline-safe claim true rather than aspirational.

**Boundary** — the redundant config resolution in the renderer (and the same shape in any sibling
derived-view renderer a whole-repo grep turns up); a pass-through or cache so an already-resolved
caller does not re-fetch. It deliberately leaves alone: `docket-config.sh`'s own fetch on the paths
that genuinely need fresh origin state, the marker contract, and every close-out failure posture.

**Reason for deferral** — 0118's branch is a close-out *marking* change; touching the config
resolver's fetch behavior changes what every docket script does at startup, which is a far larger
blast radius than the branch's intended scope and needs its own review. The 0118 spec names it as
follow-up work explicitly (§4, "No renderer-fetch elimination and no push retry").

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): already fixed in Go — change-links and backlink rendering are pure functions in internal/render; the only fetch is the transaction engine's own base fetch.
