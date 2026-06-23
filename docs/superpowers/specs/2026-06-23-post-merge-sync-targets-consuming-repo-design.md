# Design — Post-merge integration sync targets the consuming repo, not the docket clone

Change: #0041 · slug `post-merge-sync-targets-consuming-repo` · spec drafted 2026-06-23 (interactive groom)

## Problem

`sync-integration-branch.sh` (change 0029) fast-forwards a clone's local
`<integration_branch>` checkout after a docket merge, so the skills symlinked from that
checkout stop drifting behind `origin/<integration_branch>`. Its `--clone-dir` defaults to
**`dirname "$0"/..`** — the repo the *script physically lives in*.

In a **separate-clone install** (ADR-0014) docket lives in its own clone and consuming repos
reach its scripts via `DOCKET_SCRIPTS_DIR` and its skills via symlink. There that default
resolves to the **docket clone**, never the consuming repo. So when finalize (or the
`docket-status` sweep) runs in a consuming repo, the bare invocation
(`sync-integration-branch.sh --integration-branch <ib>`, no `--clone-dir`):

- fast-forwards the **docket** clone's integration branch — a no-op, since a consuming-repo PR
  merge never advances docket's own `origin`; and
- never advances the **consuming repo's** integration checkout — the one the merge actually
  pushed to — which silently drifts (markhaus, 2026-06-23: the end-of-run sync reported `main
  already current (b5c5902…)`, a *docket* object, while markhaus's local `main` stayed 6
  commits behind `origin/main`).

The two call sites invoke the helper bare and have no way to retarget it.

## Decision

Two coordinated changes, both inside `sync-integration-branch.sh`; the call sites stay bare.

### 1. Retarget the default to the invoking repo's main worktree

Change the script's default `--clone-dir` from "the repo the script lives in"
(`dirname "$0"/..`) to **"the main worktree of the repo the script was invoked from" (CWD)**.
Explicit `--clone-dir` still overrides.

The resolution must find the **main worktree** — not merely `git rev-parse --show-toplevel`,
which at the sync site returns the *linked* worktree the skill's shell happens to be in.
Recommended mechanism: the first entry of `git -C "$PWD" worktree list --porcelain`
(equivalently, the parent of `git -C "$PWD" rev-parse --path-format=absolute --git-common-dir`).
git always lists the main worktree first; it is reachable from any linked worktree in the set,
independent of CWD. If CWD is not inside a git repo, the existing not-a-repo gate still applies
(best-effort skip, exit 0).

This keeps the deterministic git resolution in the script (ADR-0012 / ADR-0007: the script owns
*how*, the skill owns *when*), requires **no change to the skill call sites**, and preserves
dogfooding unchanged (when docket runs on itself, CWD's main worktree *is* the docket primary
checkout, so the resolved dir equals the old `dirname "$0"/..`).

**Why `git rev-parse --show-toplevel` is wrong (empirically verified, 2026-06-23).** The sync
runs at the very end of the finalize run (step 6) / after the status sweep — *after* the board
step, which operates in the `.docket/` metadata worktree. The shell CWD there is typically a
**linked worktree** (`.docket/`, on `docket`), not the primary checkout. Run from inside
`.docket/`:

| probe | result |
|---|---|
| `git rev-parse --show-toplevel` | `…/docket/.docket` — the linked worktree, **on branch `docket`** |
| `git worktree list --porcelain` (first) | `…/docket` — the primary checkout, **on `main`** |

So the change body's first-named shape — `--clone-dir "$(git rev-parse --show-toplevel)"` —
would target `.docket` on the `docket` branch; gate 1 (must be on `<integration_branch>` =
`main`) then skips, and the bug is *not fixed*. Main-worktree resolution makes the retarget land
regardless of which linked worktree the shell sits in.

### 2. Keep gate 2 conservative, but make its skip loud and diagnostic

