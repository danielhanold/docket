---
slug: identity-match-relaxed-to-prefix-is-vacuous
hook: "Relaxing an identity comparison from equality to a prefix test admits the empty string, which prefixes everything — the guard then passes for every input."
topics: [guards, validation, identity]
changes: [307]
created: 2026-08-14
updated: 2026-08-14
promotion_state: candidate
promoted_to:
---

## Apply
An identity check (does this record's declared `id`/`slug` match the filename it lives under?) is
an **equality** predicate. Every relaxation of it toward "starts with" / "contains" / "matches the
first segment" silently admits the degenerate operand: the empty slug is a prefix of every
filename, so the assert goes permanently green and stops distinguishing anything. If a real case
motivates the relaxation, the fix is a *narrower explicit rule* for that case plus a non-empty
precondition — never a loosened comparison. Mutation-test the relaxed form specifically with the
empty/degenerate operand; a prefix guard that never reddens on the empty string is decoration.

## War story
- 2026-08-14 (#307, PR #208) — `identityMatchesFilename` was relaxed to a slug-*prefix* comparison
  to accommodate a filename shape, which admitted a vacuous match for a record with an empty slug:
  the check passed for any filename. Review raised it as a blocker; the function was restored to
  strict equality between the record's `id`/`slug` and its filename.
