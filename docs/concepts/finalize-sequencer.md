# Finalize as a sequencer

## The problem it solves

Landing a finished branch is not one action; it is a sequence, and every step in
it can fail in a way that must stop the rest. The branch was written against the
integration branch — the branch code lands on, usually `main` — as it stood days
ago, and other work has landed since, so it has to be rebased and retested before
anyone can trust it. The merge itself may be gated on an approval. After the
merge the work has to be archived and its branch torn down. Do these by hand, out
of order or half-way, and you get a branch that merged without retesting, or a
torn-down worktree whose **change** — one unit of planned work, roughly one pull
request, tracked as one markdown file — was never archived.

The steps are also not alike in kind. Resolving a rebase conflict is a different
job from repairing a test the rebase broke, and asking whether a human approval
is required is a policy question, not a mechanical one. Run them as one
undifferentiated blob and you have a single worker doing jobs it is bad at.

Docket runs close-out as **finalize** — the close-out sequence: rebase onto the
integration branch, retest, merge, archive — a fixed chain in which each step
gates the next and each specialized job is split out to the worker suited to it.

## The moving parts

```
  approved / merged PR
        │
        ▼
  rebase onto the integration branch
        │            └─(conflicts)─► resolver reconciles each hunk by intent
        ▼
  retest the rebased branch
        │            └─(red)─► integration repair: minimal fix, then re-gate
        ▼
  merge  ──(policy gate: branch protection, zero required approvals)
        │
        ▼
  archive the change + refresh the board
        │            └─(publish fails)─► marked "## Publish deferred", not lost
        ▼
  tear down the branch and worktree (fail-closed)
```

- The rebase is the first gate: the branch is replayed onto the current
  integration branch, and only a clean replay proceeds. A conflict is handed to a
  resolver that reconciles each hunk by merge intent, rather than being patched
  inline by the sequencer.
- Retest is the second gate: a rebase that applied cleanly can still have broken a
  test by combining two correct changes. A red suite here goes to a bounded
  integration-repair step — a minimal fix, at most a couple of attempts, never a
  weakened test — and the suite is re-run before the sequence continues.
- The merge is a policy gate, kept separate from the mechanical ones: whether a
  human approval is required is configured ahead of time, and the
  single-maintainer path is branch protection that requires a pull request but
  zero approvals.
- Archiving copies the terminal record onto the integration branch and refreshes
  the **board** — the generated overview of every change and its state, never
  edited by hand. If that publish cannot complete, the failure is marked as
  deferred rather than dropped, so a human can finish it later.
- Teardown removes the feature branch and its worktree, and is fail-closed: it
  never leaves the repository half-destroyed, so an interrupted close-out is
  recoverable rather than a worktree gone with its change unarchived.

## The invariants

- Finalize is an ordered sequence; each step gates the next, and a failed step
  stops the ones after it rather than pressing on.
- The branch is rebased onto the current integration branch and retested before
  any merge; a stale branch never merges untested.
- Conflict resolution and semantic repair are split at the rebase-completion
  boundary — resolving a conflict and repairing a rebase-broken test are
  different jobs given to different workers.
- Integration repair is bounded and never weakens a test to go green; if it
  cannot re-green the suite, the sequence stops for a human.
- Whether the merge needs a human approval is a configured policy gate, settled
  before finalize runs, not decided by the sequencer mid-flight.
- Branch and worktree teardown is fail-closed — never half-destructive — so an
  interrupted finalize is recoverable.
- A handled post-archive publish failure is marked as deferred, not dropped, so
  an expected terminal record is never lost silently.

## Decided in

- [ADR-0010](../adrs/0010-finalize-merge-gate-split-agents.md) — split
  conflict-resolution from semantic-repair at the rebase-completion boundary,
  giving the two jobs to two workers.
- [ADR-0011](../adrs/0011-finalize-consent-model.md) — set the finalize consent
  model: an ambiguity-only prompt plus a `require_pr_approval` policy gate.
- [ADR-0035](../adrs/0035-cleanup-teardown-fail-closed.md) — made the
  feature-branch teardown fail-closed, never half-destructive.
- [ADR-0043](../adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md)
  — retired bot auto-approval, making zero-approvals branch protection the
  single-maintainer merge path (reverses ADR-0042's auto-approve consent model).
- [ADR-0090](../adrs/0090-publish-deferred-covers-any-handled-post-archive-failure.md)
  — had a `## Publish deferred` note mark any handled post-archive failure that
  abandons an expected publish.
