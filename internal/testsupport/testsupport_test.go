package testsupport

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A dir with a transient post-test writer must still be removed: simulate a
// trailing writer by recreating a file once from a cleanup registered AFTER
// TempDir (LIFO: it runs before the removal cleanup? No — registered after,
// so it runs FIRST; the file it leaves is the "trailing write" removal must
// absorb via retry).
func TestTempDirRemovalAbsorbsTrailingWrite(t *testing.T) {
	var dir string
	t.Run("inner", func(t *testing.T) {
		dir = TempDir(t)
		// Registered after TempDir => runs before the removal cleanup,
		// planting a file the first RemoveAll pass may race with.
		t.Cleanup(func() {
			if err := os.WriteFile(filepath.Join(dir, "straggler"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("fixture dir survived cleanup: %v", err)
	}
}

func TestDrainRunsBeforeRemoval(t *testing.T) {
	var order []string
	var dir string
	t.Run("inner", func(t *testing.T) {
		dir = TempDir(t)
		DrainOnCleanup(t, func() { order = append(order, "drain") })
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("fixture dir survived cleanup")
	}
	if len(order) != 1 || order[0] != "drain" {
		t.Fatalf("drain did not run before removal: %v", order)
	}
}

func TestWaitQuiescedBounded(t *testing.T) {
	start := time.Now()
	if WaitQuiesced(50*time.Millisecond, time.Millisecond, func() bool { return false }) {
		t.Fatal("probe never true, but WaitQuiesced reported quiesced")
	}
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("returned before deadline")
	}
	n := 0
	if !WaitQuiesced(time.Second, time.Millisecond, func() bool { n++; return n >= 3 }) {
		t.Fatal("probe became true, but WaitQuiesced reported timeout")
	}
}
