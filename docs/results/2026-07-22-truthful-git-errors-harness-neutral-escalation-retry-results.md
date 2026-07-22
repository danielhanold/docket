# Truthful Git errors and harness-neutral escalation retry — results

Change: #128 · Branch: `feat/truthful-git-errors-harness-neutral-escalation-retry` · PR: not opened · Plan: [`docs/superpowers/plans/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry.md`](../superpowers/plans/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry.md) · ADRs: none

## Verify (human)

- [x] Ran `scripts/docket.sh preflight` as its own normal workspace-write call. It exited 1 with `docket-config: git fetch origin failed`, followed by `error: cannot open '.git/FETCH_HEAD': Operation not permitted` and the existing preflight failure posture. This observed `.git` permission denial exercised exactly one host-approved retry of the unchanged command.
- [x] That one retry exited 0 and printed `BOOTSTRAP=PROCEED`. No shell elevation, altered arguments, or broadened sandbox was used; ordinary Git failures retain their existing failure behavior and do not qualify for this recovery.

## Automated verification

- Focused checks are green: `bash tests/test_docket_config.sh`, `bash tests/test_skill_facade_wiring.sh`, and `bash tests/test_skill_size_budgets.sh`. They cover the diagnostic contract, the single harness-neutral recovery rule and structural restrictions, and the convention word budget.
- Mutation M1: restoring `2>/dev/null` and the old network-specific `die` message made `bash tests/test_docket_config.sh` exit nonzero; the wrapper, preserved-diagnostic, and old-text-absence assertions failed.
- Mutation M2: retaining captured stderr but removing only the wrapper `printf` made `bash tests/test_docket_config.sh` exit nonzero; the wrapper assertion failed.
- Task 2 recovery-subsection removal mutation made `bash tests/test_skill_facade_wiring.sh` exit 1, including the heading, nonempty-section, and recovery-rule assertions. Its R7 unsafe producer-to-`grep -q` reversion returned 141 under `pipefail`.
- The default corpus command (`suite_status=0; for test_file in tests/test_*.sh; do bash "$test_file" || suite_status=1; done; exit "$suite_status"`) blocked in `tests/test_docket_status.sh` on an interactive rebase editor. This is auto-captured as follow-up #131.
- A bounded full-corpus run with `GIT_EDITOR=true` completed. It exposed the convention word-budget breach, subsequently repaired in `433abab`; `bash tests/test_skill_size_budgets.sh` is now green. The bounded corpus's remaining failure is only existing tracked #130: the BSD `grep` repetition limit in `tests/test_finalize_disposition.sh`. A final full-suite-green result is therefore not claimed.

## Findings

No new ADR. The manual host-mediated check validated the intended boundary: only the observed `.git` permission denial qualified, and its exact one-time retry reached `BOOTSTRAP=PROCEED`.

## Follow-ups

- #131 was auto-captured for the interactive-editor block in `tests/test_docket_status.sh`.
- #130 is the deduplicated existing stub for the BSD `grep` repetition-limit failure in `tests/test_finalize_disposition.sh`.
