package gitcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// nulPathSet parses a NUL-delimited path list (e.g. diff-tree -z output) into a
// set, dropping the trailing empty element the -z terminator produces.
func nulPathSet(b []byte) map[string]bool {
	set := make(map[string]bool)
	for _, p := range bytes.Split(b, []byte{0}) {
		if len(p) == 0 {
			continue
		}
		set[string(p)] = true
	}
	return set
}

// TestCommitPathsExplicitSetTrailersHooksSigningAndDates proves CommitPaths
// commits exactly the declared path set (a create, a replace, a delete, and a
// hostile-byte replace), never an undeclared dirty file; appends the engine
// trailer block as a real trailer block; is unaffected by an exit-1 pre-commit
// hook (hooksPath override) or by commit.gpgsign=true in the repo (per-command
// signing disabled); and stamps author and committer dates from req.When.
func TestCommitPathsExplicitSetTrailersHooksSigningAndDates(t *testing.T) {
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
	gitOut(t, wt, "config", "core.quotePath", "true")
	// A repo that would sign every commit: our per-command override must defeat it
	// (no signing key is configured, so an attempted sign would fail the commit).
	gitOut(t, wt, "config", "commit.gpgsign", "true")

	// An exit-1 pre-commit hook in the repo's real hooks dir. The override points
	// at an empty dir, so it must never run.
	realHooks := filepath.Join(repo.CommonDir, "hooks")
	if err := os.MkdirAll(realHooks, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realHooks, "pre-commit"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Transaction mutations in the worktree.
	writeWorktreeFile(t, wt, "created.md", "created\n")       // create
	writeWorktreeFile(t, wt, "README.md", "changed readme\n") // replace
	if err := os.Remove(filepath.Join(wt, "docs/changes/active/0001-a.md")); err != nil {
		t.Fatal(err)
	} // delete
	writeWorktreeFile(t, wt, hostilePathTab, "changed hostile\n")   // hostile replace
	writeWorktreeFile(t, wt, "undeclared.md", "dirty undeclared\n") // MUST NOT commit

	declared := []RepoPath{
		"created.md",
		"README.md",
		"docs/changes/active/0001-a.md",
		RepoPath(hostilePathTab),
	}
	when := time.Unix(1600000000, 0).UTC()
	req := CommitRequest{
		Dir:     wt,
		Paths:   declared,
		Subject: "explicit-path commit",
		Trailers: []Trailer{
			{Key: "Docket-Transaction-ID", Value: "abc123def456"},
			{Key: "Docket-Operation", Value: "change.groom"},
		},
		HooksPath: t.TempDir(),
		When:      when,
	}
	got, err := c.CommitPaths(ctx, repo, req)
	if err != nil {
		t.Fatalf("CommitPaths: %v", err)
	}
	if wantHead := ObjectID(gitOut(t, wt, "rev-parse", "HEAD")); got != wantHead {
		t.Errorf("CommitPaths returned %q, worktree HEAD is %q", got, wantHead)
	}

	// Exactly the declared set changed vs the parent — no undeclared file.
	rawDiff := rawGitOut(t, wt, "diff-tree", "--no-renames", "--no-commit-id", "--name-only", "-z", "-r", string(got))
	changed := nulPathSet(rawDiff)
	want := map[string]bool{
		"created.md":                    true,
		"README.md":                     true,
		"docs/changes/active/0001-a.md": true,
		hostilePathTab:                  true,
	}
	if len(changed) != len(want) {
		t.Fatalf("committed path set = %v, want %v", changed, want)
	}
	for p := range want {
		if !changed[p] {
			t.Errorf("committed set missing declared path %q", p)
		}
	}
	if changed["undeclared.md"] {
		t.Error("undeclared dirty file leaked into the commit")
	}

	// Trailer block is a genuine trailer block (git's own parser sees both).
	trailers := gitOut(t, wt, "show", "-s", "--format=%(trailers:only,unfold)", string(got))
	for _, wantLine := range []string{"Docket-Transaction-ID: abc123def456", "Docket-Operation: change.groom"} {
		if !bytes.Contains([]byte(trailers), []byte(wantLine)) {
			t.Errorf("trailer block missing %q; got:\n%s", wantLine, trailers)
		}
	}

	// Author and committer dates both equal req.When.
	wantEpoch := strconv.FormatInt(when.Unix(), 10)
	if at := gitOut(t, wt, "show", "-s", "--format=%at", string(got)); at != wantEpoch {
		t.Errorf("author date epoch = %q, want %q", at, wantEpoch)
	}
	if ct := gitOut(t, wt, "show", "-s", "--format=%ct", string(got)); ct != wantEpoch {
		t.Errorf("committer date epoch = %q, want %q", ct, wantEpoch)
	}
}

// TestCommitPathsMissingIdentityFails proves that when the repository has no
// usable committer identity, CommitPaths surfaces a typed *Failure rather than
// inventing a person or panicking. The repo sets user.useConfigOnly so git will
// not synthesize an identity, and the client's HOME is pinned to an empty dir so
// no global identity leaks in.
func TestCommitPathsMissingIdentityFails(t *testing.T) {
	requireGit(t)
	ctx := context.Background()

	dir := t.TempDir()
	gitOut(t, dir, "init", "-b", "main")
	gitOut(t, dir, "config", "user.useConfigOnly", "true")

	emptyHome := t.TempDir()
	c, err := NewClient(WithBaseEnvironment([]string{"HOME=" + emptyHome, "PATH=" + os.Getenv("PATH")}))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	writeWorktreeFile(t, dir, "f.md", "x\n")
	req := CommitRequest{
		Dir:       dir,
		Paths:     []RepoPath{"f.md"},
		Subject:   "should fail",
		HooksPath: t.TempDir(),
		When:      time.Unix(1600000000, 0).UTC(),
	}
	_, err = c.CommitPaths(ctx, Repository{PrimaryWorktree: dir}, req)
	if err == nil {
		t.Fatal("CommitPaths with no identity returned nil, want a failure")
	}
	if _, ok := AsFailure(err); !ok {
		t.Fatalf("CommitPaths error is not a *Failure: %T %v", err, err)
	}
}
