# Truthful Git errors and harness-neutral escalation retry — results

Change: #128 · Branch: `feat/truthful-git-errors-harness-neutral-escalation-retry` · PR: not opened · Plan: [`docs/superpowers/plans/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry.md`](../superpowers/plans/2026-07-22-truthful-git-errors-harness-neutral-escalation-retry.md) · ADRs: none

## Verify (human)

- [x] Ran `scripts/docket.sh preflight` as its own normal workspace-write call. It exited 1 with `docket-config: git fetch origin failed`, followed by `error: cannot open '.git/FETCH_HEAD': Operation not permitted` and the existing preflight failure posture. This is the required neutral wrapper plus real sandbox/permission evidence; no arguments were changed.
- [x] Retried that exact command once through the host-approved execution path. It exited 0 and printed `BOOTSTRAP=PROCEED`. No shell elevation, altered arguments, or broadened sandbox was used.

## Automated verification

- Task 1 focused test: `bash tests/test_docket_config.sh` exited 0. It covers neutral-wrapper-first stderr, preservation of both fake-Git diagnostics, no stdout, and removal of the old network-specific diagnosis.
- Task 2 focused tests: `bash tests/test_docket_config.sh` and `bash tests/test_skill_facade_wiring.sh` each exited 0. The latter verifies the single harness-neutral recovery subsection and its structural restrictions.
- Mutation M1: restoring `2>/dev/null` and the old network-specific `die` message made `bash tests/test_docket_config.sh` exit nonzero; the wrapper, preserved-diagnostic, and old-text-absence assertions failed.
- Mutation M2: retaining captured stderr but removing only the wrapper `printf` made `bash tests/test_docket_config.sh` exit nonzero; the wrapper assertion failed.
- Task 2 recovery-subsection removal mutation made `bash tests/test_skill_facade_wiring.sh` exit 1, including the heading, nonempty-section, and recovery-rule assertions. Its R7 unsafe producer-to-`grep -q` reversion returned 141 under `pipefail`.
- Complete corpus command was started exactly as planned: `suite_status=0; for test_file in tests/test_*.sh; do bash "$test_file" || suite_status=1; done; exit "$suite_status"`. It advanced past the initial suites and into the later status-related fixtures without a test assertion failure, but twice stopped producing output and remained live; the executions were interrupted rather than left indefinitely. The reproducible blocker is an apparent hang in the later suite path (the live run's final emitted line was `branch 'docket' set up to track 'main' by rebasing.`). Therefore a full-suite pass is not claimed.

## Findings

No new ADR. The manual host-mediated check validated the intended boundary: only a real `.git` permission denial qualified, and the exact one-time retry reached `BOOTSTRAP=PROCEED`.

## Follow-ups

No auto-captured follow-up. Investigate the existing late-suite hang separately before treating the repository-wide corpus as green.
