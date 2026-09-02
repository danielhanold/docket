package process

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestAtomicWriteModesUnderHostileUmask proves the documented 0700/0600
// modes survive umask 077-style masking because they are chmod'ed, not
// merely requested at creation.
func TestAtomicWriteModesUnderHostileUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	dir := filepath.Join(testsupport.TempDir(t), "run")
	if err := ensurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, "m.json"), map[string]int{"schema": 1}); err != nil {
		t.Fatal(err)
	}
	di, _ := os.Stat(dir)
	fi, _ := os.Stat(filepath.Join(dir, "m.json"))
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %o, want 0700", di.Mode().Perm())
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %o, want 0600", fi.Mode().Perm())
	}
}

// TestEnsurePrivateDirTightensExistingDir proves the explicit chmod is
// load-bearing, not redundant: a run dir may already exist with broader
// permissions (created under a looser umask, or reused), in which case
// MkdirAll is a no-op and only the chmod pins 0700. The hostile-umask test
// above cannot catch removal of the chmod because a *fresh* MkdirAll(0700)
// already yields 0700 under umask 077 — this one exercises the case that
// makes the guard bite.
func TestEnsurePrivateDirTightensExistingDir(t *testing.T) {
	dir := filepath.Join(testsupport.TempDir(t), "run")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil { // defeat umask on setup itself
		t.Fatal(err)
	}
	if err := ensurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("pre-existing broad dir not tightened: mode %o, want 0700", fi.Mode().Perm())
	}
}

// TestAtomicWriteNeverExposesPartialJSON hammers reads during repeated
// replacement: every successful read must parse as complete JSON.
func TestAtomicWriteNeverExposesPartialJSON(t *testing.T) {
	dir := testsupport.TempDir(t)
	path := filepath.Join(dir, "r.json")
	if err := writeAtomicJSON(path, map[string]string{"v": "seed"}); err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	done := make(chan error, 1)
	go func() {
		defer close(done)
		big := make([]byte, 1<<16)
		for i := range big {
			big[i] = 'a'
		}
		for i := 0; i < 200; i++ {
			if err := writeAtomicJSON(path, map[string]string{"v": string(big)}); err != nil {
				done <- err
				return
			}
		}
		stop.Store(true)
	}()
	for !stop.Load() {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read during replacement: %v", err)
		}
		var v map[string]string
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("partial JSON observed (%d bytes): %v", len(raw), err)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// TestAtomicWriteLeavesNoTempFile — a completed write leaves exactly the
// target in the directory.
func TestAtomicWriteLeavesNoTempFile(t *testing.T) {
	dir := testsupport.TempDir(t)
	if err := writeAtomicJSON(filepath.Join(dir, "only.json"), map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "only.json" {
		t.Fatalf("directory not clean after write: %v", entries)
	}
}
