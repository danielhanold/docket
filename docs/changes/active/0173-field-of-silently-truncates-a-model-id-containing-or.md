---
id: 173
slug: field-of-silently-truncates-a-model-id-containing-or
title: field_of() silently truncates a model ID containing / or :
status: proposed
priority: medium
type: fix
created: 2026-07-31
updated: 2026-07-31
depends_on: []
related: []
discovered_from: [168]
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
gets typed by hand.

## What changes

- Widen `field_of()`'s value class so a model or effort value is consumed whole, or make an
  unconsumable value a loud generation-time error rather than a silent truncation.
- Regression coverage across all three user layers (machine-local, repo-committed, global) and the
  `agents.default` vs `agents.<harness>` merge, since the truncation is silent and only a
  value-level assert catches it.
- Check whether the same class appears in any other config reader and fix or report each.

## Out of scope

- Any vendor model allowlist or availability lookup (ADR-0015 forbids it).
- The shipped-defaults reader in `scripts/lib/harness-defaults.sh`, already fixed by change 0168.
