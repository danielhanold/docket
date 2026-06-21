# terminal-publish.sh — copy terminal records from the metadata branch onto the integration branch

## Purpose

`terminal-publish.sh` is the single executor of both publish shapes in docket-mode: it copies a
change's terminal records (or a single ADR file) from `origin/<metadata_branch>` onto the
integration branch, via a transient worktree, with a CAS push. It is the only sanctioned flow of
docket metadata onto the code line.

All terminal transitions invoke this script — `docket-finalize-change` (for `done` changes),
killing skills (`docket-new-change`, `docket-implement-next`) for `killed` changes, and
`docket-adr` for standalone or status-changed ADRs. None of those skills restate the mechanics;
they all delegate here.

`BOARD.md` is never published. Plan/results/code already arrive via the PR (`done`) or do not
exist (`killed`).

## Usage

Exactly one of `--id` or `--adr` is required (they are mutually exclusive).

**Change publish** (done or killed):

```bash
terminal-publish.sh \
  --id N \
  --outcome done|killed \
  --integration-branch B \
  --metadata-branch M \
  --changes-dir REL \
  --adrs-dir REL \
  [--message MSG] \
  [--remote R]
```

**ADR-only publish** (standalone or supersession/reversal ADR from `docket-adr`):

```bash
terminal-publish.sh \
  --adr NN \
  --integration-branch B \
  --metadata-branch M \
  --changes-dir REL \
  --adrs-dir REL \
  [--message MSG] \
  [--remote R]
```

`--remote` defaults to `origin`. `--message` defaults to
`docket(<pad>): publish terminal record (<outcome>)` (change) or
`docket(adr-<NN>): publish ADR-<NN>` (ADR-only).

Both `--id` and `--adr` must be integers; a non-integer value is rejected immediately, before any
git work.

`--outcome` is required and validated (`done` or `killed`) only in change (`--id`) mode.

Mock seam: `GIT="${GIT:-git}"`.

## Behavior

### Mode guard

When `--metadata-branch` equals `--integration-branch` (i.e., `main`-mode), the script logs a
no-op message and exits 0 immediately. In main-mode there is no separate metadata branch to copy
from — the working tree is the integration branch, so the archive move is itself the terminal
record.

### Publish shapes and copy-set assembly

**Change publish (`--id`, token `T = <id>`):**

The script resolves the archived change file for `<id>` from
`origin/<metadata_branch>:<changes_dir>/archive/` — matching the pattern
`/YYYY-MM-DD-<pad>-<slug>.md`. It then reads the archived change manifest to extract:

- `spec:` field — included in the copy-set if non-empty.
- `adrs:` list — each ADR in the list is checked against `origin/<metadata_branch>:<adrs_dir>`.
  An ADR is included in the copy-set **only if its `status:` field is `Accepted`** on the metadata
  branch (the Accepted gate). `Proposed` or draft ADRs are silently skipped with a log message.

The copy-set for a change publish is therefore:
1. The archived change file (always present).
2. The `spec:` file (iff `spec:` is non-empty in the manifest).
3. Each `adrs:` entry that passes the Accepted gate.

`BOARD.md` is never in the copy-set.

**ADR-only publish (`--adr`, token `T = adr-<NN>`):**

The copy-set is the single ADR file matching `/<pad>-<slug>.md` in `<adrs_dir>` on
`origin/<metadata_branch>`. There is no Accepted gate — the ADR is copied regardless of status.
The archive step is skipped entirely.

### Generic mechanics (both shapes)

These steps execute after the copy-set is assembled:

1. **Fetch the metadata remote tip** — `git fetch <remote> <metadata_branch>`. All subsequent
   reads use `<remote>/<metadata_branch>` (the remote ref), never a stale local ref.

2. **Provision a transient integration checkout** — creates a temp dir outside the repo (so no
   `.gitignore` entry or `.worktrees/` slug collision), then:

   ```bash
   pub="$(mktemp -d)/pub"
   git worktree prune                                         # clear any leaked registration
   git worktree add -B "pub-<T>" "$pub" origin/<int_branch>  # -B: reset-or-create (re-run safe)
   ```

   `-B` adopts a leaked `pub-<T>` branch/registration left by a prior interrupted run.

3. **Copy terminal records from the metadata remote tip** — inside `pub`:

   ```bash
   git -C "$pub" fetch <remote> <metadata_branch>
   git -C "$pub" checkout <remote>/<metadata_branch> -- <copyset...>
   ```

   If the index has changes (bytes differ from integration HEAD), commit:

   ```bash
   git -C "$pub" diff --cached --quiet || git -C "$pub" commit -m "<message>"
   ```

   The guarded commit makes a no-op re-run safe: when the bytes already match, no commit is
   created and the push loop simply confirms HEAD is already on `origin/<int_branch>`.

4. **CAS push with fast-forward-or-retry loop** — pushes `HEAD:<int_branch>` explicitly (a bare
   push would resolve the source to the stale local `refs/heads/<int_branch>`, never the publish
   commit on `pub-<T>`):

   ```bash
   until git -C "$pub" push <remote> HEAD:<int_branch>; do
     git -C "$pub" pull --rebase <remote> <int_branch> \
       || { git -C "$pub" checkout <remote>/<metadata_branch> -- <copyset...>;
            git -C "$pub" rebase --continue; }
   done
   ```

   The inner `|| { re-copy; rebase --continue }` handles the case where a concurrent push landed
   the same files with different bytes: re-copying the authoritative metadata bytes resolves the
   conflict and the rebase continues.

5. **Self-verify (fail-closed)** — after the push succeeds, re-fetches `origin/<int_branch>` and
   asserts every path in the copy-set is present on the remote tip. Exits non-zero if any path is
   missing.

6. **Teardown** — detaches HEAD in `pub`, force-removes the worktree, deletes the `pub-<T>`
   branch, and removes the temp dir. Then asserts the worktree registration is gone (a final
   postcondition guard).

## Exit codes

- `0` — the full copy-set landed on `origin/<integration_branch>` and the worktree was torn down
  cleanly. Callers should **trust the exit code**: non-zero means abort-and-report.
- Non-zero — any failure: bad arguments, missing archived change file, missing ADR file, fetch
  failure, worktree provision failure, copy failure, commit failure, push failure, or postcondition
  assertion failure. The script is fail-closed and never exits 0 unless the self-verify passes.

## Invariants

**Idempotency and re-run safety:**
- The mode guard exits 0 immediately on repeated calls in main-mode.
- `-B` + `worktree prune` adopt a leaked `pub-<T>` branch/registration from a prior interrupted
  run, so re-provisioning succeeds.
- The guarded copy+commit (`diff --cached --quiet ||`) is a no-op when bytes already match; the
  push loop then completes an interrupted push.
- A `docket-status` sweep that races `docket-finalize-change` on the same change is therefore a
  safe no-op: the copy-set bytes are already present, the commit is skipped, and the push is a
  fast-forward no-op.

**Script-wins rule:** The archive step (step 1 of the change-publish path) is performed by
`archive-change.sh` **before** this script is called — archive-first ordering is load-bearing
because this script reads the archived path from `origin/<metadata_branch>`. This script does not
archive; it only copies.

**`BOARD.md` is never published** — not in the copy-set, not in any retry path.

**Accepted gate fires at copy time** — an ADR that is `Proposed` at the moment the copy-set is
assembled is excluded, even if it was `Accepted` at claim time.

**No local branch switches** — all work happens inside the transient `pub` worktree; the main
working tree's current branch is never changed.
