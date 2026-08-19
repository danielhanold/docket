package gitcli

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

// refAbsent reports whether ref does NOT resolve in the repo at dir.
func refAbsent(t *testing.T, dir, ref string) bool {
	t.Helper()
	_, err := gitTry(dir, "rev-parse", "--verify", "--quiet", ref)
	return err != nil
}

// divergentBranches builds, in the invocation repo, a non-conflicting base
// branch and a feature branch that both descend from main, leaves the feature
// branch checked out, and returns their tips. Rebasing the feature onto the base
// replays the feature's one commit (a genuine rewrite, no conflict).
func divergentBranches(t *testing.T, r *testRepos) (featHead, baseHead ObjectID) {
	t.Helper()
	gitOut(t, r.Invocation, "checkout", "-q", "-b", "basebr")
	writeWorktreeFile(t, r.Invocation, "base.txt", "base\n")
	gitOut(t, r.Invocation, "add", "--", "base.txt")
	gitOut(t, r.Invocation, "commit", "-q", "-m", "base commit")
	baseHead = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))

	gitOut(t, r.Invocation, "checkout", "-q", "-b", "featbr", "main")
	writeWorktreeFile(t, r.Invocation, "feature.txt", "feat\n")
	gitOut(t, r.Invocation, "add", "--", "feature.txt")
	gitOut(t, r.Invocation, "commit", "-q", "-m", "feature commit")
	featHead = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	return featHead, baseHead
}

// conflictingBranches builds a shared base commit holding files, then a base
// branch and a feature branch that both rewrite every file differently, so a
// rebase of feature onto base conflicts on all of them. The feature branch is
// left checked out.
func conflictingBranches(t *testing.T, r *testRepos, files []string) (featHead, baseHead ObjectID) {
	t.Helper()
	gitOut(t, r.Invocation, "checkout", "-q", "main")
	for _, fn := range files {
		writeWorktreeFile(t, r.Invocation, fn, "orig\n")
	}
	gitOut(t, r.Invocation, append([]string{"add", "--"}, files...)...)
	gitOut(t, r.Invocation, "commit", "-q", "-m", "shared base")
	c0 := gitOut(t, r.Invocation, "rev-parse", "HEAD")

	gitOut(t, r.Invocation, "checkout", "-q", "-b", "basebr")
	for _, fn := range files {
		writeWorktreeFile(t, r.Invocation, fn, "base\n")
	}
	gitOut(t, r.Invocation, append([]string{"add", "--"}, files...)...)
	gitOut(t, r.Invocation, "commit", "-q", "-m", "base edit")
	baseHead = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))

	gitOut(t, r.Invocation, "checkout", "-q", "-b", "featbr", c0)
	for _, fn := range files {
		writeWorktreeFile(t, r.Invocation, fn, "feat\n")
	}
	gitOut(t, r.Invocation, append([]string{"add", "--"}, files...)...)
	gitOut(t, r.Invocation, "commit", "-q", "-m", "feat edit")
	featHead = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	return featHead, baseHead
}

// TestBeginRebaseNoop proves that rebasing a branch onto an ancestor is a no-op:
// disposition unchanged, HEAD untouched, and the owned orig/base anchors created.
func TestBeginRebaseNoop(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)

	head := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	base := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD~1"))

	st, err := c.BeginRebase(ctx, r.Invocation, head, base, "refs/docket/finalize/1")
	if err != nil {
		t.Fatalf("BeginRebase: %v", err)
	}
	if st.Disposition != RebaseUnchanged {
		t.Errorf("disposition = %q, want %q", st.Disposition, RebaseUnchanged)
	}
	if st.HeadOID != head {
		t.Errorf("status head = %q, want %q", st.HeadOID, head)
	}
	if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != head {
		t.Errorf("HEAD moved on a noop rebase: %q -> %q", head, got)
	}
	if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "--verify", "refs/docket/finalize/1/orig")); got != head {
		t.Errorf("orig owned ref = %q, want %q", got, head)
	}
	if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "--verify", "refs/docket/finalize/1/base")); got != base {
		t.Errorf("base owned ref = %q, want %q", got, base)
	}
}

