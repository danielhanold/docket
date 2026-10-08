//go:build integration

package app

import (
	"context"
	"errors"
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

// This file holds the real-Git `repository set-visibility` tests: the preview,
// every refusal that stops a switch before it writes, each direction's switch,
// and the acceptance proofs (round trip, resume after every phase, the private
// result, commit messages). Their names split three ways so each sibling shard
// stays under its runtime budget: TestIntegrationRepoVisibilityPrivate… (going
// private, tests/test_go_integration_app_repovisibility_private.sh),
// TestIntegrationRepoVisibilityShared… (going shared,
// tests/test_go_integration_app_repovisibility_shared.sh), and
// TestIntegrationRepoVisibilitySwitch… (preview, refusals, commit machinery,
// and the direction-neutral acceptance proofs,
// tests/test_go_integration_app_repovisibility.sh). No prefix is a prefix of
// another, so every test matches exactly one shard.

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

// TestIntegrationRepoVisibilitySwitchPreviewWritesNothing proves a private preview
// lists its phases and pin while the working tree, origin's refs, the .git
// listing, and the private store stay untouched.
func TestIntegrationRepoVisibilitySwitchPreviewWritesNothing(t *testing.T) {
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

// TestIntegrationRepoVisibilitySwitchRefusesLiveRun proves an active run refuses the
// switch, naming the cancel command for its key. The run carries the authority run
// cancel accepts (a parent capability and a confirmed claim on its change), since
// the live-run scan names cancel only for a run cancel would act on.
func TestIntegrationRepoVisibilitySwitchRefusesLiveRun(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	rec := sampleRunTrackerRecord()
	rec.ParentCap = "parent-cap-raw"
	key, err := MintRunTrackerRecord(r.invocation, rec)
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	if _, err := MintRunRecord(r.invocation, key, "375"); err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	if err := ReserveRunTrackerClaim(r.invocation, key, 375, "req-1"); err != nil {
		t.Fatalf("ReserveRunTrackerClaim: %v", err)
	}
	if err := ConfirmRunTrackerClaim(r.invocation, key, 375, "req-1", "rev-1", ""); err != nil {
		t.Fatalf("ConfirmRunTrackerClaim: %v", err)
	}
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket run cancel --key "+key)
}

// TestIntegrationRepoVisibilitySwitchRefusesBusyGateLock proves a held gate lock in
// the state folder refuses the switch.
func TestIntegrationRepoVisibilitySwitchRefusesBusyGateLock(t *testing.T) {
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

// TestIntegrationRepoVisibilitySwitchRefusesDirtyMetadataWorktree proves an
// uncommitted change in .docket refuses the switch.
func TestIntegrationRepoVisibilitySwitchRefusesDirtyMetadataWorktree(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	writeRepoFile(t, filepath.Join(r.invocation, ".docket"), "scratch.md", "unsaved\n")
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "commit or discard the changes in")
}

// TestIntegrationRepoVisibilitySwitchRefusesUnpublishedMetadataCommit proves a
// metadata commit origin lacks refuses the switch.
func TestIntegrationRepoVisibilitySwitchRefusesUnpublishedMetadataCommit(t *testing.T) {
	r := newSharedVisibilityRepo(t)
	dotDocket := filepath.Join(r.invocation, ".docket")
	gitIdentity(t, dotDocket)
	writeRepoFile(t, dotDocket, "local.md", "not pushed\n")
	runGit(t, dotDocket, "add", "--", "local.md")
	runGit(t, dotDocket, "commit", "-q", "-m", "local only")
	res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket repository prepare")
}

// TestIntegrationRepoVisibilitySwitchRefusesFreshAndLegacy proves a repository that
// was never set up names init, and a legacy one names migrate.
func TestIntegrationRepoVisibilitySwitchRefusesFreshAndLegacy(t *testing.T) {
	fresh := newInitRepo(t, defaultSetupYML, nil)
	requireVisibilityRefusal(t, fresh.runSetVisibility(t, SetVisibilityOptions{Target: "private"}), "docket repository init")

	legacy := newInitRepo(t, defaultSetupYML, cleanLegacyFiles())
	res := legacy.runSetVisibility(t, SetVisibilityOptions{Target: "private"})
	requireVisibilityRefusal(t, res, "docket repository migrate")
	if res.RepositoryState != string(reposetup.StateLegacy) {
		t.Errorf("RepositoryState = %q, want legacy", res.RepositoryState)
	}
}

// TestIntegrationRepoVisibilitySwitchWarnsAbsoluteLinksWithoutRefusing proves a
// metadata file whose prose links absolutely into the docket branch is named in
// a warning that names `docket repository repair`, and the preview still stands.
func TestIntegrationRepoVisibilitySwitchWarnsAbsoluteLinksWithoutRefusing(t *testing.T) {
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

// TestIntegrationRepoVisibilityPrivateRemovalPreviewIsTheCommit proves the
// removal preview lists exactly what the removal commit does — a .gitignore and
// AGENTS.md left empty by the removal are deletions, not writes — and that a
// user's uncommitted edit to a path the commit never touches (a committed
// CLAUDE.md with no dispatch block) neither refuses the switch nor is swept in.
func TestIntegrationRepoVisibilityPrivateRemovalPreviewIsTheCommit(t *testing.T) {
	r, _ := newPrivateSwitchRepo(t)
	writeRepoFile(t, r.invocation, "CLAUDE.md", "my notes\n")
	commitAndPushAll(t, r.invocation, "add my notes")
	writeRepoFile(t, r.invocation, "CLAUDE.md", "my notes\nmore notes\n")
	before := runGit(t, r.invocation, "rev-parse", "HEAD")

	o := SetVisibilityOptions{Target: "private", RemoveSharedFiles: true}
	preview := r.runSetVisibility(t, o)
	if !preview.ConfirmationRequired() {
		t.Fatalf("preview = %q state %q (%s), want confirmation-required", preview.Result, preview.RepositoryState, preview.HumanText())
	}
	if len(preview.Commits) != 1 {
		t.Fatalf("preview commits = %+v, want one planned row", preview.Commits)
	}
	rows := preview.Commits[0].Paths
	if want := []string{".docket.yml (delete)", ".gitignore (delete)", "AGENTS.md (delete)"}; strings.Join(rows, ",") != strings.Join(want, ",") {
		t.Errorf("preview rows = %q, want %q", rows, want)
	}
	o.Authorized, o.ExpectedSource = true, preview.SourceRev()
	requireSwitchApplied(t, r.runSetVisibility(t, o), "private")

	got := commitNameStatus(t, r.invocation, "HEAD")
	if len(got) != len(rows) {
		t.Errorf("the commit holds %v, the preview listed %q", got, rows)
	}
	for _, row := range rows {
		rel, action, _ := strings.Cut(strings.TrimSuffix(row, ")"), " (")
		wantStatus := "M"
		if action == "delete" {
			wantStatus = "D"
		}
		if got[rel] != wantStatus {
			t.Errorf("the commit's %s = %q, the preview said %s", rel, got[rel], action)
		}
	}
	if n := runGit(t, r.invocation, "rev-list", "--count", before+"..HEAD"); n != "1" {
		t.Errorf("%s new commits, want the one removal commit", n)
	}
	if b := string(mustReadFile(t, filepath.Join(r.invocation, "CLAUDE.md"))); b != "my notes\nmore notes\n" {
		t.Errorf("CLAUDE.md = %q, want the uncommitted edit kept", b)
	}
	if diff := runGit(t, r.invocation, "diff", "--name-only", "--", "CLAUDE.md"); diff != "CLAUDE.md" {
		t.Errorf("unstaged diff = %q, want CLAUDE.md still modified", diff)
	}
	if staged := runGit(t, r.invocation, "diff", "--cached", "--name-only"); staged != "" {
		t.Errorf("staged = %q, want nothing staged", staged)
	}
}

// TestIntegrationRepoVisibilityPrivateUnlocatableStoreIsUnknown proves a
// repository with no metadata branch on origin and no dckt remote, whose
// default store cannot be located (no data home and no home directory), is an
// unknown probe, never a confident "run init first".
func TestIntegrationRepoVisibilityPrivateUnlocatableStoreIsUnknown(t *testing.T) {
	r := newInitRepo(t, defaultSetupYML, nil)
	client := newGitClient(t)
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "relative-data-home")
	res := RunRepositorySetVisibility(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, SetVisibilityOptions{Target: "private"})
	if res.Result != ResultInvalidState || res.RepositoryState != string(reposetup.StateUnknown) {
		t.Fatalf("Result = %q state %q (%s), want invalid-state unknown", res.Result, res.RepositoryState, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "private-store") {
		t.Errorf("refusal text lacks the private-store probe:\n%s", res.HumanText())
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

// TestIntegrationRepoVisibilityPrivateRerunIgnoresOriginAfterDivergence proves
// a later run on a repository that already went private judges the publish by
// its own progress, not origin's docket branch: after a teammate advanced
// origin's branch and this clone made a private commit, adding
// --remove-shared-files succeeds, pulls none of origin's commits into the dckt
// branch, and --delete-shared-branch keeps origin's branch, naming both tips.
func TestIntegrationRepoVisibilityPrivateRerunIgnoresOriginAfterDivergence(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	before := runGit(t, r.invocation, "rev-parse", "HEAD")
	requireSwitchApplied(t, r.switchVisibility(t, SetVisibilityOptions{Target: "private"}), "private")
	bareRemote := expectedPrivateLayout(t, r.invocation, data).DefaultBareRemote
	advanced := r.advanceRemoteDocketChild(t, "a teammate's later write")
	createSwitchChange(t, r.invocation)
	privateTip := runGit(t, bareRemote, "rev-parse", "refs/heads/dckt")
	if _, err := tryGit(bareRemote, "merge-base", "--is-ancestor", advanced, privateTip); err == nil {
		t.Fatal("the fixture's private commit already holds the teammate's write")
	}

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true})
	requireSwitchApplied(t, res, "private")
	requireRemovalCommit(t, r, before)
	if got := phaseStatus(res, "publish"); got != visibilityPhaseDone {
		t.Errorf("publish = %q, want done on the later run", got)
	}
	if got := runGit(t, bareRemote, "rev-parse", "refs/heads/dckt"); got != privateTip {
		t.Errorf("the dckt branch moved to %s, want it kept at the private tip %s", got, privateTip)
	}

	del := r.switchVisibility(t, SetVisibilityOptions{Target: "private", DeleteSharedBranch: true})
	requireSwitchApplied(t, del, "private")
	if got := r.originTip(t, layout.SharedName); got != advanced {
		t.Errorf("origin's docket = %s, want it kept at the teammate's tip %s", got, advanced)
	}
	if got := phaseStatus(del, "delete-shared-branch"); got != visibilityPhaseKept {
		t.Errorf("delete-shared-branch = %q, want kept", got)
	}
	pending := strings.Join(del.PendingLocal, "\n")
	if !strings.Contains(pending, advanced) || !strings.Contains(pending, privateTip) {
		t.Errorf("PendingLocal = %q, want both tips (origin %s, dckt %s)", del.PendingLocal, advanced, privateTip)
	}
	if got := runGit(t, bareRemote, "rev-parse", "refs/heads/dckt"); got != privateTip {
		t.Errorf("the dckt branch moved to %s, want it kept at the private tip %s", got, privateTip)
	}
}

// TestIntegrationRepoVisibilityPrivateRerunKeepsTeammateWritesOut proves a
// later run on a repository that already went private, with no private commit
// since, never fast-forwards a teammate's write to origin's docket branch into
// the dckt branch.
func TestIntegrationRepoVisibilityPrivateRerunKeepsTeammateWritesOut(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	requireSwitchApplied(t, r.switchVisibility(t, SetVisibilityOptions{Target: "private"}), "private")
	bareRemote := expectedPrivateLayout(t, r.invocation, data).DefaultBareRemote
	privateTip := runGit(t, bareRemote, "rev-parse", "refs/heads/dckt")
	r.advanceRemoteDocketChild(t, "a teammate's later write")

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true})
	requireSwitchApplied(t, res, "private")
	if got := runGit(t, bareRemote, "rev-parse", "refs/heads/dckt"); got != privateTip {
		t.Errorf("the dckt branch moved to %s, want it kept at %s without the teammate's write", got, privateTip)
	}
}

// TestIntegrationRepoVisibilityPrivateRetiresIgnoredCursorRule proves going
// private with cursor opted in retires the generated Cursor dispatch rule,
// which shared mode ignores rather than commits, so stripping the managed
// .gitignore block leaves no untracked docket-named file behind.
func TestIntegrationRepoVisibilityPrivateRetiresIgnoredCursorRule(t *testing.T) {
	data := testsupport.TempDir(t)
	t.Setenv("XDG_DATA_HOME", data)
	r := newInitRepo(t, defaultSetupYML+"agent_harnesses: [codex, cursor]\n", nil)
	if res := r.runInitWith(t, InitOptions{}); res.Result != ResultApplied {
		t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
	}
	commitAndPushAll(t, r.invocation, "commit the docket setup")
	const cursorRule = ".cursor/rules/docket-dispatch.mdc"
	if _, err := os.Stat(filepath.Join(r.invocation, cursorRule)); err != nil {
		t.Fatalf("the Cursor dispatch rule is not installed: %v", err)
	}
	if _, err := tryGit(r.invocation, "cat-file", "-e", "HEAD:"+cursorRule); err == nil {
		t.Fatal("the fixture committed the Cursor dispatch rule; it must be ignored")
	}

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true})
	requireSwitchApplied(t, res, "private")
	if _, err := os.Lstat(filepath.Join(r.invocation, cursorRule)); !os.IsNotExist(err) {
		t.Errorf("the Cursor dispatch rule is still present (lstat err %v)", err)
	}
	if out := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all"); out != "" {
		t.Errorf("the working tree is not clean after the switch:\n%s", out)
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

