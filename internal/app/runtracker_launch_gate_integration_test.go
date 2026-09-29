//go:build integration

package app

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the app-side run launch gate tests (change 0437 Task 5). The gate is
// the production gatedrive.RunLaunchGate the driver's reservation/launch paths run
// their durable reservation body under: it locates the run by its public id
// (unique match), holds that key's run.lock across a read-only liveness read, and
// runs reserve only when the run is active AND bound to the worktree the start
// names. It never writes the run record. Every refusal fails closed.

// runLaunchGateFixture mints a real run-key directory with an ACTIVE run bound to a
// canonicalizable worktree, and returns the pieces a gate test drives: repo (for
// runRecordCAS / bindRunWorktree), gitCommonDir (for runLaunchGate + the run-tracker
// root), the run key, the public run id, and the bound worktree path.
func runLaunchGateFixture(t *testing.T) (repo, common, key, runID, worktree string) {
	t.Helper()
	repo = newRunTrackerRepo(t)
	c, err := runTrackerGitCommonDir(repo)
	if err != nil {
		t.Fatalf("runTrackerGitCommonDir: %v", err)
	}
	common = c
	key = mintTestRunKey(t, repo)
	rec, err := MintRunRecord(repo, key, "437")
	if err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	runID = rec.RunID
	worktree = testsupport.TempDir(t)
	if err := bindRunWorktree(repo, key, worktree); err != nil {
		t.Fatalf("bindRunWorktree: %v", err)
	}
	return repo, common, key, runID, worktree
}

// runTrackerRootOf builds the run registry root the gate scans, the same shape
// runLaunchGate derives internally.
func runTrackerRootOf(common string) string {
	return filepath.Join(common, "docket", runTrackerDirName)
}

// runLockHeld reports whether SOMEONE holds the per-key run.lock, by attempting
// a non-blocking exclusive flock on a fresh open file description: EWOULDBLOCK means
// the lock is held elsewhere (flock serializes across open descriptions, even within
// one process). It is the deterministic oracle for "the gate holds the run lock
// across reserve" — no timing sleep.
func runLockHeld(t *testing.T, runTrackerRoot, key string) bool {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(runTrackerRoot, key, runLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open run lock: %v", err)
	}
	defer f.Close()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return true
		}
		t.Fatalf("non-blocking flock probe: %v", err)
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}

// TestIntegrationRunRecordRunLaunchGateAdmitsActiveBoundRun proves the gate runs reserve exactly
// once, with a nil error, for an active run bound to the worktree the start names.
func TestIntegrationRunRecordRunLaunchGateAdmitsActiveBoundRun(t *testing.T) {
	_, common, _, runID, worktree := runLaunchGateFixture(t)
	gate := runLaunchGate(common)

	calls := 0
	err := gate(runID, worktree, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("an active bound run must admit, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("reserve must run exactly once, got %d", calls)
	}
}

// TestIntegrationRunRecordRunLaunchGateRefusalMatrix proves every fail-closed refusal: reserve is
// NEVER called and the error carries the mapped fence token (or the typed
// RunError for a location fault).
func TestIntegrationRunRecordRunLaunchGateRefusalMatrix(t *testing.T) {
	type wantKind int
	const (
		wantCancelled wantKind = iota
		wantStale
		wantRunError
	)

	cases := []struct {
		name string
		// setup mutates a fresh active-bound fixture and returns the (runID,
		// worktree) the gate is called with.
		setup func(t *testing.T, repo, common, key, runID, worktree string) (callRunID, callWorktree string)
		want  wantKind
	}{
		{
			name: "missing id",
			setup: func(t *testing.T, repo, common, key, runID, worktree string) (string, string) {
				return "deadbeefdeadbeefdeadbeefdeadbeef", worktree
			},
			want: wantRunError,
		},
		{
			name: "ambiguous id",
			setup: func(t *testing.T, repo, common, key, runID, worktree string) (string, string) {
				// A second run key whose run record carries the SAME public id.
				key2 := mintTestRunKey(t, repo)
				if _, err := MintRunRecord(repo, key2, "437"); err != nil {
					t.Fatalf("mint second run: %v", err)
				}
				if err := runRecordCAS(repo, key2, func(r *RunRecord) error {
					r.RunID = runID
					r.Worktree = worktree
					r.State = RunActive
					return nil
				}); err != nil {
					t.Fatalf("collide run id: %v", err)
				}
				return runID, worktree
			},
			want: wantRunError,
		},
		{
			name: "corrupt record",
			setup: func(t *testing.T, repo, common, key, runID, worktree string) (string, string) {
				path := filepath.Join(runTrackerRootOf(common), key, runRecordFileName)
				bad := `{"generation":"g","record":{"schema_version":99,"state":"active","run_id":"` + runID + `"}}`
				if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
					t.Fatalf("corrupt record: %v", err)
				}
				return runID, worktree
			},
			want: wantRunError,
		},
		{
			name: "cancelling",
			setup: func(t *testing.T, repo, common, key, runID, worktree string) (string, string) {
				fenceRun(t, repo, key, RunCancelling)
				return runID, worktree
			},
			want: wantCancelled,
		},
		{
			name: "cancelled",
			setup: func(t *testing.T, repo, common, key, runID, worktree string) (string, string) {
				fenceRun(t, repo, key, RunCancelled)
				return runID, worktree
			},
			want: wantCancelled,
		},
		{
			name: "superseded",
			setup: func(t *testing.T, repo, common, key, runID, worktree string) (string, string) {
				fenceRun(t, repo, key, RunSuperseded)
				return runID, worktree
			},
			want: wantStale,
		},
		{
			name: "bound to a different worktree",
			setup: func(t *testing.T, repo, common, key, runID, worktree string) (string, string) {
				other := testsupport.TempDir(t)
				return runID, other
			},
			want: wantStale,
		},
		{
			name: "unbound worktree",
			setup: func(t *testing.T, repo, common, key, runID, worktree string) (string, string) {
				// Clear the run's bound Worktree: an unbound run owns no worktree.
				if err := runRecordCAS(repo, key, func(r *RunRecord) error {
					r.Worktree = ""
					return nil
				}); err != nil {
					t.Fatalf("clear worktree binding: %v", err)
				}
				return runID, worktree
			},
			want: wantStale,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, common, key, runID, worktree := runLaunchGateFixture(t)
			callRunID, callWorktree := tc.setup(t, repo, common, key, runID, worktree)
			gate := runLaunchGate(common)

			called := false
			err := gate(callRunID, callWorktree, func() error {
				called = true
				return nil
			})
			if called {
				t.Fatalf("a refusal must never call reserve")
			}
			if err == nil {
				t.Fatalf("a refusal must return an error")
			}
			switch tc.want {
			case wantCancelled:
				if !errors.Is(err, ErrRunCancelled) {
					t.Fatalf("want ErrRunCancelled, got %v", err)
				}
			case wantStale:
				if !errors.Is(err, ErrStaleRunID) {
					t.Fatalf("want ErrStaleRunID, got %v", err)
				}
			case wantRunError:
				if _, ok := AsRunError(err); !ok {
					t.Fatalf("want a typed RunError, got %v", err)
				}
			}
		})
	}
}

