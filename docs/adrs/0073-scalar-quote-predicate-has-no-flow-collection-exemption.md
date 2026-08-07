---
id: 73
slug: scalar-quote-predicate-has-no-flow-collection-exemption
title: The needs-quoting predicate answers a scalar-domain question, so it carries no flow-collection exemption
status: Accepted
date: 2026-08-07
supersedes: []
reverses: []
relates_to: [65, 71]
change: 235
---

## Context

Change 0235 made `mint-stub.sh` quote every `title` unconditionally (ADR-[[0071]]) and added a
shared predicate, `docket_scalar_quote_reason` in `scripts/lib/docket-frontmatter.sh`, consumed
**only** by `board-checks.sh`'s `scalar_form_check`.

The groomed design spec gave that predicate a **flow-collection exemption**: a value opening with
`[` and closing with `]` (or `{`/`}`) was exempt, so that `discovered_from: [234]` would not be
flagged by the leading-indicator leg and turned into a string. ADR-[[0071]]'s Consequences record
that the always-quote rule made the exemption unnecessary *on the write path*, and state that it
"stays in the checker's predicate, where an arbitrary hand-authored value may still be a list."

The whole-branch review's fix loop found that last clause wrong.

## Decision

**The needs-quoting predicate answers "is this value well-formed as a bare YAML *scalar*?" — and a
flow collection is not a scalar, so the predicate carries no flow-collection exemption.** It was
removed from `docket_scalar_quote_reason` entirely.

Three arguments drove it, and each stands alone.

1. **It was a false-negative source, not a safety net.** Because the exemption was evaluated first,
   it suppressed *all five* legs rather than only the indicator leg. Measured:
   `docket_scalar_quote_reason '[a title: with colon]'` returned empty, and so did
   `'{a: b} tail}'`. Both were reported by the *pre-change* checker as `colon-space`, so the
   exemption made a previously-reachable finding unreachable — a regression introduced by the very
   change meant to tighten the check.

2. **No ordering rescues it.** Protecting `[234]` from the indicator leg requires sitting above that
   leg; colon-space and trailing-colon sit above *that*. So any placement that protects a flow
   sequence still shadows a leg — and a placement below them protects sequences only, because a flow
   *map*'s `key: value` is a colon-space by construction. The exemption could never protect the
   `{…}` half of its own name.

3. **The domain argument settles it more cleanly than any leg could.** The predicate's question is
   about a bare **scalar**. A flow collection is not a scalar, so a consumer holding a genuine list
   value is asking the wrong function — the answer belongs in the type contract, not in a special
   case. Concretely: the predicate's only consumer, `scalar_form_check`, reads exactly two free-text
   fields (`title` and `blocked_by`), and a sweep of all 236 change files on the metadata branch
   found every real `blocked_by` to be free-text prose, never a `[…]`.

The generalizable rule: **a predicate's exemption list is a symptom that its domain is stated too
loosely. Fix the domain, not the exemption** — and a caller whose value falls outside that domain
must say so at the type level rather than inherit a hidden special case.

## Consequences

- `docket_scalar_quote_reason '[234]'` now reports `indicator` **by design**. That is correct for a
  scalar-domain question and harmless in practice, because nothing passes a list field to it.
- The five syntax legs are each reachable again for flow-collection-shaped input; the false
  negatives measured above are closed.
- **The write path is unaffected.** `mint-stub` quotes `title` only, never `discovered_from`, so no
  YAML sequence is ever stringified. ADR-[[0071]]'s construction guarantee is untouched.
- `AGENTS.md`'s house rule was given an explicit carve-out saying a flow collection is not a scalar
  and must not be quoted. Without it the rule as widened would read as instructing an author to
  quote `depends_on: [3]`.
- The cost: a future consumer wanting a list-tolerant check must state that need itself — a wrapper,
  a distinct predicate, or a type guard at its own call site — rather than inheriting tolerance
  silently. Deliberate: silent inheritance is what shadowed three legs here.
- This reverses one **supporting consequence** of ADR-[[0071]], not its Decision. ADR-[[0071]] is
  Accepted and immutable, so the reversal is recorded there as a dated `## Update` note and stays
  Accepted; see its Update of 2026-08-07.
- ADR-[[0065]] established that a bare-scalar claim built on a raw-vs-consumed comparison needs an
  explicit quote leg. This ADR sharpens the same predicate's *domain*: the legs judge scalars, and
  nothing else is in scope.