// --- going shared ------------------------------------------------------------

// sharedSwitchLessons is the promoted-lessons paragraph a born-private
// repository's instructions file carries outside its dispatch block.
const sharedSwitchLessons = "## Lessons\n\n- Prefer the narrow fix over the clever one.\n"

// newBornPrivateRepo is a repository set up private from the start, with the
// codex harness opted in so its dispatch block lives in the private
// instructions file. It returns the repository and the pinned data home.
func newBornPrivateRepo(t *testing.T) (*initRepo, string) {
	t.Helper()
	r, data := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true, Harnesses: harnessFlag("codex")}); res.Result != ResultApplied {
		t.Fatalf("private init = %q (%s), want applied", res.Result, res.HumanText())
	}
	return r, data
}

// commitNameStatus maps each path of commit to its --name-status letter.
func commitNameStatus(t *testing.T, dir, commit string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, line := range strings.Split(runGit(t, dir, "show", "--name-status", "--format=", commit), "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			got[f[1]] = f[0]
		}
	}
	return got
}

// requireAddCommit asserts commit carries exactly the fixed add subject with no
// body, and touches exactly the paths in want.
func requireAddCommit(t *testing.T, dir, commit string, want ...string) {
	t.Helper()
	if msg := runGit(t, dir, "log", "-1", "--format=%B", commit); msg != visibilityAddSubject {
		t.Errorf("commit %s message = %q, want exactly %q", commit, msg, visibilityAddSubject)
	}
	got := commitNameStatus(t, dir, commit)
	if len(got) != len(want) {
		t.Errorf("commit %s holds %v, want exactly %v", commit, got, want)
	}
	for _, p := range want {
		if _, ok := got[p]; !ok {
			t.Errorf("commit %s lacks %s (%v)", commit, p, got)
		}
	}
}

// requireSharedLayoutRestored asserts the clone holds only the shared state
// folder, the .docket checkout on the docket branch with hooks off, no dckt
// remote or branch, and no exclude block.
func requireSharedLayoutRestored(t *testing.T, r *initRepo) {
	t.Helper()
	gitDir := r.gitDir(t)
	if _, err := os.Stat(filepath.Join(gitDir, layout.PrivateName)); !os.IsNotExist(err) {
		t.Errorf("the private state folder is still present (stat err %v)", err)
	}
	if fi, err := os.Stat(filepath.Join(gitDir, layout.SharedName)); err != nil || !fi.IsDir() {
		t.Errorf("the shared state folder is missing (err %v)", err)
	}
	dotDocket := filepath.Join(r.invocation, layout.SharedWorktreeDir)
	if got := runGit(t, dotDocket, "rev-parse", "--abbrev-ref", "HEAD"); got != layout.SharedName {
		t.Errorf(".docket is on %q, want docket", got)
	}
	if off := runGit(t, dotDocket, "config", "--worktree", "core.hooksPath"); off == "" {
		t.Error("the .docket checkout's hooks are not disabled")
	}
	if _, err := tryGit(r.invocation, "config", "--get", "remote.dckt.url"); err == nil {
		t.Error("the dckt remote is still configured")
	}
	if localBranchExists(r.invocation, layout.PrivateName) {
		t.Error("the local dckt branch is still present")
	}
	if exclude, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude")); err == nil && strings.Contains(string(exclude), reposetup.ExcludeStart) {
		t.Errorf(".git/info/exclude still carries the managed block:\n%s", exclude)
	}
	for _, name := range []string{layout.PrivateConfigFile, layout.PrivateLocalKeysFile, switchJournalFile} {
		if _, err := os.Stat(filepath.Join(gitDir, layout.SharedName, name)); !os.IsNotExist(err) {
			t.Errorf("%s is left in the shared state folder (stat err %v)", name, err)
		}
	}
}

