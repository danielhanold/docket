---
id: 107
slug: guard-the-readme-config-snippet-against-docket-yml-example-d
title: Guard the README config snippet against .docket.yml.example drift
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [101]
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

Change 0101 made `.docket.yml.example` the single canonical config reference and retired the other
all-keys surfaces — but it discovered mid-review that the README still carried a full commented
`.docket.yml` sample, already drifted (no `learnings:` block, no mention of the new `auto`
sentinel). That was a fourth all-keys surface: precisely the defect 0101 existed to end, surviving
inside the change that ended it.

It was cut down to a five-key illustrative snippet plus a pointer to the example. That is the right
shape, but **nothing tests the README against the example**, so the snippet is a drift surface by
construction: its five keys can silently diverge from the canonical file's values, and the pointer
can rot if the example is ever renamed or relocated. The change's own results file records this as
an accepted, unguarded residual.

## What changes

Add a guard — most likely in `tests/test_docket_yml_example.sh`, which already owns the
example's invariants — asserting that the README's illustrative snippet stays consistent with
`.docket.yml.example`: every key the snippet shows exists in the example, and each shown value
matches the example's value for that key. Plus an assert that the README's pointer resolves to the
example's real path.

Keep the snippet's *purpose* intact — it is deliberately a small taste, not a mirror, so the guard
must check the keys it shows and never demand completeness (a completeness assert here would
recreate the fourth all-keys surface the change deleted).

## Out of scope

- Regenerating the README snippet from the example (codegen was explicitly rejected for the example
  itself in ADR-0048; the same hand-maintained-mirror trade-off applies).
- Auditing the rest of the README's prose claims against the code — a broader problem than this
  snippet, and unguardable by grep (see the `verify-the-claim` finding).

## Open questions

- Should the guard live with the example's invariants (`test_docket_yml_example.sh`) or with the
  README's own doc tests? The former keeps every example-mirroring rule in one file, which is what
  ADR-0048's must-update rule points at.
- Is the direction-of-truth assert worth adding both ways here, given change 0101's finding that a
  correspondence guard proves only the direction it iterates?
