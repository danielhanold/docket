//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/testsupport"
)

// This file is the private-visibility slice of the real-Git init/setup shard
// (prefix TestIntegrationRepoSetup). A private repository keeps the metadata
// branch `dckt` on a bare `dckt` remote under the data home, its checkout under
// that store, and its config in .git/dckt/config.yml — nothing docket-named in
// the repository and nothing pushed to origin.

// newPrivateInitRepo builds the newInitRepo topology but commits NO .docket.yml
// (a private repository never reads one), points XDG_DATA_HOME at a fresh temp
// dir, and returns that data home.
func newPrivateInitRepo(t *testing.T, files map[string]string) (*initRepo, string) {
	t.Helper()
	requireRealGit(t)
	root := testsupport.TempDir(t)
	r := &initRepo{
		root:       root,
		origin:     filepath.Join(root, "origin.git"),
		writer:     filepath.Join(root, "writer"),
		invocation: filepath.Join(root, "invocation"),
	}
	runGit(t, root, "init", "--bare", "-b", "main", r.origin)
	runGit(t, root, "init", "-b", "main", r.writer)
	gitIdentity(t, r.writer)
	writeRepoFile(t, r.writer, "README.md", "readme\n")
	for rel, content := range files {
		writeRepoFile(t, r.writer, rel, content)
	}
	runGit(t, r.writer, "add", "-A")
	runGit(t, r.writer, "commit", "-q", "-m", "integration content")
	runGit(t, r.writer, "remote", "add", "origin", r.origin)
	runGit(t, r.writer, "push", "-q", "-u", "origin", "main")

	runGit(t, root, "clone", "-q", r.origin, r.invocation)
	gitIdentity(t, r.invocation)

	data := testsupport.TempDir(t)
	t.Setenv("XDG_DATA_HOME", data)
	return r, data
}

// inheritedXDGDataHome is XDG_DATA_HOME as the test process inherited it,
// read at package initialization, before any test pins it.
var inheritedXDGDataHome = os.Getenv("XDG_DATA_HOME")

// pinInitUserRoots keeps an init run off the real user's machine roots, since
// init reaches the installer (installAuthorizedSurfaces resolves its roots from
// the environment). HOME always moves to a fresh temp dir; XDG_CONFIG_HOME is
// pinned by newGitClient (or runInitWithGlobal); XDG_DATA_HOME moves to a fresh
// temp dir unless the test already pinned it, because a private store the test
// inspects (newPrivateInitRepo's data home) lives there.
func pinInitUserRoots(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", testsupport.TempDir(t))
	if v := os.Getenv("XDG_DATA_HOME"); v == "" || v == inheritedXDGDataHome {
		t.Setenv("XDG_DATA_HOME", testsupport.TempDir(t))
	}
}

