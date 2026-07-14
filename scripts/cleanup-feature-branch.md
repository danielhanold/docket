# cleanup-feature-branch.sh — provenance-guarded teardown of a finished change's feature branch and worktree

## Purpose

Removes the `feat/<slug>` worktree and branch (local + remote) for a completed or killed change.
A provenance guard ensures only worktrees physically under `.worktrees/` are removed — the
`.docket/` metadata worktree and any out-of-tree path are always refused. Invoked by
`docket-finalize-change` and `docket-implement-next` after a change is archived.

Fail-closed: self-verifies the worktree and local branch are gone after removal.
Idempotent: if the worktree and branch are already absent, exits 0 (safe no-op).

The repo root is resolved from the **main worktree** (change 0075), so the script means the same
thing from every CWD — including the `.docket/` metadata worktree — and additionally refuses,
before any destructive step, when the caller's own CWD is at or inside the target worktree it is
about to remove.

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

1. **Resolve repo root.** The **main worktree** of the repo (change 0075), via
   `docket_main_worktree` (`scripts/lib/docket-root.sh`): `git worktree list --porcelain`,
   first entry — never `git rev-parse --show-toplevel`, which returns whatever *linked*
   worktree the caller happens to be standing in (the `.docket/` metadata worktree or a
   `.worktrees/<slug>` feature worktree). Fails immediately if not inside a git repository.
   The target path `<worktrees-dir>/<slug>` is then built ABSOLUTE, anchored to this root (an
   already-absolute `--worktrees-dir` is honored verbatim) — so every step below, and the
   fail-closed refusal, mean the same thing regardless of the caller's CWD.

2. **Fail-closed CWD refusal (change 0075, defect D1).** The caller's CWD is captured *before*
   any `cd`. If it is at or inside the (canonicalized) target worktree, the script refuses
   immediately — before either destructive step below — and exits non-zero having touched
   nothing. See **Refusal** below.

3. **Provenance guard.** If the target path exists, resolves it to a canonical absolute path
   using `pwd -P` (the `canon()` helper: `cd "$dir" && pwd -P`). The canonical path must have
   the prefix `<main-worktree-root>/.worktrees/`; any other prefix causes an immediate fatal
   error. This guard — using `pwd -P` instead of raw string comparison — prevents symlink
   attacks and macOS `/var` vs `/private/var` mismatches from bypassing the check
   (LEARNINGS #25). Unchanged by change 0075.

4. **Worktree removal.** If the target exists and passes the provenance guard:
   `git -C <root> worktree remove --force <target>`. Errors from this command are fatal.
   If the target does not exist, this step is skipped (idempotent).

5. **Local branch deletion.** `git -C <root> branch -D feat/<slug>` — errors are silently
   swallowed (`|| true`); branch may already be gone or may never have been created locally.

6. **Remote branch deletion.** `git -C <root> ls-remote --exit-code <remote> feat/<slug>` probes
   for the remote branch. If found, `git -C <root> push <remote> --delete feat/<slug>` removes
   it; errors are fatal. If not found, the step is skipped.

7. **Fail-closed self-verification.** Checks: `<target>` path does not exist; `git -C <root>
   rev-parse --verify -q feat/<slug>` fails (local branch gone). Any postcondition failure is
   fatal.

## Refusal (fail-closed)

When the caller's CWD is at or inside the target worktree, the script exits non-zero having
attempted **no** destructive step — neither the worktree removal nor the remote branch delete.
Containment, not equality: a subdirectory nested arbitrarily deep inside the target also refuses.
Remedy: `cd` to the repo root (printed in the error message) and re-run.

**D1 history:** pre-0075, the repo root was resolved with `git rev-parse --show-toplevel` and the
target was built *relative* to the caller's CWD. Invoked from any linked worktree (`.docket/`, or
the feature worktree itself), the relative target never resolved: the worktree-removal step was
silently skipped, `git branch -D` failed into `|| true`, but execution still reached
`git push --delete`, which **succeeded** — the remote branch was destroyed — and only then did the
postcondition check die, reporting failure. A partial, irreversible destruction reported as an
error. Change 0075 fixes this at the root (an absolute, main-worktree-anchored target) and adds the
CWD refusal as defense in depth, since `git worktree remove --force` itself does not refuse to run
with a process CWD inside the worktree being removed — it merely orphans that CWD afterward.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Worktree and branch cleaned up — or already absent (idempotent no-op). |
| 1 | Not in a git repository, **CWD refusal** (`refusing to clean up feat/<slug>: the caller's CWD is at or inside the target worktree …` — change 0075), provenance guard violation, worktree remove failed, remote branch delete failed, or postcondition check failed. |

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

- **Root is the main worktree, never `--show-toplevel` (change 0075).** `git rev-parse
  --show-toplevel` returns whichever *linked* worktree the caller's CWD happens to be inside —
  from `.docket/` or from the `.worktrees/<slug>` feature worktree itself, that is NOT the repo's
  primary checkout. Resolving the main worktree instead (`docket_main_worktree`, reachable from
  every worktree via `git worktree list --porcelain`'s first entry) and anchoring `target` to it
  absolutely is what makes the script CWD-independent: the worktree removal, both branch deletes,
  and the postcondition all target the same absolute path no matter where the script was invoked
  from. See defect D1 above.

- **CWD refusal is fail-closed, checked before any destructive step (change 0075).** The guard
  compares the caller's CWD (captured before any `cd`) against the canonicalized target by
  containment (`case "$caller_pwd/" in "$target_rp/"*)`), and sits ahead of both the worktree
  removal *and* the remote branch delete — not just the former. A refusal that only guarded the
  worktree removal would still let the remote branch get deleted: exactly the D1 defect. Both
  `caller_pwd` and `target_rp` are canonicalized with `pwd -P` before comparison — `git worktree
  list` prints realpaths while `$PWD` may not be one (e.g. macOS `/tmp` → `/private/tmp`), so
  comparing un-canonicalized paths would silently never fire.

- **The `.worktrees/` provenance guard is unchanged by change 0075.** It still governs whether an
  *existing* target may be removed; the CWD refusal is a separate, earlier check governing whether
  cleanup may run at all from the caller's current position. Neither guard broadens the other.
