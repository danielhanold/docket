---
id: 134
slug: audit-field-call-sites-for-frontmatter-anchored-reads
title: Audit field() call sites for frontmatter-anchored reads
status: killed
priority: medium
created: 2026-07-22
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [127]
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
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`scripts/lib/docket-frontmatter.sh`'s `field()` reads the **first match anywhere in the file**, not
the first frontmatter block. For every pre-0127 field that has been safe by accident: frontmatter
sits at the top, so its line always wins over body prose discussing the same key.

It is **not** safe for a key that may be **absent** from frontmatter while present in body prose —
the match then falls through into the body and returns prose as a value. Change 0127 hit this for
real: an untyped change whose body opened a line with `type:` made the board render that prose as
its Type and made `backfill-change-types.sh` refuse to touch the record ("already has type 'this is
prose, not frontmatter'"). It was caught only because the backfill's own anchor fixture happened to
include such a body line.

0127 fixed the `type:` reads by adding `fm_field` (first frontmatter block only) and routing every
`type:` read through it. It deliberately did **not** touch the other call sites, to keep a breaking
config change from also changing read semantics repo-wide.

The residual exposure is every other `field()` call site whose key can be legitimately absent from
frontmatter — `blocked_by:`, `pr:`, `spec:`, `plan:`, `results:`, `branch:`, `issue:` are all
optional, and several are discussed in body prose in docket's own change files.

## What changes

- Audit every `field()` call site against the question "can this key be absent from frontmatter
  while appearing in body prose?", derived from a whole-repo grep rather than a hand-list.
- Decide the posture: route the exposed sites through `fm_field`, or make anchoring the default and
  keep the unanchored reader as the narrow exception.
- Add a guard that keeps new optional-field reads on the anchored path.

## Out of scope

- Re-litigating the `type:` reads, which 0127 already anchored.

## Open questions

- Should `field()` itself become anchored (fixing every site at once, with a compatibility review of
  the callers that legitimately want a whole-file read), or should the anchored reader stay opt-in?

## Triage note (2026-07-26, change 0124)

Confirmed still live, and **ADR-0057 names this change by number as its tracked follow-up**: "The
audit of `field()`'s other existing call sites … is out of scope for change 127 and is tracked as
follow-up change 134." Its Consequences section states the exposure stands until this lands. That
makes this the highest-value item left in the human queue.

The exposed surface, counted across `scripts/`:

| Key | `field()` sites | Optional? |
|---|---|---|
| `spec` | 4 (+1 in a test fixture) | yes |
| `branch` | 3 | yes |
| `issue` | 2 | yes |
| `blocked_by` | 2 | yes |
| `claimed_at` | 2 | yes |
| `promotion_state` | 2 | yes |
| `trivial` | 2 | yes |
| `results`, `plan` | 1 each (+fixtures) | yes |

Always-present keys already safe under `field()`: `status` (15), `id` (8), `title` (7), `slug` (5),
`priority` (5). `fm_field` is currently used for `type` (8 sites) plus one `title`/`id` pair.

Kept in the human queue deliberately: the open question is a repo-wide read-semantics fork
(invert the default vs. keep `fm_field` opt-in), and ADR-0058 already split these readers into two
tiers on purpose — changing the default would touch that decision, which is a call worth making
consciously rather than by a default-biased autonomous groom.

## Why killed

Consolidated into #0244 at the 2026-08-07 backlog triage: near-duplicate of #0240 (same library, same audit verb); #0244 carries the merged census — this change's residual unanchored field() sites plus 0240's fresher three-shape accessor data — and ADR-0057's follow-up pointer.
