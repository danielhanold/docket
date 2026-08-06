---
slug: phrase-grep-over-wrapped-prose
hook: "A grep whose pattern can span a line break silently doubles as a line-wrap guard — collapse whitespace before matching, or a pure re-flow reddens asserts about policy that never changed."
topics: [testing, grep, docs]
changes: [218]
created: 2026-08-06
updated: 2026-08-06
promotion_state: candidate
promoted_to:
---

## Apply
When asserting a *phrase* against hard-wrapped markdown, read a whitespace-collapsed haystack
(collapse runs of whitespace, not only newlines — indented list continuations leave four spaces
behind). Leave deliberately line-anchored asserts alone and record why: a `^`-anchored negative
where the anchor *is* the signal, line-count floors, row-shaped table loops, and `awk` section
extractors whose input must keep newlines or the slice becomes the whole file. Prove the
conversion with a positive control — re-flow the file at several widths and assert the word
stream is identical.

## War story
- 2026-08-06 (#218, PR #162) — Phrase-spanning greps over `fix-loop.md` made every re-flow a test
  failure: the pre-change suite reddened at seven of ten wrap widths tried, up to four asserts at
  once, with messages about policy that had not changed. The re-flow control also caught a defect
  in the first conversion attempt — `tr '\n' ' '` alone left indented continuations four spaces
  apart — which is why the helper collapses whitespace runs.
