---
id: 270
slug: fence-runner-config-locality-at-the-main-worktree
title: 'Fence runner-config locality at the main worktree (regression test + contract correction)'
status: proposed
priority: medium
type: chore
created: 2026-08-08
updated: 2026-08-09
depends_on: []
related: [208]
discovered_from: [269]
adrs: []
spec: docs/superpowers/specs/2026-08-09-fence-runner-config-locality-at-the-main-worktree-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-fence-runner-config-locality-at-the-main-worktree-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-fence-runner-config-locality-at-the-main-worktree-design.md) |
<!-- docket:artifacts:end -->

## Why

**Origin — a defect report whose premise was false.** This change was filed as
*"machine-local runner config is unreachable from a feature worktree"*, inheriting a claim from
change 0269's spec `## Out of scope`: that `runner-dispatch.sh` resolves `runners.<name>.permissions`
from the `--worktree` anchor, so a `permissions: auto-approve` grant written in the gitignored
`.docket.local.yml` would be invisible to a `build-*` delegation and the opencode adapter would
refuse.

It does not. `runner-dispatch.sh` anchors its config-layer loop at `docket_main_worktree()` — the
main worktree, never the anchor — and has since the loop was written (change 0079; change 0206 only
moved the *`DOCKET_REPO_ROOT` export* to the anchor). The adapters open no config files at all;
`runners/opencode.sh` reads the already-exported `DOCKET_RUNNER_CFG_PERMISSIONS` and uses
`DOCKET_REPO_ROOT` solely as `--dir`. An empirical probe (grant in a main worktree only, dispatch
from a sibling feature worktree, env-dumping fake adapter) exported `auto-approve`; an adversarial
pass over the non-reproduction — nested dispatch, XDG / `DOCKET_HARNESS_ROOT`, hand invocation of
the adapter, `--launch` detachment, worktree-list ordering — found no path either. The human who ran
0269 confirms no dispatch ever refused: the claim was written from reading the code.

**Why not simply kill it.** A competent author read this code and concluded the opposite, and the
repo made that easy in two places. `tests/test_runner_dispatch.sh` tests worktree anchoring and
config-layer precedence in two blocks that never cross, so nothing pins "a main-worktree grant
survives a feature-worktree dispatch" — the invariant could be broken by an ordinary refactor with
the suite green. And `scripts/runner-dispatch.md` step 3 writes the config paths as
`<repo>/.docket.local.yml` immediately after step 2 has defined the anchor as possibly a feature
worktree, with `<repo>` never bound; `scripts/runners/opencode.md`'s env table then names
`DOCKET_REPO_ROOT` as "the main worktree unless the caller named a feature worktree" and introduces
`runners.opencode.permissions` on the very next row without saying where *it* is read from.

The decoupling is load-bearing, not incidental: `.docket.local.yml` is gitignored, so anchoring the
config loop at `--worktree` would silently drop every machine-local runner grant on exactly the
`build-*` dispatches that require `--worktree`. An invariant that important should not rest on
nobody refactoring it.

**Scope, therefore:** fence the invariant with a regression test and correct the two contracts that
invited the misreading. No production code changes.

## What changes

Settled design in the linked spec; the shape:

- **`tests/test_runner_dispatch.sh`** gains one section: a **real** linked worktree
  (`git worktree add`, not a `mkdir` — a plain subdirectory makes the assert vacuous), the grant
  written to the main worktree's `.docket.local.yml` only, and a dispatch issued from *inside* the
  worktree with `--worktree` set. Three asserts: the grant reaches the child; the anchor handed to
  the adapter **is** the linked worktree; and it is **not** the main worktree. The second and third
  are the anti-vacuity pair — without them a regression that anchored config at `--worktree` *and*
  let the anchor fall back would stay green.
- **Mutation test, mandatory:** anchor the config loop at `$ANCHOR`, and separately export
  `DOCKET_REPO_ROOT="$REPO_ROOT"`. Each must redden its assert; both reverted before commit;
  results recorded.
- **`scripts/runner-dispatch.md`** — step 3 binds the config tree explicitly to the main worktree,
  states that it is independent of `--worktree`, and gives the gitignore reason.
- **`scripts/runners/opencode.md`** — the `DOCKET_RUNNER_CFG_PERMISSIONS` env-table row and the
  Prerequisites bullet each say which tree the grant is read from.

## Out of scope

- Editing change 0269's spec — a merged, point-in-time build record. Its stale claim is corrected
  here, not rewritten there.
- Any change to `runner-dispatch.sh` or the adapters. The invariant is already correct.
- The `--worktree` gates themselves (ADR-0034 anchoring, the `build-*` requirement, gate 3's
  membership test) — change 0208.
- `runners:` config-reader duplication — change 0256.
- Whether `permissions: auto-approve` behaves as documented against a real opencode binary; already
  flagged **Unverified** in `opencode.md`, and an external truth no in-repo test can be an oracle
  for.

## Coupling

Change **0208** edits the same two files (`scripts/runner-dispatch.sh`'s tests and the adapter
contracts) on a disjoint concern — input gates — and its spec already converts the existing
`mkdir` worktree fixtures to real `git worktree add`. A `related:` coupling, not a dependency:
#0270 stands alone. Whichever lands second rebases and prefers the other's fixture helper.
