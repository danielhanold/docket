---
id: 89
slug: shared-metadata-worktree-contention-survivable-not-impossible
title: "Shared-metadata-worktree contention is made survivable, not impossible — and a wedged tree halts"
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: [46]
change: 247
---

## Context

Docket's `.docket` metadata worktree is shared by every agent that touches the planning surface:
interactive sessions overlap autonomous loops routinely. Two defects were observed live in that
tree.

**One — a normal dirty tree hard-failed another agent's Step 0.** `scripts/lib/docket-preflight.sh`
synced with a bare `fetch && pull --rebase || return 1`. A concurrent agent's working tree is dirty
for the entire multi-tool-call window between its first edit and its commit — that is the normal
steady state, not an anomaly — and a rebase into a dirty tree fails. So one agent's ordinary
in-progress edit aborted a second agent's preflight, which is the blocking first step of every
docket skill.

**Two — pathspec-less `git commit` swept up another agent's staged work.** Commits issued in the
shared tree without a pathspec commit whatever happens to be in the index at that instant, including
files another agent staged moments earlier, under an unrelated commit message. Observed 2026-08-09:
an interactive groom's three staged files were swallowed by two concurrent autonomous commits, and
the groom's own commit then reported `nothing to commit, working tree clean`. The content survived —
its rationale did not, and the agent that lost the race could not tell a theft from a no-op.

Both defects are properties of sharing one tree. The design question was therefore whether to stop
sharing it, or to keep sharing it and bound the damage.

## Decision

**Keep the single shared `.docket` worktree, and make collisions survivable rather than
impossible.** Two mechanisms:

- **A bounded, discriminating retry in the preflight sync.** Fetch first and skip the rebase
  entirely when the remote has not moved (the most common collision is a dirty tree with nothing to
  rebase onto); on a failure that can self-heal, retry — 5 attempts on 2/4/8/8s backoff, roughly 22
  seconds — and spend retries only on those failure classes, never on a deterministic error. Never
  `--autostash`: on a shared tree it stashes *another agent's* edits.
- **Pathspec-scoped commits everywhere.** Every commit into the shared tree names the paths it
  intends to commit, so a lost race can damage nothing: the loser commits nothing rather than
  committing someone else's work.

**Rejected alternatives**, each for its own reason:

- **Per-session metadata worktrees**, which would make collisions impossible. This needs a
  mint/lease/prune lifecycle, N checkouts, and a rewrite of every shared-tree invariant, guard and
  learning built since (ADR-0046's clean-tree gates among them). Today's real concurrency is one
  interactive session overlapping one autonomous loop; parallel fan-out is change #0008, which is
  **deferred**. Buying the heavyweight answer ahead of the workload that would justify it is the
  wrong default. **#0008's revival is the explicit re-opening point for this decision** — if that
  change comes back, re-open this ADR rather than treating the shared tree as settled.
- **An advisory lock** around the write→commit→push critical section. That section spans multiple
  tool calls, so no single script can own the lock, and a crashed agent strands it; the lease and
  expiry machinery needed to fix that is most of the per-session cost with none of its benefit.
- **Shrinking the dirty window by prose alone.** It narrows a race rather than bounding damage, and
  has no checkable shape. (This rejects prose as a *race* fix; it does not reach prose as a
  *blast-radius* fix — see *Consequences*.)

**Second decision, same posture: a wedged tree halts.** A shared tree found mid-rebase or mid-merge
yields a distinct report token, `blocked-wedged-tree`, which `--must-land` treats as not-landed, so
the autonomous caller halts. It deliberately does **not** overload the existing retryable
`push-failed`. This chooses **correctness over availability**, and the availability being given up
was never real: what the old behaviour bought was committing a half-rebased tree's staged content
under a board-refresh message, which is corruption, not a feature.

Build-time measurement sharpened this and is worth recording. On git 2.55, mid-rebase with
conflicts, the **pathspec** commit form exits 0 and writes onto the rebase's detached HEAD, while
the pathspec-less form is refused. Scoping the commit therefore *removes* an accidental protection
that the unscoped form had by luck — which is precisely why the explicit wedged probe is required
rather than optional.

## Consequences

The shared tree, and every invariant, guard and learning built on it, survives unchanged; the fix is
small, reversible, and adds no config knob and no lifecycle state. It costs availability in exactly
one place — a wedged tree stops an autonomous run instead of writing through it — which is the trade
this decision exists to make.

**The blast-radius decision reaches both commit channels, and this ADR is the only record of that**
(there is deliberately no second ADR for the second channel):

- **Script-issued commits** — every `git commit` in docket's own scripts that runs in the shared
  tree stages by explicit pathspec. This channel is guarded mechanically by
  `tests/test_shared_worktree_commit_scope.sh`, so a new unscoped commit reddens the suite.
- **Agent-issued commits** — commits written from skill prose, in the same tree, under the same
  hazard. This channel is carried by the rule stated at `docket-convention`'s direct-git grant, plus
  the marker `Stage by explicit path` at all seven metadata-writing skills. The redundancy is
  deliberate: the grant sentence is where a future skill inherits the authority, and the per-site
  marker is where an agent actually reads it.

Stated honestly, the two channels are not equally strong. The agent-facing layer reduces risk; the
script layer provides the enforceable guarantee. No in-repo test can be the oracle for what a model
does with an instruction, so the marker is discipline, not enforcement — and anyone reasoning about
this invariant later should not mistake the guard's green suite for coverage of both channels.
