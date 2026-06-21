# sync-integration-branch.sh — best-effort FF-only sync of the integration branch checkout

## Purpose

After a docket merge lands on `origin/<integration_branch>`, fast-forwards the clone's local
`<integration_branch>` checkout so skills symlinked from the primary checkout stay current. Invoked
once at the end of each run by `docket-finalize-change` (after all merges) and `docket-status`
(after all swept changes are archived) — never per-change.

Best-effort, not fail-closed: every runtime skip condition exits 0 with a one-line note and never
aborts the close-out. Only usage errors (missing `--integration-branch`, unknown flag) exit non-zero.
A no-op in `main`-mode, where the metadata working tree is already the integration branch checkout.

## Usage

```
sync-integration-branch.sh \
  --integration-branch BR \
  [--clone-dir DIR] \
  [--remote R]
```

- `--integration-branch` — the branch to fast-forward (required; typically `main` or `develop`).
- `--clone-dir` — directory of the git clone to operate on. Defaults to the repo root containing
  the script itself (`dirname "$0"/..`, resolved with `pwd -P`).
- `--remote` — remote name (default `origin`).

**Mock seam:** `GIT="${GIT:-git}"` — tests substitute a wrapper.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Fast-forwarded successfully, already current, or any skip condition met (not-a-repo, wrong branch, dirty tree, non-FF divergence, fetch failure). |
| 2 | Usage error: missing `--integration-branch` or unknown flag. |

## Invariants

- **Triple gate — all three must hold before any merge is attempted:**
  1. The clone's current checkout is exactly `<integration_branch>` (not detached HEAD, not a
     feature branch, not `main` when `integration_branch` is `develop`).
  2. The working tree is clean: `git status --porcelain` produces no output (tracked modifications
     and untracked non-ignored files both block the fast-forward).
  3. The local tip is a strict ancestor of `FETCH_HEAD` (true fast-forward): `git merge-base
     --is-ancestor HEAD FETCH_HEAD`. Diverged histories are skipped, not forced.

- **FF-only merge.** `git merge --ff-only FETCH_HEAD` is the only merge operation. Failure of
  this call (e.g., unexpected conflict) is treated as best-effort: a note is emitted and the
  script exits 0 without aborting.

- **Fetch is included.** `git fetch <remote> <branch>` runs on every invocation (cheap/no-op at
  merge sites that already fetched). Fetch failure is a best-effort skip, not an error.

- **Never aborts the close-out.** No runtime condition causes a non-zero exit. The only non-zero
  exits are usage errors (exit 2) caught at startup before any git operations run.
