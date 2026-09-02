package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/suiterunner"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// These are the Git-backed StatusReader integration tests. Each builds a real
// temporary repository topology — a bare file origin, an independent writer
// clone that advances the remote, and the invocation clone under test — with
// testsupport.TempDir(t) paths, then drives NewGitStatusReader over a real gitcli client.
// The harness is modelled on internal/gitcli/harness_test.go, whose builders
// are unexported to that package, so the fixture is rebuilt here rather than
// reused.

// requireRealGit skips when git is genuinely absent from PATH.
func requireRealGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

// runGit runs real git with -C <dir> and fails the test on a non-zero exit. It
// is the plumbing oracle every fixture assertion compares against, independent
// of the adapter under test.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := tryGit(dir, args...)
	if err != nil {
		t.Fatalf("git -C %s %s: %v", dir, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(out)
}

// bgOffGit memoizes a process-shared global git config that disables the
// background maintenance/gc/fsmonitor mechanisms (suiterunner.GitBackgroundOff)
// plus a synthetic identity.
var (
	bgOffGitOnce sync.Once
	bgOffGitPath string
	bgOffGitErr  error
)

// backgroundOffGitEnv returns the "GIT_CONFIG_GLOBAL=<path>" override that points
// the direct-git oracles (tryGit and the file-local git helpers) at a config
// carrying suiterunner.GitBackgroundOff, so a detached git housekeeping child
// cannot outlive a test and keep writing into a testsupport.TempDir, racing its
// drain-then-retry removal to "directory not empty" under parallel load (change
// 0373, package sighting 2). It mirrors testsupport.GitEnv but takes no
// *testing.T: the direct oracles are shared by this package's PARALLEL e2e tests,
// where t.Setenv would panic, so a per-command cmd.Env override on one shared,
// deterministic, read-only config file is the parallel-safe seam. Git spawned
// through the product gitcli client scrubs GIT_CONFIG (sanitizeEnvironment), so
// those housekeeping children are instead absorbed by the fixture's
// drain-then-retry removal.
func backgroundOffGitEnv() string {
	bgOffGitOnce.Do(func() {
		dir, err := os.MkdirTemp("", "docket-bgoff-git-*")
		if err != nil {
			bgOffGitErr = err
			return
		}
		cfg := "[user]\n\tname = docket test\n\temail = test@docket.invalid\n[init]\n\tdefaultBranch = main\n" + suiterunner.GitBackgroundOff
		p := filepath.Join(dir, "gitconfig")
		if err := os.WriteFile(p, []byte(cfg), 0o644); err != nil {
			bgOffGitErr = err
			return
		}
		bgOffGitPath = p
	})
	if bgOffGitErr != nil {
		// A tmp-write failure here is environmental, not a test signal; fall back
		// to the inherited config rather than aborting from a helper with no t.
		return ""
	}
	return "GIT_CONFIG_GLOBAL=" + bgOffGitPath
}

// tryGit runs git -C <dir> and returns raw stdout plus an error carrying the
// captured stderr; it never touches testing.T so a caller can probe an expected
// failure. Its git child runs under backgroundOffGitEnv so no detached
// housekeeping process outlives the run to race fixture teardown.
func tryGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if kv := backgroundOffGitEnv(); kv != "" {
		cmd.Env = append(os.Environ(), kv)
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), &execErr{err: err, stderr: stderr.String()}
	}
	return stdout.String(), nil
}

type execErr struct {
	err    error
	stderr string
}

func (e *execErr) Error() string { return e.err.Error() + ": " + strings.TrimSpace(e.stderr) }

// gitIdentity pins a deterministic committer identity and disables signing so a
// developer's global config cannot perturb the fixtures.
func gitIdentity(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "config", "user.name", "t")
	runGit(t, dir, "config", "user.email", "t@t")
	runGit(t, dir, "config", "commit.gpgsign", "false")
}

