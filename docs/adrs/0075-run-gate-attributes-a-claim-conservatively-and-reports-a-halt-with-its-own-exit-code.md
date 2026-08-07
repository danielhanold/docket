---
id: 75
slug: run-gate-attributes-a-claim-conservatively-and-reports-a-halt-with-its-own-exit-code
title: The run gate attributes a claim conservatively and reports a halt with its own exit code
status: Accepted
date: 2026-08-07
supersedes: []
reverses: []
relates_to: []
change: 237
---

## Context

Change 0237 gives docket's terminal-disposition contract a mechanical consumer: `scripts/verify-run.sh`,
a git-only reader of `docket-implement-next`'s Step 7 postcondition, called from
`scripts/runner-dispatch.sh` after a delegated agent run returns. The spec settled the *shape* —
snapshot the `in-progress` change-id set before the hand-off, diff it after, verify each newly-claimed
id, allow one bounded re-dispatch. Two questions it did not settle were forced by the build and by
whole-branch review.

First, the plain set diff (`AFTER \ BEFORE`) does not identify *our* run's claim. A stale local
`BEFORE` read makes any pre-existing abandoned `in-progress` change look newly claimed. And a
concurrently-running loop's claim lands in the same diff, indistinguishable from ours by clock alone —
`docket-implement-next` re-stamps `claimed_at` at every phase boundary, so a `claimed_at` window
starting at dispatch admits a claim that is not ours. Re-dispatching onto another agent's in-flight
change would put two agents in one feature worktree.

Second, the disposition contract defines `halted` as *stop + surface* — a deliberate stop needing a
human. The gate initially returned the adapter's own exit code for a halt, which for a healthy adapter
is `0`: it told a driver to **continue**. That is precisely the untrustworthy-disposition failure this
change exists to fix.

## Decision

**Claim attribution is conservative, and stands down rather than guessing.** The metadata worktree is
re-synced on **both** sides of the hand-off, not only before the "after" read, so a stale `BEFORE`
cannot manufacture a phantom claim. A `claimed_at` window narrows the candidate set but explicitly
does **not** close the concurrent case. What closes it is **cardinality**: an implement-next run claims
at most one change, so two or more surviving candidates means none can be attributed — the gate warns
and stands down rather than re-dispatch.

One residual is accepted and documented: if this run claimed nothing (`drained`) while a concurrent
loop claimed exactly one change inside the window, that candidate is still attributed to us. Closing it
requires the child to report the id it claimed — a protocol change outside this seam.

**A halt gets its own exit code.** The gate exits **3** for `run-halted`, distinct from **1** for the
two-strikes abort (a run that never finished) and from the adapter's verbatim code on every no-action
path. The post-re-dispatch matrix is `run-complete` → 0, `run-unclaimed` → 0, `run-halted` → 3,
`run-incomplete` → 1, unparseable → the first adapter's code. This is deliberately an exit code
encoding a non-failure, so it was checked against every consumer: the only consumer of
`runner-dispatch.sh`'s exit code is the generated shim wrapper body emitted by `sync-agents.sh`, whose
rule is bare-non-zero abort-and-report — which *is* stop + surface, so the new code is read correctly.

## Consequences

**Enables.** A halt is actionable by any driver without parsing prose. The gate prefers a false
negative — stand down, warn, and let `board-checks`' `aborted-run` finding remain the backstop — over a
false positive that would spend a full agent run re-dispatching onto work that is not ours, or share a
feature worktree with a live agent.

**Costs.** Attribution is now more than a set diff: two syncs, a time window, and a cardinality rule,
carrying a documented residual that only a child-reported claim id would close.

**Gives up.** The previously two-valued exit space of `runner-dispatch.sh`. `3` is a new value in it,
so any future consumer of that exit code must be checked against the halt meaning rather than assuming
non-zero implies a failed run.
