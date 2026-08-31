// This file owns scheduling: partitioning targets into a bounded-concurrency
// parallel lane and a strictly-sequential serial lane, and driving both to
// completion while honoring context cancellation. The two lanes exist because
// some suite members are concurrency-unsafe (they contend for a shared global
// resource) and MUST NOT overlap — the serial lane runs those one at a time,
// strictly after the parallel lane has fully drained (wg.Wait). The parallel
// lane caps in-flight work at cfg.Jobs via a buffered-semaphore goroutine pool,
// mirroring the Bash oracle's `-j` bound (scripts/run-tests.sh).
package suiterunner

import (
	"context"
	"io"
	"sort"
	"sync"
	"time"
)

// Config carries the fully-resolved run configuration. The scheduler (Task 3)
// declared the lane-facing fields; run.go (Task 7) extends it IN PLACE with the
// rest of the run's inputs. suiterunner.Run never reads global state it does not
// list here — the CLI layer resolves every field and hands it a complete Config.
type Config struct {
	RepoRoot    string // git toplevel of the checkout under test
	TestsDir    string // default RepoRoot/tests; DOCKET_RUNTESTS_TESTS_DIR overrides
	BudgetsPath string // default RepoRoot/tests/runtime-budgets.tsv; DOCKET_RUNTESTS_BUDGETS overrides

	Bash     string   // path to the bash used for child targets; "" => resolve "bash" on PATH
	Jobs     int      // maximum parallel-lane targets in flight (>=1 or usage error)
	Work     string   // runner-owned scratch root (stat/, logs/, jobs/ live under it); "" => os.MkdirTemp
	ExtraEnv []string // extra env appended to every child (overrides sandbox defaults)

	StatePath string // default <git-common-dir>/docket/development-test-budget-state.tsv; DOCKET_RUNTESTS_STATE overrides
	Strict    bool   // DOCKET_RUNTESTS_STRICT=1 — confirm every candidate and gate on a breach (exit 4)
	Verbose   bool   // reserved false in 0318 (the command exposes no flags)

	// DurationsPath is the DOCKET_RUNTESTS_TEST_DURATIONS injection seam: a TSV
	// `<base>.sh\t<parallel-secs>\t<solo-secs>` that replaces measured durations so
	// the budget machinery reaches deterministic verdicts without sleeping. run.go
	// reads column 2 (parallel) for screening; budgetstate.go reads column 3 (solo)
	// for a confirmation re-run.
	DurationsPath string

	Stdout, Stderr io.Writer // the report streams here; the exit code carries the verdict

	// KillAfter bounds the grace period between the first forwarded interrupt and
	// the SIGKILL escalation. 0 => the signal layer's 5s default.
	KillAfter time.Duration
}

// Schedule partitions targets into the parallel lane and the serial lane. The
// parallel lane is ordered ceiling-descending, then path-ascending — the
// oracle's longest-budget-first order, which keeps the slowest files launching
// earliest so their wall clock overlaps the shorter ones. The serial lane keeps
// discovery (input) order. Schedule is pure and has no side effects.
func Schedule(targets []Target) (par, ser []Target) {
	for _, t := range targets {
		if t.Mode == ModeSerial {
			ser = append(ser, t)
		} else {
			par = append(par, t)
		}
	}
	sort.SliceStable(par, func(i, j int) bool {
		if par[i].Ceiling != par[j].Ceiling {
			return par[i].Ceiling > par[j].Ceiling // longest budget first
		}
		return par[i].Path < par[j].Path // deterministic tiebreak
	})
	return par, ser
}

// runLanes executes the parallel lane with at most cfg.Jobs targets in flight,
// waits for it to drain, then runs the serial lane one target at a time. Each
// successful completion is reported to onDone (used for the stderr ticker) as it
// happens; the durable result files are the authoritative record the aggregator
// later validates. Launching stops as soon as ctx is cancelled — targets that
// were never launched are returned in unlaunched, in lane-then-position order
// (remaining parallel targets first, then remaining serial targets). Targets
// already in flight are allowed to finish; cancellation gates new launches only.
func runLanes(ctx context.Context, cfg Config, par, ser []Target, reg *procRegistry, onDone func(Target, Result)) (unlaunched []Target) {
	jobs := cfg.Jobs
	if jobs < 1 {
		jobs = 1
	}

	// runOne executes a single target and reports a successful completion. An
	// ExecuteTarget error is an infrastructure failure (could not stage the
	// sandbox, start bash, or publish the record): no observed truth exists, so
	// onDone is deliberately NOT called — the aggregator finds the absent durable
	// result and records the target as NoResult, failing closed rather than
	// fabricating an observed pass.
	runOne := func(t Target) {
		res, err := ExecuteTarget(ctx, cfg.Bash, t, cfg.Work, reg, cfg.ExtraEnv)
		if err == nil && onDone != nil {
			onDone(t, res)
		}
	}

	// Parallel lane: a buffered-semaphore goroutine pool bounds in-flight work to
	// jobs. cancelled latches once ctx is observed cancelled, so the remaining
	// targets in both lanes flow straight to unlaunched.
	sem := make(chan struct{}, jobs)
	var wg sync.WaitGroup
	cancelled := false

	for i := range par {
		t := par[i]
		if cancelled {
			unlaunched = append(unlaunched, t)
			continue
		}
		// Non-blocking cancellation check before contending for a slot.
		select {
		case <-ctx.Done():
			cancelled = true
			unlaunched = append(unlaunched, t)
			continue
		default:
		}
		// Acquire a slot, cancellable. If ctx is cancelled while we wait for a
		// slot to free, stop launching.
		select {
		case <-ctx.Done():
			cancelled = true
			unlaunched = append(unlaunched, t)
			continue
		case sem <- struct{}{}:
		}
		// Re-check cancellation after acquiring the slot. A worker that cancels
		// ctx does so BEFORE releasing its slot (see runOne's defer order below),
		// so once a freed slot is observable here ctx.Done() is already closed —
		// this inner check makes "the first completion cancels the rest"
		// deterministic rather than a 50/50 race with the outer select.
		select {
		case <-ctx.Done():
			<-sem
			cancelled = true
			unlaunched = append(unlaunched, t)
			continue
		default:
		}
		wg.Add(1)
		go func() {
			// LIFO defers: onDone (inside runOne) runs first, then the slot is
			// released, then wg.Done — so a callback that cancels ctx is ordered
			// strictly before the slot frees, which the inner re-check above relies
			// on for determinism.
			defer wg.Done()
			defer func() { <-sem }()
			runOne(t)
		}()
	}
	// The serial lane starts strictly after the parallel lane has fully drained:
	// a serial (concurrency-unsafe) target must never overlap any other target.
	wg.Wait()

	// Serial lane: one target at a time in this goroutine, so members never
	// overlap each other or a still-running parallel target (there are none left).
	for i := range ser {
		t := ser[i]
		if cancelled {
			unlaunched = append(unlaunched, t)
			continue
		}
		select {
		case <-ctx.Done():
			cancelled = true
			unlaunched = append(unlaunched, t)
			continue
		default:
		}
		runOne(t)
	}
	return unlaunched
}
