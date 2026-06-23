---
id: 41
slug: post-merge-sync-targets-consuming-repo
title: Post-merge integration sync fast-forwards the docket clone, not the consuming repo where the merge landed
status: proposed
priority: medium
created: 2026-06-23
updated: 2026-06-23
depends_on: []
related: [29, 34]
adrs: []
spec: docs/superpowers/specs/2026-06-23-post-merge-sync-targets-consuming-repo-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-06-23-post-merge-sync-targets-consuming-repo-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-06-23-post-merge-sync-targets-consuming-repo-design.md) |
<!-- docket:artifacts:end -->

## Why

The post-merge integration sync (`sync-integration-branch.sh`, change 0029) exists to
fast-forward "the clone's local `<integration_branch>` checkout to the tip the merges just
pushed," so the primary checkout does not drift after a docket merge. It works when docket
dogfoods itself — there the consuming repo *is* the docket clone, so the script's default
target (`--clone-dir` = `dirname "$0"/..`, the clone the script physically lives in) is the
same repo the merge landed in.

In a **separate-clone install** — the model ADR-0014 established, where docket lives in its
own clone and consuming repos reach its scripts via `DOCKET_SCRIPTS_DIR` and its skills via
symlink — that default resolves to the **docket clone**, never the consuming repo. So when
finalize (or the `docket-status` sweep) runs in a consuming repo:

- it fast-forwards the **docket** clone's integration branch — a no-op, because a
  consuming-repo PR merge never advances docket's own `origin`; and
- the **consuming repo's** integration checkout — the one the merge actually pushed to — is
  never advanced, so it silently drifts behind `origin/<integration_branch>`.

The finalize and sweep sites invoke the helper bare (`sync-integration-branch.sh
--integration-branch <integration_branch>`, no `--clone-dir`), so they always inherit that
default and have no way to retarget it.

**Evidence (markhaus, 2026-06-23).** Finalizing change #53 merged PR #39 to markhaus's
`origin/main`. The end-of-run sync reported `main already current (b5c5902…)` — but
`b5c5902` is the *docket* clone's `main` tip, not a markhaus object; markhaus's local `main`
stayed 6 commits behind `origin/main`. Change 0029's spec foresaw this edge ("Consumer-repo
skill freshness … the skills clone is a separate repo a project merge doesn't advance") but
scoped it out, leaving the consuming repo's own integration checkout unhandled.

## What changes

Settled at grooming (2026-06-23) — see the [spec](../../superpowers/specs/2026-06-23-post-merge-sync-targets-consuming-repo-design.md).
Two coordinated changes, both inside `sync-integration-branch.sh`; the skill call sites stay
bare and inherit the fix.

- **Retarget the default.** Change the script's default `--clone-dir` from `dirname "$0"/..`
  (the repo the script lives in) to **the main worktree of the repo it was invoked from (CWD)**,
  resolved via `git worktree list --porcelain` (the first entry — CWD-independent, so it works
  even though the shell sits in the linked `.docket/` worktree at the sync site). Explicit
  `--clone-dir` still overrides; the hermetic tests pass it, so they are unaffected. Keeps the
  deterministic git resolution in the script (ADR-0012 / ADR-0007). Dogfooding is unchanged —
  on docket-on-docket, CWD's main worktree equals the old `dirname "$0"/..`.
- **Keep gate 2 conservative, make its skip loud.** The triple gate is behaviorally unchanged —
  untracked-non-ignored and dirty-tracked files both still block the fast-forward (the sync
  never FFs over a non-pristine tree). What changes is gate 2's **note**: it must state that
  untracked files also block the FF and give the remedy (clean or `.gitignore` them), so a
  consuming repo with stray untracked files (markhaus's `design/`) sees a self-explanatory skip
  instead of a silent drift.

Preserves the best-effort, FF-only posture: every skip stays a normal exit-0 that never aborts
the close-out.

## Out of scope

- **Keeping the docket *skills* clone fresh in consuming repos** — that is the separate
  "update docket" workflow change 0029 named; skills load from the docket clone regardless of
  the consuming repo's checkout. This change concerns the consuming repo's *own* integration
  checkout, not skill bytes.
- Auto-restart or fixing the in-flight session (0029's out-of-scope stands).
- Changing *which* sites run the sync, or the gate **conditions** — only gate 2's skip *note*
  text changes; the clean-tree test itself is untouched.
- Auto-advancing a consuming repo over untracked/dirty files — the gate deliberately keeps that
  blocked (a perpetually-dirty primary checkout still skips, now with a diagnostic note).

## Open questions

_All resolved at grooming (2026-06-23); recorded in the spec's "Resolved design decisions":_

- Retarget belongs in the **script's default** (not a skill-passed `--clone-dir`).
- Resolve the **main worktree** (`git worktree list --porcelain`), not `git rev-parse
  --show-toplevel` (empirically buggy here).
- The clean-tree gate **stays conservative; only its skip note gets louder** — no carve-out for
  untracked files (owner decision).
- A consuming-repo finalize does **not** also refresh the docket skills clone (separate workflow).

## Reconcile log
