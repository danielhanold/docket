//go:build integration

package gitcli

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
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
func TestIntegrationRepoAddDetachedWorktreeRegistersDetachedAndPreservesPrimary(t *testing.T) {
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

	wtPath := filepath.Join(testsupport.TempDir(t), "detached")
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
func TestIntegrationRepoRemoveWorktreeForcesDirtyWorktreeAndRejectsUnregistered(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	commit := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	wtPath := filepath.Join(testsupport.TempDir(t), "detached")
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
	err = c.RemoveWorktree(ctx, repo, filepath.Join(testsupport.TempDir(t), "never-registered"))
	if err == nil {
		t.Fatal("RemoveWorktree on an unregistered path returned nil, want a failure")
	}
	if _, ok := AsFailure(err); !ok {
		t.Fatalf("RemoveWorktree error is not a *Failure: %T %v", err, err)
	}
}

// registeredAt reports whether any worktree canonicalizes to want.
func registeredAt(t *testing.T, infos []WorktreeInfo, want string) bool {
	t.Helper()
	for _, wi := range infos {
		if canonEq(t, wi.Path, want) {
			return true
		}
	}
	return false
}

// TestAddBranchWorktreeCreatesBranchAndRefusesReset proves AddBranchWorktree
// creates a NEW branch at exactly startCommit and attaches a worktree to it,
// and that a second call for an already-existing branch is command-failed with
// no reset of the existing branch tip — the observable proof that -B/reset
// semantics are absent.
func TestIntegrationRepoAddBranchWorktreeCreatesBranchAndRefusesReset(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	c1 := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	branch := RefName("refs/heads/feat/new-x")
	wtPath := filepath.Join(testsupport.TempDir(t), "new-x")

	if err := c.AddBranchWorktree(ctx, repo, wtPath, branch, c1); err != nil {
		t.Fatalf("AddBranchWorktree: %v", err)
	}

	infos, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	wi := findWorktree(t, infos, wtPath)
	if wi.Detached {
		t.Errorf("branch worktree Detached = true, want false: %+v", wi)
	}
	if wi.Branch != branch {
		t.Errorf("worktree Branch = %q, want %q", wi.Branch, branch)
	}
	if wi.Head != c1 {
		t.Errorf("worktree Head = %q, want %q", wi.Head, c1)
	}
	if got := gitOut(t, wtPath, "symbolic-ref", "HEAD"); got != string(branch) {
		t.Errorf("worktree symbolic HEAD = %q, want %q", got, branch)
	}
	if got := gitOut(t, r.Invocation, "rev-parse", string(branch)); got != string(c1) {
		t.Errorf("branch tip = %q, want startCommit %q", got, c1)
	}

	// Advance main to a distinct commit c2, then re-add the SAME branch pointed
	// at c2. -B would reset feat/new-x to c2; the plain -b path must fail and
	// leave the branch at c1.
	writeWorktreeFile(t, r.Invocation, "advance.md", "advance\n")
	gitOut(t, r.Invocation, "add", "advance.md")
	gitOut(t, r.Invocation, "commit", "-q", "-m", "advance main")
	c2 := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	if c2 == c1 {
		t.Fatal("test setup: c2 == c1")
	}

	wtPath2 := filepath.Join(testsupport.TempDir(t), "new-x-2")
	err = c.AddBranchWorktree(ctx, repo, wtPath2, branch, c2)
	if err == nil {
		t.Fatal("AddBranchWorktree onto an existing branch returned nil, want command-failed")
	}
	if _, ok := AsFailure(err); !ok {
		t.Fatalf("AddBranchWorktree error is not a *Failure: %T %v", err, err)
	}
	infos, err = c.ListWorktrees(ctx, repo)
	if err != nil {
		t.Fatalf("ListWorktrees after failed re-add: %v", err)
	}
	if registeredAt(t, infos, wtPath2) {
		t.Errorf("a second worktree was registered at %q after a failed re-add", wtPath2)
	}
	if got := gitOut(t, r.Invocation, "rev-parse", string(branch)); got != string(c1) {
		t.Errorf("existing branch tip reset to %q, want unchanged %q (no -B/reset)", got, c1)
	}
}

// TestAddBranchWorktreeInvalidRequests proves a non-refs/heads branch, a
// well-formed ref that is not a branch, and a relative path are all rejected as
// invalid-request before any Git process runs.
func TestIntegrationRepoAddBranchWorktreeInvalidRequests(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	commit := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	absPath := filepath.Join(testsupport.TempDir(t), "wt")

	cases := []struct {
		name   string
		path   string
		branch RefName
	}{
		{"unqualified branch", absPath, RefName("feat/new-x")},
		{"non-branch ref", absPath, RefName("refs/tags/v1")},
		{"relative path", "relative/wt", RefName("refs/heads/feat/new-x")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := c.AddBranchWorktree(ctx, repo, tc.path, tc.branch, commit)
			if err == nil {
				t.Fatal("AddBranchWorktree returned nil, want invalid-request")
			}
			f, ok := AsFailure(err)
			if !ok {
				t.Fatalf("error is not a *Failure: %T %v", err, err)
			}
			if f.Kind != KindInvalidRequest {
				t.Errorf("failure kind = %q, want %q", f.Kind, KindInvalidRequest)
			}
		})
	}
}