// TestIntegrationRepoVisibilitySharedFromBornPrivate proves the whole
// going-shared switch of a repository that was never shared: the identical
// history on origin, the shared layout, the bare store kept as the backup, one
// local commit holding .docket.yml, the .gitignore block, and the dispatch
// block, the promoted lessons reported, and origin's main never pushed.
func TestIntegrationRepoVisibilitySharedFromBornPrivate(t *testing.T) {
	r, data := newBornPrivateRepo(t)
	createSwitchChange(t, r.invocation)
	gitDir := r.gitDir(t)
	insPath := filepath.Join(gitDir, layout.PrivateName, layout.PrivateInstructionsFile)
	if err := os.WriteFile(insPath, append(mustReadFile(t, insPath), []byte("\n"+sharedSwitchLessons)...), 0o644); err != nil {
		t.Fatal(err)
	}
	lay := expectedPrivateLayout(t, r.invocation, data)
	bareTip := runGit(t, lay.DefaultBareRemote, "rev-parse", "refs/heads/dckt")
	originMain := r.originTip(t, "main")
	before := runGit(t, r.invocation, "rev-parse", "HEAD")

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "shared"})
	requireSwitchApplied(t, res, "shared")

	if got := r.originTip(t, layout.SharedName); got != bareTip {
		t.Errorf("origin's docket = %s, want the bare dckt tip %s", got, bareTip)
	}
	requireSharedLayoutRestored(t, r)
	if got := runGit(t, lay.DefaultBareRemote, "rev-parse", "refs/heads/dckt"); got != bareTip {
		t.Errorf("the backup store's dckt moved to %s, want it kept at %s", got, bareTip)
	}
	canonicalStore, _ := filepath.EvalSymlinks(lay.DefaultBareRemote)
	if res.BackupRemote != lay.DefaultBareRemote && res.BackupRemote != canonicalStore {
		t.Errorf("BackupRemote = %q, want the store %s", res.BackupRemote, lay.DefaultBareRemote)
	}
	if !strings.Contains(res.HumanText(), res.BackupRemote) || res.BackupRemote == "" {
		t.Errorf("the output does not name the backup remote:\n%s", res.HumanText())
	}

	if n := runGit(t, r.invocation, "rev-list", "--count", before+"..HEAD"); n != "1" {
		t.Fatalf("%s new commits on main, want exactly one", n)
	}
	requireAddCommit(t, r.invocation, "HEAD", ".docket.yml", ".gitignore", "AGENTS.md")
	if yml := runGit(t, r.invocation, "show", "HEAD:.docket.yml"); !strings.Contains(yml, "visibility: shared") {
		t.Errorf("the committed .docket.yml lacks visibility: shared:\n%s", yml)
	}
	if ign := runGit(t, r.invocation, "show", "HEAD:.gitignore"); !reposetup.ValidGitignoreBlock([]byte(ign + "\n")) {
		t.Errorf("the committed .gitignore lacks the valid block:\n%s", ign)
	}
	if agents := runGit(t, r.invocation, "show", "HEAD:AGENTS.md"); !strings.Contains(agents, document.MarkerSpelling(instructionsBlockName)+":start") {
		t.Errorf("the committed AGENTS.md lacks the dispatch block:\n%s", agents)
	}
	if len(res.Commits) != 1 || res.Commits[0].Status != visibilityCommitCommitted || res.Commits[0].Subject != visibilityAddSubject ||
		res.Commits[0].Commit != runGit(t, r.invocation, "rev-parse", "HEAD") {
		t.Errorf("Commits = %+v, want the one committed add row", res.Commits)
	}
	if !strings.Contains(res.Lessons, "Prefer the narrow fix over the clever one.") {
		t.Errorf("Lessons = %q, want the promoted lessons paragraph", res.Lessons)
	}
	if strings.Contains(res.Lessons, document.MarkerSpelling(instructionsBlockName)) {
		t.Errorf("Lessons = %q, want the dispatch block left out", res.Lessons)
	}
	if ins := Instructions(r.invocation, InstructionsSectionAll); ins.Private || ins.Content != "" {
		t.Errorf("Instructions = %+v, want a shared repository with nothing printed", ins)
	}
	if got := r.originTip(t, "main"); got != originMain {
		t.Errorf("origin's main moved to %s; the switch must never push it", got)
	}
	if out := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all"); out != "" {
		t.Errorf("the working tree is not clean after the switch:\n%s", out)
	}
	if again := r.runSetVisibility(t, SetVisibilityOptions{Target: "shared"}); again.Result != ResultNoOp {
		t.Errorf("re-preview = %q (%s), want no-op", again.Result, again.HumanText())
	}
}

// TestIntegrationRepoVisibilitySharedCommitsOnlyTrackedSurfaces proves going
// shared with claude and cursor opted in completes: the add commit carries the
// tracked surfaces (CLAUDE.md, AGENTS.md) but never the Cursor dispatch rule,
// which the managed .gitignore block ignores, and a re-run is a no-op.
func TestIntegrationRepoVisibilitySharedCommitsOnlyTrackedSurfaces(t *testing.T) {
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true, Harnesses: harnessFlag("codex", "claude", "cursor")}); res.Result != ResultApplied {
		t.Fatalf("private init = %q (%s), want applied", res.Result, res.HumanText())
	}
	before := runGit(t, r.invocation, "rev-parse", "HEAD")

	// The preview lists the commit's real paths: the tracked surfaces, never
	// the ignored Cursor rule.
	preview := r.runSetVisibility(t, SetVisibilityOptions{Target: "shared"})
	if !preview.ConfirmationRequired() || len(preview.Commits) != 1 {
		t.Fatalf("preview = %q (%s), want confirmation-required with one planned commit", preview.Result, preview.HumanText())
	}
	if want := []string{".docket.yml (write)", ".gitignore (write)", "AGENTS.md (write)", "CLAUDE.md (write)"}; strings.Join(preview.Commits[0].Paths, ",") != strings.Join(want, ",") {
		t.Errorf("preview rows = %q, want %q", preview.Commits[0].Paths, want)
	}

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "shared"})
	requireSwitchApplied(t, res, "shared")

	if n := runGit(t, r.invocation, "rev-list", "--count", before+"..HEAD"); n != "1" {
		t.Fatalf("%s new commits on main, want exactly one", n)
	}
	requireAddCommit(t, r.invocation, "HEAD", ".docket.yml", ".gitignore", "AGENTS.md", "CLAUDE.md")
	const cursorRule = ".cursor/rules/docket-dispatch.mdc"
	if _, err := os.Stat(filepath.Join(r.invocation, cursorRule)); err != nil {
		t.Errorf("the Cursor dispatch rule is not installed: %v", err)
	}
	if _, err := tryGit(r.invocation, "check-ignore", "-q", cursorRule); err != nil {
		t.Errorf("the Cursor dispatch rule is not ignored: %v", err)
	}
	requireSharedLayoutRestored(t, r)
	if out := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all"); out != "" {
		t.Errorf("the working tree is not clean after the switch:\n%s", out)
	}
	if again := r.runSetVisibility(t, SetVisibilityOptions{Target: "shared"}); again.Result != ResultNoOp {
		t.Errorf("re-preview = %q (%s), want no-op", again.Result, again.HumanText())
	}
}