The triple gate is **behaviorally unchanged** — gate 2 still skips when `git status --porcelain`
is non-empty, so *untracked non-ignored* files (markhaus's `design/`) and dirty tracked files
both continue to block the fast-forward. This conservatism is deliberate: the post-merge sync
never fast-forwards over a non-pristine tree.

What changes is the **note**, not the gate. Today gate 2's skip emits a generic
`working tree not clean — skipping (no fast-forward onto local edits)`, which surprises a user
who only has untracked files and assumes those are harmless. Make the skip **self-explanatory**:
the note must (a) state that **untracked (non-ignored) files also block** the fast-forward, and
(b) give the remedy — clean or `.gitignore` the untracked paths (or commit/stash tracked edits)
to let the auto-FF run. Including a short summary of the offending entries (a count, or the
first few `git status --porcelain` lines) is encouraged but the exact wording/verbosity is a
build-time call; the two required facts are the untracked-blocks statement and the remedy.

**This does not auto-advance a dirty consuming repo** (markhaus must still clean/gitignore
`design/`), by design — the author chose conservatism over auto-clobbering. The loud note is the
agreed mitigation: the skip is diagnosable instead of a silent drift.

## What changes (build-time scope)

- **`scripts/sync-integration-branch.sh`**
  - When `--clone-dir` is unset, resolve it to the main worktree of the invoking repo (CWD)
    instead of `dirname "$0"/..`. Explicit `--clone-dir` continues to override.
  - Rewrite the gate-2 skip note to name untracked files as a blocker and state the remedy
    (per Decision 2). Gate logic (the `git status --porcelain` non-empty test) is unchanged.
- **`scripts/sync-integration-branch.md`** — update the `--clone-dir` default in Usage and the
  Invariants (main-worktree-of-CWD resolution and why), and the gate-2 description (note wording
  is now explicit about untracked files + remedy; behavior unchanged).
- **`tests/test_sync_integration_branch.sh`** (the de-facto CI gate per ADR-0014) — add two
  cases: (a) a **bare** invocation (no `--clone-dir`) from inside a *linked worktree* of a
  fixture repo whose main worktree is clean and on the integration branch, asserting the **main
  worktree** is the one fast-forwarded; (b) a dirty/untracked-tree skip asserting the **new
  explicit note** (untracked-blocks + remedy), so the "louder note" is locked. Keep the existing
  hermetic cases (they pass explicit `--clone-dir`, unaffected by the default change).
- **Skill call sites** (`docket-finalize-change` step 6, `docket-status` sweep) — **no change**;
  they stay bare and inherit the corrected default. (A one-clause refresh to the convention's
  Branch-model sentence describing the sync may be folded in if the implementer finds the current
  wording misleading; not required for correctness.)

No change to *which* sites run the sync, nor to the gate **conditions** (only gate 2's note).

## Resolved design decisions

The interactive brainstorm settled these; recorded so the implementer's reconcile reuses rather
than reopens them.

1. **Retarget belongs in the script's default, not a skill-passed `--clone-dir`.** Rejected
   passing `--clone-dir "$(git rev-parse --show-toplevel)"` (empirically buggy — resolves a
   linked worktree on the wrong branch → gate 1 skips) and rejected doing both (redundant). The
   skill's *where* is already its CWD; deterministic git plumbing belongs in the script
   (ADR-0012 / ADR-0007). Smallest root-cause fix; no skill prose changes; tests pass explicit
   `--clone-dir` so they are unaffected.
2. **Resolution targets the main worktree** (`git worktree list --porcelain` first entry ≡
   `--git-common-dir` parent), CWD-independent — not `--show-toplevel`. Degrades safely: if a
   consuming repo's primary checkout is not on the integration branch, gate 1 skips (a note,
   never a mistarget).
3. **Gate 2 stays conservative; only its note gets louder** (owner decision, 2026-06-23). The
   sync never fast-forwards over untracked or dirty trees; a consuming repo with stray untracked
   files (markhaus's `design/`) will still skip — but now with a self-explanatory note and
   remedy, so it is not a silent surprise. Rejected: relaxing gate 2 to ignore untracked files
   (would auto-advance over a non-pristine tree); rejected: a pure no-op leaving the skip silent.
4. **Refreshing the docket *skills* clone stays out of scope** — the separate "update docket"
   workflow (change 0029's out-of-scope); skills load from the docket clone regardless.
5. **Dogfooding preserved with no special-casing** — CWD's main worktree equals the old
   `dirname "$0"/..` when docket runs on itself.

*Dependency state:* `depends_on: []`; `related: [29, 34]` — the origin of the script (0029) and
of `DOCKET_SCRIPTS_DIR` / the separate-clone model (0034 / ADR-0014), both `done`.

## Out of scope

- Keeping the docket **skills** clone fresh in consuming repos (decision 4).
- Auto-restart or fixing an in-flight session (0029's out-of-scope stands).
- Changing *which* sites run the sync, or the gate **conditions** (only gate 2's note text
  changes; the clean-tree test is unchanged).
- Auto-advancing a consuming repo over untracked/dirty files (decision 3 keeps that blocked).
