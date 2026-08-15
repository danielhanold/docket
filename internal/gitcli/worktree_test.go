package gitcli

import (
	"context"
	"path/filepath"
	"testing"
)

// canonForCompare resolves every symlink hop of p, tolerating a nonexistent
// leaf (a removed worktree) by canonicalizing the parent and rejoining the base.
func canonForCompare(t *testing.T, p string) string {
	t.Helper()
	if c, err := filepath.EvalSymlinks(p); err == nil {
		return c
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		t.Fatalf("EvalSymlinks(dir %q): %v", filepath.Dir(p), err)
	}
	return filepath.Join(parent, filepath.Base(p))
}

// canonEq compares two filesystem paths after resolving every symlink hop, so a
// macOS /var -> /private/var difference between git's reported worktree path and
// the test's own spelling never produces a spurious mismatch.
func canonEq(t *testing.T, a, b string) bool {
	t.Helper()
	return canonForCompare(t, a) == canonForCompare(t, b)
}

// findWorktree returns the WorktreeInfo whose Path canonicalizes to want, or
// fails the test.
func findWorktree(t *testing.T, infos []WorktreeInfo, want string) WorktreeInfo {
	t.Helper()
	for _, wi := range infos {
		if canonEq(t, wi.Path, want) {
			return wi
		}
	}
	t.Fatalf("worktree %q not found in %+v", want, infos)
	return WorktreeInfo{}
}

// TestAddDetachedWorktreeRegistersDetachedAndPreservesPrimary proves
// AddDetachedWorktree registers a detached worktree at the requested commit,
// creates no branch, and leaves the primary checkout's HEAD, branch, and index
// untouched.
func TestAddDetachedWorktreeRegistersDetachedAndPreservesPrimary(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	commit := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))

	// Primary checkout state before the add.
	beforeHead := gitOut(t, r.Invocation, "rev-parse", "HEAD")
	beforeBranch := gitOut(t, r.Invocation, "symbolic-ref", "HEAD")
	beforeIndex := string(rawGitOut(t, r.Invocation, "ls-files", "-s", "-z"))
	beforeBranches := gitOut(t, r.Invocation, "for-each-ref", "--format=%(refname)", "refs/heads/")

	wtPath := filepath.Join(t.TempDir(), "detached")
	if err := c.AddDetachedWorktree(ctx, repo, wtPath, commit); err != nil {
		t.Fatalf("AddDetachedWorktree: %v", err)
	}

	infos, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	wi := findWorktree(t, infos, wtPath)
	if !wi.Detached {
		t.Errorf("worktree Detached = false, want true: %+v", wi)
	}
	if wi.Branch != "" {
		t.Errorf("detached worktree Branch = %q, want empty", wi.Branch)
	}
	if wi.Head != commit {
		t.Errorf("worktree Head = %q, want %q", wi.Head, commit)
	}

	// The detached checkout really is detached — no branch was created.
	if got := gitOut(t, r.Invocation, "for-each-ref", "--format=%(refname)", "refs/heads/"); got != beforeBranches {
		t.Errorf("local branch set changed after add:\nbefore %q\nafter  %q", beforeBranches, got)
	}
	if branchExists(r.Invocation, "detached") {
		t.Error("a branch named after the worktree directory was created")
	}

	// Primary checkout untouched.
	if got := gitOut(t, r.Invocation, "rev-parse", "HEAD"); got != beforeHead {
		t.Errorf("primary HEAD changed: %q -> %q", beforeHead, got)
	}
	if got := gitOut(t, r.Invocation, "symbolic-ref", "HEAD"); got != beforeBranch {
		t.Errorf("primary branch changed: %q -> %q", beforeBranch, got)
	}
	if got := string(rawGitOut(t, r.Invocation, "ls-files", "-s", "-z")); got != beforeIndex {
		t.Errorf("primary index changed")
	}
}

// TestRemoveWorktreeForcesDirtyWorktreeAndRejectsUnregistered proves
// RemoveWorktree deregisters a worktree carrying staged and untracked state, and
// returns a typed *Failure (never a panic) for an unregistered path.
func TestRemoveWorktreeForcesDirtyWorktreeAndRejectsUnregistered(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	commit := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	wtPath := filepath.Join(t.TempDir(), "detached")
	if err := c.AddDetachedWorktree(ctx, repo, wtPath, commit); err != nil {
		t.Fatalf("AddDetachedWorktree: %v", err)
	}

	// Give the worktree staged + untracked state so a non-forced remove would
	// refuse.
	writeWorktreeFile(t, wtPath, "staged.md", "staged content\n")
	gitOut(t, wtPath, "add", "staged.md")
	writeWorktreeFile(t, wtPath, "untracked.md", "untracked content\n")

	if err := c.RemoveWorktree(ctx, repo, wtPath); err != nil {
		t.Fatalf("RemoveWorktree on dirty worktree: %v", err)
	}
	infos, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		t.Fatalf("ListWorktrees after remove: %v", err)
	}
	for _, wi := range infos {
		if canonEq(t, wi.Path, wtPath) {
			t.Fatalf("worktree %q still registered after RemoveWorktree", wtPath)
		}
	}

	// Removing an unregistered path returns a typed failure, not a panic.
	err = c.RemoveWorktree(ctx, repo, filepath.Join(t.TempDir(), "never-registered"))
	if err == nil {
		t.Fatal("RemoveWorktree on an unregistered path returned nil, want a failure")
	}
	if _, ok := AsFailure(err); !ok {
		t.Fatalf("RemoveWorktree error is not a *Failure: %T %v", err, err)
	}
}
