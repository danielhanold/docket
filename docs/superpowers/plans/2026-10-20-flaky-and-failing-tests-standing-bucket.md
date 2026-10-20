<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0569 — Fix the flaky run-cancel worktree test](../../changes/active/0569-flaky-and-failing-tests-standing-bucket.md)**
<!-- docket:backlink:end -->

# Fix the flaky run-cancel worktree test — Implementation Plan

> **For agentic workers:** executed by `docket-build`, task by task, under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `TestIntegrationRunCancelFreesWorktreeForNextStart` waits until the launcher has released its launch claim before it calls `runCancel`, so its one cancel deterministically reports `cancelled`.

**Architecture:** Test-only. After the existing `awaitRunPhase(t, runRoot, "running")`, poll the production stop-free launch census (`productionCancelSeams(fx.repo).launchObserver.observe(fx.contextHash)`, which wraps `gatedrive.Driver.VerdictRunLaunches`) with a bounded deadline and short poll until it reports `run-live:<id>` and no `claim-busy:<id>`. Prove the fix by forcing the race with a temporary, never-committed delay in `gatedrive.launchScopeless`.

**Tech Stack:** Go (`internal/app` integration tests, real process supervisor).

**Spec:** `docs/superpowers/specs/2026-10-20-flaky-and-failing-tests-standing-bucket-design.md`

## Global Constraints

