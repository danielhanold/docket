---
slug: repo-wide-budget-guard-counts-unreachable-sites
hook: "A repo-wide budget guard counts by shape, so a narrow, unreachable call site in an unrelated file still spends the budget — and inside a bounded fix loop that one collision reverts every good fix beside it."
topics: [testing, guards, review]
changes: [258]
created: 2026-08-09
updated: 2026-08-09
promotion_state: candidate
promoted_to:
---

## Apply
A guard that budgets **how many sites of some shape exist across the repo** — N tree-walk sites, N
`eval` sites, N direct-`git`-in-tests sites — is keyed on shape and not on reachability, which is
the whole reason it works ([[byte-pattern-guard-matches-a-spelling]] is the failure of the
alternative). The cost of that design is that **any file in the corpus can breach it**, including
one whose author has never read the guard. A collision looks like a red test in a file your change
does not touch, with a diagnostic about a budget you have never heard of.

Before adding a call of a *shape a guard counts* — a tree walk, a subprocess class, a scan — grep
the suite for a budget on that shape, not just for tests over the file you are editing. And when
you hit one, the question is not "is my site dangerous" but **"is my site countable"**: a narrow
pathspec that provably cannot reach the guarded tree is still one more site by the classifier's
rules. Widening the classifier to see reachability is real, beyond-the-branch work — it needs its
own change, because a reachability judgement is exactly the kind of thing a shape-keyed guard
declines to make on purpose.

Second-order, and the part that actually costs: inside a **bounded fix loop** (docket's review fix
loop is two suite runs, revert-all-or-nothing), a guard collision on the *last* fix reverts every
fix beside it. Three mutation-proved, independently correct fixes were rolled back because a fourth
tripped an unrelated budget. So **order fix commits by collision risk, cheapest and most local
first**, and treat "this fix adds a new call site anywhere" as the signal to land it last or to
split it out — not because it is wrong, but because it is the one that can take the others with it.
When the revert fires, name the four commits and their SHAs in the results file: reverted work that
was proved correct is cherry-pickable, and it is only findable if the record says so.

## War story
- 2026-08-09 (#258, PR #189) — A review fix rewrote a marker-collection loop to use
  `git -C "$REPO" ls-files 'tests/test_docket_config*.sh'` so the family glob would admit only
  git-tracked files. That call is a **tree-walk site**, and a different change's guard,
  `tests/test_skip_allowlist_invisibility.sh`, budgets how many walk sites in `tests/` and
  `scripts/` can reach the results tree: `found 3 … budget 2`. The pathspec is narrow and cannot
  reach `docs/results/` at all, so this is a classifier gap rather than a hazard — but establishing
  that is beyond-the-branch work and the fix loop's gate is bounded. All four fix commits were
  reverted together (`2fa1c162`, `9dad467d`, `7d6e914b`, `0982b266` — the last being the one that
  broke it), returning the branch to a tree byte-identical to the one that entered the loop, and
  the SHAs were recorded in the results file for later cherry-picking. Related:
  [[guards-are-code]], [[enumerated-floor]].
