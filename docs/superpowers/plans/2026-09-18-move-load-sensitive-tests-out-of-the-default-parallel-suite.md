<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0434 — Move load-sensitive tests out of the default parallel suite lane](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0434-move-load-sensitive-tests-out-of-the-default-parallel-suite.md)**
<!-- docket:backlink:end -->
# Move Load-Sensitive Tests Out of the Default Parallel Suite Lane — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stabilize the load-sensitive coverage 0411 flagged — measure the implicated `internal/app` integration shards and default-corpus gatedrive tests, migrate/rebalance only measured-justified slow tests within the existing integration-tag machinery, and deterministically reproduce and fix the gatedrive terminal-relaunch interleaving — with every scenario staying mandatory and every wrapper staying parallel.

**Architecture:** Three independent strands. (1) A measurement/classification pass at the unchanged branch head produces the evidence that gates every migration decision. (2) The gatedrive strand adds a deterministic loser-after-terminal-settle regression to the fast corpus and makes `driveSlice`'s `reserveRelaunch` error branch treat `errAlreadyTerminal` the way it already treats `errRelaunchRaceLost` (reload authoritative state, no error). (3) Migration/rebalance reuses the existing pieces verbatim: the `integration` build tag, `TestIntegration`/`TestRaceIntegration` prefixes, `tests/test_go_integration_*.sh` declaration-only wrappers over `tests/lib/go-integration-shard.sh`, the auto-discovering completeness contract, `DOCKET_GO_TEST_CONCURRENCY`, and `tests/runtime-budgets.tsv` ceilings derived from isolated measurements.

**Tech Stack:** Go 1.x (module-pinned), `go test` with `-tags integration` / `-race` / `-count=1`, Bash suite wrappers, the Go-native suite runner (`internal/suiterunner`) via `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite-design.md` (synchronized on the `docket` metadata branch; the settled design — read it alongside this plan).

## Global Constraints

- **No new lanes.** No serial wrapper pins (every `tests/runtime-budgets.tsv` row this change touches stays `parallel`), no scheduler, no quarantine, no automatic retries, no attempt-limit changes. ADR-0108 is cited unchanged and not superseded.
- **No blanket timeout increases and no ceiling inflation to conceal growth.** Budget ceilings are re-derived from measurements by the table's own rule: "rounded up to the next multiple of 5 plus a 5s margin (min 10s)", applied to the **worst standalone isolated (serial) uncached reading**, never a contended run-of-the-day number. 60s is the table's hard ceiling — a row that cannot fit must be split, not raised.
- **Every scenario stays mandatory.** No assertion weakened, no subcase dropped, no test skipped. Concurrency-bearing integration tests keep `-race` instrumentation (SHARD_MODE=race).
- **Prefix disjointness.** When splitting a shard, never retain a broad parent prefix that also selects a new child shard's tests — `-run "^Prefix"` is a prefix match, so both sides of a split must be renamed to non-overlapping prefixes.
- **`-count=1` on every run whose result is evidence** (mutation probes, measurements, verification). Read `(cached)` as absence of evidence. The shard executor already forces `-count=1`; direct `go test` invocations in this plan must too.
- **Mutation-probe restores use a backup copy** (`cp file file.bak` … `mv -f file.bak file`), never `git checkout --`, which would destroy uncommitted work.
- **Read the budget report on every green run.** `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` are screening findings; `SERIAL CONFIRMED OVER BUDGET:` is an authoritative breach. Record them; never let a ceiling raise hide one.
- **Honesty over completion.** If measurement shows an indivisible slow case or pure host contention the existing machinery cannot fix, record it as an unmet acceptance condition in the Measurement record and the task report — do not invent a serial lane, raise a timeout, weaken an assertion, or claim tagging solved it.
- **No unrelated cleanup.** `gofmt` only files this change touches; no drive-by refactors.
- Shell discipline per repo AGENTS.md: no producer piped into an early-exiting consumer under pipefail; capture into a variable or `tee` to a file first.

---

### Task 1: Baseline measurement and classification (before moving anything)

**Files:**
- Modify: `docs/superpowers/plans/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite.md` (append the **Measurement record** section at the end of this file)
- No production or test code changes in this task.

