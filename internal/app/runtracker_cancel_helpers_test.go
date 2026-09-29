package app

import (
	"crypto/sha256"
	"encoding/hex"
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

// fakeLaunchReconciler is an injectable runLaunchReconciler: it records each
// (worktree,epoch) pair it was asked to reconcile and returns a canned report/error.
type fakeLaunchReconciler struct {
	report gatedrive.RunLaunchReport
	err    error
	calls  []string
}

func (f *fakeLaunchReconciler) reconcile(worktree, runID string) (gatedrive.RunLaunchReport, error) {
	f.calls = append(f.calls, worktree+"|"+runID)
	return f.report, f.err
}

// okLaunchReconciler is a permissive fake reconciler: every epoch's launch
// obligations are already accounted with no findings, so a cancel test that does not
// exercise the launch-reconciliation path behaves exactly as before the seam existed.
func okLaunchReconciler() *fakeLaunchReconciler {
	return &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{Accounted: true}}
}

// cancelFixture is one prepared cancelable run: a gate record with a parent-held
// authority, an active epoch bound to change 42 with a confirmed claim, a canonical
// feature worktree, and a confirmed worktree execution slot whose process is runDir.
type cancelFixture struct {
	repo     string
	key      string
	runID    string
	worktree string
	runDir   string
	store    *gatedrive.Store
	common   string
}

// newCancelFixture builds a fully authorized cancelable run. slot controls whether a
// worktree execution slot is reserved+confirmed; a run with no slot exercises the
// keyless/standalone path.
func newCancelFixture(t *testing.T, slot bool) cancelFixture {
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
	})
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	ep, err := MintRunRecord(repo, key, "42")
	if err != nil {
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

	fx := cancelFixture{repo: repo, key: key, runID: ep.RunID, worktree: worktree, common: common}
	fx.store = gatedrive.OpenStore(common)
	if slot {
		// The slot records a real owning RunID so the ownership-checked
		// teardown treats it as slotOwned (change 0435) — the same teardown behavior
		// the raw (epoch-less) reservation used to get, now anchored on true epoch
		// ownership rather than the worktree location alone.
		runDir := filepath.Join(worktree, "run-1")
		token, terr := fx.store.ReserveWorktreeExecutionForRun(common, worktree, ep.RunID, nil)
		if terr != nil {
			t.Fatalf("ReserveWorktreeExecutionForRun: %v", terr)
		}
		if cerr := fx.store.ConfirmWorktreeExecution(worktree, token, "run-1", runDir); cerr != nil {
			t.Fatalf("ConfirmWorktreeExecution: %v", cerr)
		}
		fx.runDir = runDir
	}
	return fx
}

// loadRunState reads the epoch's current state.
func loadRunState(t *testing.T, repo, key string) runState {
	t.Helper()
	ep, _, err := LoadRunRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	return ep.State
}

// loadSlotState reads the worktree slot's current state as a string.
func loadSlotState(t *testing.T, store *gatedrive.Store, worktree string) string {
	t.Helper()
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	return string(slot.State)
}

// loadSlotRun reads the worktree slot's current RunID.
func loadSlotRun(t *testing.T, store *gatedrive.Store, worktree string) string {
	t.Helper()
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	return slot.RunID
}

func hasFinding(findings []string, prefix string) bool {
	for _, f := range findings {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// admissionRecordFile returns the worktree slot's record path at the documented
// storage layout (see removeAdmissionRecord): the byte-identity probe the
// retirement-convergence tests use to prove a successor's slot is untouched.
func admissionRecordFile(t *testing.T, common, worktree string) string {
	t.Helper()
	canon, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	sum := sha256.Sum256([]byte(canon))
	return filepath.Join(common, "docket", "gate-admission", "v2", hex.EncodeToString(sum[:]), "record.json")
}