// TestBeginRebaseRewrites proves a divergent base rewrites history: disposition
// rebased, a new HEAD distinct from orig, and the orig owned ref preserving the
// exact pre-rebase commit.
func TestBeginRebaseRewrites(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)

	featHead, baseHead := divergentBranches(t, r)

	st, err := c.BeginRebase(ctx, r.Invocation, featHead, baseHead, "refs/docket/finalize/2")
	if err != nil {
		t.Fatalf("BeginRebase: %v", err)
	}
	if st.Disposition != RebaseRebased {
		t.Fatalf("disposition = %q, want %q", st.Disposition, RebaseRebased)
	}
	newHead := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	if newHead == featHead {
		t.Errorf("HEAD unchanged after a rewriting rebase: %q", newHead)
	}
	if st.HeadOID != newHead {
		t.Errorf("status head = %q, want %q", st.HeadOID, newHead)
	}
	if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "--verify", "refs/docket/finalize/2/orig")); got != featHead {
		t.Errorf("orig owned ref = %q, want pre-rebase %q", got, featHead)
	}
}

// TestBeginRebaseConflict proves a conflicting rebase reports conflicted with the
// exact unmerged path set, including a path carrying a space (NUL-read, not
// line-split).
func TestBeginRebaseConflict(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)

	files := []string{"conflict.txt", "spa ce.txt"}
	featHead, baseHead := conflictingBranches(t, r, files)

	st, err := c.BeginRebase(ctx, r.Invocation, featHead, baseHead, "refs/docket/finalize/3")
	if err != nil {
		t.Fatalf("BeginRebase: %v", err)
	}
	if st.Disposition != RebaseConflicted {
		t.Fatalf("disposition = %q, want %q", st.Disposition, RebaseConflicted)
	}
	got := append([]string(nil), st.UnmergedPaths...)
	sort.Strings(got)
	want := []string{"conflict.txt", "spa ce.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnmergedPaths = %q, want %q", got, want)
	}
}

