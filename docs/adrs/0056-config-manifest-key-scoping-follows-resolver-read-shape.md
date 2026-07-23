---
id: 56
slug: config-manifest-key-scoping-follows-resolver-read-shape
title: Config-manifest keys are qualified by their ancestor path; the duplicate-name floor is derived from the resolver's read shape
status: Accepted
date: 2026-07-22
supersedes: []
reverses: []
relates_to: []
change: 127
---

## Context

`.docket.yml` mixes flat top-level keys (`metadata_branch`, `auto_capture`, …) with nested
block-scoped keys (`learnings.enabled`, `reclaim.enabled`, `skills.build`, …). A naive rule —
"every key name in the file must be globally unique" — would forbid two blocks from ever sharing a
bare leaf name, even when nothing reads them ambiguously. But the resolver does not read every key
the same way: some keys are read flat (`lcl <leaf>` / `yaml_get "$CFG" <leaf>`, a bare-name lookup
that takes the first match via `head -n1`), and some are read scoped, within their own
`yaml_block_body` extracted for that block only.

`typed-changes-selective-auto-capture` (change 127) introduced `auto_capture.enabled` alongside the
pre-existing `learnings.enabled`, both block-scoped leaves sharing the bare name `enabled`. Whether
this is a collision depends entirely on how each is read, not on the file's flat text.

## Decision

Config-manifest keys are qualified by their ancestor path, and the uniqueness floor a test or
reviewer enforces must be derived from the resolver's **read shape**, not from bare key-name
text:

- A **block-scoped leaf** (e.g. `learnings.enabled`, `reclaim.enabled`, `skills.build`,
  `auto_capture.enabled`) is read within its own `yaml_block_body` extraction for that block —
  it may legitimately share a bare leaf name with a leaf in a different block. `learnings.enabled`
  and `auto_capture.enabled` coexisting is correct, not a latent bug.
- A **flat-read key** — every top-level key, plus the `finalize.*` leaves (which are read via
  bare `lcl <leaf>` / `yaml_get "$CFG" <leaf>` despite nesting under `finalize:` in the file) —
  must be globally unique across the whole manifest, because `yaml_get`'s `head -n1` resolves the
  first textual match regardless of which block it sits under; a second key of the same bare name
  anywhere earlier in the file would silently shadow it.

The guard enforcing this lives in `tests/test_docket_example_yml.sh`: it asserts uniqueness only
over the flat-read key set, and treats block-scoped leaves as scoped by ancestor path.

## Consequences

New block-scoped config sections (like `auto_capture:`) can freely reuse leaf names already used
elsewhere (`enabled`, `cap`, …) without tripping a false-positive duplicate-key guard. The guard
still catches the real hazard — two flat-read keys, or a `finalize.*` leaf colliding with another
flat-read key — because those genuinely resolve through the same `head -n1` bare lookup. The
tradeoff is that the uniqueness check is no longer a single flat scan; it must classify each key by
how the resolver actually reads it, and that classification needs revisiting whenever a new
top-level block introduces leaves read via `lcl`/`yaml_get` rather than `yaml_block_body`.
