---
slug: printed-remedy-state-validity
hook: "A remedy command you print for a user to run verbatim must be valid in the exact repo state that produced it."
topics: [ux, remedies, migration]
changes: [51]
created: 2026-07-10
updated: 2026-07-10
promotion_state: retained
promoted_to:
---

## Apply
A remedy command you print for a user to run verbatim must be valid in the *exact* repo state that
produced it — branch the printed text on the same condition that gates the underlying write, never
emit one fixed command for divergent states.

## War story
- 2026-07-10 (#51, PR #60) — A printed migration remedy chained `git add .gitignore && git commit`
  unconditionally, but in a repo with stale tracked wrappers and no current opt-in no block is
  written, so the command failed as-run — the remedy was valid only in the state the author
  pictured, not the state that triggered it.
