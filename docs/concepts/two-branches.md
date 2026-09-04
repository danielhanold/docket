# Two branches and the metadata worktree

## The problem it solves

A team's planning artifacts — what work is queued, why past decisions were made,
what earlier builds taught — usually sit in a wiki or an issue tracker,
disconnected from the code and its history. Put them in the repo instead and the
opposite problem appears: every branch, merge, and diff of the code drags the
planning noise along, and checking out an old commit shows you a stale backlog
that has nothing to do with the code at that revision.

Docket keeps both in one git repository but on two separate branches. Code lives
on the **integration branch** — the branch code lands on, usually `main`.
Planning lives on the **metadata branch** — the `docket` git branch where the
backlog, specs, and decisions are stored, separate from the code. The two never
merge into each other, so a `git log` of your code stays pure code history, and
the backlog can be rewritten freely without ever touching a source file.

The catch is that git gives you one working tree per branch by default, and you
do not want backlog edits landing in the same checkout you are writing code in.
So docket parks a second checkout of the repo — the **metadata worktree**, a
second checkout of the repo at `.docket/`, parked on the metadata branch, so
backlog edits never touch your code checkout — right beside your code.

## The moving parts

```
your repo/
  ├── (your working tree, on the integration branch: your code)
  │           ▲
  │           │  code merges land here
  │           │
  │   [ integration branch: main ] ── never merges ── [ metadata branch: docket ]
  │                                                            ▲
  │        terminal record copied ──────────────────────►     │  backlog / spec / ADR
  │        onto the integration branch                         │  edits commit here
  │                                                            │
  └── .docket/   (metadata worktree, checked out on the metadata branch)
           ├── changes/      one markdown file per change
           ├── adrs/         one file per decision
           ├── BOARD.md      the generated board
           └── learnings/    the loop's memory
```

A **change** — one unit of planned work, roughly one pull request, tracked as one
markdown file — and its **spec**, the design document a change links to, written
before building, live under `.docket/` on the metadata branch. So does every
**ADR**, an architecture decision record: one file per decision, immutable once
accepted; the **board**, the generated overview of every change and its state,
never edited by hand; and the **learnings**, the loop's memory of lessons from
past builds, curated by a human. Your code checkout never sees any of them.

When a change closes out, its terminal record — the archived change file and any
results — reaches the integration branch by copying the file across, not by
merging the metadata branch. That leaves a durable record on `main` for anyone
browsing the code without the metadata worktree, while keeping the two histories
disjoint.

The metadata worktree also fixes where docket resolves the repository root: a
bookkeeping commit runs against `.docket/` explicitly, never against whatever
directory you happen to be standing in when you invoke a command.

## The invariants

- The metadata branch and the integration branch never merge into each other; a
  terminal record reaches the integration branch by copy, not merge.
- Docket-mode is the default; a repository that is not yet set up is refused with
  a migration prompt rather than left half-initialized.
- Backlog, spec, ADR, board, and learnings edits commit to the metadata worktree
  at `.docket/`, never to your code checkout.
- Bookkeeping commits in the metadata worktree skip the repository's shared git
  hooks, so a code-side pre-commit hook never fires on a backlog edit.
- A destructive reset in the shared metadata worktree first requires a
  tracked-files-only clean tree, so a concurrent loop's untracked scratch files
  are never wiped out.
- Docket resolves the repository root from the main worktree, never from the
  caller's current directory.

## Decided in

- [ADR-0001](../adrs/0001-docket-metadata-branch-model.md) — put planning
  metadata on an orphan `docket` branch and publish terminal records by copy
  instead of merging the two branches.
- [ADR-0002](../adrs/0002-docket-mode-default-and-bootstrap.md) — made
  docket-mode the default and set the refuse-and-migrate response for a
  repository that is not yet initialized.
- [ADR-0025](../adrs/0025-docket-worktrees-disable-git-hooks.md) — scoped
  `core.hooksPath` per worktree so bookkeeping commits skip the shared git hooks.
- [ADR-0034](../adrs/0034-repo-root-anchored-to-main-worktree.md) — anchored the
  repository root to the main worktree rather than the caller's current
  directory.
- [ADR-0046](../adrs/0046-cas-reset-hard-shared-worktree-tracked-clean-tree-precondition.md)
  — required a tracked-files-only clean-tree precondition before a
  compare-and-swap reset in the shared metadata worktree.
- [ADR-0089](../adrs/0089-shared-metadata-worktree-contention-survivable-not-impossible.md)
  — made concurrent contention on the shared metadata worktree survivable and
  made a wedged tree halt rather than corrupt state.
- [ADR-0051](../adrs/0051-publish-deferred-marker-not-branch-diff-detector.md) —
  marked a deferred terminal publish with a presence-encoded marker instead of a
  branch-diff detector.
