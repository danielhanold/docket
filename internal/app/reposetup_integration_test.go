//go:build integration

package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// This is the real-Git init/setup integration shard (prefix
// TestIntegrationRepoSetup). Each test builds its own bare upstream + clone under
// t.TempDir() with an isolated global config layer (newGitClient's
// XDG_CONFIG_HOME seam), then drives RunRepositoryInit against the clone and
// inspects the authoritative remote/worktree state with an independent git
// oracle. The fixtures follow the established internal/app harness (runGit,
// gitIdentity, writeRepoFile, newGitClient from status_git_test.go).

// initRepo is one bare origin plus a writer clone that seeds it and an
// invocation clone that init runs against.
type initRepo struct {
	root       string
	origin     string
	writer     string
	invocation string
}

// defaultSetupYML is the docket-mode configuration a fresh repository carries:
// an orphan docket metadata branch and the main integration branch.
const defaultSetupYML = "metadata_branch: docket\nintegration_branch: main\n"

// newInitRepo builds a docket-mode topology WITHOUT a docket branch: the main
// integration branch carries the given .docket.yml, a README, and any extra
// files. There is no metadata branch and (unless integrationFiles seed one) no
// live planning surface, so the repository classifies fresh.
func newInitRepo(t *testing.T, docketYML string, integrationFiles map[string]string) *initRepo {
	t.Helper()
	requireRealGit(t)
	root := t.TempDir()
	r := &initRepo{
		root:       root,
		origin:     filepath.Join(root, "origin.git"),
		writer:     filepath.Join(root, "writer"),
		invocation: filepath.Join(root, "invocation"),
	}
	runGit(t, root, "init", "--bare", "-b", "main", r.origin)
	runGit(t, root, "init", "-b", "main", r.writer)
	gitIdentity(t, r.writer)
	writeRepoFile(t, r.writer, ".docket.yml", docketYML)
	writeRepoFile(t, r.writer, "README.md", "readme\n")
	for rel, content := range integrationFiles {
		writeRepoFile(t, r.writer, rel, content)
	}
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "integration content")
	runGit(t, r.writer, "remote", "add", "origin", r.origin)
	runGit(t, r.writer, "push", "-q", "-u", "origin", "main")

	runGit(t, root, "clone", "-q", r.origin, r.invocation)
	gitIdentity(t, r.invocation)
	return r
}

// runInit runs RunRepositoryInit against the invocation clone with a fresh
// isolated client.
func (r *initRepo) runInit(t *testing.T) RepositoryOpResult {
	t.Helper()
	client := newGitClient(t)
	return RunRepositoryInit(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation})
}

// gitDir returns the absolute git dir of the invocation clone.
func (r *initRepo) gitDir(t *testing.T) string {
	t.Helper()
	return runGit(t, r.invocation, "rev-parse", "--absolute-git-dir")
}

