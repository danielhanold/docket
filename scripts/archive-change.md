# archive-change.sh — terminal-transition archive primitive

## Purpose

Moves a change from `active/<id>-<slug>.md` to a dated `archive/<YYYY-MM-DD>-<id>-<slug>.md` on the
metadata working tree, sets terminal frontmatter, commits the **change file only**, and pushes to
`origin/<metadata_branch>` with a rebase-retry loop. Handles both `done` and `killed` outcomes.
Invoked by `docket-finalize-change`, `docket-status` (merge sweep), `docket-new-change` (kill),
and `docket-implement-next` (kill).

Fail-closed: self-verifies all postconditions (archive present, active gone, frontmatter correct,
push landed) and exits non-zero with a diagnostic on any deviation.

Idempotent: if a matching `archive/<date>-<id>-*.md` already exists, exits 0 immediately (safe
no-op for concurrent archivers — `docket-finalize-change` racing the `docket-status` sweep).

## Usage

```
archive-change.sh \
  --changes-dir DIR \
  --id N \
  --outcome done|killed \
  --date YYYY-MM-DD \
  [--message MSG] \
  [--results PATH] \
  [--reason TEXT] \
  [--remote R]
```

- `--changes-dir` — absolute path to the `.docket/<changes_dir>` directory inside the metadata
  working tree (used to locate `active/` and `archive/` subdirectories).
- `--id` — numeric change ID (zero-padded to four digits internally).
- `--outcome` — `done` or `killed` (any other value is an error).
- `--date` — UTC merge/kill date (`YYYY-MM-DD`). Callers **must** derive this from the manifest,
  never from `now()`, so concurrent archivers produce tree-identical commits.
- `--message` — optional commit message override. Default:
  `docket(<pad>): <outcome> — archived (status <outcome>, <date>)`.
- `--results` — path to the results file (used only for `--outcome done`; sets `results:` frontmatter).
- `--reason` — free-text reason for a kill; appended as the body of the `## Why killed` section.
  Defaults to `Killed.` when omitted.
- `--remote` — remote name (default `origin`).

**Mock seam:** `GIT="${GIT:-git}"` — tests substitute a wrapper.

## Behavior

1. **Validate inputs.** All four required flags (`--changes-dir`, `--id`, `--outcome`, `--date`)
   must be present; `--changes-dir` must be an existing directory inside a git worktree.

2. **Resolve paths.** Derives the worktree root via `git -C "$CHANGES_DIR" rev-parse --show-toplevel`.
   Computes `REL` (changes-dir relative to the worktree root) using `pwd -P` to resolve symlinks
   before stripping the root prefix — matching how `git mv` and `git commit` expect paths.

3. **Idempotency probe.** Globs `archive/*-<pad>-*.md`. If any match exists, logs a one-liner and
   exits 0 — the file is already archived; nothing to do.

4. **Locate the active file.** Globs `active/<pad>-*.md`. Exactly one match is required; any other
   count is a fatal error.

5. **Dated move.** `mkdir -p archive/` then `git mv active/<pad>-<slug>.md archive/<date>-<pad>-<slug>.md`.

6. **Frontmatter mutation.** Mutates the moved file in place with a portable `sed` one-pass
   (`set_field`), scoped to the first `---…---` frontmatter block:
   - `status:` set to `done` or `killed`.
   - `updated:` set to `--date`.
   - `results:` set to `--results` (only when `--outcome done` and `--results` is non-empty).
   - For `--outcome killed`: appends `\n## Why killed\n\n<reason>` to the file body.

7. **Change-file-only commit.** `git commit … -- <src_rel> <dest_rel>` pins the commit explicitly
   to the two halves of the rename. `git mv` pre-staged both; the `--` path list prevents any
   other staged content from sneaking into the commit. This is the key invariant (see Invariants).

8. **Push with rebase-retry.** `cas_push` loops `git push`; on non-fast-forward it retries
   `git pull --rebase` then pushes again. The rebase-retry means concurrent archivers converge:
   the second push lands tree-identically because both staged the same bytes.

9. **Fail-closed self-verification.** Checks: active file gone, archive file present, `status`
   frontmatter equals `--outcome`, `updated` equals `--date`, `results` set (if applicable),
   and `git rev-parse @` equals `git rev-parse origin/<branch>` (push landed).

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Archived successfully — or already archived (idempotent no-op). |
| non-zero | Any validation failure, `git mv` failure, commit failure, rebase failure, or postcondition violation. The run is aborted; callers must not continue. |

Usage/flag errors (`--help`) also exit 0 (help printed) or non-zero (unknown flag).

## Invariants

- **Change-file-only commit.** The explicit `-- <src_rel> <dest_rel>` path list on `git commit`
  ensures no other staged content (board updates, spec files, ADR changes) is bundled into the
  archive commit. This is the LEARNINGS #26 guard: `git mv` pre-stages both the old and new path;
  the `--` pins the commit to exactly those two paths. The `active/<pad>-<slug>.md` source path is
  always included even though it is gone after the move — git requires both halves of a rename in
  one commit.

- **Concurrent-safe.** `--date` must be derived from the manifest by the caller. Two concurrent
  archivers (finalize + sweep) produce the same `git mv` target, the same frontmatter bytes, and
  therefore an identical tree. The second push CAS-resolves cleanly via the rebase-retry loop.

- **`pwd -P` symlink resolution.** `REL` is computed with `cd "$CHANGES_DIR" && pwd -P` before
  stripping the worktree root — required on macOS where `mktemp` gives `/var/…` but `git
  rev-parse` gives `/private/var/…`. Without this, path-stripping produces a wrong relative path
  and `git mv` / `git commit` target the wrong file.

- **Metadata working tree only.** `--changes-dir` must point inside the metadata working tree
  (`.docket/` in docket-mode, primary working tree in main-mode). The script never touches the
  integration branch's working tree.
