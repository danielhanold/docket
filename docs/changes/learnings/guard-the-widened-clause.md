---
slug: guard-the-widened-clause
hook: "When a clause was WIDENED during design, re-derive the guard from the widened text — a guard written against the original shape leaves the added part free, and every assert stays green when it is deleted."
topics: [testing, guards, spec]
changes: [249]
created: 2026-08-08
updated: 2026-08-08
promotion_state: candidate
promoted_to:
---

## Apply
A spec's `## Assumptions` (or its critic/review record) tells you which clauses grew a part *after*
the first draft — and that added part is exactly the one the guard tends to miss, because the guard
was drafted alongside the original. Enumerate the clause's parts from the final text, not from
memory of the design, and mutation-test each part separately: delete only the added conjunct and
confirm something reddens. The added part is usually the one a design gate insisted on, i.e. the
hazard someone already identified — so an unguarded one silently reinstates a known failure mode.

## War story
- 2026-08-08 (#249, PR #178) — spec assumption A1 records that the inline consequence was widened at
  the auto-groom critic gate from never-yield alone to never-yield **+ finite observation +**
  fail-closed, precisely because "observe by blocking" without a bound turns a measured yield-and-stall
  into an unbounded silent block. The guard block pinned never-yield and fail-closed; deleting
  `keep the observation **finite**` reinstated the exact hazard the critic gate existed to prevent
  with every change-0249 assert still green. Review finding 3; fixed by one more single-gap assert
  (`a7f64d20`), mutation-proven to be the only assert that reddens on that deletion.
