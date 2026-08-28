//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// setupTreeRepo initializes a fresh non-bare repository with a deterministic
// committer identity under t.TempDir() and returns its path plus the Repository
// the setup-tree primitives run against. No commits are made; callers that need
// history add it themselves.
func setupTreeRepo(t *testing.T) (string, Repository) {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	gitOut(t, dir, "init", "-b", "main")
	configRepoIdentity(t, dir)
	return dir, Repository{PrimaryWorktree: dir}
}

// TestIntegrationSetupTreeEmptyTreeOID proves EmptyTreeOID reports exactly the
// object id git computes for the empty tree, hash-algorithm-agnostically (the
// fixture's own `git hash-object -t tree /dev/null` is the oracle).
func TestIntegrationSetupTreeEmptyTreeOID(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := setupTreeRepo(t)

	got, err := c.EmptyTreeOID(ctx, repo)
	if err != nil {
		t.Fatalf("EmptyTreeOID: %v", err)
	}
	want := ObjectID(gitOut(t, dir, "hash-object", "-t", "tree", "/dev/null"))
	if got != want {
		t.Errorf("EmptyTreeOID = %q, want %q", got, want)
	}
}

// TestIntegrationSetupTreeCommitTreeParentless proves CommitTree creates a
// parentless root commit over the empty tree, carrying its subject and trailer
// block, without firing any repository hook.
func TestIntegrationSetupTreeCommitTreeParentless(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := setupTreeRepo(t)

	// A pre-commit hook that, if it ever ran, would leave a marker file behind.
	hooks := filepath.Join(dir, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "hook-ran")
	hookBody := "#!/bin/sh\ntouch " + marker + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte(hookBody), 0o755); err != nil {
		t.Fatal(err)
	}

	empty, err := c.EmptyTreeOID(ctx, repo)
	if err != nil {
		t.Fatalf("EmptyTreeOID: %v", err)
	}
	trailers := []Trailer{{Key: "Docket-Operation", Value: "repository-init-root/v1"}}
	root, err := c.CommitTree(ctx, repo, empty, nil, "docket: initialize metadata branch", trailers)
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}

	// It is a real commit object.
	if typ := gitOut(t, dir, "cat-file", "-t", string(root)); typ != "commit" {
		t.Errorf("root object type = %q, want commit", typ)
	}
	// The parentless-root set reachable from it is exactly itself.
	if roots := gitOut(t, dir, "rev-list", "--max-parents=0", string(root)); roots != string(root) {
		t.Errorf("rev-list --max-parents=0 = %q, want %q", roots, root)
	}
	// Its tree is the empty tree.
	if tree := gitOut(t, dir, "rev-parse", "--verify", string(root)+"^{tree}"); ObjectID(tree) != empty {
		t.Errorf("root tree = %q, want empty tree %q", tree, empty)
	}
	// The trailer is a genuine trailer block.
	got := gitOut(t, dir, "show", "-s", "--format=%(trailers:only,unfold)", string(root))
	if !strings.Contains(got, "Docket-Operation: repository-init-root/v1") {
		t.Errorf("trailer block missing operation trailer; got:\n%s", got)
	}
	// The subject is exactly what we asked for.
	if subj := gitOut(t, dir, "show", "-s", "--format=%s", string(root)); subj != "docket: initialize metadata branch" {
		t.Errorf("subject = %q, want %q", subj, "docket: initialize metadata branch")
	}
	// No hook fired.
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("pre-commit hook marker present (err=%v): a hook fired during commit-tree", err)
	}
}

