---
slug: no-checkout-in-shared-worktree
hook: "Review subagents must NOT git checkout in a shared worktree — and after every push, SHA-compare local vs origin."
topics: [git, worktrees, subagents]
changes: [15, 94]
created: 2026-06-17
updated: 2026-07-19
promotion_state: retained
promoted_to:
---

## Apply
Review/inspection subagents must NOT `git checkout` in a shared worktree (use `git show`/`git diff <sha>`,
or a throwaway worktree); after every push the controller SHA-compares local vs origin AND checks
`git symbolic-ref -q HEAD` is the feature branch — never trust the push exit code alone.

Ownership is the same hazard seen from the other end: a subagent **resumed** with `SendMessage` is
live again in that shared worktree, so treat the tree as **still owned by it** until you observe its
git-state transition. A controller that resumes an agent and then edits the same tree is racing
**itself** — and the symptom (files moving under you, an unreviewed edit appearing after a green
suite) reads exactly like an external or rogue writer, which sends you hunting for a concurrency bug
that does not exist. Resolve the agent id against this session's own `subagents/` directory before
escalating.

## War story
- 2026-06-17 (#15, PR #32) — A read-only review subagent ran `git checkout <sha>` in the SHARED
  feature worktree to inspect a diff, detaching HEAD; the controller's later commits (plan, results)
  landed on the detached HEAD, the branch ref stayed put, and a plain `git push` of the branch
  silently published only the pre-detach tip (the PR was briefly missing files).
- 2026-07-19 (#94, PR #108) — A fix subagent reported the working tree changing under it, and after
  the final green suite an **unreviewed uncommitted edit** to `skills/docket-implement-next/SKILL.md`
  appeared (it dropped the non-`proposed` skip-reason clause). It was discarded, not adopted — the
  committed state was the reviewed and tested one — and that call was right either way. But the
  writer was first recorded as an external process; it was traced afterwards to **this build's own
  resumed Task 3 implementer**, still live in the shared feature worktree after the controller had
  moved on. Nothing foreign was ever in the branch. The transferable half is the diagnosis, not the
  fix: self-collision presents as an external hazard, so check your own live children first.