// TestIntegrationRepoVisibilitySharedIdentityKeysStopThenResume proves a
// private config that sets a repository-identity key first commits .docket.yml
// and stops (nothing published), refuses until that commit reaches origin's
// default branch, then completes without recommitting .docket.yml.
func TestIntegrationRepoVisibilitySharedIdentityKeysStopThenResume(t *testing.T) {
	r, _ := newBornPrivateRepo(t)
	cfgPath := filepath.Join(r.gitDir(t), layout.PrivateName, layout.PrivateConfigFile)
	if err := os.WriteFile(cfgPath, append(mustReadFile(t, cfgPath), []byte("integration_branch: main\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	before := runGit(t, r.invocation, "rev-parse", "HEAD")

	run1 := r.switchVisibility(t, SetVisibilityOptions{Target: "shared"})
	// The stop is a needs-review refusal naming the remedy, never applied: the
	// repository is still private.
	if run1.Result != ResultInvalidState || run1.RepositoryState != string(reposetup.StateNeedsReview) {
		t.Fatalf("run 1 = %q state %q (%s), want invalid-state needs-review", run1.Result, run1.RepositoryState, run1.HumanText())
	}
	if len(run1.Commits) != 1 || run1.Commits[0].Status != visibilityCommitCommitted || run1.Commits[0].Commit == "" {
		t.Errorf("run 1 commits = %+v, want the one committed identity commit", run1.Commits)
	}
	if got := phaseStatus(run1, "identity-keys"); got != visibilityPhaseApplied {
		t.Errorf("identity-keys = %q, want applied", got)
	}
	if got := phaseStatus(run1, "publish"); got != visibilityPhasePending {
		t.Errorf("publish = %q, want still pending after the stop", got)
	}
	if n := runGit(t, r.invocation, "rev-list", "--count", before+"..HEAD"); n != "1" {
		t.Fatalf("%s new commits after run 1, want exactly one", n)
	}
	identityCommit := runGit(t, r.invocation, "rev-parse", "HEAD")
	if len(run1.Commits) == 1 && run1.Commits[0].Commit != identityCommit {
		t.Errorf("run 1 reports commit %s, want HEAD %s", run1.Commits[0].Commit, identityCommit)
	}
	requireAddCommit(t, r.invocation, identityCommit, ".docket.yml")
	if yml := runGit(t, r.invocation, "show", "HEAD:.docket.yml"); !strings.Contains(yml, "integration_branch: main") {
		t.Errorf("the identity commit's .docket.yml lacks integration_branch:\n%s", yml)
	}
	remedy := "get this commit onto main on origin"
	if !strings.Contains(run1.HumanText(), remedy) {
		t.Errorf("run 1 text = %q, want the push remedy", run1.HumanText())
	}
	if r.remoteBranchExists(t, layout.SharedName) {
		t.Error("origin gained a docket branch before the identity keys reached it")
	}

	run2 := r.switchVisibility(t, SetVisibilityOptions{Target: "shared"})
	requireVisibilityRefusal(t, run2, remedy)
	if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != identityCommit {
		t.Errorf("run 2 moved HEAD to %s, want it kept at the identity commit", got)
	}
	if r.remoteBranchExists(t, layout.SharedName) {
		t.Error("run 2 published origin's docket branch")
	}

	runGit(t, r.invocation, "push", "-q", "origin", "main")
	run3 := r.switchVisibility(t, SetVisibilityOptions{Target: "shared"})
	requireSwitchApplied(t, run3, "shared")
	if got := phaseStatus(run3, "identity-keys"); got != visibilityPhaseDone {
		t.Errorf("identity-keys = %q on run 3, want done", got)
	}
	if n := runGit(t, r.invocation, "rev-list", "--count", identityCommit+"..HEAD"); n != "1" {
		t.Fatalf("%s new commits on run 3, want exactly one", n)
	}
	requireAddCommit(t, r.invocation, "HEAD", ".gitignore", "AGENTS.md")
	requireSharedLayoutRestored(t, r)
}

// TestIntegrationRepoVisibilitySharedFastForwardsLeftBehindBranch proves a
// round trip without --delete-shared-branch, with a private write in between,
// fast-forwards the docket branch origin kept instead of refusing it.
func TestIntegrationRepoVisibilitySharedFastForwardsLeftBehindBranch(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	leftBehind := r.originTip(t, layout.SharedName)
	requireSwitchApplied(t, r.switchVisibility(t, SetVisibilityOptions{Target: "private"}), "private")
	createSwitchChange(t, r.invocation)
	bareTip := runGit(t, expectedPrivateLayout(t, r.invocation, data).DefaultBareRemote, "rev-parse", "refs/heads/dckt")
	if bareTip == leftBehind {
		t.Fatal("the private write did not advance the dckt branch")
	}

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "shared"})
	requireSwitchApplied(t, res, "shared")
	if got := r.originTip(t, layout.SharedName); got != bareTip {
		t.Errorf("origin's docket = %s, want it fast-forwarded to %s", got, bareTip)
	}
	if _, err := tryGit(r.origin, "merge-base", "--is-ancestor", leftBehind, bareTip); err != nil {
		t.Errorf("the left-behind tip %s is not an ancestor of the published tip %s", leftBehind, bareTip)
	}
	requireSharedLayoutRestored(t, r)
}

// TestIntegrationRepoVisibilitySharedRefusesUnrelatedOriginBranch proves an
// unrelated docket branch already on origin refuses the publish, and nothing
// is renamed or pushed.
func TestIntegrationRepoVisibilitySharedRefusesUnrelatedOriginBranch(t *testing.T) {
	r, _ := newBornPrivateRepo(t)
	foreign := filepath.Join(r.root, "foreign")
	runGit(t, r.root, "init", "-q", "-b", layout.SharedName, foreign)
	gitIdentity(t, foreign)
	writeRepoFile(t, foreign, "other.md", "another backlog\n")
	runGit(t, foreign, "add", "-A")
	runGit(t, foreign, "commit", "-q", "-m", "another backlog")
	runGit(t, foreign, "push", "-q", r.origin, layout.SharedName+":refs/heads/"+layout.SharedName)
	foreignTip := runGit(t, foreign, "rev-parse", "HEAD")
	privateDir := filepath.Join(r.gitDir(t), layout.PrivateName)
	stateBefore := treeListing(t, privateDir)
	before := runGit(t, r.invocation, "rev-parse", "HEAD")

	res := r.switchVisibility(t, SetVisibilityOptions{Target: "shared"})
	requireVisibilityRefusal(t, res, "two backlogs are never merged", foreignTip)
	if got := r.originTip(t, layout.SharedName); got != foreignTip {
		t.Errorf("origin's docket moved to %s, want the unrelated tip %s kept", got, foreignTip)
	}
	if after := treeListing(t, privateDir); after != stateBefore {
		t.Errorf("the private state folder changed:\nbefore:\n%s\nafter:\n%s", stateBefore, after)
	}
	if _, err := os.Stat(filepath.Join(r.gitDir(t), layout.SharedName)); !os.IsNotExist(err) {
		t.Errorf("the shared state folder exists after the refusal (stat err %v)", err)
	}
	if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != before {
		t.Errorf("HEAD moved to %s after the refusal", got)
	}
}

// TestIntegrationRepoVisibilitySharedRestoresLocalKeys proves a clone-local key
// survives shared -> private -> shared in .docket.local.yml (with no
// visibility), and never lands in .docket.yml. The going-shared run is
// interrupted while the repository still reads private, after .docket.yml is
// written: no .docket.local.yml may exist yet (a private repository refuses
// one beside its config), so the re-run resumes and completes.
func TestIntegrationRepoVisibilitySharedRestoresLocalKeys(t *testing.T) {
	r, _ := newPrivateSwitchRepo(t)
	writeRepoFile(t, r.invocation, localConfigRel, privateSwitchLocalYML)
	requireSwitchApplied(t, r.switchVisibility(t, SetVisibilityOptions{Target: "private"}), "private")
	interrupted := r.switchVisibilityWithHooks(t, SetVisibilityOptions{Target: "shared"}, setupHooks{
		afterVisibilityPhase: func(phase string) error {
			if phase == "config-committed" {
				return errors.New("killed after config-committed")
			}
			return nil
		},
	})
	if interrupted.Result != ResultExternalFailed {
		t.Fatalf("interrupted run = %q (%s), want external-failed", interrupted.Result, interrupted.HumanText())
	}
	requireSwitchApplied(t, r.switchVisibility(t, SetVisibilityOptions{Target: "shared"}), "shared")

	local := mustReadFile(t, filepath.Join(r.invocation, localConfigRel))
	vals, err := reposetup.ConfigLeafValues(local, []string{"reclaim.auto", "visibility"})
	if err != nil {
		t.Fatal(err)
	}
	if vals["reclaim.auto"] != "true" {
		t.Errorf(".docket.local.yml = %q, want reclaim.auto: true restored", local)
	}
	if _, ok := vals["visibility"]; ok {
		t.Errorf(".docket.local.yml = %q, want no visibility", local)
	}
	for _, yml := range []string{
		string(mustReadFile(t, filepath.Join(r.invocation, ".docket.yml"))),
		runGit(t, r.invocation, "show", "HEAD:.docket.yml"),
	} {
		if strings.Contains(yml, "reclaim") {
			t.Errorf(".docket.yml carries the clone-local key:\n%s", yml)
		}
	}
	if tracked := runGit(t, r.invocation, "ls-files", "--", localConfigRel); tracked != "" {
		t.Errorf(".docket.local.yml is tracked: %q", tracked)
	}
	if out := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all"); out != "" {
		t.Errorf("the working tree is not clean after the round trip:\n%s", out)
	}
	requireSharedLayoutRestored(t, r)
}

