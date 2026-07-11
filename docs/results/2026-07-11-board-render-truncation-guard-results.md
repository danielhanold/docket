# board-refresh.sh non-empty guard — results
Change: #0060 · Branch: feat/board-render-truncation-guard · PR: <set-on-open> · Plan: docs/superpowers/plans/2026-07-11-board-render-truncation-guard.md · ADRs: none

## Verify (human)

No interactive checks required — the change is fully covered by `tests/test_board_refresh.sh` #11 (empty render leaves BOARD.md byte-identical, exits non-zero, no temp leak, stderr names the failure) and the full suite is green. The finalize `local` gate re-runs the suite post-rebase.

- [x] Automated: full `tests/test_*.sh` suite green (SUITE rc=0); board-refresh #11 verifies the guard end-to-end.

## Findings

- **This change was rescoped ~90% smaller by the reconcile pass.** Its original spec proposed adding a `render-board.sh --out` atomic-write mode and migrating every Board-pass call site off the `> BOARD.md` redirect. Change **#0059** (board-refresh-surface-gate, PR #64) merged first and delivered exactly that — a new `scripts/board-refresh.sh` that owns the atomic write and to which every call site was already migrated; the `> BOARD.md` redirect no longer exists anywhere. Both incidents in the original "Why" (#0052 `/dev/null` misdirection, #0055 unknown-flag → exit 2 → truncation) are therefore already structurally prevented. The reconcile folded #0060 down to the single sub-case #0059 left unimplemented: the **non-empty** half of the spec's success test (`exit 0 AND [ -s tmp ]`). See the change's `## Reconcile log`.
- **The defensive branch is verified unreachable by the real renderer.** `render-board.sh` unconditionally emits `# Backlog\n\n` at the top of every clean run, so a legitimate exit-0 render is never empty — the new guard only fires on a future render-board regression or an injected `RENDER_BOARD` stub. It is pure defense-in-depth (belt-and-suspenders companion to #0059's existing exit-code guard, test #9), with no false-positive risk on real boards.
- **Exit code `1` is shared with the existing `mv`-failure branch** (whole-branch review, Minor). Harmless and within the spec's accepted design: the spec required `1` be distinct from usage `exit 2` and a propagated renderer code (it is), and every caller treats any non-zero identically as "skip the commit." Distinct stderr messages disambiguate in logs. Not machine-distinguishable by code alone — noted so no future caller assumes `1` uniquely means "empty render."

## Follow-ups

None. (Machine-distinguishable skip-reason exit codes were considered and left out as YAGNI, consistent with the spec rejecting format-level validation.)
