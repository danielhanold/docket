---
id: 47
slug: digest-only-read-tier-skips-preflight
title: docket-status --digest-only is a read tier that deliberately skips docket_preflight
status: Accepted
date: 2026-07-19
supersedes: []
reverses: []
relates_to: [12]
change: 94
---

## Context

`docket-implement-next` step 1 needs an ordered build-ready queue — the digest's `ready`
line — to pick the next change deterministically. Acquiring that queue must be a *read*:
a selection read must not also be a write.

The only pre-existing path that emitted the digest was `docket-status --board-only`, which
also commits and pushes `BOARD.md`. That path is unusable for selection — a caller that
merely wants to know what's next should not also be publishing a board update as a side
effect.

`main()` in `docket-status.sh` opens with `docket_preflight`, which fetches and
`pull --rebase`s the metadata worktree. That is a working-tree mutation and can move
`HEAD`. Reusing `main()`'s existing entry sequence for a new digest-only mode would have
made the "read" a write by construction.

## Decision

`--digest-only` short-circuits in `main()` *before* `docket_preflight` runs. It resolves
config only (the config export), enforces the bootstrap verdict fail-closed, and runs the
existing backlog pass. It performs no worktree sync, no sweep, no health checks, no board
render, no commit and no push, and emits no `board …` line and no `pass ok`.

Because it skips preflight, it cannot rely on preflight's guarantee that the metadata
worktree exists, so it carries its own fail-closed gates: a missing metadata worktree, a
non-`PROCEED` bootstrap verdict, a failed config export, or a digest render that produces
no output all exit non-zero with no stdout. `backlog_pass` remains deliberately best-effort
for the report paths (a failed digest must never abort a board write or a sweep), but the
digest-only path has nothing else to deliver — an empty digest *is* the failure there.

Exit status is the disambiguation channel callers depend on:

- non-zero exit ⇒ hard error ⇒ the caller STOPs (`halted`); it must never fall back and
  never report `drained`.
- exit 0 + a bare `ready` line ⇒ the queue is genuinely empty ⇒ `drained`.
- exit 0 + no `ready` line at all ⇒ an older `render-board` predating the queue ⇒ degrade to
  walking `active/` and report it.

Without this split, a broken config in an autonomous drain loop would silently read as
"nothing to build."

Mutually exclusive (exit 2) with `--board-only`, `--must-land`, `--repo`, `--project`,
`--auto-create-project`, and `--project-owner` — rather than silently ignoring them.

## Consequences

`--digest-only` gives selection callers a genuine read: no commit, no push, no worktree
mutation, no risk of moving `HEAD` mid-selection.

The cost: the digest is a snapshot of the change files as it finds them — it does not
refresh them. Callers must run it *after* their own Step-0 preflight and sweep, or they
will read stale bytes (a pre-sweep digest lists already-merged changes).

This is the same script-vs-model boundary as ADR-0012: the script owns the deterministic
ordering; the model does not re-derive it. `--digest-only` extends that boundary with a
second axis — not just which side computes the order, but whether obtaining it is
side-effect-free.
