<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0328 — De-flake TestRecoverMarksCleanlyAbandonedOwnedRun under full-suite load](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0328-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full.md)**
<!-- docket:backlink:end -->

# De-flake TestRecoverMarksCleanlyAbandonedOwnedRun Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `TestRecoverMarksCleanlyAbandonedOwnedRun` deterministic under full parallel-suite contention by asserting the abandoned precondition it currently assumes, and re-driving setup with a bounded retry when that precondition is not met.

**Architecture:** Test-side only. The test builds an "abandoned run" shape (owned run whose supervisor was SIGKILLed, leaving no durable verdict) and asserts `Recover` marks it. Today it *assumes* the shape was built; under load the setup can land in a different shape, and the test reports that as a `Marked:0` failure of production code that is in fact behaving correctly. The fix makes the setup self-verifying: check every durable record that outranks the group probe in `classifyRun`, and if any is present, discard that run and rebuild. The `Marked==1` assertion is never weakened.

**Tech Stack:** Go, `internal/process` package, standard `testing`. No new dependencies.

**Spec:** No spec — change 0328 is `trivial: true`. The authority is the change file itself: `docs/changes/active/0328-de-flake-testrecovermarkscleanlyabandonedownedrun-under-full.md` on the `docket` branch, whose `## What changes` and `## Reconcile log` this plan implements.

## Global Constraints

- **Test-only.** No file outside `internal/process/*_test.go` may be modified. Production-side synchronization hooks in the launch/supervisor path were considered and rejected as invasive (change 0328, `## Out of scope`).
- **The `Marked==1` assertion is never weakened.** A run provably lacking every durable verdict, with its group provably absent, must be marked at any load. Retry re-drives *setup*, never the assertion.
- **Bounded retry.** Setup attempts are capped; exhaustion is a hard `t.Fatalf`, never a skip and never a pass.
- **Budget awareness.** `test_go_toolchain` was measured at **150s / OVER BUDGET** during change 0325's finalize gate. Any wall-clock added here must be justified and reported as a number at close-out, per the `budget-headroom-is-spent-before-it-is-breached` learning. Retry costs nothing on the happy path — it only runs when setup actually lost.
- **Deadline widening follows 0325's precedent.** Widening an isolation-calibrated wait for contention is safe in the loose direction only because `waitFor` returns the instant its predicate holds; the larger ceiling costs wall-time solely on a genuine hang. Record the calibration next to the constant (`tolerance-constant-calibrated-on-one-machine`).

---

### Task 1: Diagnose which `classifyRun` path actually fires under load

Change 0328's grooming attributed the flake to the run writing its own `terminal.json`. Reconcile found that unproven: the helper's `sleep` mode blocks for `time.Hour` with default signal disposition, so the child never exits on its own, and a SIGKILLed supervisor writes nothing. This task replaces the hypothesis with a measurement before any fix is written. **Do not skip to Task 2** — the fix in Task 2 is deliberately broad enough to cover every path, but the recorded evidence is what justifies it and what the results file reports.

**Files:**
- Modify (temporarily, reverted at the end of this task): `internal/process/recover_test.go`
- Read only: `internal/process/recover.go` (the `classifyRun` decision order), `internal/process/main_test.go` (`case "sleep"`, `waitFor`)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: a recorded observation — the actual `RecoveryEntry.Disposition` and `.Reason` seen on a failing run — consumed by Task 2's precondition list and by the results file.

- [ ] **Step 1: Sharpen the failure message so a stress run reports the cause, not just the count**

The current `t.Fatalf("recover: %+v", res)` does print the entries, but a stress run's output is easier to triage with the disposition called out. Temporarily replace the assertion block in `TestRecoverMarksCleanlyAbandonedOwnedRun`:

```go
	if res.Marked != 1 || len(res.Entries) != 1 || res.Entries[0].Disposition != "abandoned-marked" {
		if len(res.Entries) == 1 {
			t.Fatalf("recover: Marked=%d disposition=%q reason=%q",
				res.Marked, res.Entries[0].Disposition, res.Entries[0].Reason)
		}
		t.Fatalf("recover: %+v", res)
	}
```

