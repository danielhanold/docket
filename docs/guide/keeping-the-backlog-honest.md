# Keeping the backlog honest

By the end of this page you can tell whether your backlog still reflects reality — and fix it
when it does not. You will know the difference between the routine sweep that closes out merged
work and the checks that flag what needs a human, what each of those checks means and how to
answer it, how a change that a crashed run left stuck heals itself, and how to recover a run that
stopped and asked for you.

## Status versus the terminal sweep

Two different mechanisms keep the backlog current, and it helps to keep them apart.

The **terminal sweep** is close-out. When a change (one unit of planned work, roughly one pull
request, tracked as one markdown file) has its pull request merged, something has to move it to
`done`, archive it, and refresh the **board** (the generated overview of every change and its
state, never edited by hand). The deliberate way to do that is to close the change out yourself
right after the merge — see [Landing changes safely](./landing-changes.md). But you do not have
to: a periodic status run is the safety net. It sweeps every already-merged pull request to
`done` and regenerates the board on its own, so close-out still happens even when you skip the
deliberate step.

The status run does more than sweep, though. On the same pass it runs a set of **health checks**
(a status-time scan for things a human should look at: stale claims, broken links, stalled
dependencies) — it fixes nothing, it surfaces what you should look at. The sweep keeps the
*shipped* work honest; the health checks keep the *in-flight* work honest. Run status whenever
you want the board refreshed and a read on what needs attention: it is the one command that both
tidies and reports.

## The health checks and what to do about each

A status run reports three kinds of problem. None of them is fixed silently — each is a flag
for you.

- **Stale claims.** A change sitting at `in-progress` long after it was claimed, with no branch
  to show for it, is almost always a run that died. Status flags it and, in the safe case, tells
  you it can heal itself — see [Reclaiming stale claims](#reclaiming-stale-claims) below.
- **Broken links.** A change's `spec`, `plan`, or `results` field points at a file that is not
  there. The remedy is to repoint the field at the real path or, if the artifact was never
  written, clear the field — a dangling pointer is a promise the backlog cannot keep.
- **Stalled dependencies.** A change is waiting on another change (a `depends_on` entry) that is
  itself blocked, killed, or going nowhere, so the waiting change can never start. Status names
  the stall; you break it by merging or reviving the dependency, or by dropping the dependency
  if it is no longer real.

## Reclaiming stale claims

When a run crashes or is killed **before it ever pushes a branch**, its change is left stuck at
`in-progress` — but that one case does not need a human to notice and fix it by hand. Reclaim
closes it automatically, and only for the situation it can close *safely*: an expired **claim
lease** (a timestamp on a claim; when it expires with no branch behind it, the change goes back
to the queue) with no feature branch.

Here is the rule in full. Every **claim** (the moment a change is picked up for building; it
records which branch will carry the work and when it was taken) stamps the time it was taken.
Once that timestamp is older than `reclaim.lease_ttl` (in hours; default `72`, kept at or above
the three-day window status uses to call a claim stale) *and* no `feat/<slug>` branch for the
change exists anywhere status can see, the change is eligible to flip back to `proposed` and
re-enter the queue — the one edge in a change's life that runs backward.

- **Detection is always on.** Status flags every eligible change on each run, whatever your
  reclaim settings are.
- **Healing is opt-in.** With `reclaim.auto: false` (the default) status only recommends it — it
  prints a line like `reclaim: <n> expired-lease change(s) can self-heal — run: docket change
  reclaim` and leaves the change alone. With `reclaim.auto: true` status reclaims every eligible
  change itself on each pass.
- **Run it by hand any time** with `docket change reclaim --id <n> --version <v>` — per change,
  at its exact recorded version — whether or not `reclaim.auto` is set.
- **A change that already has a branch is left to you.** A pushed branch might carry real,
  un-merged work, so reclaim never touches it — the concrete risk is throwing away code nobody
  backed up. It stays flagged instead, for you to resume or discard.

```yaml
reclaim:
  lease_ttl: 72   # hours; kept at or above status's three-day stale-in-progress window
  auto: false     # true => status self-heals eligible claims on every pass
```

## Recovering a halted run

An autonomous run ends by declaring one of four outcomes, and one of them — `halted` — means it
stopped and needs you: an escalation it could not resolve, a design its reconcile step (a check
at build time that the change is still worth doing and its assumptions still hold, before any
code is written) found invalidated, or a broken precondition. The four outcomes and how a loop
keys on them are covered from the build side in
[Building without supervision](./building-without-supervision.md); here the point is what *you*
do when a run halts or simply dies.

- **It halted and said why.** Read the run's final report — it names what it could not get past.
  You make the call it could not: re-design the change if reconcile found the design invalidated
  (a fresh design conversation, see [Designing before building](./designing-before-building.md)),
  or clear the blocker, then let the loop pick it up again.
- **It crashed before pushing a branch.** Nothing to recover — reclaim heals it back to
  `proposed` as above, and the next run rebuilds from scratch.
- **It crashed after pushing a branch.** The branch holds real work, so reclaim leaves it
  flagged. Resume it deliberately by naming its id to the drainer, and tell the run what is
  already built — a bare, unscoped run skips an in-progress change rather than resuming it.

## Supported modes and the bootstrap guard

The backlog can only stay honest if the repo is set up the way the tools expect. docket's
supported default is **docket-mode**: planning metadata lives on the **metadata branch** (the
`docket` git branch where the backlog, specs, and decisions are stored, separate from the code)
via a dedicated worktree, and terminal records stay there unless the repo opts in to publishing
them onto the integration branch (the branch code lands on, usually `main`). Trunk-based and
GitFlow layouts are both supported. **main-mode** — everything on one branch — is a
fully-supported opt-out: pin `metadata_branch: main` (and `integration_branch: main`) to keep
the original single-branch behavior exactly.

An existing single-branch repo moves over with `docket repository migrate`. Until it does, a
**bootstrap guard** refuses to run rather than touching your data — so a half-set-up repo fails
loud instead of scattering metadata into the wrong place. Where the metadata lives, and how the
one-time migration works, is [Where the metadata lives](./where-the-metadata-lives.md).