**Interfaces:**
- Consumes: the unchanged branch head (`git rev-parse HEAD` — record it; it must be the pre-build head for a comparable baseline).
- Produces: the committed **Measurement record** appendix in this plan file — per-shard isolated timings, whole-suite baseline wall time and budget findings, per-test durations for implicated corpora, environment facts, and one explicit **classification verdict per candidate** that Tasks 3 and 4 consume. Raw logs live in the fixed scratch dir `"${TMPDIR:-/tmp}/docket-0434-evidence"` (create with `mkdir -p`; self-heals over any prior run's leftovers).

- [ ] **Step 1: Record the environment and revision**

```bash
EVID="${TMPDIR:-/tmp}/docket-0434-evidence"; mkdir -p "$EVID"
{ git rev-parse HEAD; uname -a; sysctl -n hw.ncpu; echo "jobs=${DOCKET_RUNTESTS_JOBS:-default}"; date -u +%Y-%m-%dT%H:%M:%SZ; } | tee "$EVID/env.txt"
```

- [ ] **Step 2: Capture two baseline whole-suite runs at default/configured parallelism**

```bash
go run ./cmd/docket development test 2>&1 | tee "$EVID/baseline-suite-1.log"
go run ./cmd/docket development test 2>&1 | tee "$EVID/baseline-suite-2.log"
```

From each log record: total wall time, per-file timings for every `tests/test_go_integration_app_*.sh` wrapper plus `tests/test_go_race.sh` and `tests/test_go_toolchain.sh`, every `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` / `SERIAL CONFIRMED OVER BUDGET:` line, and any failure. These runs are evidence collection, not the governed build gate — do not touch gate accounting. A red baseline is itself a finding: record which target failed and whether it matches 0411's reported shapes (app package-timeout under load; `terminal_relaunch_winner_fails` returning `gatedrive: drive already terminal`).

- [ ] **Step 3: Measure the implicated shards isolated and uncached**

Solo, nothing else running, one at a time (the executor already forces `-count=1`):

```bash
for f in tests/test_go_integration_app_rebase.sh tests/test_go_integration_app_concurrency.sh \
         tests/test_go_integration_app_change.sh tests/test_go_integration_app_sweep.sh \
         tests/test_go_integration_app_sync.sh tests/test_go_integration_app_state.sh \
         tests/test_go_integration_app_workflow.sh; do
  ( time bash "$f" ) > "$EVID/iso-$(basename "$f" .sh).log" 2>&1
done
```

That list is the 0411-implicated shard (`app_rebase`) plus every app row whose ceiling is ≥45s or whose baseline parallel timing from Step 2 sits near its `tests/runtime-budgets.tsv` ceiling — extend the list from the Step 2 numbers, do not shrink it. Record each `real` time against its budget row.

- [ ] **Step 4: Per-test durations inside the implicated shards**

```bash
go test -tags integration -count=1 -run '^TestIntegrationFinalizeRebase' -v ./internal/app \
  2>&1 | tee "$EVID/pertest-app-rebase.log"
grep -E -e '^--- PASS' "$EVID/pertest-app-rebase.log"
```

Repeat with each implicated shard's declared `SHARD_PREFIX`/`SHARD_PKG` (read them from the wrapper files; add `-race` when `SHARD_MODE=race`). The `--- PASS: Name (12.34s)` lines are the per-test cost breakdown: identify whether cost is aggregate, a single dominant test, or contention amplification (isolated time far below the parallel baseline time).

- [ ] **Step 5: Measure the default-corpus gatedrive tests**

```bash
go test -count=1 -v ./internal/gatedrive/ 2>&1 | tee "$EVID/pertest-gatedrive-plain.log"
go test -race -count=1 -v ./internal/gatedrive/ 2>&1 | tee "$EVID/pertest-gatedrive-race.log"
grep -E -e '^--- (PASS|FAIL)' "$EVID/pertest-gatedrive-race.log"
```

Record per-test durations and whether `TestConcurrentSameOwnerAdvanceRelaunchesOnce/terminal_relaunch_winner_fails` fails here (isolated it typically passes — the 0411 failures were under whole-module load; Task 2 owns the determinization either way). The gatedrive concurrency fixture is fake-`ProcessSeam`-backed: expect it to measure fast, which per the spec keeps it in the default corpus — its failure is a correctness problem, not a placement problem.

- [ ] **Step 6: Write the classification verdicts and commit**

Append a `## Measurement record` section to this plan file containing: the environment block, a table of `wrapper | budget ceiling | baseline parallel s | isolated serial s`, the per-test duration highlights, the budget-report lines, and — for each candidate — exactly one verdict with its justifying numbers:

- **split/rebalance** (already-tagged app shard whose isolated serial time approaches or exceeds its ceiling, or breaks the sub-60s shard regime, or is dominated by a separable test group);
- **migrate** (a default-corpus test whose isolated uncached cost and real-repo/subprocess workload justify the slow-test partition — expected rare; fast fake-backed tests stay);
- **leave in place** (with the number that says so);
- **functional failure** (route to Task 2 — moving a faulty test fixes nothing);
- **unresolved limitation** (evidence recorded; carried as an unmet acceptance condition, not papered over).

```bash
git add docs/superpowers/plans/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite.md
git commit -m "chore(0434): baseline measurement record and shard classification verdicts"
```

---

### Task 2: Deterministically reproduce and fix the gatedrive terminal-relaunch interleaving

**Files:**
- Modify: `internal/gatedrive/driver.go` (the `reserveRelaunch` error branch inside `driveSlice`, in the `process.StateSignaled, process.StateVanished` case)
- Test: `internal/gatedrive/driver_concurrency_test.go` (new fixture + new test; existing tests unchanged)

**Interfaces:**
- Consumes: `Store.reserveRelaunch(id, ownerGen string) (*relaunchClaim, error)`; sentinels `errAlreadyTerminal`, `errRelaunchRaceLost`; `sliceResult.relaunchRaceLost` (consumed by `driveAndPersistClaim`: on true it reloads via `d.store.Load(id)` and returns `d.recordedDoc(id, ownerGen, cur), nil`); existing test helpers `seedDrive`, `seedRecord`, `fakeClock`, `startEpoch`, `pollTick`, `stableGit`, `reopenStore`, `testsupport.TempDir`.
- Produces: test `TestLoserAfterTerminalSettleReturnsRecordedState` and fixtures `terminalSettleProc` / `gatedLoserSeam` in the fast (default) gatedrive corpus.

**The proven interleaving (verified against source):** `reserveRelaunch`'s owner-CAS checks `isTerminalOutcome(rec.LastOutcome)` **before** the relaunch-consumed check, so a same-owner loser whose reservation attempt lands after the winner persisted the replacement's terminal outcome receives `errAlreadyTerminal`. The `driveSlice` branch that calls `reserveRelaunch` recognizes only `errRelaunchRaceLost`; `errAlreadyTerminal` falls through to `res.err` and `Advance` returns the raw sentinel `gatedrive: drive already terminal` as an error — exactly the 0411 failure text. The later `attachReservedRelaunch` branch and the persist CAS already treat both sentinels as "reload authoritative state". Confirm this reading against the current source before writing code; if the code has moved, this task's fix follows the same intent (loser returns recorded state), not these exact lines.

- [ ] **Step 1: Write the deterministic failing test**

Append to `internal/gatedrive/driver_concurrency_test.go`:

```go
// terminalSettleProc is the shared ProcessSeam core for the deterministic
// loser-after-terminal-settle regression: the seeded original run observes as
// signaled (dead), and every relaunched run reports StateFailed so the winner
// settles the drive terminally within its single Advance.
type terminalSettleProc struct {
	mu        sync.Mutex
	launchSeq int
	launched  map[string]bool
	stops     []string
}

func newTerminalSettleProc() *terminalSettleProc {
	return &terminalSettleProc{launched: map[string]bool{}}
}

func (p *terminalSettleProc) Launch(process.LaunchRequest) (*process.LaunchOutcome, error) {
	p.mu.Lock()
	p.launchSeq++
	id := fmt.Sprintf("relaunch%d", p.launchSeq)
	dir := "/runs/" + id
	p.launched[dir] = true
	p.mu.Unlock()
	return &process.LaunchOutcome{RunID: id, RunDir: dir, State: process.StateRunning}, nil
}

func (p *terminalSettleProc) Observe(runDir string) (*process.Observation, error) {
	if strings.HasSuffix(runDir, "run1") {
		return &process.Observation{State: process.StateSignaled, RunDir: runDir}, nil
	}
	return &process.Observation{State: process.StateFailed, RunDir: runDir}, nil
}

func (p *terminalSettleProc) Stop(runDir, reason string) (*process.StopOutcome, error) {
	p.mu.Lock()
	p.stops = append(p.stops, runDir)
	p.mu.Unlock()
	if strings.HasSuffix(runDir, "run1") {
		return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
			Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
	}
	return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
}

func (p *terminalSettleProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	return &process.ReservationResolution{Disposition: "never-launched"}, nil
}

func (p *terminalSettleProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return process.RecoveryEntry{Disposition: "invalid"}, nil
}

// gatedLoserSeam wraps the shared core for the LOSER driver only: its first
// dead-run observation signals `observing` (proving the loser loaded a
// nonterminal record and entered its slice) and then parks on `gate` until the
// test releases it — after the winner's Advance has fully returned with the
// terminal outcome durably persisted. This forces, deterministically, the
// interleaving where the loser's reserveRelaunch CAS finds a terminal record.
type gatedLoserSeam struct {
	core      *terminalSettleProc
	gate      <-chan struct{}
	observing chan struct{}
	once      sync.Once
}

func (s *gatedLoserSeam) Launch(req process.LaunchRequest) (*process.LaunchOutcome, error) {
	return s.core.Launch(req)
}

func (s *gatedLoserSeam) Observe(runDir string) (*process.Observation, error) {
	if strings.HasSuffix(runDir, "run1") {
		s.once.Do(func() { close(s.observing) })
		<-s.gate
	}
	return s.core.Observe(runDir)
}

func (s *gatedLoserSeam) Stop(runDir, reason string) (*process.StopOutcome, error) {
	return s.core.Stop(runDir, reason)
}

func (s *gatedLoserSeam) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	return s.core.ResolveReservation(root, token)
}

func (s *gatedLoserSeam) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return s.core.ClassifyRun(runDir, mark)
}

// TestLoserAfterTerminalSettleReturnsRecordedState pins the deterministic
// resolution of the 0411 interleaving: a same-owner advance that loses the
// relaunch race only AFTER the winner has persisted the replacement's terminal
// outcome must return the authoritative recorded state — never the raw
// errAlreadyTerminal sentinel as an Advance error. Exactly one launch, one
// relaunch, one attempt increment, no orphan.
func TestLoserAfterTerminalSettleReturnsRecordedState(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	seeded := seedRecord(t)
	id, ownerGen := seedDrive(t, store, seeded)

	core := newTerminalSettleProc()
	gate := make(chan struct{})
	loserSeam := &gatedLoserSeam{core: core, gate: gate, observing: make(chan struct{})}

	mkDriver := func(seam any) *Driver {
		clk := &fakeClock{now: startEpoch().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, seam, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		return d
	}

	type advanceResult struct {
		doc DriveDoc
		err error
	}
	loserDone := make(chan advanceResult, 1)
	go func() {
		doc, err := mkDriver(loserSeam).Advance(id, ownerGen)
		loserDone <- advanceResult{doc: doc, err: err}
	}()

	// The loser is parked inside its slice holding a stale nonterminal record.
	<-loserSeam.observing

	// The winner runs to completion: dead original observed, relaunch reserved
	// and launched, replacement observed StateFailed, FAILED persisted.
	winnerDoc, winnerErr := mkDriver(core).Advance(id, ownerGen)
	if winnerErr != nil {
		t.Fatalf("winner advance: %v", winnerErr)
	}
	if winnerDoc.Outcome != FAILED {
		t.Fatalf("winner outcome = %s, want %s", winnerDoc.Outcome, FAILED)
	}

	// Only now may the loser proceed to its reservation attempt.
	close(gate)
	loser := <-loserDone
	if loser.err != nil {
		t.Fatalf("the reservation loser must return recorded state, not an error: %v", loser.err)
	}
	if loser.doc.Outcome != FAILED {
		t.Fatalf("loser doc outcome = %s (%s), want %s", loser.doc.Outcome, loser.doc.Cause, FAILED)
	}
	if loser.doc.Cause != winnerDoc.Cause {
		t.Fatalf("loser doc cause = %q, want the winner's recorded cause %q", loser.doc.Cause, winnerDoc.Cause)
	}

	if core.launchSeq != 1 {
		t.Fatalf("exactly one backend launch, got %d", core.launchSeq)
	}
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.RelaunchCount != 1 {
		t.Fatalf("RelaunchCount = %d, want 1", rec.RelaunchCount)
	}
	if rec.Attempt != 2 {
		t.Fatalf("Attempt = %d, want 2", rec.Attempt)
	}
	if rec.LastOutcome != FAILED {
		t.Fatalf("recorded outcome = %s (%s), want %s", rec.LastOutcome, rec.LastCause, FAILED)
	}
	if !rec.Deadline.Equal(seeded.Deadline) {
		t.Fatalf("the original deadline must be preserved: got %v, seeded %v", rec.Deadline, seeded.Deadline)
	}
	core.mu.Lock()
	var relaunchStops int
	for _, s := range core.stops {
		if core.launched[s] {
			relaunchStops++
		}
	}
	core.mu.Unlock()
	if relaunchStops != 0 {
		t.Fatalf("the loser must not create or stop an orphan, stopped %d relaunch runs", relaunchStops)
	}
}
```

Adaptation notes for the implementer (verify each against the file, do not guess): `mkDriver`'s seam parameter type must be whatever interface `NewDriver`'s process argument declares (spell it exactly — replace `any` with that type). `DriveDoc` field names (`Outcome`, `Cause`), `seedRecord`'s `Deadline` field/type, and the `FAILED` outcome constant must match the package's existing spellings — the sibling tests in this same file use all of them. If `driveRecord.Deadline` is not a `time.Time` (e.g. an epoch value), compare with `==` instead of `.Equal`. Preserve every assertion's substance.

- [ ] **Step 2: Run the new test and verify it fails with the propagated sentinel**

```bash
go test -count=1 -run 'TestLoserAfterTerminalSettleReturnsRecordedState' -v ./internal/gatedrive/
```

Expected: FAIL, with `the reservation loser must return recorded state, not an error: gatedrive: drive already terminal`. If it fails any other way, stop and debug the fixture (superpowers:systematic-debugging) — a different failure means the interleaving was not the one hypothesized, and the spec requires recording the causal trace and fixing the *actual* defect within this same boundary, never accepting `errAlreadyTerminal` as an allowed error to make an assertion pass.

- [ ] **Step 3: Make the narrow product correction**

In `internal/gatedrive/driver.go`, inside `driveSlice`'s `process.StateSignaled, process.StateVanished` case, the `reserveRelaunch` error branch currently reads:

```go
claim, err = d.store.reserveRelaunch(id, ownerGen)
if err != nil {
	if errors.Is(err, errRelaunchRaceLost) {
		res.relaunchRaceLost = true
		return res
	}
	res.err = err
	return res
}
```

Change the condition to recognize both same-owner race sentinels, mirroring the `attachReservedRelaunch` branch below it:

```go
claim, err = d.store.reserveRelaunch(id, ownerGen)
if err != nil {
	// A same-owner competitor may have consumed the relaunch
	// (errRelaunchRaceLost) or already settled the drive terminally
	// before this reservation CAS (errAlreadyTerminal). Either way the
	// loser reloads and returns the authoritative recorded state; it
	// never launches, spends no attempt, and surfaces no error.
	if errors.Is(err, errRelaunchRaceLost) || errors.Is(err, errAlreadyTerminal) {
		res.relaunchRaceLost = true
		return res
	}
	res.err = err
	return res
}
```

Genuine failures are untouched: `verifyOwner` errors, `ErrIO` from token generation, and store I/O errors still propagate through `res.err`. `res.relaunchRaceLost` already routes to `driveAndPersistClaim`'s reload-and-return path, which performs no attempt/relaunch mutation and preserves the deadline. Also extend the `sliceResult.relaunchRaceLost` field comment (and the `errAlreadyTerminal` comment's "it never escapes as a workflow error" claim, which this fix makes true for this path) to cover the terminal-settle case.