// runInitWith runs RunRepositoryInit against the invocation clone with o.
func (r *initRepo) runInitWith(t *testing.T, o InitOptions) RepositoryOpResult {
	t.Helper()
	pinInitUserRoots(t)
	client := newGitClient(t)
	return RunRepositoryInit(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
}

// runInitWithGlobal runs init with a global config layer holding globalYML.
// newGitClient isolates XDG_CONFIG_HOME, so the global layer is installed after
// the client is built.
func (r *initRepo) runInitWithGlobal(t *testing.T, globalYML string, o InitOptions) RepositoryOpResult {
	t.Helper()
	pinInitUserRoots(t)
	client := newGitClient(t)
	cfgHome := testsupport.TempDir(t)
	if err := os.MkdirAll(filepath.Join(cfgHome, "docket"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgHome, "docket", "config.yml"), []byte(globalYML), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	return RunRepositoryInit(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
}

// privateLayoutOf resolves the clone's private layout from its origin URL and
// the test's data home, independently of the code under test's own resolution.
func privateLayoutOf(t *testing.T, dir, data string) layout.Layout {
	t.Helper()
	common := runGit(t, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	top := runGit(t, dir, "rev-parse", "--show-toplevel")
	slug, err := layout.OwnerRepo(runGit(t, dir, "config", "--get", "remote.origin.url"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(data)
	if err != nil {
		t.Fatal(err)
	}
	return layout.PrivateLayout(common, top, canonical, slug)
}

func TestIntegrationRepoSetupPrivateFreshInitCreatesNeutralLayout(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	res := r.runInitWith(t, InitOptions{Private: true})
	if res.Result != ResultApplied {
		t.Fatalf("Result = %q (%s), want applied", res.Result, res.HumanText())
	}
	lay := privateLayoutOf(t, r.invocation, data)
	gitDir := r.gitDir(t)

	cfg, err := os.ReadFile(filepath.Join(gitDir, "dckt", "config.yml"))
	if err != nil || !strings.Contains(string(cfg), "visibility: private") {
		t.Errorf(".git/dckt/config.yml = %q (%v), want it to hold visibility: private", cfg, err)
	}
	exclude, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	if err != nil || !reposetup.ValidExcludeBlock(exclude) {
		t.Errorf(".git/info/exclude = %q (%v), want a valid # dckt: block", exclude, err)
	}

	url := runGit(t, r.invocation, "config", "--get", "remote.dckt.url")
	if url != lay.DefaultBareRemote || !strings.HasSuffix(url, filepath.Join("dckt", filepath.Base(lay.StoreDir), "remote.git")) {
		t.Errorf("dckt remote URL = %q, want %q", url, lay.DefaultBareRemote)
	}
	if _, err := tryGit(url, "rev-parse", "--verify", "--quiet", "refs/heads/dckt"); err != nil {
		t.Errorf("the bare store has no refs/heads/dckt: %v", err)
	}
	if r.remoteBranchExists(t, "docket") || r.remoteBranchExists(t, "dckt") {
		t.Error("origin gained a docket or dckt branch; private metadata must never reach origin")
	}
	wts := runGit(t, r.invocation, "worktree", "list", "--porcelain")
	if !strings.Contains(wts, "worktree "+lay.MetadataWorktree+"\n") || !strings.HasPrefix(lay.MetadataWorktree, filepath.Dir(lay.StoreDir)) {
		t.Errorf("worktree list has no checkout %s under the data home:\n%s", lay.MetadataWorktree, wts)
	}
	for _, name := range []string{".docket", ".docket.yml", ".gitignore", ".docket.local.yml"} {
		if _, err := os.Lstat(filepath.Join(r.invocation, name)); !os.IsNotExist(err) {
			t.Errorf("%s exists in the repository root (err=%v); private init writes nothing there", name, err)
		}
	}
	if st := runGit(t, r.invocation, "status", "--porcelain"); st != "" {
		t.Errorf("git status --porcelain = %q, want empty", st)
	}
}

// TestIntegrationRepoSetupPrivateRerunIsNoOp proves a re-run of a finished
// private init reports no-op whatever the clock does between the two runs. The
// commit dates are pinned so each case is deterministic: with the same dates the
// re-run would rebuild a byte-identical init root (two runs inside one second),
// and with a later date it would build a different one. Either way the re-run
// adopts the published branch instead of republishing it.
func TestIntegrationRepoSetupPrivateRerunIsNoOp(t *testing.T) {
	for _, tc := range []struct {
		name         string
		first, rerun string
	}{
		{name: "identical root within one second", first: "1700000000 +0000", rerun: "1700000000 +0000"},
		{name: "different root a second later", first: "1700000000 +0000", rerun: "1700000001 +0000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, data := newPrivateInitRepo(t, nil)
			pinCommitDate(t, tc.first)
			if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
				t.Fatalf("first init = %q (%s), want applied", res.Result, res.HumanText())
			}
			lay := privateLayoutOf(t, r.invocation, data)
			tip := runGit(t, lay.DefaultBareRemote, "rev-parse", "refs/heads/dckt")
			pinCommitDate(t, tc.rerun)
			res := r.runInitWith(t, InitOptions{})
			if res.Result != ResultNoOp {
				t.Fatalf("re-run = %q (%s), want no-op", res.Result, res.HumanText())
			}
			if res.MetadataTip != tip {
				t.Errorf("re-run MetadataTip = %q, want the published tip %q", res.MetadataTip, tip)
			}
			if got := runGit(t, lay.DefaultBareRemote, "rev-parse", "refs/heads/dckt"); got != tip {
				t.Errorf("the store's dckt branch moved from %s to %s on re-run", tip, got)
			}
			prep := RunRepositoryPrepare(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: r.invocation}, PrepareOptions{})
			if prep.Disposition != PrepareDispositionNoOp {
				t.Fatalf("prepare disposition = %q (%s), want no-op", prep.Disposition, prep.HumanText())
			}
			if prep.Context == nil || prep.Context.MetadataRemote != "dckt" || prep.Context.MetadataWorktreePath != lay.MetadataWorktree {
				t.Errorf("prepare context = %+v, want MetadataRemote dckt and MetadataWorktreePath %q", prep.Context, lay.MetadataWorktree)
			}
		})
	}
}

// pinCommitDate fixes the author and committer dates of every commit the next
// git client builds; gitcli passes both variables through its sanitized
// environment, and newGitClient snapshots the environment per run.
func pinCommitDate(t *testing.T, date string) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_DATE", date)
	t.Setenv("GIT_COMMITTER_DATE", date)
}

// TestIntegrationRepoSetupPrivateInterruptedAfterConfigResumes proves the
// presence-encoded-state recovery: a run that died right after writing
// .git/dckt/ (so the repository reads as private) but before registering the
// dckt remote finishes on a plain re-run.
func TestIntegrationRepoSetupPrivateInterruptedAfterConfigResumes(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	gitDir := r.gitDir(t)
	if err := os.MkdirAll(filepath.Join(gitDir, "dckt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "dckt", "config.yml"), []byte("visibility: private\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := r.runInitWith(t, InitOptions{})
	if res.Result != ResultApplied {
		t.Fatalf("resumed init = %q (%s), want applied", res.Result, res.HumanText())
	}
	lay := privateLayoutOf(t, r.invocation, data)
	if url := runGit(t, r.invocation, "config", "--get", "remote.dckt.url"); url != lay.DefaultBareRemote {
		t.Errorf("dckt remote URL = %q, want %q", url, lay.DefaultBareRemote)
	}
}

func TestIntegrationRepoSetupPrivateSecondCloneAdoptsSharedStore(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("clone A init = %q (%s), want applied", res.Result, res.HumanText())
	}
	node := planningDepsFor(t, r.invocation)
	created := ChangeCreate(context.Background(), node.deps, r.invocation, validChangeCreateRequest())
	if created.Result != ResultApplied {
		t.Fatalf("ChangeCreate in clone A = %q (%s)", created.Result, created.HumanText())
	}
	layA := privateLayoutOf(t, r.invocation, data)
	headA := runGit(t, layA.MetadataWorktree, "rev-parse", "HEAD")

	b := r.freshClone(t)
	bRepo := &initRepo{root: r.root, origin: r.origin, writer: r.writer, invocation: b}
	if res := bRepo.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("clone B init = %q (%s), want applied", res.Result, res.HumanText())
	}
	layB := privateLayoutOf(t, b, data)

	urlA := runGit(t, r.invocation, "config", "--get", "remote.dckt.url")
	urlB := runGit(t, b, "config", "--get", "remote.dckt.url")
	if urlA != urlB {
		t.Errorf("dckt URLs differ: A %q, B %q; both clones must share one store", urlA, urlB)
	}
	if layA.MetadataWorktree == layB.MetadataWorktree {
		t.Errorf("both clones resolved the same checkout %q", layA.MetadataWorktree)
	}
	if _, err := os.Stat(filepath.Join(layB.MetadataWorktree, created.Path)); err != nil {
		t.Errorf("clone B's checkout lacks A's change file %s: %v", created.Path, err)
	}
	if got := runGit(t, layA.MetadataWorktree, "rev-parse", "HEAD"); got != headA {
		t.Errorf("clone A's checkout HEAD moved from %s to %s", headA, got)
	}
}

// TestIntegrationRepoSetupPrivateStoreOriginCollisionRefuses proves the default
// store records the origin that created it and a repository whose distinct
// origin derives the same <owner>-<repo> store name refuses instead of adopting
// the other repository's backlog: no dckt remote, no checkout, and the store's
// branch and record stay untouched.
func TestIntegrationRepoSetupPrivateStoreOriginCollisionRefuses(t *testing.T) {
	a, data := newPrivateInitRepo(t, nil)
	if res := a.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("repo A init = %q (%s), want applied", res.Result, res.HumanText())
	}
	layA := privateLayoutOf(t, a.invocation, data)
	recordPath := filepath.Join(layA.StoreDir, storeOriginRecordName)
	record, err := os.ReadFile(recordPath)
	if err != nil || strings.TrimSpace(string(record)) != a.origin {
		t.Fatalf("store origin record = %q (%v), want A's origin %q", record, err, a.origin)
	}
	tipA := runGit(t, layA.DefaultBareRemote, "rev-parse", "refs/heads/dckt")

	// Repo B: a different origin whose last two path segments equal A's, so it
	// derives the same store name — the same owner/repo on another host.
	b, _ := newPrivateInitRepo(t, nil)
	t.Setenv("XDG_DATA_HOME", data)
	collider := filepath.Join(testsupport.TempDir(t), filepath.Base(a.root), "origin.git")
	runGit(t, b.root, "clone", "-q", "--bare", b.origin, collider)
	runGit(t, b.invocation, "remote", "set-url", "origin", collider)
	if layB := privateLayoutOf(t, b.invocation, data); layB.StoreDir != layA.StoreDir {
		t.Fatalf("fixture: B's store %q does not collide with A's %q", layB.StoreDir, layA.StoreDir)
	}

	res := b.runInitWith(t, InitOptions{Private: true})
	if res.Result != ResultInvalidState || res.RepositoryState != string(reposetup.StateConflict) {
		t.Fatalf("colliding init = %q/%q (%s), want invalid-state/conflict", res.Result, res.RepositoryState, res.HumanText())
	}
	text := res.HumanText()
	if !strings.Contains(text, a.origin) || !strings.Contains(text, collider) || !strings.Contains(text, "--metadata-remote") {
		t.Errorf("refusal %q must name both origins and the --metadata-remote remedy", text)
	}
	if _, err := tryGit(b.invocation, "config", "--get", "remote.dckt.url"); err == nil {
		t.Error("the colliding repository gained a dckt remote; the refusal must write nothing further")
	}
	if wts := runGit(t, b.invocation, "worktree", "list", "--porcelain"); strings.Contains(wts, layA.CheckoutsDir) {
		t.Errorf("the colliding repository attached a checkout under the shared store:\n%s", wts)
	}
	if got := runGit(t, layA.DefaultBareRemote, "rev-parse", "refs/heads/dckt"); got != tipA {
		t.Errorf("the store's dckt branch moved from %s to %s", tipA, got)
	}
	if got, _ := os.ReadFile(recordPath); string(got) != string(record) {
		t.Errorf("store origin record changed from %q to %q", record, got)
	}
}

// TestIntegrationRepoSetupPrivateUnrecordedStoreAdoptsAndRecords proves a store
// created before the origin record existed is adopted by a matching repository
// and gains the record.
func TestIntegrationRepoSetupPrivateUnrecordedStoreAdoptsAndRecords(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("clone A init = %q (%s), want applied", res.Result, res.HumanText())
	}
	lay := privateLayoutOf(t, r.invocation, data)
	recordPath := filepath.Join(lay.StoreDir, storeOriginRecordName)
	if err := os.Remove(recordPath); err != nil {
		t.Fatal(err)
	}
	b := r.freshClone(t)
	bRepo := &initRepo{root: r.root, origin: r.origin, writer: r.writer, invocation: b}
	if res := bRepo.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("clone B init over an unrecorded store = %q (%s), want applied", res.Result, res.HumanText())
	}
	if got, err := os.ReadFile(recordPath); err != nil || strings.TrimSpace(string(got)) != r.origin {
		t.Errorf("store origin record = %q (%v), want it rewritten to %q", got, err, r.origin)
	}
}