// TestIntegrationRunRecordRunLaunchGateRefusesCompletingAndCompleted: the launch gate refuses a start
// (or a delayed-ticket relaunch) on a completing or completed run (change 0441) with
// the distinct ErrRunCompleted token — its refusal is what later settles a pre-fence
// never-launched ticket terminal — and never runs reserve.
func TestIntegrationRunRecordRunLaunchGateRefusesCompletingAndCompleted(t *testing.T) {
	for _, s := range []runState{RunCompleting, RunCompleted} {
		t.Run(string(s), func(t *testing.T) {
			repo, common, key, runID, worktree := runLaunchGateFixture(t)
			fenceRun(t, repo, key, s)
			gate := runLaunchGate(common)

			called := false
			err := gate(runID, worktree, func() error {
				called = true
				return nil
			})
			if called {
				t.Fatalf("a completing/completed refusal must never call reserve")
			}
			if !errors.Is(err, ErrRunCompleted) {
				t.Fatalf("want ErrRunCompleted, got %v", err)
			}
		})
	}
}

// TestIntegrationRunRecordRunRevokedResolverRevokesCompletingAndCompleted: the takeover revocation
// resolver reports revoked for a completing or completed run (change 0441), mirroring
// the cancelled/superseded cases — a takeover of a completing/completed run refuses,
// and explicit references to a completed run remain revoked.
func TestIntegrationRunRecordRunRevokedResolverRevokesCompletingAndCompleted(t *testing.T) {
	for _, s := range []runState{RunCompleting, RunCompleted} {
		t.Run(string(s), func(t *testing.T) {
			repo, common, key, runID, _ := runLaunchGateFixture(t)
			fenceRun(t, repo, key, s)
			revoked, err := runRevokedResolver(common)(runID)
			if err != nil {
				t.Fatalf("resolver err: %v", err)
			}
			if !revoked {
				t.Fatalf("state %q must be revoked", s)
			}
		})
	}
}

// fenceRun flips a run to the given fenced/terminal state through the CAS, the
// same durable transition run.cancel/resume drive it into.
func fenceRun(t *testing.T, repo, key string, state runState) {
	t.Helper()
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		r.State = state
		return nil
	}); err != nil {
		t.Fatalf("fence run to %s: %v", state, err)
	}
}