- [ ] **Step 4: Run the regression and the full gatedrive package, plain and race**

```bash
go test -count=1 -run 'TestLoserAfterTerminalSettleReturnsRecordedState' -v ./internal/gatedrive/
go test -count=1 ./internal/gatedrive/
go test -race -count=1 ./internal/gatedrive/
```

Expected: all PASS, including both existing `TestConcurrentSameOwnerAdvanceRelaunchesOnce` subtests (running replacement and immediately terminal replacement — their coverage is preserved untouched) and `TestRelaunchReservationHolderCannotBeStolenBeforeLaunch`.

- [ ] **Step 5: Revert-reddens proof (backup copy, not git checkout)**

```bash
cp internal/gatedrive/driver.go internal/gatedrive/driver.go.bak
# Edit: restore the condition to `errors.Is(err, errRelaunchRaceLost)` only.
go test -count=1 -run 'TestLoserAfterTerminalSettleReturnsRecordedState' ./internal/gatedrive/
# Expected: FAIL with the gatedrive: drive already terminal error — proves the
# regression detects the removed fix, and that the mutation actually landed.
mv -f internal/gatedrive/driver.go.bak internal/gatedrive/driver.go
go test -count=1 -run 'TestLoserAfterTerminalSettleReturnsRecordedState' ./internal/gatedrive/
# Expected: PASS again.
```

