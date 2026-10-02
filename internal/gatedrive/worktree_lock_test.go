package gatedrive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// fakeObserver answers Observe from a map; a missing dir is an error.
type fakeObserver struct{ states map[string]process.State }

func (f fakeObserver) Observe(runDir string) (*process.Observation, error) {
	st, ok := f.states[runDir]
	if !ok {
		return nil, fmt.Errorf("unobservable")
	}
	return &process.Observation{RunDir: runDir, State: st}, nil
}

func TestTryWorktreeLockRefusesSecondHolder(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	root := testsupport.TempDir(t)
	first, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	_, err = store.TryWorktreeLock(root, nil)
	if !isOwnershipKind(err, ErrWorktreeBusy) {
		t.Fatalf("second acquire = %v, want worktree-busy", err)
	}
	if oe, _ := AsOwnershipError(err); oe.Op != "worktree-admission" {
		t.Fatalf("op = %q", oe.Op)
	}
	first.Release()
	again, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatalf("released lock must readmit with no recovery step: %v", err)
	}
	again.Release()
}

// TestWorktreeLockTakeFileHandsOffOwnership pins the handoff contract: the
// taken file still holds the lock (a second acquire is busy), Release after
// TakeFile is a no-op on the handed-off file, and closing the taken file frees
// the worktree.
func TestWorktreeLockTakeFileHandsOffOwnership(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	root := testsupport.TempDir(t)
	l, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := l.TakeFile()
	if f == nil {
		t.Fatal("TakeFile returned nil on a held lock")
	}
	if again := l.TakeFile(); again != nil {
		t.Fatal("second TakeFile must return nil")
	}
	l.Release() // must not close the handed-off file
	if _, err := store.TryWorktreeLock(root, nil); !isOwnershipKind(err, ErrWorktreeBusy) {
		t.Fatalf("handed-off file must keep the lock held, got %v", err)
	}
	f.Close()
	next, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatalf("closing the handed-off file must free the worktree: %v", err)
	}
	next.Release()
	var nilLock *WorktreeLock
	nilLock.Release()
	if nilLock.TakeFile() != nil {
		t.Fatal("nil lock TakeFile must be nil")
	}
}

