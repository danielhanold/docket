---
id: 108
slug: guard-the-readme-s-remaining-config-fences-against-key-drift
title: Guard the README's remaining config fences against key drift
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [107]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0107 added `(8) README SNIPPET CORRESPONDENCE` to `tests/test_docket_yml_example.sh`,
guarding the README's per-repo-settings `.docket.yml` snippet against drift from
`.docket.yml.example`. That guard is deliberately scoped to **one fence** — the section's single
worked example — and its whole-branch review surfaced that the README carries **several other
config fences that nothing guards at all**:

- `auto_capture: true` (~README:264)
- `terminal_publish: true` (~README:407)
- `metadata_branch: main` (~README:433)
- the global `config.yml` sample (~README:291) and the `.docket.local.yml` sample (~README:315)
- the `skills:`/runner fences (~README:574, ~594)

Each is a place a documented key name or value can rot exactly the way the per-repo snippet could
before 0107 — a key renamed in the resolver, or a key that never existed, would sit in the README
indefinitely.

The reason 0107 did not simply extend its loop is recorded in its own test comment: those fences
**deliberately show NON-default values** to illustrate opting in, so 0107's value-equality assert
would go spuriously RED against them for being correct. Guarding them needs a different assert —
key **existence** in `.docket.yml.example` (and/or in the resolver's export surface) without value
comparison — which is a real design call, not a mechanical copy of the existing section.

## What changes

Extend the example-mirroring suite to cover the README's remaining config fences with an
existence-only correspondence check, leaving value comparison to the one fence that documents
shipped defaults. Needs a design pass on: which fences are in scope, whether the check anchors on
the example or on the resolver's export keys, and how a fence declares "these values are
deliberately non-default" so the guard stays honest as the README grows.

## Out of scope

- Re-litigating 0107's forward-only direction, or adding any reverse/completeness loop over the
  example's keys — that is the all-keys surface change 0101 deleted.
- Auditing the README's non-config prose claims (see the `verify-the-claim` finding).