- [ ] **Step 6: Stress supplement (evidence, not the proof)**

```bash
go test -race -count=25 -run 'TestConcurrentSameOwnerAdvanceRelaunchesOnce|TestLoserAfterTerminalSettleReturnsRecordedState' ./internal/gatedrive/ 2>&1 | tail -5
```

Expected: PASS. Record the command and result; the deterministic test is the proof, this is the supplement.

- [ ] **Step 7: gofmt touched files and commit**

```bash
gofmt -l internal/gatedrive/ && gofmt -w internal/gatedrive/driver.go internal/gatedrive/driver_concurrency_test.go
git add internal/gatedrive/driver.go internal/gatedrive/driver_concurrency_test.go
git commit -m "fix(gatedrive): reservation loser after a terminal settle returns recorded state

A same-owner advance whose reserveRelaunch CAS lands after the winner
persisted the replacement's terminal outcome received the raw
errAlreadyTerminal sentinel as an Advance error (0411's failure). The
loser now reloads and returns the authoritative recorded state, exactly
as the attach-race path already did. Deterministic regression in the
fast corpus; reverting the branch reddens it."
```

---

### Task 3: Apply the measured migrations and shard rebalance

**Files:**
- Modify: the `internal/app` `*_integration_test.go` files named by Task 1's split verdicts (test renames only — assertions untouched)
- Create: one `tests/test_go_integration_app_<topic>.sh` wrapper per justified new shard
- Modify: the affected existing `tests/test_go_integration_app_*.sh` wrappers' `SHARD_PREFIX` (and header prose)
- Modify (only if a **migrate** verdict exists): the source test file moving behind the tag → a sibling `*_integration_test.go` with `//go:build integration`

