package gitcli

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"testing"
)

// TestProbeRemoteBranchValidatesInput proves malformed remote/ref inputs are
// rejected as invalid-request before any git process runs.
func TestProbeRemoteBranchValidatesInput(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	repo := Repository{PrimaryWorktree: testsupport.TempDir(t)}

	for _, rem := range []RemoteName{"", "-o", "or/igin"} {
		_, err := c.ProbeRemoteBranch(ctx, repo, rem, "refs/heads/main")
		assertKind(t, err, KindInvalidRequest)
	}
	for _, ref := range []RefName{"main", "-o", "refs/heads/", ":(top)x"} {
		_, err := c.ProbeRemoteBranch(ctx, repo, "origin", ref)
		assertKind(t, err, KindInvalidRequest)
	}
}
