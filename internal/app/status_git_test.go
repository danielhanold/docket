package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// These are the Git-backed StatusReader integration tests. Each builds a real
// temporary repository topology — a bare file origin, an independent writer
// clone that advances the remote, and the invocation clone under test — with
// t.TempDir() paths, then drives NewGitStatusReader over a real gitcli client.
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

// tryGit runs git -C <dir> and returns raw stdout plus an error carrying the
// captured stderr; it never touches testing.T so a caller can probe an expected
// failure.
func tryGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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

// newMainModeRepo builds a main-mode topology: every planning record lives on
// the default branch main, whose .docket.yml declares metadata_branch: main.
func newMainModeRepo(t *testing.T, files map[string]string) *gitRepo {
	t.Helper()
	requireRealGit(t)
	root := t.TempDir()
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

// newDocketModeRepo builds a docket-mode topology: main carries code + a
// .docket.yml declaring metadata_branch: docket, while an orphan docket branch
// carries the planning records. The two branches carry genuinely different
// trees, so a corpus read that came from the wrong branch is observable.
func newDocketModeRepo(t *testing.T, mainFiles, docketRecords map[string]string) *gitRepo {
	t.Helper()
	requireRealGit(t)
	root := t.TempDir()
	r := &gitRepo{
		root:       root,
		origin:     filepath.Join(root, "origin.git"),
		writer:     filepath.Join(root, "writer"),
		invocation: filepath.Join(root, "invocation"),
	}

	runGit(t, root, "init", "--bare", "-b", "main", r.origin)
	runGit(t, root, "init", "-b", "main", r.writer)
	gitIdentity(t, r.writer)

	writeRepoFile(t, r.writer, ".docket.yml", "metadata_branch: docket\nintegration_branch: main\n")
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

// TestGitStatusReaderDiscoversFromNestedSubdir proves discovery canonicalizes a
// nested invocation directory to the same repository, so a pin succeeds from
// anywhere inside the worktree.
func TestGitStatusReaderDiscoversFromNestedSubdir(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	})
	nested := filepath.Join(repo.invocation, "docs", "changes", "active")

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), nested)
	if err != nil {
		t.Fatalf("PinContext from nested dir: %v", err)
	}
	if pin.DefaultBranch != "main" {
		t.Errorf("default branch = %q, want main", pin.DefaultBranch)
	}
	if pin.Mode != "main" {
		t.Errorf("mode = %q, want main", pin.Mode)
	}
}

// TestGitStatusReaderMainModePinAndCorpus is the main-mode end-to-end read: the
// pin resolves the default branch and both revisions collapse to it, and the
// corpus carries the active change with its blob id from the pinned revision.
func TestGitStatusReaderMainModePinAndCorpus(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md":             changeRecord(1, "alpha", "Alpha"),
		"docs/changes/active/0002-beta.md":              changeRecord(2, "beta", "Beta"),
		"docs/changes/archive/2026-01-01-0003-gamma.md": changeRecord(3, "gamma", "Gamma"),
		"docs/adrs/0001-first.md":                       "---\nid: 1\nslug: first\ntitle: First\nstatus: Accepted\ndate: 2026-01-02\n---\n\nContext.\n",
		"docs/changes/learnings/some-lesson.md":         "---\nslug: some-lesson\ntitle: Some lesson\n---\n\nA lesson.\n",
	})

	client := newGitClient(t)
	reader := NewGitStatusReader(client)
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.Mode != "main" {
		t.Fatalf("mode = %q, want main", pin.Mode)
	}
	if pin.DefaultRevision == "" || pin.IntegrationRevision != pin.DefaultRevision {
		t.Errorf("main mode should collapse revisions: default=%q integration=%q", pin.DefaultRevision, pin.IntegrationRevision)
	}
	if pin.MetadataBranch != "" || pin.MetadataRevision != "" {
		t.Errorf("main mode carried a metadata branch: %+v", pin)
	}

	blobs, err := reader.ReadCorpus(context.Background(), pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	byPath := map[string]StatusBlob{}
	for _, b := range blobs {
		byPath[b.Path] = b
	}
	got, ok := byPath["docs/changes/active/0001-alpha.md"]
	if !ok {
		t.Fatalf("corpus missing the active change; got paths %v", pathsOf(blobs))
	}
	wantID := repo.blobID(t, repo.invocation, pin.DefaultRevision, "docs/changes/active/0001-alpha.md")
	if got.Version != wantID {
		t.Errorf("blob version = %q, want the pinned revision's blob id %q", got.Version, wantID)
	}
	if string(got.Data) != changeRecord(1, "alpha", "Alpha") {
		t.Errorf("blob bytes did not match the record content:\n%s", got.Data)
	}
	// Every configured record kind is present.
	for _, want := range []string{
		"docs/changes/active/0002-beta.md",
		"docs/changes/archive/2026-01-01-0003-gamma.md",
		"docs/adrs/0001-first.md",
		"docs/changes/learnings/some-lesson.md",
	} {
		if _, ok := byPath[want]; !ok {
			t.Errorf("corpus missing %q; got %v", want, pathsOf(blobs))
		}
	}
}

