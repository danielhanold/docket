# Finalize: Landing changes safely

By the end of this page you will know how an approved change gets from an open pull request into your
mainline and out of your backlog: what the close-out step does in order, how to run it hands-off
across a whole set of changes, what stops it and how to clear the block, and the one branch-protection
setting that lets it merge without you standing over it.

## What finalize does, in order

**Finalize** (the close-out sequence: rebase, retest, merge, archive) is the closing bookend of a
change's (one unit of planned work, roughly one pull request, tracked as one markdown file) life. Once
its pull request is approved or merged, `docket-finalize-change`:

1. rebases the feature branch onto the change's base: the **integration branch** (the branch code
   lands on, usually `main`), or for a stacked change whose parent is still open, the parent's
   branch. A conflict goes to a conflict resolver;
2. re-runs the test suite on the rebased branch (the finalize gate). A red suite goes to a repair
   pass, capped by `finalize.repair_max_attempts`;
3. pushes the rebased head and updates the build evidence in the pull request
   (`docket finalize publish`);
4. merges the pull request with the first merge method the repository permits: rebase, then merge
   commit, then squash;
5. marks the change `done` and archives it on the `docket` branch, refreshing the **board** (the
   generated overview of every change and its state, never edited by hand);
6. cleans up the change's worktree and feature branch.

Nothing is copied onto the integration branch at close-out: the change record, its spec, and its
decisions stay on the `docket` branch.

The retest is the load-bearing step. The build's own tests certified the branch as it stood when the
build finished; the rebase re-checks it against whatever merged in the meantime, so a branch that was
green in isolation but conflicts with newer work cannot land a broken integration. `finalize.gate` is
the on/off switch for that finalize gate — leave it `local` (the default) unless you trust each pull
request's own continuous-integration checks, in which case `off` skips the local rebase-and-retest.
When the rebase changed nothing and the pull request already carries green build evidence for that
exact head, recorded with the same test command, the retest is skipped.
The step-by-step mechanism, and what happens when the rebased suite reds, is
[Finalize as a sequencer](../concepts/finalize-sequencer.md); re-greening after the rebase is covered
in [Proving the build](./proving-the-build.md).

## Closing out hands-free with `/loop`

`docket-finalize-change` ends every run declaring one of four run outcomes — `advanced` (merged one
change and closed it out), `contended` (another writer got there first, nothing merged), `drained`
(nothing eligible in scope), or `halted` (needs a human). implement-next ends with the same four
words, so a single driver keys on both drains the same way: **continue on `advanced`/`contended`,
stop on `drained`/`halted`.** The built-in `/loop` is the recommended driver:

- `/loop docket-finalize-change` — closes out every eligible `implemented` change, **one merge per
  iteration**, stopping on `drained`.
- `/loop docket-finalize-change <id>,<id>,<id>` — bounds the run to that id set. **Naming the ids
  is the authorization:** it merges pull requests `finalize.require_pr_approval` would otherwise
  hold.

Unlike the drainer that only builds changes, this driver **does merge** — that is the whole point of
it, and it is the one place docket itself merges. Every merge still passes the finalize gate, so
`finalize.gate` stays your correctness control. (Draining the build side the same way is
[Building without supervision](./building-without-supervision.md).)

Selection is ordered by **mergeability** rather than priority: already-merged pull requests that
only need closing out come first, then changes whose dependencies are `done`, with GitHub's
`MERGEABLE` signal ahead of conflicting or unknown ones, then the smallest diff, with priority → age →
id as the tiebreak — so each drain lands as many changes as it can before anything stops it.

## When finalize is blocked

A change whose finalize run stops for a human is marked with a `## Finalize blocked` section (dated in
its body) and shows on the board as **finalize blocked — needs you**. The section is a note, not a
lock: later runs still select the change and retry it, so a transient failure (a flaky test, a busy
worktree, a moved base) heals on its own, and closeout removes the section once the change merges. A block that needs a human halts each unscoped finalize run at that change, so fix it, or finalize other changes by naming their ids.
To retry one change specifically, name its id: `/loop docket-finalize-change <id>`.

Some blocks need a human hand before the retry will take. A rebase that conflicts, or a pull request
whose pushed head no longer matches the branch finalize just rebased and retested (an **identity
mismatch**), halts the run rather than merging something it did not verify. You resolve the conflict
or realign the pushed head with your local rebase, then name the id to finalize again. The identity
check's place in the sequence — rebase, verify head, retest, merge — is
[Finalize as a sequencer](../concepts/finalize-sequencer.md).

## The prerequisite: branch protection that permits an unattended merge

An unattended merge only lands if your branch protection permits it. One setting makes it work, and it
routes around a harness quirk worth understanding first.

**The Claude Code auto-mode classifier.** In interactive auto-mode, Claude Code's permission
classifier *soft-denies* capability-granting and merge-adjacent `gh` actions — notably
`gh workflow run`, and `gh pr merge` on an unreviewed pull request (occasionally even a post-merge
`gh pr view`). A soft-deny is a model-side judgment, not a permission lookup: for the `gh` actions
named above a `permissions.allow` entry **cannot** clear it — a claim scoped to those actions as
observed, not a general property of every allow-rule. The behavior is also scoped to the harness
**mode** and **version** it was observed in — headless and interactive diverge, on the same repo, on
the same day — so treat any statement about it as an observation with an expiry date, not a fact.

**The single-maintainer recipe.** Configure branch protection on the integration branch to **require a
pull request** but require **zero** approvals (`required_approving_review_count: 0`; leave
`enforce_admins` off). A solo maintainer cannot approve their own pull request, so a nonzero
requirement is structurally unsatisfiable — but with zero required approvals,
`docket-finalize-change` runs its finalize gate and then merges with a plain `gh pr merge`:
**no `--admin`, no bot, and nothing for the classifier to deny.** Changing the real state of
the external system beats arguing with the guard. Without this setting the drain stops at `halted` on
the first merge.

**Repos that require approvals (human sign-off preserved).** With
`required_approving_review_count >= 1`, a human approves the pull request on GitHub — a co-maintainer,
or the maintainer running finalize when they are an eligible reviewer. That makes `reviewDecision:
APPROVED` satisfy both branch protection and `finalize.require_pr_approval: true`, and finalize merges with
**no `--admin`**. The attended, explicit-id `--admin` path remains the escape hatch when a sole
maintainer deliberately forces past an unsatisfiable required review.

When branch protection also turns on GitHub's **Dismiss stale pull request approvals when new commits
are pushed**, a repair finalize pushes to turn the rebased suite green dismisses that approval, and the
merge waits for a fresh one — that is how a team gets repairs re-reviewed. The setting is off by
default; with it off, the earlier approval stands and the repair merges like any other green change.
