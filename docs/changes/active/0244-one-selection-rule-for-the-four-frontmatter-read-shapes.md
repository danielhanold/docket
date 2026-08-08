---
id: 244
slug: one-selection-rule-for-the-four-frontmatter-read-shapes
title: 'One selection rule for the four frontmatter read shapes'
status: in-progress
priority: medium
type: refactor
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: [235]
discovered_from: [134, 240]
adrs: []
spec: docs/superpowers/specs/2026-08-07-one-selection-rule-for-the-four-frontmatter-read-shapes-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/one-selection-rule-for-the-four-frontmatter-read-shapes
claimed_at: 2026-08-08T08:37:35Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-one-selection-rule-for-the-four-frontmatter-read-shapes-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-one-selection-rule-for-the-four-frontmatter-read-shapes-design.md) |
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

Designed 2026-08-07 (auto-groom; see spec for the census, the rule table, the per-site migration list, and the decision audit trail):

- Re-take the census at build time: every `field()` / `field_raw()` / `fm_field*` / `int_field` / `list_field` call site in `scripts/` and `scripts/lib/`, classified by key optionality, ` #`-in-value risk, and caller decoding/judging needs.
- Record one selection rule as a decision table in the library header (canonical) plus a cross-reference note in `scripts/board-checks.md`; no new ADR — ADR-0057/0058 remain the decision record. This resolves the former open question: the table lives in the header, not a separate per-tier document.
- Migrate the ~10 consumers' optional-key `field()` reads to the anchored tier: `fm_field` for structured values; `fm_field_verbatim` for the free-prose `blocked_by` (its ` #…` is data — `fm_field`'s comment strip would truncate it).
- Keep `fm_field_raw` as a documented orphan (zero production callers; tests pin it).
- Guard: a static (accessor, key) census-allowlist test + an orphan pin for `fm_field_raw` + one absent-key behavioral fixture through `render-change-links.sh`.

## Out of scope

- Inverting `field()`'s whole-file default (ADR-0058's deliberate two-tier split stays; per-site migration only).
- New read shapes (including a fifth "anchored, quote-strip, no comment-strip" tier — `fm_field_verbatim` serves `blocked_by`).
- The `list_field`/`int_field` wrappers and mint-stub's write path.