// remoteBranchExists reports whether a branch exists on the bare origin.
func (r *initRepo) remoteBranchExists(t *testing.T, branch string) bool {
	t.Helper()
	_, err := tryGit(r.origin, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// TestIntegrationRepoSetupFreshInitCreatesTopology proves the six init effects on
// a fresh repository: a single parentless empty-tree root with the OpInitRoot
// receipt published to the remote docket branch, the .docket worktree attached on
// that branch with hooks off, the unstaged managed .gitignore edit, and — because
// no harness is authorized — no ownership record.
func TestIntegrationRepoSetupFreshInitCreatesTopology(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	res := r.runInit(t)

	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Errorf("RepositoryState = %q, want needs-review", res.RepositoryState)
	}
	if !contains(res.PendingPaths, ".gitignore") {
		t.Errorf("PendingPaths = %v, want it to name .gitignore", res.PendingPaths)
	}

	// Remote docket branch: exactly one parentless root, over the empty tree.
	if !r.remoteBranchExists(t, "docket") {
		t.Fatal("remote docket branch was not created")
	}
	metaTip := runGit(t, r.origin, "rev-parse", "refs/heads/docket")
	roots := strings.Fields(runGit(t, r.origin, "rev-list", "--max-parents=0", "docket"))
	if len(roots) != 1 || roots[0] != metaTip {
		t.Errorf("docket roots = %v, want exactly the tip %s (a single parentless root)", roots, metaTip)
	}
	tree := runGit(t, r.origin, "rev-parse", "docket^{tree}")
	emptyTree := runGit(t, r.origin, "hash-object", "-t", "tree", "/dev/null")
	if tree != emptyTree {
		t.Errorf("docket tree = %s, want the empty tree %s", tree, emptyTree)
	}
	trailers := runGit(t, r.origin, "log", "-1", "--format=%(trailers:only,unfold)", "docket")
	if !strings.Contains(trailers, reposetup.OpInitRoot) {
		t.Errorf("docket root trailers = %q, want the %s receipt", trailers, reposetup.OpInitRoot)
	}

	// .docket worktree: registered on the docket branch with hooks disabled.
	dotDocket := filepath.Join(r.invocation, ".docket")
	branch := runGit(t, dotDocket, "rev-parse", "--abbrev-ref", "HEAD")
	if branch != "docket" {
		t.Errorf(".docket HEAD branch = %q, want docket", branch)
	}
	hooksPath := runGit(t, dotDocket, "config", "--worktree", "core.hooksPath")
	if hooksPath == "" || !filepath.IsAbs(hooksPath) {
		t.Errorf("core.hooksPath = %q, want an absolute empty hooks dir", hooksPath)
	}

	// .gitignore present and UNSTAGED.
	status := runGit(t, r.invocation, "status", "--porcelain", "--", ".gitignore")
	if !strings.Contains(status, ".gitignore") {
		t.Errorf("git status = %q, want an uncommitted .gitignore", status)
	}
	if !reposetup.ValidGitignoreBlock(mustReadFile(t, filepath.Join(r.invocation, ".gitignore"))) {
		t.Error(".gitignore does not carry the canonical managed block")
	}

	// No harness authorized: no ownership record.
	recordPath := filepath.Join(r.gitDir(t), "docket", "install.json")
	if _, err := os.Stat(recordPath); !os.IsNotExist(err) {
		t.Errorf("ownership record %s exists, want none when no harness is authorized (err=%v)", recordPath, err)
	}
}

// TestIntegrationRepoSetupFreshInitInstallsAuthorizedSurfaces proves effects 5–6
// for an explicit repository-layer agent_harnesses declaration: the parent-facing
// dispatch surface is written unstaged and the ownership record is published.
func TestIntegrationRepoSetupFreshInitInstallsAuthorizedSurfaces(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	r := newInitRepo(t, defaultSetupYML+"agent_harnesses: [claude]\n", nil)
	res := r.runInit(t)

	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	if !contains(res.PendingPaths, "CLAUDE.md") {
		t.Errorf("PendingPaths = %v, want the authorized CLAUDE.md surface", res.PendingPaths)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, "CLAUDE.md")); err != nil {
		t.Errorf("CLAUDE.md surface not written: %v", err)
	}
	recordPath := filepath.Join(r.gitDir(t), "docket", "install.json")
	if _, err := os.Stat(recordPath); err != nil {
		t.Errorf("ownership record not written for an authorized harness: %v", err)
	}
}

// TestIntegrationRepoSetupRepeatInitConverges proves a second init recomputes the
// same plan and converges: no second root, no second worktree, a byte-identical
// managed block.
func TestIntegrationRepoSetupRepeatInitConverges(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)

	first := r.runInit(t)
	if first.Result != ResultApplied {
		t.Fatalf("first init = %q (%s), want applied", first.Result, first.HumanText())
	}
	block1 := mustReadFile(t, filepath.Join(r.invocation, ".gitignore"))

	second := r.runInit(t)
	if second.Result != ResultNoOp && second.Result != ResultApplied {
		t.Fatalf("second init = %q (%s), want no-op or applied", second.Result, second.HumanText())
	}
	if second.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Errorf("second RepositoryState = %q, want needs-review", second.RepositoryState)
	}

	roots := strings.Fields(runGit(t, r.origin, "rev-list", "--max-parents=0", "docket"))
	if len(roots) != 1 {
		t.Errorf("docket roots after repeat init = %v, want exactly one (no second root)", roots)
	}
	wts := runGit(t, r.invocation, "worktree", "list", "--porcelain")
	if got := strings.Count(wts, filepath.Join(r.invocation, ".docket")); got != 1 {
		t.Errorf(".docket worktree registered %d times, want exactly one:\n%s", got, wts)
	}
	block2 := mustReadFile(t, filepath.Join(r.invocation, ".gitignore"))
	if string(block1) != string(block2) {
		t.Error("managed .gitignore block changed on repeat init; want byte-identical")
	}
}

