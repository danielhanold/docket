---
id: 191
slug: enforce-yaml-scalar-wellformedness-in-change-frontmatter
title: Enforce YAML scalar well-formedness in change-file frontmatter
status: proposed
priority: medium
type: fix
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: [190, 138]
discovered_from: [190]
adrs: []
spec:
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

- Field coverage: the four board-rendered fields (`slug`/`priority`/`title`/`type`) vs every
  frontmatter scalar read by docket.
- Exact scalar-form predicate: detecting a colon-space inside an unquoted value without
  false-positives, and detecting a bare boolean keyword, both consistent with
  `field()`/`fm_field()`'s existing quote-unwrap (0138).
- Where the finding reports (a new check-id vs an extended `field-domain` arm), and its
  `BOARD_CHECK_IDS` pinning obligations.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