// writeRepoFile writes content (creating parents) at a repo-relative path.
func writeRepoFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newGitClient builds a production gitcli client, isolating the global docket
// configuration layer to an empty XDG dir so a developer's own config cannot
// steer resolution.
func newGitClient(t *testing.T) *gitcli.Client {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", testsupport.TempDir(t))
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	return client
}

// changeRecord is a minimal, valid active-change record body.
func changeRecord(id int, slug, title string) string {
	return "---\n" +
		"id: " + strconv.Itoa(id) + "\n" +
		"slug: " + slug + "\n" +
		"title: " + title + "\n" +
		"status: proposed\n" +
		"priority: high\n" +
		"type: feat\n" +
		"created: 2026-01-02\n" +
		"---\n\nBody of " + slug + ".\n"
}

// gitRepo is the bare origin plus writer and invocation clones.
type gitRepo struct {
	root       string
	origin     string
	writer     string
	invocation string
}

// newLegacyRepo builds the RETIRED single-branch topology (change 0363): every
// planning record lives on the default branch main, whose .docket.yml still
// declares the obsolete metadata_branch: main key, and no docket metadata
// branch exists. When the committed files include a live planning surface, the
// operational gate classifies this repository `legacy` and refuses every
// ordinary command — use it ONLY for tests whose subject is that refusal or
// the migration path; a test that needs a WORKING repository uses
// newWorkingRepo.
func newLegacyRepo(t *testing.T, files map[string]string) *gitRepo {
	t.Helper()
	requireRealGit(t)
	root := testsupport.TempDir(t)
	r := &gitRepo{
		root:       root,
		origin:     filepath.Join(root, "origin.git"),
		writer:     filepath.Join(root, "writer"),
		invocation: filepath.Join(root, "invocation"),
	}

	runGit(t, root, "init", "--bare", "-b", "main", r.origin)
	runGit(t, root, "init", "-b", "main", r.writer)
	gitIdentity(t, r.writer)

	writeRepoFile(t, r.writer, ".docket.yml", "metadata_branch: main\n")
	writeRepoFile(t, r.writer, "README.md", "readme\n")
	for rel, content := range files {
		writeRepoFile(t, r.writer, rel, content)
	}
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "main content")
	runGit(t, r.writer, "remote", "add", "origin", r.origin)
	runGit(t, r.writer, "push", "-q", "-u", "origin", "main")

	runGit(t, root, "clone", "-q", r.origin, r.invocation)
	gitIdentity(t, r.invocation)
	return r
}

// newWorkingRepo builds the ONE supported repository topology (change 0363):
// an orphan docket metadata branch beside the integration branch main. The
// given files are partitioned by their repository role — metadata-side records
// (changes, ADRs, specs) land on the docket branch, everything else on main —
// so a former single-branch fixture converts by intent, not by path surgery at
// every call site.
func newWorkingRepo(t *testing.T, files map[string]string) *gitRepo {
	t.Helper()
	mainFiles := map[string]string{}
	docketRecords := map[string]string{}
	for rel, content := range files {
		if isMetadataSidePath(rel) {
			docketRecords[rel] = content
		} else {
			mainFiles[rel] = content
		}
	}
	return newDocketModeRepo(t, mainFiles, docketRecords)
}

// isMetadataSidePath reports whether a repo-relative fixture path belongs on
// the docket metadata branch under the default configuration: changes (active,
// archive, learnings, board), ADRs, and specs. Plans, results, code, and
// documentation stay on the integration branch.
func isMetadataSidePath(rel string) bool {
	return strings.HasPrefix(rel, "docs/changes/") ||
		strings.HasPrefix(rel, "docs/adrs/") ||
		strings.HasPrefix(rel, "docs/superpowers/specs/")
}

