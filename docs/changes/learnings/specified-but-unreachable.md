---
slug: specified-but-unreachable
hook: "Sentinels over prose assert a claim is PRESENT, never that it is REACHABLE — where a contract has a producer and a consumer, anchor one assert on the producer."
topics: [testing, sentinels, review]
changes: [87, 94]
created: 2026-07-19
updated: 2026-07-19
promotion_state: candidate
promoted_to:
---

## Apply
A contract can be fully specified — semantics, clearing rule, board cell, convention entry — and
still ship **inert**, because nothing ever writes it. Every individual sentinel can be a valid,
mutation-provable guard and the *set* still miss this: they all anchor on the **definition**, and a
definition is present whether or not any procedural path produces it. Consumer-side asserts are the
trap, because they pass identically in both worlds — "selection SKIPS a marked change" is green
whether or not anything can ever mark one.

So: when a feature has a **producer** (the step that writes the artifact) and a **consumer** (the
step that reads it), audit the sentinel set for **producer coverage** specifically, and anchor at
least one assert on the paragraph that performs the write — not on the section that defines what
the write means. Ask of any prose deliverable: *which numbered step, in which procedure, emits
this?* If the answer is only "the section that describes it," the feature is decoration.

## War story
- 2026-07-19 (#87, PR #103) — The `## Finalize blocked` marker was specified end to end and never
  written. The gate Flow didn't write it, the abort-and-report set didn't, and *Where the reason
  surfaces* enumerated exactly what happens on an abort (relay in-context, comment on the PR) and
  stopped there. Every marker sentinel passed on the definition alone; the whole-branch review
  caught it, not the suite. Fixed by wiring the write into the surfacing step and adding a sentinel
  anchored on **that paragraph** — the pre-existing consumer assert ("selection SKIPS a marked
  change") passes whether or not anything writes the marker, which is exactly how the gap survived
  to review.
- 2026-07-19 (#94, PR #108) — The same trap one layer down, in the *range expression* of a producer
  assert. A sentinel scoped itself with `awk "/^main\(\)/,/docket_preflight/"` to prove that
  `--digest-only` short-circuits **before** the preflight call. The range closed on the explanatory
  **comment** that merely contains the string `docket_preflight`, so the window never reached the
  code it was meant to cover — the assert was anchored on the producer by name and still never read
  it. It presented as a false BLOCKED (the implementer correctly stopped rather than proceeding past
  an apparently-regressed producer assert). Fixed by replacing the text range with a line-number
  **order** comparison anchored on the executable line `docket_preflight "$SCRIPTS_DIR"`. Lesson:
  a sentinel that names the producer is not yet a sentinel that *reaches* it — when the anchor
  string also occurs in prose, the region is bounded by the comment, not the code.
