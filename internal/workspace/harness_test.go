package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// This file builds real temporary Git repositories for the workspace Prepare
// tests. Two topologies are produced — a plain "main mode" repo and a
// docket-style repo carrying an orphan "docket" branch plus a registered
// `.docket/` worktree, one detached transaction-style worktree, and one sibling
// feature worktree — each backed by a bare file remote plus an independent
// writer clone that advances the remote. All paths live under testsupport.TempDir(t); the
// builders return the raw testsupport.TempDir(t) spelling (never filepath.EvalSymlinks-
// canonicalized) so the tests exercise the macOS /tmp -> /private/tmp symlinked
// case Prepare must canonicalize through. Everything here is _test.go only and
// is never referenced by product code.
//
// A workspace's checkout lands at <primary>/.worktrees/<slug>; both fixtures
// gitignore `.worktrees/` and `.docket/` so a newly attached workspace stays
// invisible to the primary worktree's own git status, which is exactly what the
// preservation proofs assert.

// wsRepos is a bare origin, a writer clone that pushes to advance the origin,
// and the primary clone under test. Preserve lists every worktree whose bytes a
// Prepare must leave untouched (the primary itself plus, in docket mode, the
// .docket/transaction/sibling worktrees).
type wsRepos struct {
	Origin   string
	Writer   string
	Primary  string
	Preserve []string
}

// requireGit skips when no real git is on PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

// useBackgroundOffGit points the git children these tests spawn at a per-fixture
// GIT_CONFIG_GLOBAL (testsupport.GitEnv) that disables auto-gc, auto-maintenance,
// and fsmonitor. The direct oracle helpers (gitOut/gitOutRaw/gitTry) inherit the
// test-process environment, so this reaches them; without it a detached git
// housekeeping child spawned by a fixture commit can outlive the test and keep
// writing into a testsupport.TempDir, racing RemoveAll teardown to "directory
// not empty" under parallel load (change 0373). Git spawned through the product
// gitcli client scrubs GIT_CONFIG, so its housekeeping children are instead
// absorbed by the fixture's drain-then-retry removal. Set process-wide via
// t.Setenv because gitTry takes no *testing.T; safe because this package runs no
// test in parallel. Call it from every repo builder before the first git spawn.
func useBackgroundOffGit(t *testing.T) {
	t.Helper()
	for _, kv := range testsupport.GitEnv(t) {
		if v, ok := strings.CutPrefix(kv, "GIT_CONFIG_GLOBAL="); ok {
			t.Setenv("GIT_CONFIG_GLOBAL", v)
		}
	}
}

// gitOut runs real git directly (independent of the adapter under test) with
// -C <dir>, returns trimmed stdout, and fails the test on a non-zero exit. It is
// the plumbing oracle the fixtures and assertions compare against.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitTry(dir, args...)
	if err != nil {
		t.Fatalf("git -C %s %s: %v", dir, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out)
}

// gitOutRaw is gitOut without trimming, so NUL-delimited output survives intact.
func gitOutRaw(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git -C %s %s: %v: %s", dir, strings.Join(args, " "), err, stderr.String())
	}
	return []byte(stdout.String())
}

// gitTry runs git -C <dir> and returns raw stdout plus an error carrying the
// captured stderr; it never touches testing.T so callers can probe for an
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

func (e *gitError) Error() string { return e.err.Error() + ": " + strings.TrimSpace(e.stderr) }

// configRepoIdentity pins a deterministic committer identity and disables gpg
// signing so a developer's global config cannot perturb the fixtures.
func configRepoIdentity(t *testing.T, dir string) {
	t.Helper()
	gitOut(t, dir, "config", "user.name", "t")
	gitOut(t, dir, "config", "user.email", "t@t")
	gitOut(t, dir, "config", "commit.gpgsign", "false")
}

// writeWorktreeFile writes content (creating parent directories) at a
// repo-relative path.
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

