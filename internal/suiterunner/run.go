// This file owns the orchestration entrypoint Run: the whole flow from discovery
// through the exit code, wiring the pieces the earlier tasks built (discovery,
// budgets, sandbox/execution, scheduling, validation/aggregation, the
// screen-then-confirm budget machinery, and signal handling) into the one
// non-interactive whole-suite runner that backs `docket development test`
// (change 0318; contracted to the final topology by change 0370). Discovery is
// category-declared and fail-closed; the same budget classification and state
// machine, and the same exit contract, follow.
//
// Exit contract (precedence 1 > 3 > 4 > 0): 0 all passed (advisory breaches
// included); 1 a test failed; 2 usage error / runner-internal fail-closed
// (unusable bash, missing/duplicate targets, undeclared/malformed suite category,
// bad env value); 3 a scheduled target produced no valid result; 4 a strict-mode
// confirmed/failed budget breach; 130/143 interrupted by SIGINT/SIGTERM. Every
// internal uncertainty fails closed to a non-zero, attributable exit — never a
// fabricated pass.
//
// EXIT 5 IS RETIRED (change 0370). It formerly signalled a source-hygiene
// preflight violation, run by scripts/check-test-source-hygiene.sh before any
// target executed. That preflight is gone: the surviving shell test surface is
// small and house-style, and its still-meaningful invariant — no maintained
// shell test source carries a backtick the shell would execute at source-read —
// is now a Go guard (internal/repoguard TestNoExecutableBacktickInSuiteSource)
// that runs at the build gate rather than as a per-run preflight.
package suiterunner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Run executes the whole flow and returns the process exit code. It writes the
// deterministic report to cfg.Stdout and the ticker/diagnostics to cfg.Stderr;
// the exit code carries the verdict, so the caller (the CLI) bypasses the JSON
// presenter and returns this integer directly.
func Run(ctx context.Context, cfg Config) int {
	stdout := writerOr(cfg.Stdout, os.Stdout)
	stderr := writerOr(cfg.Stderr, os.Stderr)

	// Resolve and validate bash first: an unusable bash is a fail-closed usage
	// error (exit 2), the "the runner will not start" family. LookPath handles
	// both a bare name (found on PATH) and an absolute path (checked directly).
	bashSpec := cfg.Bash
	if bashSpec == "" {
		bashSpec = "bash"
	}
	resolvedBash, err := exec.LookPath(bashSpec)
	if err != nil {
		fmt.Fprintf(stderr, "development test: bash is not usable (%q): %v\n", bashSpec, err)
		return 2
	}
	cfg.Bash = resolvedBash

	// Jobs must be >= 1 (spec "DOCKET_RUNTESTS_JOBS overrides (>=1 or usage error)").
	if cfg.Jobs < 1 {
		fmt.Fprintf(stderr, "development test: jobs must be >= 1 (got %d)\n", cfg.Jobs)
		return 2
	}

	// Discover the corpus, join the budget table, and validate the WHOLE input
	// set before anything runs. A discovery failure (including an undeclared or
	// malformed suite category), an unreadable budget table, or a target-set
	// violation (missing file, duplicate basename) is a fail-closed usage error
	// (exit 2).
	discovered, err := Discover(cfg.TestsDir)
	if err != nil {
		fmt.Fprintf(stderr, "development test: %v\n", err)
		return 2
	}
	budgets, err := LoadBudgets(cfg.BudgetsPath)
	if err != nil {
		fmt.Fprintf(stderr, "development test: cannot read budget table %q: %v\n", cfg.BudgetsPath, err)
		return 2
	}
	targets, err := ResolveTargets(discovered, budgets)
	if err != nil {
		fmt.Fprintf(stderr, "development test: %v\n", err)
		return 2
	}

	// Runner-owned scratch: stat/, logs/, jobs/ live under it. A caller-supplied
	// Work is honored (the no-result test pre-creates one); otherwise a tempdir
	// this run owns and removes.
	work := cfg.Work
	ownWork := false
	if work == "" {
		w, err := os.MkdirTemp("", "docket-devtest-*")
		if err != nil {
			fmt.Fprintf(stderr, "development test: cannot create work dir: %v\n", err)
			return 2
		}
		work = w
		ownWork = true
	}
	if ownWork {
		defer os.RemoveAll(work)
	}
	cfg.Work = work

	statDir := filepath.Join(work, "stat")
	logsDir := filepath.Join(work, "logs")
	for _, d := range []string{statDir, logsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			fmt.Fprintf(stderr, "development test: cannot create %s: %v\n", d, err)
			return 2
		}
	}

	// Interruption lifecycle: a cancellable child ctx and a signal handler that
	// cancels it, forwards to the child GROUPS, and records which signal fired.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	reg := newProcRegistry()
	fired, stop := InstallSignalHandling(cancel, reg, cfg.KillAfter)
	defer stop()

	// Bound total Go test load before the parallel lane launches (change 0373):
	// each target's sandbox exports this cap as DOCKET_GO_TEST_CONCURRENCY, which
	// the Go wrappers translate into `go test -p` / GOMAXPROCS, so -j targets in
	// flight sum to ~mult*cpus concurrent Go test packages instead of jobs*cpus.
	cfg.GoTestConcurrency = GoTestConcurrency(cfg.Jobs, runtime.NumCPU())

	par, ser := Schedule(targets)
	observed := newObservedSink()
	start := time.Now()
	unlaunched := runLanes(runCtx, cfg, par, ser, reg, observed.onDone)
	wall := int(time.Since(start).Seconds())

	// Interruption classification: a never-launched target is interrupted, and
	// when a signal fired every target with no observed completion is too. The
	// already-durable results of finished targets are still collected, validated,
	// and rendered — an intentional improvement over the Bash oracle, which
	// discards its report on interrupt.
	sig, didFire := fired()
	interrupted := make(map[string]bool)
	for _, tg := range unlaunched {
		interrupted[tg.Base] = true
	}
	if didFire {
		for _, tg := range targets {
			if _, ok := observed.get(tg.Base); !ok {
				interrupted[tg.Base] = true
			}
		}
	}

	outcomes, unknown := ValidateResults(targets, observed.snapshot(), statDir, interrupted)

	// Budget classification, mirroring the oracle's report loop: substitute an
	// injected parallel duration (durations seam column 2) for the measured one,
	// then split on whether the measurement is UNCONTENDED. An uncontended
	// measurement (-j1, or a serial-mode file whose serial-lane wall clock is
	// uncontended) gets the authoritative solo threshold and may be labeled OVER
	// BUDGET directly; a contended parallel-mode measurement is only a SCREENING
	// observation, collected here and classified by the state machine below.
	var screenObs []ScreenObs
	cpus := runtime.NumCPU()
	osName, arch := unameOS(), unameArch()
	for i := range outcomes {
		o := &outcomes[i]
		if o.Kind != OutcomePassed && o.Kind != OutcomeFailed {
			continue
		}
		secs := o.Result.Seconds
		if inj, ok := injectedParallel(cfg.DurationsPath, o.Target.Base); ok {
			secs = inj
			o.Result.Seconds = inj // report the injected value, as the oracle does
		}
		ceil := o.Target.Ceiling
		if cfg.Jobs == 1 || o.Target.Mode == ModeSerial {
			if SoloOver(secs, ceil) {
				o.OverDirect = true
			}
		} else {
			over := o.Result.RC == 0 && ScreenOver(secs, ceil)
			o.Screened = over
			key := ContextKey(o.Target.Path, cfg.Jobs, cpus, osName, arch, ceil, o.Target.Mode)
			screenObs = append(screenObs, ScreenObs{Key: key, Path: o.Target.Path, Ceiling: ceil, Secs: secs, Over: over})
		}
	}

	tally := RenderReport(stdout, outcomes, unknown, wall, cfg.Verbose, cfg.Strict, logsDir)

	// The budget-state machinery runs BELOW the exit-affecting report so it can
	// never change the verdict on its own, and is skipped entirely on an
	// interrupted run (the oracle exits before reaching it). It is fail-open: a
	// missing or unlockable store never fails the run — only a strict-confirmed
	// breach arms exit 4, and that arming survives an unwritable store.
	strictArmed := false
	if !didFire {
		clean := tally.Failed == 0 && tally.NoResult == 0 && tally.Invalid == 0 && len(unknown) == 0
		strictCandidates := 0
		for _, o := range screenObs {
			if o.Over {
				strictCandidates++
			}
		}
		switch {
		case cfg.Strict && cfg.Jobs > 1 && clean && strictCandidates > 0:
			// --strict bypasses the schedule: confirm EVERY current candidate now,
			// ignoring streak history and the one-per-run bound.
			st := Load(cfg.StatePath, cfg.Jobs, stderr)
			for _, o := range screenObs {
				if o.Over {
					st.candidates = append(st.candidates, o)
				}
			}
			strictArmed = st.StrictConfirmCandidates(runCtx, cfg, stdout)
			st.Save()
		case cfg.Jobs > 1 && clean:
			// A qualifying run advances the screening state and takes at most one
			// scheduled solo confirmation, then emits the screening report.
			st := Load(cfg.StatePath, cfg.Jobs, stderr)
			st.ApplyScreenObservations(screenObs)
			st.ScheduleConfirmation(runCtx, cfg, stdout)
			st.Save()
			st.EmitScreenReport(stdout)
		}
	}
	// A direct authoritative crossing (the oracle's `overbudget`) is advisory by
	// default and gates the run only under --strict (exit 4).
	if cfg.Strict && tally.OverDirect > 0 {
		strictArmed = true
	}

	// An interrupted run can never exit 0: its derived code wins over the tally.
	if didFire {
		return InterruptExitCode(sig, didFire)
	}
	return ExitCode(tally, len(unknown), strictArmed)
}

