//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

const ffIdent = "user.email=t@example.com"

// newCheckedOutBranchFixture builds an invocation clone whose branch "meta" sits at
// base and is checked out in a linked worktree wt, plus one later commit on main
// (target) that descends from base, rewrites README.md, and adds target.txt.
func newCheckedOutBranchFixture(t *testing.T) (r *testRepos, wt string, base, target ObjectID) {
	t.Helper()
	r = newMainModeRepos(t)
	base = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	wt = filepath.Join(testsupport.TempDir(t), "meta-wt")
	gitOut(t, r.Invocation, "worktree", "add", "-q", "-b", "meta", wt, string(base))
	writeWorktreeFile(t, r.Invocation, "README.md", "readme v2\n")
	writeWorktreeFile(t, r.Invocation, "target.txt", "target\n")
	gitOut(t, r.Invocation, "add", "--", "README.md", "target.txt")
	gitOut(t, r.Invocation, "-c", "user.name=t", "-c", ffIdent, "commit", "-q", "-m", "target")
	target = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	return r, wt, base, target
}

func metaTip(t *testing.T, r *testRepos) ObjectID {
	t.Helper()
	return ObjectID(gitOut(t, r.Invocation, "rev-parse", "refs/heads/meta"))
}

// installHooks writes the named hooks into a fresh dir; each appends its own name to
// sentinel and exits with code. The dir is returned for core.hooksPath.
func installHooks(t *testing.T, sentinel string, code int, names ...string) string {
	t.Helper()
	dir := filepath.Join(testsupport.TempDir(t), "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		body := "#!/bin/sh\necho " + n + " >> '" + sentinel + "'\nexit " + strconv.Itoa(code) + "\n"
		if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestIntegrationBranchTipFastForwardCheckedOutBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("advances branch, index, and files in place", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		before, err := os.Stat(wt)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch: %v", err)
		}
		if got := metaTip(t, r); got != target {
			t.Fatalf("meta = %s, want %s", got, target)
		}
		if got := ObjectID(gitOut(t, wt, "rev-parse", "HEAD")); got != target {
			t.Fatalf("worktree HEAD = %s, want %s", got, target)
		}
		if s := gitOut(t, wt, "status", "--porcelain", "--untracked-files=all"); s != "" {
			t.Fatalf("worktree not clean after fast-forward:\n%s", s)
		}
		if b, _ := os.ReadFile(filepath.Join(wt, "target.txt")); string(b) != "target\n" {
			t.Fatalf("target.txt = %q, want the target content", b)
		}
		after, err := os.Stat(wt)
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("worktree directory was replaced (err=%v)", err)
		}
		if log := gitOut(t, r.Invocation, "reflog", "show", "--format=%gs", "refs/heads/meta"); !strings.Contains(log, "docket: fast-forward") {
			t.Fatalf("branch reflog lacks the fast-forward entry:\n%s", log)
		}
	})

	t.Run("ignored file at an untouched path survives", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		appendExclude(t, r.Invocation, "*.local")
		writeWorktreeFile(t, wt, "notes.local", "keep me\n")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch: %v", err)
		}
		if b, err := os.ReadFile(filepath.Join(wt, "notes.local")); err != nil || string(b) != "keep me\n" {
			t.Fatalf("ignored file lost: %q, %v", b, err)
		}
	})

	// Git's normal checkout rule: an ignored file at a path the target tracks is
	// expendable, so it is replaced by the target's version (spec §3 property 4).
	t.Run("ignored file at a newly tracked path follows Git's checkout rule", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		appendExclude(t, r.Invocation, "target.txt")
		writeWorktreeFile(t, wt, "target.txt", "mine\n")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch: %v", err)
		}
		if got := metaTip(t, r); got != target {
			t.Fatalf("meta = %s, want %s", got, target)
		}
		if b, _ := os.ReadFile(filepath.Join(wt, "target.txt")); string(b) != "target\n" {
			t.Fatalf("target.txt = %q; want the target's tracked version", b)
		}
	})

	t.Run("untracked file in the way refuses and rolls back", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		writeWorktreeFile(t, wt, "target.txt", "mine\n")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward over an untracked file succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s after refusal, want rolled back to %s", got, base)
		}
		if got := ObjectID(gitOut(t, wt, "rev-parse", "HEAD")); got != base {
			t.Fatalf("worktree HEAD = %s after refusal, want %s", got, base)
		}
		if b, _ := os.ReadFile(filepath.Join(wt, "target.txt")); string(b) != "mine\n" {
			t.Fatalf("untracked file changed: %q", b)
		}
	})

	t.Run("modified tracked file on a changed path refuses and rolls back", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		writeWorktreeFile(t, wt, "README.md", "local edit\n")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward over a modified tracked file succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s after refusal, want rolled back to %s", got, base)
		}
		if b, _ := os.ReadFile(filepath.Join(wt, "README.md")); string(b) != "local edit\n" {
			t.Fatalf("README.md = %q, want the local modification intact", b)
		}
	})

	// An unfinished merge whose staged content does not collide with the target would
	// let read-tree succeed and strand MERGE_HEAD; the operation check refuses first.
	t.Run("unfinished merge refuses and changes nothing", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		gitOut(t, wt, "checkout", "-q", "-b", "side")
		writeWorktreeFile(t, wt, "side.txt", "side\n")
		gitOut(t, wt, "add", "--", "side.txt")
		gitOut(t, wt, "-c", "user.name=t", "-c", ffIdent, "commit", "-q", "-m", "side")
		gitOut(t, wt, "checkout", "-q", "meta")
		gitOut(t, wt, "-c", "user.name=t", "-c", ffIdent, "merge", "-q", "--no-ff", "--no-commit", "side")
		mergeHead := filepath.Join(gitOut(t, wt, "rev-parse", "--absolute-git-dir"), "MERGE_HEAD")
		if _, err := os.Stat(mergeHead); err != nil {
			t.Fatalf("premise: no merge in progress: %v", err)
		}
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward during an unfinished merge succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
		if _, err := os.Stat(mergeHead); err != nil {
			t.Fatalf("MERGE_HEAD gone after refusal: %v", err)
		}
		if b, _ := os.ReadFile(filepath.Join(wt, "side.txt")); string(b) != "side\n" {
			t.Fatalf("merged side.txt = %q, want the in-progress merge content intact", b)
		}
	})

	t.Run("stale stat info on a changed path still fast-forwards", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		future := time.Now().Add(2 * time.Hour)
		if err := os.Chtimes(filepath.Join(wt, "README.md"), future, future); err != nil {
			t.Fatal(err)
		}
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch with stale stat: %v", err)
		}
		if got := metaTip(t, r); got != target {
			t.Fatalf("meta = %s, want %s", got, target)
		}
	})

	t.Run("locked worktree fast-forwards and stays locked", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		gitOut(t, r.Invocation, "worktree", "lock", wt)
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch on a locked worktree: %v", err)
		}
		if list := gitOut(t, r.Invocation, "worktree", "list", "--porcelain"); !strings.Contains(list, "\nlocked") {
			t.Fatalf("lock lost:\n%s", list)
		}
	})

	t.Run("repository hooks never run", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		sentinel := filepath.Join(testsupport.TempDir(t), "hook-ran")
		hooks := installHooks(t, sentinel, 1, "reference-transaction", "post-index-change", "post-checkout", "post-merge")
		gitOut(t, r.Invocation, "config", "core.hooksPath", hooks)
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch with failing repo hooks: %v", err)
		}
		if b, err := os.ReadFile(sentinel); err == nil {
			t.Fatalf("a repository hook ran: %q", b)
		}
	})

	t.Run("stale expected tip refuses and changes nothing", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		writeWorktreeFile(t, wt, "local-only.txt", "local\n")
		gitOut(t, wt, "add", "--", "local-only.txt")
		gitOut(t, wt, "-c", "user.name=t", "-c", ffIdent, "commit", "-q", "-m", "local-only")
		moved := metaTip(t, r)
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward from a stale expected tip succeeded; want refusal")
		}
		if got := metaTip(t, r); got != moved {
			t.Fatalf("meta = %s, want the local commit %s kept", got, moved)
		}
		if _, err := os.Stat(filepath.Join(wt, "local-only.txt")); err != nil {
			t.Fatalf("local commit's file gone: %v", err)
		}
	})

	t.Run("target not descending refuses", func(t *testing.T) {
		r, wt, base, _ := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		tree := gitOut(t, r.Invocation, "rev-parse", "HEAD^{tree}")
		orphan := ObjectID(gitOut(t, r.Invocation, "-c", "user.name=t", "-c", ffIdent, "commit-tree", tree, "-m", "orphan"))
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, orphan); err == nil {
			t.Fatal("fast-forward to a non-descendant succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})

	t.Run("HEAD not on the branch refuses", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		gitOut(t, wt, "checkout", "-q", "--detach")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward with a detached HEAD succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})
}

// appendExclude adds a pattern to the repository's shared info/exclude.
func appendExclude(t *testing.T, repoDir, pattern string) {
	t.Helper()
	p := filepath.Join(repoDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(pattern + "\n"); err != nil {
		t.Fatal(err)
	}
}

// TestIntegrationBranchTipInterruptedFastForward pins the probe that tells an
// interrupted in-place fast-forward (ref swapped, tree not) from every other state.
func TestIntegrationBranchTipInterruptedFastForward(t *testing.T) {
	ctx := context.Background()
	probe := func(t *testing.T, c *Client, wt string) bool {
		t.Helper()
		got, err := c.InterruptedFastForward(ctx, wt, "refs/heads/meta")
		if err != nil {
			t.Fatalf("InterruptedFastForward: %v", err)
		}
		return got
	}

	t.Run("ref swapped but tree not is interrupted", func(t *testing.T) {
		_, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		gitOut(t, wt, "update-ref", "-m", fastForwardReflogMessage, "refs/heads/meta", string(target), string(base))
		if !probe(t, c, wt) {
			t.Fatal("interrupted fast-forward read as not interrupted")
		}
	})

	t.Run("completed fast-forward is not interrupted", func(t *testing.T) {
		_, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch: %v", err)
		}
		if probe(t, c, wt) {
			t.Fatal("completed fast-forward read as interrupted")
		}
		// A later local edit staged on top is ordinary dirt, not an interruption.
		writeWorktreeFile(t, wt, "README.md", "local edit\n")
		gitOut(t, wt, "add", "--", "README.md")
		if probe(t, c, wt) {
			t.Fatal("staged edit after a completed fast-forward read as interrupted")
		}
	})

	t.Run("rolled-back fast-forward is not interrupted", func(t *testing.T) {
		_, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		writeWorktreeFile(t, wt, "target.txt", "mine\n")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward over an untracked file succeeded; want refusal")
		}
		if probe(t, c, wt) {
			t.Fatal("rolled-back fast-forward read as interrupted")
		}
	})

	t.Run("a ref move by any other writer is not interrupted", func(t *testing.T) {
		_, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		gitOut(t, wt, "update-ref", "-m", "someone else", "refs/heads/meta", string(target), string(base))
		if probe(t, c, wt) {
			t.Fatal("a foreign ref move read as an interrupted docket fast-forward")
		}
	})
}
