package transaction

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file builds real temporary Git repositories for the engine tests, ported
// from internal/gitcli/harness_test.go and extended with a small complete Docket
// corpus committed on the target branch. Two topologies are produced: a plain
// "main mode" repo whose target branch is refs/heads/main, and a docket-style
// repo whose target branch is an orphan refs/heads/docket with a linked .docket
// worktree in the invocation clone. Each is backed by a bare file origin, an
// independent writer clone that advances the origin, and the invocation clone the
// engine discovers and operates. All paths live under testsupport.TempDir(t); builders
// return the raw testsupport.TempDir(t) spelling so the symlinked /tmp -> /private/tmp case
// on macOS is exercised. core.quotePath=true is pinned so a developer's global
// "false" cannot disarm a hostile-path proof. Everything here is _test.go only.

// testRepos is a bare origin plus two clones: a writer that advances the origin,
// and the invocation clone the engine discovers. Target names the fully qualified
// branch the corpus lives on and the engine writes to.
type testRepos struct {
	Origin     string
	Writer     string
	Invocation string
	Target     gitcli.RefName
}

// requireGit skips the test when no real git is on PATH.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	useBackgroundOffGit(t)
}

// useBackgroundOffGit points the git children these tests spawn at a per-fixture
// GIT_CONFIG_GLOBAL (testsupport.GitEnv) that disables auto-gc, auto-maintenance,
// and fsmonitor. The direct oracle helpers (hgitOut/hgitTry, matGit, newTxnRepo's
// run) inherit the test-process environment, so this reaches them; without it a
// detached git housekeeping child spawned by a fixture commit can outlive the
// test and keep writing into a testsupport.TempDir, racing RemoveAll teardown to
// "directory not empty" under parallel load (change 0373, sighting 4:
// TestKeyedCommitCarriesFiveTrailers/keyed). Git spawned through the product
// gitcli client scrubs GIT_CONFIG, so its housekeeping children are instead
// absorbed by the fixture's drain-then-retry removal. Set process-wide via
// t.Setenv because the low-level helpers take no *testing.T; safe because this
// package runs no test in parallel.
func useBackgroundOffGit(t *testing.T) {
	t.Helper()
	for _, kv := range testsupport.GitEnv(t) {
		if v, ok := strings.CutPrefix(kv, "GIT_CONFIG_GLOBAL="); ok {
			t.Setenv("GIT_CONFIG_GLOBAL", v)
		}
	}
}

// hgitOut runs real git with -C <dir>, returns trimmed stdout, and fails on a
// non-zero exit. It is the plumbing oracle fixture assertions compare against.
func hgitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := hgitTry(dir, args...)
	if err != nil {
		t.Fatalf("git -C %s %s: %v", dir, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out)
}

// hgitTry runs git -C <dir> and returns raw stdout plus an error carrying stderr;
// it never touches testing.T so callers can probe for an expected failure.
func hgitTry(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), &harnessGitError{err: err, stderr: stderr.String()}
	}
	return stdout.String(), nil
}

type harnessGitError struct {
	err    error
	stderr string
}

func (e *harnessGitError) Error() string {
	return e.err.Error() + ": " + strings.TrimSpace(e.stderr)
}

// hconfigIdentity pins a deterministic committer identity and disables signing.
func hconfigIdentity(t *testing.T, dir string) {
	t.Helper()
	hgitOut(t, dir, "config", "user.name", "t")
	hgitOut(t, dir, "config", "user.email", "t@t")
	hgitOut(t, dir, "config", "commit.gpgsign", "false")
}

// hwriteFile writes content (creating parent directories) at a repo-relative
// path, tolerating hostile bytes in the name.
func hwriteFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// corpusFiles is the small complete Docket corpus committed on the target branch:
// two active changes and one Accepted ADR. Every filename encodes the record's id
// and slug exactly as docket's writers do, so a snapshot over it carries no error
// findings and a before/after gate passes.
func corpusFiles() map[string]string {
	return map[string]string{
		"docs/changes/active/0001-first-change.md":  corpusChange(1, "first-change", "proposed"),
		"docs/changes/active/0002-second-change.md": corpusChange(2, "second-change", "proposed"),
		"docs/adrs/0001-first-decision.md":          corpusADR(1, "first-decision"),
	}
}

// corpusChange renders a well-formed change record with no cross-references, so a
// snapshot over it carries no error findings.
func corpusChange(id int, slug, status string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoa(id) + "\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A change'\n")
	b.WriteString("status: " + status + "\n")
	b.WriteString("priority: medium\n")
	b.WriteString("type: feat\n")
	b.WriteString("created: 2026-08-01\n")
	b.WriteString("updated: 2026-08-02\n")
	b.WriteString("---\n\n## Why\n\nBody.\n")
	return b.String()
}

// corpusADR renders a well-formed Accepted ADR record.
func corpusADR(id int, slug string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoa(id) + "\n")
	b.WriteString("slug: " + slug + "\n")
	b.WriteString("title: 'A decision'\n")
	b.WriteString("status: Accepted\n")
	b.WriteString("date: 2026-08-01\n")
	b.WriteString("---\n\n## Decision\n\nBody.\n")
	return b.String()
}

