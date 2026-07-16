---
slug: no-checkout-in-shared-worktree
hook: "Review subagents must NOT git checkout in a shared worktree — and after every push, SHA-compare local vs origin."
topics: [git, worktrees, subagents]
changes: [15]
created: 2026-06-17
updated: 2026-06-17
promotion_state: retained
promoted_to:
---

## Apply
Review/inspection subagents must NOT `git checkout` in a shared worktree (use `git show`/`git diff <sha>`,
or a throwaway worktree); after every push the controller SHA-compares local vs origin AND checks
`git symbolic-ref -q HEAD` is the feature branch — never trust the push exit code alone.

## War story
- 2026-06-17 (#15, PR #32) — A read-only review subagent ran `git checkout <sha>` in the SHARED
  feature worktree to inspect a diff, detaching HEAD; the controller's later commits (plan, results)
  landed on the detached HEAD, the branch ref stayed put, and a plain `git push` of the branch
  silently published only the pre-detach tip (the PR was briefly missing files).
