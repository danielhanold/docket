---
id: 244
slug: one-selection-rule-for-the-four-frontmatter-read-shapes
title: 'One selection rule for the four frontmatter read shapes'
status: proposed
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [134]
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

Consolidates #0134 and #0240 (2026-08-07 triage): two near-duplicate whole-repo audits of `scripts/lib/docket-frontmatter.sh` readers, filed four changes apart. The library now offers four read shapes with different silent behaviors — whole-file `field()` (docket-frontmatter.sh:168), and the anchored `fm_field` / `fm_field_raw` / `fm_field_verbatim` trio — and no recorded rule says which shape each call site should use.

Verified 2026-08-07:

- `field()` is still whole-file-first-match, so an optional key absent from frontmatter but present in body prose returns prose. Live exposed optional-key `field()` reads: `render-change-links.sh:87-91` (branch, spec, plan, results, pr), `terminal-publish.sh:248,311` (spec, plan, results), `reclaim-claims.sh:70` (branch), `docket-status.sh:1017,1056` (blocked_by, promotion_state), `board-checks.sh:257,409,420-421`, `render-learnings-index.sh:115,120`, `github-mirror.sh:159,203`, `docket-frontmatter.sh:312`.
- The population has partly migrated since #0134 was filed: `board-checks.sh` and `docket-status.sh:631,636` now use `fm_field`/`fm_field_verbatim` with ADR-0057 rationale (`board-checks.md:227-234`) — the audit must re-census, not inherit #0134's list.
- #0240's census (re-verified): `fm_field_raw` has **zero** production callers (definitions/comments only); `fm_field_verbatim` has one (`board-checks.sh:396`); `fm_field` has six consumers — `board-checks.sh` (8 sites), `backfill-change-types.sh` (5), `render-artifact-backlink.sh` (4), `github-mirror.sh` (2), `render-board.sh` (2), `docket-status.sh` (2). #0240's own body listed five consumers; `docket-status.sh` is the sixth.
- 0235's `_fm_scan` edit changed `fm_field`'s quoted-value return across all consumers without per-consumer verification — the concrete incident motivating a recorded rule.
- ADR-0057 names change #0134 by number as its tracked follow-up; this change inherits that pointer.

## What changes

- Re-take the census: every `field()` / `fm_field*` call site in `scripts/` and `scripts/lib/`, classified by key optionality and the silent-behavior difference that matters at that site.
- Record one selection rule (library header + `board-checks.md`-style contract note): when to use each of the four shapes. Default disposition for `fm_field_raw` per #0240: keep it, record that it has no in-repo caller (mirroring how 0235 resolved its finding 7).
- Migrate the call sites the rule says are wrong — the unanchored optional-key `field()` reads above are the primary suspects.
- Guard: a test that pins the rule's load-bearing claims (e.g. the orphan status of `fm_field_raw`, or per-site accessor choices) in a mutation-detectable shape.

## Out of scope

- Inverting `field()`'s whole-file default (ADR-0058's deliberate two-tier split stays; per-site migration only).
- New read shapes.

## Open questions

- Whether the ADR-0058 two-tier split should gain a written per-tier decision table, or the selection rule lives purely in the library header.
