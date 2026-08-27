//go:build integration

package gitcli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This file builds real temporary Git repositories for the gitcli tests. Two
// topologies are produced — a plain "main mode" repo and a docket-style repo
// with an orphan "docket" branch plus linked worktrees — each backed by a bare
// file remote and an independent writer clone that advances the remote. All
// paths live under t.TempDir(); builders return the raw t.TempDir() spelling
// (never filepath.EvalSymlinks-canonicalized) so tests exercise the symlinked
// /tmp -> /private/tmp case on macOS. Everything here is _test.go only and must
// never be referenced by product code.

// Hostile fixture path names (bytes that quoted/line-split parsers mishandle):
// a space, a non-ASCII rune, and a literal tab; an embedded newline.
const (
	hostilePathTab     = "spa ce/né tab\tfile.md"
	hostilePathNewline = "line\nbreak.md"
	// Pathspec-magic fixture paths: real files whose names begin with Git
	// pathspec-magic bytes. Under a NON-literal pathspec, "-- ':weird.md'"
	// matches nothing (the real file would read Found:false) and "-- ':'" expands
	// to the whole tree (a prefix escapes its requested scope); the ':(top)…'
	// long-form magic behaves the same way. GIT_LITERAL_PATHSPECS=1 forces every
	// caller path to its literal meaning, so these resolve to exactly their own
	// entries.
	pathspecMagicColon    = ":weird.md"
	pathspecMagicColonTop = ":(top)x.md"
)

// testRepos is a bare origin plus two clones: a writer that pushes to advance
// the origin, and the invocation clone under test whose "origin" remote points
// at Origin.
type testRepos struct {
	Origin     string // bare repo path (file remote)
	Writer     string // writer clone: pushes advance origin
	Invocation string // clone under test; remote "origin" -> Origin
}

// gitOut runs real git directly (independent of the adapter under test) with
// -C <dir>, returns trimmed stdout, and fails the test on a non-zero exit. It
// is the plumbing oracle every fixture assertion and later test compares
// against.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitTry(dir, args...)
	if err != nil {
		t.Fatalf("git -C %s %s: %v", dir, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out)
}

// gitTry runs git -C <dir> and returns raw stdout plus an error carrying the
// captured stderr; it never touches the testing.T so callers can probe for an
// expected failure (e.g. branch existence).
func gitTry(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), &gitError{err: err, stderr: stderr.String()}
	}
	return stdout.String(), nil
}

type gitError struct {
	err    error
	stderr string
}

func (e *gitError) Error() string {
	return e.err.Error() + ": " + strings.TrimSpace(e.stderr)
}

// configRepoIdentity pins a deterministic committer identity and disables
// gpg signing so a developer's global config cannot perturb the fixtures.
func configRepoIdentity(t *testing.T, dir string) {
	t.Helper()
	gitOut(t, dir, "config", "user.name", "t")
	gitOut(t, dir, "config", "user.email", "t@t")
	gitOut(t, dir, "config", "commit.gpgsign", "false")
}

