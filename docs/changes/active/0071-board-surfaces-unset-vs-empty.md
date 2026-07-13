---
id: 71
slug: board-surfaces-unset-vs-empty
title: An unresolved $BOARD_SURFACES is indistinguishable from a deliberately disabled board
status: proposed
priority: high
created: 2026-07-13
updated: 2026-07-13
depends_on: []
related: [59, 69, 70]
adrs: [28]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0028](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0028-report-channel-is-not-a-board-surface.md) |
<!-- docket:artifacts:end -->

## Why

Every docket skill's Board pass invokes `board-refresh.sh --changes-dir … --surfaces "$BOARD_SURFACES"`. If `$BOARD_SURFACES` is not set in the shell that runs that command, the quoted expansion produces `--surfaces ""` — and `board-refresh.sh` reads an empty value as **"no surfaces configured"**: it prints `inline disabled — no-op` and exits 0. The board is not refreshed. The caller's `git status --porcelain -- BOARD.md` is then empty, which the skill prose explicitly licenses as "a genuine no-op," so the caller commits nothing and proceeds believing the board is current. It isn't. The board silently goes stale, with a success exit code the whole way down.

This is not the wiring bug change 0059 defended against. That one — *forgetting the flag* — is caught: `board-refresh.sh` tracks `SURFACES_SET` and exits 2 on a missing `--surfaces`, and its header comment calls out that the two cases are "tracked separately." But a *present flag carrying an unresolved variable* lands in the empty-value branch, which is the legitimate `board_surfaces: []` path. `docket-config.sh:190` maps `[]` to exactly `BOARD_SURFACES=""`, so a disabled repo and a mis-wired caller are byte-identical at the script boundary.

The reason this is worth a change rather than a caution is that the harness makes the mis-wiring *likely*, not exotic. Shell state does not persist between Bash tool calls: an agent that runs the Step-0 `eval "$(docket-config.sh --export)"` in one call and its Board pass in a later call has an **unset** `$BOARD_SURFACES` by the time it matters. This was hit live while filing change 0070 — the board pass no-op'd, and only an explicit check of the output caught it.

This is the same defect class change 0069 just finished fixing on the report channel: an exit-0 no-op that is indistinguishable from success. ADR-0028 drew the line between a report channel and a board surface; this is the board surface exhibiting the very failure the report side was hardened against.

## What changes

Make "the caller failed to resolve the config" distinguishable from "this repo has no board surfaces," so the first fails loudly and the second stays a clean no-op.

The two states are genuinely different and both are legitimate — the fix is to stop encoding them as the same value. Whatever ships must keep `board_surfaces: []` a silent, non-truncating no-op (that is change 0059's whole point and must not regress) while making an unresolved caller impossible to ignore.

## Out of scope

- Changing `board_surfaces: []` semantics. A repo that disables the board keeps a no-op that never truncates a prior `BOARD.md`.
- The `github` surface and `github-mirror.sh`. Same caller pattern, but that surface is best-effort by design; scope this to the `inline` write decision.
- Retrofitting stale boards. Any board left stale by this bug self-heals at the next Board pass that *is* correctly wired.

## Open questions

- **Where does the fix belong — the export, the script, or the prose?** Three candidates, not exclusive:
  - **Positive sentinel:** have `docket-config.sh` emit something like `BOARD_SURFACES=none` for `[]` instead of the empty string. Then empty is *always* a wiring bug and `board-refresh.sh` can exit 2 on it. Costs a token change every caller and test must agree on.
  - **`${BOARD_SURFACES?}` in skill prose:** the unset-only form (not `:?`) fails loudly when the variable was never exported, but passes a deliberately-empty value straight through — which maps onto exactly the distinction we need, since a `[]` repo exports it *set-but-empty*. Cheapest fix; relies on every call site being written correctly, which is the thing that just failed.
  - **`board-refresh.sh` re-resolves config itself** rather than trusting a caller-passed value, removing the seam entirely. Most robust, but it puts a config read inside a script the convention currently describes as taking the caller's already-resolved tokens verbatim.
- Should the skills' Step-0 preamble state that the config `eval` must be re-run in **any** shell that invokes a docket script, or is per-call re-resolution too noisy to be followed in practice? The `DOCKET_SCRIPTS_DIR` precedent is instructive: it survives across calls only because `install.sh` injects it into the harness `env`, which is precisely why its `:?` guard never fires and `$BOARD_SURFACES`'s absence does.
- Does the same unresolved-variable hazard exist for other exported config the skills pass into scripts by value (`$CHANGES_DIR`, `$ADRS_DIR`, `$INTEGRATION_BRANCH`)? Those tend to fail loudly (a script given an empty `--changes-dir` exits 2), but this should be checked rather than assumed — a silent-empty-default anywhere else is the same bug.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
