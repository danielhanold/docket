//go:build integration

package gitcli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// ffBranchFixture builds three commits A<-B<-C on main of a fresh invocation
// clone and creates feat/ff at A. It returns the repo, its primary worktree,
// and the three commit ids.
func ffBranchFixture(t *testing.T, c *Client) (Repository, string, ObjectID, ObjectID, ObjectID) {
	t.Helper()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)
	a := commitFile(t, r.Invocation, "ff-a.txt", "a\n", "A")
	b := commitFile(t, r.Invocation, "ff-b.txt", "b\n", "B")
	cc := commitFile(t, r.Invocation, "ff-c.txt", "c\n", "C")
	gitOut(t, r.Invocation, "branch", "feat/ff", string(a))
	return repo, r.Invocation, a, b, cc
}

// TestIntegrationBranchTipFastForwardBranchRef proves FastForwardBranchRef moves
// an unchecked-out branch only by compare-and-swap fast-forward: a moved tip,
// a backward target, and a checked-out branch are each refused with the branch
// left where it was.
func TestIntegrationBranchTipFastForwardBranchRef(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	const branch = RefName("refs/heads/feat/ff")

	t.Run("moves", func(t *testing.T) {
		c := newRealClient(t)
		repo, dir, a, _, cc := ffBranchFixture(t, c)
		if err := c.FastForwardBranchRef(ctx, repo, branch, a, cc); err != nil {
			t.Fatalf("FastForwardBranchRef: %v", err)
		}
		if got := gitOut(t, dir, "rev-parse", string(branch)); got != string(cc) {
			t.Errorf("tip = %s, want C %s", got, cc)
		}
	})

	t.Run("moved tip", func(t *testing.T) {
		c := newRealClient(t)
		repo, dir, a, b, cc := ffBranchFixture(t, c)
		gitOut(t, dir, "update-ref", string(branch), string(b))
		if err := c.FastForwardBranchRef(ctx, repo, branch, a, cc); err == nil {
			t.Fatal("FastForwardBranchRef with a stale expected tip succeeded, want error")
		}
		if got := gitOut(t, dir, "rev-parse", string(branch)); got != string(b) {
			t.Errorf("tip = %s, want unchanged B %s", got, b)
		}
	})

	t.Run("backward", func(t *testing.T) {
		c := newRealClient(t)
		repo, dir, a, _, cc := ffBranchFixture(t, c)
		gitOut(t, dir, "update-ref", string(branch), string(cc))
		err := c.FastForwardBranchRef(ctx, repo, branch, cc, a)
		assertKind(t, err, KindInvalidRepository)
		if got := gitOut(t, dir, "rev-parse", string(branch)); got != string(cc) {
			t.Errorf("tip = %s, want unchanged C %s", got, cc)
		}
	})

	t.Run("checked out", func(t *testing.T) {
		c := newRealClient(t)
		repo, dir, a, _, cc := ffBranchFixture(t, c)
		wt := filepath.Join(testsupport.TempDir(t), "ff-wt")
		gitOut(t, dir, "worktree", "add", "-q", wt, "feat/ff")
		err := c.FastForwardBranchRef(ctx, repo, branch, a, cc)
		assertKind(t, err, KindInvalidRepository)
		if got := gitOut(t, dir, "rev-parse", string(branch)); got != string(a) {
			t.Errorf("tip = %s, want unchanged A %s", got, a)
		}
	})
}
