---
id: 191
slug: enforce-yaml-scalar-wellformedness-in-change-frontmatter
title: Enforce YAML scalar well-formedness in change-file frontmatter
status: done
priority: medium
type: fix
created: 2026-08-01
updated: 2026-08-03
depends_on: []
related: [190, 138]
discovered_from: [190]
adrs: []
spec: docs/superpowers/specs/2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter-design.md
plan: docs/superpowers/plans/2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter.md
results: docs/results/2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter-results.md
trivial: false
auto_groomable: true
branch: feat/enforce-yaml-scalar-wellformedness-in-change-frontmatter
pr: https://github.com/danielhanold/docket/pull/151
blocked_by:
reconciled: true
claimed_at: 
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter-design.md) |
| Plan | [2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter.md) |
| Results | [2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-01-enforce-yaml-scalar-wellformedness-in-change-frontmatter-results.md) |
<!-- docket:artifacts:end -->

## Why

AGENTS.md's house rule requires quoting any hand-authored YAML scalar that carries a colon-space
or a bare boolean keyword — but it is prose, not a guard. Change 0190's `title:` shipped as an
**unquoted** scalar containing a colon-space; the line-based readers (ADR-0062) tolerated it and
the board rendered it correctly, and nothing reddened — the defect surfaced only in review.
AGENTS.md states the risk bluntly: *"Today's grep/awk reader tolerating it is not evidence it is
well-formed."*

The board's `field-domain` check already guards the one shape that breaks its own output — a
`title` containing `|` (markdown column injection) — but no check enforces scalar
well-formedness. A malformed scalar therefore survives silently until a strict YAML consumer or a
human happens to notice. This change closes that enforcement gap.

## What changes

Extend `board-checks.sh`'s `field-domain` check (or a sibling check in the same family) with a
**scalar-form arm** that flags an unquoted frontmatter scalar carrying a colon-space or a bare
boolean keyword (`on`/`off`/`yes`/`no`/`true`/`false`), following the same shape-not-spelling
discipline as the existing pipe check — the field set and predicate derived from what docket
actually reads, not an enumerated bad-value list. A correctly quoted scalar (per the 0138
quote-unwrap convention) must NOT be flagged.

## Out of scope

- Rewriting the line-based readers or adopting an external YAML parser (ADR-0062 stands).
- Changing the board's rendering or the reader unwrap behavior.
- Auto-fixing flagged files.

## Open questions

Resolved during autonomous grooming (see the linked spec's Assumptions for the full audit trail):

- **Field coverage** — a new `scalar-form` check scans `title` and `blocked_by` only: the sole
  free-text string scalars that docket reads and that are not already gated by a shape/domain check.
  The natively-boolean fields (`trivial`, `auto_groomable`, `reconciled`) are deliberately NOT scanned —
  a bare `true`/`false` there is correct, well-formed YAML.
- **Scalar-form predicate** — three legs over the raw token (`field_raw` for `title`, a new
  `fm_field_raw` for the optional `blocked_by:` key — anchored per ADR-0057): skip quoted/empty,
  flag an unquoted `: ` (colon-space), flag an unquoted bare YAML boolean keyword
  (`on`/`off`/`yes`/`no`/`true`/`false`, case-insensitive).
- **Finding location** — a new sibling check-id `scalar-form` beside `field-domain` (not an arm of it),
  with its `BOARD_CHECK_IDS` pinned across all four surfaces (check-id array, `--help` header,
  `board-checks.md`, `docket-status.md` report row) plus the tests.

## Reconcile log

### 2026-08-02

Claimed and reconciled against merged `origin/main` — current tip `868e55c2` (ADR-0036 publish).
**Design holds — no scope change.**

- Spec baseline verified against current code: `BOARD_CHECK_IDS` is 13 (spec's 13 → 14 pin is
  accurate), `field-domain` lives in `scripts/board-checks.sh`'s per-file walk, `field_raw` /
  `fm_field` / `_docket_unwrap_quotes` are present, and **no `fm_field_raw` exists yet** — the
  read-side helper the spec's anchored `blocked_by` read requires is a real, still-pending add.
  No target file (`board-checks.sh`, `lib/docket-frontmatter.sh`, `board-checks.md`,
  `docket-status.md`, `test_board_checks.sh`) changed on `origin/main` since 2026-08-01 (verified
  via `git log origin/main --since`), so the physical baseline matches the spec byte-for-byte.
- Spec assumption 6 re-verified: change 0121's active file carries the unquoted colon-space title
  (`The manifest's elsewhere: check proves a word occurrence, not a real config read`) — the
  check's expected first warn-only finding on real history, not a regression.
- Related change 0190 (in-progress) is halted at Step 5 on a build-dispatch availability issue;
  unrelated to 0191's scope, and this session registers the `docket-build-*` profile agents the
  halt remedy named. Related change 0138 (archived) — the quote-unwrap convention 0191's quote leg
  relies on — is intact.
- No follow-up surfaced worth minting: the 0121 finding is expected behavior of this very check,
  already tracked, not new work.
