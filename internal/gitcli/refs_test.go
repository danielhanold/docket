package gitcli

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"testing"
)

// TestRefsValidationBlocksSmuggling proves ref/remote validation rejects
// unqualified, option-shaped, pathspec-magic, and non-branch inputs with
// invalid-request before any git process starts. The client is a "dump" helper:
// had a spawn happened, the flow would surface a different kind (invalid-output
// from the bogus rev-parse answer), so an invalid-request verdict proves the
// short-circuit.
func TestRefsValidationBlocksSmuggling(t *testing.T) {
	c := helperClient(t, "dump")
	ctx := context.Background()
	repo := Repository{PrimaryWorktree: testsupport.TempDir(t), CommonDir: testsupport.TempDir(t)}

	for _, br := range []RefName{"main", "-o", ":(top)x", "refs/tags/v1", "heads/main", "refs/heads/"} {
		_, err := c.FetchBranch(ctx, repo, "origin", br)
		assertKind(t, err, KindInvalidRequest)
	}
	for _, rem := range []RemoteName{"-o", "or/igin", ""} {
		_, err := c.RemoteDefaultBranch(ctx, repo, rem)
		assertKind(t, err, KindInvalidRequest)
		_, err = c.FetchBranch(ctx, repo, rem, "refs/heads/main")
		assertKind(t, err, KindInvalidRequest)
	}
	_, err := c.ResolveRef(ctx, repo, "main")
	assertKind(t, err, KindInvalidRequest)
}
