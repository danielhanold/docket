//go:build integration

package gitcli

import (
	"context"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// gitMaybe runs real git -C <dir> and tolerates a non-zero exit (e.g. a
// deliberately conflicted merge), returning trimmed stdout without touching the
// testing.T. It never weakens gitOut, which still fails the test on error.
func gitMaybe(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, _ := gitTry(dir, args...)
	return out
}

func TestIntegrationRepoWorktreeCheckoutState(t *testing.T) {
	ctx := context.Background()

	t.Run("attached clean branch", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		st, err := c.WorktreeCheckoutState(ctx, r.Invocation)
		if err != nil {
			t.Fatalf("WorktreeCheckoutState: %v", err)
		}
		if st.Detached || st.Branch != "refs/heads/main" || st.OperationInProgress {
			t.Fatalf("state = %+v, want attached refs/heads/main with no operation", st)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != st.Head {
			t.Fatalf("Head = %s, want %s", st.Head, got)
		}
	})

	t.Run("detached HEAD", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		gitOut(t, r.Invocation, "checkout", "-q", "--detach", "HEAD")
		st, err := c.WorktreeCheckoutState(ctx, r.Invocation)
		if err != nil {
			t.Fatalf("WorktreeCheckoutState: %v", err)
		}
		if !st.Detached || st.Branch != "" {
			t.Fatalf("state = %+v, want detached with empty branch", st)
		}
	})

	t.Run("merge in progress", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		// Build two divergent commits and start a real conflicted merge.
		writeWorktreeFile(t, r.Invocation, "conflict.txt", "local\n")
		gitOut(t, r.Invocation, "add", "--", "conflict.txt")
		gitOut(t, r.Invocation, "commit", "-q", "-m", "local side")
		gitOut(t, r.Invocation, "checkout", "-q", "-b", "other", "HEAD~1")
		writeWorktreeFile(t, r.Invocation, "conflict.txt", "other\n")
		gitOut(t, r.Invocation, "add", "--", "conflict.txt")
		gitOut(t, r.Invocation, "commit", "-q", "-m", "other side")
		gitOut(t, r.Invocation, "checkout", "-q", "main")
		_ = gitMaybe(t, r.Invocation, "merge", "other") // conflicts; ignore exit
		st, err := c.WorktreeCheckoutState(ctx, r.Invocation)
		if err != nil {
			t.Fatalf("WorktreeCheckoutState: %v", err)
		}
		if !st.OperationInProgress {
			t.Fatalf("state = %+v, want OperationInProgress", st)
		}
	})

	t.Run("probe failure is an error, not a zero state", func(t *testing.T) {
		c := newRealClient(t)
		if _, err := c.WorktreeCheckoutState(ctx, testsupport.TempDir(t)); err == nil {
			t.Fatal("want error for a non-repository directory")
		}
	})
}
