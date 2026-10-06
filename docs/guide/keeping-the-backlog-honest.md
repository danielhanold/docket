# Status: Keeping the backlog honest

By the end of this page you can tell whether your backlog still reflects reality — and fix it
when it does not. You will know the difference between the read-only status report and the
maintenance sweep that closes out merged work, what each health finding means and how to answer
it, how a change that a crashed run left stuck goes back to the queue, and how to recover a run
that stopped and asked for you.

## Status versus the maintenance sweep

Two different commands keep the backlog current, and it helps to keep them apart.

**`docket status` is read-only.** It reports the backlog — what is proposed, ready, in progress,
implemented, or blocked, and the build-ready queue in selection order — plus any health findings.
It never merges, archives, reclaims, or rewrites anything, so you can run it as often as you like.

**`docket maintenance sweep` does the tidying.** When a change (one unit of planned work, roughly
one pull request, tracked as one markdown file) has its pull request merged, something has to move
it to `done` and archive it. The deliberate way is to close the change out yourself right after
the merge — see [Landing changes safely](./landing-changes.md). But you do not have to: the sweep
is the safety net. In one pass it closes out every change whose pull request already merged,
retries any close-out cleanup that did not finish, and reclaims expired claims (below). That
cleanup retry also repoints the backlink in a merged pull request's description when it still
names the change's old `active/` path, even for a change archived long ago; to preview every such
pull request first and repoint them all at once, run `docket repository repair --pr-backlinks`. Every
change it touches re-renders the **board** (the generated overview of every change and its state,
never edited by hand) in the same commit.

The `docket-status` skill runs the read alone when you just want to see the backlog, and runs the
sweep first when you ask it to refresh or clean up. implement-next runs its own narrower startup
pass, `docket maintenance preflight`, before it picks a change.

## The health findings and what to do about each

A status run reports structural problems in the backlog. None of them is fixed silently — each is
a flag for you.

- **`artifact-missing`.** A change's `spec`, `plan`, or `results` field points at a file that is
  not there. Repoint the field at the real path or, if the artifact was never written, clear the
  field — a dangling pointer is a promise the backlog cannot keep.
- **`change-reference-dangling`.** A change names another change (in `depends_on`, for example)
  or an ADR that does not exist. Fix or drop the reference.
- **`change-dependency-cycle`.** Two or more changes depend on each other, so none of them can
  ever start. Break the cycle by dropping the dependency that is not real.
- **`branch-malformed`.** A change's recorded `branch` is not a valid git branch name. Relink the
  change to its real branch with `docket change relink`.
- **Configuration and parse problems.** A configuration file docket cannot accept, or a change
  record it cannot read, is reported with the file and the remedy. `docket diagnostic config
  --repo-dir .` shows the resolved configuration and anything that blocks writes.

A dependency only counts as satisfied once the change it names reaches `done`, so a change waiting
on unmerged work simply stays out of the build-ready queue; status shows which dependencies are
still unmet.

## Reclaiming expired claims

When a run crashes or is killed **before it ever pushes a branch**, its change is left stuck at
`in-progress`. Reclaim returns it to the queue, and only in the situation it can handle *safely*.

Every **claim** (the moment a change is picked up for building; it records which branch will
carry the work and when it was taken) stamps the time it was taken. A change is eligible for
reclaim only when all three hold:

- its claim lease has expired: the stamp is older than `reclaim.lease_ttl` hours (default `72`);
- no feature branch for it exists, locally or on the remote;
- no feature workspace for it exists.

An eligible change goes back to `proposed` and re-enters the queue — the one edge in a change's
life that runs backward. Anything docket cannot confirm (a probe that fails, a branch it cannot
see clearly) leaves the change alone.

- **Automatic reclaim is opt-in.** With `reclaim.auto: false` (the default) the maintenance sweep
  leaves every eligible change alone and reports it as skipped with the reason
  `reclaim-auto-disabled`. With `reclaim.auto: true` the sweep reclaims every eligible change
  itself.
- **Run it by hand any time** with `docket change reclaim --id <id> --revision <revision>` — per
  change, at its exact recorded revision — whatever `reclaim.auto` says.
- **A change that already has a branch is left to you.** A pushed branch might carry real,
  un-merged work, so reclaim never touches it — the concrete risk is throwing away code nobody
  backed up.

```yaml
reclaim:
  lease_ttl: 72   # hours before an unattended claim counts as expired
  auto: false     # true => the maintenance sweep reclaims eligible changes itself
```

## Recovering a halted run

A build run ends by declaring one of four outcomes, and one of them — `halted` — means it stopped
and needs you: an escalation it could not resolve, a design its reconcile step (a check at build
time that the change is still worth doing and its assumptions still hold, before any code is
written) found invalidated, or a broken precondition. The four outcomes and how a driver keys on
them are covered from the build side in
[Building without supervision](./building-without-supervision.md); here the point is what *you*
do when a run halts or simply dies.

A halted change stays `in-progress`, and the board shows it as **run halted — needs you**.

- **It halted and said why.** Read the run's final report — it names what it could not get past.
  Make the call it could not: re-design the change if reconcile found the design invalidated (a
  fresh design conversation, see [Designing before building](./designing-before-building.md)), or
  clear the blocker. Then recover the change with `docket change resume-halted`, which checks the
  workspace, refreshes the claim, and removes the halt marker, and start the next run on it with
  `docket run start implement-next --resume <id>`.
- **It crashed before pushing a branch.** Nothing to recover — once the lease expires, reclaim
  returns it to `proposed` as above, and the next run rebuilds from scratch.
- **It crashed after pushing a branch.** The branch holds real work, so reclaim leaves it alone.
  Resume it deliberately with `docket run start implement-next --resume <id>` and name the id to
  implement-next — a bare, unscoped run skips an in-progress change rather than resuming it.

## Before any of this: a set-up repository

Every workflow starts with a startup check that refuses to run against a repository that is not
set up, rather than scattering metadata into the wrong place. A repository that has never used
docket runs `docket repository init` once; one still on the old single-branch layout runs
`docket repository migrate`. Where the metadata lives, and what those commands do, is
[Where the metadata lives](./where-the-metadata-lives.md).
