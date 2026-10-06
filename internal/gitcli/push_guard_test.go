package gitcli

import (
	"context"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestPushRefusesNonBranchRefs proves both lease pushes refuse any ref outside
// refs/heads/ as an invalid request before git runs, so a local refs/docket/
// scratch ref (or any other non-branch ref) can never reach a remote. The repo is
// an empty directory: were the guard missing, git would run there and the push
// would come back as a plain failed disposition with no error, not this refusal.
func TestPushRefusesNonBranchRefs(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	repo := Repository{PrimaryWorktree: testsupport.TempDir(t)}
	const commit = ObjectID("1111111111111111111111111111111111111111")
	const expected = ObjectID("2222222222222222222222222222222222222222")

	for _, ref := range []RefName{"refs/docket/finalize/7/orig", "refs/dckt/x", "refs/tags/v1"} {
		t.Run(string(ref), func(t *testing.T) {
			out, err := c.PushLease(ctx, repo, "origin", ref, commit, expected)
			if out != (PushOutcome{}) {
				t.Errorf("PushLease(%s) outcome = %+v, want zero", ref, out)
			}
			assertKind(t, err, KindInvalidRequest)

			out, err = c.PushCreateLease(ctx, repo, "origin", ref, commit)
			if out != (PushOutcome{}) {
				t.Errorf("PushCreateLease(%s) outcome = %+v, want zero", ref, out)
			}
			assertKind(t, err, KindInvalidRequest)
		})
	}
}