func TestIntegrationRepoSetupPrivateRemoteURLConflictRefuses(t *testing.T) {
	r, _ := newPrivateInitRepo(t, nil)
	elsewhere := filepath.Join(r.root, "elsewhere.git")
	runGit(t, r.invocation, "remote", "add", "dckt", elsewhere)
	res := r.runInitWith(t, InitOptions{Private: true})
	if res.Result == ResultApplied || res.Result == ResultNoOp {
		t.Fatalf("init = %q (%s), want a refusal", res.Result, res.HumanText())
	}
	text := res.HumanText()
	if !strings.Contains(text, elsewhere) || !strings.Contains(text, "remote.git") {
		t.Errorf("refusal %q must name both the existing URL %q and the wanted store", text, elsewhere)
	}
	if !strings.Contains(text, "re-run `docket repository init`") {
		t.Errorf("refusal %q must name the presence-encoded recovery", text)
	}
	if url := runGit(t, r.invocation, "config", "--get", "remote.dckt.url"); url != elsewhere {
		t.Errorf("dckt remote rewritten to %q; docket never rewrites a remote", url)
	}
}

// TestIntegrationRepoSetupPrivateForeignStoreBranchRefuses proves the adopt
// path a private init takes for a dckt branch the store already holds verifies
// the branch first: a foreign branch refuses and stays byte-untouched, never
// adopted and never republished.
func TestIntegrationRepoSetupPrivateForeignStoreBranchRefuses(t *testing.T) {
	r, _ := newPrivateInitRepo(t, nil)
	store := filepath.Join(testsupport.TempDir(t), "store.git")
	runGit(t, r.root, "init", "--bare", "-q", store)
	runGit(t, r.writer, "push", "-q", store, "main:refs/heads/dckt")
	foreign := runGit(t, store, "rev-parse", "refs/heads/dckt")

	res := r.runInitWith(t, InitOptions{Private: true, MetadataRemote: store})
	if res.Result != ResultInvalidState {
		t.Fatalf("init over a foreign store branch = %q (%s), want invalid-state", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateConflict) {
		t.Errorf("RepositoryState = %q, want conflict", res.RepositoryState)
	}
	if got := runGit(t, store, "rev-parse", "refs/heads/dckt"); got != foreign {
		t.Errorf("the store's foreign dckt branch moved from %s to %s", foreign, got)
	}
}

func TestIntegrationRepoSetupPrivateMetadataRemoteFlag(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	backup := filepath.Join(testsupport.TempDir(t), "backup.git")
	runGit(t, r.root, "init", "--bare", "-q", backup)
	res := r.runInitWith(t, InitOptions{Private: true, MetadataRemote: backup})
	if res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	if url := runGit(t, r.invocation, "config", "--get", "remote.dckt.url"); url != backup {
		t.Errorf("dckt remote URL = %q, want the flag %q", url, backup)
	}
	if _, err := tryGit(backup, "rev-parse", "--verify", "--quiet", "refs/heads/dckt"); err != nil {
		t.Errorf("the flag remote has no refs/heads/dckt: %v", err)
	}
	lay := privateLayoutOf(t, r.invocation, data)
	if _, err := os.Stat(lay.DefaultBareRemote); !os.IsNotExist(err) {
		t.Errorf("default bare remote %s was created (err=%v); a flag URL never creates it", lay.DefaultBareRemote, err)
	}
}

// TestIntegrationRepoSetupPrivateMetadataRemoteRerunIsNoOp proves a flagless
// re-run on a repository set up with --metadata-remote keeps the configured
// dckt remote: it reports no-op, never creates the default store, and never
// refuses a healthy repository with a false remote conflict.
func TestIntegrationRepoSetupPrivateMetadataRemoteRerunIsNoOp(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	backup := filepath.Join(testsupport.TempDir(t), "backup.git")
	runGit(t, r.root, "init", "--bare", "-q", backup)
	if res := r.runInitWith(t, InitOptions{Private: true, MetadataRemote: backup}); res.Result != ResultApplied {
		t.Fatalf("first init = %q (%s), want applied", res.Result, res.HumanText())
	}
	tip := runGit(t, backup, "rev-parse", "refs/heads/dckt")

	res := r.runInitWith(t, InitOptions{})
	if res.Result != ResultNoOp {
		t.Fatalf("flagless re-run = %q (%s), want no-op", res.Result, res.HumanText())
	}
	if res.RepositoryState != string(reposetup.StateHealthy) {
		t.Errorf("RepositoryState = %q, want healthy", res.RepositoryState)
	}
	if res.MetadataTip != tip {
		t.Errorf("re-run MetadataTip = %q, want the published tip %q", res.MetadataTip, tip)
	}
	if url := runGit(t, r.invocation, "config", "--get", "remote.dckt.url"); url != backup {
		t.Errorf("dckt remote URL = %q, want the flag %q", url, backup)
	}
	lay := privateLayoutOf(t, r.invocation, data)
	if _, err := os.Stat(lay.DefaultBareRemote); !os.IsNotExist(err) {
		t.Errorf("default bare remote %s was created on re-run (err=%v)", lay.DefaultBareRemote, err)
	}
}

func TestIntegrationRepoSetupInitModeFlagsRefuseSwitch(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	if res := r.runInit(t); res.Result != ResultApplied {
		t.Fatalf("shared init = %q (%s), want applied", res.Result, res.HumanText())
	}
	res := r.runInitWith(t, InitOptions{Private: true})
	if res.Result != ResultInvalidState {
		t.Fatalf("--private on a shared repository = %q (%s), want invalid-state", res.Result, res.HumanText())
	}
	if _, err := os.Lstat(filepath.Join(r.gitDir(t), "dckt")); !os.IsNotExist(err) {
		t.Errorf(".git/dckt exists after a refused switch (err=%v)", err)
	}
}

func TestIntegrationRepoSetupGlobalPrivateDefault(t *testing.T) {
	const global = "visibility: private\n"

	t.Run("fresh with no flags inits private", func(t *testing.T) {
		r, _ := newPrivateInitRepo(t, nil)
		res := r.runInitWithGlobal(t, global, InitOptions{})
		if res.Result != ResultApplied {
			t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
		}
		if _, err := os.Stat(filepath.Join(r.gitDir(t), "dckt")); err != nil {
			t.Errorf("no .git/dckt after a global-private init: %v", err)
		}
		if r.remoteBranchExists(t, "docket") {
			t.Error("origin gained a docket branch")
		}
	})

	t.Run("committed visibility shared wins", func(t *testing.T) {
		r := newInitRepo(t, defaultSetupYML+"visibility: shared\n", nil)
		t.Setenv("XDG_DATA_HOME", testsupport.TempDir(t))
		res := r.runInitWithGlobal(t, global, InitOptions{})
		if res.Result != ResultApplied {
			t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
		}
		if !r.remoteBranchExists(t, "docket") {
			t.Error("committed visibility: shared did not init shared")
		}
		if _, err := os.Lstat(filepath.Join(r.gitDir(t), "dckt")); !os.IsNotExist(err) {
			t.Errorf(".git/dckt exists (err=%v)", err)
		}
	})

	t.Run("--shared wins", func(t *testing.T) {
		r := newInitRepo(t, defaultSetupYML, nil)
		t.Setenv("XDG_DATA_HOME", testsupport.TempDir(t))
		res := r.runInitWithGlobal(t, global, InitOptions{Shared: true})
		if res.Result != ResultApplied {
			t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
		}
		if !r.remoteBranchExists(t, "docket") {
			t.Error("--shared did not init shared")
		}
	})

	t.Run("healthy shared stays shared", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", testsupport.TempDir(t))
		r := newHealthyRepo(t)
		res := r.runInitWithGlobal(t, global, InitOptions{})
		if res.Result != ResultNoOp && res.Result != ResultApplied {
			t.Fatalf("re-init = %q (%s), want it admitted as shared", res.Result, res.HumanText())
		}
		if _, err := os.Lstat(filepath.Join(r.gitDir(t), "dckt")); !os.IsNotExist(err) {
			t.Errorf(".git/dckt exists after re-init (err=%v); config never moves a repository", err)
		}
		chk := r.runCheck(t)
		for _, f := range chk.Findings {
			if f.Code == "visibility-mismatch" {
				t.Errorf("check reported %+v; a global visibility never mismatches", f)
			}
		}
	})
}

