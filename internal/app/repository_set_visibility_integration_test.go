//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/testsupport"
)

// This file is the real-Git `repository set-visibility` shard (prefix
// TestIntegrationRepoVisibility): the preview, and every refusal that stops a
// switch before it writes.

// runSetVisibility runs RunRepositorySetVisibility against the invocation clone
// with the user's machine roots pinned to temp dirs.
func (r *initRepo) runSetVisibility(t *testing.T, o SetVisibilityOptions) RepositorySetVisibilityResult {
	t.Helper()
	return r.runSetVisibilityWithHooks(t, o, setupHooks{})
}

// runSetVisibilityWithHooks is runSetVisibility with the interruption seams set.
func (r *initRepo) runSetVisibilityWithHooks(t *testing.T, o SetVisibilityOptions, hooks setupHooks) RepositorySetVisibilityResult {
	t.Helper()
	pinInitUserRoots(t)
	client := newGitClient(t)
	return RunRepositorySetVisibility(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation, hooks: hooks}, o)
}

// newSharedVisibilityRepo is a shared repository right after init.
func newSharedVisibilityRepo(t *testing.T) *initRepo {
	t.Helper()
	r := newInitRepo(t, defaultSetupYML, nil)
	if res := r.runInit(t); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	return r
}

// visibilitySnapshot captures what a preview must never change: the working
// tree status, every ref on origin, the top-level .git listing, and whether the
// state folders exist.
func visibilitySnapshot(t *testing.T, r *initRepo) string {
	t.Helper()
	var b strings.Builder
	b.WriteString(runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all") + "\n--\n")
	b.WriteString(runGit(t, r.origin, "for-each-ref", "--format=%(refname) %(objectname)") + "\n--\n")
	entries, err := os.ReadDir(r.gitDir(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.Name() == "FETCH_HEAD" {
			continue // a read-only probe fetches; FETCH_HEAD is git's scratch record of that
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	b.WriteString(strings.Join(names, " "))
	return b.String()
}

func requireVisibilityRefusal(t *testing.T, res RepositorySetVisibilityResult, want ...string) {
	t.Helper()
	if res.Result != ResultInvalidState || res.ConfirmationRequired() {
		t.Fatalf("Result = %q state %q (%s), want an invalid-state refusal", res.Result, res.RepositoryState, res.HumanText())
	}
	for _, w := range want {
		if !strings.Contains(res.HumanText(), w) {
			t.Errorf("refusal text lacks %q:\n%s", w, res.HumanText())
		}
	}
}

// TestIntegrationRepoVisibilityPreviewWritesNothing proves a private preview
// lists its phases and pin while the working tree, origin's refs, the .git
// listing, and the private store stay untouched.
func TestIntegrationRepoVisibilityPreviewWritesNothing(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	before := visibilitySnapshot(t, r)
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	after := visibilitySnapshot(t, r)

	if res.Result != ResultInvalidState || !res.ConfirmationRequired() {
		t.Fatalf("Result = %q state %q (%s), want a confirmation-required preview", res.Result, res.RepositoryState, res.HumanText())
	}
	if before != after {
		t.Errorf("the preview changed the repository:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if res.Current != "shared" || res.Target != "private" {
		t.Errorf("current/target = %q/%q, want shared/private", res.Current, res.Target)
	}
	if !strings.HasPrefix(res.SourceRevision, "origin-docket=") || !strings.Contains(res.SourceRevision, " dckt=absent ") {
		t.Errorf("SourceRevision = %q, want the composite pin with no dckt branch", res.SourceRevision)
	}
	pending := 0
	for _, p := range res.Phases {
		if p.Status == visibilityPhasePending {
			pending++
		}
		if !strings.Contains(res.HumanText(), "] "+p.Name+": ") {
			t.Errorf("preview text does not list phase %s:\n%s", p.Name, res.HumanText())
		}
	}
	if pending == 0 {
		t.Errorf("phases = %+v, want pending phases", res.Phases)
	}
	priv := expectedPrivateLayout(t, r.invocation, os.Getenv("XDG_DATA_HOME"))
	if _, err := os.Stat(priv.StoreDir); !os.IsNotExist(err) {
		t.Errorf("the preview touched the private store %s (stat err %v)", priv.StoreDir, err)
	}
}

// TestIntegrationRepoVisibilityRefusesLiveRun proves an active run refuses the
// switch, naming the cancel command for its key.
func TestIntegrationRepoVisibilityRefusesLiveRun(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	key, err := MintRunTrackerRecord(r.invocation, sampleRunTrackerRecord())
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	if _, err := MintRunRecord(r.invocation, key, "375"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket run cancel --key "+key)
}

// TestIntegrationRepoVisibilityRefusesBusyGateLock proves a held gate lock in
// the state folder refuses the switch.
func TestIntegrationRepoVisibilityRefusesBusyGateLock(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	dir := filepath.Join(r.gitDir(t), layout.SharedName, "worktree-locks", "x")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, busy, err := process.TryExclusiveLock(filepath.Join(dir, "busy.lock"))
	if err != nil || busy {
		t.Fatalf("TryExclusiveLock: busy=%v err=%v", busy, err)
	}
	defer f.Close()
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "busy.lock")
}

// TestIntegrationRepoVisibilityRefusesDirtyMetadataWorktree proves an
// uncommitted change in .docket refuses the switch.
func TestIntegrationRepoVisibilityRefusesDirtyMetadataWorktree(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	writeRepoFile(t, filepath.Join(r.invocation, ".docket"), "scratch.md", "unsaved\n")
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "commit or discard the changes in")
}

// TestIntegrationRepoVisibilityRefusesUnpublishedMetadataCommit proves a
// metadata commit origin lacks refuses the switch.
func TestIntegrationRepoVisibilityRefusesUnpublishedMetadataCommit(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	dotDocket := filepath.Join(r.invocation, ".docket")
	gitIdentity(t, dotDocket)
	writeRepoFile(t, dotDocket, "local.md", "not pushed\n")
	runGit(t, dotDocket, "add", "--", "local.md")
	runGit(t, dotDocket, "commit", "-q", "-m", "local only")
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket repository prepare")
}

// TestIntegrationRepoVisibilityRefusesFreshAndLegacy proves a repository that
// was never set up names init, and a legacy one names migrate.
func TestIntegrationRepoVisibilityRefusesFreshAndLegacy(t *testing.T) {
	fresh := newInitRepo(t, defaultSetupYML, nil)
	requireVisibilityRefusal(t, fresh.runSetVisibility(t, SetVisibilityOptions{Target: "private"}), "docket repository init")

	legacy := newInitRepo(t, defaultSetupYML, cleanLegacyFiles())
	res := legacy.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket repository migrate")
	if res.RepositoryState != string(reposetup.StateLegacy) {
		t.Errorf("RepositoryState = %q, want legacy", res.RepositoryState)
	}
}

// TestIntegrationRepoVisibilityWarnsAbsoluteLinksWithoutRefusing proves a
// metadata file whose prose links absolutely into the docket branch is named in
// a warning that names `docket repository repair`, and the preview still stands.
func TestIntegrationRepoVisibilityWarnsAbsoluteLinksWithoutRefusing(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	runGit(t, r.invocation, "remote", "set-url", "origin", "https://github.com/acme/app.git")
	runGit(t, r.invocation, "config", "url."+r.origin+".insteadOf", "https://github.com/acme/app.git")

	const spec = "docs/superpowers/specs/2026-10-01-example-design.md"
	dotDocket := filepath.Join(r.invocation, ".docket")
	gitIdentity(t, dotDocket)
	writeRepoFile(t, dotDocket, spec, "# Example\n\nSee https://github.com/acme/app/blob/docket/x.md for context.\n")
	runGit(t, dotDocket, "add", "--", spec)
	runGit(t, dotDocket, "commit", "-q", "-m", "a spec with an absolute link")
	runGit(t, dotDocket, "push", "-q", "origin", layout.SharedName)

	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	if res.Result != ResultInvalidState || !res.ConfirmationRequired() {
		t.Fatalf("Result = %q state %q (%s), want a confirmation-required preview", res.Result, res.RepositoryState, res.HumanText())
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, spec) && strings.Contains(w, "docket repository repair") {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings %q do not name %s and `docket repository repair`", res.Warnings, spec)
	}
}

// --- going private -----------------------------------------------------------

// privateSwitchYML is the committed configuration the going-private fixtures
// carry: the shared defaults, a harness opt-in (so init writes the committed
// dispatch surfaces), and a build command the private config must keep.
const privateSwitchYML = defaultSetupYML + "agent_harnesses: [codex]\nbuild:\n  test_command: make\n"

// privateSwitchLocalYML is the clone-local configuration whose leaves must
// survive the fold.
const privateSwitchLocalYML = "reclaim:\n  auto: true\n"

// newPrivateSwitchRepo is a shared repository right after init whose init
// output is committed and pushed (so the working tree is clean), with
// XDG_DATA_HOME pinned for the whole test so every run resolves one store. It
// returns the repository and that data home.
func newPrivateSwitchRepo(t *testing.T) (*initRepo, string) {
	t.Helper()
	data := testsupport.TempDir(t)
	t.Setenv("XDG_DATA_HOME", data)
	r := newInitRepo(t, privateSwitchYML, nil)
	if res := r.runInitWith(t, InitOptions{}); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	commitAndPushAll(t, r.invocation, "commit the docket setup")
	if out := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all"); out != "" {
		t.Fatalf("the fixture is not clean after committing init's output:\n%s", out)
	}
	return r, data
}

// commitAndPushAll commits every pending path in dir and pushes main.
func commitAndPushAll(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	if out := runGit(t, dir, "status", "--porcelain"); out != "" {
		runGit(t, dir, "commit", "-q", "-m", msg)
	}
	runGit(t, dir, "push", "-q", "origin", "main")
}

// switchVisibility previews o, then applies it pinned to the preview.
func (r *initRepo) switchVisibility(t *testing.T, o SetVisibilityOptions) RepositorySetVisibilityResult {
	t.Helper()
	return r.switchVisibilityWithHooks(t, o, setupHooks{})
}

// switchVisibilityWithHooks is switchVisibility with the seams set on the
// authorized run.
func (r *initRepo) switchVisibilityWithHooks(t *testing.T, o SetVisibilityOptions, hooks setupHooks) RepositorySetVisibilityResult {
	t.Helper()
	preview := r.runSetVisibility(t, o)
	if !preview.ConfirmationRequired() {
		t.Fatalf("preview = %q state %q (%s), want confirmation-required", preview.Result, preview.RepositoryState, preview.HumanText())
	}
	o.Authorized, o.ExpectedSource = true, preview.SourceRev()
	return r.runSetVisibilityWithHooks(t, o, hooks)
}

// requireSwitchApplied fails unless res is an applied switch landing in mode.
func requireSwitchApplied(t *testing.T, res RepositorySetVisibilityResult, mode string) {
	t.Helper()
	if res.Result != ResultApplied || res.RepositoryState != mode {
		t.Fatalf("switch = %q state %q, want applied %s:\n%s", res.Result, res.RepositoryState, mode, res.HumanText())
	}
}

// phaseStatus returns the status of the named phase row ("" when absent).
func phaseStatus(res RepositorySetVisibilityResult, name string) string {
	for _, p := range res.Phases {
		if p.Name == name {
			return p.Status
		}
	}
	return ""
}

// createSwitchChange records one change through the production planning path
// and returns its result.
func createSwitchChange(t *testing.T, dir string) ChangeCreateResult {
	t.Helper()
	node := planningDepsFor(t, dir)
	res := ChangeCreate(context.Background(), node.deps, node.dir, validChangeCreateRequest())
	if res.Result != ResultApplied {
		t.Fatalf("ChangeCreate = %q (%v)", res.Result, res.Findings)
	}
	return res
}

// localBranchExists reports whether dir's repository has the local branch.
func localBranchExists(dir, branch string) bool {
	_, err := tryGit(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// requireNoSharedStateLeft asserts the clone holds only the private state
// folder and no .docket checkout or local docket branch.
func requireNoSharedStateLeft(t *testing.T, r *initRepo) {
	t.Helper()
	gitDir := r.gitDir(t)
	if _, err := os.Stat(filepath.Join(gitDir, layout.SharedName)); !os.IsNotExist(err) {
		t.Errorf("the shared state folder is still present (stat err %v)", err)
	}
	if fi, err := os.Stat(filepath.Join(gitDir, layout.PrivateName)); err != nil || !fi.IsDir() {
		t.Errorf("the private state folder is missing (err %v)", err)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, layout.SharedWorktreeDir)); !os.IsNotExist(err) {
		t.Errorf("the .docket checkout is still present (stat err %v)", err)
	}
	if localBranchExists(r.invocation, layout.SharedName) {
		t.Error("the local docket branch is still present")
	}
}

// TestIntegrationRepoVisibilityPrivateHappyPath proves the whole going-private
// switch: the identical history on the bare dckt remote, origin's branch kept,
// the folded private config with the local keys saved, the exclude block, the
// private checkout, the private dispatch instructions, the committed files
// untouched, a clean working tree, and a no-op re-preview and prepare.
func TestIntegrationRepoVisibilityPrivateHappyPath(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	createSwitchChange(t, r.invocation)
	writeRepoFile(t, r.invocation, ".docket.local.yml", privateSwitchLocalYML)
	originTip := r.originTip(t, layout.SharedName)
	agentsBefore := runGit(t, r.invocation, "show", "HEAD:AGENTS.md")
	headBefore := runGit(t, r.invocation, "rev-parse", "HEAD")

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "private"})
	requireSwitchApplied(t, res, "private")

	lay := expectedPrivateLayout(t, r.invocation, data)
	if got := runGit(t, lay.DefaultBareRemote, "rev-parse", "refs/heads/dckt"); got != originTip {
		t.Errorf("bare dckt tip = %s, want origin's docket tip %s", got, originTip)
	}
	if got := r.originTip(t, layout.SharedName); got != originTip {
		t.Errorf("origin's docket moved to %s, want it kept at %s", got, originTip)
	}
	requireNoSharedStateLeft(t, r)
	gitDir := r.gitDir(t)

	folded, err := os.ReadFile(filepath.Join(gitDir, "dckt", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	snap, _, err := config.Resolve([]config.Source{{Layer: config.LayerRepository, Name: layout.PrivateConfigDisplay, Data: folded}}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("the private config does not resolve: %v\n%s", err, folded)
	}
	if v := snap.Effective.Visibility.Value; v != "private" {
		t.Errorf("visibility = %q, want private", v)
	}
	if v := snap.Effective.Build.TestCommand.Value; v != "make" {
		t.Errorf("build.test_command = %q, want make", v)
	}
	if !snap.Effective.Reclaim.Auto.Value {
		t.Errorf("reclaim.auto = false, want the local key carried:\n%s", folded)
	}
	if saved, err := os.ReadFile(filepath.Join(gitDir, "dckt", layout.PrivateLocalKeysFile)); err != nil || string(saved) != privateSwitchLocalYML {
		t.Errorf("local-keys.yml = %q (%v), want the old local file verbatim", saved, err)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket.local.yml")); !os.IsNotExist(err) {
		t.Errorf(".docket.local.yml is still present (stat err %v)", err)
	}
	exclude, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	if err != nil || !reposetup.ValidExcludeBlock(exclude) {
		t.Errorf(".git/info/exclude = %q (%v), want the valid managed block", exclude, err)
	}

	if got := runGit(t, lay.MetadataWorktree, "rev-parse", "--abbrev-ref", "HEAD"); got != "dckt" {
		t.Errorf("the private checkout is on %q, want dckt", got)
	}
	if off := runGit(t, lay.MetadataWorktree, "config", "--worktree", "core.hooksPath"); off == "" {
		t.Error("the private checkout's hooks are not disabled")
	}
	ins := Instructions(r.invocation, InstructionsSectionDispatch)
	if !strings.Contains(ins.Content, document.MarkerSpelling(instructionsBlockName)+":start") {
		t.Errorf("Instructions(dispatch) = %+v, want the dispatch block", ins)
	}
	if got := runGit(t, r.invocation, "show", "HEAD:AGENTS.md"); got != agentsBefore {
		t.Errorf("the committed AGENTS.md changed:\n%s", got)
	}
	if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != headBefore {
		t.Errorf("HEAD moved to %s without --remove-shared-files", got)
	}
	if out := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all"); out != "" {
		t.Errorf("the working tree is not clean after the switch:\n%s", out)
	}

	again := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	if again.Result != ResultNoOp {
		t.Errorf("re-preview = %q (%s), want no-op", again.Result, again.HumanText())
	}
	prep := RunRepositoryPrepare(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: r.invocation}, PrepareOptions{})
	if prep.Disposition != PrepareDispositionNoOp || prep.Context == nil || prep.Context.MetadataRemote != "dckt" {
		t.Errorf("prepare = %q context %+v (%s), want no-op on the dckt remote", prep.Disposition, prep.Context, prep.HumanText())
	}
}

// TestIntegrationRepoVisibilityPrivateDeleteSharedBranch proves
// --delete-shared-branch removes origin's docket branch once the bare remote
// holds its tip, and a re-run is a no-op.
func TestIntegrationRepoVisibilityPrivateDeleteSharedBranch(t *testing.T) {
	r, _ := newPrivateSwitchRepo(t)
	o := SetVisibilityOptions{Target: "private", DeleteSharedBranch: true}
	res := r.switchVisibility(t, o)
	requireSwitchApplied(t, res, "private")
	if r.remoteBranchExists(t, layout.SharedName) {
		t.Error("origin still has the docket branch")
	}
	if got := phaseStatus(res, "delete-shared-branch"); got != visibilityPhaseApplied {
		t.Errorf("delete-shared-branch = %q, want applied", got)
	}
	if again := r.runSetVisibility(t, o); again.Result != ResultNoOp {
		t.Errorf("re-run = %q (%s), want no-op", again.Result, again.HumanText())
	}
}

// TestIntegrationRepoVisibilityPrivateDeleteSharedBranchKeepsOnMismatch proves
// a teammate's write to origin's docket branch after the publish keeps the
// branch: the phase is kept and the pending line names both tips.
func TestIntegrationRepoVisibilityPrivateDeleteSharedBranchKeepsOnMismatch(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	var advanced string
	res := r.switchVisibilityWithHooks(t, SetVisibilityOptions{Target: "private", DeleteSharedBranch: true}, setupHooks{
		afterVisibilityPhase: func(phase string) error {
			if phase == "publish" {
				advanced = r.advanceRemoteDocketChild(t, "a teammate's later write")
			}
			return nil
		},
	})
	requireSwitchApplied(t, res, "private")
	if got := r.originTip(t, layout.SharedName); got != advanced {
		t.Errorf("origin's docket = %s, want it kept at the teammate's tip %s", got, advanced)
	}
	if got := phaseStatus(res, "delete-shared-branch"); got != visibilityPhaseKept {
		t.Errorf("delete-shared-branch = %q, want kept", got)
	}
	bare := runGit(t, expectedPrivateLayout(t, r.invocation, data).DefaultBareRemote, "rev-parse", "refs/heads/dckt")
	pending := strings.Join(res.PendingLocal, "\n")
	if !strings.Contains(pending, advanced) || !strings.Contains(pending, bare) {
		t.Errorf("PendingLocal = %q, want both tips (origin %s, dckt %s)", res.PendingLocal, advanced, bare)
	}
}

// requireRemovalCommit asserts HEAD is exactly one new commit over before,
// with the fixed subject and no body, deleting .docket.yml and removing the
// managed .gitignore block and the AGENTS.md dispatch block, and nothing else.
func requireRemovalCommit(t *testing.T, r *initRepo, before string) {
	t.Helper()
	if n := runGit(t, r.invocation, "rev-list", "--count", before+"..HEAD"); n != "1" {
		t.Fatalf("%s new commits on main, want exactly one", n)
	}
	if msg, err := tryGit(r.invocation, "log", "-1", "--format=%B"); err != nil || msg != visibilityRemoveSubject+"\n\n" && msg != visibilityRemoveSubject+"\n" {
		t.Errorf("commit message = %q (%v), want exactly the subject", msg, err)
	}
	got := map[string]string{}
	for _, line := range strings.Split(runGit(t, r.invocation, "show", "--name-status", "--format=", "HEAD"), "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			got[f[1]] = f[0]
		}
	}
	if got[".docket.yml"] != "D" {
		t.Errorf(".docket.yml status = %q, want D (%v)", got[".docket.yml"], got)
	}
	for _, p := range []string{".gitignore", "AGENTS.md"} {
		if s := got[p]; s != "M" && s != "D" {
			t.Errorf("%s status = %q, want M or D (%v)", p, s, got)
		}
	}
	for p := range got {
		if !contains(visibilityCommitPaths, p) {
			t.Errorf("the commit holds %s, which is not one of the switch's paths", p)
		}
	}
	if b, err := tryGit(r.invocation, "show", "HEAD:AGENTS.md"); err == nil && strings.Contains(b, document.MarkerSpelling(instructionsBlockName)) {
		t.Errorf("the committed AGENTS.md still carries the dispatch block:\n%s", b)
	}
}

// TestIntegrationRepoVisibilityPrivateRemoveSharedFiles proves
// --remove-shared-files makes one local commit with the fixed subject holding
// only the switch's paths: a staged unrelated file stays staged and origin's
// main is not pushed.
func TestIntegrationRepoVisibilityPrivateRemoveSharedFiles(t *testing.T) {
	r, _ := newPrivateSwitchRepo(t)
	writeRepoFile(t, r.invocation, "notes.txt", "mine\n")
	runGit(t, r.invocation, "add", "--", "notes.txt")
	before := runGit(t, r.invocation, "rev-parse", "HEAD")
	originMain := r.originTip(t, "main")

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true})
	requireSwitchApplied(t, res, "private")
	requireRemovalCommit(t, r, before)
	if staged := runGit(t, r.invocation, "diff", "--cached", "--name-only"); staged != "notes.txt" {
		t.Errorf("staged = %q, want notes.txt still staged", staged)
	}
	if got := r.originTip(t, "main"); got != originMain {
		t.Errorf("origin's main moved to %s; the switch must never push it", got)
	}
	if len(res.Commits) != 1 || res.Commits[0].Status != visibilityCommitCommitted || res.Commits[0].Branch != "main" ||
		res.Commits[0].Commit != runGit(t, r.invocation, "rev-parse", "HEAD") {
		t.Errorf("Commits = %+v, want the one committed row on main", res.Commits)
	}
	if again := r.runSetVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true}); again.Result != ResultNoOp {
		t.Errorf("re-run = %q (%s), want no-op", again.Result, again.HumanText())
	}
}