// newDocketModeRepo builds a docket-topology repository: main carries code
// while an orphan docket branch carries the planning records. The two branches
// carry genuinely different trees, so a corpus read that came from the wrong
// branch is observable.
func newDocketModeRepo(t *testing.T, mainFiles, docketRecords map[string]string) *gitRepo {
	t.Helper()
	requireRealGit(t)
	root := testsupport.TempDir(t)
	r := &gitRepo{
		root:       root,
		origin:     filepath.Join(root, "origin.git"),
		writer:     filepath.Join(root, "writer"),
		invocation: filepath.Join(root, "invocation"),
	}

	runGit(t, root, "init", "--bare", "-b", "main", r.origin)
	runGit(t, root, "init", "-b", "main", r.writer)
	gitIdentity(t, r.writer)

	writeRepoFile(t, r.writer, ".docket.yml", "integration_branch: main\n")
	writeRepoFile(t, r.writer, "main.go", "package main\n")
	for rel, content := range mainFiles {
		writeRepoFile(t, r.writer, rel, content)
	}
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "main content")
	runGit(t, r.writer, "remote", "add", "origin", r.origin)
	runGit(t, r.writer, "push", "-q", "-u", "origin", "main")

	// Orphan docket branch: unrelated history, planning records only. Clear the
	// whole working tree (everything but .git) so no code file carried over from
	// main can leak into the docket tree.
	runGit(t, r.writer, "checkout", "-q", "--orphan", "docket")
	runGit(t, r.writer, "rm", "-rfq", "--cached", ".")
	clearWorktree(t, r.writer)
	if len(docketRecords) == 0 {
		// A metadata branch always holds at least the board view, so an empty
		// record set still produces a committable orphan tree.
		writeRepoFile(t, r.writer, "docs/changes/BOARD.md", "# Board\n")
	}
	for rel, content := range docketRecords {
		writeRepoFile(t, r.writer, rel, content)
	}
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "docket planning")
	runGit(t, r.writer, "push", "-q", "-u", "origin", "docket")
	runGit(t, r.writer, "checkout", "-q", "main")

	runGit(t, root, "clone", "-q", r.origin, r.invocation)
	gitIdentity(t, r.invocation)
	return r
}

// clearWorktree removes every top-level entry of a worktree except .git, so an
// orphan branch can be populated from a clean slate.
func clearWorktree(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
			t.Fatal(err)
		}
	}
}

// writerAdvance commits files on branch in the writer clone (creating it from
// the current checkout when absent), pushes to origin, and returns the new
// commit id read from the writer's own rev-parse — an independent oracle.
func (r *gitRepo) writerAdvance(t *testing.T, branch string, files map[string]string) string {
	t.Helper()
	if _, err := tryGit(r.writer, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
		runGit(t, r.writer, "checkout", "-q", branch)
	} else {
		runGit(t, r.writer, "checkout", "-q", "-b", branch)
	}
	for rel, content := range files {
		writeRepoFile(t, r.writer, rel, content)
	}
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "advance "+branch)
	runGit(t, r.writer, "push", "-q", "origin", branch)
	return runGit(t, r.writer, "rev-parse", "HEAD")
}

// blobID reads the object id of a repo-relative path at a revision from the
// writer clone — the oracle every version assertion compares against.
func (r *gitRepo) blobID(t *testing.T, dir, rev, path string) string {
	t.Helper()
	return runGit(t, dir, "rev-parse", rev+":"+path)
}

// --- tests ----------------------------------------------------------------

// --- small assertion helpers ---------------------------------------------

func pathsOf(blobs []StatusBlob) []string {
	out := make([]string, 0, len(blobs))
	for _, b := range blobs {
		out = append(out, b.Path)
	}
	sort.Strings(out)
	return out
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func isStatusExternal(err error) bool {
	return errors.Is(err, ErrStatusExternal)
}

// worktreeChecksum returns a path->content-hash map over every file under root
// except the .git directory, so a stray write anywhere in the worktree is
// observable.
func worktreeChecksum(t *testing.T, root string) map[string]string {
	t.Helper()
	sums := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		h := sha256.Sum256(data)
		sums[rel] = hex.EncodeToString(h[:])
		return nil
	})
	if err != nil {
		t.Fatalf("worktree checksum: %v", err)
	}
	return sums
}

func equalChecksums(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
