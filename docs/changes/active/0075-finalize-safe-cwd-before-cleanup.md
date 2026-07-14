---
id: 75
slug: finalize-safe-cwd-before-cleanup
title: Finalize from a durable checkout — don't run cleanup while CWD is the feature worktree
status: proposed
priority: medium
created: 2026-07-14
updated: 2026-07-14
depends_on: []
related: [76]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

When `docket-finalize-change` runs while the agent's CWD / Cursor workspace is the feature worktree under `.worktrees/<slug>`, step 4 (`cleanup-feature-branch`) correctly removes that worktree — and thereby deletes the agent's working directory mid-run. Cleanup is already in finalize scope (terminal close-out + provenance guard under `.worktrees/` only); the gap is operational posture: finalize should perform merge/metadata/cleanup from a durable checkout, not from inside the feature worktree about to be removed.

## What changes

Ensure finalize's merge, metadata updates, and feature-branch cleanup run from a durable checkout — the primary integration checkout or the `.docket/` metadata worktree — so removing `.worktrees/<slug>` cannot yank the agent's CWD out from under the run.

## Out of scope

- Changing whether cleanup is part of finalize (it already is)
- Broadening cleanup beyond the `.worktrees/` provenance guard
- Redesigning the finalize consent / merge-authorization model

## Open questions

- Should the skill refuse to start (or self-`cd`) when CWD is under `.worktrees/`, or always switch to a known durable root before step 4?
- Preferred durable root: primary integration checkout vs `.docket/` metadata worktree?

## Auto-groom blocked

**2026-07-14** — `docket-auto-groom` designed this stub to a full, critic-verified spec and then
abstained at emission. The design is settled; the **scope boundary between #75 and #76 is not**, and
that boundary is a human's call.

### Why it abstained — the blocker

Change **#76** (`cwd-independent-repo-root-resolution`) was minted **while this groom was running**
and explicitly claims the script-level root-resolution fix, scoping *out* "finalize's
durable-checkout posture and the cleanup-deletes-my-CWD hazard — that is #75." But the two are
entangled at the seam, and #75 cannot be specified without deciding who owns what:

- The durable-root posture (#75's core) needs a **`REPO_ROOT`** literal in the Step-0 `preflight`
  block — that is an edit to the resolver, i.e. #76's surface.
- `cleanup-feature-branch.sh`'s root bug is *caused* by CWD-derived root resolution (#76's stated
  root cause) but lives in the script #75 owns.

A spec emitted now = build-ready = the autonomous builder implements #76's declared scope inside
#75. **Kill, defer, and re-scoping are never autonomous**, so the drain stopped here rather than
redraw a boundary a human is actively drawing.

### What a human should supply

Pick one:

1. **Split** — #76 owns the resolver (main-worktree anchor + `REPO_ROOT` + the preflight ensure
   step); #75 narrows to the skill posture + `cleanup-feature-branch.sh`'s guard, and gains
   `depends_on: [76]`. *(Recommended — it matches #76's own stated division.)*
2. **Merge** — kill #76 and let #75 carry the whole root-anchor fix (the design below is already
   whole and covers both halves).
3. **Invert** — #76 carries everything including cleanup; #75 narrows to the skill-body posture only.

Then flip `auto_groomable: true` and **delete this section** to re-arm.

### Findings — verified against the code, on throwaway fixtures (keep these regardless of the split)

Three defects, one root cause: `git rev-parse --show-toplevel` / `cd "$REPO_DIR" && pwd -P` return
the *linked worktree* they stand in, not the repo. Two scripts already solve this with the
main-worktree idiom — `sync-integration-branch.sh:40-51` (with a comment saying exactly why) and
`disable-worktree-hooks.sh:34`.