// --- acceptance --------------------------------------------------------------

// visibilityIdentityRemedy is the remedy a going-shared run names when its
// identity-keys commit must reach origin's default branch first.
const visibilityIdentityRemedy = "get this commit onto main on origin"

// driveVisibility switches r to o.Target, pushing main whenever the switch stops
// for its identity-keys commit, until the repository lands in the target mode.
// A preview that is already a no-op counts as landed.
func (r *initRepo) driveVisibility(t *testing.T, o SetVisibilityOptions) RepositorySetVisibilityResult {
	t.Helper()
	for i := 0; i < 4; i++ {
		preview := r.runSetVisibility(t, o)
		if preview.Result == ResultNoOp {
			return preview
		}
		if !preview.ConfirmationRequired() {
			if strings.Contains(preview.HumanText(), visibilityIdentityRemedy) {
				runGit(t, r.invocation, "push", "-q", "origin", "main")
				continue
			}
			t.Fatalf("preview = %q state %q (%s), want confirmation-required", preview.Result, preview.RepositoryState, preview.HumanText())
		}
		run := o
		run.Authorized, run.ExpectedSource = true, preview.SourceRev()
		res := r.runSetVisibility(t, run)
		if res.Result == ResultApplied && res.RepositoryState == o.Target {
			return res
		}
		if res.Result == ResultInvalidState && strings.Contains(res.HumanText(), visibilityIdentityRemedy) {
			runGit(t, r.invocation, "push", "-q", "origin", "main")
			continue
		}
		t.Fatalf("switch = %q state %q, want applied %s:\n%s", res.Result, res.RepositoryState, o.Target, res.HumanText())
	}
	t.Fatalf("the switch to %s did not land in four runs", o.Target)
	return RepositorySetVisibilityResult{}
}

// metadataTreeAt lists every path and blob of commit in dir's repository.
func metadataTreeAt(t *testing.T, dir, commit string) string {
	t.Helper()
	return runGit(t, dir, "ls-tree", "-r", commit)
}

// TestIntegrationRepoVisibilitySwitchRoundTripPreservesRecords proves shared ->
// private (both flags) -> shared publishes back the identical metadata history:
// origin's docket tip and tree are what they were, and replaying an earlier
// change create and claim returns their original outcomes without a commit.
func TestIntegrationRepoVisibilitySwitchRoundTripPreservesRecords(t *testing.T) {
	r, _ := newPrivateSwitchRepo(t)
	ctx := context.Background()
	created := createSwitchChange(t, r.invocation)
	claimID := created.ID + 1
	recPath := groomPath(claimID, "widget")
	dotDocket := filepath.Join(r.invocation, layout.SharedWorktreeDir)
	runGit(t, dotDocket, "fetch", "-q", "origin", layout.SharedName)
	runGit(t, dotDocket, "merge", "-q", "--ff-only", "FETCH_HEAD")
	r.writeDocketFileAndPush(t, recPath, buildReadyChange(claimID, "widget"), "a build-ready change to claim")
	node := planningDepsFor(t, r.invocation)
	bundle := ContextImplementation(ctx, node.deps, node.dir, ImplementationContextRequest{ID: claimID})
	if bundle.Result != ResultApplied || bundle.Context == nil {
		t.Fatalf("context read = %q (reason %q); want a bundle", bundle.Result, bundle.Reason)
	}
	claimReq := ChangeClaimRequest{ID: claimID, Revision: bundle.Context.Change.Revision}
	if claim := ChangeClaim(ctx, node.deps, node.dir, claimReq); claim.Result != ResultApplied || claim.Disposition != ClaimDispositionApplied {
		t.Fatalf("claim = (%q, %q), want applied/applied (findings %v)", claim.Result, claim.Disposition, claim.Findings)
	}
	tip := r.originTip(t, layout.SharedName)
	tree := metadataTreeAt(t, r.origin, tip)

	r.driveVisibility(t, SetVisibilityOptions{Target: "private", DeleteSharedBranch: true, RemoveSharedFiles: true})
	runGit(t, r.invocation, "push", "-q", "origin", "main")
	if r.remoteBranchExists(t, layout.SharedName) {
		t.Fatal("origin still has the docket branch after going private with --delete-shared-branch")
	}
	r.driveVisibility(t, SetVisibilityOptions{Target: "shared"})

	if got := r.originTip(t, layout.SharedName); got != tip {
		t.Fatalf("origin's docket = %s after the round trip, want the original tip %s", got, tip)
	}
	if got := metadataTreeAt(t, r.origin, tip); got != tree {
		t.Errorf("the metadata tree changed:\nbefore:\n%s\nafter:\n%s", tree, got)
	}

	node = planningDepsFor(t, r.invocation)
	replay := ChangeCreate(ctx, node.deps, node.dir, validChangeCreateRequest())
	if replay.Result != ResultApplied || !replay.Replayed || replay.ID != created.ID {
		t.Errorf("create replay = %q replayed=%v id=%d (findings %v), want a replay of %d", replay.Result, replay.Replayed, replay.ID, replay.Findings, created.ID)
	}
	claimReplay := ChangeClaim(ctx, node.deps, node.dir, claimReq)
	if claimReplay.Result != ResultApplied || claimReplay.Disposition != ClaimDispositionAlreadyClaimed {
		t.Errorf("claim replay = (%q, %q), want applied/already-claimed (findings %v)", claimReplay.Result, claimReplay.Disposition, claimReplay.Findings)
	}
	if got := r.originTip(t, layout.SharedName); got != tip {
		t.Errorf("a replay moved origin's docket %s -> %s", tip, got)
	}
}

// visibilityFinalState summarizes what an uninterrupted switch and a resumed
// one must agree on, independent of the fixture's temp paths and object ids:
// which state folders exist, the worktree registrations, every local branch,
// remote, and origin/store branch, how often each fixed subject is on main, the
// working tree status, and whether the metadata tip is still metaTip.
func visibilityFinalState(t *testing.T, r *initRepo, data, metaTip string) string {
	t.Helper()
	gitDir := r.gitDir(t)
	var b strings.Builder
	for _, name := range []string{layout.SharedName, layout.PrivateName} {
		_, err := os.Stat(filepath.Join(gitDir, name))
		b.WriteString("state-folder " + name + " " + strconv.FormatBool(err == nil) + "\n")
	}
	worktrees := 0
	for _, line := range strings.Split(runGit(t, r.invocation, "worktree", "list", "--porcelain"), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			worktrees++
		}
	}
	b.WriteString("worktrees " + strconv.Itoa(worktrees) + "\n")
	b.WriteString("local " + runGit(t, r.invocation, "for-each-ref", "--format=%(refname:short)", "refs/heads") + "\n")
	b.WriteString("remotes " + runGit(t, r.invocation, "remote") + "\n")
	b.WriteString("origin " + runGit(t, r.origin, "for-each-ref", "--format=%(refname:short)", "refs/heads") + "\n")
	store := expectedPrivateLayout(t, r.invocation, data).DefaultBareRemote
	if _, err := os.Stat(store); err == nil {
		b.WriteString("store " + runGit(t, store, "for-each-ref", "--format=%(refname:short)", "refs/heads") + "\n")
		if got, err := tryGit(store, "rev-parse", "--verify", "--quiet", "refs/heads/"+layout.PrivateName); err == nil {
			b.WriteString("store-tip-unchanged " + strconv.FormatBool(got == metaTip) + "\n")
		}
	}
	if r.remoteBranchExists(t, layout.SharedName) {
		b.WriteString("origin-tip-unchanged " + strconv.FormatBool(r.originTip(t, layout.SharedName) == metaTip) + "\n")
	}
	subjects := strings.Split(runGit(t, r.invocation, "log", "--format=%s", "main"), "\n")
	for _, s := range []string{visibilityRemoveSubject, visibilityAddSubject} {
		n := 0
		for _, got := range subjects {
			if got == s {
				n++
			}
		}
		b.WriteString("subject " + s + " " + strconv.Itoa(n) + "\n")
	}
	b.WriteString("status " + runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all") + "\n")
	return b.String()
}

