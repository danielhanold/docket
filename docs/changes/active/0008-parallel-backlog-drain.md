---
id: 8
slug: parallel-backlog-drain
title: Parallel backlog drain — fan out concurrent implement-next runs over independent build-ready changes
status: proposed
priority: medium
created: 2026-06-11
updated: 2026-06-11
depends_on: []
related: [9]
adrs: [1]
spec:
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
type: feat
---

## Why

Synthesized from the AgentRQ competitive review (2026-06-11). AgentRQ's product model is
supervisor/worker orchestration: a supervisor agent creates tasks across workspaces and worker
agents drain them concurrently (its ACP gateway even ships a `--max-concurrency` task queue).
Notably, AgentRQ gets the hard part wrong for multi-agent use — its `getNextTask` does NOT
atomically claim, so two workers polling one workspace receive the same task; it stays safe only
by serializing to one ongoing task per workspace.

docket has the opposite shape: the hard part is already solved — claiming is a compare-and-swap
push race on the metadata branch with deterministic convergent selection (ADR-0001 territory),
explicitly designed so concurrent implementers "never claim the same change" — but nothing
orchestrates concurrency. `docket-implement-next` is documented as "runs solo per change", and on
losing a claim race it aborts rather than picking the next eligible change. A backlog of N
independent build-ready changes drains serially today.

## What changes

- Loser-picks-next claim semantics in `docket-implement-next`: when the post-race re-read shows
  the selected change is no longer `proposed`, return to selection and claim the next build-ready
  change instead of aborting (bounded retries).
- A drain entry point (new `docket-drain` skill, or a mode of `docket-implement-next` — brainstorm
  decides) that fans out up to N concurrent implement-next runs, one per independent build-ready
  change, each in its own feature worktree, all stopping at their human merge gates.
- Independence guard: never concurrently build changes linked by `depends_on` (already enforced by
  build-readiness, since deps must be `done`) and decide how to treat `related:` overlap.
- Concurrency cap and a summary report of opened PRs at the end of the drain.

## Out of scope

- Any server, queue daemon, or live agent-to-agent messaging — coordination stays entirely in git
  (claim CAS on the metadata branch) plus the orchestrating session.
- Auto-merging: every drained change still stops at its own human merge gate.
- Cross-repo orchestration.

## Open questions

- Orchestrator shape: subagents from one session, or instructions for N independent sessions?
- Merge-conflict exposure: N open PRs cut from the same integration tip will conflict if they
  touch overlapping files — does the drain pre-partition by predicted file overlap, or accept
  rebase pain at the merge gate?
- Board churn under concurrency: the best-effort Board pass (change 0004) was designed for one
  writer; verify it stays calm with N concurrent best-effort refreshes.

## Reconcile log
