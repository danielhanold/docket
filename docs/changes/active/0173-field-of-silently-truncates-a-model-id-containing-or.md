---
id: 173
slug: field-of-silently-truncates-a-model-id-containing-or
title: field_of() silently truncates a model ID containing / or :
status: in-progress
priority: medium
type: fix
created: 2026-07-31
updated: 2026-07-31
depends_on: []
related: [175]
discovered_from: [168]
adrs: []
spec: docs/superpowers/specs/2026-07-31-field-of-silently-truncates-a-model-id-containing-or-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/field-of-silently-truncates-a-model-id-containing-or
claimed_at: 2026-07-31T15:00:18Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-31-field-of-silently-truncates-a-model-id-containing-or-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-31-field-of-silently-truncates-a-model-id-containing-or-design.md) |
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
