package app

import (
	"github.com/danielhanold/docket/internal/gatedrive"
)

// Run-gate completion test helpers shared with default-build (untagged) test files.
// The completion tests themselves live behind the integration tag in
// rungate_complete_integration_test.go (change 0465); these fakes stay untagged
// because other untagged test files still reference them.

// fakeProcessObserver is an injectable processObserver: it answers proven/unproven
// per run dir (falling back to defaultProven), can return a canned error, records
// every handle it observed, and — like the cancel tests' onStop barrier — can inject a
// race via onObserve. It stops nothing.
type fakeProcessObserver struct {
	proven        map[string]bool
	defaultProven bool
	err           error
	calls         []string
	onObserve     func(handle string)
}

func (f *fakeProcessObserver) observeProcessTerminal(runDir string) (bool, error) {
	f.calls = append(f.calls, runDir)
	if f.onObserve != nil {
		f.onObserve(runDir)
	}
	if f.err != nil {
		return false, f.err
	}
	if f.proven != nil {
		if v, ok := f.proven[runDir]; ok {
			return v, nil
		}
	}
	return f.defaultProven, nil
}

// fakeLaunchObserver is an injectable epochLaunchObserver: it records each
// (worktree,epoch) pair, returns a canned report/error, and can inject a race via
// onObserve (a late participant registered after the accounting snapshot but before
// re-enumeration). It settles nothing.
type fakeLaunchObserver struct {
	report    gatedrive.EpochLaunchReport
	err       error
	calls     []string
	onObserve func()
}

func (f *fakeLaunchObserver) observe(worktree, epochID string) (gatedrive.EpochLaunchReport, error) {
	f.calls = append(f.calls, worktree+"|"+epochID)
	if f.onObserve != nil {
		f.onObserve()
	}
	return f.report, f.err
}
