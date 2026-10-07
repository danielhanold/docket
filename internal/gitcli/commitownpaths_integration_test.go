//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// newOwnPathsRepo builds a non-bare repository on branch main holding tracked
// a.txt, gone.txt, and other.txt in one commit, with a pinned identity.
func newOwnPathsRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := filepath.Join(testsupport.TempDir(t), "repo")
	gitOut(t, filepath.Dir(dir), "init", "-q", "-b", "main", dir)
	configRepoIdentity(t, dir)
	writeWorktreeFile(t, dir, "a.txt", "a\n")
	writeWorktreeFile(t, dir, "gone.txt", "gone\n")
	writeWorktreeFile(t, dir, "other.txt", "other\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "base")
	return dir
}

// TestIntegrationRepoCommitOwnPathsLeavesOtherStagedChanges proves CommitOwnPaths
// commits exactly the named paths (a modify, an untracked create, and a delete)
// with the bare subject as the whole message, while a change the user staged
// beforehand stays staged and out of the commit.
func TestIntegrationRepoCommitOwnPathsLeavesOtherStagedChanges(t *testing.T) {
	c := newRealClient(t)
	ctx := context.Background()
	dir := newOwnPathsRepo(t)

	writeWorktreeFile(t, dir, "other.txt", "user staged edit\n")
	gitOut(t, dir, "add", "other.txt")

	writeWorktreeFile(t, dir, "a.txt", "a changed\n")
	writeWorktreeFile(t, dir, "new.yml", "k: v\n")
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}

	subject := "Add docket configurations to repository"
	got, err := c.CommitOwnPaths(ctx, dir, []RepoPath{"a.txt", "new.yml", "gone.txt"}, subject)
	if err != nil {
		t.Fatalf("CommitOwnPaths: %v", err)
	}
	if head := ObjectID(gitOut(t, dir, "rev-parse", "HEAD")); got != head {
		t.Fatalf("CommitOwnPaths returned %q, HEAD is %q", got, head)
	}

	msg, err := gitTry(dir, "show", "-s", "--format=%B", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if want := subject + "\n\n"; msg != want {
		t.Fatalf("commit message = %q, want %q", msg, want)
	}
	nameStatus := gitOut(t, dir, "diff-tree", "--no-commit-id", "--name-status", "-r", "HEAD")
	lines := strings.Split(nameStatus, "\n")
	sort.Strings(lines)
	wantLines := []string{"D\tgone.txt", "M\ta.txt", "A\tnew.yml"}
	sort.Strings(wantLines)
	if strings.Join(lines, "|") != strings.Join(wantLines, "|") {
		t.Fatalf("committed name-status = %q, want %q", lines, wantLines)
	}

	if staged := gitOut(t, dir, "diff", "--cached", "--name-only"); staged != "other.txt" {
		t.Fatalf("staged after commit = %q, want only other.txt", staged)
	}
}

// TestIntegrationRepoCommitOwnPathsRunsRepositoryHooks proves the repository's
// own hooks run: a failing commit-msg hook fires, fails the commit as
// command-failed, and leaves HEAD and the working-tree edits untouched.
func TestIntegrationRepoCommitOwnPathsRunsRepositoryHooks(t *testing.T) {
	c := newRealClient(t)
	ctx := context.Background()
	dir := newOwnPathsRepo(t)
	before := gitOut(t, dir, "rev-parse", "HEAD")

	marker := filepath.Join(testsupport.TempDir(t), "hook-ran")
	hook := "#!/bin/sh\ntouch '" + marker + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "commit-msg"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	writeWorktreeFile(t, dir, "a.txt", "a changed\n")

	_, err := c.CommitOwnPaths(ctx, dir, []RepoPath{"a.txt"}, "Remove docket configurations from repository")
	assertKind(t, err, KindCommandFailed)
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatalf("commit-msg hook did not run: %v", statErr)
	}
	if after := gitOut(t, dir, "rev-parse", "HEAD"); after != before {
		t.Fatalf("HEAD moved from %s to %s after a failing hook", before, after)
	}
	data, rerr := os.ReadFile(filepath.Join(dir, "a.txt"))
	if rerr != nil || string(data) != "a changed\n" {
		t.Fatalf("a.txt = %q (%v), want the edit kept", data, rerr)
	}
}

// TestIntegrationRepoCommitOwnPathsRefusesDetachedHead proves a detached HEAD is
// an invalid request and commits nothing.
func TestIntegrationRepoCommitOwnPathsRefusesDetachedHead(t *testing.T) {
	c := newRealClient(t)
	ctx := context.Background()
	dir := newOwnPathsRepo(t)
	gitOut(t, dir, "checkout", "-q", "--detach")
	before := gitOut(t, dir, "rev-parse", "HEAD")
	writeWorktreeFile(t, dir, "a.txt", "a changed\n")

	_, err := c.CommitOwnPaths(ctx, dir, []RepoPath{"a.txt"}, "Add docket configurations to repository")
	assertKind(t, err, KindInvalidRequest)
	if after := gitOut(t, dir, "rev-parse", "HEAD"); after != before {
		t.Fatalf("HEAD moved on a detached refusal")
	}
}

// TestIntegrationRepoCommitOwnPathsValidatesRequest proves a relative dir, an
// empty or multi-line subject, an empty path list, and an escaping path are
// invalid requests.
func TestIntegrationRepoCommitOwnPathsValidatesRequest(t *testing.T) {
	c := newRealClient(t)
	ctx := context.Background()
	dir := newOwnPathsRepo(t)
	cases := []struct {
		name    string
		dir     string
		paths   []RepoPath
		subject string
	}{
		{"relative dir", "repo", []RepoPath{"a.txt"}, "s"},
		{"empty subject", dir, []RepoPath{"a.txt"}, ""},
		{"multi-line subject", dir, []RepoPath{"a.txt"}, "s\nbody"},
		{"no paths", dir, nil, "s"},
		{"escaping path", dir, []RepoPath{"../x"}, "s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := c.CommitOwnPaths(ctx, tc.dir, tc.paths, tc.subject)
			assertKind(t, err, KindInvalidRequest)
		})
	}
}
