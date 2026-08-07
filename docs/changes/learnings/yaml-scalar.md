---
slug: yaml-scalar
hook: "Quote any YAML scalar carrying a colon-space, a trailing colon, a ' #', a leading indicator character, or a boolean keyword — whoever writes it, model or script; a script writing free-text prose quotes unconditionally at the write boundary."
topics: [yaml, frontmatter, config]
changes: [5, 15, 235]
created: 2026-06-10
updated: 2026-08-07
promotion_state: promoted
promoted_to: AGENTS.md
---

## Apply
Quote (or reword around) any YAML scalar carrying a colon-space, a trailing colon, a ` #`, a
leading indicator character, or a boolean keyword (on/off/yes/no/true/false) — whoever writes it,
model or script; today's reader tolerating it is not evidence the value is well-formed (flagged
for #0018/yq). A **script** writing free-text prose into frontmatter quotes unconditionally at the
write boundary rather than predicating on shape, because a conditional is only as good as its
enumeration (ADR-0071; `mint-stub.sh`'s `title` write is the reference).

## War story
- 2026-06-10 / 2026-06-17 (#5 PR #6; #15 PR #32 — merged, one YAML-scalar family) — Two ways a value
  docket writes by hand parses differently once a real YAML loader is in play: an unquoted frontmatter
  scalar cannot contain ": " (colon-space), and a config enum colliding with a YAML 1.1 boolean keyword
  (`gate: off`) is safe under docket's grep/awk reads — it stays the literal string "off" — but would
  load as `false`.
- 2026-08-07 (#235 — the rule was promoted to AGENTS.md and still lost) — `board-checks.sh` had been
  reporting `scalar-form` findings for weeks while `mint-stub.sh` went on minting fresh violations
  on every stub: a detector aimed at the reader cannot stop a writer, and five change files
  accumulated a broken `title:` (one of them, #0173, slipped even the detector, whose colon leg
  only matched a colon-*space* and not a title ending in `:`). The fix was not another instance of
  the rule but a change of kind — quote **unconditionally** at the one write boundary, so validity
  is structural rather than enumerated, and the reader learns the exact inverse (`''` → `'`) so the
  round-trip stays byte-for-byte. The checker keeps its predicate, because it still judges scalars
  it did not write: guarantee and detect are two jobs, not one (ADR-0071).
