---
slug: cas-re-read-fresh-origin
hook: "A CAS retry must re-derive eligibility from FRESH ORIGIN state — re-reading the working tree you just wrote always reads back your own write and mislabels every real race as a no-op."
topics: [git, concurrency, scripts]
changes: [89, 91]
created: 2026-07-18
updated: 2026-07-19
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

**Gate the reset on a clean tree — counting TRACKED files only.** In a worktree other agents share,
`reset --hard` destroys their uncommitted work, not just yours, and the script still reports
success. So the reset needs a clean-tree precondition — but gating on plain
`git status --porcelain` counts **untracked** files too, so a stray `.DS_Store` hard-fails the
operation on exactly the contended path the feature exists for. Scope the check to tracked
modifications (`git status --porcelain --untracked-files=no`, or `git diff --quiet HEAD`). Both
sides are real bugs, so a one-sided test accepts either the over-broad gate or no gate at all —
prove it **two-sided** (dirty-tracked must fail, untracked-only must pass). Graduated to ADR-0046.

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
- 2026-07-19 (#91, PR #104) — **The predicted recurrence arrived.** `mint-stub.sh` is the second
  user of the fresh-origin CAS reset, so the pattern graduated to **ADR-0046** as flagged above. It
  also hit the hazard the first user did not: `mint-stub.sh` runs inside the `.docket` metadata
  worktree that concurrent agents share, and review reproduced **real data loss** — an unrelated
  agent's uncommitted change file wiped, with the script still exiting 0. Then it reproduced the
  *over*-correction: the first fix gated on plain `git status --porcelain`, so an untracked
  `.DS_Store` hard-failed the mint on the contended path the feature is for. The two-sided proof is
  what pinned the narrow gate; a one-sided test would have blessed either error.
