package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestWorkspacesRootFollowsStateFolder proves the workspaces root sits beneath
// the per-repo state folder: <common>/dckt/ when that directory exists,
// <common>/docket/ once it is gone.
func TestWorkspacesRootFollowsStateFolder(t *testing.T) {
	common := testsupport.TempDir(t)
	priv := filepath.Join(common, "dckt")
	if err := os.Mkdir(priv, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, want := workspacesRoot(common), filepath.Join(priv, "workspaces"); got != want {
		t.Errorf("private workspacesRoot = %q, want %q", got, want)
	}
	if err := os.Remove(priv); err != nil {
		t.Fatal(err)
	}
	if got, want := workspacesRoot(common), filepath.Join(common, "docket", "workspaces"); got != want {
		t.Errorf("shared workspacesRoot = %q, want %q", got, want)
	}
}
