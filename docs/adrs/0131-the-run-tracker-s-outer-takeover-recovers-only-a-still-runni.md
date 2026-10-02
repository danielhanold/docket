---
id: 131
slug: 'the-run-tracker-s-outer-takeover-recovers-only-a-still-runni'
title: 'The run tracker''s outer takeover recovers only a still-running drive'
status: 'Accepted'
date: '2026-10-02'
supersedes: []
reverses: []
relates_to: [107, 130]
change: 489
---

## Context

After change 0488, build-task workers no longer start task-owned gate drives. The run tracker keeps one outer recovery scope per run: `run.start` prepares it, `run.verdict` binds the change and takes over a drive that a dead implement-next left behind, and `run.cancel` reads its worktree.

The candidate rule (`FindScopeDriveIDs`) counted terminal drives that had an owner generation set as recoverable. Only the task path (`retirePredecessor`/`Acknowledge`) ever cleared that generation. Since 0488, build-owned starts carry `--change-id`, so every finished build gate stayed a candidate. A run that died after a red-then-green gate sequence ended in a terminal `run-stop takeover-ambiguous`, or in a fingerprint halt after a later commit. The outer scope never carried a run id.

## Decision

- Delete the task-owned gate-drive machinery.
- Keep a minimal in-process outer scope (repo, change, branch, worktree, two capability hashes, a closed flag). It exists only at the coordinator -> implement-next boundary and has no per-test lifecycle.
- `FindScopeDriveIDs` returns only nonterminal drives. A finished drive is never a takeover candidate.
- `Takeover` takes an explicit drive id and still accepts a drive that finished between the scan and the takeover.
- Remove the takeover run-revocation check. It is unreachable because outer scopes carry no run id.

## Consequences

- A run that dies mid-gate still continues via `run-continue`, taking over the live drive.
- Accepted loss: a gate verdict that finished before implement-next died is not reused. The retry (`run-retry-once`) re-runs the suite, costing one `build.max_attempts` attempt and the run's retry. If the attempt budget is already spent, the retry halts for a human.
- The ambiguous-takeover and stale-fingerprint stops caused by finished build gates no longer occur.
- Less code: the task-owned scope lifecycle, generation clearing, and run-revocation check are gone.

## Alternatives considered

- Keep the newest finished drive when the worktree is unchanged. Rejected: it needs an ordering rule plus a fingerprint pre-check to cover a rare window.
- Mark build drives consumed when evidence is recorded. Rejected: it adds a new write path into build evidence.
- Leave the candidate rule for a follow-up change. Rejected: early-stopped runs would stop for a human in the meantime.
