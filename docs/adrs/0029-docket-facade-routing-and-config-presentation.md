---
id: 29
slug: docket-facade-routing-and-config-presentation
title: docket facade — routing-boundary dispatch and model-ward config presentation
status: Accepted
date: 2026-07-13
supersedes: []
reverses: []
relates_to: [12]
change: 68
---

## Context

Change 0068 adds `scripts/docket.sh`, a finite-operation executable facade, so
docket's runtime command surface is a finite allowlist (permission classifiers
like Cursor's can approve it) and so resolved config reaches the model as
literals on stdout rather than through an eval'd shell round-trip. Three points
in the design spec were under-determined and had to be resolved during
implementation; a future maintainer could "correct" them wrongly without this
record.

## Decision

1. **Dispatch is a pure routing boundary.** The 11 wrapped helper operations
   forward their arguments verbatim and `exec` the helper (exit code + stderr
   unmasked); they do NOT each run the shared preflight internally. The shared
   preflight (`scripts/lib/docket-preflight.sh`, `docket_preflight`) is realized
   in exactly two places: the `preflight` verb (the sanctioned Step-0 / mid-run
   re-sync) and `docket-status.sh` (which was refactored to reuse it, replacing
   its private `ensure_and_sync_worktree`). Rationale: making every metadata op
   self-preflight would contradict the binding "routing boundary, not a second
   implementation" constraint, double-sync after the agent's Step-0 `preflight`,
   and misfire for primary-tree ops (`sync-integration-branch`,
   `cleanup-feature-branch`) that do not operate in the metadata worktree. This
   preserves the pre-0068 contract where the caller syncs `.docket` at Step-0.

2. **`env`/`preflight` emit raw `KEY=value` (never eval'd by an agent),
   absolutizing only `METADATA_WORKTREE`.** The `*_DIR` keys (`CHANGES_DIR` /
   `ADRS_DIR` / `RESULTS_DIR`) stay repo-relative subpaths because their correct
   absolute root differs by consumer — `CHANGES_DIR` / `ADRS_DIR` compose against
   the metadata worktree, `RESULTS_DIR` against a feature worktree — so a blanket
   absolutization would mislead at least one. This narrows (does not drop) the
   spec's "path-valued keys are absolute": the cwd-dangerous worktree root is
   absolute; the dirs are composed one step (`$METADATA_WORKTREE/$CHANGES_DIR`)
   against an absolute root.

3. **The raw presentation lives in the resolver as `docket-config.sh --format
   plain`, selected by the facade.** `--format shell` (`%q`, eval-able) stays the
   default and is byte-unchanged for existing `eval "$(docket-config.sh
   --export)"` callers. Keeping the ordered key list in one place (the resolver)
   avoids a second, drift-prone copy in the facade; "the facade owns the
   presentation" is realized as "the facade owns the selection of the plain
   presentation."

## Consequences

- The runtime command surface is a finite, documented allowlist:
  `scripts/docket.md`'s subcommand table IS the permission inventory, guarded by
  a grep-derived, mutation-tested sentinel that asserts docket.sh ↔ docket.md
  op-set parity AND that the dispatch `case` contains only the known arms (no
  hand-added routable op), plus that there is no `run` / `exec` / `shell` / `eval`
  escape-hatch op and docket.sh never calls `eval`.
- Wrapped metadata operations assume the caller ran `preflight` at Step-0 (a
  synced metadata tree); the facade does not enforce this, and does not re-sync
  per op.
- Nothing docket emits is eval'd or sourced by an agent; the only `eval` is the
  resolver's own `%q` output inside the shared preflight lib (the pre-existing
  trust model, unchanged).
- Follow-up change 0072 (skill / Step-0 rewiring) will consume `preflight` +
  literal interpolation and compose metadata paths from the absolute
  `METADATA_WORKTREE` plus the relative `*_DIR` subpaths. Cursor documentation is
  0073.