// TestIntegrationRepoVisibilityPrivateRemoveSharedFilesLater proves the flag
// on a repository that already went private makes the same one commit.
func TestIntegrationRepoVisibilityPrivateRemoveSharedFilesLater(t *testing.T) {
	r, _ := newPrivateSwitchRepo(t)
	before := runGit(t, r.invocation, "rev-parse", "HEAD")
	requireSwitchApplied(t, r.switchVisibility(t, SetVisibilityOptions{Target: "private"}), "private")
	if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != before {
		t.Fatalf("HEAD moved to %s without the flag", got)
	}
	res := r.switchVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true})
	requireSwitchApplied(t, res, "private")
	requireRemovalCommit(t, r, before)
	for _, p := range res.Phases {
		if p.Name != "remove-shared-files" && p.Status != visibilityPhaseDone {
			t.Errorf("phase %s = %q, want done on the later run", p.Name, p.Status)
		}
	}
}

// TestIntegrationRepoVisibilityPrivateRefusesForeignBareRemote proves a dckt
// branch in the default store that is unrelated to origin's docket branch
// refuses the publish, naming both tips, before the state folder moves.
func TestIntegrationRepoVisibilityPrivateRefusesForeignBareRemote(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	lay := expectedPrivateLayout(t, r.invocation, data)
	if err := os.MkdirAll(filepath.Dir(lay.DefaultBareRemote), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, r.root, "init", "-q", "--bare", lay.DefaultBareRemote)
	foreign := filepath.Join(r.root, "foreign")
	runGit(t, r.root, "init", "-q", "-b", "dckt", foreign)
	gitIdentity(t, foreign)
	writeRepoFile(t, foreign, "other.md", "another backlog\n")
	runGit(t, foreign, "add", "-A")
	runGit(t, foreign, "commit", "-q", "-m", "another backlog")
	runGit(t, foreign, "push", "-q", lay.DefaultBareRemote, "dckt:refs/heads/dckt")
	foreignTip := runGit(t, foreign, "rev-parse", "HEAD")
	originTip := r.originTip(t, layout.SharedName)
	stateBefore := treeListing(t, filepath.Join(r.gitDir(t), layout.SharedName))

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, foreignTip, originTip)
	if got := runGit(t, lay.DefaultBareRemote, "rev-parse", "refs/heads/dckt"); got != foreignTip {
		t.Errorf("the foreign dckt branch moved to %s", got)
	}
	if after := treeListing(t, filepath.Join(r.gitDir(t), layout.SharedName)); after != stateBefore {
		t.Errorf("the shared state folder changed:\nbefore:\n%s\nafter:\n%s", stateBefore, after)
	}
	if _, err := os.Stat(filepath.Join(r.gitDir(t), layout.PrivateName)); !os.IsNotExist(err) {
		t.Errorf("the private state folder exists after the refusal (stat err %v)", err)
	}
}

