---
id: 151
slug: vacuous-docket-bash-path-asserts-sit-in-eval-free-blocks-out
title: Vacuous DOCKET_BASH_PATH asserts sit in eval-free blocks, out of the poison-prelude guard's reach
status: proposed
priority: medium
type: fix
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

`tests/test_docket_config.sh` contains asserts of the form `[ -z "$DOCKET_BASH_PATH" ]` (near the
0132 runtime-resolution section) inside blocks that contain **no `eval` at all**. They can never
fail regardless of what the variable actually holds — they are vacuous.

Change 0126's poison-prelude guard cannot see them. Its whole mechanism is keyed on `eval` sites, so
a correspondence guard of that shape has no reach into fixtures that never eval anything. This is a
distinct defect class from the stale-value hazard 0126 addressed, and was explicitly left out of its
scope.

## What changes

- Identify every assert in `tests/test_docket_config.sh` that reads a resolver-exported variable
  inside a block that performs no resolver `eval`.
- Decide per site: give the block a real resolver invocation, or delete the assert as meaningless.
- Consider whether the 0126 guard (or a sibling) can be widened to detect the class, rather than
  fixing only today's instances.

## Out of scope

- The poison-prelude guard's need-window / cleared-window asymmetry (documented in-file by 0126).
- The exempt-ceiling drift question (parked separately in 0126's results).

## Open questions

- Is a general "assert reads an exported resolver key in a block with no eval" check feasible, or
  does it produce too many false positives against helper-driven fixtures?
