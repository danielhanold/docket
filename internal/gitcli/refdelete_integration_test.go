//go:build integration

package gitcli

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"path/filepath"
	"testing"
)

// TestDeleteLocalBranchChecked proves the checked local-branch delete: it
// removes a branch whose tip matches exactly and which is checked out nowhere,
// and refuses (leaving the branch intact) both when the tip has moved and when
// the branch is checked out in a worktree.
func TestIntegrationRepoDeleteLocalBranchChecked(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)

	t.Run("deletes on exact tip", func(t *testing.T) {
		r := newMainModeRepos(t)
		repo := mustDiscover(t, c, r.Invocation)
		tip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		gitOut(t, r.Invocation, "branch", "feat/del", string(tip))

		if err := c.DeleteLocalBranchChecked(ctx, repo, RefName("refs/heads/feat/del"), tip); err != nil {
			t.Fatalf("DeleteLocalBranchChecked: %v", err)
		}
		if branchExists(r.Invocation, "feat/del") {
			t.Error("branch feat/del still present after a checked delete")
		}
	})

	t.Run("refuses on moved tip", func(t *testing.T) {
		r := newMainModeRepos(t)
		repo := mustDiscover(t, c, r.Invocation)
		oldTip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		gitOut(t, r.Invocation, "branch", "feat/del", string(oldTip))
		// Advance HEAD (main) then point the branch at the new commit.
		writeWorktreeFile(t, r.Invocation, "advance.md", "advance\n")
		gitOut(t, r.Invocation, "add", "--", "advance.md")
		gitOut(t, r.Invocation, "commit", "-q", "-m", "advance")
		newTip := gitOut(t, r.Invocation, "rev-parse", "HEAD")
		gitOut(t, r.Invocation, "branch", "-f", "feat/del", newTip)

		err := c.DeleteLocalBranchChecked(ctx, repo, RefName("refs/heads/feat/del"), oldTip)
		if err == nil {
			t.Fatal("DeleteLocalBranchChecked on a moved tip returned nil, want refusal")
		}
		// The explicit tip guard must refuse EARLY — an invalid-repository refusal
		// read off the live tip — before any mutating git command is attempted,
		// not only via update-ref's atomic old-value rejection (command-failed).
		f, ok := AsFailure(err)
		if !ok {
			t.Fatalf("error is not a *Failure: %T %v", err, err)
		}
		if f.Kind != KindInvalidRepository {
			t.Errorf("failure kind = %q, want %q (early tip-mismatch refusal)", f.Kind, KindInvalidRepository)
		}
		if got := gitOut(t, r.Invocation, "rev-parse", "refs/heads/feat/del"); got != newTip {
			t.Errorf("branch tip = %q, want unchanged %q", got, newTip)
		}
	})

	t.Run("refuses while checked out", func(t *testing.T) {
		r := newMainModeRepos(t)
		repo := mustDiscover(t, c, r.Invocation)
		tip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		wtPath := filepath.Join(testsupport.TempDir(t), "co")
		if err := c.AddBranchWorktree(ctx, repo, wtPath, RefName("refs/heads/feat/co"), tip); err != nil {
			t.Fatalf("AddBranchWorktree: %v", err)
		}

		err := c.DeleteLocalBranchChecked(ctx, repo, RefName("refs/heads/feat/co"), tip)
		if err == nil {
			t.Fatal("DeleteLocalBranchChecked on a checked-out branch returned nil, want refusal")
		}
		if _, ok := AsFailure(err); !ok {
			t.Fatalf("error is not a *Failure: %T %v", err, err)
		}
		if !branchExists(r.Invocation, "feat/co") {
			t.Error("checked-out branch feat/co was deleted despite refusal")
		}
	})
}

