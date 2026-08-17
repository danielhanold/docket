<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0314 — Native process supervisor and local gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-17-0314-native-process-supervisor-and-local-gate.md)**
<!-- docket:backlink:end -->

# Native process supervisor and local gate — results

Change: #314 · Branch: feat/native-process-supervisor-and-local-gate · PR: <set on open> · Plan: docs/superpowers/plans/2026-08-16-native-process-supervisor-and-local-gate.md · ADRs: 95 (supersedes 81), 80, 87

## Verify (human)

- [ ] **Linux real-process execution.** The `internal/process` suite exercises real launch / observe / stop / recover against live PIDs, sessions, and process groups. It was executed and is green on **Darwin** (this build host). Linux was covered only by cross-compilation (`GOOS=linux go build ./...` / `TestCrossCompileApprovedTargets`), which proves the `SYS_GETSID` spelling and build tags but is **not execution evidence** (spec §Testing requires both targets). Run `go test -count=1 ./internal/process/` on a Linux host before merge, or confirm CI does so.

## Findings

- **ADR-0095 minted, superseding ADR-0081.** The Go per-run supervisor establishes a genuine new session (`setsid`) on Darwin and Linux and reads the raw child wait status, eliminating both ADR-0081's per-platform narrowing and the shell-level `128+signal` exit/signal ambiguity — with no discovered interpreter, second executable, or daemon. ADR-0080 (delegated-agent boundary) and ADR-0087 (clean-absence vs unprovable-liveness) are unchanged and explicitly not superseded.
- **Review (docket-review-deep) returned 4 findings — all fixed in-branch, no blockers.** See the PR body's disposition table for per-finding commit SHAs:
  - important — `pgid<=1` was unguarded in the group liveness/signal primitives (`groupAlive`/`getPGID`/`signalGroup`): a `0` pgid addressed the caller's own group, making a `PGID:0` leaked-allocation slot unmarkable and leaving a latent self-`SIGKILL` gated only non-locally by `identityConjunction`. Now fails closed (`probeUnknown` / typed error) for `pgid<=1`.
  - minor — the 128-bit ownership `Token` is written but read by no clause; comments corrected to describe it as reserved/unused (field retained for schema stability).
  - minor — added a guard test pinning the no-leak invariant (argv / env / child-output bytes never reach protocol `Reason`/`HumanText` or `failure.json`; run-dir logs may contain them). Bite-check reddened as expected under a temporary sentinel interpolation.
  - minor — documented that `recover`'s `classifyRun` deliberately omits `failure.json` (left for human inspection, unlike `observe`).

- **Build gate — runtime budgets.** Full resolved suite (`scripts/run-tests.sh`) is green (`SUITE_EXIT=0`, all files `notok=0`). Change 314's own runtime-budget rows — `test_go_toolchain`, `test_go_race`, and the new `test_go_race_process` shard (`EXPECTED_TOTAL` 2080 → 2115: +25 new shard, +10 toolchain re-budget) — were **within budget** on both suite runs. The pre-fix full run reported **zero** `OVER BUDGET` rows. The post-fix full run reported 7 `OVER BUDGET` rows, but **all 7 are unrelated shell/config suites** (`test_board_checks`, `test_docket_config`, `test_harness_defaults_validator`, `test_sync_agents`, `test_sync_agents_codex_dispatch`, `test_sync_agents_defaults`, `test_sync_agents_drift_docs`) — none touched by this change, each over by a similar factor. That whole-suite cliff on a similar factor across unrelated rows, absent in the pre-fix run of the same suite, is the machine-saturation signature (`scripts/run-tests.md`): the wall-clock number is machine-dependent and does not fail the run. No budget was touched; a `-j 3` re-run on an unloaded host is expected to clear it. **This is not a code-level regression in change 314.**

## Follow-ups

- None. Downstream work (workflow relaunch/repair and the claim→implemented transition, finalize recovery + physical cleanup, release packaging + harness acceptance, Bash removal/cutover) already exists as stubs 0315–0318 per the spec's scope allocation; nothing new surfaced during reconcile or review.