- Test-only. No production code changes; `git diff <base>..HEAD -- ':!*_test.go'` is empty at the end.
- The wait is a bounded deadline with a short poll (same shape as the test's other waits: `time.Now().Add(...)` deadline, `time.Sleep(10 * time.Millisecond)` poll), never a fixed sleep as the synchronization.
- The signal must mean "the launcher has released its claim" — "the run dir is recorded" is not enough (`attachLaunch` runs before `claim.close()`).
- Poll the census only AFTER `awaitRunPhase(..., "running")` (keeps the census off its never-launched settle path).
- Keep every existing assert unchanged: one `runCancel` call; disposition `cancelled`; finding `replacement-stopped:<id>`; then the bounded wait for the worktree lock to come free.
- Not allowed: retrying the cancel, accepting `cancellation-pending`, lengthening a timeout as the fix, removing or loosening any existing assert.
- Cross-references in comments anchor on symbol names (`launchScopeless`, `claim.close`), never line numbers (ADR-0054).
- Any manual or repeated `go test` run uses `-count=1` (or `-count=N`) so the result cache cannot serve a stale tree.

## Review Focus

1. **The census itself settles or stops something.** `VerdictRunLaunches` stops nothing; it settles only a proven never-launched first launch, reachable only when the claim is free and no run dir is attached. Ordering the poll after `awaitRunPhase("running")` keeps it unreachable — the task's red/green proof (cancel still reports `replacement-stopped:<id>`) pins that the drive is still live when cancel runs.
2. **The wait passes on a weaker signal than claim release.** A wait keyed on `attachLaunch` alone still races. Pinned by the forced-delay proof: the delay sits between `attachLaunch` and `claim.close()`, so only a claim-release signal goes green.
3. **The wait wedges on an unexpected finding** (e.g. `resolution-unresolved:<id>`, or the census returning an error). The helper's deadline failure message prints the last report's findings and error, so a wedge fails loudly and diagnosably instead of hanging.
4. **The old test was not actually exposed to the race.** Pinned by the red step: the unchanged test under the delay must fail with `cancellation-pending` and a `claim-busy:<id>` finding, not some other failure.
5. **Other tests cancel the same way.** Covered by Task 2's repo-wide sweep.

---

### Task 1: Wait for the launch claim to be released before cancelling, proven red/green on demand

**Files:**
- Modify: `internal/app/runtracker_cancel_integration_test.go` — add helper `awaitLaunchSettled` next to `awaitRunPhase`/`awaitStart`; call it in `TestIntegrationRunCancelFreesWorktreeForNextStart`.
- Temporary only (never committed, never staged): `internal/gatedrive/driver.go`, inside `(*Driver).launchScopeless`, between the `d.store.attachLaunch(...)`/`t.lock.WriteHolder(...)` block and `claim.close()`.

**Interfaces:**
- Consumes (all existing): `productionCancelSeams(repoDir string) cancelSeams` (field `launchObserver runLaunchObserver`, method `observe(contextHash string) (gatedrive.RunLaunchReport, error)`); `gatedrive.RunLaunchReport{Accounted bool; Findings []string}`; `hasFinding(findings []string, prefix string) bool`; `onlyDriveID(t, fx) string`; `cancelFixture.repo`, `cancelFixture.contextHash`.
- Produces: `func awaitLaunchSettled(t *testing.T, fx cancelFixture, id string)` in `internal/app/runtracker_cancel_integration_test.go`.

- [ ] **Step 1: Force the race and watch the UNCHANGED test go red**

Copy `internal/gatedrive/driver.go` to a backup outside the tree first (restore from the copy, not `git checkout`, if you have other uncommitted edits). Add, temporarily, immediately before the non-deferred `claim.close()` in `launchScopeless`:

```go
time.Sleep(400 * time.Millisecond) // TEMP #569 race proof — never commit
```

(add the `time` import if `driver.go` lacks it). Run:

`go test ./internal/app/ -run '^TestIntegrationRunCancelFreesWorktreeForNextStart$' -count=1 -v`

Expected: FAIL at the first assert — `disposition = "cancellation-pending", want cancelled (findings=[... claim-busy:<id> ...])`. Copy the exact failure line for the task report. If it does not fail this way, stop and report NEEDS_ESCALATION (the groomed cause is a hypothesis; do not proceed on an unproven race).

- [ ] **Step 2: Add `awaitLaunchSettled` and call it**

Helper contract: poll `productionCancelSeams(fx.repo).launchObserver.observe(fx.contextHash)` until `err == nil`, `hasFinding(report.Findings, "run-live:"+id)` and `!hasFinding(report.Findings, "claim-busy:"+id)`; deadline 30 seconds, poll every 10 ms; on deadline `t.Fatalf` naming the last findings and error. Doc comment says why: the manifest reaches `running` inside `d.proc.Launch`, but `launchScopeless` holds the drive's launch claim through `attachLaunch` until `claim.close()`, and a cancel in that gap correctly reports `cancellation-pending` (ADR-0132); the census reports `claim-busy` while held and `run-live` once released, and it stops nothing.

In the test, move `id := onlyDriveID(t, fx)` up if needed and insert `awaitLaunchSettled(t, fx, id)` after the worktree-lock precondition check and before `runCancel`. Leave every other line of the test as is.

- [ ] **Step 3: Same delay, fixed test — green**

Run: `go test ./internal/app/ -run '^TestIntegrationRunCancelFreesWorktreeForNextStart$' -count=1 -v`
Expected: PASS. Record for the report.

- [ ] **Step 4: Remove the delay, confirm no production file changed**

Restore `internal/gatedrive/driver.go` from the backup. Run `git status --porcelain` and `git diff --stat`.
Expected: only `internal/app/runtracker_cancel_integration_test.go` is modified.

- [ ] **Step 5: Repeat the fixed test without the delay**

Run: `go test ./internal/app/ -run '^TestIntegrationRunCancelFreesWorktreeForNextStart$' -count=50`
Expected: `ok` (50 of 50 pass).

- [ ] **Step 6: Commit**

```bash
git add internal/app/runtracker_cancel_integration_test.go
git commit -m "test(app): run-cancel worktree test waits for the launch claim to be released before cancelling"
```

In the task report, include for the results file: the delay used (400 ms before `claim.close()` in `launchScopeless`), the red failure line from Step 1, the green result from Step 3, the `git diff --stat` from Step 4, and the `-count=50` result.

### Task 2: Sweep for tests with the same shape

**Files:**
- Read: all `*_test.go` in the repo. Modify only a test that turns up with the same shape (none expected).

**Interfaces:**
- Consumes: `awaitLaunchSettled` from Task 1 (for any test fixed the same way).
- Produces: nothing new.

- [ ] **Step 1: Run the repo-wide grep**

Run from the worktree root:

`grep -rn -E -e 'awaitRunPhase|runCancel\(|ReconcileRunLaunches|VerdictRunLaunches|"running"' --include='*_test.go' .`

For each hit, classify: does it cancel a run, or rely on the census being settled, right after seeing `running` while its `Start` goroutine may still be mid-launch (i.e. before `awaitStart` returns or a claim-release wait)? Seeded drive records, synchronous starts, deliberate fake-driven interleavings (`driver_concurrency_test.go`, `driver_launch_claim_test.go`), and callers that `awaitStart` first (the `vanished` case of `TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled`) are not the shape.

- [ ] **Step 2: Fix any match the same way, or record none**

For a match: insert `awaitLaunchSettled` (Task 1) before its cancel/census call, run it with `-count=1`, and commit with message `test(<pkg>): <test> waits for the launch claim to be released`. If none match, make no commit and state in the task report "sweep: no other test cancels or reads the census mid-launch" with the hit count classified.

The full suite runs once at the build gate (`build.test_command` from config), owned by `docket-build`.
