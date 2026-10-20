# The run-cancel worktree test waits for the launch to finish before it cancels — design

Change 0569 — Fix the flaky run-cancel worktree test

Change #569, groomed 2026-10-20. Found during #562's build gate.

## The problem

`TestIntegrationRunCancelFreesWorktreeForNextStart` (`internal/app/runtracker_cancel_integration_test.go`)
failed once in #562's full parallel suite and then passed 3 of 3 alone. The failure was the first
assert: `runCancel` returned `cancellation-pending`, with a `claim-busy:<drive>` finding, where the
test wants `cancelled`.

The test does this today:

1. `startRunDrive` runs the build drive's `Start` on a goroutine, with the suite command `sleep 60`.
2. `awaitRunPhase(t, runRoot, "running")` polls the run's `manifest.json` until its phase reads
   `running`.
3. It checks the supervisor holds the worktree lock, then calls `runCancel` once and expects
   `cancelled` plus `replacement-stopped:<drive>`.
4. It waits (bounded) for the worktree lock to come free.

**Cause (traced in code).** Starting a drive takes the drive's launch claim (the claimant flock on
the drive's `relaunch.lock`) in `revalidateAdmittedLaunch` and holds it across the launch. In
`gatedrive.launchScopeless` the order is:

1. `d.proc.Launch(...)`. The supervisor writes `manifest.json` with phase `running`, then sends the
   `running` handshake, and `Launch` returns when it reads the handshake.
2. `d.store.attachLaunch(...)`, an fsync'd record write, then the holder note.
3. `claim.close()`. The claim is released here, before the drive's observation slice.

The test polls the manifest file directly, so it can see `running` at step 1, before the launcher
reaches step 3. A cancel that lands in that window finds the claim busy (`reconcileRunDrive` returns
`claim-busy:<id>`), and `runCancel` correctly reports `cancellation-pending`. Under the parallel
suite, fsync contention stretches steps 1–2, so the window grows. That matches the failure showing up
only in a full run.

**The product is right; the test's precondition is too weak.** ADR-0132 names `claim-busy` as one of
the causes of `cancellation-pending`, and the documented remedy is to re-run the cancel. The test
means to say "a run whose drive is running and attached cancels in one call". Its wait only proves
"the suite process has started".

## The fix

Test-only. No production code changes.

After `awaitRunPhase(..., "running")` and before `runCancel`, the test also waits until the launch
is attached and its claim is released. The wait uses the same bounded-deadline, short-poll shape as
the test's other waits, never a fixed sleep.

**Suggested signal (builder may pick another one that is just as deterministic).** The production
read-side census, `productionCancelSeams(fx.repo).launchObserver.observe(fx.contextHash)` (it wraps
`gatedrive.Driver.VerdictRunLaunches`). While the launch is in flight it reports
`claim-busy:<id>`. Once the claim is released (the run dir is attached by then) it reports
`run-live:<id>`. Wait until the findings contain `run-live:<id>` and not `claim-busy:<id>`.

- This census stops and signals nothing. The only thing it ever settles is a *proven
  never-launched* first launch, and it reaches that path only while the claim is free and no run dir
  is attached. Once the manifest says `running`, the launcher already holds the claim (or has
  attached and released it), so that path can't be reached here. Keep the order: poll only *after*
  `awaitRunPhase(..., "running")`.
- Another acceptable signal is probing the drive's claim lock directly. Either way, the signal must
  mean "the launcher has released its claim". "The run dir is recorded" is not enough, because
  `attachLaunch` happens before `claim.close()`, so that wait still leaves a gap.

**What the test keeps asserting, unchanged:** one `runCancel` call; disposition `cancelled`; the
`replacement-stopped:<id>` finding; then the bounded wait for the worktree lock to come free.

**Not allowed:** retrying the cancel, accepting `cancellation-pending`, lengthening a timeout as the
fix, or removing or loosening any existing assert.

## Proving the fix

1. **Make the failure happen on demand first.** Temporarily (never committed) add a delay of a few
   hundred milliseconds in `launchScopeless` between `attachLaunch` and `claim.close()`. The
   *unchanged* test must fail with `cancellation-pending` / `claim-busy`.
2. **Same delay, fixed test:** it passes.
3. Remove the delay. Confirm with `git diff` that no production file changed.
4. Run the fixed test alone, repeated (for example `-count=50`), and run the full suite once at the
   build gate.

Record steps 1–3 (the delay used, the red message, the green re-run) in the results file.

## Sweep for the same shape

Look for other tests that cancel a run, or count on the census being settled, right after seeing a
run reach `running` while its `Start` goroutine may still be mid-launch. Find them with a
repo-wide grep (`awaitRunPhase`, `runCancel`, `ReconcileRunLaunches`, `VerdictRunLaunches`,
`"running"` waits in `_test.go` files), not by listing this file alone. Fix any that turn up the same
way, or say in the results that there were none.

A sweep at grooming time found none. The only other `awaitRunPhase(..., "running")` caller,
`TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled` (its `vanished` case), waits for
`Start` to return (`awaitStart`) before it cancels, and `Start` returns only after `claim.close()`.
The other census and cancel tests use seeded drive records or synchronous starts, or interleave
with a launch on purpose using fakes (`driver_concurrency_test.go`, `driver_launch_claim_test.go`).
Re-run the grep at build time in case anything new landed.

## Out of scope

- Changing how cancel, the census, or the launch claim behave (ADR-0132).
- Any other flaky test. A later one gets its own change.
- Slow tests and budget lines (the budget report owns those).
