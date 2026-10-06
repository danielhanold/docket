package gatedrive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestOpenStoreRootsFollowStateFolder proves every root OpenStore composes sits
// beneath the per-repo state folder: <common>/dckt/ when that directory exists,
// <common>/docket/ once it is gone.
func TestOpenStoreRootsFollowStateFolder(t *testing.T) {
	common := testsupport.TempDir(t)
	priv := filepath.Join(common, "dckt")
	if err := os.Mkdir(priv, 0o755); err != nil {
		t.Fatal(err)
	}
	assertStoreRootsUnder(t, OpenStore(common), priv)
	if err := os.Remove(priv); err != nil {
		t.Fatal(err)
	}
	assertStoreRootsUnder(t, OpenStore(common), filepath.Join(common, "docket"))
}

func assertStoreRootsUnder(t *testing.T, s *Store, dir string) {
	t.Helper()
	want := Store{
		root:            filepath.Join(dir, "gate-drives", "v2"),
		scopeRoot:       filepath.Join(dir, "gate-scopes", "v2"),
		suiteBudgetRoot: filepath.Join(dir, "gate-suite-budgets", "v1"),
		lockRoot:        filepath.Join(dir, "worktree-locks"),
	}
	if *s != want {
		t.Errorf("OpenStore roots = %+v, want %+v", *s, want)
	}
}
