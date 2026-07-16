---
slug: frontmatter-edit-anchor
hook: "Anchor a frontmatter-field edit to the first ---…--- block, never a bare column-0 line match."
topics: [yaml, frontmatter, sed]
changes: [25]
created: 2026-06-19
updated: 2026-06-19
promotion_state: candidate
promoted_to:
---

## Apply
Anchor a frontmatter-field edit to the first `---…---` block, never a bare line match — and lock it with a
test where a body `status:` line survives verbatim while the frontmatter field is set.

## War story
- 2026-06-19 (#25, PR #36) — An in-place `sed` that sets a frontmatter field (`status:`/`updated:`/
  `results:`) was unanchored, so it would have rewritten *any* column-0 match — including body prose,
  a live risk for docket's own change/ADR files (which discuss those field names).
