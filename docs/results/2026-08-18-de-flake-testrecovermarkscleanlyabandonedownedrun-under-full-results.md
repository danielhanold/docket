<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0328 — De-flake TestRecoverMarksCleanlyAbandonedOwnedRun under full-suite load](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0328-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full.md)**
<!-- docket:backlink:end -->

# De-flake TestRecoverMarksCleanlyAbandonedOwnedRun under full-suite load — results

Change: #0328 · Branch: feat/de-flake-testrecovermarkscleanlyabandonedownedrun-under-full · PR: (see change file `pr:`) · Plan: docs/superpowers/plans/2026-08-18-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full.md · ADRs: none

## Verify (human)

- [ ] **Accept that the flake's trigger was never reproduced.** This branch lands a fix whose
      triggering condition could not be observed on this machine (see Findings 1). The change is
      justified as *diagnostics* — it converts a future mystery `Marked:0` into a named setup
      failure — not as a demonstrated repair. If you want a demonstrated repair instead, this
      branch should not merge as-is; the alternative is to leave the test alone and wait for the
      next occurrence to capture its disposition string.
- [ ] **Decide whether the doubled failure deadline is acceptable.** The two `waitFor` calls went
      from 30s to 60s. On a *genuine* hang (a real regression) the test now takes 60s to fail
      instead of 30s, inside `test_go_toolchain`, which is already 153s against a 55s ceiling.
      Zero cost on the happy path; the cost lands only when something is actually broken.

## Findings

1. **The trigger did not reproduce, at any contention level tried.** Plan Task 1 required
   diagnosing which `classifyRun` path actually fires before fixing. It did not fire:
   - 8 concurrent copies × `-count=5` of the target test alone — all green, ~1s per copy.
   - Escalated to 16 concurrent copies × `-count=2` of the whole `internal/process` package
     (~50s per copy under real self-contention; 32 executions of the target test) — all green.

   Reported as non-reproduction rather than escalated indefinitely. The consequence is recorded in
   *Verify (human)* above rather than buried: the fix's trigger is inferred from the code, not
   observed.

2. **The grooming's stated root cause was wrong, and reconcile caught it.** Change 0328's `## Why`
   attributed the flake to "the owned run wrote its own durable terminal record". That does not
   survive reading the code: the test helper's `sleep` mode blocks for `time.Hour` with default
   signal disposition (`main_test.go`, `case "sleep"`), so the child never exits on its own, and a
   SIGKILLed supervisor cannot write anything. `Marked:0` is reachable **four** ways through
   `classifyRun` (`internal/process/recover.go`): a durable terminal record; a held or unprovable
   live lock; a pre-existing stopped/abandoned marker; and `recoverGroupProbe` answering
   `probeLive`/`probeUnknown` — the last being the most plausible under load, where the recorded
   PGID can be recycled between the test's own `groupAlive` wait and recover's re-probe. The
   groomed fix checked only `terminal.json`, which would have left three of the four paths flaky.
   The change file's `## Reconcile log` carries the full correction.

3. **The new guard is mutation-proven, not decoration.** Per AGENTS.md's "a guard is code:
   mutation-test it — strip the thing it guards, watch it redden":
   - *Mutation A* — `abandonedPreconditionUnmet` forced to return permanently unmet: the loop
     re-drove twice and then failed at the cap with `setup never reached the abandoned shape in 3
     attempts`. Confirms the bound and the fatal.
   - *Mutation B* — forced unmet on the first call only: one `re-driving` log line, then **PASS**.
     Confirms the retry genuinely re-drives setup rather than only aborting.

   Both mutations were reverted; the branch carries no residue (`grep -c "mutCalls\|MUTATION"` = 0).

4. **Runtime cost is nil on the happy path — recorded as numbers, per the budget learning.**
   16-copy whole-package stress, before vs after: **47.9–50.8s → 45.8–48.3s**. Zero `re-driving`
   events across all 16 verify copies, meaning the retry never executed on a healthy run. The
   package under `-race`: 17.1s, green.

5. **Full suite green; the OVER BUDGET set is pre-existing and machine-driven, not this change.**
   `scripts/run-tests.sh`: **122 files, 122 passed, 0 failed, 9402 asserts, wall 281s, exit 0.**
   Ten files printed `OVER BUDGET`. The two this branch could plausibly touch:
   - `test_go_toolchain` — **153s against a 55s ceiling**. It was measured at **150s** during change
     0325's finalize gate, before this change existed. Delta ≈ 3s, i.e. noise.
   - `test_go_race` — **168s against a 60s ceiling** (hard ceiling; no re-budget available).

   Ten rows over at once, most by roughly 3x, is the whole-suite cliff that
   `tolerance-constant-calibrated-on-one-machine` (#312 entry) says to read as a statement about
   the machine, not the suite — and this run followed 32 concurrent package executions on the same
   box. Recorded as numbers rather than as "did not trip the budget check", per
   `budget-headroom-is-spent-before-it-is-breached`.

6. **Plan deviation (deliberate).** Plan Task 1 Step 4 said to revert the sharpened failure message
   (`Marked=%d disposition=%q reason=%q`) after diagnosis. It was **kept** instead and folded into
   Task 2's commit. Rationale: it is test-only, in the same test, and strictly better diagnostics —
   a real production failure now names the disposition and reason instead of dumping `%+v`. Given
   Finding 1 (the trigger is still unobserved), keeping the sharper message is the thing most
   likely to make the *next* occurrence diagnosable in one shot.

## Follow-ups

- **No new stub minted for the budget breaches — deduped against change 0280**, "Shard or
  re-budget the test files the suite runner reports OVER BUDGET" (`status: proposed`), which
  already owns exactly this work and names the same remedy. Auto-capture is enabled
  (`AUTO_CAPTURE_ENABLED=true`); this is a dedup skip, not a suppression, and it consumed no mint
  slot. Note for whoever takes 0280: `test_go_race` at 168s sits against the **hard** 60s ceiling,
  so per the budget learning its only remaining move is a shard, not a re-budget.
- If the flake recurs after this lands, the failure output will now name the disposition and reason
  directly. That string is the missing input to Finding 1 — capture it and the correct narrow fix
  becomes a one-liner.
