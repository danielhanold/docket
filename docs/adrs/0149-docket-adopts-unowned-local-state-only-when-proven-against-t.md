---
id: 149
slug: 'docket-adopts-unowned-local-state-only-when-proven-against-t'
title: 'Docket adopts unowned local state only when proven against the PR-and-remote-agreed head'
status: 'Accepted'
date: '2026-10-15'
supersedes: []
reverses: []
relates_to: []
change:
---

## Context

`workspace prepare` refuses any branch it did not create ("pre-Go work is never adopted") because a fresh build has nothing to prove the branch against. Finalize therefore cannot run on a machine other than the one that built the change, or after `gh pr checkout` or a hand-made `git worktree add`.

The new `docket workspace checkout` operation adopts an `implemented` change's pushed branch into `.worktrees/<slug>`, because the open PR and the freshly fetched remote branch together prove its head. It fast-forwards a behind local branch only by compare-and-swap, and removes a foreign worktree holding the branch only after fresh removal proofs (linked, attached to the feature ref, at the expected head, clean including untracked files, no operation in progress, not locked, no conflicting manifest) followed by git's clean-only remove.

## Decision

Docket may adopt or replace unowned local state for a change only when that state is proven identical to, or an ancestor of, the change's PR-and-remote-agreed head, and it never removes anything that is not clean.

## Consequences

Finalize works on any machine. Ignored files in a removed foreign worktree are deleted with it. `workspace prepare`'s refuse-unowned rule is unchanged. Adoption must never be extended to states without this proof (for example in-progress changes, where a live run may own the branch).

## Alternatives considered

- Extend `workspace prepare` to adopt: rejected, its safety rules are the opposite (a fresh build has no head to prove against).
- Force-remove or reset foreign state: rejected, it can lose work.
- Ask the human to hand-repair the local state: the status quo, rejected.
