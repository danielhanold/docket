---
id: 173
slug: field-of-silently-truncates-a-model-id-containing-or
title: 'field_of() silently truncates a model ID containing / or :'
status: done
priority: medium
type: fix
created: 2026-07-31
updated: 2026-07-31
depends_on: []
related: [175]
discovered_from: [168]
adrs: [65]
spec: docs/superpowers/specs/2026-07-31-field-of-silently-truncates-a-model-id-containing-or-design.md
plan: docs/superpowers/plans/2026-07-31-field-of-value-class-truncation.md
results: docs/results/2026-07-31-field-of-silently-truncates-a-model-id-containing-or-results.md
trivial: false
auto_groomable:
branch: feat/field-of-silently-truncates-a-model-id-containing-or
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/142
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-31-field-of-silently-truncates-a-model-id-containing-or-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-31-field-of-silently-truncates-a-model-id-containing-or-design.md) |
| Plan | [2026-07-31-field-of-value-class-truncation.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-31-field-of-value-class-truncation.md) |
| Results | [2026-07-31-field-of-silently-truncates-a-model-id-containing-or-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-31-field-of-silently-truncates-a-model-id-containing-or-results.md) |
| ADRs | [ADR-0065](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0065-bare-scalar-validation-needs-an-explicit-quote-leg.md) |
<!-- docket:artifacts:end -->

## Why

`field_of()` in `sync-agents.sh` extracts a config value with the character class
`([A-Za-z0-9._-]+)`. A model ID containing `/` or `:` is therefore silently truncated to its first
segment: a user who writes `model: anthropic/claude-opus-5` in `.docket.yml`, `.docket.local.yml`,
or the global `config.yml` resolves to `anthropic`, and the generator bakes that wrong pin into the
wrapper without a warning. Provider-prefixed model IDs are ordinary, and ADR-0015 makes model IDs
opaque passthrough values with no vendor allowlist — so docket has no basis for assuming the
narrow class.

The defect is pre-existing (identical on `origin/main`; the function is marked "UNCHANGED — kept
verbatim"). Change 0168 hit the same class in the new shipped-defaults reader
`scripts/lib/harness-defaults.sh` and fixed it there, which is what surfaced this sibling. The two
readers now disagree: the sidecar accepts a provider-prefixed ID, user config still truncates it.

This is arguably the worse of the two, since user config is exactly where a provider-prefixed ID
gets typed by hand. Change 0168 is now merged, so the corrected twin is readable on `main` and the
two readers demonstrably disagree.

A third reader shares the class: `scripts/runner-dispatch.sh:75`, which exports free-form
`runners.<RUNNER>.*` values as `DOCKET_RUNNER_CFG_*`.

## What changes

- Widen `field_of()`'s value class to the flow-map delimiter class, and add a `field_of_raw`
  companion plus a validator that fails generation loudly — before any wrapper is written — when a
  value is not a bare scalar. Matches `harness-defaults.sh`'s `hd_field`/`hd_field_raw` pair.
- Fix the same class in `scripts/runner-dispatch.sh`'s value read, with a block-mapping value class
  and a deliberately tolerant posture (skip, never die — it is a live dispatch path).
- Regression coverage across all three user layers and the `agents.default` vs `agents.<harness>`
  merge, asserted at value level since the truncation is silent.

## Out of scope

- Any vendor model allowlist or availability lookup (ADR-0015 forbids it).
- The shipped-defaults reader in `scripts/lib/harness-defaults.sh`, already fixed by change 0168.
- Factoring a shared extractor across the three readers — a follow-up, deliberately deferred until
  change 0175 settles `field_of`'s implementation.

## Reconcile log

### 2026-07-31 — reconciled at claim, no scope change

Re-read the change and its spec against `related` (0175), `discovered_from` (0168, merged),
recently-archived changes, and CURRENT code on `origin/main` (`9d41fa6b`). Everything the spec
asserts still holds verbatim:

- `sync-agents.sh:262` still carries `([A-Za-z0-9._-]+)` under the comment `field_of() — UNCHANGED
  (kept verbatim from the prior version)`. `sync-agents.sh` sits at the **repo root**, not under
  `scripts/` — worth stating because every other reader named here is under `scripts/`.
- `scripts/lib/harness-defaults.sh` carries the corrected twin (`hd_field` `[^,}[:space:]]+`,
  `hd_field_raw` `[^,}]*`, and `hd_validate`'s bare-scalar leg with the exact diagnostic shape the
  spec asks this change to mirror). The two readers do disagree today, as the Why claims.
- `scripts/runner-dispatch.sh:75` still reads its value with the narrow class, and the per-key
  precedence claim (`seen_keys` claimed before the value is parsed) is present and must survive.
- Both target test files exist: `tests/test_sync_agents.sh`, `tests/test_runner_dispatch.sh`.

Build-order coupling confirmed from the other side: 0175 carries `depends_on: [173]`, so this
change lands first and 0175 inherits the widened class.

No scope adjustment, no work already done elsewhere, no new constraint. The deferred
shared-extractor refactor is already tracked as change 0179 (`waiting-on-175-unbuilt`), so it is
not re-captured here.