// TestAttachBranchWorktreeAttachesExistingAndRejectsMissing proves
// AttachBranchWorktree attaches an existing local branch to a new worktree and
// that a missing branch is command-failed with nothing created — never a create.
func TestIntegrationRepoAttachBranchWorktreeAttachesExistingAndRejectsMissing(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	// A local branch that exists but is not checked out anywhere.
	tip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	gitOut(t, r.Invocation, "branch", "feat/attach-me", string(tip))

	branch := RefName("refs/heads/feat/attach-me")
	wtPath := filepath.Join(testsupport.TempDir(t), "attach-me")
	if err := c.AttachBranchWorktree(ctx, repo, wtPath, branch); err != nil {
		t.Fatalf("AttachBranchWorktree: %v", err)
	}
	infos, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	wi := findWorktree(t, infos, wtPath)
	if wi.Branch != branch {
		t.Errorf("attached worktree Branch = %q, want %q", wi.Branch, branch)
	}
	if wi.Head != tip {
		t.Errorf("attached worktree Head = %q, want branch tip %q", wi.Head, tip)
	}
	if got := gitOut(t, wtPath, "symbolic-ref", "HEAD"); got != string(branch) {
		t.Errorf("attached worktree symbolic HEAD = %q, want %q", got, branch)
	}

	// A missing branch is command-failed, nothing created.
	missPath := filepath.Join(testsupport.TempDir(t), "missing")
	err = c.AttachBranchWorktree(ctx, repo, missPath, RefName("refs/heads/feat/does-not-exist"))
	if err == nil {
		t.Fatal("AttachBranchWorktree onto a missing branch returned nil, want command-failed")
	}
	if _, ok := AsFailure(err); !ok {
		t.Fatalf("error is not a *Failure: %T %v", err, err)
	}
	infos, err = c.ListWorktrees(ctx, repo)
	if err != nil {
		t.Fatalf("ListWorktrees after failed attach: %v", err)
	}
	if registeredAt(t, infos, missPath) {
		t.Errorf("a worktree was registered at %q after a failed attach", missPath)
	}
	if _, err := os.Stat(missPath); err == nil {
		t.Errorf("directory %q exists after a failed attach", missPath)
	}
}