// treeListing lists every path under dir with its size, sorted.
func treeListing(t *testing.T, dir string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		lines = append(lines, rel+" "+info.Mode().String()+" "+strconv.FormatInt(info.Size(), 10))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestIntegrationRepoVisibilityPrivateMetadataRemoteFlag proves
// --metadata-remote publishes to the named bare repository and never creates
// the default store's remote.git.
func TestIntegrationRepoVisibilityPrivateMetadataRemoteFlag(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	backup := filepath.Join(r.root, "backup.git")
	runGit(t, r.root, "init", "-q", "--bare", backup)
	originTip := r.originTip(t, layout.SharedName)

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "private", MetadataRemote: backup})
	requireSwitchApplied(t, res, "private")
	canonical, err := filepath.EvalSymlinks(backup)
	if err != nil {
		t.Fatal(err)
	}
	if got := runGit(t, r.invocation, "config", "--get", "remote.dckt.url"); got != backup && got != canonical {
		t.Errorf("remote.dckt.url = %q, want %s", got, backup)
	}
	if got := runGit(t, backup, "rev-parse", "refs/heads/dckt"); got != originTip {
		t.Errorf("backup dckt = %s, want origin's docket tip %s", got, originTip)
	}
	if _, err := os.Stat(expectedPrivateLayout(t, r.invocation, data).DefaultBareRemote); !os.IsNotExist(err) {
		t.Errorf("the default store's remote.git exists (stat err %v)", err)
	}
	if again := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"}); again.Result != ResultNoOp {
		t.Errorf("a flagless re-run = %q (%s), want no-op", again.Result, again.HumanText())
	}
}

