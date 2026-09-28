package workspace

import (
	"context"
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// The Inspect tests build each StateKind with the real-Git harness primitives
// and assert the classification, the exact DirtyPaths summary, and that Inspect
// mutates nothing — a full tree snapshot before and after every Inspect is
// identical. Malformed and foreign manifests are data (StateForeign with a
// parse detail), never an error; only an unreadable manifest slot is an error.

// inspectOK runs Inspect and fails on error, returning the Inspection.
func inspectOK(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) Inspection {
	t.Helper()
	insp, err := svc.Inspect(context.Background(), InspectRequest{Repository: repo, Target: tgt})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	return insp
}

// assertInspectReadOnly runs Inspect and proves it changed no observable byte of
// the workspace or any preserved worktree.
func assertInspectReadOnly(t *testing.T, svc *Service, r *wsRepos, repo gitcli.Repository, tgt Target) Inspection {
	t.Helper()
	ws := wsPathOf(repo)
	var beforeWs map[string]string
	if _, err := os.Stat(ws); err == nil {
		beforeWs = snapshotTree(t, ws)
	}
	beforePreserve := r.snapshotAll(t)
	insp := inspectOK(t, svc, repo, tgt)
	if beforeWs != nil {
		assertUnchanged(t, beforeWs, ws)
	}
	r.assertAllUnchanged(t, beforePreserve)
	return insp
}
