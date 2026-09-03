---
id: 319
slug: test-bash-runtime-routing-sh-s-inventory-assert-is-cwd-depen
title: 'test_bash_runtime_routing.sh''s inventory assert is cwd-dependent — relative rg globs resolve against the process cwd'
status: 'killed'
priority: medium
type: fix
created: 2026-08-13
updated: '2026-09-03'
depends_on: []
stacked_on:
related: []
discovered_from: [304]
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

**Trigger** — surfaced while running the change 0304 suite from the feature worktree by absolute
path rather than from the repo root. `tests/test_bash_runtime_routing.sh`'s whole-repo inventory
assert reddened, and the file was confirmed byte-identical to `main` — a pre-existing defect this
change neither introduced nor touched.

**Opportunity** — the assert searches an absolute `"$REPO"` while spelling its exclusions as
relative globs (`--glob '!tests/**'`, and likewise for the `docs` and `.superpowers` globs). rg
resolves a relative glob against the **process cwd**, not the search root, so any invocation from a
directory other than the repo root leaves `tests/**` unexcluded and the assert drowns in test-file
matches. Confirmed both ways: `-j 1` from the repo root passes 25/25; the identical invocation from
another cwd reddens that one assert. The fix is the cwd-independent `!**/tests/**` form for each
glob.

**Independent value** — a suite file that only passes from one cwd is a latent false red for every
caller that launches the suite by absolute path: worktree-based gate runs, CI images with a
different working directory, and any agent-driven run. Worth fixing whether or not the Go migration
proceeds.

**Boundary** — repair the relative-glob spellings in `tests/test_bash_runtime_routing.sh` and add a
guard that the file passes from a non-root cwd. It deliberately leaves alone the runtime-routing
policy the test asserts, the budget regime, and any other suite file — unless the same relative-glob
shape is found there by a repo-wide grep, which is the natural first step.

**Reason for deferral** — 0304 established the Go executable skeleton and touched none of the Bash
runtime-routing surface; repairing an unrelated pre-existing suite file would have expanded that
branch's scope and mixed an incidental fix into a foundation-slice diff.

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — tests/test_bash_runtime_routing.sh and the routing mechanism it asserted are deleted.