// itoa is a tiny local integer formatter that keeps the corpus builders free of
// an fmt import for one field.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// newMainModeRepos builds a bare origin whose branch main holds the corpus plus a
// .docket.yml and README.md, a writer clone that advances origin, and an
// invocation clone checked out on main with core.quotePath=true. Its Target is
// refs/heads/main.
func newMainModeRepos(t *testing.T) *testRepos {
	t.Helper()
	requireGit(t)
	root := testsupport.TempDir(t)
	r := &testRepos{
		Origin:     filepath.Join(root, "origin.git"),
		Writer:     filepath.Join(root, "writer"),
		Invocation: filepath.Join(root, "invocation"),
		Target:     "refs/heads/main",
	}

	hgitOut(t, root, "init", "--bare", "-b", "main", r.Origin)

	hgitOut(t, root, "init", "-b", "main", r.Writer)
	hconfigIdentity(t, r.Writer)
	hgitOut(t, r.Writer, "config", "core.quotePath", "true")

	hwriteFile(t, r.Writer, "README.md", "readme\n")
	hwriteFile(t, r.Writer, ".docket.yml", "version: 1\n")
	for rel, content := range corpusFiles() {
		hwriteFile(t, r.Writer, rel, content)
	}
	hgitOut(t, r.Writer, "add", "-A")
	hgitOut(t, r.Writer, "commit", "-q", "-m", "main content")
	hgitOut(t, r.Writer, "remote", "add", "origin", r.Origin)
	hgitOut(t, r.Writer, "push", "-q", "-u", "origin", "main")

	hgitOut(t, root, "clone", "-q", r.Origin, r.Invocation)
	hgitOut(t, r.Invocation, "config", "core.quotePath", "true")
	hconfigIdentity(t, r.Invocation)
	return r
}

// newDocketModeRepos builds a bare origin with branch main (.docket.yml, code, and
// a .gitignore excluding .docket/ and .worktrees/) plus an orphan "docket" branch
// holding the corpus. The invocation clone adds a linked ".docket" worktree parked
// on docket. Its Target is refs/heads/docket.
func newDocketModeRepos(t *testing.T) *testRepos {
	t.Helper()
	requireGit(t)
	root := testsupport.TempDir(t)
	r := &testRepos{
		Origin:     filepath.Join(root, "origin.git"),
		Writer:     filepath.Join(root, "writer"),
		Invocation: filepath.Join(root, "invocation"),
		Target:     "refs/heads/docket",
	}

	hgitOut(t, root, "init", "--bare", "-b", "main", r.Origin)

	hgitOut(t, root, "init", "-b", "main", r.Writer)
	hconfigIdentity(t, r.Writer)
	hgitOut(t, r.Writer, "config", "core.quotePath", "true")

	hwriteFile(t, r.Writer, ".docket.yml", "version: 1\n")
	hwriteFile(t, r.Writer, "main.go", "package main\n")
	hwriteFile(t, r.Writer, ".gitignore", ".docket/\n.worktrees/\n")
	hgitOut(t, r.Writer, "add", "-A")
	hgitOut(t, r.Writer, "commit", "-q", "-m", "main content")
	hgitOut(t, r.Writer, "remote", "add", "origin", r.Origin)
	hgitOut(t, r.Writer, "push", "-q", "-u", "origin", "main")

	// Orphan docket branch: unrelated history, the corpus only.
	hgitOut(t, r.Writer, "checkout", "-q", "--orphan", "docket")
	hgitOut(t, r.Writer, "rm", "-rfq", "--cached", ".")
	for _, name := range []string{".docket.yml", "main.go", ".gitignore"} {
		if err := os.Remove(filepath.Join(r.Writer, name)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	for rel, content := range corpusFiles() {
		hwriteFile(t, r.Writer, rel, content)
	}
	hgitOut(t, r.Writer, "add", "-A")
	hgitOut(t, r.Writer, "commit", "-q", "-m", "docket corpus")
	hgitOut(t, r.Writer, "push", "-q", "-u", "origin", "docket")
	hgitOut(t, r.Writer, "checkout", "-q", "main")

	hgitOut(t, root, "clone", "-q", r.Origin, r.Invocation)
	hconfigIdentity(t, r.Invocation)
	hgitOut(t, r.Invocation, "config", "core.quotePath", "true")
	hgitOut(t, r.Invocation, "worktree", "add", "-q", "-B", "docket", ".docket", "origin/docket")
	return r
}

// short returns the short branch name of the target ref (the part after
// refs/heads/), for oracle git commands that take a branch name.
func (r *testRepos) short() string {
	return strings.TrimPrefix(string(r.Target), "refs/heads/")
}

// discover builds a gitcli.Client and discovers the invocation repository the
// engine operates, exactly as the engine's caller would.
func (r *testRepos) discover(t *testing.T) (*gitcli.Client, gitcli.Repository) {
	t.Helper()
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: r.Invocation})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return client, repo
}

