---
id: 208
slug: runner-dispatch-worktree-gate-3-proves-repo-containment-not
title: runner-dispatch --worktree gate 3 proves repo containment, not worktree membership
status: proposed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: []
discovered_from: [206]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0206's whole-branch review found that `runner-dispatch.sh`'s third `--worktree` gate proves
**containment in the repo**, not **worktree membership**, so it does not reject the one value it
most needs to.

`docket_main_worktree` returns the main worktree for *any* directory inside *any* worktree of the
repo. The gate is:

    [ "$(docket_main_worktree "$ANCHOR")" = "$REPO_ROOT" ] || die "--worktree $ANCHOR is not a worktree of this repository"

so it succeeds for the main worktree itself, and for every ordinary subdirectory of it
(`<repo>/docs`, `<repo>/scripts`, ...). A `build-*` delegation whose caller supplies the repo root
therefore clears all three gates and anchors the build worker in the primary checkout on the
integration branch — precisely the failure 0206 exists to eliminate — while the diagnostic asserts
a worktree membership that was never actually checked.

This is 0206's own defect class reappearing inside the code that fixes it (learnings:
`fix-reintroduces-its-own-defect-class`): the gate makes an **omission** loud but leaves a **wrong
value** silent, and the design only accepted that trade because the value is prose-supplied.

## What changes

- Test real membership rather than containment: capture `git -C "$ANCHOR" worktree list --porcelain`
  into a variable (never pipe into `grep -q` under `pipefail`) and require an exact
  `worktree <ANCHOR>` line.
- For `build-*` specifically, reject `ANCHOR == REPO_ROOT` with a diagnostic naming the
  integration-branch hazard.
- Align the gate's wording with what it actually verifies.
- Close the paired test gap the same review raised: no assert covers the change's central success
  path — a `build-*` agent *with* `--worktree` succeeding and anchoring at the named tree. Legs (b)
  and (c) use `--agent status`; leg (d) uses `build-economy` only in its rejected state. A mutation
  making `build-*` abort unconditionally leaves the suite green.

## Out of scope

- Widening the gate to other feature-scoped agent families (its own change).
