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
related: [149, 151]
discovered_from: [126]
adrs: []
spec: docs/superpowers/specs/2026-07-28-unfalsifiable-runtime-asserts-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-07-28-unfalsifiable-runtime-asserts-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-28-unfalsifiable-runtime-asserts-design.md) |
<!-- docket:artifacts:end -->

## Why

`tests/test_docket_config.sh` asserts `[ -z "$DOCKET_BASH_PATH" ]` at two sites on the 0132
fail-closed runtime paths (the `runtime invalid` loop, five cases, and the `runtime absent` block).
Each is preceded by a bare `DOCKET_BASH_PATH=""` — and that seed is the whole defect: it forces the
asserted value to the value the assert demands, so neither assert can ever fail. They are green for
a reason unrelated to the property they name.

Change 0126's poison-prelude guard does *see* these asserts (its need-windows tile the file), but it
cannot *detect* the vacuity: the guard proves each site clears the variables its window's asserts
read, and a `VAR=""` clear satisfies it — a limitation the file already names in-comment. 0126 left
both lines byte-untouched and recorded them as out of scope, which was the honest call.

The property the asserts name is already proven one line above, by `export is empty`. On a
fail-closed path the resolver emits nothing, and the export is the sole channel by which it could
set the variable in a caller — so the per-variable claim is implied, not additive.

## What changes

Delete both asserts and their `DOCKET_BASH_PATH=""` seeds, recording the sole-channel reasoning
in-file so they do not get re-added. Delete the `DOCKET_BASH_PATH=__poison__` **clause** (not the
whole compound line) in the `require_pr_approval` fixture along with its now-false "Load-bearing; do
not delete" comment — it exists solely to satisfy the asserts being removed and becomes dead code
with them. Deliberately do **not** insert a no-op `eval` to make the asserts falsifiable: on a
provably-empty export that is guard-gaming, not coverage.

Verified empirically: assert count drops 381 → 375, the 0126 guard's `TOTALS` line stays
`sites=64 exempt=3 ok=61 viol=0`, and no other assert changes verdict.

Design settled in the linked spec.

## Out of scope

- Change 0126's correspondence guard logic, which is working as designed.
- The guard's exempt-bound shape — change 0149 owns it, in the same file.
- A general "assert reads an exported key in a block with no eval" checker: over-fitting against two
  known instances, both removed here.
- Change 0151's sweep-and-widen framing, of which this discharges the concrete half.