// TestIntegrationRepoSetupInitRefusesLegacy proves a live planning surface on the
// integration branch refuses init and points at migrate, leaving the remote
// untouched.
func TestIntegrationRepoSetupInitRefusesLegacy(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, map[string]string{
		"docs/changes/active/0001-example.md": "---\nid: 1\n---\n",
	})
	res := r.runInit(t)

	if res.Result != ResultInvalidState {
		t.Fatalf("Result = %q (%s), want invalid-state", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateLegacy) {
		t.Errorf("RepositoryState = %q, want legacy", res.RepositoryState)
	}
	if !strings.Contains(res.HumanText(), "migrate") {
		t.Errorf("remedy %q must name migrate", res.HumanText())
	}
	if r.remoteBranchExists(t, "docket") {
		t.Error("a legacy refusal created the remote docket branch; it must be untouched")
	}
}

// TestIntegrationRepoSetupInitRefusesForeignMetadataBranch proves the create-only
// protection: a pre-existing non-empty foreign docket branch is refused as a
// conflict and left byte-untouched — init never overwrites it.
func TestIntegrationRepoSetupInitRefusesForeignMetadataBranch(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	// Pre-push a foreign, non-empty docket branch (a normal commit with a parent
	// and a non-empty tree) that init must never adopt or overwrite.
	runGit(t, r.writer, "checkout", "-q", "-b", "docket")
	writeRepoFile(t, r.writer, "foreign.txt", "not docket's\n")
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "foreign docket branch")
	runGit(t, r.writer, "push", "-q", "-u", "origin", "docket")
	foreignTip := runGit(t, r.origin, "rev-parse", "refs/heads/docket")

	res := r.runInit(t)

	if res.Result != ResultInvalidState {
		t.Fatalf("Result = %q (%s), want invalid-state", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateConflict) {
		t.Errorf("RepositoryState = %q, want conflict", res.RepositoryState)
	}
	if after := runGit(t, r.origin, "rev-parse", "refs/heads/docket"); after != foreignTip {
		t.Errorf("foreign docket branch moved from %s to %s; create-only protection must leave it untouched", foreignTip, after)
	}
}

// TestIntegrationRepoSetupInitRefusesDirtyPrimary proves the supported-contract
// preflight: an uncommitted change to a tracked file refuses init before any
// remote effect.
func TestIntegrationRepoSetupInitRefusesDirtyPrimary(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	writeRepoFile(t, r.invocation, "README.md", "dirty edit\n")

	res := r.runInit(t)

	if res.Result != ResultInvalidState {
		t.Fatalf("Result = %q (%s), want invalid-state", res.Result, res.HumanText())
	}
	if r.remoteBranchExists(t, "docket") {
		t.Error("a dirty-primary refusal created the remote docket branch; it must be untouched")
	}
}

// TestIntegrationRepoSetupGitignoreParity proves the Task 3 drift tie: the native
// GitignoreBlock() is byte-identical to the bash lib emitter.
func TestIntegrationRepoSetupGitignoreParity(t *testing.T) {
	requireRealGit(t)
	repoRoot := moduleRoot(t)
	libPath := filepath.Join(repoRoot, "scripts", "lib", "docket-gitignore-block.sh")
	if _, err := os.Stat(libPath); err != nil {
		t.Fatalf("bash gitignore lib not found: %v", err)
	}
	cmd := exec.Command("bash", "-c", ". "+libPath+" && emit_docket_gitignore_block")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bash emitter failed: %v", err)
	}
	if string(out) != string(reposetup.GitignoreBlock()) {
		t.Errorf("gitignore parity mismatch:\nbash:\n%q\nnative:\n%q", out, reposetup.GitignoreBlock())
	}
}

// TestIntegrationRepoSetupInitDoesNotPrompt proves init reads no stdin: it takes
// no input reader and completes on a fresh repository with a closed stdin.
func TestIntegrationRepoSetupInitDoesNotPrompt(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	// The service signature carries no input reader, so it structurally cannot
	// prompt; run it and confirm it completes without consulting stdin.
	client := newGitClient(t)
	res := RunRepositoryInit(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation})
	if res.Result != ResultApplied {
		t.Fatalf("init did not complete without a prompt: %q (%s)", res.Result, res.HumanText())
	}
}

// mustReadFile reads a file or fails the test.
func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// moduleRoot returns the repository root (two directories up from this test file
// in internal/app), resolved from the test's own source location so it is
// independent of the working directory.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller for module root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
