---
slug: frontmatter-edit-anchor
hook: "Anchor a frontmatter-field edit to the first ---…--- block, and match trailing space with [[:blank:]]*, never \\s* — it eats the newline."
topics: [yaml, frontmatter, sed, perl]
changes: [25, 206]
created: 2026-06-19
updated: 2026-08-05
promotion_state: promoted
promoted_to: AGENTS.md
---

## Apply
Anchor a frontmatter-field edit to the first `---…---` block, never a bare line match — and lock it with a
test where a body `status:` line survives verbatim while the frontmatter field is set.

Match the trailing run with `[[:blank:]]*`, never `\s*`. In Perl `\s` includes `\n`, and `$` matches
*before* the final newline rather than consuming it — so on an **empty-valued** field
(`results:` with nothing after the colon, the shape every docket field is born in) `s/^results:\s*$/results: X/`
consumes the line terminator and welds the next field onto the value: `results: X trivial: false`.
Read the field back after every write; the exit code is 0 either way.

## War story
- 2026-06-19 (#25, PR #36) — An in-place `sed` that sets a frontmatter field (`status:`/`updated:`/
  `results:`) was unanchored, so it would have rewritten *any* column-0 match — including body prose,
  a live risk for docket's own change/ADR files (which discuss those field names).
- 2026-08-05 (#206, PR #157) — A `docket-implement-next` run set `plan:` with
  `perl -pi -e 's/^plan:\s*$/plan: <path>/'` and silently produced a welded two-field line. It hit
  again later in the same run on `pr:` and `results:` together. Both were caught only because the run
  re-read the file after writing; a run that trusted the exit code would have pushed corrupt
  frontmatter to `origin/docket`. The anchoring half of this finding was already satisfied — the
  regex was correctly anchored and still wrong.