**Interfaces:**
- Consumes: Task 1's Measurement record verdicts — this task implements **only** the moves that record justifies, and nothing when none do.
- Produces: disjoint `SHARD_PREFIX` declarations that Task 4 budgets and Task 5 guards validate; test names later tasks can list via `DOCKET_SHARD_INSPECT=1 bash tests/test_go_integration_app_<topic>.sh`.

**If Task 1 recorded no split/migrate verdict:** this task is a recorded no-op — state in the task report that measurement did not justify any move (with the numbers), commit nothing here, and continue; do not manufacture a migration to have something to show.

- [ ] **Step 1 (split verdict): rename the measured-oversized group into disjoint child prefixes**

For a shard split, rename the Go test functions so the two (or more) cohesive groups carry non-overlapping prefixes. Example shape for `app_rebase` (substitute the actual groups the per-test durations identified — cohesion first, then balance):

- Group A keeps the wrapper but under a **new, narrower** prefix, e.g. `TestIntegrationFinalizeRebaseGate*`
- Group B becomes e.g. `TestIntegrationFinalizeRebaseRecovery*`
- **No test keeps the bare parent name** `TestIntegrationFinalizeRebase<something ambiguous>` such that one wrapper's `^Prefix` selects another wrapper's tests. Verify disjointness mechanically in Step 3.

