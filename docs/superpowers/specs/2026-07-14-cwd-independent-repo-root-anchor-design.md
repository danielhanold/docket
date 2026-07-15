# Design: Anchor the repo root to the main worktree — CWD-independent scripts, a fail-closed cleanup guard, and a durable finalize posture (change 0075)

Date: 2026-07-14
Change: 0075-cwd-independent-repo-root-anchor
Status: approved (carries the whole root-anchor fix; change 0076 folded in and killed)

## Context

docket runs a three-worktree layout: the main checkout, the persistent `.docket/` metadata worktree
(parked on `docket`), and ephemeral `.worktrees/<slug>` feature worktrees. Every operating skill's
Step 0 runs `docket.sh preflight`, and agents, hooks, and humans all `cd` freely between the three.

But the scripts derive the repo root from **where they are standing**: `docket-config.sh` defaults
`REPO_DIR="."` (line 36) and absolutizes it with `cd "$REPO_DIR" && pwd -P` (line 290);
`cleanup-feature-branch.sh` takes `git rev-parse --show-toplevel` (line 30). Both return the
**linked worktree the caller happens to be in**, not the repo's primary checkout. A stray `cd` is
therefore enough to make a script target the wrong tree — and today, in three distinct ways.

Two scripts in this same repo already solve exactly this, with the idiom this change generalizes:

```sh
main_wt="$("$GIT" -C "$dir" worktree list --porcelain | sed -n '1s/^worktree //p')"
```

git lists the **main worktree first**, and the list is reachable from any linked worktree in the
set. `sync-integration-branch.sh:40-51` uses it and carries a comment stating precisely why
(`--show-toplevel` would return the `.docket/` worktree and its gate would skip);
`disable-worktree-hooks.sh:34` uses it too. The scripts already know how to ask git where they
are — they just don't, everywhere else.

### The three defects (verified against the code, on throwaway fixtures, 2026-07-14)

**D1 — `cleanup-feature-branch.sh` deletes the REMOTE branch, then fails. Unreported partial data
loss.** `target="$WORKTREES_DIR/$SLUG"` (line 33) is **relative to CWD**. From any linked-worktree
CWD it never exists, so the removal block (37–44) is skipped; `git branch -D` then fails (the branch
is still checked out in the surviving worktree) into `|| true`; execution **reaches
`git push --delete` (line 49), which succeeds**; and only then does the postcondition (line 54)
die. Same repo, same slug, varying only CWD:

| CWD | rc | worktree | local branch | remote branch |
|---|---|---|---|---|
| main root | 0 | removed | deleted | deleted — *the only path the tests cover* |
| `.docket/` | **1** | survives | survives | **DELETED** |
| `.worktrees/<slug>` | **1** | survives | survives | **DELETED** |

So today's failure mode is a *partial, irreversible destructive action that reports failure*. Every
existing cleanup test invokes from the main root, so the whole class is untested. Note this refutes
the original stub's premise ("step 4 correctly removes that worktree"): from a linked-worktree CWD
cleanup does **not** remove it — it deletes the remote branch and exits 1.

**D2 — `preflight` from a linked worktree mints a nested metadata worktree.** `docket-config.sh`
absolutizes `METADATA_WORKTREE` **only in the `plain` format** (287–295); `docket_preflight`
(`scripts/lib/docket-preflight.sh:20-22`) `eval`s the **shell** format, where the value is still the
relative `.docket`. It then tests `[ ! -d "$wt" ]` against CWD and runs `git worktree add "$wt"`
with **no `-C`** (31–35). Run from `<repo>/.docket` on a clone with no `.docket/` beneath it, this
creates a real `<repo>/.docket/.docket` worktree and prints `BOOTSTRAP=PROCEED`, exit 0. Observed
live during change 0073's finalize: the nested tree registered as `-docket1`, detached at a stale
commit, and a later render wrote an **empty** `BOARD.md` (`0 changes`) into it. Nothing was
corrupted — but only because the empty board landed in the stray tree rather than the real one. The
failure is silent; the debris is visible only in `git worktree list`. (With the local `docket`
branch already checked out it instead fails closed, so the behavior is clone-state-dependent.) The
same relative value feeds `docket-status.sh`'s `mw=` sites (the Board pass) and
`render-change-links.sh:43-52`.

**D3 — cleanup deletes the agent's own CWD.** `git worktree remove --force` *succeeds* with a
process CWD inside the target (the process merely orphans its CWD), but the agent's **next** Bash
call cannot start (`cd: no such file or directory`). Steps 5 (board) and 6 (sync) then die with an
opaque `chdir` error **after** the destructive step already landed. **No script can fix this** — a
child cannot change its parent's CWD. This half is irreducibly skill-side, which is why the change
carries a skill-posture component and not only a script fix.

