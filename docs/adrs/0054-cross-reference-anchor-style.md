---
id: 54
slug: cross-reference-anchor-style
title: Cross-references in maintained source anchor on symbols or quoted clauses, never line numbers — and the guard is deliberately partial
status: Accepted
date: 2026-07-21
supersedes: []
reverses: []
relates_to: [31, 50]
change: 114
---

## Context

A survey of maintained source found 26 comments that cross-referenced other code by naming a
file and a numeric line position. Several had already drifted: one had moved by twenty-five
lines from the position its comment still claimed, and four others had each moved by two lines
the moment an unrelated change inserted two header lines above them.

The underlying problem is not carelessness — it is that a numeric line position is structurally
uncheckable. Nothing in the toolchain can decide whether the line currently at that position
still means what the comment says it means; the comment and the code it describes can drift
apart with no signal anywhere. A cross-reference anchored on a symbol name or a verbatim-quoted
clause is different in kind: it is greppable. If the named symbol or the quoted text is no longer
present at the referenced location, that absence is mechanically visible. That asymmetry —
computable drift detection versus none — is the entire basis for the decision below.

Two categories of file are deliberately out of scope for any guard built on this decision.
Point-in-time records — results files, archived changes, specs, and `Accepted` ADRs — are
excluded because a pointer that was true when authored is a correct historical record; rewriting
it to stay current would falsify what the record is for. `docs/adrs/` carries an additional,
structural reason: an `Accepted` ADR's body is immutable except its status line, so a guard that
demanded a repair inside that body would be demanding something the ADR convention itself
forbids.

## Decision

Cross-references in maintained source anchor on a symbol name or a verbatim-quoted clause —
never on a numeric line position.

The enforcing check, `tests/test_comment_anchor_style.sh`, guards exactly ONE of the three forms
line-number cross-references can take: the explicit-file form, where a filename carrying a
source-file extension is followed by a colon and a digit run. That is the only one of the three
forms measurable without false positives, so it is the only one mechanically enforced.

The other two forms are handled by hand, not by the guard:

- The bare colon-number form (a colon-digit pair with no filename, implicitly same-file) measured
  roughly a 38% false-positive rate against real comment text.
- The prose form ("line N" spelled out in a sentence) measured roughly 60%.

Neither can be tightened into a safe guard. The repo runs a no-allowlist rule — every exclusion
comes from walk scope, never from a per-line exception entry — so there is no escape valve for
the false positives a stricter predicate would throw on legitimate prose. Tightening the bare
form to reduce its false-positive rate was tried and it costs a false negative on real anchors:
the tightened predicate stops seeing genuine same-file line references, which is worse than
leaving it unguarded and documented. These two forms rest on the authoring rule in `AGENTS.md`
plus ordinary review — a human or reviewing agent judgment call, not a computed check.

Scope is walk-based only. The guard walks version-control-tracked files in maintained source;
point-in-time records and `docs/adrs/` are out of scope by virtue of not being in that walk, and
no exception list exists anywhere in the guard for any individual reference.

## Consequences

- **What it enables:** the highest-confidence, zero-false-positive slice of line-number drift —
  the explicit-file form — is caught mechanically and can never silently regress in maintained
  source going forward.
- **The honest cost:** the guard covers roughly half of the 26 surveyed references by count, and
  only about half of the rot actually observed. The worst offender by rot density — the prose
  "line N" form referring to the comment's own file — measured roughly 50% stale, against 3 stale
  out of 24 for everything else combined. That worst-offender form is precisely the one left
  UNGUARDED. This is accepted, not overlooked: the alternative would require a predicate carrying
  a 60% false-positive rate with no exception path available under the no-allowlist rule, and a
  guard with that failure rate would not survive.
- **A guard with a high false-positive rate gets switched off within a month; a clean partial one
  survives.** Choosing the narrower, always-correct check over a broader, sometimes-wrong one is
  the same trade this repo has made before (see ADR-0050 on backstops that compute rather than
  re-enumerate) and the same bound on what source-syntax scanning can reliably see (ADR-0031).
- **Coverage lag:** the guard walks tracked files, so a newly created, unstaged file carrying a
  line-number cross-reference is invisible to it until the file is staged.
- **What is given up:** uniform enforcement across all three anchor forms. Two of the three forms
  rely permanently on human review rather than a computed check, and that reliance is not a
  temporary gap awaiting a future tightening — the false-positive math does not improve with
  effort, only with abandoning the no-allowlist rule, which this decision declines to do.