- [ ] **Step 2: Reproduce under self-contention (0325's 8-copy technique)**

Run from the repository root:

```bash
cd /Users/homer/dev/docket/.worktrees/de-flake-testrecovermarkscleanlyabandonedownedrun-under-full
for i in $(seq 1 8); do
  go test ./internal/process/ -run '^TestRecoverMarksCleanlyAbandonedOwnedRun$' -count=5 \
    > "/tmp/0328-stress-$i.log" 2>&1 &
done
wait
grep -h "recover: Marked=" /tmp/0328-stress-*.log | sort | uniq -c
```

Expected: at least one failure line naming a disposition. Record the exact `disposition=` and `reason=` values.

If 8×5 produces no failure, escalate the contention rather than declaring the flake absent: raise to 16 copies, and/or run the whole package (`go test ./internal/process/ -count=2`) in 8 copies so the test contends with its siblings the way the real suite makes it.

- [ ] **Step 3: Record the finding**

Write the observed disposition/reason into the task notes for the results file. Expected candidates, in `classifyRun` order — all four produce `Marked:0`:

| Observed disposition | Reason string | What it means |
|---|---|---|
| `terminal` | `durable terminal record present` | the grooming's hypothesis |
| `live` | `supervisor holds the live lock` | lock wait passed then re-took |
| `needs-inspection` | `live lock unprovable; not marked` | `probeFlock` → `probeUnknown` |
| `needs-inspection` | `recorded group is live or unprovable; not marked` | PGID recycled between the test's wait and recover's re-probe |

- [ ] **Step 4: Revert the temporary edit**

```bash
git checkout -- internal/process/recover_test.go
```

Task 1 lands **no commit**. Its deliverable is the recorded observation; Task 2 makes the real change.

---

### Task 2: Assert the abandoned precondition and re-drive setup on a bounded retry

**Files:**
- Modify: `internal/process/recover_test.go` — add the `abandonedPreconditionUnmet` helper, rewrite `TestRecoverMarksCleanlyAbandonedOwnedRun`'s setup into a bounded retry loop, widen the two `waitFor` deadlines.
- Test: `internal/process/recover_test.go` (the test *is* the unit under change).

**Interfaces:**
- Consumes: Task 1's recorded disposition, which must appear in the helper's checked set.
- Produces: `func abandonedPreconditionUnmet(t *testing.T, runDir string, pgid int) string` — returns `""` when the run at `runDir` is the clean abandoned shape, else a human-readable reason. Available to any sibling test in `package process` that needs the same shape.

- [ ] **Step 1: Write the failing test — the precondition helper**

Add to `internal/process/recover_test.go`, above `TestRecoverMarksCleanlyAbandonedOwnedRun`:

```go
// abandonedPreconditionUnmet reports why the run at runDir is not the clean
// abandoned shape Recover must mark, or "" when it is. Every condition checked
// here outranks the group probe in classifyRun's decision order, so any one of
// them makes Marked==1 unreachable for a reason that is SETUP, not a defect —
// which is exactly the distinction this test could not make before change 0328.
// Checked in classifyRun's own order so the reported reason names the branch
// that would actually have fired.
func abandonedPreconditionUnmet(t *testing.T, runDir string, pgid int) string {
	t.Helper()
	if held, ans := probeFlock(filepath.Join(runDir, liveLockFile)); ans == probeUnknown {
		return "live lock unprovable"
	} else if held {
		return "live lock still held"
	}
	if term, err := readTerminal(runDir); err != nil {
		return fmt.Sprintf("terminal record unreadable: %v", err)
	} else if term != nil {
		return "durable terminal record present"
	}
	if st, err := readStopped(runDir); err != nil {
		return fmt.Sprintf("stopped marker unreadable: %v", err)
	} else if st != nil {
		return "completed-stop marker present"
	}
	if ab, err := readAbandoned(runDir); err != nil {
		return fmt.Sprintf("abandoned marker unreadable: %v", err)
	} else if ab != nil {
		return "abandoned marker already present"
	}
	if got := groupAlive(pgid); got != probeAbsent {
		return fmt.Sprintf("recorded group %d probes %v, not absent", pgid, got)
	}
	return ""
}
```

Add `"fmt"` to the import block if absent.

- [ ] **Step 2: Run to verify it compiles and the existing test still passes in isolation**

Run: `go test ./internal/process/ -run '^TestRecoverMarksCleanlyAbandonedOwnedRun$' -v`
Expected: PASS (the helper is not yet called — this step only proves it compiles and that `fmt`/`filepath` are wired).

- [ ] **Step 3: Rewrite the test's setup as a bounded retry**

Replace the body of `TestRecoverMarksCleanlyAbandonedOwnedRun` from `svc := newTestService(t)` down to (but not including) `res, err := svc.Recover(root)`:

```go
func TestRecoverMarksCleanlyAbandonedOwnedRun(t *testing.T) {
	svc := newTestService(t)

	// Setup builds the abandoned shape: an owned run whose supervisor is
	// SIGKILLed, leaving a free lock, no durable verdict, and a gone group.
	// Under full-suite contention the setup can lose that race, and the run
	// lands in a shape Recover CORRECTLY declines to mark — a setup failure
	// the test used to report as a production defect (change 0328). Re-drive
	// setup on a fresh root when that happens; never weaken the assertion.
	const setupAttempts = 3
	var root string
	var out *LaunchOutcome
	var m *manifestRecord
	for attempt := 1; ; attempt++ {
		root = t.TempDir()
		out = launchHelper(t, svc, root, "sleep")
		var err error
		m, err = readManifest(out.RunDir)
		if err != nil || m == nil {
			t.Fatalf("read manifest: %v (m=%v)", err, m)
		}
		// KILL the whole group: supervisor dies without a terminal record —
		// the abandoned shape.
		signalGroup(m.PGID, syscall.SIGKILL)
		// 60s, not an isolation-calibrated 30s: under full parallel-suite CPU
		// contention a SIGKILLed group needs longer merely to be reaped. Safe
		// in the loose direction because waitFor returns the instant its
		// predicate holds, so the wider ceiling costs wall-time only on a
		// genuine hang — the same trade change 0325 made for its barrier waits.
		waitFor(t, "lock release", 60*time.Second, func() bool {
			held, _ := probeFlock(filepath.Join(out.RunDir, liveLockFile))
			return !held
		})
		waitFor(t, "group gone", 60*time.Second, func() bool {
			return groupAlive(m.PGID) == probeAbsent
		})
		why := abandonedPreconditionUnmet(t, out.RunDir, m.PGID)
		if why == "" {
			break
		}
		if attempt == setupAttempts {
			t.Fatalf("setup never reached the abandoned shape in %d attempts; last: %s",
				setupAttempts, why)
		}
		t.Logf("setup attempt %d did not reach the abandoned shape (%s); re-driving", attempt, why)
	}

	res, err := svc.Recover(root)
```

Everything from `res, err := svc.Recover(root)` onward is unchanged — the assertions keep their current form.

- [ ] **Step 4: Run the test in isolation**

Run: `go test ./internal/process/ -run '^TestRecoverMarksCleanlyAbandonedOwnedRun$' -count=5 -v`
Expected: PASS 5/5, with no `re-driving` log lines on an unloaded machine.

- [ ] **Step 5: Run the stress reproduction from Task 1 against the fix**

```bash
for i in $(seq 1 8); do
  go test ./internal/process/ -run '^TestRecoverMarksCleanlyAbandonedOwnedRun$' -count=5 \
    > "/tmp/0328-verify-$i.log" 2>&1 &
done
wait
grep -c FAIL /tmp/0328-verify-*.log
grep -h "re-driving" /tmp/0328-verify-*.log | wc -l
```

Expected: zero FAIL across all 8 logs. The `re-driving` count is the evidence the retry is load-bearing — record it for the results file. A count of zero under stress means the contention was not reproduced, not that the fix is unnecessary; escalate copies as in Task 1 Step 2 before accepting.

- [ ] **Step 6: Run the package under race, then the full suite**

```bash
go test ./internal/process/ -race -count=1
```
Expected: PASS.

Then the whole suite via the resolved gate command — `scripts/run-tests.sh` (from `finalize.test_command`; never a second copy). Expected: green. Note `test_go_toolchain`'s reported wall clock and whether it prints `OVER BUDGET`; that number goes in the results file as a number, per the budget learning.

- [ ] **Step 7: Commit**

```bash
git add internal/process/recover_test.go
git commit -m "fix(0328): assert the abandoned precondition and re-drive setup on a bounded retry"
```

---

## Self-Review

**Spec coverage** — change 0328's `## What changes`, as widened by its reconcile log:

| Requirement | Task |
|---|---|
| 1. Diagnose the exact race first under a concurrent-stress run (0325's 8-copy technique) | Task 1, Steps 1–3 |
| 2. Assert every durable verdict absent *and* group still `probeAbsent` | Task 2, Step 1 (`abandonedPreconditionUnmet`) + Step 3 (call site) |
| 3. Bounded retry re-driving setup; `Marked==1` never weakened | Task 2, Step 3 (`setupAttempts = 3`, fresh `root` per attempt, assertions untouched) |
| 4. Evidence: multi-copy stress run green, plus the full suite | Task 2, Steps 5–6 |

**Placeholder scan** — no TBDs; every code step carries the actual code; the stress commands are literal and runnable.

**Type consistency** — `abandonedPreconditionUnmet(t *testing.T, runDir string, pgid int) string` is defined in Task 2 Step 1 and called with exactly that signature in Step 3. `readTerminal`/`readStopped`/`readAbandoned` are used with the `(record, error)` shape `classifyRun` uses in `internal/process/recover.go`; `probeFlock` as `(held bool, ans probeAnswer)`; `groupAlive(pgid) probeAnswer` compared against `probeAbsent`; `readManifest` as `(*manifestRecord, error)`. `LaunchOutcome` and `manifestRecord` are the existing package types.

**Known-weak point, flagged deliberately:** if Task 1 cannot reproduce the flake at any contention level, Task 2 is a fix whose trigger was never observed. That is still worth landing — the precondition assertion converts a future mystery `Marked:0` into a named setup failure, which is strictly better diagnostics — but the results file must say the trigger was not reproduced rather than implying it was.
