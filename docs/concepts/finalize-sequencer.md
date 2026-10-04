# Finalize as a sequencer

## The problem it solves

Landing a finished branch is not one action; it is a sequence, and every step in
it can fail in a way that must stop the rest. The branch was written against its
base as it stood days ago, and other work has landed since, so it has to be
rebased and retested before anyone can trust it. The merge itself may be gated
on an approval. After the merge the work has to be closed out and its branch
torn down. Do these by hand, out of order or half-way, and you get a branch that
merged without retesting, or a torn-down worktree whose **change** — one unit of
planned work, roughly one pull request, tracked as one markdown file — was never
closed out.

The steps are also not alike in kind. Resolving a rebase conflict is a different
job from repairing a test the rebase broke, and asking whether a human approval
is required is a policy question, not a mechanical one. Run them as one
undifferentiated blob and you have a single worker doing jobs it is bad at.

Docket runs close-out as **finalize**, a fixed chain of `docket finalize`
operations — rebase, retest, publish, merge, close out, clean up — in which each
step gates the next and each specialized job is split out to the worker suited
to it.

## The moving parts

```
  approved PR
        │
        ▼
  finalize retarget-children: open child PRs move to the parent's base
        │            (only when open children exist)
        ▼
  finalize rebase: onto the change's effective base
        │            └─(conflicts)─► resolver reconciles each hunk by intent
        ▼
  retest the rebased branch
        │            └─(red)─► integration repair: minimal fix, then retest
        │                       (at most finalize.repair_max_attempts)
        ▼
  finalize publish: push the rebased head, update the PR's build evidence
        │
        ▼
  finalize merge: rebase, merge commit, or squash
        │            (the first method the repository permits)
        ▼
  finalize closeout: archive the change on the docket branch,
        │            retarget backlinks
        ▼
  finalize cleanup: tear down the branch and worktree (fail-closed)
```

- Stacked children whose pull requests target this branch are retargeted onto
  the parent's effective base first, so none points at a branch about to be
  merged and deleted; the merge refuses while a child is still open against it.
- The rebase is the first gate: the branch is replayed onto the change's
  **effective base** — the integration branch (the branch code lands on, usually
  `main`), or for a stacked change its parent's branch while the parent is still
  live. Only a clean replay proceeds. A conflict is handed to a resolver that
  reconciles each hunk by merge intent, rather than being patched inline by the
  sequencer.
- Retest is the second gate: a rebase that applied cleanly can still have broken a
  test by combining two correct changes. A red suite here goes to a bounded
  integration-repair step — a minimal fix, never a weakened test — and the suite
  is re-run, at most `finalize.repair_max_attempts` times, before the sequence
  continues. When the rebase was a no-op and the branch already carries green
  build evidence for that exact head, produced by the same test command, the
  suite is not run again.
- A repair that turns the suite green merges like any other green change; the
  run report and the archived record's closeout notes name what broke and the
  repair commits. Finalize adds no human stop of its own: when the repository
  requires approvals and dismisses stale approvals on new commits, pushing the
  repair removes the approval and the merge waits for a fresh one.
- `finalize.gate: off` skips the rebase and the retest entirely; the remaining
  steps still run in order.
- Publish pushes the rebased head and updates the build-evidence block in the
  pull request body, so the evidence names the exact commit that will merge.
- The merge is a policy gate, kept separate from the mechanical ones: whether a
  human approval is required is configured ahead of time
  (`finalize.require_pr_approval`), and the single-maintainer path is branch
  protection that requires a pull request but zero approvals. Finalize merges
  with the first method the repository permits: rebase, then merge commit, then
  squash.
- Closeout proves the merge landed, then marks the change `done` and archives it
  on the `docket` branch (a stacked change merged into its parent is marked
  `stacked-merged` instead), and retargets the backlinks in the change's spec, plan,
  and results files to the archived record. Nothing is copied to the
  integration branch. A backlink retarget that fails leaves a
  `final-backlink-pending` finding (the change stays `done`), which
  `docket finalize cleanup` repairs.
- Cleanup removes the feature branch and its worktree, and is fail-closed: it
  never leaves the repository half-destroyed, so an interrupted close-out is
  recoverable rather than a worktree gone with its change not closed out.

## The invariants

- Finalize is an ordered sequence; each step gates the next, and a failed step
  stops the ones after it rather than pressing on.
- The branch is rebased onto its current effective base and retested before any
  merge; a stale branch never merges untested, unless `finalize.gate: off` says
  so explicitly.
- Conflict resolution and semantic repair are split at the rebase-completion
  boundary — resolving a conflict and repairing a rebase-broken test are
  different jobs given to different workers.
- Integration repair is bounded by `finalize.repair_max_attempts` and never
  weakens a test to go green; if it cannot re-green the suite, the sequence stops
  for a human. A repair that re-greens the suite merges; reviewing repairs is
  the repository's approval policy, not a finalize step.
- Whether the merge needs a human approval is a configured policy gate, settled
  before finalize runs, not decided by the sequencer mid-flight.
- A change is marked `done` only after closeout has proved its pull request
  merged.
- Branch and worktree teardown is fail-closed — never half-destructive — so an
  interrupted finalize is recoverable.

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