// mainModeRepo builds a bare origin whose main branch holds README.md,
// .docket.yml, main.go, and a .gitignore excluding .docket/ and .worktrees/. A
// writer clone advances origin; the primary clone under test is checked out on
// main. The only worktree whose bytes a Prepare must preserve is the primary.
func mainModeRepo(t *testing.T) *wsRepos {
	t.Helper()
	requireGit(t)
	useBackgroundOffGit(t)
	root := testsupport.TempDir(t)
	r := &wsRepos{
		Origin:  filepath.Join(root, "origin.git"),
		Writer:  filepath.Join(root, "writer"),
		Primary: filepath.Join(root, "primary"),
	}

	gitOut(t, root, "init", "--bare", "-b", "main", r.Origin)

	gitOut(t, root, "init", "-b", "main", r.Writer)
	configRepoIdentity(t, r.Writer)
	writeWorktreeFile(t, r.Writer, "README.md", "readme\n")
	writeWorktreeFile(t, r.Writer, ".docket.yml", "version: 1\n")
	writeWorktreeFile(t, r.Writer, "main.go", "package main\n")
	writeWorktreeFile(t, r.Writer, ".gitignore", ".docket/\n.worktrees/\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "main content")
	gitOut(t, r.Writer, "remote", "add", "origin", r.Origin)
	gitOut(t, r.Writer, "push", "-q", "-u", "origin", "main")

	gitOut(t, root, "clone", "-q", r.Origin, r.Primary)
	configRepoIdentity(t, r.Primary)

	r.Preserve = []string{r.Primary}
	return r
}

// docketModeRepo builds a bare origin with branch main (as in main mode) plus an
// orphan "docket" branch holding planning files. The primary clone adds three
// linked worktrees: ".docket" parked on docket, a detached transaction-style
// worktree outside the primary, and a sibling ".worktrees/other" feature
// worktree — four registered worktrees in total, all of which a Prepare must
// leave byte-identical.
func docketModeRepo(t *testing.T) *wsRepos {
	t.Helper()
	requireGit(t)
	useBackgroundOffGit(t)
	root := testsupport.TempDir(t)
	r := &wsRepos{
		Origin:  filepath.Join(root, "origin.git"),
		Writer:  filepath.Join(root, "writer"),
		Primary: filepath.Join(root, "primary"),
	}

	gitOut(t, root, "init", "--bare", "-b", "main", r.Origin)

	gitOut(t, root, "init", "-b", "main", r.Writer)
	configRepoIdentity(t, r.Writer)
	writeWorktreeFile(t, r.Writer, ".docket.yml", "version: 1\n")
	writeWorktreeFile(t, r.Writer, "main.go", "package main\n")
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

	gitOut(t, root, "clone", "-q", r.Origin, r.Primary)
	configRepoIdentity(t, r.Primary)

	docketWt := filepath.Join(r.Primary, ".docket")
	txnWt := filepath.Join(root, "txn")
	siblingWt := filepath.Join(r.Primary, ".worktrees", "other")
	gitOut(t, r.Primary, "worktree", "add", "-q", "-B", "docket", docketWt, "origin/docket")
	gitOut(t, r.Primary, "worktree", "add", "-q", "--detach", txnWt, "main")
	gitOut(t, r.Primary, "worktree", "add", "-q", "-b", "feat/other", siblingWt, "main")

	r.Preserve = []string{r.Primary, docketWt, txnWt, siblingWt}
	return r
}

// advanceMain commits a new file on main in the writer clone, pushes it to
// origin, and returns the new origin-main commit. The primary clone's
// origin/main tracking ref is deliberately left stale, so a Prepare that reports
// this commit as its base proves it performed a real fetch rather than trusting
// the cached tracking ref.
func (r *wsRepos) advanceMain(t *testing.T) gitcli.ObjectID {
	t.Helper()
	gitOut(t, r.Writer, "checkout", "-q", "main")
	writeWorktreeFile(t, r.Writer, "advanced.txt", "moved forward\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "advance main")
	gitOut(t, r.Writer, "push", "-q", "origin", "main")
	return gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "HEAD"))
}

// pushBranch creates <branch> in the writer clone from <from> (a ref the writer
// can resolve, e.g. "main" or another branch), adds one distinguishing file,
// commits, pushes it to origin, and returns the new commit. It lets a stacked
// scenario give the resolved base branch a real remote commit distinct from
// main's tip, so a Prepare that starts the workspace there is observable.
func (r *wsRepos) pushBranch(t *testing.T, branch, from string) gitcli.ObjectID {
	t.Helper()
	gitOut(t, r.Writer, "checkout", "-q", "-B", branch, from)
	writeWorktreeFile(t, r.Writer, "on-"+strings.ReplaceAll(branch, "/", "-")+".txt", "commit on "+branch+"\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "branch "+branch)
	gitOut(t, r.Writer, "push", "-q", "origin", branch)
	head := gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "HEAD"))
	gitOut(t, r.Writer, "checkout", "-q", "main")
	return head
}

// newService builds a real gitcli.Client, wraps it in a Service, and discovers
// the canonical Repository from the primary worktree. The returned Repository is
// symlink-canonical (Discover resolves every hop), which every path identity
// comparison in Prepare depends on.
func (r *wsRepos) newService(t *testing.T) (*Service, gitcli.Repository) {
	t.Helper()
	c, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	svc, err := NewService(c)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	repo, err := c.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: r.Primary})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return svc, repo
}

// symbolicHead returns the branch HEAD points at, or "DETACHED" for a detached
// HEAD, so a snapshot records which branch a worktree is on.
func symbolicHead(t *testing.T, dir string) string {
	t.Helper()
	out, err := gitTry(dir, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return "DETACHED"
	}
	return strings.TrimSpace(out)
}