// requireNoSwitchDebris asserts the state folders are never both present, no
// journal is left, and the shared state folder holds no private config files.
func requireNoSwitchDebris(t *testing.T, r *initRepo) {
	t.Helper()
	gitDir := r.gitDir(t)
	_, serr := os.Stat(filepath.Join(gitDir, layout.SharedName))
	_, perr := os.Stat(filepath.Join(gitDir, layout.PrivateName))
	if serr == nil && perr == nil {
		t.Error("both state folders exist")
	}
	for _, folder := range []string{layout.SharedName, layout.PrivateName} {
		if _, err := os.Stat(filepath.Join(gitDir, folder, switchJournalFile)); !os.IsNotExist(err) {
			t.Errorf("the journal is left in %s (stat err %v)", folder, err)
		}
	}
	for _, name := range []string{layout.PrivateConfigFile, layout.PrivateLocalKeysFile} {
		if _, err := os.Stat(filepath.Join(gitDir, layout.SharedName, name)); !os.IsNotExist(err) {
			t.Errorf("%s is left in the shared state folder (stat err %v)", name, err)
		}
	}
}

// interruptAt is a hook that fails the run right after phase.
func interruptAt(phase string) setupHooks {
	return setupHooks{afterVisibilityPhase: func(p string) error {
		if p == phase {
			return errors.New("killed after " + phase)
		}
		return nil
	}}
}

// visibilityDirection is one direction of the interruption table: a fixture
// builder returning the repository, its data home, and its metadata tip, and
// the options of the switch.
type visibilityDirection struct {
	name    string
	fixture func(t *testing.T) (*initRepo, string, string)
	o       SetVisibilityOptions
}

// TestIntegrationRepoVisibilityPrivateInterruptedPhasesResume proves a switch
// to private killed right after any phase completes on the next run and lands
// in exactly the state an uninterrupted switch does: the same folders,
// worktree registrations, branches, and remotes, each fixed subject once, the
// metadata tip unchanged, and no switch debris.
func TestIntegrationRepoVisibilityPrivateInterruptedPhasesResume(t *testing.T) {
	requireInterruptedPhasesResume(t, visibilityDirection{
		name: "private",
		fixture: func(t *testing.T) (*initRepo, string, string) {
			r, data := newPrivateSwitchRepo(t)
			return r, data, r.originTip(t, layout.SharedName)
		},
		o: SetVisibilityOptions{Target: "private", DeleteSharedBranch: true, RemoveSharedFiles: true},
	})
}