- **D1 — `cleanup-feature-branch.sh` deletes the REMOTE branch, then fails. (Unreported data loss;
  worse than this stub's premise.)** From a linked-worktree CWD the relative `target` (line 33) never
  exists, so the removal block (37-44) is skipped, `git branch -D` fails into `|| true`, and
  execution **reaches `git push --delete` (line 49), which succeeds** — then the postcondition (54)
  dies. Measured, same repo+slug:

  | CWD | rc | worktree | local branch | remote branch |
  |---|---|---|---|---|
  | main root | 0 | removed | deleted | deleted (the only path the tests cover) |
  | `.docket/` | **1** | survives | survives | **DELETED** |
  | `.worktrees/<slug>` | **1** | survives | survives | **DELETED** |

  So today's failure is a *partial, irreversible destructive action* that reports failure. Every
  existing cleanup test invokes from the main root, so the class is untested.

- **D2 — `preflight` from a feature worktree mints a nested metadata worktree.** `docket-config.sh`
  absolutizes `METADATA_WORKTREE` **only in the `plain` format** (288-295); `docket_preflight`
  (`lib/docket-preflight.sh:20-22`) evals the **shell** format, where it is still relative, then runs
  a bare `git worktree add "$wt"` with **no `-C`** (32-35). On a clone with no `.docket/` yet, this
  creates a real `…/.worktrees/<slug>/.docket` worktree and prints `BOOTSTRAP=PROCEED`, exit 0.
  (With the local `docket` branch already checked out it instead fails closed — clone-state-
  dependent.) This is #76's bug, observed independently of its 0073 sighting. The same relative value
  feeds `docket-status.sh`'s `mw=` sites (the **Board pass**) and `render-change-links.sh:43-52`.

- **D3 — cleanup deletes the agent's own CWD.** `git worktree remove --force` *succeeds* with a
  process CWD inside the target (the process merely orphans its CWD), but the agent's **next** Bash
  call cannot start (`cd: no such file or directory`). Steps 5 (board) and 6 (sync) then die with an
  opaque `chdir` error **after** the destructive step landed. No script can fix this — a child cannot
  change its parent's CWD. **This half is irreducibly skill-side**, which is the strongest argument
  that #75 must survive the split in some form.

### Decisions already settled (and adversarially checked) — reusable by whoever owns them

- **The durable root is the MAIN worktree, not `.docket/`.** Forced, not a preference: in `main`-mode
  there *is* no `.docket/` (`docket-config.sh:166` → `METADATA_WORKTREE=.`), and `.docket/` is itself
  a linked worktree — the exact shape that misresolves. (Answers this stub's open question #2; #76
  independently reached the same conclusion.)
- **Cleanup should refuse (fail-closed) when the caller's CWD is at or inside the target**, placed
  before *both* the worktree removal and the remote delete — capturing `caller_pwd` **before** any
  `cd` (a `cd` first would compare `$root` to itself and the guard could never fire). Given D1, the
  refusal takes away nothing that works: it converts a partial irreversible destruction into a clean,
  recoverable stop. (Answers open question #1: refuse at the destructive step — *not* at skill start,
  which would also block the gate's suite run, which legitimately belongs in the feature worktree.)
- **`REPO_ROOT` must be emitted in `plain` format ONLY.** `ensure-claude-settings.sh` sets its own
  `REPO_ROOT` (line 24), `eval`s the shell config (33), and reads it after (38, 74) — emitting it in
  shell silently captures that name. And skills must **not** derive it as `dirname
  $METADATA_WORKTREE`: in `main`-mode `METADATA_WORKTREE` *is* the repo root, so `dirname` yields the
  repo's **parent**.
- **Anchoring the resolver is NOT behavior-preserving for a subdirectory** (today `cd $REPO_DIR &&
  pwd -P` yields `<subdir>/.docket`, reads `<subdir>/.docket.local.yml`, and `--bootstrap` seeds
  `<subdir>/.gitignore`). Those all move to the root. Intended fixes — but they are behavior changes
  and must be stated, not denied.
- **`archive-change.sh:53` is correct as-is** — its `git -C "$CHANGES_DIR" rev-parse --show-toplevel`
  resolves the worktree of the *passed* changes dir (the metadata worktree), which is what it wants.
  Do **not** over-apply the idiom to it.
- **Do not make the facade (`docket.sh`) `cd`** — it would silently re-resolve every caller-supplied
  relative path argument across all 11 wrapped ops.
- **Landmine if the resolver is anchored:** `docket-status.sh:363-370`'s artifacts-refresh
  `add`/`commit`/`push` block is **currently dead** (its pathspec matches nothing under a relative
  `$mw`), so the regenerated `## Artifacts` block on an archived change is silently never committed.
  An absolute `$mw` brings it to life **for the first time** — including a newly-reachable
  `sweep-failed … push-failed` early-`return` that abandons `terminal-publish` *and* `cleanup`. Budget
  and test for it.
- **Test-surface delta** (whoever anchors the resolver): `test_docket_config.sh:43,66` pin the
  relative shell values; `test_docket_preflight.sh:16,22,27,42` (fixtures) **and `:52`** (an assert)
  pin `.docket`; `test_docket_status.sh:168,220,257,875,920,1020` feed a relative `mw` (they won't
  break, but they leave the absolute path — where the `:363` landmine lives — unexercised);
  `test_render_board.sh:989` is a **source-text sentinel** pinning the literal `mw=` line, so leave
  those lines textually untouched.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- 2026-07-14 (auto-groom) — Two of this stub's premises are wrong and should be corrected whenever it
  is re-scoped: (1) "step 4 correctly removes that worktree" — from a linked-worktree CWD, cleanup
  does **not** remove it; it deletes the *remote* branch and exits 1 (D1). (2) "the primary
  integration checkout **or** the `.docket/` metadata worktree" — `.docket/` is **not** a safe durable
  root (D2); the primary checkout is the only one.