## Settled decisions

Each was reached in the 2026-07-14 auto-groom and survived its adversarial critic pass.

- **The durable root is the MAIN worktree, never `.docket/`.** This is forced, not preferred: in
  `main`-mode there *is* no `.docket/` (`docket-config.sh:166` → `METADATA_WORKTREE=.`), and
  `.docket/` is itself a linked worktree — the exact shape that misresolves (D2 is triggered by
  running preflight from inside it).
- **Cleanup refuses fail-closed when the caller's CWD is at or inside the target**, and the guard
  sits **before both** the worktree removal and the remote delete. `caller_pwd` must be captured
  **before any `cd`** — a `cd` first would compare `$root` against itself and the guard could never
  fire. Given D1, refusing takes away nothing that works: it converts a partial irreversible
  destruction into a clean, recoverable stop.
- **The refusal belongs at the destructive step, not at skill start.** Refusing to *start* finalize
  from a feature worktree would also block the merge gate's suite run, which legitimately belongs in
  the feature worktree.
- **`REPO_ROOT` is emitted in `plain` format ONLY.** `ensure-claude-settings.sh` sets its own
  `REPO_ROOT` (line 24), `eval`s the shell config (line 33), and reads it after (lines 38, 74) —
  emitting `REPO_ROOT` in the shell format would silently capture that name.
- **Skills must not derive the root as `dirname $METADATA_WORKTREE`.** In `main`-mode
  `METADATA_WORKTREE` *is* the repo root, so `dirname` yields the repo's **parent**. They read the
  `REPO_ROOT` literal from the `preflight` block instead.
- **Anchoring the resolver is NOT behavior-preserving for a subdirectory invocation.** Today
  `cd $REPO_DIR && pwd -P` from `<repo>/sub/` yields `<sub>/.docket`, reads `<sub>/.docket.local.yml`,
  and `--bootstrap` seeds `<sub>/.gitignore`. All three move to the repo root. These are the intended
  fixes — but they are behavior changes and must be stated, not denied.
- **`archive-change.sh:53` is correct as-is.** Its `git -C "$CHANGES_DIR" rev-parse --show-toplevel`
  resolves the worktree of the *passed* changes dir (the metadata worktree), which is exactly what it
  wants. Do **not** over-apply the idiom to it.
- **Do not make the facade (`docket.sh`) `cd`.** It would silently re-resolve every caller-supplied
  relative path argument across all 11 wrapped ops.

## 1. Resolver anchor (`scripts/docket-config.sh`)

Resolve the repo from the **main worktree** rather than CWD, using the established idiom. `REPO_DIR`
keeps its `--repo-dir` override (tests depend on it); only the *default* changes — from "CWD" to
"the main worktree of the repo containing CWD", falling back to CWD when that resolution is empty
(not-a-repo, which the existing `rev-parse --is-inside-work-tree` gate at line 105 already handles).

Everything downstream of `REPO_DIR` (`g()` at line 53, `LCFG` at 133, the `--bootstrap` gitignore
seed at 278, `REPO_ABS` at 290) then anchors for free.

## 2. `REPO_ROOT` in the export block (`plain` only)

Emit `REPO_ROOT=<absolute main-worktree path>` alongside `METADATA_WORKTREE` in the `plain` format
— the format skills read as cwd-independent literals — and **not** in `shell` (see the settled
decision above). This is the literal `docket-finalize-change` uses for its durable-root `cd`.

## 3. Preflight ensure step (`scripts/lib/docket-preflight.sh`)

The D2 fix. The `eval`'d shell format keeps `METADATA_WORKTREE` relative, so preflight must anchor
it itself before use:

- Resolve the main worktree, and build the metadata-worktree path **absolute** from it.
- Pass `-C <main-worktree>` (or the absolute path) to `git worktree add` — never a bare relative
  `$wt` interpreted against CWD.
- **Guard:** refuse when the computed metadata-worktree target would land *inside* an existing
  worktree of this repo. `<repo>/.docket/.docket` is never a legitimate target, and the guard is
  what makes the operation idempotent with respect to CWD rather than merely correct from the root.

The existing `-d` existence test and the `disable-worktree-hooks.sh` / fetch / `pull --rebase` calls
all take the absolute path too.

## 4. `cleanup-feature-branch.sh` — the CWD guard

- Anchor `root` to the main worktree; build `target` **absolute** from it, so the removal block and
  the postconditions stop being CWD-relative.
