---
slug: cas-re-read-fresh-origin
hook: "A CAS retry must re-derive eligibility from FRESH ORIGIN state — re-reading the working tree you just wrote always reads back your own write and mislabels every real race as a no-op."
topics: [git, concurrency, scripts]
changes: [89]
created: 2026-07-18
updated: 2026-07-18
promotion_state: candidate
promoted_to:
---

## Apply
In a compare-and-swap loop that pushes to a shared branch, the post-rejection re-read must come
from origin, not from your own tree: `fetch` + `reset --hard <remote>/<branch>`, then re-evaluate
the eligibility predicate against that fresh state. Re-reading the working tree you just mutated
returns your own pending write, so the predicate always says "already handled" — every genuine
race is silently reported as skipped while a stale unpushed commit sits on the local branch.

`reset --hard` is only safe here because the loop pushes per item, so the local branch never
carries more than the current item's single unpushed commit. Preserve that invariant if you copy
the pattern: batch several items before pushing and the reset discards real work.

Distinguish the two failure modes in the exit path — a lost race (retry) versus a genuine
fetch/reset failure or exhausted retry budget (`die`). Collapsing them mislabels an infrastructure
failure as benign contention, which is the same class of lie as the stale-read bug.

## War story
- 2026-07-18 (#89, PR #99) — The reclaim script's plan sketched its CAS retry as "re-read the
  change file and re-check eligibility," which read back the just-written `proposed` flip from the
  working tree; every concurrent-claim race would have reported "skipped, already handled" while
  leaving an unpushed commit behind. Corrected during the build to `fetch` + `reset --hard` +
  re-derive from origin, mirroring `docket-implement-next`'s sanctioned claim loop (ADR-0004's
  final-push CAS). This is docket's first `reset --hard` in a CAS path — the precedent everywhere
  else is `pull --rebase` — so it rides ADR-0004's principle plus the `scripts/reclaim-claims.md`
  contract rather than its own ADR; the results file flags promotion to an ADR if the pattern
  recurs.
