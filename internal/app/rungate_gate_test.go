package app

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the app-side epoch launch gate tests (change 0437 Task 5). The gate is
// the production gatedrive.EpochLaunchGate the driver's reservation/launch paths run
// their durable reservation body under: it locates the epoch by its public id
// (unique match), holds that key's epoch.lock across a read-only liveness read, and
// runs reserve only when the epoch is active AND bound to the worktree the start
// names. It never writes the epoch record. Every refusal fails closed.

// epochGateFixture mints a real gate-key directory with an ACTIVE epoch bound to a
// canonicalizable worktree, and returns the pieces a gate test drives: repo (for
// epochCAS / bindEpochWorktree), gitCommonDir (for epochLaunchGate + the rungate
// root), the gate key, the public epoch id, and the bound worktree path.
func epochGateFixture(t *testing.T) (repo, common, key, epochID, worktree string) {
	t.Helper()
	repo = newGateRepo(t)
	c, err := gateGitCommonDir(repo)
	if err != nil {
		t.Fatalf("gateGitCommonDir: %v", err)
	}
	common = c
	key = mintTestGateKey(t, repo)
	rec, err := MintEpochRecord(repo, key, "437")
	if err != nil {
		t.Fatalf("MintEpochRecord: %v", err)
	}
	epochID = rec.EpochID
	worktree = testsupport.TempDir(t)
	if err := bindEpochWorktree(repo, key, worktree); err != nil {
		t.Fatalf("bindEpochWorktree: %v", err)
	}
	return repo, common, key, epochID, worktree
}

// rungateRootOf builds the run-epoch registry root the gate scans, the same shape
// epochLaunchGate derives internally.
func rungateRootOf(common string) string {
	return filepath.Join(common, "docket", "rungate")
}

