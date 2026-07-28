---
id: 148
slug: two-unfalsifiable-z-asserts-in-the-config-suite-sit-in-eval
title: Two unfalsifiable -z asserts in the config suite sit in eval-free blocks
status: proposed
priority: medium
type: chore
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [126]
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

`tests/test_docket_config.sh:1534` and `:1550` each assert `[ -z "$DOCKET_BASH_PATH" ]` inside a
block that contains **no `eval` at all**, preceded by a local `DOCKET_BASH_PATH=""`. The asserted
value and the assigned value are the same, and nothing between them can change it — so neither
assert can ever fail. They are unfalsifiable: green for a reason that has nothing to do with the
property they name.

Found by the whole-branch review of change 0126, which added a correspondence guard over this same
file. The guard deliberately did **not** close this: it reasons about eval sites and the variables
their asserts read, and a block with no eval site is structurally outside its view. Change 0126 left
the two lines byte-untouched and recorded them as out of scope, which is the honest call — but it
means the file now carries a guard that proves a real property alongside two asserts that prove
nothing, and the guard's green makes that harder to notice rather than easier.

The `guards-are-code` rule already covers this class. What is needed is a decision about these two
specific asserts: either give them a real eval so they can fail, or delete them as dead weight.

## What changes

- Decide, per assert, what property `:1534` / `:1550` were meant to pin — the change-0132 runtime
  section is the context to read.
- Either wire each to a real resolver run so it can redden, or remove it.
- Consider whether a general check for "assert reads an exported key in a segment with no eval" is
  worth adding, or whether that is over-fitting to two sites.

## Out of scope

- The change-0126 correspondence guard's own logic, which is working as designed.
