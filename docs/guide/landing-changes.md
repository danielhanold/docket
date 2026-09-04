# Landing changes safely

By the end of this page you will know how an approved change gets from an open pull request into your
mainline and out of your backlog: what the close-out step does in order, how to run it hands-off
across a whole set of changes, what stops it and how to clear the block, and the one branch-protection
setting that lets it merge without you standing over it.

## What finalize does, in order

**Finalize** (the close-out sequence: rebase onto the integration branch, retest, merge, archive) is
the closing bookend of a change's (one unit of planned work, roughly one pull request, tracked as one
markdown file) life. Once its pull request is approved or merged, `docket-finalize-change`:

1. rebases the feature branch onto the **integration branch** (the branch code lands on, usually
   `main`),
2. re-runs the test suite on the rebased branch (the merge gate),
3. merges the pull request,
4. archives the change to `done` and refreshes the **board** (the generated overview of every change
   and its state, never edited by hand).

The retest is the load-bearing step. The build's own tests certified the branch as it stood when the
build finished; the rebase re-checks it against whatever merged in the meantime, so a branch that was
green in isolation but conflicts with newer work cannot land a broken integration. `finalize.gate` is
the on/off switch for that merge gate — leave it `local` (the default) unless you trust each pull
request's own continuous-integration checks, in which case `off` skips the local rebase-and-retest.
The step-by-step mechanism, and what happens when the rebased suite reds, is
[Finalize as a sequencer](../concepts/finalize-sequencer.md); re-greening after the rebase is covered
in [Proving the build](./proving-the-build.md).

## Selective publish on close-out

On a **terminal transition** — a change reaching `done` (its pull request merged) or `killed`
(abandoned) — the driving skill archives that change on the **metadata branch** (the `docket` git
branch where the backlog, specs, and decisions are stored, separate from the code). A repo that opts
in with `terminal_publish: true` *also* copies that change's terminal records — the archived change
file, its spec if any, and the `Accepted` ADRs (an architecture decision record: one file per
decision, immutable once accepted) from its manifest — onto the integration branch in one dedicated
commit, sourced from the metadata branch.

That copy is **selective**: a file copy, never a branch merge, so none of the planning churn comes
with it, and the live board stays on the metadata branch and is never published. The result for a repo
that opts in is a code history that reads as code plus a clean trail of closed-out changes, while the
working backlog churns entirely on the metadata branch. Where `terminal_publish` may be set and why to
opt in deliberately is [Where the metadata lives](./where-the-metadata-lives.md).

## Closing out hands-free with `/loop`

`docket-finalize-change` ends every run declaring one of four run outcomes — `advanced` (merged one
change and closed it out), `contended` (another writer got there first, nothing merged), `drained`
(nothing eligible in scope), or `halted` (needs a human). A single driver keys on both halves of the
daily loop the same way: **continue on `advanced`/`contended`, stop on `drained`/`halted`.** The
built-in `/loop` is the recommended driver:

- `/loop docket-finalize-change` — closes out every eligible `implemented` change, **one merge per
  iteration**, stopping on `drained`.
- `/loop docket-finalize-change 90,92,94` — bounds the run to that id set. **Naming the ids is the
  authorization** the *attended* multi-candidate prompt would otherwise have collected. Neither drain
  prompts (one merge per iteration is never a batch); what naming the ids adds is that it merges pull
  requests `require_pr_approval` would otherwise hold, and retries a change already marked
  `## Finalize blocked`.

Unlike the drainer that only builds changes, this driver **does merge** — that is the whole point of
it, and it is the one place docket itself merges. Every merge still passes the rebase-retest gate, so
`finalize.gate` stays your correctness control. (Draining the build side with the same loop is
[Building without supervision](./building-without-supervision.md).)

Selection is ordered by **mergeability** rather than priority: `depends_on` order first (a hard
constraint), then GitHub's own `mergeable` signal, then the smallest diff, with priority → age → id as
the tiebreak — so each drain lands as many changes as it can before anything stops it.

## When finalize is blocked

A change whose merge gate fails is marked with a `## Finalize blocked` section (dated in its body),
shows on the board as **finalize blocked — needs you**, and is skipped by later *unscoped* runs until
a successful finalize clears it automatically. **Name its id to retry it:**
`/loop docket-finalize-change 90` re-attempts change 90 specifically, and naming the id is exactly
what re-runs a change already sitting under `## Finalize blocked`.

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
the same day — so treat any statement about it as an observation with an expiry date, not a fact. This
is precisely why docket's earlier bot-approval design failed: its very first step was a
`gh workflow run` dispatch, which is exactly what gets denied. That subsystem has since been retired.

**The single-maintainer recipe.** Configure branch protection on the integration branch to **require a
pull request** but require **zero** approvals (`required_approving_review_count: 0`; leave
`enforce_admins` off). A solo maintainer cannot approve their own pull request, so a nonzero
requirement is structurally unsatisfiable — but with zero required approvals,
`docket-finalize-change` runs its rebase-retest gate and then merges via a plain `gh pr merge
--rebase`: **no `--admin`, no bot, and nothing for the classifier to deny.** Changing the real state of
the external system beats arguing with the guard. Without this setting the drain stops at `halted` on
the first merge.

**Repos that require approvals (human sign-off preserved).** With
`required_approving_review_count >= 1`, a human approves the pull request on GitHub — a co-maintainer,
or the maintainer running finalize when they are an eligible reviewer. That makes `reviewDecision:
APPROVED` satisfy both branch protection and `require_pr_approval: true`, and finalize merges with
**no `--admin`**. The attended, explicit-id `--admin` path remains the escape hatch when a sole
maintainer deliberately forces past an unsatisfiable required review.