// TestIntegrationRepoVisibilitySharedInterruptedPhasesResume is the shared
// direction of the same proof, plus the identity-keys phase that is pending
// only when the private config sets a repository identity key.
func TestIntegrationRepoVisibilitySharedInterruptedPhasesResume(t *testing.T) {
	requireInterruptedPhasesResume(t, visibilityDirection{
		name: "shared",
		fixture: func(t *testing.T) (*initRepo, string, string) {
			r, data := newBornPrivateRepo(t)
			return r, data, runGit(t, expectedPrivateLayout(t, r.invocation, data).DefaultBareRemote, "rev-parse", "refs/heads/"+layout.PrivateName)
		},
		o: SetVisibilityOptions{Target: "shared"},
	})

	// identity-keys is pending only when the private config sets a repository
	// identity key; a kill right after its commit still resumes once that
	// commit reaches origin.
	t.Run("identity-keys", func(t *testing.T) {
		r, _ := newBornPrivateRepo(t)
		cfgPath := filepath.Join(r.gitDir(t), layout.PrivateName, layout.PrivateConfigFile)
		if err := os.WriteFile(cfgPath, append(mustReadFile(t, cfgPath), []byte("integration_branch: main\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		before := runGit(t, r.invocation, "rev-parse", "HEAD")
		first := r.switchVisibilityWithHooks(t, SetVisibilityOptions{Target: "shared"}, interruptAt("identity-keys"))
		if first.Result != ResultExternalFailed {
			t.Fatalf("run 1 = %q (%s), want external-failed", first.Result, first.HumanText())
		}
		r.driveVisibility(t, SetVisibilityOptions{Target: "shared"})
		requireNoSwitchDebris(t, r)
		requireSharedLayoutRestored(t, r)
		if n := runGit(t, r.invocation, "rev-list", "--count", before+"..HEAD"); n != "2" {
			t.Errorf("%s new commits, want the identity commit and the final commit", n)
		}
	})
}

// requireInterruptedPhasesResume runs dir's switch once uninterrupted for the
// reference state and its pending phases, then once per pending phase killed
// right after that phase and resumed, requiring the resumed state to equal the
// reference.
func requireInterruptedPhasesResume(t *testing.T, dir visibilityDirection) {
	t.Helper()
	var want string
	var pending []string
	t.Run("uninterrupted", func(t *testing.T) {
		r, data, tip := dir.fixture(t)
		preview := r.runSetVisibility(t, dir.o)
		for _, p := range preview.Phases {
			if p.Status == visibilityPhasePending {
				pending = append(pending, p.Name)
			}
		}
		requireSwitchApplied(t, r.switchVisibility(t, dir.o), dir.o.Target)
		requireNoSwitchDebris(t, r)
		want = visibilityFinalState(t, r, data, tip)
	})
	if want == "" || len(pending) < 5 {
		t.Fatalf("the uninterrupted %s switch left no reference state (pending phases %q)", dir.name, pending)
	}
	for _, phase := range pending {
		t.Run(phase, func(t *testing.T) {
			r, data, tip := dir.fixture(t)
			first := r.switchVisibilityWithHooks(t, dir.o, interruptAt(phase))
			if first.Result != ResultExternalFailed {
				t.Fatalf("run 1 = %q (%s), want external-failed at %s", first.Result, first.HumanText(), phase)
			}
			requireNoSwitchDebrisFolders(t, r)
			r.driveVisibility(t, dir.o)
			requireNoSwitchDebris(t, r)
			if got := visibilityFinalState(t, r, data, tip); got != want {
				t.Errorf("resumed after %s:\n%s\nwant (uninterrupted):\n%s", phase, got, want)
			}
		})
	}
}

// requireNoSwitchDebrisFolders asserts an interrupted run never leaves both
// state folders.
func requireNoSwitchDebrisFolders(t *testing.T, r *initRepo) {
	t.Helper()
	gitDir := r.gitDir(t)
	_, serr := os.Stat(filepath.Join(gitDir, layout.SharedName))
	_, perr := os.Stat(filepath.Join(gitDir, layout.PrivateName))
	if serr == nil && perr == nil {
		t.Error("both state folders exist after the interrupted run")
	}
}

// TestIntegrationRepoVisibilityPrivateResultIsClean proves a repository that
// went private with both flags checks clean: no error, no metadata on the
// shared remote, no visibility mismatch, nothing docket-named at the root, and
// the metadata checkout outside the primary checkout.
func TestIntegrationRepoVisibilityPrivateResultIsClean(t *testing.T) {
	r, data := newPrivateSwitchRepo(t)
	createSwitchChange(t, r.invocation)
	r.driveVisibility(t, SetVisibilityOptions{Target: "private", DeleteSharedBranch: true, RemoveSharedFiles: true})
	// The removal commit is the user's to publish; check reads an unpushed
	// local commit as its own finding.
	runGit(t, r.invocation, "push", "-q", "origin", "main")

	chk := r.runCheck(t)
	for _, f := range chk.Findings {
		if f.Severity == reposetup.SeverityError || f.Code == FindingMetadataOnSharedRemote || f.Code == FindingVisibilityMismatch {
			t.Errorf("check reported %+v", f)
		}
	}
	entries, err := os.ReadDir(r.invocation)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != ".git" && strings.Contains(strings.ToLower(e.Name()), "docket") {
			t.Errorf("the root still holds %s", e.Name())
		}
	}
	checkout := expectedPrivateLayout(t, r.invocation, data).MetadataWorktree
	if rel, err := filepath.Rel(r.invocation, checkout); err != nil || !strings.HasPrefix(rel, "..") {
		t.Errorf("the metadata checkout %s is inside the primary checkout %s", checkout, r.invocation)
	}
	if rel, err := filepath.Rel(canonicalDir(t, r.invocation), canonicalDir(t, checkout)); err != nil || !strings.HasPrefix(rel, "..") {
		t.Errorf("the metadata checkout %s resolves inside the primary checkout %s", checkout, r.invocation)
	}
}

// canonicalDir resolves dir's symlinks.
func canonicalDir(t *testing.T, dir string) string {
	t.Helper()
	out, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestIntegrationRepoVisibilitySwitchCommitMessages proves every commit a switch makes
// carries one of the two fixed subjects and no body, a hand-edited .gitignore
// refuses the going-shared preview, a detached HEAD refuses a committing plan,
// and a commit hook that fails once leaves the edits for a re-run that makes
// exactly one commit.
func TestIntegrationRepoVisibilitySwitchCommitMessages(t *testing.T) {
	t.Run("round trip subjects", func(t *testing.T) {
		r, _ := newPrivateSwitchRepo(t)
		before := runGit(t, r.invocation, "rev-parse", "HEAD")
		r.driveVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true})
		r.driveVisibility(t, SetVisibilityOptions{Target: "shared"})
		commits := strings.Fields(runGit(t, r.invocation, "rev-list", before+"..HEAD"))
		if len(commits) < 2 {
			t.Fatalf("%d switch commits, want at least the removal and the addition", len(commits))
		}
		for _, c := range commits {
			subject := runGit(t, r.invocation, "log", "-1", "--format=%s", c)
			if subject != visibilityRemoveSubject && subject != visibilityAddSubject {
				t.Errorf("commit %s subject = %q, want one of the two fixed subjects", c, subject)
			}
			if body := runGit(t, r.invocation, "log", "-1", "--format=%b", c); body != "" {
				t.Errorf("commit %s body = %q, want none", c, body)
			}
		}
	})

	t.Run("hand-edited gitignore refuses", func(t *testing.T) {
		r, _ := newPrivateSwitchRepo(t)
		r.driveVisibility(t, SetVisibilityOptions{Target: "private"})
		ign := filepath.Join(r.invocation, ".gitignore")
		if err := os.WriteFile(ign, append(mustReadFile(t, ign), []byte("scratch/\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		res := r.runSetVisibility(t, SetVisibilityOptions{Target: "shared"})
		requireVisibilityRefusal(t, res, "commit or set aside these edits", ".gitignore")
	})

	t.Run("detached HEAD refuses", func(t *testing.T) {
		r, _ := newPrivateSwitchRepo(t)
		runGit(t, r.invocation, "checkout", "-q", "--detach")
		res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true})
		requireVisibilityRefusal(t, res, "detached HEAD")
	})

	t.Run("hook fails once", func(t *testing.T) {
		r, _ := newPrivateSwitchRepo(t)
		gitDir := r.gitDir(t)
		marker := filepath.Join(testsupport.TempDir(t), "rejected-once")
		hook := "#!/bin/sh\nif [ -e '" + marker + "' ]; then exit 0; fi\n: > '" + marker + "'\nexit 1\n"
		if err := os.MkdirAll(filepath.Join(gitDir, "hooks"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "hooks", "commit-msg"), []byte(hook), 0o755); err != nil {
			t.Fatal(err)
		}
		before := runGit(t, r.invocation, "rev-parse", "HEAD")
		o := SetVisibilityOptions{Target: "private", RemoveSharedFiles: true}
		first := r.switchVisibility(t, o)
		if first.Result != ResultExternalFailed {
			t.Fatalf("run 1 = %q (%s), want external-failed on the hook's rejection", first.Result, first.HumanText())
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("the commit hook did not run: %v", err)
		}
		if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != before {
			t.Fatalf("HEAD moved to %s on a rejected commit", got)
		}
		if _, err := os.Stat(filepath.Join(r.invocation, ".docket.yml")); !os.IsNotExist(err) {
			t.Errorf("the journaled deletion of .docket.yml was not kept (stat err %v)", err)
		}
		if len(first.Commits) != 1 || first.Commits[0].Status != visibilityCommitFailed {
			t.Errorf("Commits = %+v, want one failed row", first.Commits)
		}
		requireSwitchApplied(t, r.switchVisibility(t, o), "private")
		requireRemovalCommit(t, r, before)
	})
}

// TestIntegrationRepoVisibilitySwitchUserEditsSurvive proves a staged unrelated file
// and an unstaged edit to a tracked file outside the switch's paths survive
// both directions, never entering a switch commit.
func TestIntegrationRepoVisibilitySwitchUserEditsSurvive(t *testing.T) {
	r, _ := newPrivateSwitchRepo(t)
	writeRepoFile(t, r.invocation, "notes.txt", "mine\n")
	runGit(t, r.invocation, "add", "--", "notes.txt")
	writeRepoFile(t, r.invocation, "README.md", "readme\nmy unstaged edit\n")
	before := runGit(t, r.invocation, "rev-parse", "HEAD")

	requireEdits := func(when string) {
		t.Helper()
		if staged := runGit(t, r.invocation, "diff", "--cached", "--name-only"); staged != "notes.txt" {
			t.Errorf("%s: staged = %q, want notes.txt", when, staged)
		}
		if unstaged := runGit(t, r.invocation, "diff", "--name-only"); unstaged != "README.md" {
			t.Errorf("%s: unstaged = %q, want README.md", when, unstaged)
		}
		if got := string(mustReadFile(t, filepath.Join(r.invocation, "README.md"))); got != "readme\nmy unstaged edit\n" {
			t.Errorf("%s: README.md = %q, want the user's edit", when, got)
		}
		for _, c := range strings.Fields(runGit(t, r.invocation, "rev-list", before+"..HEAD")) {
			for p := range commitNameStatus(t, r.invocation, c) {
				if p == "notes.txt" || p == "README.md" {
					t.Errorf("%s: switch commit %s holds the user's %s", when, c, p)
				}
			}
		}
	}
	r.driveVisibility(t, SetVisibilityOptions{Target: "private", RemoveSharedFiles: true})
	requireEdits("after going private")
	r.driveVisibility(t, SetVisibilityOptions{Target: "shared"})
	requireEdits("after going shared")
}

// TestIntegrationRepoVisibilitySwitchSecondCloneShared proves two clones sharing one
// private store: once the first goes shared, the second's publish is already
// done and it completes with local phases only, leaving origin and the store
// untouched.
func TestIntegrationRepoVisibilitySwitchSecondCloneShared(t *testing.T) {
	a, data := newPrivateSwitchRepo(t)
	b := &initRepo{root: a.root, origin: a.origin, writer: a.writer, invocation: filepath.Join(a.root, "second")}
	runGit(t, a.root, "clone", "-q", a.origin, b.invocation)
	gitIdentity(t, b.invocation)
	if res := b.runInitWith(t, InitOptions{}); res.Result != ResultApplied && res.Result != ResultNoOp {
		t.Fatalf("second clone init = %q (%s)", res.Result, res.HumanText())
	}
	a.driveVisibility(t, SetVisibilityOptions{Target: "private"})
	b.driveVisibility(t, SetVisibilityOptions{Target: "private"})
	if expectedPrivateLayout(t, a.invocation, data).DefaultBareRemote != expectedPrivateLayout(t, b.invocation, data).DefaultBareRemote {
		t.Fatal("the two clones do not share one private store")
	}
	a.driveVisibility(t, SetVisibilityOptions{Target: "shared"})

	store := expectedPrivateLayout(t, b.invocation, data).DefaultBareRemote
	originRefs := runGit(t, b.origin, "for-each-ref", "--format=%(refname) %(objectname)")
	storeRefs := runGit(t, store, "for-each-ref", "--format=%(refname) %(objectname)")
	preview := b.runSetVisibility(t, SetVisibilityOptions{Target: "shared"})
	if !preview.ConfirmationRequired() {
		t.Fatalf("second clone preview = %q (%s), want confirmation-required", preview.Result, preview.HumanText())
	}
	for _, name := range []string{"identity-keys", "publish"} {
		if got := phaseStatus(preview, name); got != visibilityPhaseDone {
			t.Errorf("%s = %q, want done (the first clone already published)", name, got)
		}
	}
	res := b.runSetVisibility(t, SetVisibilityOptions{Target: "shared", Authorized: true, ExpectedSource: preview.SourceRev()})
	requireSwitchApplied(t, res, "shared")
	if got := runGit(t, b.origin, "for-each-ref", "--format=%(refname) %(objectname)"); got != originRefs {
		t.Errorf("the second clone's switch changed origin:\nbefore:\n%s\nafter:\n%s", originRefs, got)
	}
	if got := runGit(t, store, "for-each-ref", "--format=%(refname) %(objectname)"); got != storeRefs {
		t.Errorf("the second clone's switch changed the store:\nbefore:\n%s\nafter:\n%s", storeRefs, got)
	}
	requireSharedLayoutRestored(t, b)
	requireNoSwitchDebris(t, b)
}

// TestIntegrationRepoVisibilitySwitchSameModeIsAlignOnly proves a switch to the mode a
// never-switched repository is already in only aligns the clone-local
// visibility value: a freshly initialized shared repository with uncommitted
// edits to the switch's own paths (init's unstaged .gitignore block, a dirty
// AGENTS.md) neither refuses nor commits, and leaves every edit as it was; a
// born-private repository switched to private is likewise a no-op.
func TestIntegrationRepoVisibilitySwitchSameModeIsAlignOnly(t *testing.T) {
	newDirtyShared := func(t *testing.T) *initRepo {
		t.Helper()
		t.Setenv("XDG_DATA_HOME", testsupport.TempDir(t))
		r := newInitRepo(t, privateSwitchYML, nil)
		if res := r.runInitWith(t, InitOptions{}); res.Result != ResultApplied {
			t.Fatalf("init = %q (%s), want applied", res.Result, res.HumanText())
		}
		agents := filepath.Join(r.invocation, "AGENTS.md")
		writeRepoFile(t, r.invocation, "AGENTS.md", string(mustReadFile(t, agents))+"\nmy own note\n")
		return r
	}
	requireUntouched := func(t *testing.T, r *initRepo, head, status, agents string) {
		t.Helper()
		if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != head {
			t.Errorf("HEAD moved %s -> %s; a same-mode switch must not commit", head, got)
		}
		if got := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all"); got != status {
			t.Errorf("the working tree changed:\nbefore:\n%s\nafter:\n%s", status, got)
		}
		if got := string(mustReadFile(t, filepath.Join(r.invocation, "AGENTS.md"))); got != agents {
			t.Errorf("AGENTS.md changed:\nbefore:\n%s\nafter:\n%s", agents, got)
		}
		if _, err := os.Stat(filepath.Join(r.gitDir(t), layout.SharedName, switchJournalFile)); !os.IsNotExist(err) {
			t.Errorf("a switch journal was left (stat err %v)", err)
		}
	}

	t.Run("shared to shared is a no-op", func(t *testing.T) {
		r := newDirtyShared(t)
		head := runGit(t, r.invocation, "rev-parse", "HEAD")
		status := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all")
		agents := string(mustReadFile(t, filepath.Join(r.invocation, "AGENTS.md")))
		if preview := r.runSetVisibility(t, SetVisibilityOptions{Target: "shared"}); preview.Result != ResultNoOp {
			t.Errorf("preview = %q state %q, want no-op:\n%s", preview.Result, preview.RepositoryState, preview.HumanText())
		}
		if res := r.runSetVisibility(t, SetVisibilityOptions{Target: "shared", Authorized: true}); res.Result != ResultNoOp {
			t.Errorf("--yes = %q state %q, want no-op:\n%s", res.Result, res.RepositoryState, res.HumanText())
		}
		requireUntouched(t, r, head, status, agents)
	})

	t.Run("shared to shared aligns only the local value", func(t *testing.T) {
		r := newDirtyShared(t)
		writeRepoFile(t, r.invocation, ".docket.local.yml", "visibility: private\n")
		head := runGit(t, r.invocation, "rev-parse", "HEAD")
		agents := string(mustReadFile(t, filepath.Join(r.invocation, "AGENTS.md")))
		res := r.runSetVisibility(t, SetVisibilityOptions{Target: "shared", Authorized: true})
		if res.Result != ResultApplied || res.RepositoryState != "shared" {
			t.Fatalf("--yes = %q state %q, want applied shared:\n%s", res.Result, res.RepositoryState, res.HumanText())
		}
		for _, p := range res.Phases {
			want := visibilityPhaseDone
			if p.Name == "align-visibility" {
				want = visibilityPhaseApplied
			}
			if p.Status != want {
				t.Errorf("phase %s = %q, want %q", p.Name, p.Status, want)
			}
		}
		if len(res.Commits) != 0 {
			t.Errorf("Commits = %+v, want none", res.Commits)
		}
		if got := string(mustReadFile(t, filepath.Join(r.invocation, ".docket.local.yml"))); !strings.Contains(got, "visibility: shared") {
			t.Errorf(".docket.local.yml = %q, want visibility: shared", got)
		}
		status := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all")
		requireUntouched(t, r, head, status, agents)
	})

	t.Run("private to private is a no-op", func(t *testing.T) {
		r, _ := newBornPrivateRepo(t)
		head := runGit(t, r.invocation, "rev-parse", "HEAD")
		status := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all")
		if preview := r.runSetVisibility(t, SetVisibilityOptions{Target: "private"}); preview.Result != ResultNoOp {
			t.Errorf("preview = %q state %q, want no-op:\n%s", preview.Result, preview.RepositoryState, preview.HumanText())
		}
		if res := r.runSetVisibility(t, SetVisibilityOptions{Target: "private", Authorized: true}); res.Result != ResultNoOp {
			t.Errorf("--yes = %q state %q, want no-op:\n%s", res.Result, res.RepositoryState, res.HumanText())
		}
		if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != head {
			t.Errorf("HEAD moved %s -> %s", head, got)
		}
		if got := runGit(t, r.invocation, "status", "--porcelain", "--untracked-files=all"); got != status {
			t.Errorf("the working tree changed:\nbefore:\n%s\nafter:\n%s", status, got)
		}
	})
}

// TestIntegrationRepoVisibilitySharedResumesCommitWithoutJournaledEdits proves
// a going-shared run killed after its metadata checkout moved still resumes its
// commit when no earlier phase journaled an edit: the repository went private
// keeping its committed files, .docket.yml already holds what the private
// configuration splits into, and the user dropped the dispatch block from
// AGENTS.md. The resumed run reinstalls the block and commits it with the add
// subject instead of reading the repository as never switched.
func TestIntegrationRepoVisibilitySharedResumesCommitWithoutJournaledEdits(t *testing.T) {
	r, _ := newPrivateSwitchRepo(t)
	r.driveVisibility(t, SetVisibilityOptions{Target: "private"})
	gitDir := r.gitDir(t)
	cfg := mustReadFile(t, filepath.Join(gitDir, layout.PrivateName, layout.PrivateConfigFile))
	keys, _, err := readOptionalFile(filepath.Join(gitDir, layout.PrivateName, layout.PrivateLocalKeysFile))
	if err != nil {
		t.Fatal(err)
	}
	committed, _, err := reposetup.SplitPrivateConfig(cfg, keys, config.RepoOnlyPaths())
	if err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, r.invocation, ".docket.yml", string(committed))
	writeRepoFile(t, r.invocation, "AGENTS.md", "# Agents\n")
	runGit(t, r.invocation, "add", "--", ".docket.yml", "AGENTS.md")
	runGit(t, r.invocation, "commit", "-q", "-m", "hand edits while private")
	before := runGit(t, r.invocation, "rev-parse", "HEAD")

	first := r.switchVisibilityWithHooks(t, SetVisibilityOptions{Target: "shared"}, interruptAt("metadata-worktree"))
	if first.Result != ResultExternalFailed {
		t.Fatalf("run 1 = %q (%s), want external-failed after metadata-worktree", first.Result, first.HumanText())
	}
	if got := phaseStatus(first, "config-committed"); got != visibilityPhaseDone {
		t.Fatalf("config-committed = %q, want done (the fixture must journal no edit)", got)
	}
	r.driveVisibility(t, SetVisibilityOptions{Target: "shared"})
	requireSharedLayoutRestored(t, r)
	requireNoSwitchDebris(t, r)
	if n := runGit(t, r.invocation, "rev-list", "--count", before+"..HEAD"); n != "1" {
		t.Fatalf("%s new commits, want the one add commit", n)
	}
	requireAddCommit(t, r.invocation, "HEAD", "AGENTS.md")
	if agents := runGit(t, r.invocation, "show", "HEAD:AGENTS.md"); !strings.Contains(agents, document.MarkerSpelling(instructionsBlockName)+":start") {
		t.Errorf("the committed AGENTS.md lacks the dispatch block:\n%s", agents)
	}
}