// TestIntegrationSetupTreeBuildTreeIncludeRemovePut proves BuildTree composes a
// tree from an empty index by including two source prefixes, removing one path,
// and putting one new blob, producing exactly the expected leaf listing.
func TestIntegrationSetupTreeBuildTreeIncludeRemovePut(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := setupTreeRepo(t)

	writeWorktreeFile(t, dir, "docs/changes/active/0001-a.md", "active a\n")
	writeWorktreeFile(t, dir, "docs/changes/archive/0002-b.md", "archive b\n")
	writeWorktreeFile(t, dir, "docs/adrs/0001.md", "adr one\n")
	writeWorktreeFile(t, dir, "unrelated.go", "package x\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "seed")
	head := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

	ops := []TreeOp{
		{IncludePrefix: &IncludePrefixOp{From: head, Prefix: "docs/changes"}},
		{IncludePrefix: &IncludePrefixOp{From: head, Prefix: "docs/adrs"}},
		{RemovePath: &RemovePathOp{Path: "docs/changes/archive/0002-b.md"}},
		{PutBlob: &PutBlobOp{Path: "docs/newfile.md", Content: []byte("brand new\n"), Mode: "100644"}},
	}
	tree, err := c.BuildTree(ctx, repo, "", ops)
	if err != nil {
		t.Fatalf("BuildTree: %v", err)
	}

	got := gitOut(t, dir, "ls-tree", "-r", "--name-only", string(tree))
	gotLeaves := strings.Split(strings.TrimSpace(got), "\n")
	sort.Strings(gotLeaves)
	want := []string{"docs/adrs/0001.md", "docs/changes/active/0001-a.md", "docs/newfile.md"}
	if strings.Join(gotLeaves, "\n") != strings.Join(want, "\n") {
		t.Errorf("BuildTree leaves =\n%s\nwant\n%s", strings.Join(gotLeaves, "\n"), strings.Join(want, "\n"))
	}
	// The put blob carries exactly the requested bytes.
	if content := gitOut(t, dir, "cat-file", "-p", string(tree)+":docs/newfile.md"); content != "brand new" {
		t.Errorf("put blob content = %q, want %q", content, "brand new")
	}
}

// TestIntegrationSetupTreeBuildTreeAbsentPrefixErrors proves that including a
// source prefix that does not exist in the source tree is a hard error, never a
// silent skip.
func TestIntegrationSetupTreeBuildTreeAbsentPrefixErrors(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := setupTreeRepo(t)

	writeWorktreeFile(t, dir, "docs/adrs/0001.md", "adr one\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "seed")
	head := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

	ops := []TreeOp{
		{IncludePrefix: &IncludePrefixOp{From: head, Prefix: "docs/does-not-exist"}},
	}
	if _, err := c.BuildTree(ctx, repo, "", ops); err == nil {
		t.Fatal("BuildTree with an absent source prefix returned nil, want an error")
	}
}

// TestIntegrationSetupTreeRootCommitsMultipleRoots proves RootCommits enumerates
// every parentless root reachable from a tip: a merge of two unrelated histories
// reports both roots.
func TestIntegrationSetupTreeRootCommitsMultipleRoots(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := setupTreeRepo(t)

	writeWorktreeFile(t, dir, "a.txt", "a\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "root1")
	root1 := gitOut(t, dir, "rev-parse", "HEAD")

	gitOut(t, dir, "checkout", "-q", "--orphan", "other")
	gitOut(t, dir, "rm", "-rfq", "--cached", ".")
	if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	writeWorktreeFile(t, dir, "b.txt", "b\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "root2")
	root2 := gitOut(t, dir, "rev-parse", "HEAD")

	gitOut(t, dir, "checkout", "-q", "main")
	gitOut(t, dir, "merge", "-q", "--allow-unrelated-histories", "--no-edit", "other")
	tip := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

	roots, err := c.RootCommits(ctx, repo, tip)
	if err != nil {
		t.Fatalf("RootCommits: %v", err)
	}
	if len(roots) != 2 {
		t.Fatalf("RootCommits returned %d roots, want 2: %v", len(roots), roots)
	}
	set := map[ObjectID]bool{roots[0]: true, roots[1]: true}
	if !set[ObjectID(root1)] || !set[ObjectID(root2)] {
		t.Errorf("RootCommits = %v, want the set {%s, %s}", roots, root1, root2)
	}
}

// TestIntegrationSetupTreeDisableWorktreeHooks proves DisableWorktreeHooks turns
// on extensions.worktreeConfig and points the worktree's core.hooksPath at an
// existing, empty, absolute directory — idempotently across repeated calls.
func TestIntegrationSetupTreeDisableWorktreeHooks(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, _ := setupTreeRepo(t)

	writeWorktreeFile(t, dir, "f.txt", "x\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "init")

	wt := filepath.Join(t.TempDir(), "wt")
	gitOut(t, dir, "worktree", "add", "-q", "-b", "feat", wt, "main")

	if err := c.DisableWorktreeHooks(ctx, wt); err != nil {
		t.Fatalf("DisableWorktreeHooks: %v", err)
	}

	if got := gitOut(t, wt, "config", "--local", "--get", "extensions.worktreeConfig"); got != "true" {
		t.Errorf("extensions.worktreeConfig = %q, want true", got)
	}
	hp := gitOut(t, wt, "config", "--worktree", "core.hooksPath")
	if !filepath.IsAbs(hp) {
		t.Errorf("core.hooksPath = %q, want an absolute path", hp)
	}
	info, err := os.Stat(hp)
	if err != nil || !info.IsDir() {
		t.Fatalf("core.hooksPath %q is not an existing dir (err=%v)", hp, err)
	}
	entries, err := os.ReadDir(hp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("core.hooksPath dir %q is not empty: %v", hp, entries)
	}

	// Idempotent: a second call succeeds and leaves the same value in place.
	if err := c.DisableWorktreeHooks(ctx, wt); err != nil {
		t.Fatalf("DisableWorktreeHooks (second call): %v", err)
	}
	if got := gitOut(t, wt, "config", "--worktree", "core.hooksPath"); got != hp {
		t.Errorf("core.hooksPath after second call = %q, want unchanged %q", got, hp)
	}
}