// epochLockHeld reports whether SOMEONE holds the per-key epoch.lock, by attempting
// a non-blocking exclusive flock on a fresh open file description: EWOULDBLOCK means
// the lock is held elsewhere (flock serializes across open descriptions, even within
// one process). It is the deterministic oracle for "the gate holds the epoch lock
// across reserve" — no timing sleep.
func epochLockHeld(t *testing.T, rungateRoot, key string) bool {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(rungateRoot, key, epochLockFileName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open epoch lock: %v", err)
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

// TestEpochLaunchGateAdmitsActiveBoundEpoch proves the gate runs reserve exactly
// once, with a nil error, for an active epoch bound to the worktree the start names.
func TestEpochLaunchGateAdmitsActiveBoundEpoch(t *testing.T) {
	_, common, _, epochID, worktree := epochGateFixture(t)
	gate := epochLaunchGate(common)

	calls := 0
	err := gate(epochID, worktree, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("an active bound epoch must admit, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("reserve must run exactly once, got %d", calls)
	}
}

// TestEpochLaunchGateRefusalMatrix proves every fail-closed refusal: reserve is
// NEVER called and the error carries the mapped fence token (or the typed
// EpochError for a location fault).
func TestEpochLaunchGateRefusalMatrix(t *testing.T) {
	type wantKind int
	const (
		wantCancelled wantKind = iota
		wantStale
		wantEpochError
	)

	cases := []struct {
		name string
		// setup mutates a fresh active-bound fixture and returns the (epochID,
		// worktree) the gate is called with.
		setup func(t *testing.T, repo, common, key, epochID, worktree string) (callEpochID, callWorktree string)
		want  wantKind
	}{
		{
			name: "missing id",
			setup: func(t *testing.T, repo, common, key, epochID, worktree string) (string, string) {
				return "deadbeefdeadbeefdeadbeefdeadbeef", worktree
			},
			want: wantEpochError,
		},
		{
			name: "ambiguous id",
			setup: func(t *testing.T, repo, common, key, epochID, worktree string) (string, string) {
				// A second gate key whose epoch record carries the SAME public id.
				key2 := mintTestGateKey(t, repo)
				if _, err := MintEpochRecord(repo, key2, "437"); err != nil {
					t.Fatalf("mint second epoch: %v", err)
				}
				if err := epochCAS(repo, key2, func(r *EpochRecord) error {
					r.EpochID = epochID
					r.Worktree = worktree
					r.State = EpochActive
					return nil
				}); err != nil {
					t.Fatalf("collide epoch id: %v", err)
				}
				return epochID, worktree
			},
			want: wantEpochError,
		},
		{
			name: "corrupt record",
			setup: func(t *testing.T, repo, common, key, epochID, worktree string) (string, string) {
				path := filepath.Join(rungateRootOf(common), key, epochRecordFileName)
				bad := `{"generation":"g","record":{"schema_version":99,"state":"active","epoch_id":"` + epochID + `"}}`
				if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
					t.Fatalf("corrupt record: %v", err)
				}
				return epochID, worktree
			},
			want: wantEpochError,
		},
		{
			name: "cancelling",
			setup: func(t *testing.T, repo, common, key, epochID, worktree string) (string, string) {
				fenceEpoch(t, repo, key, EpochCancelling)
				return epochID, worktree
			},
			want: wantCancelled,
		},
		{
			name: "cancelled",
			setup: func(t *testing.T, repo, common, key, epochID, worktree string) (string, string) {
				fenceEpoch(t, repo, key, EpochCancelled)
				return epochID, worktree
			},
			want: wantCancelled,
		},
		{
			name: "superseded",
			setup: func(t *testing.T, repo, common, key, epochID, worktree string) (string, string) {
				fenceEpoch(t, repo, key, EpochSuperseded)
				return epochID, worktree
			},
			want: wantStale,
		},
		{
			name: "bound to a different worktree",
			setup: func(t *testing.T, repo, common, key, epochID, worktree string) (string, string) {
				other := testsupport.TempDir(t)
				return epochID, other
			},
			want: wantStale,
		},
		{
			name: "unbound worktree",
			setup: func(t *testing.T, repo, common, key, epochID, worktree string) (string, string) {
				// Clear the epoch's bound Worktree: an unbound epoch owns no worktree.
				if err := epochCAS(repo, key, func(r *EpochRecord) error {
					r.Worktree = ""
					return nil
				}); err != nil {
					t.Fatalf("clear worktree binding: %v", err)
				}
				return epochID, worktree
			},
			want: wantStale,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, common, key, epochID, worktree := epochGateFixture(t)
			callEpochID, callWorktree := tc.setup(t, repo, common, key, epochID, worktree)
			gate := epochLaunchGate(common)

			called := false
			err := gate(callEpochID, callWorktree, func() error {
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
				if !errors.Is(err, ErrStaleRunEpoch) {
					t.Fatalf("want ErrStaleRunEpoch, got %v", err)
				}
			case wantEpochError:
				if _, ok := AsEpochError(err); !ok {
					t.Fatalf("want a typed EpochError, got %v", err)
				}
			}
		})
	}
}

// fenceEpoch flips an epoch to the given fenced/terminal state through the CAS, the
// same durable transition run.cancel/resume drive it into.
func fenceEpoch(t *testing.T, repo, key string, state epochState) {
	t.Helper()
	if err := epochCAS(repo, key, func(r *EpochRecord) error {
		r.State = state
		return nil
	}); err != nil {
		t.Fatalf("fence epoch to %s: %v", state, err)
	}
}

// TestEpochLaunchGatePerformsNoWrite proves the gate never mutates the epoch record:
// its bytes and physical generation are byte-identical before and after both an
// admitted call and a refused call (spec AC6).
func TestEpochLaunchGatePerformsNoWrite(t *testing.T) {
	repo, common, key, epochID, worktree := epochGateFixture(t)
	gate := epochLaunchGate(common)
	path := filepath.Join(rungateRootOf(common), key, epochRecordFileName)

	snapshot := func() ([]byte, string) {
		buf, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read epoch record: %v", err)
		}
		_, gen, err := readStoredEpoch(filepath.Dir(path), "snapshot")
		if err != nil {
			t.Fatalf("readStoredEpoch: %v", err)
		}
		return buf, gen
	}

	// Admitted call.
	before, genBefore := snapshot()
	if err := gate(epochID, worktree, func() error { return nil }); err != nil {
		t.Fatalf("admitted gate: %v", err)
	}
	after, genAfter := snapshot()
	if string(before) != string(after) || genBefore != genAfter {
		t.Fatalf("an admitted gate call must not write the epoch record")
	}

	// Refused call (fence first).
	fenceEpoch(t, repo, key, EpochCancelling)
	before, genBefore = snapshot()
	if err := gate(epochID, worktree, func() error { return nil }); !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("refused gate must be ErrRunCancelled, got %v", err)
	}
	after, genAfter = snapshot()
	if string(before) != string(after) || genBefore != genAfter {
		t.Fatalf("a refused gate call must not write the epoch record")
	}
}

