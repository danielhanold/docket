package testsupport

// The unfiltered-run guard (change 0479). `go test -tags integration ./internal/app/`
// with no -run filter runs that package's whole integration corpus in one process,
// which outlasts go test's default 10m per-package timeout and dies in a
// goroutine-dump panic. The suite never does this (every shard runner passes
// -run "^${SHARD_PREFIX}"); hand-typed and plan-prescribed runs did. The guard
// refuses that run shape right after compile and names the supported forms.
//
// Build split, mirroring InstallNoGitGuard: the decision, the flag reading, and
// the remedy text live here, untagged, so the default build unit-tests them.
// RefuseUnfilteredIntegrationRun is real only under `//go:build integration`
// (unfiltered_guard.go) and a no-op otherwise (unfiltered_guard_off.go).

import (
	"errors"
	"flag"
	"fmt"
	"time"
)

// goTestDefaultTimeout is the -test.timeout `go test` passes when the caller gives
// no -timeout. An explicit -timeout 10m arrives identically and is treated the same.
const goTestDefaultTimeout = 10 * time.Minute

// WholeCorpusTimeout is the -timeout the remedy offers for a deliberate whole-package
// run. Inputs at change 0479: the 39 tests/test_go_integration_app_*.sh ceilings in
// tests/runtime-budgets.tsv sum to 1125s (about 18.75m). A whole-package run compiles
// once and runs the same tests, so that sum bounds its wall from above; 30m is about
// 1.6x it. Recompute from tests/runtime-budgets.tsv when the shard ceilings move.
const WholeCorpusTimeout = "30m"

// refuseUnfilteredRun is the decision: refuse only an unfiltered (-run empty),
// non-listing (-list empty) run at go test's default timeout. -skip is not a filter.
func refuseUnfilteredRun(run, list string, timeout time.Duration) bool {
	return run == "" && list == "" && timeout == goTestDefaultTimeout
}

// UnfilteredRunRemedy is the refusal text for package pkg (module-relative dir),
// naming its shard runners by shardGlob.
func UnfilteredRunRemedy(pkg, shardGlob string) string {
	return fmt.Sprintf("%s: the integration-tagged corpus outlasts go test's default 10m timeout when run whole (change 0479).\n"+
		"Run one shard:   bash <one of %s>\n"+
		"or filter:       go test -tags integration -count=1 -run '^<Prefix>' ./%s/\n"+
		"or run it whole: add -timeout %s",
		pkg, shardGlob, pkg, WholeCorpusTimeout)
}

// checkUnfilteredRun reads test.run, test.list, and test.timeout from fs and
// returns the remedy as an error on refusal, nil to allow. A missing or mistyped
// flag, or an empty pkg/shardGlob, is a setup error: a guard that cannot read its
// inputs must never silently allow.
func checkUnfilteredRun(fs *flag.FlagSet, pkg, shardGlob string) error {
	if pkg == "" || shardGlob == "" {
		return fmt.Errorf("unfiltered-run guard: package %q and shard glob %q must both be non-empty", pkg, shardGlob)
	}
	lookup := func(name string) (*flag.Flag, error) {
		f := fs.Lookup(name)
		if f == nil {
			return nil, fmt.Errorf("%s: unfiltered-run guard: testing flag %s is not registered", pkg, name)
		}
		return f, nil
	}
	runF, err := lookup("test.run")
	if err != nil {
		return err
	}
	listF, err := lookup("test.list")
	if err != nil {
		return err
	}
	timeoutF, err := lookup("test.timeout")
	if err != nil {
		return err
	}
	getter, ok := timeoutF.Value.(flag.Getter)
	if !ok {
		return fmt.Errorf("%s: unfiltered-run guard: testing flag test.timeout is not readable as a duration", pkg)
	}
	timeout, ok := getter.Get().(time.Duration)
	if !ok {
		return fmt.Errorf("%s: unfiltered-run guard: testing flag test.timeout is not a duration (got %T)", pkg, getter.Get())
	}
	if refuseUnfilteredRun(runF.Value.String(), listF.Value.String(), timeout) {
		return errors.New(UnfilteredRunRemedy(pkg, shardGlob))
	}
	return nil
}