// TestIntegrationRunRecordRunLaunchGatePerformsNoWrite proves the gate never mutates the run record:
// its bytes and physical generation are byte-identical before and after both an
// admitted call and a refused call (spec AC6).
func TestIntegrationRunRecordRunLaunchGatePerformsNoWrite(t *testing.T) {
	repo, common, key, runID, worktree := runLaunchGateFixture(t)
	gate := runLaunchGate(common)
	path := filepath.Join(runTrackerRootOf(common), key, runRecordFileName)

	snapshot := func() ([]byte, string) {
		buf, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read run record: %v", err)
		}
		_, gen, err := readStoredRun(filepath.Dir(path), "snapshot")
		if err != nil {
			t.Fatalf("readStoredRun: %v", err)
		}
		return buf, gen
	}

	// Admitted call.
	before, genBefore := snapshot()
	if err := gate(runID, worktree, func() error { return nil }); err != nil {
		t.Fatalf("admitted gate: %v", err)
	}
	after, genAfter := snapshot()
	if string(before) != string(after) || genBefore != genAfter {
		t.Fatalf("an admitted gate call must not write the run record")
	}

	// Refused call (fence first).
	fenceRun(t, repo, key, RunCancelling)
	before, genBefore = snapshot()
	if err := gate(runID, worktree, func() error { return nil }); !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("refused gate must be ErrRunCancelled, got %v", err)
	}
	after, genAfter = snapshot()
	if string(before) != string(after) || genBefore != genAfter {
		t.Fatalf("a refused gate call must not write the run record")
	}
}

// TestRaceIntegrationAppConcurrencyRunLaunchGateSerializesWithFence proves the gate holds the run lock across
// reserve so a concurrent active→cancelling fence serializes against it, and that a
// fence that lands FIRST makes the gate refuse. Ordering is proven by channels and a
// direct non-blocking lock probe — never a timing sleep.
// Race shard (change 0465): a launch-gate reserve and a run fence CAS run in two goroutines against one run lock.
func TestRaceIntegrationAppConcurrencyRunLaunchGateSerializesWithFence(t *testing.T) {
	repo, common, key, runID, worktree := runLaunchGateFixture(t)
	runTrackerRoot := runTrackerRootOf(common)
	gate := runLaunchGate(common)

	entered := make(chan struct{})
	release := make(chan struct{})
	runTrackerErr := make(chan error, 1)
	lockHeld := make(chan bool, 1)

	go func() {
		runTrackerErr <- gate(runID, worktree, func() error {
			// The gate must hold the run lock while reserve runs.
			lockHeld <- runLockHeld(t, runTrackerRoot, key)
			close(entered)
			<-release
			return nil
		})
	}()

	<-entered
	if !<-lockHeld {
		t.Fatalf("the gate must hold the run lock across reserve")
	}

	// A concurrent fence CAS must block on the held run lock: it cannot complete
	// until reserve returns and the gate releases the lock.
	casErr := make(chan error, 1)
	casStarted := make(chan struct{})
	go func() {
		close(casStarted)
		casErr <- runRecordCAS(repo, key, func(r *RunRecord) error {
			r.State = RunCancelling
			return nil
		})
	}()
	<-casStarted
	select {
	case err := <-casErr:
		t.Fatalf("the fence CAS completed while the gate held the run lock: %v", err)
	default:
	}

	close(release)
	if err := <-runTrackerErr; err != nil {
		t.Fatalf("the gate over an active bound run must admit: %v", err)
	}
	if err := <-casErr; err != nil {
		t.Fatalf("the fence CAS after release: %v", err)
	}

	// Reverse ordering: the fence has now landed, so a fresh gate call refuses.
	if err := gate(runID, worktree, func() error {
		t.Fatalf("reserve must not run after the fence landed")
		return nil
	}); !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("a gate after the fence must refuse ErrRunCancelled, got %v", err)
	}
}

// TestIntegrationRunRecordFindRunDirByID proves the unique-match locator: a unique match returns the
// directory and record, zero matches is ErrRunNotFound, and two matching dirs are
// ErrRunAmbiguous.
func TestIntegrationRunRecordFindRunDirByID(t *testing.T) {
	repo, common, key, runID, worktree := runLaunchGateFixture(t)
	runTrackerRoot := runTrackerRootOf(common)

	dir, rec, err := findRunDirByID(runTrackerRoot, runID)
	if err != nil {
		t.Fatalf("unique match: %v", err)
	}
	if dir != filepath.Join(runTrackerRoot, key) {
		t.Fatalf("dir = %q, want %q", dir, filepath.Join(runTrackerRoot, key))
	}
	if rec.RunID != runID {
		t.Fatalf("record run id = %q, want %q", rec.RunID, runID)
	}

	if _, _, err := findRunDirByID(runTrackerRoot, "nomatchnomatchnomatchnomatch1234"); !isRunKind(err, ErrRunNotFound) {
		t.Fatalf("zero matches must be ErrRunNotFound, got %v", err)
	}

	key2 := mintTestRunKey(t, repo)
	if _, err := MintRunRecord(repo, key2, "437"); err != nil {
		t.Fatalf("mint second run: %v", err)
	}
	if err := runRecordCAS(repo, key2, func(r *RunRecord) error {
		r.RunID = runID
		r.Worktree = worktree
		return nil
	}); err != nil {
		t.Fatalf("collide run id: %v", err)
	}
	if _, _, err := findRunDirByID(runTrackerRoot, runID); !isRunKind(err, ErrRunAmbiguous) {
		t.Fatalf("two matches must be ErrRunAmbiguous, got %v", err)
	}
}
