---
id: 107
slug: guard-the-readme-config-snippet-against-docket-yml-example-d
title: Guard the README config snippet against .docket.yml.example drift
status: implemented
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [101]
adrs: []
spec: docs/superpowers/specs/2026-07-20-readme-snippet-drift-guard-design.md
plan: docs/superpowers/plans/2026-07-20-readme-snippet-drift-guard-plan.md
results: docs/results/2026-07-20-readme-snippet-drift-guard-results.md
trivial: false
auto_groomable:
branch: feat/guard-the-readme-config-snippet-against-docket-yml-example-d
claimed_at: 2026-07-20T13:36:50Z
pr: https://github.com/danielhanold/docket/pull/110
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-readme-snippet-drift-guard-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-readme-snippet-drift-guard-design.md) |
| Plan | [2026-07-20-readme-snippet-drift-guard-plan.md](https://github.com/danielhanold/docket/blob/feat/guard-the-readme-config-snippet-against-docket-yml-example-d/docs/superpowers/plans/2026-07-20-readme-snippet-drift-guard-plan.md) |
| Results | [2026-07-20-readme-snippet-drift-guard-results.md](https://github.com/danielhanold/docket/blob/feat/guard-the-readme-config-snippet-against-docket-yml-example-d/docs/results/2026-07-20-readme-snippet-drift-guard-results.md) |
| PR | [#110](https://github.com/danielhanold/docket/pull/110) |
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

## Reconcile log

### 2026-07-20 — reconcile at claim

Verified the spec's premises against `origin/main` at the moment of claim. Everything it was
designed against still holds; **no scope change**.

- `tests/test_docket_yml_example.sh` exists and still ends at numbered section **`(7) README +
  dogfooding`**, so the new section lands as `(8)` exactly as specified. `(7)` already asserts
  README facts (the step-2 heading, the example reference, the absence of `config.yml.example`),
  so the precedent for guarding the README from this file is intact.
- The README still carries the five-key snippet under `### `.docket.yml` — per-repo settings`,
  fenced as `yaml`, with `finalize:` → `gate:` as its one nested key. Flattening yields exactly
  the five paths the spec predicted: `metadata_branch`, `integration_branch`, `board_surfaces`,
  `finalize`, `finalize.gate`.
- All five values agree with `.docket.yml.example` today (`metadata_branch: docket`,
  `integration_branch: auto`, `board_surfaces: [inline]`, `finalize:` header, `gate: local`), so
  the guard goes green on first run — it is a fresh guard over a currently-honest pair, not a
  latent red.
- The canonical-reference pointer in that section links to `.docket.yml.example`, which exists at
  the repo root. Note for implementation: the README names `.docket.yml.example` in several other
  places (the tooling list, the layered-config prose), so the pointer assert must be **scoped to
  the section**, not a whole-file grep — an unscoped match would stay green if the section's own
  link rotted.

Cross-checked the two open PRs for base drift (the `moving-base` learning):

- **PR #89** (change 0078) touches `README.md` only around line ~418 (the sync-agents prose) — no
  overlap with the config snippet or the test file.
- **PR #69** (change 0044, `blocked`, flagged stale by `docket-status`) touches `README.md` in a
  `finalize:` snippet region under the pre-refactor line numbering. It adds config-snippet keys.
  This is not a conflict for our base, and it needs no scope change here — if that PR ever rebases
  and lands a new key in the README snippet, the guard this change adds is precisely what forces
  the same key into `.docket.yml.example`. The guard working on it is the intended outcome.

No change to `## What changes` or `## Out of scope`. The `no ADR` call in the spec still stands:
the forward-only direction is a test-design decision scoped to one guard, recorded in the spec and
(per the spec) required to be written into the test as a comment.