// TestBeginRebaseRefusals proves the three preconditions each refuse and leave
// the worktree and owned refs untouched: a dirty tree (error), a wrong expected
// head (error), and a pre-existing foreign rebase (in-progress-foreign, no
// owned refs written).
func TestBeginRebaseRefusals(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)

	t.Run("dirty tree", func(t *testing.T) {
		r := newMainModeRepos(t)
		featHead, baseHead := divergentBranches(t, r)
		writeWorktreeFile(t, r.Invocation, "dirty-untracked.txt", "dirty\n")

		st, err := c.BeginRebase(ctx, r.Invocation, featHead, baseHead, "refs/docket/finalize/4")
		if err == nil {
			t.Fatalf("BeginRebase on a dirty tree returned nil (disposition %q), want error", st.Disposition)
		}
		if _, ok := AsFailure(err); !ok {
			t.Fatalf("error is not a *Failure: %T %v", err, err)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != featHead {
			t.Errorf("HEAD moved on a refused rebase: %q -> %q", featHead, got)
		}
		if !refAbsent(t, r.Invocation, "refs/docket/finalize/4/orig") {
			t.Error("orig owned ref was written despite refusal")
		}
	})

	t.Run("wrong expected head", func(t *testing.T) {
		r := newMainModeRepos(t)
		featHead, baseHead := divergentBranches(t, r)

		st, err := c.BeginRebase(ctx, r.Invocation, baseHead, baseHead, "refs/docket/finalize/5")
		if err == nil {
			t.Fatalf("BeginRebase with a wrong expected head returned nil (disposition %q), want error", st.Disposition)
		}
		if _, ok := AsFailure(err); !ok {
			t.Fatalf("error is not a *Failure: %T %v", err, err)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != featHead {
			t.Errorf("HEAD moved on a refused rebase: %q -> %q", featHead, got)
		}
		if !refAbsent(t, r.Invocation, "refs/docket/finalize/5/orig") {
			t.Error("orig owned ref was written despite refusal")
		}
	})

	t.Run("foreign in-progress rebase", func(t *testing.T) {
		r := newMainModeRepos(t)
		featHead, baseHead := conflictingBranches(t, r, []string{"conflict.txt"})
		// Start a rebase with real git and leave it mid-conflict.
		if _, err := gitTry(r.Invocation, "rebase", string(baseHead)); err == nil {
			t.Fatal("expected the setup rebase to conflict")
		}

		st, err := c.BeginRebase(ctx, r.Invocation, featHead, baseHead, "refs/docket/finalize/6")
		if err != nil {
			t.Fatalf("BeginRebase over a foreign rebase returned error: %v", err)
		}
		if st.Disposition != RebaseInProgressForeign {
			t.Errorf("disposition = %q, want %q", st.Disposition, RebaseInProgressForeign)
		}
		if !refAbsent(t, r.Invocation, "refs/docket/finalize/6/orig") {
			t.Error("orig owned ref was written despite a foreign rebase")
		}
	})
}

// TestStageAndContinueRebaseMultiConflict proves two sequential conflicts drive
// through: the first continue surfaces the second conflict, the second continue
// completes with a rebased disposition and the resolved contents on the tip.
func TestStageAndContinueRebaseMultiConflict(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)

	// Shared base with fileA and fileB.
	gitOut(t, r.Invocation, "checkout", "-q", "main")
	writeWorktreeFile(t, r.Invocation, "fileA", "0\n")
	writeWorktreeFile(t, r.Invocation, "fileB", "0\n")
	gitOut(t, r.Invocation, "add", "--", "fileA", "fileB")
	gitOut(t, r.Invocation, "commit", "-q", "-m", "shared base")
	c0 := gitOut(t, r.Invocation, "rev-parse", "HEAD")

	// Base rewrites both files in one commit.
	gitOut(t, r.Invocation, "checkout", "-q", "-b", "basebr")
	writeWorktreeFile(t, r.Invocation, "fileA", "base\n")
	writeWorktreeFile(t, r.Invocation, "fileB", "base\n")
	gitOut(t, r.Invocation, "add", "--", "fileA", "fileB")
	gitOut(t, r.Invocation, "commit", "-q", "-m", "base edit")
	baseHead := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))

	// Feature: two commits, each touching one file (two sequential conflicts).
	gitOut(t, r.Invocation, "checkout", "-q", "-b", "featbr", c0)
	writeWorktreeFile(t, r.Invocation, "fileA", "feat1\n")
	gitOut(t, r.Invocation, "add", "--", "fileA")
	gitOut(t, r.Invocation, "commit", "-q", "-m", "feat commit 1")
	writeWorktreeFile(t, r.Invocation, "fileB", "feat2\n")
	gitOut(t, r.Invocation, "add", "--", "fileB")
	gitOut(t, r.Invocation, "commit", "-q", "-m", "feat commit 2")
	featHead := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))

	st, err := c.BeginRebase(ctx, r.Invocation, featHead, baseHead, "refs/docket/finalize/7")
	if err != nil {
		t.Fatalf("BeginRebase: %v", err)
	}
	if st.Disposition != RebaseConflicted || !reflect.DeepEqual(st.UnmergedPaths, []string{"fileA"}) {
		t.Fatalf("first conflict = (%q, %q), want (conflicted, [fileA])", st.Disposition, st.UnmergedPaths)
	}

	writeWorktreeFile(t, r.Invocation, "fileA", "resolved\n")
	st2, err := c.StageAndContinueRebase(ctx, r.Invocation, []string{"fileA"})
	if err != nil {
		t.Fatalf("StageAndContinueRebase 1: %v", err)
	}
	if st2.Disposition != RebaseConflicted || !reflect.DeepEqual(st2.UnmergedPaths, []string{"fileB"}) {
		t.Fatalf("second conflict = (%q, %q), want (conflicted, [fileB])", st2.Disposition, st2.UnmergedPaths)
	}

	writeWorktreeFile(t, r.Invocation, "fileB", "resolved\n")
	st3, err := c.StageAndContinueRebase(ctx, r.Invocation, []string{"fileB"})
	if err != nil {
		t.Fatalf("StageAndContinueRebase 2: %v", err)
	}
	if st3.Disposition != RebaseRebased {
		t.Fatalf("final disposition = %q, want %q", st3.Disposition, RebaseRebased)
	}
	if got := gitOut(t, r.Invocation, "show", "HEAD:fileA"); got != "resolved" {
		t.Errorf("fileA on tip = %q, want resolved", got)
	}
	if got := gitOut(t, r.Invocation, "show", "HEAD:fileB"); got != "resolved" {
		t.Errorf("fileB on tip = %q, want resolved", got)
	}
	// No rebase left in progress.
	state, err := c.RebaseState(ctx, r.Invocation)
	if err != nil {
		t.Fatalf("RebaseState: %v", err)
	}
	if state.Disposition == RebaseConflicted || state.Disposition == RebaseInProgressForeign {
		t.Errorf("rebase still in progress after completion: %q", state.Disposition)
	}
}