// TestEpochLaunchGateSerializesWithFence proves the gate holds the epoch lock across
// reserve so a concurrent active→cancelling fence serializes against it, and that a
// fence that lands FIRST makes the gate refuse. Ordering is proven by channels and a
// direct non-blocking lock probe — never a timing sleep.
func TestEpochLaunchGateSerializesWithFence(t *testing.T) {
	repo, common, key, epochID, worktree := epochGateFixture(t)
	rungateRoot := rungateRootOf(common)
	gate := epochLaunchGate(common)

	entered := make(chan struct{})
	release := make(chan struct{})
	gateErr := make(chan error, 1)
	lockHeld := make(chan bool, 1)

	go func() {
		gateErr <- gate(epochID, worktree, func() error {
			// The gate must hold the epoch lock while reserve runs.
			lockHeld <- epochLockHeld(t, rungateRoot, key)
			close(entered)
			<-release
			return nil
		})
	}()

	<-entered
	if !<-lockHeld {
		t.Fatalf("the gate must hold the epoch lock across reserve")
	}

	// A concurrent fence CAS must block on the held epoch lock: it cannot complete
	// until reserve returns and the gate releases the lock.
	casErr := make(chan error, 1)
	casStarted := make(chan struct{})
	go func() {
		close(casStarted)
		casErr <- epochCAS(repo, key, func(r *EpochRecord) error {
			r.State = EpochCancelling
			return nil
		})
	}()
	<-casStarted
	select {
	case err := <-casErr:
		t.Fatalf("the fence CAS completed while the gate held the epoch lock: %v", err)
	default:
	}

	close(release)
	if err := <-gateErr; err != nil {
		t.Fatalf("the gate over an active bound epoch must admit: %v", err)
	}
	if err := <-casErr; err != nil {
		t.Fatalf("the fence CAS after release: %v", err)
	}

	// Reverse ordering: the fence has now landed, so a fresh gate call refuses.
	if err := gate(epochID, worktree, func() error {
		t.Fatalf("reserve must not run after the fence landed")
		return nil
	}); !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("a gate after the fence must refuse ErrRunCancelled, got %v", err)
	}
}

// TestFindEpochDirByID proves the unique-match locator: a unique match returns the
// directory and record, zero matches is ErrEpochNotFound, and two matching dirs are
// ErrEpochAmbiguous.
func TestFindEpochDirByID(t *testing.T) {
	repo, common, key, epochID, worktree := epochGateFixture(t)
	rungateRoot := rungateRootOf(common)

	dir, rec, err := findEpochDirByID(rungateRoot, epochID)
	if err != nil {
		t.Fatalf("unique match: %v", err)
	}
	if dir != filepath.Join(rungateRoot, key) {
		t.Fatalf("dir = %q, want %q", dir, filepath.Join(rungateRoot, key))
	}
	if rec.EpochID != epochID {
		t.Fatalf("record epoch id = %q, want %q", rec.EpochID, epochID)
	}

	if _, _, err := findEpochDirByID(rungateRoot, "nomatchnomatchnomatchnomatch1234"); !isEpochKind(err, ErrEpochNotFound) {
		t.Fatalf("zero matches must be ErrEpochNotFound, got %v", err)
	}

	key2 := mintTestGateKey(t, repo)
	if _, err := MintEpochRecord(repo, key2, "437"); err != nil {
		t.Fatalf("mint second epoch: %v", err)
	}
	if err := epochCAS(repo, key2, func(r *EpochRecord) error {
		r.EpochID = epochID
		r.Worktree = worktree
		return nil
	}); err != nil {
		t.Fatalf("collide epoch id: %v", err)
	}
	if _, _, err := findEpochDirByID(rungateRoot, epochID); !isEpochKind(err, ErrEpochAmbiguous) {
		t.Fatalf("two matches must be ErrEpochAmbiguous, got %v", err)
	}
}