- Capture `caller_pwd="$PWD"` at the top, **before any `cd`**.
- Refuse (exit non-zero, no destructive step attempted) when `caller_pwd` is at or inside `target` —
  placed **before** the worktree removal *and* before the remote delete.
- The existing `.worktrees/<slug>` provenance guard is unchanged and still governs what may be
  removed.

## 5. The `docket-status.sh:363` landmine

`docket-status.sh:363-370`'s artifacts-refresh `add`/`commit`/`push` block is **currently dead**: its
pathspec matches nothing under a relative `$mw` (`git -C "$mw" status --porcelain -- "$archived"`
with `$archived` prefixed by that same relative `$mw`), so the regenerated `## Artifacts` block on an
archived change is silently never committed. **An absolute `$mw` brings it to life for the first
time** — including a newly-reachable `sweep-failed … push-failed` early-`return` that abandons both
`terminal-publish` **and** `cleanup`.

This must be budgeted and tested as part of this change, not discovered in production: the block
must commit the refreshed artifacts (its actual intent), and its failure path must not silently
abandon the rest of the close-out sequence.

`render-change-links.sh:43-52` consumes the same relative value and gets audited in the same pass.

## 6. Finalize's durable-checkout posture (`skills/docket-finalize-change`)

The irreducibly skill-side half (D3). Finalize performs its **merge, metadata updates, and cleanup**
from the durable root — the `REPO_ROOT` literal from §2 — so that removing `.worktrees/<slug>` can
never yank the agent's own CWD out from under the run. The merge gate's suite run still happens in
the feature worktree, which is where it belongs; only the close-out steps move.

Script-side, §4's guard is the backstop: if a caller still stands inside the target, cleanup now
stops cleanly instead of destroying half of it.

## Test plan

New coverage — each is a regression test for a defect that is live today:

- **D2:** run the Step-0 preflight with CWD set to the metadata worktree; assert **no second
  worktree** is created (`git worktree list` count unchanged) and the existing `.docket/` is
  resolved.
- **D1:** invoke cleanup from each of the three CWD classes (main root, `.docket/`,
  `.worktrees/<slug>`). From the main root: unchanged happy path. From inside the target: **refuses,
  and the remote branch still exists** — the assertion that would have caught the data loss.
- **§5:** an archived change with a stale `## Artifacts` block is actually refreshed **and
  committed** by the sweep, and a failure in that block does not abandon `terminal-publish` /
  `cleanup`.
- **Subdirectory behavior change (§1):** invoked from `<repo>/sub/`, the resolver reads
  `<repo>/.docket.local.yml` and targets `<repo>/.docket` — pinning the intended new behavior.

Existing test-surface delta for whoever anchors the resolver:

- `tests/test_docket_config.sh:43,66` — pin the **relative** shell values.
- `tests/test_docket_preflight.sh:16,22,27,42` (fixtures) **and `:52`** (an assert) — pin `.docket`.
- `tests/test_docket_status.sh:168,220,257,875,920,1020` — feed a relative `mw`. They won't break,
  but they leave the **absolute** path — where the §5 landmine lives — unexercised.
- `tests/test_render_board.sh:989` — a **source-text sentinel** pinning the literal `mw=` line.
  Leave those lines textually untouched.

## Risks

- **The §5 landmine is the sharp edge**: a dead code path comes alive with an early-`return` that
  can abandon the close-out. It is the one place where "just anchor the root" has non-local
  consequences.
- **Blast radius**: every script reached through the facade resolves config, so the anchor touches
  all of them. Mitigated by the fact that the correct root is what each already intended, and by the
  subdirectory-behavior test above.

## Out of scope

- Changing whether cleanup is part of finalize (it already is).
- Broadening the `.worktrees/<slug>` provenance guard.
- Redesigning the finalize consent / merge-authorization model.
- Any change to what the metadata worktree *is*, where it lives, or the branch model.
- Automatic removal of an already-stray nested worktree (removing a worktree is destructive; the
  provenance guard deliberately refuses paths outside `.worktrees/<slug>`). Left to the human.
- Making `docket.sh` `cd` (see settled decisions).

## Provenance

The design was produced by `docket-auto-groom` on 2026-07-14 and passed its adversarial critic, but
the drain **abstained at emission**: change 0076 (`cwd-independent-repo-root-resolution`) had been
minted concurrently and claimed the script-level resolver fix, so the scope boundary between the two
was a live human decision and re-scoping is never autonomous. The human resolved it on 2026-07-14 by
**merging**: 0075 carries the whole root-anchor fix, and 0076 was killed as folded-in.