// TestAbortRebaseRestoresOrig proves abort restores HEAD to the proven orig and
// that a mismatched orig is reported as an error.
func TestAbortRebaseRestoresOrig(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)

	t.Run("restores orig", func(t *testing.T) {
		r := newMainModeRepos(t)
		featHead, baseHead := conflictingBranches(t, r, []string{"conflict.txt"})
		st, err := c.BeginRebase(ctx, r.Invocation, featHead, baseHead, "refs/docket/finalize/8")
		if err != nil {
			t.Fatalf("BeginRebase: %v", err)
		}
		if st.Disposition != RebaseConflicted {
			t.Fatalf("disposition = %q, want conflicted", st.Disposition)
		}
		if err := c.AbortRebase(ctx, r.Invocation, featHead); err != nil {
			t.Fatalf("AbortRebase: %v", err)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != featHead {
			t.Errorf("HEAD after abort = %q, want orig %q", got, featHead)
		}
	})

	t.Run("mismatched orig errors", func(t *testing.T) {
		r := newMainModeRepos(t)
		featHead, baseHead := conflictingBranches(t, r, []string{"conflict.txt"})
		if _, err := c.BeginRebase(ctx, r.Invocation, featHead, baseHead, "refs/docket/finalize/9"); err != nil {
			t.Fatalf("BeginRebase: %v", err)
		}
		// baseHead is a valid but wrong orig id.
		err := c.AbortRebase(ctx, r.Invocation, baseHead)
		if err == nil {
			t.Fatal("AbortRebase with a mismatched orig returned nil, want error")
		}
		if _, ok := AsFailure(err); !ok {
			t.Fatalf("error is not a *Failure: %T %v", err, err)
		}
	})
}

// TestOwnedRefFence proves SetOwnedRef and DeleteOwnedRef refuse any ref outside
// refs/docket/ (a refs/heads name), touch nothing on refusal, and round-trip a
// genuine owned ref.
func TestOwnedRefFence(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	head := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	mainBefore := gitOut(t, r.Invocation, "rev-parse", "refs/heads/main")

	if err := c.SetOwnedRef(ctx, repo, RefName("refs/heads/evil"), head); err == nil {
		t.Error("SetOwnedRef on refs/heads/evil returned nil, want refusal")
	} else if _, ok := AsFailure(err); !ok {
		t.Errorf("SetOwnedRef error is not a *Failure: %T %v", err, err)
	}
	if !refAbsent(t, r.Invocation, "refs/heads/evil") {
		t.Error("SetOwnedRef created refs/heads/evil despite the fence")
	}
	if err := c.DeleteOwnedRef(ctx, repo, RefName("refs/heads/main")); err == nil {
		t.Error("DeleteOwnedRef on refs/heads/main returned nil, want refusal")
	} else if _, ok := AsFailure(err); !ok {
		t.Errorf("DeleteOwnedRef error is not a *Failure: %T %v", err, err)
	}
	if got := gitOut(t, r.Invocation, "rev-parse", "refs/heads/main"); got != mainBefore {
		t.Errorf("refs/heads/main changed after a refused DeleteOwnedRef: %q -> %q", mainBefore, got)
	}

	owned := RefName("refs/docket/test/x")
	if err := c.SetOwnedRef(ctx, repo, owned, head); err != nil {
		t.Fatalf("SetOwnedRef on an owned ref: %v", err)
	}
	if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "--verify", string(owned))); got != head {
		t.Errorf("owned ref = %q, want %q", got, head)
	}
	if err := c.DeleteOwnedRef(ctx, repo, owned); err != nil {
		t.Fatalf("DeleteOwnedRef on an owned ref: %v", err)
	}
	if !refAbsent(t, r.Invocation, string(owned)) {
		t.Error("owned ref still present after DeleteOwnedRef")
	}
}