// L8: the holder is printed only while its run is running.
func TestBusyRefusalNamesHolderOnlyWhileRunning(t *testing.T) {
	for _, tc := range []struct {
		name  string
		note  *HolderNote
		raw   []byte
		state process.State
		want  bool
	}{
		{"running drive", &HolderNote{Kind: "drive", DriveID: strings.Repeat("a", 32), RunDir: "/runs/0123456789abcdef0123456789abcdef", ChangeID: "490", Owner: "build"}, nil, process.StateRunning, true},
		{"stale note, run passed", &HolderNote{Kind: "drive", DriveID: strings.Repeat("a", 32), RunDir: "/runs/0123456789abcdef0123456789abcdef"}, nil, process.StatePassed, false},
		{"stale note, run vanished", &HolderNote{Kind: "raw", RunDir: "/runs/0123456789abcdef0123456789abcdef", Owner: "raw"}, nil, process.StateVanished, false},
		{"unobservable run", &HolderNote{Kind: "raw", RunDir: "/runs/0123456789abcdef0123456789abcdef", Owner: "raw"}, nil, "", false},
		{"missing note", nil, nil, "", false},
		{"unreadable note", nil, []byte("{not json"), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := OpenStore(testsupport.TempDir(t))
			root := testsupport.TempDir(t)
			held, err := store.TryWorktreeLock(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer held.Release()
			if tc.note != nil {
				held.WriteHolder(*tc.note)
			}
			if tc.raw != nil {
				if err := os.WriteFile(filepath.Join(held.dir, worktreeHolderFile), tc.raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			obs := fakeObserver{states: map[string]process.State{}}
			if tc.note != nil && tc.state != "" {
				obs.states[tc.note.RunDir] = tc.state
			}
			_, err = store.TryWorktreeLock(root, obs)
			oe, ok := AsOwnershipError(err)
			if !ok || oe.Kind != ErrWorktreeBusy {
				t.Fatalf("want worktree-busy, got %v", err)
			}
			if got := oe.Incumbent != nil; got != tc.want {
				t.Fatalf("holder named = %v, want %v (%+v)", got, tc.want, oe.Incumbent)
			}
			if tc.want && (oe.Incumbent.DriveID != tc.note.DriveID || oe.Incumbent.RawRunDir != tc.note.RunDir ||
				oe.Incumbent.ChangeID != tc.note.ChangeID || oe.Incumbent.Owner != tc.note.Owner || oe.Incumbent.Kind != tc.note.Kind) {
				t.Fatalf("holder snapshot %+v does not match note %+v", oe.Incumbent, tc.note)
			}
			if tc.want && oe.Incumbent.RawRunID != filepath.Base(tc.note.RunDir) {
				t.Fatalf("holder run id = %q, want %q", oe.Incumbent.RawRunID, filepath.Base(tc.note.RunDir))
			}
		})
	}
}

// A nil observer can never confirm a holder: the refusal says "holder unknown"
// even over a well-formed note.
func TestBusyRefusalNilObserverIsHolderUnknown(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	root := testsupport.TempDir(t)
	held, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	held.WriteHolder(HolderNote{Kind: "raw", RunDir: "/runs/x", Owner: "raw"})
	_, err = store.TryWorktreeLock(root, nil)
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrWorktreeBusy || oe.Incumbent != nil {
		t.Fatalf("nil observer must refuse busy with no holder, got %v (%+v)", err, oe)
	}
}

func TestWorktreeLockPriorHolderRoundTrips(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	l, err := store.TryWorktreeLock(testsupport.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	if _, ok := l.PriorHolder(); ok {
		t.Fatal("a fresh lock has no prior holder")
	}
	want := HolderNote{Kind: "drive", DriveID: strings.Repeat("b", 32), RunDir: "/runs/y", ChangeID: "7", Owner: "finalize"}
	l.WriteHolder(want)
	got, ok := l.PriorHolder()
	if !ok {
		t.Fatal("PriorHolder after WriteHolder must read the note")
	}
	if got.WrittenAt.IsZero() {
		t.Fatal("WriteHolder must stamp written_at")
	}
	got.WrittenAt = want.WrittenAt
	if got != want {
		t.Fatalf("PriorHolder = %+v, want %+v", got, want)
	}
}

func TestTryWorktreeLockIOErrorIsNotFree(t *testing.T) {
	common := testsupport.TempDir(t)
	// Make <common>/docket a regular file so the lock root cannot be created.
	if err := os.WriteFile(filepath.Join(common, "docket"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := OpenStore(common).TryWorktreeLock(testsupport.TempDir(t), nil)
	if se, ok := AsStoreError(err); !ok || se.Kind != ErrIO {
		t.Fatalf("unopenable lock root must be a typed IO error, got %v", err)
	}
}

func TestWorktreeLockFilesArePrivateAndNeverDeleted(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	l, err := store.TryWorktreeLock(testsupport.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	l.WriteHolder(HolderNote{Kind: "raw", RunDir: "/runs/x", Owner: "raw"})
	l.Release()
	for path, want := range map[string]os.FileMode{
		l.dir: 0o700, filepath.Join(l.dir, worktreeLockFile): 0o600, filepath.Join(l.dir, worktreeHolderFile): 0o600,
	} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s must survive Release: %v", path, err)
		}
		if fi.Mode().Perm() != want {
			t.Fatalf("%s mode %o, want %o", path, fi.Mode().Perm(), want)
		}
	}
}

// The lock directory is keyed on the lowercase hex sha256 of the root, so two
// distinct roots never share a lock.
func TestWorktreeLockKeyedOnRootDigest(t *testing.T) {
	common := testsupport.TempDir(t)
	store := OpenStore(common)
	a, b := testsupport.TempDir(t), testsupport.TempDir(t)
	la, err := store.TryWorktreeLock(a, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer la.Release()
	if want := filepath.Join(common, "docket", "worktree-locks", worktreeLockKey(a)); la.dir != want {
		t.Fatalf("lock dir = %q, want %q", la.dir, want)
	}
	if k := worktreeLockKey(a); len(k) != 64 || strings.ToLower(k) != k {
		t.Fatalf("key %q is not lowercase hex sha256", k)
	}
	lb, err := store.TryWorktreeLock(b, nil)
	if err != nil {
		t.Fatalf("a distinct root must get its own lock: %v", err)
	}
	lb.Release()
}

func TestTryWorktreeLockRefusesRelativeRoot(t *testing.T) {
	_, err := OpenStore(testsupport.TempDir(t)).TryWorktreeLock("relative/root", nil)
	if se, ok := AsStoreError(err); !ok || se.Kind != ErrInvalidID {
		t.Fatalf("relative root = %v, want ErrInvalidID", err)
	}
}
