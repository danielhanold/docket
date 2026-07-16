---
slug: dormant-code-live-mid-branch
hook: "When a premise is 'X is dead today', re-probe X's liveness at the task that flips its precondition, not against the pre-branch tree."
topics: [process, spec, reconcile]
changes: [75]
created: 2026-07-15
updated: 2026-07-15
promotion_state: retained
promoted_to:
---

## Apply
When a premise is "X is dead today," re-probe X's liveness at the task that flips its precondition, not
against the pre-branch tree; a `return`/early-exit in a block you're activating is abandoning
every close-out step downstream of it until proven otherwise.

## War story
- 2026-07-15 (#75, PR #84) — A spec that frames code as "dead/dormant today, comes alive only once
  this change lands" can already be LIVE mid-branch, because an earlier task in the same branch
  flipped its precondition. Here Task 3 made `METADATA_WORKTREE` absolute, so `docket-status.sh`'s
  artifacts-refresh block (which just reads `mw="${METADATA_WORKTREE:-.docket}"`) was live from
  commit `c42ae5b` onward — and its `return 0` on a failed push had been silently abandoning
  `terminal-publish` AND `cleanup` the whole time, not merely from the final commit.