// observedSink collects onDone callbacks — which fire concurrently on the
// parallel lane — into a Base->Result map under a mutex. It is the runner's
// observed truth, which the aggregator later cross-checks against the durable
// files, and its absence for a target is what marks the target unfinished.
type observedSink struct {
	mu     sync.Mutex
	byBase map[string]Result
}

func newObservedSink() *observedSink { return &observedSink{byBase: make(map[string]Result)} }

func (s *observedSink) onDone(t Target, r Result) {
	s.mu.Lock()
	s.byBase[t.Base] = r
	s.mu.Unlock()
}

func (s *observedSink) get(base string) (Result, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.byBase[base]
	return r, ok
}

func (s *observedSink) snapshot() map[string]Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := make(map[string]Result, len(s.byBase))
	for k, v := range s.byBase {
		m[k] = v
	}
	return m
}

// injectedParallel reads the parallel seconds (column 2) for base from the
// durations seam, mirroring the oracle's `awk -F'\t' '$1==b{print $2}'`. Missing
// file, missing row, or a non-numeric field means "use the measured time". Its
// column-3 sibling injectedSolo lives in budgetstate.go.
func injectedParallel(durationsPath, base string) (int, bool) {
	if durationsPath == "" {
		return 0, false
	}
	f, err := os.Open(durationsPath)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), "\t")
		if len(fields) >= 2 && fields[0] == base {
			n, err := strconv.Atoi(strings.TrimSpace(fields[1]))
			if err != nil {
				return 0, false
			}
			return n, true
		}
	}
	return 0, false
}

// unameOS / unameArch render the current platform in the oracle's `uname -s` /
// `uname -m` spelling, so the budget-state context key reads the way a human
// reading the store expects (the mapping is exec-free and stable run-to-run).
func unameOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "Darwin"
	case "linux":
		return "Linux"
	default:
		if runtime.GOOS == "" {
			return "Unknown"
		}
		return strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
	}
}

func unameArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "386":
		return "i386"
	default:
		return runtime.GOARCH // arm64 already matches uname -m
	}
}

// writerOr returns w, or fallback when w is nil, so a Config left without streams
// still writes somewhere rather than panicking on a nil io.Writer.
func writerOr(w io.Writer, fallback io.Writer) io.Writer {
	if w == nil {
		return fallback
	}
	return w
}