Rename only; every test body, subtest, and assertion stays byte-identical. Race-instrumented groups keep a `TestRaceIntegration…` prefix and a `SHARD_MODE=race` wrapper.

- [ ] **Step 2 (split verdict): update/create the declaration-only wrappers**

New wrapper, modeled byte-for-byte on the existing template (only the four commented/declared values change):

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_<topic>.sh — Go integration shard (change 0434):
# <one-line description of the group>, behind the `integration` build tag, prefix
# ^TestIntegrationFinalizeRebase<Group>. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationFinalizeRebase<Group>"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

Update the sibling wrapper's `SHARD_PREFIX` to its narrowed prefix. Prefer reusing an existing sibling shard with headroom over minting a new file (tests/README.md placement rule); create a new wrapper only where the Measurement record justifies it.

- [ ] **Step 2b (migrate verdict only): move a default-corpus test behind the tag**

Move the test (and only what it needs) into a `*_integration_test.go` file in the same package, first line `//go:build integration`, rename with the `TestIntegration` (or `TestRaceIntegration` for concurrency-bearing, assigned to a `SHARD_MODE=race` wrapper) prefix of its owning shard, assertions untouched. Confirm the moved test no longer compiles into the default corpus: `go test -count=1 -run '^<NewName>$' -list '.*' <pkg>` must list nothing without `-tags integration` and list it with.

