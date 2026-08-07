---
id: 256
slug: config-reader-consolidation-one-extractor-or-a-recorded-adr
title: 'Config-reader consolidation: one extractor or a recorded ADR'
status: proposed
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [179]
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

Consolidates #0179 and #0165 (2026-08-07 triage): the same decision — one owner for docket's hand-rolled config value extractors, or a recorded reason they stay separate — asked about two different reader families. One ruling should cover both.

Verified 2026-08-07:

- **The flow-map family (#0179) — blocker expired, duplication grown.** The three readers are still separate: `hd_field` (`scripts/lib/harness-defaults.sh:85-92`, sed, class `[^,}[:space:]]+`), `field_of` (`sync-agents.sh:407-411`, bash `=~`, same class), and `runner-dispatch.sh:111`'s deliberately tolerant sed. The `_raw` companions doubled the duplication (`hd_field_raw` :100-107, `field_of_raw` :420-426 — same idea, two languages). The class has already been wrong in two of them, and ADR-0065's "every pair, present and future, needs the quote leg" is itself an argument for one owner — #0180's very existence shows the rule was applied to one copy only. The stated blocker (0175's `field_of` rewrite) landed 2026-08-01.
- **The flat-scalar family (#0165) — premise stale, residual real.** The documented "identical duplicate" no longer exists: `migrate-to-docket.sh:73-78` is a small sed `yaml_get()` reading 4 keys, while `docket-config.sh` was rewritten into the bash-native `config_*` family (`:105-146`) with quote handling and layer loading. The cited docket-config.sh comment is gone. What remains is a smaller, non-identical duplication whose standalone constraint (migrate runs pre-install) needs re-derivation — the script lives in the docket repo next to `scripts/lib/`, so sourcing is likely available.

## What changes

One deliberate ruling, then its artifact:

- Either factor a shared extractor (likely home: `scripts/lib/`) that the flow-map readers — and possibly `migrate-to-docket.sh`'s `yaml_get` — consume, with byte-identical behavior gates; or
- Record an ADR that the copies stay separate (per-reader constraints: runner-dispatch's tolerance is deliberate; migrate's standalone posture), and add a correspondence guard so the value-class and quote-leg rules cannot drift one-sided again.

Both #0179's and #0165's bodies carried stale premises (see above) — the spec must restate the current shapes, not inherit theirs.

## Out of scope

- `docket-frontmatter.sh`'s `field`/`fm_field*` family — owned by the frontmatter-accessor audit change.
- Changing any reader's accepted-value semantics (ADR-0065 validation posture stands).
