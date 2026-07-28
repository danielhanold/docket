---
id: 147
slug: extend-the-2c-orphan-key-check-past-its-column-0-anchor-to-n
title: Extend the (2c) orphan-key check past its column-0 anchor to nested keys
status: proposed
priority: medium
type: fix
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [122]
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

`tests/test_docket_example_yml.sh`'s `(2c)` orphan-key check enumerates `.docket.example.yml`'s
keys with the same column-0 anchor that change 0122 is removing from the scope-tag guard
(`^[A-Za-z_][A-Za-z0-9_]*:`). Every nested key — the 17 under `finalize`, `learnings`, `reclaim`,
`auto_capture`, `runners.codex`, and `skills` — is therefore invisible to the orphan-key direction
too: a documented nested key with no consumer anywhere in the codebase would never be flagged.

Change 0122 deliberately left this alone and recorded it as an observation. Its spec's assumption 8
explains why it is not a mechanical widening: `(2c)` anchors on **consumers**, and nested keys reach
their consumers through different paths than top-level keys do — `runners.*` through the runner
adapters, `skills.*` through the `SKILL_*` export names rather than the YAML key names. Deciding
what "has a consumer" means per nested-key family is a design question, not an edit to a regex.

## What changes

Decide and implement the consumer-resolution rule for nested keys in the `(2c)` orphan-key check,
then extend its enumeration past the column-0 anchor. At minimum the design has to settle how
`skills.*` keys map to their `SKILL_*` exports and how `runners.<runner>.*` keys map to their
adapter call sites, since neither is a literal key-name grep.

## Out of scope

- The scope-tag guard itself (change 0122 owns it).
- The classification manifest / `elsewhere:` check (change 0121 owns it).