// originTip returns the origin's current commit for the target ref, read from the
// bare origin directly — an oracle independent of the adapter under test.
func (r *testRepos) originTip(t *testing.T) gitcli.ObjectID {
	t.Helper()
	return gitcli.ObjectID(hgitOut(t, r.Origin, "rev-parse", string(r.Target)))
}

// blobID returns the object id of a path at the target ref's tip on origin.
func (r *testRepos) blobID(t *testing.T, path string) gitcli.ObjectID {
	t.Helper()
	out, err := hgitTry(r.Origin, "rev-parse", string(r.Target)+":"+path)
	if err != nil {
		t.Fatalf("blobID %q: %v", path, err)
	}
	return gitcli.ObjectID(strings.TrimSpace(out))
}

// advanceOrigin makes the writer clone push one new change on the target branch,
// advancing origin so a subsequent lease loses. It returns the writer's new tip.
func (r *testRepos) advanceOrigin(t *testing.T, rel, content string) gitcli.ObjectID {
	t.Helper()
	branch := r.short()
	if branchExistsWriter(r.Writer, branch) {
		hgitOut(t, r.Writer, "checkout", "-q", branch)
	} else {
		hgitOut(t, r.Writer, "checkout", "-q", "-b", branch)
	}
	hwriteFile(t, r.Writer, rel, content)
	hgitOut(t, r.Writer, "add", "--", rel)
	hgitOut(t, r.Writer, "commit", "-q", "-m", "writer advance")
	hgitOut(t, r.Writer, "push", "-q", "origin", branch)
	return gitcli.ObjectID(hgitOut(t, r.Writer, "rev-parse", "HEAD"))
}

// branchExistsWriter reports whether refs/heads/<branch> exists in the writer.
func branchExistsWriter(dir, branch string) bool {
	_, err := hgitTry(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// diffTreePaths returns the exact changed-path set of commit against its first
// parent, rename detection off and NUL-delimited so hostile paths survive.
func diffTreePaths(t *testing.T, originDir string, commit gitcli.ObjectID) []string {
	t.Helper()
	out := hgitOutRaw(t, originDir, "diff-tree", "--no-renames", "--no-commit-id",
		"--name-only", "-z", "-r", string(commit))
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// hgitOutRaw runs git -C <dir> and returns UNtrimmed stdout (NUL-delimited output
// must not be trimmed), failing the test on a non-zero exit.
func hgitOutRaw(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := hgitTry(dir, args...)
	if err != nil {
		t.Fatalf("git -C %s %s: %v", dir, strings.Join(args, " "), err)
	}
	return out
}

// transactionsEmpty reports whether the repository's transactions root has no
// candidate directories left (every candidate cleaned). A missing root counts as
// empty.
func transactionsEmpty(t *testing.T, repo gitcli.Repository) bool {
	t.Helper()
	entries, err := os.ReadDir(transactionsRoot(repo))
	if err != nil {
		if os.IsNotExist(err) {
			return true
		}
		t.Fatalf("read transactions root: %v", err)
	}
	for _, e := range entries {
		// registry.lock is a mutex file, not a candidate; ignore it.
		if e.Name() == registryLockName {
			continue
		}
		return false
	}
	return true
}

// TestHarnessBuildersProduceExpectedTopology proves each builder yields the
// topology the engine tests depend on: a bare origin, a clean invocation checkout,
// the corpus on the target branch, and (docket mode) a linked docket worktree.
func TestHarnessBuildersProduceExpectedTopology(t *testing.T) {
	requireGit(t)

	t.Run("main", func(t *testing.T) {
		r := newMainModeRepos(t)
		if got := hgitOut(t, r.Origin, "rev-parse", "--is-bare-repository"); got != "true" {
			t.Errorf("origin is-bare = %q, want true", got)
		}
		if got := hgitOut(t, r.Invocation, "status", "--porcelain"); got != "" {
			t.Errorf("invocation status not clean:\n%s", got)
		}
		names := hgitOut(t, r.Origin, "ls-tree", "-r", "--name-only", "main")
		for _, want := range []string{"docs/adrs/0001-first-decision.md", "docs/changes/active/0001-first-change.md"} {
			if !strings.Contains(names, want) {
				t.Errorf("main tree missing corpus record %q", want)
			}
		}
	})

	t.Run("docket", func(t *testing.T) {
		r := newDocketModeRepos(t)
		if got := hgitOut(t, r.Invocation, "status", "--porcelain"); got != "" {
			t.Errorf("docket-mode invocation status not clean:\n%s", got)
		}
		docketNames := hgitOut(t, r.Invocation, "ls-tree", "-r", "--name-only", "origin/docket")
		if !strings.Contains(docketNames, "docs/changes/active/0001-first-change.md") {
			t.Errorf("docket branch missing corpus:\n%s", docketNames)
		}
		mainNames := hgitOut(t, r.Invocation, "ls-tree", "-r", "--name-only", "main")
		if strings.Contains(mainNames, "0001-first-change.md") {
			t.Errorf("main tree unexpectedly carries docket corpus:\n%s", mainNames)
		}
	})
}
