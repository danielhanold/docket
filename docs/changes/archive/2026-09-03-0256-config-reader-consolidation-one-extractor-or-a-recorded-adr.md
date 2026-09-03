---
id: 256
slug: config-reader-consolidation-one-extractor-or-a-recorded-adr
title: 'Config-reader consolidation: one extractor or a recorded ADR'
status: 'killed'
priority: medium
type: refactor
created: 2026-08-07
updated: '2026-09-03'
depends_on: []
related: [244, 255]
discovered_from: [179, 165]
adrs: []
spec: docs/superpowers/specs/2026-08-07-config-reader-consolidation-one-extractor-or-a-recorded-adr-design.md
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
| Spec | [2026-08-07-config-reader-consolidation-one-extractor-or-a-recorded-adr-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-config-reader-consolidation-one-extractor-or-a-recorded-adr-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0179 and #0165 (2026-08-07 triage): the same decision — one owner for docket's hand-rolled config value extractors, or a recorded reason they stay separate — asked about two different reader families. One ruling should cover both.

Verified 2026-08-07:

- **The flow-map family (#0179) — blocker expired, duplication grown.** The three readers are still separate: `hd_field` (`scripts/lib/harness-defaults.sh:85-92`, sed, class `[^,}[:space:]]+`), `field_of` (`sync-agents.sh:407-411`, bash `=~`, same class), and `runner-dispatch.sh:111`'s deliberately tolerant sed. The `_raw` companions doubled the duplication (`hd_field_raw` :100-107, `field_of_raw` :420-426 — same idea, two languages). The class has already been wrong in two of them, and ADR-0065's "every pair, present and future, needs the quote leg" is itself an argument for one owner — #0180's very existence shows the rule was applied to one copy only. The stated blocker (0175's `field_of` rewrite) landed 2026-08-01.
- **The flat-scalar family (#0165) — premise stale, residual real.** The documented "identical duplicate" no longer exists: `migrate-to-docket.sh:73-78` is a small sed `yaml_get()` reading 4 keys, while `docket-config.sh` was rewritten into the bash-native `config_*` family (`:105-146`) with quote handling and layer loading. The cited docket-config.sh comment is gone. What remains is a smaller, non-identical duplication whose standalone constraint (migrate runs pre-install) needs re-derivation — the script lives in the docket repo next to `scripts/lib/`, so sourcing is likely available.

## What changes

Settled by the 2026-08-07 auto-groom spec (see Artifacts): **one extractor where readers must agree; a recorded ADR where divergence is deliberate.**

- Consolidate the flow-map pair: line-level `hd_line_field`/`hd_line_field_raw` helpers in `scripts/lib/harness-defaults.sh`; `hd_field*` and sync-agents' `field_of*` become delegating wrappers (sync-agents already sources the lib). The lib's :8-12 non-reuse header is rewritten — its concern is directional and the delegation runs the other way. Byte-identical behavior gated by the existing test pins plus a correspondence probe on the ADR-0065 rows.
- The block-mapping reader (`runner-dispatch.sh`, deliberately tolerant) and the flat-scalar family (`migrate-to-docket.sh`'s `yaml_get` vs `docket-config.sh`'s `config_*`) stay separate: one new ADR records the ruling and each survivor's true constraint — including that #0165's standalone premise was re-derived and found false (migrate already sources `scripts/lib/`; the real reason is contract divergence). Cross-reference comment additions at each surviving copy.

Grooming note: #0165's "standalone pre-install" claim lived in the killed stub, not the code — `yaml_get`'s no-YAML-dependency comment is still true and stays.

## Out of scope

- `docket-frontmatter.sh`'s `field`/`fm_field*` family — owned by #0244 (boundary checked: 0244's census guard patterns do not match `field_of*` call sites).
- Changing any reader's accepted-value semantics (ADR-0065 validation posture stands); the validators' quote legs are #0255's territory (same file, disjoint functions — either merge order is fine).
- Any vendor model allowlist (ADR-0015).

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): already fixed in Go — every reader was Bash; internal/config is the single config resolver.
