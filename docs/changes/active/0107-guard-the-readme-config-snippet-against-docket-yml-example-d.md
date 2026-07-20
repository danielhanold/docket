---
id: 107
slug: guard-the-readme-config-snippet-against-docket-yml-example-d
title: Guard the README config snippet against .docket.yml.example drift
status: in-progress
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [101]
adrs: []
spec: docs/superpowers/specs/2026-07-20-readme-snippet-drift-guard-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/guard-the-readme-config-snippet-against-docket-yml-example-d
claimed_at: 2026-07-20T12:14:15Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-readme-snippet-drift-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-readme-snippet-drift-guard-design.md) |
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

Add one new numbered section to `tests/test_docket_yml_example.sh` — the file that already owns
every example-mirroring invariant, which is where ADR-0048's must-update rule points. It extracts
the README snippet's keys, flattens them to dotted paths generically (the snippet already nests
under `finalize:`, and more nesting is expected), and asserts each shown key exists in
`.docket.yml.example` with a matching value — plus that the README's pointer to the example
resolves to a real path.

The guard runs **one direction only**, by design: it iterates the snippet's keys, never the
example's. The reverse loop would be a completeness assert, which is precisely the fourth all-keys
surface change 0101 deleted — the snippet is a deliberate proper subset, not a mirror. That
departure from the `correspondence-guard-runs-one-way` learning is reasoned out in the spec and must
be written into the test as a comment, so a later reader does not "fix" the absent reverse loop.

Guarding the forward direction alone makes vacuity the live risk, so the section also carries an
exact-count assert on the extracted keys — a broken heading or fence must redden rather than pass
with an empty loop.

## Out of scope

- Regenerating the README snippet from the example (codegen was explicitly rejected for the example
  itself in ADR-0048; the same hand-maintained-mirror trade-off applies).
- Auditing the rest of the README's prose claims against the code — a broader problem than this
  snippet, and unguardable by grep (see the `verify-the-claim` finding).
- Any reverse/completeness direction over the example's keys.