// TestIntegrationRepoSetupPrivateMigrateRefuses proves migrate refuses a
// private repository before any phase logic: it converts legacy repositories
// to the shared layout, and a private one has nothing to migrate.
func TestIntegrationRepoSetupPrivateMigrateRefuses(t *testing.T) {
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	res := r.runMigrate(t, MigrateOptions{Authorized: true})
	if res.Result != ResultInvalidState || !strings.Contains(res.HumanText(), "this repository is private and has nothing to migrate") {
		t.Fatalf("migrate = %q (%s), want the private refusal", res.Result, res.HumanText())
	}
	if r.remoteBranchExists(t, "docket") {
		t.Error("migrate published a docket branch to origin")
	}
}

// initPrivateHealthy inits a private repository recording no agents
// (--harnesses none) and sets its test policy with configure-tests --command
// true, the path to a healthy private repository.
func initPrivateHealthy(t *testing.T) (*initRepo, string) {
	t.Helper()
	r, data := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true, Harnesses: harnessFlag("none")}); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	cmd := "true"
	ct := r.runConfigureTestsWith(t, ConfigureTestsOptions{Command: &cmd})
	if ct.Result != ResultApplied {
		t.Fatalf("configure-tests = %q (%s), want applied", ct.Result, ct.HumanText())
	}
	if len(ct.PendingPaths) != 0 || !strings.Contains(ct.HumanText(), "wrote "+layout.PrivateConfigDisplay) {
		t.Errorf("private configure-tests = pending %v, %q; want no pending path and the private config named", ct.PendingPaths, ct.HumanText())
	}
	return r, data
}

