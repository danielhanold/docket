package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// detachedChildCommit creates a detached worktree at parent, writes rel=content,
// commits it with the ambient (fixture) identity, and returns the new commit id.
// The commit object lands in the invocation clone's shared object store, so
// PushLease run against the primary worktree can push it.
func detachedChildCommit(t *testing.T, c *Client, repo Repository, r *testRepos, parent ObjectID, rel, content string) ObjectID {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "child")
	if err := c.AddDetachedWorktree(context.Background(), repo, wt, parent); err != nil {
		t.Fatalf("AddDetachedWorktree: %v", err)
	}
	writeWorktreeFile(t, wt, rel, content)
	gitOut(t, wt, "add", "--", rel)
	gitOut(t, wt, "commit", "-q", "-m", "child "+rel)
	return ObjectID(gitOut(t, wt, "rev-parse", "HEAD"))
}

// TestPushLeaseAppliedWhenExpectedMatches proves a push whose expected old value
// matches the remote target applies, and the origin ref then equals the pushed
// commit.
func TestPushLeaseAppliedWhenExpectedMatches(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	base := ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/main"))
	newCommit := detachedChildCommit(t, c, repo, r, base, "pushed.md", "pushed\n")

	out, err := c.PushLease(ctx, repo, "origin", "refs/heads/main", newCommit, base)
	if err != nil {
		t.Fatalf("PushLease: %v", err)
	}
	if out.Disposition != PushApplied {
		t.Fatalf("Disposition = %q, want %q", out.Disposition, PushApplied)
	}
	if out.Remote != newCommit {
		t.Errorf("Remote = %q, want %q", out.Remote, newCommit)
	}
	if originHead := ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/main")); originHead != newCommit {
		t.Errorf("origin main = %q, want %q", originHead, newCommit)
	}
}

// TestPushLeaseLostWhenRemoteAdvanced proves that when the remote target has been
// advanced to a commit the pushed commit does not contain, the structurally
// rejected push is classified lease-lost with Remote set to the winner.
func TestPushLeaseLostWhenRemoteAdvanced(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	base := ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/main"))
	newCommit := detachedChildCommit(t, c, repo, r, base, "mine.md", "mine\n")

	// A concurrent winner advances origin/main away from base.
	winner := r.writerCommit(t, "main", map[string]string{"winner.md": "winner\n"})
	if winner == base {
		t.Fatal("writerCommit did not advance origin")
	}

	out, err := c.PushLease(ctx, repo, "origin", "refs/heads/main", newCommit, base)
	if err != nil {
		t.Fatalf("PushLease: %v", err)
	}
	if out.Disposition != PushLeaseLost {
		t.Fatalf("Disposition = %q, want %q", out.Disposition, PushLeaseLost)
	}
	if out.Remote != winner {
		t.Errorf("Remote = %q, want winner %q", out.Remote, winner)
	}
	// The loser never landed on origin.
	if originHead := ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/main")); originHead != winner {
		t.Errorf("origin main = %q, want winner %q (loser must not have applied)", originHead, winner)
	}
}

// TestPushLeaseFailedNotLeaseLostOnTransportError proves that a transport-level
// failure (the remote made unreadable) with an otherwise-matching expected value
// classifies as PushFailed, never lease-lost — a non-zero git status alone is
// never a lease loss.
func TestPushLeaseFailedNotLeaseLostOnTransportError(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	base := ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/main"))
	newCommit := detachedChildCommit(t, c, repo, r, base, "pushed.md", "pushed\n")

	if err := os.Chmod(r.Origin, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(r.Origin, 0o755)

	out, err := c.PushLease(ctx, repo, "origin", "refs/heads/main", newCommit, base)
	if err != nil {
		t.Fatalf("PushLease: %v", err)
	}
	if out.Disposition != PushFailed {
		t.Fatalf("Disposition = %q, want %q (transport error is never lease-lost)", out.Disposition, PushFailed)
	}
}

// TestIsAncestorTruthTable proves IsAncestor reports the exact ancestry relation:
// forward true, reverse false, reflexive true, and diverged siblings false both
// ways.
func TestIsAncestorTruthTable(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	base := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	// Linear chain base -> c1 -> c2 in one detached worktree.
	wt := filepath.Join(t.TempDir(), "chain")
	if err := c.AddDetachedWorktree(ctx, repo, wt, base); err != nil {
		t.Fatalf("AddDetachedWorktree: %v", err)
	}
	writeWorktreeFile(t, wt, "c1.md", "1\n")
	gitOut(t, wt, "add", "c1.md")
	gitOut(t, wt, "commit", "-q", "-m", "c1")
	c1 := ObjectID(gitOut(t, wt, "rev-parse", "HEAD"))
	writeWorktreeFile(t, wt, "c2.md", "2\n")
	gitOut(t, wt, "add", "c2.md")
	gitOut(t, wt, "commit", "-q", "-m", "c2")
	c2 := ObjectID(gitOut(t, wt, "rev-parse", "HEAD"))
	// A sibling c3 diverging from base.
	c3 := detachedChildCommit(t, c, repo, r, base, "c3.md", "3\n")

	cases := []struct {
		name      string
		anc, desc ObjectID
		want      bool
	}{
		{"forward", base, c2, true},
		{"reverse", c2, base, false},
		{"reflexive", c1, c1, true},
		{"sibling-fwd", c2, c3, false},
		{"sibling-rev", c3, c2, false},
	}
	for _, tc := range cases {
		got, err := c.IsAncestor(ctx, repo, tc.anc, tc.desc)
		if err != nil {
			t.Fatalf("%s: IsAncestor: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: IsAncestor(%s, %s) = %v, want %v", tc.name, tc.anc, tc.desc, got, tc.want)
		}
	}
}
