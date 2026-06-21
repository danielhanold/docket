# cleanup-feature-branch.sh — provenance-guarded teardown of a finished change's feature branch and worktree

## Purpose

Removes the `feat/<slug>` worktree and branch (local + remote) for a completed or killed change.
A provenance guard ensures only worktrees physically under `.worktrees/` are removed — the
`.docket/` metadata worktree and any out-of-tree path are always refused. Invoked by
`docket-finalize-change` and `docket-implement-next` after a change is archived.

Fail-closed: self-verifies the worktree and local branch are gone after removal.
Idempotent: if the worktree and branch are already absent, exits 0 (safe no-op).

## Usage

```
cleanup-feature-branch.sh \
  --slug SLUG \
  [--worktrees-dir DIR] \
  [--remote R]
```

- `--slug` — the change slug (from the change filename `<pad>-<slug>.md`). Resolves to the
  worktree path `<worktrees-dir>/<slug>` and the branch name `feat/<slug>`.
- `--worktrees-dir` — worktrees directory relative to the repo root (default `.worktrees`).
- `--remote` — remote name (default `origin`).

**Mock seam:** `GIT="${GIT:-git}"` — tests substitute a wrapper.

## Behavior

1. **Resolve repo root.** `git rev-parse --show-toplevel` from the current working directory;
   fails immediately if not inside a git repository.

2. **Provenance guard.** If the target path `<worktrees-dir>/<slug>` exists, resolves it to a
   canonical absolute path using `pwd -P` (the `canon()` helper: `cd "$dir" && pwd -P`). The
   canonical path must have the prefix `<canonical-repo-root>/.worktrees/`; any other prefix
   causes an immediate fatal error. This guard — using `pwd -P` instead of raw string comparison
   — prevents symlink attacks and macOS `/var` vs `/private/var` mismatches from bypassing the
   check (LEARNINGS #25).

3. **Worktree removal.** If the target exists and passes the provenance guard:
   `git worktree remove --force <target>`. Errors from this command are fatal.
   If the target does not exist, this step is skipped (idempotent).

4. **Local branch deletion.** `git branch -D feat/<slug>` — errors are silently swallowed (`|| true`);
   branch may already be gone or may never have been created locally.

5. **Remote branch deletion.** `git ls-remote --exit-code <remote> feat/<slug>` probes for the
   remote branch. If found, `git push <remote> --delete feat/<slug>` removes it; errors are fatal.
   If not found, the step is skipped.

6. **Fail-closed self-verification.** Checks: `<target>` path does not exist; `git rev-parse
   --verify -q feat/<slug>` fails (local branch gone). Any postcondition failure is fatal.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Worktree and branch cleaned up — or already absent (idempotent no-op). |
| non-zero | Not in a git repository, provenance guard violation, worktree remove failed, remote branch delete failed, or postcondition check failed. |

Usage/flag errors (unknown flag) also exit non-zero.

## Invariants

- **Provenance guard via `pwd -P`.** The worktree target is canonicalized with `pwd -P` before
  comparing to the allowed prefix `<repo-root>/.worktrees/`. This is LEARNINGS #25: raw path
  comparison would fail on macOS (where `mktemp` paths and `git rev-parse` paths differ under
  `/var` vs `/private/var`), and would be bypassable with symlinks. The `canon()` helper is empty
  when the directory does not exist, so a missing target simply skips the worktree removal step
  rather than erroring.

- **Never removes the `.docket/` metadata worktree.** The `allowed_root` is always anchored to
  `<repo-root>/.worktrees/`, not to the generic worktrees list. The `.docket/` path will not match
  this prefix and is always refused.

- **Local branch deletion is best-effort.** `git branch -D` failure is silenced. The branch
  may have been deleted by a prior run or may never have been pushed/created. The postcondition
  only checks that it is gone, not that this step did the removal.
