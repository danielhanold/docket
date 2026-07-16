---
slug: yaml-scalar
hook: "Quote any hand-authored scalar carrying a colon-space or a boolean keyword — today's grep/awk reader tolerating it is not evidence it is well-formed."
topics: [yaml, frontmatter, config]
changes: [5, 15]
created: 2026-06-10
updated: 2026-06-17
promotion_state: candidate
promoted_to:
---

## Apply
Quote (or reword around) any hand-authored scalar carrying a colon-space or a boolean keyword
(on/off/yes/no/true/false); today's reader tolerating it is not evidence the value is
well-formed (flagged for #0018/yq).

## War story
- 2026-06-10 / 2026-06-17 (#5 PR #6; #15 PR #32 — merged, one YAML-scalar family) — Two ways a value
  docket writes by hand parses differently once a real YAML loader is in play: an unquoted frontmatter
  scalar cannot contain ": " (colon-space), and a config enum colliding with a YAML 1.1 boolean keyword
  (`gate: off`) is safe under docket's grep/awk reads — it stays the literal string "off" — but would
  load as `false`.