// TestGitStatusReaderDocketModeDistinctRevisions proves docket mode pins the
// metadata branch separately from the integration branch and reads the corpus
// from the metadata revision, not the code branch.
func TestGitStatusReaderDocketModeDistinctRevisions(t *testing.T) {
	requireRealGit(t)
	repo := newDocketModeRepo(t,
		map[string]string{
			// A decoy record on main that must never appear in the corpus.
			"docs/changes/active/0009-decoy.md": changeRecord(9, "decoy", "Decoy"),
		},
		map[string]string{
			"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
		})

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if pin.Mode != "docket" {
		t.Fatalf("mode = %q, want docket", pin.Mode)
	}
	if pin.MetadataBranch != "docket" {
		t.Errorf("metadata branch = %q, want docket", pin.MetadataBranch)
	}
	if pin.MetadataRevision == "" || pin.IntegrationRevision == "" {
		t.Fatalf("revisions unset: %+v", pin)
	}
	if pin.MetadataRevision == pin.IntegrationRevision {
		t.Errorf("metadata and integration revisions must differ (orphan branches): both %q", pin.MetadataRevision)
	}

	blobs, err := reader.ReadCorpus(context.Background(), pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	paths := pathsOf(blobs)
	if !contains(paths, "docs/changes/active/0001-alpha.md") {
		t.Errorf("corpus missing the metadata-branch record; got %v", paths)
	}
	if contains(paths, "docs/changes/active/0009-decoy.md") {
		t.Errorf("corpus leaked a record from the integration branch; got %v", paths)
	}
}

// TestGitStatusReaderMissingMetadataBranchIsExternal proves a metadata branch
// declared in configuration but absent from the remote fails as an external
// error, not a silent empty pin.
func TestGitStatusReaderMissingMetadataBranchIsExternal(t *testing.T) {
	requireRealGit(t)
	// A main-mode topology whose .docket.yml is overwritten to demand a docket
	// branch that was never pushed.
	repo := newMainModeRepo(t, nil)
	repo.writerAdvance(t, "main", map[string]string{
		".docket.yml": "metadata_branch: docket\nintegration_branch: main\n",
	})

	reader := NewGitStatusReader(newGitClient(t))
	_, err := reader.PinContext(context.Background(), repo.invocation)
	if err == nil {
		t.Fatal("PinContext succeeded despite a missing metadata branch")
	}
	if !isStatusExternal(err) {
		t.Errorf("error = %v, want an ErrStatusExternal classification", err)
	}
}

// TestGitStatusReaderBranchFacts proves a pushed feature branch reads present
// and an absent one reads absent, with no error for the absent case.
func TestGitStatusReaderBranchFacts(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	})
	repo.writerAdvance(t, "feat-present", map[string]string{"feature.txt": "x\n"})

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	facts, err := reader.BranchFacts(context.Background(), pin, []string{"feat-present", "feat-absent"})
	if err != nil {
		t.Fatalf("BranchFacts: %v", err)
	}
	if !facts.HasBranch("feat-present") {
		t.Errorf("feat-present should be present on the remote")
	}
	if facts.HasBranch("feat-absent") {
		t.Errorf("feat-absent should be absent on the remote")
	}
}

