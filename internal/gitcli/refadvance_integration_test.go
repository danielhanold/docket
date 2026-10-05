//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// newUnattachedBranchFixture: branch "meta" at base (checked out nowhere) and a
// later commit on main (target) descending from base.
func newUnattachedBranchFixture(t *testing.T) (r *testRepos, base, target ObjectID) {
	t.Helper()
	r = newMainModeRepos(t)
	base = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	gitOut(t, r.Invocation, "branch", "meta", string(base))
	writeWorktreeFile(t, r.Invocation, "target.txt", "target\n")
	gitOut(t, r.Invocation, "add", "--", "target.txt")
	gitOut(t, r.Invocation, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "target")
	target = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	return r, base, target
}

func TestIntegrationRepoAdvanceBranchChecked(t *testing.T) {
	ctx := context.Background()

	t.Run("advances an unattached branch", func(t *testing.T) {
		r, base, target := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("AdvanceBranchChecked: %v", err)
		}
		if got := metaTip(t, r); got != target {
			t.Fatalf("meta = %s, want %s", got, target)
		}
	})

	t.Run("equal tips are a verified no-op", func(t *testing.T) {
		r, base, _ := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, base); err != nil {
			t.Fatalf("AdvanceBranchChecked equal tips: %v", err)
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want %s", got, base)
		}
	})

	t.Run("stale expected tip refuses", func(t *testing.T) {
		r, base, target := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", target, target); err == nil {
			t.Fatal("advance from a stale expected tip succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})

	t.Run("branch checked out in a worktree refuses", func(t *testing.T) {
		r, base, target := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		wt := filepath.Join(testsupport.TempDir(t), "holder")
		gitOut(t, r.Invocation, "worktree", "add", "-q", wt, "meta")
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, target); err == nil {
			t.Fatal("advance of a checked-out branch succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})

	t.Run("target not descending refuses", func(t *testing.T) {
		r, base, _ := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		tree := gitOut(t, r.Invocation, "rev-parse", "HEAD^{tree}")
		orphan := ObjectID(gitOut(t, r.Invocation, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit-tree", tree, "-m", "orphan"))
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, orphan); err == nil {
			t.Fatal("advance to a non-descendant succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})

	t.Run("repository hooks never run", func(t *testing.T) {
		r, base, target := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		sentinel := filepath.Join(testsupport.TempDir(t), "hook-ran")
		hooks := installHooks(t, sentinel, 1, "reference-transaction")
		gitOut(t, r.Invocation, "config", "core.hooksPath", hooks)
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("AdvanceBranchChecked with a failing reference-transaction hook: %v", err)
		}
		if b, err := os.ReadFile(sentinel); err == nil {
			t.Fatalf("a repository hook ran: %q", b)
		}
	})
}