// TestIntegrationRepoVisibilityPrivateSecondCloneAfterCleanup proves a second
// clone of a repository that already went private with both flags (origin's
// docket branch gone, .docket.yml off origin's default branch) adopts the
// store on the same machine: its publish is already done, its config folds
// from its own HEAD, and its checkout holds the first clone's change.
func TestIntegrationRepoVisibilityPrivateSecondCloneAfterCleanup(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	b := &initRepo{root: r.root, origin: r.origin, writer: r.writer, invocation: filepath.Join(r.root, "second")}
	runGit(t, r.root, "clone", "-q", r.origin, b.invocation)
	gitIdentity(t, b.invocation)
	if res := b.runInitWith(t, InitOptions{}); res.Result != ResultApplied && res.Result != ResultNoOp {
		t.Fatalf("second clone init = %q (%s)", res.Result, res.HumanText())
	}

	created := createSwitchChange(t, r.invocation)
	requireSwitchApplied(t, r.switchVisibility(t, SetVisibilityOptions{Target: "private", DeleteSharedBranch: true, RemoveSharedFiles: true}), "private")
	runGit(t, r.invocation, "push", "-q", "origin", "main")
	if r.remoteBranchExists(t, layout.SharedName) {
		t.Fatal("origin still has the docket branch after the first clone's switch")
	}

	preview := b.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	if !preview.ConfirmationRequired() {
		t.Fatalf("second clone preview = %q (%s), want confirmation-required", preview.Result, preview.HumanText())
	}
	if got := phaseStatus(preview, "publish"); got != visibilityPhaseDone {
		t.Errorf("publish = %q, want done (the store already holds the history)", got)
	}
	for _, p := range preview.Phases {
		if p.Name == "config" && !strings.Contains(p.Detail, "HEAD") {
			t.Errorf("config detail = %q, want it to name the primary checkout's HEAD", p.Detail)
		}
	}
	res := b.runSetVisibility(t, SetVisibilityOptions{Target: "private", Authorized: true, ExpectedSource: preview.SourceRev()})
	requireSwitchApplied(t, res, "private")
	if got := phaseStatus(res, "publish"); got != visibilityPhaseDone {
		t.Errorf("publish = %q after the run, want done", got)
	}
	requireNoSharedStateLeft(t, b)
	checkout := expectedPrivateLayout(t, b.invocation, data).MetadataWorktree
	if _, err := os.Stat(filepath.Join(checkout, filepath.FromSlash(created.Path))); err != nil {
		t.Errorf("the second clone's checkout lacks the first clone's change %s: %v", created.Path, err)
	}
}
