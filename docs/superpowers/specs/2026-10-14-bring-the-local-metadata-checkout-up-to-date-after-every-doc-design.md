# Bring the local metadata checkout up to date after every docket write — design

Change 0549 — Bring the local metadata checkout up to date after every docket write

Change #549, groomed 2026-10-14.

## Problem

Every typed metadata operation (create, groom, claim, reconcile, implemented, halt, kill, finalize
closeout, learning and ADR writes, and the rest) runs through the transaction engine
(`internal/repository/transaction`, `Engine.Execute`). The engine commits in a throwaway candidate
worktree under `<git-common-dir>/<state>/transactions/` and pushes the commit straight to the
metadata remote under an exact lease. It never moves the clone's local metadata branch or the
metadata checkout on disk. Only `repository prepare` fast-forwards that checkout (#523), and it runs
at the start of the next workflow or `docket open`.

In a shared repository this is invisible: the board is read on GitHub, which reads the pushed
branch. In a private repository the metadata checkout
(`<data>/dckt/<owner>-<repo>/checkouts/<clone-id>/`) is the only place a human can read
`BOARD.md`, so after finalize regenerates the board the file on disk still shows the old state
until something runs prepare. Board generation itself is correct: in a private test repository
every board-visible operation rewrote `BOARD.md` on the branch. Only the local copy lags.

`repository repair` has the same gap through a different path: it commits with `CommitTree` and
pushes with `PushLease` itself, outside the engine.

## Goal

When a docket write returns, the metadata checkout on disk already shows what that write pushed,
whenever that can be done safely. In particular, right after finalize returns, `BOARD.md` in the
checkout shows the change as done.

Non-goals: changing how or when the board is rendered, changing `repository repair`'s job,
changing `repository prepare`, moving transactions off their throwaway worktree, changing any
command's output, or adding a board-refresh command. `docket open board` stays the manual way to
sync and view the board (`--print` prints the path).

## Design

### 1. Engine: an optional after-apply hook

Add an optional field to `transaction.Engine`, alongside `AdmissionHook`:

```go
// AppliedHook, when set, runs once after an attempt's outcome is applied or
// already-applied, with the request and the final result. It cannot change the
// result and has no error return. Set it once at wiring time, before the first
// Execute; it must be safe for concurrent invocation.
AppliedHook func(ctx context.Context, req Request, res Result)
```

- It runs only when the final disposition is `DispositionApplied` or `DispositionAlreadyApplied`
  and `res.AppliedCommit` is non-empty. It never runs on refused, contended, no-op, interrupted, or
  failed outcomes.
- It runs once per `Execute`, after the attempt loop settles, not per attempt.
- A nil hook keeps today's behavior exactly.
- The engine stays git-only and layout-agnostic: the hook receives the request (repository, remote,
  target ref) and the result; it decides everything else.

### 2. App: follow the pushed commit

Add one app-layer function, for example `followMetadataCheckout(ctx, client, repo, targetRef,
pushed)`, that is best-effort and silent:

1. Resolve the repository's layout (`resolveLayout`, which already yields the shared `.docket/` or
   the private `checkouts/<clone-id>/` path). On any error, return.
2. Return unless `targetRef` equals the layout's metadata ref (`Layout.MetadataRef()`).
3. Return unless the metadata worktree exists and has that branch checked out. Never create or
   attach a checkout here; that stays `prepare`'s job.
4. Read the local metadata branch tip. If it already equals `pushed`, return.
5. Call the existing in-place fast-forward,
   `gitcli.Client.FastForwardCheckedOutBranch(ctx, worktree, metaRef, localTip, pushed)`.
   It already:
   - moves the branch by compare-and-swap from `localTip`;
   - requires `pushed` to descend from `localTip`, so it never moves the branch backwards or across
     a divergence;
   - refuses on an unfinished Git operation, and its `read-tree -u -m` refuses rather than
     overwrite a local change or an untracked file;
   - runs no repository hooks.
6. Ignore any error. Nothing is reported; the next `prepare` sees the same state it sees today and
   reports it (`metadata-worktree-dirty`, ahead, diverged) with its own remedy.

No fetch is needed: the pushed commit was created in this repository, so its objects are local.
The cost is a handful of local git calls.

### 3. Wiring

- In `newPlanningDepsOver` (`internal/cli/change.go`), the single place the engine is assembled
  and where `AdmissionHook` is already wired, set `engine.AppliedHook` to call
  `followMetadataCheckout` with the request's repository, its target ref, and
  `res.AppliedCommit`. Wire it unconditionally, not only when a feature `repoDir` is supplied, so
  maintenance and standalone builds (sweep closeouts included) are covered.
- In `executeRepositoryRepair`, after the push is applied and the remote revision is verified to
  equal the repair commit, call the same function with the metadata ref and that commit.

### 4. Behavior in the cases that matter

| Local checkout state when a write lands | Result |
|---|---|
| clean, at the write's base (normal) | fast-forwarded to the pushed commit; `BOARD.md` current |
| clean, behind the write's base | fast-forwarded (the pushed commit descends from it) |
| already at or past the pushed commit (a concurrent write moved it) | untouched |
| uncommitted edits, untracked file in the way, or unfinished Git operation | untouched; the write still reports applied |
| local branch has its own commits (ahead or diverged) | untouched |
| checkout absent, or a different branch checked out | untouched; the next `prepare` attaches |

Two concurrent writes cannot move the branch backwards: each fast-forward is a compare-and-swap
that requires the target to descend from the observed tip. Writes run from a feature worktree
behave the same, because the metadata checkout is per clone and resolved from the shared common
dir.

## Docs

`docs/guide/where-the-metadata-lives.md`, in *The `.docket/` metadata worktree*: add that every
docket write also moves the metadata worktree forward to the commit it just pushed when it safely
can, so the files there (the board included) are current when the write returns, and that
`docket open board` opens the board from it. Describe current behavior only.

## Testing

- **Engine unit tests:** the hook runs exactly once on applied and on already-applied (idempotent
  replay), with the applied commit; it never runs on refused, contended, no-op, or failed outcomes;
  a nil hook changes nothing.
- **Integration, shared and private fixtures:** after a board-changing typed write (for example
  `change.create` or a finalize closeout), the metadata checkout's HEAD equals the pushed commit and
  `BOARD.md` on disk equals the branch's `BOARD.md`.
- **Left alone:** with an uncommitted edit in the checkout, the write reports applied and the
  checkout's branch, HEAD, and files are unchanged. With the checkout already moved past the write's
  commit, it is unchanged.
- **Repair:** after `repository repair --yes` applies, the checkout is at the repair commit.
- **Mutation check:** removing the `AppliedHook` wiring in `newPlanningDepsOver` turns the
  integration test red.
- Run the whole suite at the build gate.

## Related

- #523 made `prepare`'s in-place fast-forward safe and defined a clean, behind-only checkout as
  healthy. This change reuses that fast-forward and leaves `check`'s health rules unchanged.
- #531 / ADR-0142: the private layout this mostly benefits.
- ADR-0089: metadata-worktree contention is survivable; this adds no new contention beyond what
  `prepare` already does.
- #360 (session-scoped sync) is adjacent: fewer stale checkouts mean fewer full syncs, but this
  change adds no sync step.
