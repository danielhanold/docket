# Design: fast-forward the local integration branch after a docket merge (change 0029)

**Status:** design (brainstormed 2026-06-20)
**Change:** 0029
**Depends on:** — (extends finalize + the status sweep; no new dependency)
**Motivated by:** the close-out of change 0026 and the post-mortem recorded in the kill of change 0028.
**Precedent:** the close-out scripts (0025) for the helper pattern; `github-mirror.sh` (0011) for the **best-effort, never-abort** posture this helper takes.
**ADRs:** 0007 (script-owns-*how* / skill-owns-*when*).

---

## 1. Context — the post-mortem

Closing out change 0026, `docket-finalize-change` hand-rolled the archive and dropped a
`status: done` edit onto `main`. The investigation (recorded in 0028's `## Why killed`)
found the trigger was **not** a missing rewire — 0025's close-out-script rewire shipped
in full — but a **stale skills source**: `~/.claude/skills/*` are absolute symlinks into
the docket clone's working tree, and that tree (the primary checkout on the integration
branch) was **39 commits / ~12 hours behind `origin/main`**, spanning the 0025 + 0026
merges. The harness loaded the pre-0025 *manual-archive* skill from the stale tree, and
running it produced the bug.

The tree had drifted because, in docket-mode, the primary checkout is **never
fast-forwarded** — all work happens in `.docket/` and feature worktrees, so the primary
tree just sits at whatever commit it was on while `origin/<integration_branch>` advances
underneath it, taking the symlinked skills with it.

**Key observation — staleness has exactly one source.** A skill's bytes on
`origin/<integration_branch>` change **only** when a PR that edited `skills/` is merged.
The only docket operations that merge PRs are **`docket-finalize-change`** and the
**`docket-status` merge sweep**. Terminal-publish (kills, ADRs) advances the integration
branch but copies only change-files/specs/ADRs — never `skills/`; the PM skills never
change skills either. So drift is *created* in exactly two places — which is where it
should be *repaired*, rather than detected everywhere after the fact.

## 2. Goal / non-goals

**Goal.** After a docket merge lands on `origin/<integration_branch>`, **best-effort
fast-forward the clone's local `<integration_branch>` checkout** so it (and the skills
symlinked from it) tracks the branch commit-by-commit. This is the standard
merge-then-sync reflex, applied at docket's two merge sites. It shrinks the
skill-staleness window to its irreducible minimum: commits that land *after* the current
session started (which nothing can fix mid-session — see §5).

**Non-goals.**
- **No Step 0 staleness guard.** Detect-and-warn at config-resolution time was considered
  and dropped: the two merge sites are the *cause*, and any drift from outside the docket
  flow self-heals at the next finalize/sweep. Revisit only if real outside-flow drift
  appears.
- **No auto-restart and no claim to fix the current run.** New skill bytes load only at
  process start; this keeps *future* sessions fresh, it cannot re-load the running one.
- **No re-link / `sync-agents.sh`.** A FF refreshes *existing* skills' content in place
  (symlinks resolve to updated bytes); a brand-new skill or agent wrapper still needs
  `link-skills.sh` / `sync-agents.sh`. Rare, orthogonal, out of scope.
- **No touching a dirty or diverged tree.** FF-only, guarded (§3); never a merge commit,
  never a checkout switch, never a stash.
- **Consumer repos (docket used on a *different* project).** There, the project repo and
  the docket *skills clone* are different repos, so a project merge does not advance the
  skills clone — keeping that clone fresh is a separate "update docket" workflow, out of
  scope. The FF still runs as ordinary merge-then-sync hygiene on the project's own
  integration branch; it simply isn't what refreshes skills there. The skill-staleness
  guarantee in §1 is specific to **dogfooding docket on itself**, which is where the bug
  occurred and where the primary checkout *is* the skills clone.

## 3. Mechanism

A small, guarded, **best-effort** helper — `scripts/sync-integration-branch.sh` — that
both merge sites invoke after their merges land:

```
sync-integration-branch.sh --clone-dir <repo-root> --integration-branch <branch> [--remote origin]
```

Behavior, in `--clone-dir`:
1. Resolve the current branch and tree state. **Skip (exit 0, one-line note) unless**
   the checkout is *on* `<integration_branch>` AND the working tree is clean.
2. Fetch `<remote>/<integration_branch>` (finalize/sweep already fetched; cheap/no-op),
   and **skip** unless `origin/<branch>` is strictly ahead with the local tip as an
   ancestor (a true fast-forward).
3. `git merge --ff-only <remote>/<integration_branch>`.

It is **best-effort like `github-mirror.sh`**, not fail-closed like `archive-change.sh`:
every skip condition (dirty tree, on a feature branch, non-FF divergence, even a fetch
failure) is *normal* and returns success with a note. It **must never abort or alter the
close-out** — the merge has already landed; this is downstream housekeeping. `--clone-dir`
defaults to the helper's own repo root (`dirname "$0"/..`).

**Call sites (both required — omitting the sweep leaves swept close-outs stale):**
- **`docket-finalize-change`** — once at the very end of a run (after the board step), so
  a batch finalize FFs once after all its merges, mirroring the single end-of-run board
  regen.
- **`docket-status` merge sweep** — once after the sweep's merges + publishes, same
  single-pass placement.

The skill prose addition is one line per site ("then `sync-integration-branch.sh …`,
best-effort"); the mechanics live in the script. `docket-convention`'s Branch model gets
a one-sentence pointer so the behavior has a single documented source.

## 4. Testing

`tests/test_sync_integration_branch.sh`, hermetic (bare origin + clone, no network), in
the `test_closeout.sh` style:
- **FF case:** clone on `<branch>`, origin advanced → helper FFs local to origin tip.
- **Dirty tree:** uncommitted change → skip, local tip unchanged, exit 0.
- **Wrong branch:** clone on a feature branch → skip, exit 0.
- **Non-FF divergence:** local has a commit origin doesn't → skip (no merge commit), exit 0.
- **Already current:** no-op, exit 0.
- **Fetch failure** (origin removed) → skip with note, exit 0 (best-effort).

## 5. Caveats (stated, not hidden)

- **Restart still required** for new skill bytes to be *loaded* by the harness; the FF
  only makes them *present on disk* for the next process. The change closes the
  drift-over-time gap, not the within-session gap (which is irreducible).
- **Only the dogfood case refreshes skills** (§2 consumer note).

## 6. Risk

Low. The helper is best-effort, FF-only, guarded against dirty/diverged/feature-branch
trees, and runs strictly *after* the merge has landed — it can keep the local tree
current but can never corrupt it or disturb the close-out. The main risk is a missed
call site (only the sweep, easy to verify) or an over-eager guard that FFs a tree the
user intentionally pinned — mitigated by the on-branch + clean + true-FF triple gate.