// snapshotTree captures a worktree's observable state as a path->hash map: its
// HEAD commit, symbolic branch, porcelain-v2 status, staged index, and a content
// hash of every tracked or non-ignored-untracked file. It reads through git
// (respecting .gitignore) so a freshly attached, ignored .worktrees/<slug> is
// invisible to a preservation comparison of the primary worktree.
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	m := map[string]string{
		"HEAD":     gitOut(t, dir, "rev-parse", "HEAD"),
		"symbolic": symbolicHead(t, dir),
		"status":   string(gitOutRaw(t, dir, "status", "--porcelain=v2", "-z", "--untracked-files=all")),
		"index":    string(gitOutRaw(t, dir, "ls-files", "--stage", "-z")),
	}
	raw := gitOutRaw(t, dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	for _, name := range strings.Split(string(raw), "\x00") {
		if name == "" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			m["file:"+name] = "unreadable:" + err.Error()
			continue
		}
		sum := sha256.Sum256(b)
		m["file:"+name] = hex.EncodeToString(sum[:])
	}
	return m
}

// assertUnchanged fails the test if the current snapshot of dir differs in any
// key from before, naming every drifted key. It is the uninvolved-worktree
// preservation proof used after each Prepare scenario.
func assertUnchanged(t *testing.T, before map[string]string, dir string) {
	t.Helper()
	after := snapshotTree(t, dir)
	for k, bv := range before {
		av, ok := after[k]
		if !ok {
			t.Errorf("preservation: %s: key %q disappeared", dir, k)
			continue
		}
		if av != bv {
			t.Errorf("preservation: %s: key %q changed\n before=%q\n after =%q", dir, k, bv, av)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("preservation: %s: key %q appeared", dir, k)
		}
	}
}

// snapshotAll snapshots every worktree a Prepare must preserve.
func (r *wsRepos) snapshotAll(t *testing.T) map[string]map[string]string {
	t.Helper()
	out := make(map[string]map[string]string, len(r.Preserve))
	for _, wt := range r.Preserve {
		out[wt] = snapshotTree(t, wt)
	}
	return out
}

// assertAllUnchanged replays every preserved worktree's snapshot.
func (r *wsRepos) assertAllUnchanged(t *testing.T, before map[string]map[string]string) {
	t.Helper()
	for wt, snap := range before {
		assertUnchanged(t, snap, wt)
	}
}

// eachTopology runs fn against both fixtures, so every core scenario is proven
// on the plain repo and the docket-style repo with its extra worktrees.
func eachTopology(t *testing.T, fn func(t *testing.T, r *wsRepos)) {
	t.Helper()
	t.Run("main", func(t *testing.T) { fn(t, mainModeRepo(t)) })
	t.Run("docket", func(t *testing.T) { fn(t, docketModeRepo(t)) })
}

// TestHarnessBuildersProduceExpectedTopology is the harness self-test: it proves
// each builder produces the topology the Prepare tests depend on.
func TestHarnessBuildersProduceExpectedTopology(t *testing.T) {
	requireGit(t)

	t.Run("main", func(t *testing.T) {
		r := mainModeRepo(t)
		if got := gitOut(t, r.Origin, "rev-parse", "--is-bare-repository"); got != "true" {
			t.Errorf("origin is-bare-repository = %q, want true", got)
		}
		if got := gitOut(t, r.Primary, "status", "--porcelain"); got != "" {
			t.Errorf("primary status not clean:\n%s", got)
		}
		if n := countWorktrees(gitOut(t, r.Primary, "worktree", "list", "--porcelain")); n != 1 {
			t.Errorf("registered worktrees = %d, want 1", n)
		}
	})

	t.Run("docket", func(t *testing.T) {
		r := docketModeRepo(t)
		if got := gitOut(t, r.Primary, "status", "--porcelain"); got != "" {
			t.Errorf("docket primary status not clean:\n%s", got)
		}
		wl := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
		if n := countWorktrees(wl); n != 4 {
			t.Errorf("registered worktrees = %d, want 4 (primary + .docket + txn + sibling):\n%s", n, wl)
		}
		for _, want := range []string{"refs/heads/docket", "refs/heads/feat/other"} {
			if !strings.Contains(wl, want) {
				t.Errorf("worktree list missing %q:\n%s", want, wl)
			}
		}
		if len(r.Preserve) != 4 {
			t.Errorf("Preserve = %d worktrees, want 4", len(r.Preserve))
		}
	})
}

// countWorktrees counts "worktree <path>" stanza headers in porcelain output.
func countWorktrees(porcelain string) int {
	n := 0
	for _, line := range strings.Split(porcelain, "\n") {
		if strings.HasPrefix(line, "worktree ") {
			n++
		}
	}
	return n
}
