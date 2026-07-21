---
id: 120
slug: docket-finalize-change-claims-integration-branch-is-read-fro
title: docket-finalize-change claims integration_branch is read from .docket.yml, but it is an exported resolver key
status: proposed
priority: medium
created: 2026-07-21
updated: 2026-07-21
depends_on: []
related: []
discovered_from: [102]
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

`skills/docket-finalize-change/SKILL.md` states that `<integration_branch>` is "resolved from
`.docket.yml`" — but `INTEGRATION_BRANCH` is an **exported resolver key**, emitted in the Step-0
`preflight` block like `FINALIZE_GATE` and `CHANGES_DIR`.

This is the exact bug class change 0102 just closed, one key over: a user who sets
`integration_branch` in `.docket.local.yml` gets a value that every *script* honors (they read the
export) while the *skill body* ignores it (it is told to read the committed file). The two halves
of the toolchain would disagree about where code lands — a worse blast radius than 0102's, since
`integration_branch` decides the merge target.

Note `integration_branch` IS coordination-key fenced (ADR-0019), so a machine-scoped value is
warned-and-ignored rather than silently dropped. That makes this less severe than 0102 — the user
gets a warning — but the skill's prose is still factually wrong about its own read channel, and
ADR-0052 now states the rule it violates.

Surfaced by the whole-branch review of change 0102.

## What changes

- Correct the skill body's provenance claim to name the exported `INTEGRATION_BRANCH` read from the
  Step-0 block, matching how 0102 fixed `FINALIZE_REQUIRE_PR_APPROVAL`.
- Audit the other skill bodies for the same shape — any prose telling an agent to read a value
  "from `.docket.yml`" when that value is an exported resolver key. ADR-0052 makes this a rule, so
  this is now enforcement of a stated boundary rather than a one-off fix.
- Consider whether a sentinel can guard the class, rather than fixing occurrences one at a time.

**Note:** `skills/docket-finalize-change/SKILL.md` sits near its word budget; change 0102 raised it
to 4200 to leave headroom, so this edit should fit, but check before assuming.

## Out of scope

- Changing what `integration_branch` means or how it resolves.
- Re-litigating its coordination-key fencing.
