---
slug: phrase-grep-over-wrapped-prose
hook: "A grep whose pattern can span a line break silently doubles as a line-wrap guard — collapse whitespace before matching, or a pure re-flow reddens asserts about policy that never changed."
topics: [testing, grep, docs]
changes: [218, 231, 224]
created: 2026-08-06
updated: 2026-08-07
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
- 2026-08-07 (#231, PR #170) — The same wrap-sensitivity showed up in the *mutation probe* rather
  than in the assert: `grep -c` over a guarded phrase that happened to wrap reported 0 both before
  and after the mutation, so a mutation that never landed was indistinguishable from a guard that
  correctly survived it — a false proof of robustness, the inverse of #218's false failure. Counts
  were taken through a whitespace-flattened copy instead. The rule is the same one, and it binds
  the verification step as hard as it binds the assert.
- 2026-08-07 (#224, PR #174) — the third hit, and again in the *verification* step: the plan's own
  mutation probe used a `perl` deletion built with a literal-space `quotemeta`, which cannot match a
  phrase that wraps mid-line. The one guarded phrase that happened to wrap was the change's central
  claim — `green if and only if the resolved suite command exits zero` — so the probe reported
  `before=1 after=1`. What caught it was not the probe but the harness around it: the `before/after`
  count check reported `MUTATION DID NOT LAND` instead of letting a never-applied mutation read as a
  guard that correctly survived. Two independent recurrences in the verification step (#231, #224)
  say the mitigation belongs in the probe *template*, not in each author's care.