- [ ] **Step 3: Verify selection disjointness and completeness mechanically**

```bash
for f in tests/test_go_integration_app_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$f"; done
bash tests/test_go_integration_contract.sh
```

Expected: the contract passes — exactly-one ownership (proves no overlapping prefixes), race-direction correspondence, no default-corpus leakage, no empty selection. Then run each touched shard for real:

```bash
bash tests/test_go_integration_app_<each touched wrapper>.sh
```

Expected: every `ok -` line green, `every selected test actually ran and passed (N declared)` with the expected N per group.

- [ ] **Step 4: Commit**

```bash
git add internal/app/ tests/test_go_integration_app_*.sh
git commit -m "chore(0434): rebalance measured-oversized app integration shards into disjoint prefixes"
```

(Adjust the message to what was actually done; a migrate-only outcome says so.)

---

### Task 4: Derive the affected budget ceilings from isolated measurements

**Files:**
- Modify: `tests/runtime-budgets.tsv` (rows for touched/created wrappers only)

**Interfaces:**
- Consumes: Task 3's final wrapper set; Task 1's measurement method.
- Produces: one `parallel` row per wrapper, ceilings Task 5's correspondence guard and the runner enforce.

**If Task 3 was a no-op:** so is this task (existing rows already correspond); record that and continue.

- [ ] **Step 1: Measure each touched/created wrapper isolated and uncached**

```bash
for f in <each touched or created wrapper>; do ( time bash "$f" ) > "${TMPDIR:-/tmp}/docket-0434-evidence/final-iso-$(basename "$f" .sh).log" 2>&1; done
```

Take the **worst** standalone serial reading per wrapper (run twice if the two readings diverge materially).

- [ ] **Step 2: Set each row by the table's rule**

Ceiling = worst isolated serial reading rounded up to the next multiple of 5, plus 5s margin, minimum 10s; every row `parallel` (ADR-0108 — no serial pins). A derived ceiling above 60s means the shard is still too big: go back to Task 3 and split further, or — if the group is genuinely indivisible — record the unmet acceptance condition per Global Constraints instead of raising past the regime. Remove rows for deleted wrappers, add rows for new ones, and leave untouched wrappers' rows alone: never inflate a ceiling to absorb growth.

- [ ] **Step 3: Verify correspondence and commit**

```bash
go test -count=1 -run 'TestRuntimeBudgetsCorrespondence' ./internal/repoguard/
git add tests/runtime-budgets.tsv
git commit -m "chore(0434): derive rebalanced shard ceilings from isolated measurements"
```

---

### Task 5: Partition contract, budget guard, and mutation checks on the coverage boundaries

**Files:**
- No intended durable changes — this task is verification; any red finding is fixed where it points (and that fix committed with its own explanation).

**Interfaces:**
- Consumes: Tasks 2–4's committed state.
- Produces: recorded green guard runs and landed-mutation evidence for the task report.

If Tasks 3–4 were no-ops, run Steps 1 and the build-tag/race mutations of Step 2 against one existing shard anyway — the guards protect the unchanged partition this change leaned on, and the spec requires proving they still bite.

- [ ] **Step 1: Run the guards for real**

```bash
bash tests/test_go_integration_contract.sh
go test -count=1 -run 'TestRuntimeBudgetsCorrespondence' ./internal/repoguard/
```

Expected: PASS.

- [ ] **Step 2: Mutation-check each coverage boundary (backup-copy restores, `-count=1`, verify each mutation landed before believing its result)**

For each mutation: `cp` the file to `.bak`, apply, run the named guard expecting RED, then `mv -f` the backup back and re-run expecting GREEN.