// TestRemoveWorktreeCleanPreservesBranchAndRefusesDirty proves the non-forcing
// removal: a clean worktree is deregistered and its directory removed while the
// local branch survives at its tip; a worktree with a dirty tracked file or an
// untracked file is refused (git rechecks cleanliness at the destructive
// boundary) with the file bytes and the registration intact.
func TestIntegrationRepoRemoveWorktreeCleanPreservesBranchAndRefusesDirty(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)

	newBranchWorktree := func(t *testing.T) (*testRepos, Repository, string, RefName, ObjectID) {
		t.Helper()
		r := newMainModeRepos(t)
		repo := mustDiscover(t, c, r.Invocation)
		tip := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		branch := RefName("refs/heads/feat/clean-x")
		wtPath := filepath.Join(testsupport.TempDir(t), "clean-x")
		if err := c.AddBranchWorktree(ctx, repo, wtPath, branch, tip); err != nil {
			t.Fatalf("AddBranchWorktree: %v", err)
		}
		return r, repo, wtPath, branch, tip
	}

	t.Run("clean removal preserves branch", func(t *testing.T) {
		r, repo, wtPath, branch, tip := newBranchWorktree(t)
		if err := c.RemoveWorktreeClean(ctx, repo, wtPath); err != nil {
			t.Fatalf("RemoveWorktreeClean on a clean worktree: %v", err)
		}
		infos, err := c.ListWorktrees(ctx, repo)
		if err != nil {
			t.Fatalf("ListWorktrees: %v", err)
		}
		if registeredAt(t, infos, wtPath) {
			t.Errorf("worktree %q still registered after clean removal", wtPath)
		}
		if _, err := os.Stat(wtPath); err == nil {
			t.Errorf("directory %q still present after clean removal", wtPath)
		}
		if !branchExists(r.Invocation, "feat/clean-x") {
			t.Error("local branch feat/clean-x was deleted by RemoveWorktreeClean")
		}
		if got := gitOut(t, r.Invocation, "rev-parse", string(branch)); got != string(tip) {
			t.Errorf("branch tip changed to %q, want %q", got, tip)
		}
	})

	t.Run("dirty tracked file refused", func(t *testing.T) {
		_, repo, wtPath, _, _ := newBranchWorktree(t)
		writeWorktreeFile(t, wtPath, "README.md", "locally modified\n")
		err := c.RemoveWorktreeClean(ctx, repo, wtPath)
		if err == nil {
			t.Fatal("RemoveWorktreeClean on a dirty tracked worktree returned nil, want command-failed")
		}
		if _, ok := AsFailure(err); !ok {
			t.Fatalf("error is not a *Failure: %T %v", err, err)
		}
		if got, _ := os.ReadFile(filepath.Join(wtPath, "README.md")); string(got) != "locally modified\n" {
			t.Errorf("dirty file bytes changed to %q", got)
		}
		infos, err := c.ListWorktrees(ctx, repo)
		if err != nil {
			t.Fatalf("ListWorktrees: %v", err)
		}
		if !registeredAt(t, infos, wtPath) {
			t.Errorf("worktree %q was deregistered despite a refused removal", wtPath)
		}
	})

	t.Run("untracked file refused", func(t *testing.T) {
		_, repo, wtPath, _, _ := newBranchWorktree(t)
		writeWorktreeFile(t, wtPath, "untracked.md", "untracked bytes\n")
		err := c.RemoveWorktreeClean(ctx, repo, wtPath)
		if err == nil {
			t.Fatal("RemoveWorktreeClean on a worktree with an untracked file returned nil, want command-failed")
		}
		if _, ok := AsFailure(err); !ok {
			t.Fatalf("error is not a *Failure: %T %v", err, err)
		}
		if got, _ := os.ReadFile(filepath.Join(wtPath, "untracked.md")); string(got) != "untracked bytes\n" {
			t.Errorf("untracked file bytes changed to %q", got)
		}
		infos, err := c.ListWorktrees(ctx, repo)
		if err != nil {
			t.Fatalf("ListWorktrees: %v", err)
		}
		if !registeredAt(t, infos, wtPath) {
			t.Errorf("worktree %q was deregistered despite a refused removal", wtPath)
		}
	})
}