// TestDeleteRemoteRefLease proves the three-outcome lease-scoped remote delete:
// an exact-tip delete lands (applied) and the ref is gone; a concurrently-moved
// remote is rejected (lease-lost) with the ref retained at the winner; an
// already-absent ref reads as applied (idempotent), distinct from an
// unobservable remote which reads as failed (unknown, retain).
func TestIntegrationRepoDeleteRemoteRefLease(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)

	t.Run("deletes on exact tip", func(t *testing.T) {
		r := newMainModeRepos(t)
		repo := mustDiscover(t, c, r.Invocation)
		tip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		gitOut(t, r.Invocation, "push", "-q", "origin", "HEAD:refs/heads/feat/rdel")

		out, err := c.DeleteRemoteRefLease(ctx, repo, "origin", RefName("refs/heads/feat/rdel"), tip)
		if err != nil {
			t.Fatalf("DeleteRemoteRefLease: %v", err)
		}
		if out.Disposition != PushApplied {
			t.Errorf("disposition = %q, want %q", out.Disposition, PushApplied)
		}
		rr, err := c.ProbeRemoteBranch(ctx, repo, "origin", RefName("refs/heads/feat/rdel"))
		if err != nil {
			t.Fatalf("ProbeRemoteBranch: %v", err)
		}
		if rr.State != RemoteRefAbsent {
			t.Errorf("remote ref state = %q, want absent after delete", rr.State)
		}
	})

	t.Run("concurrent move rejected and retained", func(t *testing.T) {
		r := newMainModeRepos(t)
		repo := mustDiscover(t, c, r.Invocation)
		tip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		gitOut(t, r.Invocation, "push", "-q", "origin", "HEAD:refs/heads/feat/rmove")
		// Another writer advances the remote ref past tip.
		newTip := r.writerCommit(t, "feat/rmove", map[string]string{"x.txt": "x\n"})
		if newTip == tip {
			t.Fatal("test setup: writer did not advance the ref")
		}

		out, err := c.DeleteRemoteRefLease(ctx, repo, "origin", RefName("refs/heads/feat/rmove"), tip)
		if err != nil {
			t.Fatalf("DeleteRemoteRefLease: %v", err)
		}
		if out.Disposition != PushLeaseLost {
			t.Fatalf("disposition = %q, want %q", out.Disposition, PushLeaseLost)
		}
		if out.Remote != newTip {
			t.Errorf("observed remote = %q, want winner %q", out.Remote, newTip)
		}
		rr, err := c.ProbeRemoteBranch(ctx, repo, "origin", RefName("refs/heads/feat/rmove"))
		if err != nil {
			t.Fatalf("ProbeRemoteBranch: %v", err)
		}
		if rr.State != RemoteRefFound || rr.Commit != newTip {
			t.Errorf("remote ref = (%q,%q), want retained at (found,%q)", rr.State, rr.Commit, newTip)
		}
	})

	t.Run("already absent reads applied", func(t *testing.T) {
		r := newMainModeRepos(t)
		repo := mustDiscover(t, c, r.Invocation)
		tip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))

		out, err := c.DeleteRemoteRefLease(ctx, repo, "origin", RefName("refs/heads/feat/never"), tip)
		if err != nil {
			t.Fatalf("DeleteRemoteRefLease: %v", err)
		}
		if out.Disposition != PushApplied {
			t.Errorf("disposition = %q, want %q (already gone is idempotent)", out.Disposition, PushApplied)
		}
	})

	t.Run("unobservable remote reads failed", func(t *testing.T) {
		r := newMainModeRepos(t)
		repo := mustDiscover(t, c, r.Invocation)
		tip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		// A configured remote whose URL points at nothing: the push cannot reach
		// it, and neither can the classification probe.
		broken := filepath.Join(testsupport.TempDir(t), "does-not-exist.git")
		gitOut(t, r.Invocation, "remote", "add", "broken", broken)

		out, err := c.DeleteRemoteRefLease(ctx, repo, "broken", RefName("refs/heads/feat/x"), tip)
		if err != nil {
			t.Fatalf("DeleteRemoteRefLease returned a hard error: %v", err)
		}
		if out.Disposition != PushFailed {
			t.Errorf("disposition = %q, want %q (unknown, retain)", out.Disposition, PushFailed)
		}
	})
}
