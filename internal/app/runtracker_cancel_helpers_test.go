package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// Run-cancel test helpers shared with default-build (untagged) test files. The
// cancel tests themselves live behind the integration tag in
// runtracker_cancel_integration_test.go (change 0465); these fixtures, fakes and probes
// stay untagged because other untagged test files still reference them.

// fakeCancelStopper is an injectable cancelStopper: it records every run dir it was
// asked to stop, answers proven/unproven from a per-dir map, and (via onStop) lets a
// test inject a race between the fence and the stop.
type fakeCancelStopper struct {
	proven map[string]bool
	calls  []string
	onStop func(runDir string)
}

func (f *fakeCancelStopper) stopProcess(runDir string) (bool, error) {
	f.calls = append(f.calls, runDir)
	if f.onStop != nil {
		f.onStop(runDir)
	}
	return f.proven[runDir], nil
}

// fakeNativeCanceller records the native handles it was asked to cancel and returns
// a canned error.
type fakeNativeCanceller struct {
	calls []string
	err   error
}

func (f *fakeNativeCanceller) cancelNativeTask(handle string) error {
	f.calls = append(f.calls, handle)
	return f.err
}

// fakeLaunchReconciler is an injectable runLaunchReconciler: it records each run
// context hash it was asked to reconcile and returns a canned report/error.
type fakeLaunchReconciler struct {
	report gatedrive.RunLaunchReport
	err    error
	calls  []string
}

func (f *fakeLaunchReconciler) reconcile(contextHash string) (gatedrive.RunLaunchReport, error) {
	f.calls = append(f.calls, contextHash)
	return f.report, f.err
}

// okLaunchReconciler is a permissive fake reconciler: every run's launch
// obligations are already accounted with no findings, so a cancel test that does not
// exercise the launch-reconciliation path behaves exactly as before the seam existed.
func okLaunchReconciler() *fakeLaunchReconciler {
	return &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{Accounted: true}}
}

// cancelFixture is one prepared cancelable run: a gate record with a parent-held
// authority and a run context, an active run bound to change 42 with a confirmed
// claim, and a canonical feature worktree. contextHash is the record's
// child_context_hash — the hash of cancelFixtureRunContext, so a drive started (or
// seeded) with that raw context is the run's to the launch census (change 0490).
// runDir is a run directory under the worktree that nothing registers: a test that
// needs an execution to stop registers it as a participant or starts a drive.
type cancelFixture struct {
	repo        string
	key         string
	worktree    string
	runDir      string
	store       *gatedrive.Store
	common      string
	contextHash string
}

// cancelFixtureRunContext is the raw run context every cancel fixture's record is
// minted with (each fixture lives in its own repository).
const cancelFixtureRunContext = "cancel-fixture-run-context"

// newCancelFixture builds a fully authorized cancelable run. It writes no gate
// state: no drive, no participant, and no worktree record.
func newCancelFixture(t *testing.T) cancelFixture {
	t.Helper()
	repo := newRunTrackerRepo(t)
	common, err := runTrackerGitCommonDir(repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	key, err := MintRunTrackerRecord(repo, RunTrackerRecord{
		Target:       runStartStoredTarget,
		AttemptLimit: 2,
		Retry:        RetryUnused,
		Disposition:  "run-started",
		ParentCap:    "parent-cap-raw",
		ScopeID:      "scope-1",
		// The run's context hash attributes its drives to the launch census.
		ChildContextHash: runTrackerHashToken(cancelFixtureRunContext),
	})
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	if _, err := MintRunRecord(repo, key, "42"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	if err := ReserveRunTrackerClaim(repo, key, 42, "req-1"); err != nil {
		t.Fatalf("ReserveRunTrackerClaim: %v", err)
	}
	if err := ConfirmRunTrackerClaim(repo, key, 42, "req-1", "rev-1", ""); err != nil {
		t.Fatalf("ConfirmRunTrackerClaim: %v", err)
	}

	worktree := filepath.Join(repo, "feature-wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		r.Worktree = worktree
		return nil
	}); err != nil {
		t.Fatalf("runRecordCAS set worktree: %v", err)
	}

	return cancelFixture{repo: repo, key: key, worktree: worktree, common: common,
		runDir: filepath.Join(worktree, "run-1"), store: gatedrive.OpenStore(common),
		contextHash: runTrackerHashToken(cancelFixtureRunContext)}
}

// loadRunState reads the run's current state.
func loadRunState(t *testing.T, repo, key string) runState {
	t.Helper()
	ep, _, err := LoadRunRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	return ep.State
}

func hasFinding(findings []string, prefix string) bool {
	for _, f := range findings {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}
