package gitcli

import (
	"context"
	"path/filepath"
	"testing"
)

// TestChangedPathsExactSetWithStagedFlags exercises ChangedPaths against a
// detached transaction worktree carrying the full spread of index/working-tree
// deltas — staged and unstaged modifications, a staged deletion, an untracked
// file, a hostile non-ASCII/space/tab path, and a `git mv` — and asserts the
// exact set of paths with their Staged flags, byte-exact hostile names, and that
// a move reports BOTH source and destination (rename detection off).
func TestChangedPathsExactSetWithStagedFlags(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	commit := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	wt := filepath.Join(t.TempDir(), "txn")
	if err := c.AddDetachedWorktree(ctx, repo, wt, commit); err != nil {
		t.Fatalf("AddDetachedWorktree: %v", err)
	}
	// The transaction worktree inherits the hostile-path-preserving config.
	gitOut(t, wt, "config", "core.quotePath", "true")

	// 1. Unstaged modification of a tracked file.
	writeWorktreeFile(t, wt, "README.md", "readme changed\n")
	// 2. Untracked new file.
	writeWorktreeFile(t, wt, "brand-new.md", "brand new\n")
	// 3. Staged modification.
	writeWorktreeFile(t, wt, ".docket.yml", "version: 2\n")
	gitOut(t, wt, "add", ".docket.yml")
	// 4. Staged deletion.
	gitOut(t, wt, "rm", "-q", "--", "docs/changes/active/0001-a.md")
	// 5. Staged modification of a hostile-byte path.
	writeWorktreeFile(t, wt, hostilePathTab, "hostile changed\n")
	gitOut(t, wt, "add", "--", hostilePathTab)
	// 6. A move: staged, reported as delete(source)+add(dest) under --no-renames.
	gitOut(t, wt, "mv", "--", "tool.sh", "tool2.sh")

	changes, err := c.ChangedPaths(ctx, wt)
	if err != nil {
		t.Fatalf("ChangedPaths: %v", err)
	}

	got := make(map[RepoPath]bool, len(changes))
	for _, ch := range changes {
		if _, dup := got[ch.Path]; dup {
			t.Fatalf("ChangedPaths returned duplicate path %q", ch.Path)
		}
		got[ch.Path] = ch.Staged
	}

	want := map[RepoPath]bool{
		"README.md":                     false, // unstaged modification
		"brand-new.md":                  false, // untracked
		".docket.yml":                   true,  // staged modification
		"docs/changes/active/0001-a.md": true,  // staged deletion
		RepoPath(hostilePathTab):        true,  // staged, byte-exact hostile path
		"tool.sh":                       true,  // move source
		"tool2.sh":                      true,  // move destination
	}

	if len(got) != len(want) {
		t.Fatalf("ChangedPaths set size = %d, want %d\ngot:  %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for p, wantStaged := range want {
		gotStaged, ok := got[p]
		if !ok {
			t.Errorf("ChangedPaths missing path %q", p)
			continue
		}
		if gotStaged != wantStaged {
			t.Errorf("ChangedPaths[%q].Staged = %v, want %v", p, gotStaged, wantStaged)
		}
	}
}

// TestChangedPathsCleanWorktreeIsEmpty proves a worktree with no delta from HEAD
// yields an empty set — the baseline the transaction engine relies on to detect
// "the plan changed nothing".
func TestChangedPathsCleanWorktreeIsEmpty(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	commit := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	wt := filepath.Join(t.TempDir(), "clean")
	if err := c.AddDetachedWorktree(ctx, repo, wt, commit); err != nil {
		t.Fatalf("AddDetachedWorktree: %v", err)
	}

	changes, err := c.ChangedPaths(ctx, wt)
	if err != nil {
		t.Fatalf("ChangedPaths: %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("ChangedPaths on a clean worktree = %+v, want empty", changes)
	}
}
