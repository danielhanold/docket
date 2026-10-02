package gatedrive

import (
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// The run tracker's gatedrive stores were renamed by RESET, not migrated
// (ADR-0129 row 38 and Decision 3, change 0471): each store whose persisted names
// changed moved to a new root, so the binary never reads retired state. This file
// is excluded from change 0471's textual rename passes because its fixtures spell
// the RETIRED layout on purpose.

func TestRunTrackerResetStoreRoots(t *testing.T) {
	common := testsupport.TempDir(t)
	s := OpenStore(common)
	for name, c := range map[string]struct{ got, want string }{
		"drives": {s.root, filepath.Join(common, "docket", "gate-drives", "v2")},
		"scopes": {s.scopeRoot, filepath.Join(common, "docket", "gate-scopes", "v2")},
		"locks":  {s.lockRoot, filepath.Join(common, "docket", "worktree-locks")},
		// The suite-budget store carries no retired key, so it keeps v1.
		"budgets": {s.suiteBudgetRoot, filepath.Join(common, "docket", "gate-suite-budgets", "v1")},
	} {
		if c.got != c.want {
			t.Errorf("%s root = %q, want %q", name, c.got, c.want)
		}
	}
}