1. **Missing runner / orphaned row:** delete (temporarily) one touched wrapper file → `TestRuntimeBudgetsCorrespondence` must fail naming the row; restore.
2. **Missing budget row:** delete that wrapper's row from `tests/runtime-budgets.tsv` → same guard fails in the other direction; restore.
3. **Overlapping prefix:** set one split sibling's `SHARD_PREFIX` back to the broad parent prefix → `tests/test_go_integration_contract.sh` exactly-one-ownership must fail; restore. (Prove the mutation landed: `DOCKET_SHARD_INSPECT=1 bash <wrapper>` shows the broad prefix.)
4. **Missing build tag:** remove `//go:build integration` from one touched `*_integration_test.go` → the contract's default-corpus-leak check must fail; restore. (Landed-proof: `go test -count=1 -list '^TestIntegration' <pkg>` now lists it untagged.)
5. **Dropped race instrumentation:** flip a `SHARD_MODE="race"` wrapper to `"normal"` → the contract's race-direction check must fail; restore. (Landed-proof: the wrapper's inspection line shows `race=` empty.)
6. **Empty selection:** point one wrapper's `SHARD_PREFIX` at a name that selects nothing → the executor's own `selects at least one tagged test` assert reddens on a real run; restore.

Any mutation that leaves its guard green is a defect to fix now (repo rule: a guard is code), not a note.

- [ ] **Step 3: Record**

Append the mutation matrix (mutation → guard → observed red → restored green) to the task report. Commit only if a guard defect was found and fixed.

---

### Task 6: Final-head whole-suite reliability evidence and baseline comparison

**Files:**
- Modify: `docs/superpowers/plans/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite.md` (append **Final evidence** under the Measurement record)

**Interfaces:**
- Consumes: the final code head (all prior tasks committed); Task 1's baseline logs in `"${TMPDIR:-/tmp}/docket-0434-evidence"`.
- Produces: the committed final-evidence record the results file will cite.

- [ ] **Step 1: Three consecutive whole-suite runs at the final head, default/configured parallelism**

```bash
for i in 1 2 3; do go run ./cmd/docket development test 2>&1 | tee "${TMPDIR:-/tmp}/docket-0434-evidence/final-suite-$i.log"; done
```

Expected: three consecutive green runs. These are direct evidence runs; the governed BUILD gate itself is driven afterwards by docket-build through `docket gate drive advance` slices as usual — never bypass or reset gate accounting to reach the repetition target, and never count a gate attempt twice. Read every budget line in all three logs: screening findings get recorded; any `SERIAL CONFIRMED OVER BUDGET:` must be explained and acted on (split further or report), never absorbed by a ceiling raise. Distinguish change-induced findings from pre-existing ones using the Task 1 baseline logs. A red run here is a real failure to debug at the final head — an isolated green shard is not a substitute.

- [ ] **Step 2: Final isolated timings vs baseline, same host and settings**

Re-run the Task 1 Step 3 isolated loop at the final head; tabulate `wrapper | baseline iso s | final iso s | delta` plus `full suite: baseline wall s (x2) | final wall s (x3)`. Report the measured delta honestly — no unmeasured speedup claims. A material slowdown is a finding to investigate before proceeding; tagging/splitting with no evidence of reduced bottleneck or improved reliability is not acceptance (record it as such if that is what the numbers say).

- [ ] **Step 3: Append the Final evidence section and commit**

Include: the three run results, budget-line inventory with baseline attribution, the timing comparison table, the gatedrive determinism/stress results (from Task 2), the mutation matrix (from Task 5), any non-reproduction or measurement limits, and an explicit list of unmet acceptance conditions (empty if none).

```bash
git add docs/superpowers/plans/2026-09-18-move-load-sensitive-tests-out-of-the-default-parallel-suite.md
git commit -m "chore(0434): final-head reliability evidence and baseline comparison"
```

---

## Self-review notes (plan author)

- Spec §"Measure and classify" → Task 1; §"Reuse tagged feature shards" → Tasks 3–4; §"Resolve the gatedrive terminal interleaving" → Task 2; §"Coverage and validation" → Tasks 5–6. Boundaries section → Global Constraints. ADR-0108 → Global Constraints + Task 4.
- The code-level hypothesis in Task 2 was verified against the current worktree source (`driveSlice`'s reserve branch recognizes only `errRelaunchRaceLost`; `reserveRelaunch`'s CAS returns `errAlreadyTerminal` first on a terminal record; `driveAndPersistClaim` reloads on `relaunchRaceLost`). Task 2 still instructs re-verification and names the false-hypothesis path per the spec.
- Tasks 3/4 are measurement-gated by design: the spec forbids moving anything unmeasured, so their conditional structure (including the recorded no-op path) is the honest shape, not a placeholder.
