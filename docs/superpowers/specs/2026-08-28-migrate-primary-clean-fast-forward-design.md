<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0364 — Advance the primary in place on migrate via a gitcli clean-fast-forward primitive](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0364-migrate-primary-clean-fast-forward.md)**
<!-- docket:backlink:end -->

# Advance the primary in place on migrate via a gitcli clean-fast-forward primitive

## Purpose and boundary

`docket repository migrate` (change 0352) publishes the metadata and integration tips to the remote
durably, then finishes the local side: it attaches the `.docket` worktree, disables its hooks,
installs authorized surfaces, and reconciles the primary worktree with the migrated integration
branch. That last step is not performed in place. Instead `migratePrimarySyncRemedy`
(`internal/app/repository_migrate.go`) returns a `pending_local` remedy string telling the human to
run `git -C <primary> merge --ff-only <remote>/<branch>` themselves, and the migration result is
marked `needs-review`.

The reason is stated in the code itself (`repository_migrate.go`, `migratePrimarySyncRemedy` doc
comment): a clean-fast-forward working-tree primitive is intentionally outside the migrate service's
Git surface, because `internal/gitcli` has no such primitive and change 0352 added none. On the
happy path — the primary worktree still parked exactly at the pinned source commit, working tree
clean — that hand-off is pure friction: the advance is a guaranteed clean fast-forward that migrate
could perform itself.

This change adds the missing primitive to `internal/gitcli` and wires migrate's happy path to it, so
a routine migration leaves the primary already advanced and reports `healthy`, while every unsafe
or ambiguous case degrades to exactly today's `pending_local` behavior.

**In scope**

- One new `internal/gitcli` primitive that fast-forwards a worktree's checked-out branch to a target
  ref, refusing any non-fast-forward and any dirty working tree.
- Wiring `migrateLocalFinish` / `migratePrimarySyncRemedy` to attempt the in-place advance on the
  happy path only, falling back to the existing remedy string on refusal.

**Out of scope**

- The `moved` (reconcile) branch of `migratePrimarySyncRemedy` — a primary that has moved past the
  pinned source stays a `git pull --rebase` remedy string, unchanged.
- Any caller other than migrate. `init` and `check` are untouched.
- Auto-stashing, auto-committing, or otherwise resolving a dirty tree. A dirty tree is a refusal,
  never a mutation of the user's uncommitted work (chosen 2026-08-28).
- Rolling back or re-erroring the migration when the local advance fails: the remote migration is
  already durable and must never be undone by a local-finish failure.

## The primitive

Add to `internal/gitcli` a method with the shape (working name; the implementer may rename to match
the package's verb conventions):

```
func (c *Client) FastForwardWorktree(ctx context.Context, worktree string, target ObjectID) (advanced bool, err error)
```

Semantics:

- **Mechanism.** Run `git -C <worktree> merge --ff-only <target>`. This is worktree-aware and
  already refuses, without mutating anything, both a non-fast-forward (target is not a descendant of
  the worktree's current head) and a dirty working tree (uncommitted changes that the fast-forward
  would overwrite). Do not hand-roll these checks by shelling `reset --hard` — that would silently
  discard local work. `IsAncestor` already exists in the package and MAY be used for an explicit
  pre-check to distinguish a non-fast-forward refusal from a dirty-tree refusal in the returned error
  kind, but is not required for correctness.
- **Success.** A clean fast-forward that advances the branch returns `advanced = true, err = nil`.
  An `--ff-only` that is a no-op because the worktree is already at the target (already up to date)
  is also a non-error success; return `advanced = false, nil` so the caller can tell "advanced" from
  "already there" if it wants to, though migrate treats both as "no pending line needed".
- **Refusal.** A non-fast-forward or dirty-tree refusal returns a typed failure (reuse the package's
  `newFailure` / `Kind*` vocabulary — e.g. `KindCommandFailed`, or a more specific kind if the
  pre-check distinguishes them). The caller decides whether a refusal is fatal; for migrate it is
  not.
- **Validation.** Validate `target` with the existing `validateObjectID` guard as the other
  primitives do, returning `KindInvalidRequest` on a malformed id.

Follow the established `internal/gitcli` patterns: a dedicated `Operation` constant, `newFailure`
wrapping with a `stderrExcerpt`, and an `..._integration_test.go` file exercising it against a real
repository (the primitive touches the working tree, so it belongs in the integration partition, not
a unit test).

## Migrate wiring

In `migrateLocalFinish` / `migratePrimarySyncRemedy` (`internal/app/repository_migrate.go`):

1. Keep the existing `moved` determination (walk `ListWorktrees`, compare the primary worktree head
   to the pinned `sourceOID`). The `moved` branch is unchanged: it still returns the
   `git pull --rebase` reconcile remedy string.
2. On the **not-moved happy path** (primary head still equals `sourceOID`), call the new primitive to
   advance the primary worktree to `metadataTip`'s corresponding integration tip — i.e. the migrated
   integration branch commit the remedy string names (`<remote>/<branch>`). Resolve the target the
   same way the remedy string does today.
   - On **success**, emit **no** `pending_local` line for the primary sync. With no other pending
     items, `migrateApplied` then reports `RepositoryState: healthy` rather than `needs-review`.
   - On **refusal / any error**, fall back to exactly today's remedy string (the same
     `git merge --ff-only` line), so behavior is byte-for-byte the current one in every case the
     advance can't be performed cleanly. The failure is swallowed into the remedy, never surfaced as
     a migrate error.
3. The other `migrateLocalFinish` pending items (worktree attach, hooks disable, surface install) are
   unchanged and independent of this path.

The failure posture is the same one the local-finish step already documents: the remote migration is
durable, so any local step that cannot complete is reported as `pending_local`, never rolled back.

## Testing

- **gitcli integration test** (new `*_integration_test.go`): against a real repo/worktree —
  (a) a clean fast-forward advances the branch and returns `advanced = true`; (b) an already-at-target
  worktree returns `advanced = false, nil`; (c) a non-fast-forward (divergent head) is refused with a
  typed failure and the worktree is left untouched; (d) a dirty working tree over an otherwise-clean
  fast-forward is refused and the uncommitted changes are preserved; (e) a malformed target id
  returns `KindInvalidRequest`.
- **migrate-level test** (in the existing repository-migrate integration coverage): the not-moved
  happy path advances the primary worktree in place and the result carries **no** primary-sync
  `pending_local` line and reports `healthy`; a dirty or moved primary still yields the corresponding
  remedy string and `needs-review`.
- Place both in the established integration partition so no individual test or shard reaches the 60s
  budget ceiling, consistent with 0352's test layout.

## Open questions

- Exact method name and whether the dirty-vs-non-ff distinction is worth an explicit `IsAncestor`
  pre-check for a more precise error kind, or whether a single refusal kind is enough. Either
  satisfies the contract; the implementer picks per the package's conventions.