// TestGitStatusReaderReadOnly witnesses both halves of the read-only contract:
// the worktree, index, HEAD, and symbolic ref are byte-identical before and
// after a full read, while the one permitted mutation — a remote-tracking ref
// advancing to a newly-pushed commit — is positively observed.
func TestGitStatusReaderReadOnly(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	})

	beforeFiles := worktreeChecksum(t, repo.invocation)
	beforeHead := runGit(t, repo.invocation, "rev-parse", "HEAD")
	beforeSymref := runGit(t, repo.invocation, "symbolic-ref", "HEAD")
	beforeStatus := runGit(t, repo.invocation, "status", "--porcelain")
	beforeTracking := runGit(t, repo.invocation, "rev-parse", "refs/remotes/origin/main")

	// Advance the remote so the permitted mutation has something to witness.
	newHead := repo.writerAdvance(t, "main", map[string]string{"docs/changes/active/0002-beta.md": changeRecord(2, "beta", "Beta")})

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	if _, err := reader.ReadCorpus(context.Background(), pin); err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	if _, err := reader.BranchFacts(context.Background(), pin, []string{"feat-absent"}); err != nil {
		t.Fatalf("BranchFacts: %v", err)
	}

	// Read-only over the worktree, index, HEAD, and checked-out branch.
	afterFiles := worktreeChecksum(t, repo.invocation)
	if !equalChecksums(beforeFiles, afterFiles) {
		t.Errorf("worktree files changed:\nbefore=%v\nafter=%v", beforeFiles, afterFiles)
	}
	if got := runGit(t, repo.invocation, "rev-parse", "HEAD"); got != beforeHead {
		t.Errorf("HEAD moved: %q -> %q", beforeHead, got)
	}
	if got := runGit(t, repo.invocation, "symbolic-ref", "HEAD"); got != beforeSymref {
		t.Errorf("symbolic ref moved: %q -> %q", beforeSymref, got)
	}
	if got := runGit(t, repo.invocation, "status", "--porcelain"); got != beforeStatus {
		t.Errorf("working tree status changed: %q -> %q", beforeStatus, got)
	}

	// The permitted mutation, positively witnessed: the tracking ref advanced to
	// the newly-pushed commit.
	afterTracking := runGit(t, repo.invocation, "rev-parse", "refs/remotes/origin/main")
	if afterTracking == beforeTracking {
		t.Errorf("remote-tracking ref did not move despite an advanced remote (still %q)", afterTracking)
	}
	if afterTracking != newHead {
		t.Errorf("remote-tracking ref = %q, want the newly-pushed commit %q", afterTracking, newHead)
	}
	if pin.DefaultRevision != newHead {
		t.Errorf("pinned default revision = %q, want the freshly fetched %q", pin.DefaultRevision, newHead)
	}
}

// TestGitStatusReaderConcurrentRemoteMovement proves a corpus read observes the
// exact pinned revision even when the remote advances after the pin: the source
// is fixed at open time and never re-fetches.
func TestGitStatusReaderConcurrentRemoteMovement(t *testing.T) {
	requireRealGit(t)
	repo := newMainModeRepo(t, map[string]string{
		"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha"),
	})

	reader := NewGitStatusReader(newGitClient(t))
	pin, err := reader.PinContext(context.Background(), repo.invocation)
	if err != nil {
		t.Fatalf("PinContext: %v", err)
	}
	pinnedRev := pin.DefaultRevision
	wantID := repo.blobID(t, repo.invocation, pinnedRev, "docs/changes/active/0001-alpha.md")

	// The remote advances the SAME record to different content after the pin.
	repo.writerAdvance(t, "main", map[string]string{"docs/changes/active/0001-alpha.md": changeRecord(1, "alpha", "Alpha REWRITTEN")})

	blobs, err := reader.ReadCorpus(context.Background(), pin)
	if err != nil {
		t.Fatalf("ReadCorpus: %v", err)
	}
	byPath := map[string]StatusBlob{}
	for _, b := range blobs {
		byPath[b.Path] = b
	}
	got := byPath["docs/changes/active/0001-alpha.md"]
	if got.Version != wantID {
		t.Errorf("corpus read the advanced revision: version=%q want pinned %q", got.Version, wantID)
	}
	if strings.Contains(string(got.Data), "REWRITTEN") {
		t.Errorf("corpus content came from the advanced remote, not the pinned revision:\n%s", got.Data)
	}
}

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