// writeWorktreeFile writes content (creating parent directories) at a
// repo-relative path, tolerating hostile bytes in the name (space, non-ASCII,
// tab, embedded newline).
func writeWorktreeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// branchExists reports whether refs/heads/<branch> exists in the repo at dir.
func branchExists(dir, branch string) bool {
	_, err := gitTry(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// newMainModeRepos builds a bare origin whose branch main holds README.md,
// .docket.yml, docs/changes/active/0001-a.md, the two hostile paths, the two
// pathspec-magic paths (pathspecMagicColon, pathspecMagicColonTop), a symlink
// link.md -> README.md, an empty file empty.txt, an executable tool.sh (mode
// 100755), and a gitlink "sub" (mode 160000) added via update-index cacheinfo.
// A writer clone advances origin; the invocation clone under test is checked
// out on main with core.quotePath=true so a developer's global "false" cannot
// disarm the hostile-path proof.
func newMainModeRepos(t *testing.T) *testRepos {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	r := &testRepos{
		Origin:     filepath.Join(root, "origin.git"),
		Writer:     filepath.Join(root, "writer"),
		Invocation: filepath.Join(root, "invocation"),
	}

	gitOut(t, root, "init", "--bare", "-b", "main", r.Origin)

	gitOut(t, root, "init", "-b", "main", r.Writer)
	configRepoIdentity(t, r.Writer)
	gitOut(t, r.Writer, "config", "core.quotePath", "true")

	writeWorktreeFile(t, r.Writer, "README.md", "readme\n")
	writeWorktreeFile(t, r.Writer, ".docket.yml", "version: 1\n")
	writeWorktreeFile(t, r.Writer, "docs/changes/active/0001-a.md", "change a\n")
	writeWorktreeFile(t, r.Writer, hostilePathTab, "hostile tab/space/non-ascii\n")
	writeWorktreeFile(t, r.Writer, hostilePathNewline, "hostile newline\n")
	writeWorktreeFile(t, r.Writer, pathspecMagicColon, "colon-leading pathspec-magic file\n")
	writeWorktreeFile(t, r.Writer, pathspecMagicColonTop, "top-magic pathspec file\n")
	writeWorktreeFile(t, r.Writer, "empty.txt", "")
	writeWorktreeFile(t, r.Writer, "tool.sh", "#!/bin/sh\necho hi\n")
	if err := os.Chmod(filepath.Join(r.Writer, "tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("README.md", filepath.Join(r.Writer, "link.md")); err != nil {
		t.Fatal(err)
	}
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "main content")

	// A real commit id to point the synthetic gitlink at (needs no submodule
	// machinery; the object exists so ls-tree reports a well-formed entry).
	base := gitOut(t, r.Writer, "rev-parse", "HEAD")
	gitOut(t, r.Writer, "update-index", "--add", "--cacheinfo", "160000,"+base+",sub")
	gitOut(t, r.Writer, "commit", "-q", "-m", "add gitlink")

	gitOut(t, r.Writer, "remote", "add", "origin", r.Origin)
	gitOut(t, r.Writer, "push", "-q", "-u", "origin", "main")

	gitOut(t, root, "clone", "-q", r.Origin, r.Invocation)
	gitOut(t, r.Invocation, "config", "core.quotePath", "true")
	configRepoIdentity(t, r.Invocation)
	return r
}

// newDocketModeRepos builds a bare origin with branch main (.docket.yml, code,
// and a .gitignore excluding .docket/ and .worktrees/) plus an orphan "docket"
// branch holding docs/changes/active planning files. The invocation clone adds
// two linked worktrees: ".docket" parked on docket and ".worktrees/feat-x" on a
// feature branch — three registered worktrees in total.
func newDocketModeRepos(t *testing.T) *testRepos {
	t.Helper()
	requireGit(t)
	root := t.TempDir()
	r := &testRepos{
		Origin:     filepath.Join(root, "origin.git"),
		Writer:     filepath.Join(root, "writer"),
		Invocation: filepath.Join(root, "invocation"),
	}

	gitOut(t, root, "init", "--bare", "-b", "main", r.Origin)

	gitOut(t, root, "init", "-b", "main", r.Writer)
	configRepoIdentity(t, r.Writer)

	writeWorktreeFile(t, r.Writer, ".docket.yml", "version: 1\n")
	writeWorktreeFile(t, r.Writer, "main.go", "package main\n")
	// Real docket repos ignore these so the nested linked worktrees do not show
	// as untracked in the primary worktree.
	writeWorktreeFile(t, r.Writer, ".gitignore", ".docket/\n.worktrees/\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "main content")
	gitOut(t, r.Writer, "remote", "add", "origin", r.Origin)
	gitOut(t, r.Writer, "push", "-q", "-u", "origin", "main")

	// Orphan docket branch: unrelated history, planning files only.
	gitOut(t, r.Writer, "checkout", "-q", "--orphan", "docket")
	gitOut(t, r.Writer, "rm", "-rfq", "--cached", ".")
	for _, name := range []string{".docket.yml", "main.go", ".gitignore"} {
		if err := os.Remove(filepath.Join(r.Writer, name)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	writeWorktreeFile(t, r.Writer, "docs/changes/active/0001-plan.md", "plan\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "docket planning")
	gitOut(t, r.Writer, "push", "-q", "-u", "origin", "docket")
	gitOut(t, r.Writer, "checkout", "-q", "main")

	gitOut(t, root, "clone", "-q", r.Origin, r.Invocation)
	configRepoIdentity(t, r.Invocation)
	gitOut(t, r.Invocation, "worktree", "add", "-q", "-B", "docket", ".docket", "origin/docket")
	gitOut(t, r.Invocation, "worktree", "add", "-q", "-b", "feat-x", ".worktrees/feat-x", "main")
	return r
}

// writerCommit writes files on branch in the writer clone (creating the branch
// from the current HEAD when it does not yet exist), commits, pushes to origin,
// and returns the new commit id read from the writer's own rev-parse — an
// oracle independent of the adapter under test.
func (r *testRepos) writerCommit(t *testing.T, branch string, files map[string]string) ObjectID {
	t.Helper()
	if branchExists(r.Writer, branch) {
		gitOut(t, r.Writer, "checkout", "-q", branch)
	} else {
		gitOut(t, r.Writer, "checkout", "-q", "-b", branch)
	}
	paths := make([]string, 0, len(files))
	for rel, content := range files {
		writeWorktreeFile(t, r.Writer, rel, content)
		paths = append(paths, rel)
	}
	gitOut(t, r.Writer, append([]string{"add", "--"}, paths...)...)
	gitOut(t, r.Writer, "commit", "-q", "-m", "writerCommit on "+branch)
	gitOut(t, r.Writer, "push", "-q", "origin", branch)
	return ObjectID(gitOut(t, r.Writer, "rev-parse", "HEAD"))
}

// TestHarnessBuildersProduceExpectedTopology is the harness self-test: it proves
// each builder produces the topology later tasks depend on — a bare origin, a
// clean invocation checkout, the raw hostile bytes on main, and (docket mode)
// three registered worktrees.
func TestIntegrationRepoHarnessBuildersProduceExpectedTopology(t *testing.T) {
	requireGit(t)

	t.Run("main", func(t *testing.T) {
		r := newMainModeRepos(t)

		if got := gitOut(t, r.Origin, "rev-parse", "--is-bare-repository"); got != "true" {
			t.Errorf("origin is-bare-repository = %q, want true", got)
		}
		if got := gitOut(t, r.Invocation, "status", "--porcelain"); got != "" {
			t.Errorf("invocation status not clean:\n%s", got)
		}

		// Raw NUL-delimited names carry the hostile bytes verbatim.
		names := gitOut(t, r.Writer, "ls-tree", "-r", "-z", "--name-only", "main")
		for _, want := range []string{hostilePathTab, hostilePathNewline} {
			if !strings.Contains(names, want) {
				t.Errorf("main tree missing raw hostile path %q", want)
			}
		}

		// The distinctive fixture entries: a mode-120000 symlink, a mode-100755
		// executable, and a mode-160000 gitlink.
		full := gitOut(t, r.Writer, "ls-tree", "-r", "main")
		for _, want := range []string{"120000 blob", "100755 blob", "160000 commit"} {
			if !strings.Contains(full, want) {
				t.Errorf("main tree missing entry shape %q in:\n%s", want, full)
			}
		}
	})

	t.Run("docket", func(t *testing.T) {
		r := newDocketModeRepos(t)

		if got := gitOut(t, r.Origin, "rev-parse", "--is-bare-repository"); got != "true" {
			t.Errorf("origin is-bare-repository = %q, want true", got)
		}
		if got := gitOut(t, r.Invocation, "status", "--porcelain"); got != "" {
			t.Errorf("docket-mode invocation primary status not clean:\n%s", got)
		}

		wl := gitOut(t, r.Invocation, "worktree", "list", "--porcelain")
		if n := countWorktrees(wl); n != 3 {
			t.Errorf("registered worktrees = %d, want 3 (primary + .docket + feature):\n%s", n, wl)
		}
		for _, want := range []string{"refs/heads/docket", "refs/heads/feat-x"} {
			if !strings.Contains(wl, want) {
				t.Errorf("worktree list missing %q:\n%s", want, wl)
			}
		}

		mainNames := gitOut(t, r.Invocation, "ls-tree", "-r", "--name-only", "main")
		if !strings.Contains(mainNames, ".docket.yml") {
			t.Errorf("main tree missing .docket.yml:\n%s", mainNames)
		}
		docketNames := gitOut(t, r.Invocation, "ls-tree", "-r", "--name-only", "origin/docket")
		if !strings.Contains(docketNames, "docs/changes/active/") {
			t.Errorf("docket branch missing planning files:\n%s", docketNames)
		}
		// The two branches carry genuinely different trees (orphan history).
		if !strings.Contains(docketNames, "docs/changes/active/0001-plan.md") {
			t.Errorf("docket branch missing 0001-plan.md:\n%s", docketNames)
		}
		if strings.Contains(mainNames, "0001-plan.md") {
			t.Errorf("main tree unexpectedly contains docket-only planning file:\n%s", mainNames)
		}
	})
}

// countWorktrees counts the "worktree <path>" stanza headers in the porcelain
// worktree-list output.
func countWorktrees(porcelain string) int {
	n := 0
	for _, line := range strings.Split(porcelain, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			n++
		}
	}
	return n
}

// mustDiscover resolves repository identity from an invocation path or fails the
// test; refs tests operate against the discovered primary worktree.
func mustDiscover(t *testing.T, c *Client, path string) Repository {
	t.Helper()
	repo, err := c.Discover(context.Background(), DiscoverOptions{InvocationPath: path})
	if err != nil {
		t.Fatalf("Discover(%q): %v", path, err)
	}
	return repo
}