// checkIn runs RunRepositoryCheck from dir.
func checkIn(t *testing.T, dir string) RepositoryCheckResult {
	t.Helper()
	return RunRepositoryCheck(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: dir})
}

// findingsWithCode returns the findings carrying code.
func findingsWithCode(fs []reposetup.Finding, code string) []reposetup.Finding {
	var out []reposetup.Finding
	for _, f := range fs {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

func TestIntegrationRepoSetupPrivateCheckHealthy(t *testing.T) {
	r, _ := initPrivateHealthy(t)
	chk := r.runCheck(t)
	if chk.RepositoryState != string(reposetup.StateHealthy) {
		t.Fatalf("check state = %q, want healthy:\n%s", chk.RepositoryState, chk.HumanText())
	}
	for _, f := range chk.Findings {
		text := f.Ref + " " + f.Message + " " + f.Remedy
		for _, name := range []string{".gitignore", ".docket.yml", ".docket"} {
			if strings.Contains(text, name) {
				t.Errorf("finding %+v names %s; a private repository has none", f, name)
			}
		}
	}
	if len(chk.Findings) != 0 || chk.CheckExitCode() != 0 {
		t.Errorf("healthy private check = exit %d, findings %+v; want 0 and none", chk.CheckExitCode(), chk.Findings)
	}
}

func TestIntegrationRepoSetupPrivateCheckFlagsMissingExclude(t *testing.T) {
	r, _ := initPrivateHealthy(t)
	exclude := filepath.Join(r.gitDir(t), "info", "exclude")
	if err := os.WriteFile(exclude, []byte("# a user line\n*.swp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chk := r.runCheck(t)
	if chk.RepositoryState == string(reposetup.StateHealthy) {
		t.Fatalf("check is healthy with the # dckt: block stripped:\n%s", chk.HumanText())
	}
	got := findingsWithCode(chk.Findings, "committed-ignore-invalid")
	if len(got) != 1 {
		t.Fatalf("committed-ignore findings = %+v, want exactly one:\n%s", got, chk.HumanText())
	}
	if got[0].Ref != ".git/info/exclude" || !strings.Contains(got[0].Message, ".git/info/exclude") ||
		!strings.Contains(got[0].Remedy, "docket repository init") {
		t.Errorf("finding = %+v, want it to name .git/info/exclude and the init remedy", got[0])
	}
}

func TestIntegrationRepoSetupPrivateVisibilityMismatch(t *testing.T) {
	r, _ := initPrivateHealthy(t)
	before := r.runCheck(t)
	cfgPath := filepath.Join(r.gitDir(t), "dckt", "config.yml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "visibility: private\n") {
		t.Fatalf("private config %q lacks visibility: private", raw)
	}
	edited := strings.Replace(string(raw), "visibility: private\n", "visibility: shared\n", 1)
	if err := os.WriteFile(cfgPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	chk := r.runCheck(t)
	got := findingsWithCode(chk.Findings, FindingVisibilityMismatch)
	if len(got) != 1 || got[0].Ref != layout.PrivateConfigDisplay || !strings.Contains(got[0].Message, layout.PrivateConfigDisplay) {
		t.Fatalf("mismatch findings = %+v, want one naming %s:\n%s", got, layout.PrivateConfigDisplay, chk.HumanText())
	}
	if chk.RepositoryState != before.RepositoryState {
		t.Errorf("state moved from %q to %q; the mismatch never changes the state", before.RepositoryState, chk.RepositoryState)
	}
	if _, err := os.Stat(filepath.Join(r.gitDir(t), "dckt")); err != nil {
		t.Errorf(".git/dckt is gone (%v); config never moves a repository", err)
	}
}

func TestIntegrationRepoSetupPrivateBothLocalConfigsRefuse(t *testing.T) {
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	local := filepath.Join(r.invocation, ".docket.local.yml")
	if err := os.WriteFile(local, []byte("board_presentation: classic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(r.gitDir(t), "dckt", "config.yml")

	chk := r.runCheck(t)
	if chk.Result != ResultUnsupportedConfig {
		t.Errorf("check result = %q (%s), want unsupported-config", chk.Result, chk.HumanText())
	}
	if text := chk.HumanText(); !strings.Contains(text, local) || !strings.Contains(text, privatePath) {
		t.Errorf("check text %q must name both %s and %s", text, local, privatePath)
	}

	st := Status(context.Background(), NewGitStatusReader(newGitClient(t)), StatusOptions{RepoDir: r.invocation})
	if st.Result != ResultInvalidInput {
		t.Errorf("status result = %q (%s), want invalid-input", st.Result, st.Message)
	}
	if !strings.Contains(st.Message, local) || !strings.Contains(st.Message, privatePath) {
		t.Errorf("status message %q must name both %s and %s", st.Message, local, privatePath)
	}
}

// TestIntegrationRepoSetupPrivateInitRefusesBesideLocalConfig proves a private
// init over a repository that already carries a .docket.local.yml refuses before
// any write, with the same both-paths unsupported-config refusal every private
// operation gives: writing .git/dckt/ first would turn the repository private
// and leave every later operation, the recommended init re-run included,
// refusing.
func TestIntegrationRepoSetupPrivateInitRefusesBesideLocalConfig(t *testing.T) {
	// The local file is ignored, as a shared repository's usually is, so the
	// primary worktree reads clean.
	r, data := newPrivateInitRepo(t, map[string]string{".gitignore": ".docket.local.yml\n"})
	local := filepath.Join(r.invocation, ".docket.local.yml")
	if err := os.WriteFile(local, []byte("learnings: {cap: 250}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitDir := r.gitDir(t)
	excludePath := filepath.Join(gitDir, "info", "exclude")
	excludeBefore, _ := os.ReadFile(excludePath)

	res := r.runInitWith(t, InitOptions{Private: true})
	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("init = %q (%s), want unsupported-config", res.Result, res.HumanText())
	}
	top, err := filepath.EvalSymlinks(r.invocation)
	if err != nil {
		t.Fatal(err)
	}
	wantLocal, wantPrivate := filepath.Join(top, ".docket.local.yml"), filepath.Join(top, ".git", "dckt", "config.yml")
	if text := res.HumanText(); !strings.Contains(text, wantLocal) || !strings.Contains(text, wantPrivate) {
		t.Errorf("refusal %q must name both %s and %s", text, wantLocal, wantPrivate)
	}
	if strings.Contains(res.HumanText(), privateInitRecovery) {
		t.Errorf("refusal %q carries the post-write recovery text; nothing was written", res.HumanText())
	}
	if _, err := os.Lstat(filepath.Join(gitDir, "dckt")); !os.IsNotExist(err) {
		t.Errorf(".git/dckt exists after the refusal (err=%v); init must write nothing", err)
	}
	if _, err := tryGit(r.invocation, "config", "--get", "remote.dckt.url"); err == nil {
		t.Error("a dckt remote was added; init must write nothing")
	}
	lay := privateLayoutOf(t, r.invocation, data)
	if _, err := os.Lstat(lay.StoreDir); !os.IsNotExist(err) {
		t.Errorf("store %s exists after the refusal (err=%v); init must write nothing", lay.StoreDir, err)
	}
	if excludeAfter, _ := os.ReadFile(excludePath); string(excludeAfter) != string(excludeBefore) {
		t.Errorf(".git/info/exclude changed to %q; init must write nothing", excludeAfter)
	}
	if r.remoteBranchExists(t, "docket") || r.remoteBranchExists(t, "dckt") {
		t.Error("origin gained a docket or dckt branch")
	}
}

// TestIntegrationRepoSetupPrivateRecloneSamePathReattaches deletes a private
// clone and re-clones it at the same path. The store survives and the clone id
// is the same, so the old checkout still sits at the new clone's checkout path,
// its .git file naming a gitdir the new clone no longer has. Init must treat
// that stale checkout as replaceable and attach a fresh one; prepare must do
// the same when a checkout loses its registration.
func TestIntegrationRepoSetupPrivateRecloneSamePathReattaches(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("first init = %q (%s), want applied", res.Result, res.HumanText())
	}
	checkout := privateLayoutOf(t, r.invocation, data).MetadataWorktree
	if err := os.RemoveAll(r.invocation); err != nil {
		t.Fatal(err)
	}
	runGit(t, r.root, "clone", "-q", r.origin, r.invocation)
	gitIdentity(t, r.invocation)
	if got := privateLayoutOf(t, r.invocation, data).MetadataWorktree; got != checkout {
		t.Fatalf("re-clone resolved checkout %q, want the same path %q", got, checkout)
	}
	if _, err := os.Stat(checkout); err != nil {
		t.Fatalf("the old checkout should survive the clone deletion: %v", err)
	}

	res := r.runInitWith(t, InitOptions{Private: true})
	if res.Result != ResultApplied {
		t.Fatalf("re-clone init = %q (%s), want applied", res.Result, res.HumanText())
	}
	wts := runGit(t, r.invocation, "worktree", "list", "--porcelain")
	if !strings.Contains(wts, "worktree "+checkout+"\n") {
		t.Fatalf("re-clone init did not attach %s:\n%s", checkout, wts)
	}
	d := SetupDeps{Git: newGitClient(t), RepoDir: r.invocation}
	if prep := RunRepositoryPrepare(context.Background(), d, PrepareOptions{}); prep.Disposition != PrepareDispositionNoOp {
		t.Fatalf("prepare after re-clone init = %q (%s), want no-op", prep.Disposition, prep.HumanText())
	}

	// The checkout loses its registration (its gitdir is gone): prepare replaces
	// the stale checkout and attaches a live one.
	gitdir, ok := checkoutGitdir(checkout)
	if !ok {
		t.Fatalf("cannot read the gitdir of %s", checkout)
	}
	if err := os.RemoveAll(gitdir); err != nil {
		t.Fatal(err)
	}
	prep := RunRepositoryPrepare(context.Background(), d, PrepareOptions{})
	if prep.Disposition != PrepareDispositionApplied || prep.Context == nil || prep.Context.MetadataWorktreePath != checkout {
		t.Fatalf("prepare over a stale checkout = %q (%s), want %s attached", prep.Disposition, prep.HumanText(), checkout)
	}
	if prep = RunRepositoryPrepare(context.Background(), d, PrepareOptions{}); prep.Disposition != PrepareDispositionNoOp {
		t.Fatalf("prepare re-run = %q (%s), want no-op", prep.Disposition, prep.HumanText())
	}
	if orphans := findingsWithCode(checkIn(t, r.invocation).Findings, FindingOrphanedCheckout); len(orphans) != 0 {
		t.Errorf("check reports orphans after the re-attach: %+v", orphans)
	}
}

// TestIntegrationRepoSetupPrivateMovedCloneOrphanPruned moves a private clone:
// its old checkout under the store now points at a gitdir that no longer
// exists, so check reports it orphaned and repair removes it.
func TestIntegrationRepoSetupPrivateMovedCloneOrphanPruned(t *testing.T) {
	r, data := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	old := privateLayoutOf(t, r.invocation, data).MetadataWorktree
	moved := filepath.Join(r.root, "moved")
	if err := os.Rename(r.invocation, moved); err != nil {
		t.Fatal(err)
	}

	chk := checkIn(t, moved)
	orphans := findingsWithCode(chk.Findings, FindingOrphanedCheckout)
	if len(orphans) != 1 || orphans[0].Ref != old {
		t.Fatalf("orphaned-checkout findings = %+v, want one for %s:\n%s", orphans, old, chk.HumanText())
	}

	d := SetupDeps{Git: newGitClient(t), RepoDir: moved}
	preview := RunRepositoryRepair(context.Background(), d, RepairOptions{})
	if !preview.ConfirmationRequired() || !strings.Contains(preview.HumanText(), "remove orphaned checkout: "+old) {
		t.Fatalf("repair preview = %q (%s), want it to list the orphan", preview.Result, preview.HumanText())
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("the preview touched the orphan: %v", err)
	}
	applied := RunRepositoryRepair(context.Background(), d, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRevision})
	if applied.Result != ResultApplied {
		t.Fatalf("repair = %q (%s), want applied", applied.Result, applied.HumanText())
	}
	if _, err := os.Lstat(old); !os.IsNotExist(err) {
		t.Errorf("the orphaned checkout %s survived the repair (err=%v)", old, err)
	}
	if again := findingsWithCode(checkIn(t, moved).Findings, FindingOrphanedCheckout); len(again) != 0 {
		t.Errorf("re-check still reports orphans: %+v", again)
	}

	// Prepare does NOT attach the new checkout directly: the moved clone's worktree
	// registration still names the removed checkout, so the dckt branch is held
	// by a now-prunable registration. Prepare refuses with the held-elsewhere
	// finding and names `git worktree prune`; after the prune it attaches.
	d = SetupDeps{Git: newGitClient(t), RepoDir: moved}
	prep := RunRepositoryPrepare(context.Background(), d, PrepareOptions{})
	if prep.Disposition != PrepareDispositionRefused || !strings.Contains(prep.HumanText(), "git worktree prune") {
		t.Fatalf("prepare = %q (%s), want the held-elsewhere refusal naming git worktree prune", prep.Disposition, prep.HumanText())
	}
	runGit(t, moved, "worktree", "prune")
	newCheckout := privateLayoutOf(t, moved, data).MetadataWorktree
	prep = RunRepositoryPrepare(context.Background(), d, PrepareOptions{})
	if prep.Disposition != PrepareDispositionApplied || prep.Context == nil || prep.Context.MetadataWorktreePath != newCheckout {
		t.Fatalf("prepare after prune = %q (%s), want the new checkout %s attached", prep.Disposition, prep.HumanText(), newCheckout)
	}
}

// fatalRepairGitHub is a RepairGitHub whose every method fails the test: a
// private repository's PR-backlink repair must never reach GitHub.
type fatalRepairGitHub struct{ t *testing.T }

func (g fatalRepairGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	g.t.Fatalf("a private PR-backlink repair resolved the GitHub repository")
	return githubcli.Repository{}, nil
}

func (g fatalRepairGitHub) ViewPullRequestsBatch(context.Context, githubcli.Repository, []int) (map[int]githubcli.BatchPRResult, error) {
	g.t.Fatalf("a private PR-backlink repair read pull-request bodies")
	return nil, nil
}

func (g fatalRepairGitHub) EditPullRequestBody(context.Context, githubcli.Repository, int, string, string) (githubcli.BodyEditOutcome, githubcli.PullRequest, error) {
	g.t.Fatalf("a private PR-backlink repair edited a pull-request body")
	return githubcli.BodyUnknown, githubcli.PullRequest{}, nil
}

// TestIntegrationRepoSetupPrivatePRBacklinkRepairIsNoOp: a private repository's
// pull requests carry no backlink, so `repository repair --pr-backlinks` is a
// no-op that says so and never touches GitHub.
func TestIntegrationRepoSetupPrivatePRBacklinkRepairIsNoOp(t *testing.T) {
	r, _ := initPrivateHealthy(t)
	d := SetupDeps{Git: newGitClient(t), RepoDir: r.invocation, GitHub: fatalRepairGitHub{t: t}}
	res := RunRepositoryRepair(context.Background(), d, RepairOptions{PRBacklinks: true})
	if res.Result != ResultNoOp {
		t.Fatalf("private PR-backlink repair = %q (%s), want no-op", res.Result, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "a private repository's pull requests carry no backlink") {
		t.Errorf("private PR-backlink repair text = %q; want it to say a private repository's pull requests carry no backlink", res.HumanText())
	}
}

// TestIntegrationRepoSetupPrivateMetadataOnSharedRemote proves a docket-named
// branch or refs/docket/ ref on a private repository's origin is reported by
// check and prepare as a warning, and never changes the classified state.
func TestIntegrationRepoSetupPrivateMetadataOnSharedRemote(t *testing.T) {
	r, _ := initPrivateHealthy(t)
	before := r.runCheck(t)
	if got := findingsWithCode(before.Findings, FindingMetadataOnSharedRemote); len(got) != 0 {
		t.Fatalf("a clean private origin already reports %+v", got)
	}

	runGit(t, r.writer, "push", "-q", "origin", "main:refs/heads/docket", "main:refs/docket/x")
	want := []string{"refs/docket/x", "refs/heads/docket"}
	refsOf := func(fs []reposetup.Finding) []string {
		var out []string
		for _, f := range findingsWithCode(fs, FindingMetadataOnSharedRemote) {
			if f.Severity != reposetup.SeverityWarning {
				t.Errorf("finding %+v is not a warning", f)
			}
			out = append(out, f.Ref)
		}
		return out
	}

	chk := r.runCheck(t)
	if got := refsOf(chk.Findings); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("check %s refs = %v, want %v:\n%s", FindingMetadataOnSharedRemote, got, want, chk.HumanText())
	}
	if chk.RepositoryState != before.RepositoryState {
		t.Errorf("state moved from %q to %q; the finding never changes the state", before.RepositoryState, chk.RepositoryState)
	}

	prep := RunRepositoryPrepare(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: r.invocation}, PrepareOptions{})
	if prep.Disposition != PrepareDispositionApplied && prep.Disposition != PrepareDispositionNoOp {
		t.Fatalf("prepare disposition = %q (%s), want applied or no-op", prep.Disposition, prep.HumanText())
	}
	if got := refsOf(prep.Findings); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("prepare %s refs = %v, want %v:\n%s", FindingMetadataOnSharedRemote, got, want, prep.HumanText())
	}
	if !strings.Contains(prep.HumanText(), "- [warning] "+FindingMetadataOnSharedRemote+" refs/heads/docket") {
		t.Errorf("prepare human text does not list the finding:\n%s", prep.HumanText())
	}
}
