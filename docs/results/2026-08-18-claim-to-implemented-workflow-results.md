<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0315 — Claim-to-implemented agent workflow](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0315-claim-to-implemented-workflow.md)**
<!-- docket:backlink:end -->

# Claim-to-implemented agent workflow — results
Change: #315 · Branch: feat/claim-to-implemented-workflow · Plan: docs/superpowers/plans/2026-08-17-claim-to-implemented-workflow-plan.md · ADRs: 59, 66, 83, 92, 94, 95

## Verify (human)

- [ ] Confirm the whole suite is green on a quiet machine — this run reported `rc=0` (all files `notok=0`) but under heavy machine contention (load avg ~89 from concurrent suite runs), so the wall-clock `OVER BUDGET` advisories on this run are contention noise, not real budget regressions. Nothing in the run failed.
- [ ] Re-measure `tests/test_go_toolchain.sh` serially on a quiet machine with the standalone protocol (`scripts/run-tests.sh -j 1 --timings PATH tests/test_go_toolchain.sh`) and act on the budget row only from that reading — see Follow-ups.

## Findings

- The final build-gate remediation greened five items reported red at the whole-suite gate:
  - **Boundary guards (workspace, evidence):** the inward import-boundary tests now allowlist exactly `internal/app` + `internal/cli`, mirroring the already-landed `githubcli` pattern (commit 0c2f1bb4). Every other landed package stays guarded; the allowlist keys on the package directory itself (a hypothetical `internal/app/foo` subpackage is still guarded).
  - **Per-harness `docket-plan-writer` goldens:** regenerated (`go test -update`) for the Task 13 asset revision across claude/codex/cursor/opencode; only that artifact changed.
  - **`TestEvidenceRecordRefusals/vanished_dir`:** NOT a source bug. It passes green (`reason=gate-vanished`) when the suite runs with the sandbox disabled, as required. The prior `probe-blocked` reading was a sandbox artifact — under the sandbox, `process.Observe`'s lock-identity probe cannot prove process facts (`observe.go` "live lock unprovable" → `FailBlocked` → `probe-blocked`). Reclassifying `probe-blocked` → `gate-vanished` in `evidence_ops.go` was deliberately NOT done: it would mask a genuine unprovable/blocked probe, defeating a correct fail-closed guard. Independently reproduced green by the reviewer.
  - **Shell test staleness owed by the design (`test_docket_config`, `test_skill_handoff_precedence`):** change 0315 replaced Step 7's `SKILL_FINISH` role invocation with the direct `docket workspace publish` + `docket pr publish` operations, so `docket-implement-next` no longer delegates a finish role. Removed the now-false "implement-next finish uses SKILL_FINISH" assertion (finalize retains it) and lowered the handoff-precedence genuine-invocation floor from 4 to 3 (plan, build, review). These are corrections of assertions whose premise the design removed, not test weakenings.

## Follow-ups

- **Budget re-measurement for `tests/test_go_toolchain.sh`.** This change grows that file's `go test ./...` (new app/cli packages + real-git integration + e2e). Its row is currently `55` (parallel) with a hard `60` ceiling; no new `tests/test_*.sh` file was added, so no new budget row is owed. The whole-suite gate reported this file `OVER BUDGET`, but that run was under heavy contention (every file, including a 52s-vs-10s-ceiling file, tripped OVER BUDGET), so the number is not a valid measurement. Take one clean serial reading on a quiet machine; if the worst reading's next-multiple-of-5-plus-5 exceeds 60, the table remedy is to SHARD the file, not raise the ceiling.
