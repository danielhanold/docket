package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/testsupport"
	"github.com/danielhanold/docket/internal/workspace"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file drives `finalize cleanup` and `gate cleanup` over REAL Git topology
// (the same bare-remote main/docket matrix the other finalize integration tests
// use) plus recording fakes for the GitHub and process seams a hermetic suite
// cannot reach. Cleanup is destructive ref/worktree/run-dir deletion where every
// leg must fail closed on absent/unknown proof: the tests exist to prove a
// resource is RETAINED — never destroyed — on every probe error, moved ref,
// live lock, unproven ancestry, open child, or lease rejection.

// --- fake GitHub for cleanup ----------------------------------------------

// fakeCleanupGitHub answers the GitHub calls `finalize cleanup` makes:
// DiscoverRepository, ProbeMerged (the closing reprobe), and
// FindOpenPullRequestsByHead (the fresh no-open-child probe). Every other
// finalize-half method panics so an accidental call is loud.
type fakeCleanupGitHub struct {
	repo       githubcli.Repository
	merged     map[int]closeoutProbe
	probeErr   error
	openByHead map[string][]githubcli.PullRequest
	findErr    error
}

func (f *fakeCleanupGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return f.repo, nil
}
func (f *fakeCleanupGitHub) ProbeMerged(_ context.Context, _ githubcli.Repository, number int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	if f.probeErr != nil {
		return githubcli.MergeUnknown, githubcli.MergedFacts{}, f.probeErr
	}
	p, ok := f.merged[number]
	if !ok {
		return githubcli.MergeNotMergeable, githubcli.MergedFacts{}, nil
	}
	return p.outcome, p.facts, nil
}
func (f *fakeCleanupGitHub) ViewPullRequest(context.Context, githubcli.Repository, int) (githubcli.PullRequest, error) {
	panic("ViewPullRequest: cleanup must not call this")
}
func (f *fakeCleanupGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, head string) ([]githubcli.PullRequest, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.openByHead[head], nil
}
func (f *fakeCleanupGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: cleanup must not call this")
}
func (f *fakeCleanupGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: cleanup must not call this")
}
func (f *fakeCleanupGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: cleanup must not call this")
}
func (f *fakeCleanupGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeResult, error) {
	panic("MergePullRequest: cleanup must not call this")
}

// --- fault-injection seams ------------------------------------------------

// faultyCleanupGit wraps the real client and injects one error into exactly one
// of the branch-deletion probes/effects, so a probe-error test proves the leg is
// retained rather than destructive.
type faultyCleanupGit struct {
	inner *gitcli.Client
	fail  string // "resolve" | "remote-probe" | "list" | "ancestor" | "fetch"
}

var errCleanupProbe = errors.New("injected cleanup probe failure")

func (g *faultyCleanupGit) ResolveRef(ctx context.Context, repo gitcli.Repository, ref gitcli.RefName) (gitcli.ObjectID, error) {
	if g.fail == "resolve" {
		return "", errCleanupProbe
	}
	return g.inner.ResolveRef(ctx, repo, ref)
}
func (g *faultyCleanupGit) ProbeRemoteBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName) (gitcli.RemoteRef, error) {
	if g.fail == "remote-probe" {
		return gitcli.RemoteRef{}, errCleanupProbe
	}
	return g.inner.ProbeRemoteBranch(ctx, repo, remote, ref)
}
func (g *faultyCleanupGit) ListWorktrees(ctx context.Context, repo gitcli.Repository) ([]gitcli.WorktreeInfo, error) {
	if g.fail == "list" {
		return nil, errCleanupProbe
	}
	return g.inner.ListWorktrees(ctx, repo)
}
func (g *faultyCleanupGit) IsAncestor(ctx context.Context, repo gitcli.Repository, ancestor, descendant gitcli.ObjectID) (bool, error) {
	if g.fail == "ancestor" {
		return false, errCleanupProbe
	}
	return g.inner.IsAncestor(ctx, repo, ancestor, descendant)
}
func (g *faultyCleanupGit) FetchBranch(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (gitcli.Revision, error) {
	if g.fail == "fetch" {
		return gitcli.Revision{}, errCleanupProbe
	}
	return g.inner.FetchBranch(ctx, repo, remote, branch)
}
func (g *faultyCleanupGit) DeleteLocalBranchChecked(ctx context.Context, repo gitcli.Repository, branch gitcli.RefName, expectedTip gitcli.ObjectID) error {
	return g.inner.DeleteLocalBranchChecked(ctx, repo, branch, expectedTip)
}
func (g *faultyCleanupGit) DeleteRemoteRefLease(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName, expectedTip gitcli.ObjectID) (gitcli.PushOutcome, error) {
	return g.inner.DeleteRemoteRefLease(ctx, repo, remote, ref, expectedTip)
}

// faultyCleanupWorkspace wraps the real service and injects a manifest/lock
// probe failure into Cleanup so the workspace-removal leg is retained.
type faultyCleanupWorkspace struct {
	*workspace.Service
	failCleanup bool
}

func (w *faultyCleanupWorkspace) Cleanup(ctx context.Context, req workspace.CleanupRequest) (workspace.CleanupResult, error) {
	if w.failCleanup {
		return workspace.CleanupResult{Disposition: workspace.CleanupFailed}, errCleanupProbe
	}
	return w.Service.Cleanup(ctx, req)
}

// --- fixture helpers ------------------------------------------------------

// archiveClosed drives a real merge into main and a FinalizeCloseout so the
// change reaches done+archived — exactly the final state cleanup consumes.
// It returns the merged head and merge commit.
func (f *closeoutFixture) archiveClosed(t *testing.T) (head, mergeCommit string) {
	t.Helper()
	mergeCommit = f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("archiveClosed: closeout = %q disp %q (%s)", res.Result, res.Disposition, res.Message)
	}
	return f.head, mergeCommit
}

// cleanupDeps assembles the FinalizeDeps a cleanup test drives.
func (f *closeoutFixture) cleanupDeps(gh FinalizeGitHub, git FinalizeCleanupGit, ws FinalizeWorkspace) FinalizeDeps {
	return FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: ws, CleanupGit: git}
}

// mergedCleanupFake returns a cleanup fake whose PR reprobes merged into main at
// the fixture head and the given merge commit, with no open children.
func (f *closeoutFixture) mergedCleanupFake(head, mergeCommit string) *fakeCleanupGitHub {
	return &fakeCleanupGitHub{
		repo:       retargetRepo(),
		merged:     map[int]closeoutProbe{closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(head, "main", mergeCommit)}},
		openByHead: map[string][]githubcli.PullRequest{},
	}
}

// localBranchPresent reports whether the feature branch exists locally.
func (f *closeoutFixture) localBranchPresent(t *testing.T) bool {
	t.Helper()
	_, err := f.deps.Client.ResolveRef(context.Background(), f.gitrepo, gitcli.RefName("refs/heads/feat/"+f.slug))
	return err == nil
}

// remoteBranchPresent reports whether the feature branch exists on origin.
func (f *closeoutFixture) remoteBranchPresent(t *testing.T) bool {
	t.Helper()
	rr, err := f.deps.Client.ProbeRemoteBranch(context.Background(), f.gitrepo, "origin", gitcli.RefName("refs/heads/feat/"+f.slug))
	if err != nil {
		t.Fatalf("remote probe: %v", err)
	}
	return rr.State == gitcli.RemoteRefFound
}

// --- TestFinalizeCleanupOnlyAfterFinal ------------------------------------

// TestFinalLifecycleCodeSpellings pins the ADR-0129 rows 57 and 59 wire
// spellings (change 0474). Drivers and skills key on these strings, not on the
// constant names.
func TestFinalLifecycleCodeSpellings(t *testing.T) {
	for _, c := range []struct{ name, got, want string }{
		{"ReasonCloseoutNotesFrozen", string(ReasonCloseoutNotesFrozen), "final-notes-frozen"},
		{"ReasonCleanupNotFinal", string(ReasonCleanupNotFinal), "not-final"},
		{"ReasonCleanupKilledRetained", string(ReasonCleanupKilledRetained), "killed-retained"},
		{"childOutcomeSkippedDone", string(childOutcomeSkippedDone), "skipped-final"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestAlreadyOutcomeSpellings pins every no-op replay result to the
// already-<what happened> house pattern: a bare "already" never says what
// already happened. Drivers key on these strings, not on the constant names.
func TestAlreadyOutcomeSpellings(t *testing.T) {
	for _, c := range []struct{ name, got, want string }{
		{"CompleteDispAlreadyCompleted", CompleteDispAlreadyCompleted, "already-completed"},
		{"CloseoutDispAlready", CloseoutDispAlready, "already-closed-out"},
		{"BlockDispAlready", BlockDispAlready, "already-recorded"},
		{"childOutcomeAlready", childOutcomeAlready, "already-retargeted"},
		{"prBacklinkAlready", prBacklinkAlready, "already-repointed"},
		{"githubcli.BodyAlready", string(githubcli.BodyAlready), "already-edited"},
		{"githubcli.CommentAlready", string(githubcli.CommentAlready), "already-posted"},
		{"githubcli.RetargetAlready", string(githubcli.RetargetAlready), "already-retargeted"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// --- TestFinalizeCleanupBranchDeletion ------------------------------------

// seedStackChild adds a child record stacked on the fixture change to the
// metadata ref so StackChildren sees it.
func (f *closeoutFixture) seedStackChild(t *testing.T, id int, slug string) {
	t.Helper()
	f.seedStackChildBranch(t, id, slug, "")
}

// seedStackChildBranch is seedStackChild with an optional non-derived recorded
// branch override (empty keeps the claim-minted feat/<slug>).
func (f *closeoutFixture) seedStackChildBranch(t *testing.T, id int, slug, branch string) {
	t.Helper()
	// Advance the metadata branch to origin first: closeout has already pushed the
	// archive commit, so the writer clone's local branch is behind and a plain
	// advance would be rejected (fetch first).
	runGit(t, f.repo.writer, "fetch", "-q", "origin", f.branch)
	runGit(t, f.repo.writer, "checkout", "-q", f.branch)
	runGit(t, f.repo.writer, "reset", "-q", "--hard", "origin/"+f.branch)
	rec := lifecycleChange(id, slug, "in-progress")
	rec = strings.Replace(rec, "stacked_on:\n", "stacked_on: "+itoa(f.id)+"\n", 1)
	if branch != "" {
		rec = strings.Replace(rec, "branch: feat/"+slug+"\n", "branch: "+branch+"\n", 1)
	}
	writeRepoFile(t, f.repo.writer, groomPath(id, slug), rec)
	runGit(t, f.repo.writer, "add", "-A")
	runGit(t, f.repo.writer, "commit", "-q", "-m", "seed child")
	runGit(t, f.repo.writer, "push", "-q", "origin", f.branch)
}

// --- TestFinalizeCleanupStackedRetained -----------------------------------

// setupStackedMergedCleanupFixture builds a closeout fixture whose record is
// stacked-merged (a merged-but-retained state).
func setupStackedMergedCleanupFixture(t *testing.T) *closeoutFixture {
	t.Helper()
	f := setupCloseoutFixture(t, planRepoModeDocket())
	recPath := groomPath(f.id, f.slug)
	src := closeoutRecord(f.id, f.slug, "stacked-merged", closeoutRef, f.specPath, f.planPath, f.resultsPath)
	f.repo.writerAdvance(t, f.branch, map[string]string{recPath: src})
	f.revision = blobRevisionAt(t, f.repo.origin, f.branch, recPath)
	return f
}

// --- TestFinalizeCleanupInjectedProbeErrors -------------------------------

// --- TestFinalizeCleanupRetryable -----------------------------------------

// --- TestGateCleanupRetention ---------------------------------------------

func TestGateCleanupRetention(t *testing.T) {
	t.Run("passed-removed", func(t *testing.T) {
		dir := writeGateRun(t, gateRunSpec{state: "passed"})
		res := GateCleanup(context.Background(), FinalizeDeps{}, dir)
		if res.Result != ResultApplied || res.Disposition != CleanupDispCleaned {
			t.Fatalf("a passed run must be cleaned, got %q disp %q (%s)", res.Result, res.Disposition, res.Message)
		}
		if gateRunLogsPresent(t, dir) {
			t.Fatalf("a cleaned run must have its logs removed")
		}
		// Second call is a no-op via the receipt tombstone.
		again := GateCleanup(context.Background(), FinalizeDeps{}, dir)
		if again.Result != ResultNoOp || again.Disposition != CleanupDispAlready {
			t.Fatalf("second cleanup must be a receipt no-op, got %q disp %q", again.Result, again.Disposition)
		}
	})

	t.Run("failed-retained", func(t *testing.T) {
		dir := writeGateRun(t, gateRunSpec{state: "failed"})
		res := GateCleanup(context.Background(), FinalizeDeps{}, dir)
		if res.Disposition == CleanupDispCleaned {
			t.Fatalf("a failed run must be retained for its diagnostics")
		}
		if !gateRunLogsPresent(t, dir) {
			t.Fatalf("a retained run must keep its logs")
		}
	})

	t.Run("running-retained", func(t *testing.T) {
		dir := writeGateRun(t, gateRunSpec{state: "running"})
		res := GateCleanup(context.Background(), FinalizeDeps{}, dir)
		if res.Disposition == CleanupDispCleaned {
			t.Fatalf("a running run must be retained")
		}
		if !gateRunLogsPresent(t, dir) {
			t.Fatalf("a live run must keep its logs")
		}
	})

	t.Run("foreign-dir-retained", func(t *testing.T) {
		dir := testsupport.TempDir(t)
		res := GateCleanup(context.Background(), FinalizeDeps{}, dir)
		if res.Disposition == CleanupDispCleaned {
			t.Fatalf("a foreign directory must never be cleaned")
		}
	})
}

// --- TestCleanupNeverTouchesForeignTrees ----------------------------------

// --- gate-run fixtures ----------------------------------------------------

type gateRunSpec struct {
	state string // "passed" | "failed" | "running" | "stopped"
}

// writeGateRun writes a minimal but valid process run directory in the required
// state: a manifest whose run id matches the directory name, the logs, and the
// state-selecting records.
func writeGateRun(t *testing.T, spec gateRunSpec) string {
	t.Helper()
	root := testsupport.TempDir(t)
	runID := strings.Repeat("a", 32)
	dir := filepath.Join(root, runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir run: %v", err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("manifest.json", `{"schema":1,"run_id":"`+runID+`","root":"`+root+`","run_dir":"`+dir+`","phase":"terminal","cwd":"`+root+`","argv0":"go","argc":1,"created_at":"2026-08-18T00:00:00Z","updated_at":"2026-08-18T00:00:00Z"}`)
	write("stdout.log", "stdout\n")
	write("stderr.log", "stderr\n")
	write("supervisor.log", "sup\n")
	switch spec.state {
	case "passed":
		write("terminal.json", `{"schema":1,"run_id":"`+runID+`","kind":"exit","exit_code":0,"signal":0,"recorded_at":"2026-08-18T00:00:01Z"}`)
	case "failed":
		write("terminal.json", `{"schema":1,"run_id":"`+runID+`","kind":"exit","exit_code":1,"signal":0,"recorded_at":"2026-08-18T00:00:01Z"}`)
	case "stopped":
		write("stop-intent.json", `{"schema":1,"run_id":"`+runID+`","reason":"halt","recorded_at":"2026-08-18T00:00:01Z"}`)
		write("stopped.json", `{"schema":1,"run_id":"`+runID+`","verified_at":"2026-08-18T00:00:02Z"}`)
	case "running":
		// No terminal record and no live lock: Observe classifies this as vanished,
		// which is retained the same as running for cleanup purposes. To model a
		// genuinely running run without a live supervisor, leave it recordless.
	}
	return dir
}

func gateRunLogsPresent(t *testing.T, dir string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, "stdout.log"))
	return err == nil
}

// planRepoModeDocket returns the docket metadata mode, the one supported
// topology, where the metadata branch is a genuinely different ref from the
// integration branch.
func planRepoModeDocket() planRepoMode {
	for _, m := range planRepoModes() {
		if m.name == "docket" {
			return m
		}
	}
	panic("docket mode not found")
}

// TestCleanupHumanTextNamesWorkspaceRemnant: the human one-liner of a cleaned
// result must still name a workspace-remnant warning and its leftover path.
func TestCleanupHumanTextNamesWorkspaceRemnant(t *testing.T) {
	r := newCleanupResult(OperationFinalizeCleanup, ResultApplied, CleanupOpResult{
		ID: 7, Disposition: CleanupDispCleaned,
		Findings: []StatusFinding{cleanupWarning(FindingWorkspaceRemnant, "delete /repo/.worktrees/x by hand")},
	})
	got := r.HumanText()
	if !strings.Contains(got, FindingWorkspaceRemnant) || !strings.Contains(got, "/repo/.worktrees/x") {
		t.Fatalf("HumanText must name the remnant warning; got %q", got)
	}
}

// --- results-artifact (no-PR) cleanup -------------------------------------

// raNoGitHub is the GitHub seam a results-artifact cleanup must never reach: it
// embeds a nil FinalizeGitHub so any call panics and fails the test.
type raNoGitHub struct{ FinalizeGitHub }

// raWorkspace scripts the workspace-removal leg of a results-artifact cleanup.
// insp is what Inspect reports; its zero value is no legible state, so a test
// that does not set it reaches Cleanup exactly as before.
type raWorkspace struct {
	FinalizeWorkspace
	disp    workspace.CleanupDisposition
	err     error
	calls   int
	insp    workspace.Inspection
	inspErr error
}

func (w *raWorkspace) Inspect(context.Context, workspace.InspectRequest) (workspace.Inspection, error) {
	return w.insp, w.inspErr
}

func (w *raWorkspace) Cleanup(context.Context, workspace.CleanupRequest) (workspace.CleanupResult, error) {
	w.calls++
	if w.err != nil {
		return workspace.CleanupResult{Disposition: workspace.CleanupFailed}, w.err
	}
	return workspace.CleanupResult{Disposition: w.disp}, nil
}

// raGit scripts the local-ref leg of a results-artifact cleanup. Every remote
// method fails the test: a completed change's cleanup never touches a remote ref.
type raGit struct {
	t           *testing.T
	tip         gitcli.ObjectID
	resolveErr  error
	fetchErr    error
	integration gitcli.ObjectID
	ancestor    bool
	ancestorErr error
	worktrees   []gitcli.WorktreeInfo
	listErr     error
	deleted     []gitcli.RefName
	remoteCalls int
}

func (g *raGit) ResolveRef(context.Context, gitcli.Repository, gitcli.RefName) (gitcli.ObjectID, error) {
	return g.tip, g.resolveErr
}
func (g *raGit) ProbeRemoteBranch(context.Context, gitcli.Repository, gitcli.RemoteName, gitcli.RefName) (gitcli.RemoteRef, error) {
	g.remoteCalls++
	g.t.Errorf("a results-artifact cleanup must never probe a remote ref")
	return gitcli.RemoteRef{}, errCleanupProbe
}
func (g *raGit) ListWorktrees(context.Context, gitcli.Repository) ([]gitcli.WorktreeInfo, error) {
	return g.worktrees, g.listErr
}
func (g *raGit) IsAncestor(context.Context, gitcli.Repository, gitcli.ObjectID, gitcli.ObjectID) (bool, error) {
	return g.ancestor, g.ancestorErr
}
func (g *raGit) FetchBranch(context.Context, gitcli.Repository, gitcli.RemoteName, gitcli.RefName) (gitcli.Revision, error) {
	if g.fetchErr != nil {
		return gitcli.Revision{}, g.fetchErr
	}
	return gitcli.Revision{Commit: g.integration}, nil
}
func (g *raGit) DeleteLocalBranchChecked(_ context.Context, _ gitcli.Repository, branch gitcli.RefName, expected gitcli.ObjectID) error {
	if expected != g.tip {
		g.t.Errorf("local delete expected tip %q, want the live tip %q", expected, g.tip)
	}
	g.deleted = append(g.deleted, branch)
	return nil
}
func (g *raGit) DeleteRemoteRefLease(context.Context, gitcli.Repository, gitcli.RemoteName, gitcli.RefName, gitcli.ObjectID) (gitcli.PushOutcome, error) {
	g.remoteCalls++
	g.t.Errorf("a results-artifact cleanup must never delete a remote ref")
	return gitcli.PushOutcome{}, errCleanupProbe
}

// happyRAGit returns a git fake whose feature tip is contained in the
// integration tip and checked out nowhere.
func happyRAGit(t *testing.T) *raGit {
	return &raGit{t: t, tip: gitcli.ObjectID(strings.Repeat("a", 40)), integration: gitcli.ObjectID(strings.Repeat("b", 40)), ancestor: true}
}

// raContext builds the closeout context of a done results-artifact change 41
// (no PR). withBranch=false strips its recorded branch.
func raContext(t *testing.T, withBranch bool) *closeoutContext {
	t.Helper()
	blob := finalizeBlob(41, "research", "done", "high", "", "deliverable: results-artifact\n")
	if !withBranch {
		blob.Data = []byte(strings.Replace(string(blob.Data), "branch: feat/research\n", "", 1))
	}
	f := newAssessFixture(t, []StatusBlob{blob}, nil)
	c, out := f.inv.snap.Change(41)
	if out != domain.LookupFound {
		t.Fatalf("record 41 not found")
	}
	return &closeoutContext{pin: f.pin, eff: f.pin.Config.Effective, snap: f.inv.snap, change: c, integrationBranch: "main", link: linkContextOf(f.pin)}
}

func raDeps(ws FinalizeWorkspace, git FinalizeCleanupGit) FinalizeDeps {
	return FinalizeDeps{GitHub: raNoGitHub{}, Workspace: ws, CleanupGit: git}
}

func TestFinalizeCleanupResultsArtifactRemovesCleanWorkspaceAndOwnlessBranch(t *testing.T) {
	git := happyRAGit(t)
	res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), raContext(t, true))
	if res.Result != ResultApplied || res.Disposition != CleanupDispCleaned {
		t.Fatalf("cleanup = %q disp %q (%s)", res.Result, res.Disposition, res.Message)
	}
	if len(res.RemovedRefs) != 1 || res.RemovedRefs[0] != "local:feat/research" {
		t.Fatalf("RemovedRefs = %v, want [local:feat/research]", res.RemovedRefs)
	}
	if len(git.deleted) != 1 || git.deleted[0] != featureRefFor("research") {
		t.Fatalf("deleted = %v, want the feature ref", git.deleted)
	}
}

func TestFinalizeCleanupResultsArtifactKeepsDirtyWorkspace(t *testing.T) {
	git := happyRAGit(t)
	res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupBlocked}, git), raContext(t, true))
	if res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupWorkspaceBlocked {
		t.Fatalf("a dirty workspace must be pending/workspace-blocked, got %q/%q", res.Disposition, res.Reason)
	}
	if len(git.deleted) != 0 {
		t.Fatalf("a retained workspace must keep the branch; deleted %v", git.deleted)
	}
}

func TestFinalizeCleanupResultsArtifactKeepsBranchWithOwnCommit(t *testing.T) {
	git := happyRAGit(t)
	git.ancestor = false
	res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), raContext(t, true))
	if res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupUnreachable {
		t.Fatalf("an own commit must be pending/%s, got %q/%q", ReasonCleanupUnreachable, res.Disposition, res.Reason)
	}
	if len(git.deleted) != 0 {
		t.Fatalf("a branch carrying its own commit must be kept; deleted %v", git.deleted)
	}
}

func TestFinalizeCleanupResultsArtifactNeverTouchesRemote(t *testing.T) {
	git := happyRAGit(t)
	_ = finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), raContext(t, true))
	if git.remoteCalls != 0 {
		t.Fatalf("remote calls = %d, want 0", git.remoteCalls)
	}
}

func TestFinalizeCleanupResultsArtifactAlreadyClean(t *testing.T) {
	t.Run("no-branch", func(t *testing.T) {
		ws := &raWorkspace{disp: workspace.CleanupCleaned}
		res := finalizeCleanupResultsArtifact(context.Background(), raDeps(ws, happyRAGit(t)), raContext(t, false))
		if res.Result != ResultNoOp || res.Disposition != CleanupDispAlready {
			t.Fatalf("no branch: cleanup = %q disp %q (%s)", res.Result, res.Disposition, res.Message)
		}
		if ws.calls != 0 {
			t.Fatalf("no branch: the workspace must not be touched")
		}
	})
	t.Run("workspace-and-ref-gone", func(t *testing.T) {
		git := happyRAGit(t)
		git.tip, git.resolveErr = "", &gitcli.Failure{Kind: gitcli.KindRefUnavailable}
		res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupAlreadyClean}, git), raContext(t, true))
		if res.Result != ResultNoOp || res.Disposition != CleanupDispAlready {
			t.Fatalf("already clean: cleanup = %q disp %q (%s)", res.Result, res.Disposition, res.Message)
		}
	})
	// A change claimed but never prepared on this machine (or cleaned on another)
	// has no workspace manifest, which the real Cleanup reports as blocked. A
	// proven-absent inspection (no manifest, ref, path, or registration) is
	// already-clean and never reaches Cleanup.
	t.Run("never-prepared-is-already-clean", func(t *testing.T) {
		git := happyRAGit(t)
		git.tip, git.resolveErr = "", &gitcli.Failure{Kind: gitcli.KindRefUnavailable}
		ws := &raWorkspace{disp: workspace.CleanupBlocked, insp: workspace.Inspection{Kind: workspace.StateAbsent, NoManifest: true}}
		res := finalizeCleanupResultsArtifact(context.Background(), raDeps(ws, git), raContext(t, true))
		if res.Result != ResultNoOp || res.Disposition != CleanupDispAlready {
			t.Fatalf("never prepared: cleanup = %q disp %q reason %q (%s)", res.Result, res.Disposition, res.Reason, res.Message)
		}
		if ws.calls != 0 {
			t.Fatalf("a proven-absent workspace must not be handed to Cleanup")
		}
	})
	// A missing manifest with a leftover (a local branch, an occupied path, or a
	// registration) inspects as foreign and stays blocked.
	t.Run("foreign-leftover-stays-blocked", func(t *testing.T) {
		ws := &raWorkspace{disp: workspace.CleanupBlocked, insp: workspace.Inspection{Kind: workspace.StateForeign, NoManifest: true}}
		res := finalizeCleanupResultsArtifact(context.Background(), raDeps(ws, happyRAGit(t)), raContext(t, true))
		if res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupWorkspaceBlocked {
			t.Fatalf("foreign leftover: got %q/%q, want pending/%s", res.Disposition, res.Reason, ReasonCleanupWorkspaceBlocked)
		}
	})
	t.Run("inspect-error-is-not-absence", func(t *testing.T) {
		ws := &raWorkspace{disp: workspace.CleanupBlocked, inspErr: errCleanupProbe}
		res := finalizeCleanupResultsArtifact(context.Background(), raDeps(ws, happyRAGit(t)), raContext(t, true))
		if res.Disposition != CleanupDispPending {
			t.Fatalf("inspect error: got %q/%q, want pending", res.Disposition, res.Reason)
		}
	})
	t.Run("ref-probe-error-is-not-absence", func(t *testing.T) {
		git := happyRAGit(t)
		git.resolveErr = errCleanupProbe
		res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupAlreadyClean}, git), raContext(t, true))
		if res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupRefProbe {
			t.Fatalf("probe error: got %q/%q, want pending/%s", res.Disposition, res.Reason, ReasonCleanupRefProbe)
		}
	})
}

func TestFinalizeCleanupResultsArtifactBranchCheckedOutElsewhere(t *testing.T) {
	git := happyRAGit(t)
	git.worktrees = []gitcli.WorktreeInfo{{Path: "/primary", Branch: featureRefFor("research")}}
	res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupAlreadyClean}, git), raContext(t, true))
	if res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupCheckedOut {
		t.Fatalf("checked out elsewhere: got %q/%q, want pending/%s", res.Disposition, res.Reason, ReasonCleanupCheckedOut)
	}
	if len(git.deleted) != 0 {
		t.Fatalf("a checked-out branch must be kept; deleted %v", git.deleted)
	}
}

func TestFinalizeCleanupResultsArtifactFetchFailureRetainsBranch(t *testing.T) {
	git := happyRAGit(t)
	git.fetchErr = errCleanupProbe
	res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), raContext(t, true))
	if res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupAncestryProbe {
		t.Fatalf("fetch failure: got %q/%q, want pending/%s", res.Disposition, res.Reason, ReasonCleanupAncestryProbe)
	}
	if len(git.deleted) != 0 {
		t.Fatalf("an unprovable ancestry must keep the branch; deleted %v", git.deleted)
	}
}

func TestResultsArtifactCleanedMessageNamesOnlyWhatWasRemoved(t *testing.T) {
	for _, tc := range []struct {
		ws, branch bool
		want       string
	}{
		{true, true, "the completed change was cleaned: workspace removed and local feature branch deleted"},
		{true, false, "the completed change was cleaned: workspace removed"},
		{false, true, "the completed change was cleaned: local feature branch deleted"},
	} {
		if got := resultsArtifactCleanedMessage(tc.ws, tc.branch); got != tc.want {
			t.Errorf("ws=%v branch=%v: got %q, want %q", tc.ws, tc.branch, got, tc.want)
		}
	}
}

// --- kept-on-purpose branch warnings --------------------------------------

// raContextCorpus is raContext over a caller-supplied corpus that must hold the
// done results-artifact record 41.
func raContextCorpus(t *testing.T, corpus []StatusBlob) *closeoutContext {
	t.Helper()
	f := newAssessFixture(t, corpus, nil)
	c, out := f.inv.snap.Change(41)
	if out != domain.LookupFound {
		t.Fatalf("record 41 not found")
	}
	return &closeoutContext{pin: f.pin, eff: f.pin.Config.Effective, snap: f.inv.snap, change: c, integrationBranch: "main", link: linkContextOf(f.pin)}
}

// keptRemoteGit is a raGit whose remote probe answers a found remote ref at a
// fixed commit, so the PR remote leg sees a moved remote.
type keptRemoteGit struct {
	*raGit
	remote gitcli.ObjectID
}

func (g *keptRemoteGit) ProbeRemoteBranch(context.Context, gitcli.Repository, gitcli.RemoteName, gitcli.RefName) (gitcli.RemoteRef, error) {
	return gitcli.RemoteRef{State: gitcli.RemoteRefFound, Commit: g.remote}, nil
}

// failingDeleteGit is a raGit whose checked local delete fails.
type failingDeleteGit struct{ *raGit }

func (g *failingDeleteGit) DeleteLocalBranchChecked(context.Context, gitcli.Repository, gitcli.RefName, gitcli.ObjectID) error {
	return errCleanupProbe
}

func soleFinding(t *testing.T, res CleanupOpResult, code string) StatusFinding {
	t.Helper()
	for _, f := range res.Findings {
		if f.Code == code {
			return f
		}
	}
	t.Fatalf("no %s finding in %+v (reason %q, %s)", code, res.Findings, res.Reason, res.Message)
	return StatusFinding{}
}

func assertContainsAll(t *testing.T, label, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("%s: %q does not contain %q", label, got, w)
		}
	}
}

func TestKeptBranchWarningsNameBranchAndCommand(t *testing.T) {
	ctx := context.Background()
	t.Run("no-pr-unreachable", func(t *testing.T) {
		git := happyRAGit(t)
		git.ancestor = false
		res := finalizeCleanupResultsArtifact(ctx, raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), raContext(t, true))
		f := soleFinding(t, res, ReasonCleanupUnreachable)
		assertContainsAll(t, "unreachable", f.Message, "local branch feat/research was kept", "not on main", "(git branch -D feat/research)")
		if !isKeptBranchFinding(f) {
			t.Errorf("unreachable must be a kept-branch finding")
		}
	})
	t.Run("no-pr-checked-out", func(t *testing.T) {
		git := happyRAGit(t)
		git.worktrees = []gitcli.WorktreeInfo{{Path: "/elsewhere", Branch: featureRefFor("research")}}
		res := finalizeCleanupResultsArtifact(ctx, raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), raContext(t, true))
		f := soleFinding(t, res, ReasonCleanupCheckedOut)
		assertContainsAll(t, "checked-out", f.Message, "local branch feat/research was kept", "checked out in another worktree", "git branch -D feat/research")
		if !isKeptBranchFinding(f) {
			t.Errorf("checked-out must be a kept-branch finding")
		}
	})
	t.Run("pr-local-tip-moved", func(t *testing.T) {
		git := happyRAGit(t)
		facts := githubcli.MergedFacts{HeadOID: strings.Repeat("c", 40), MergeCommit: strings.Repeat("d", 40)}
		_, _, f := finalizeCleanupLocalRef(ctx, FinalizeDeps{}, git, raContext(t, true), featureRefFor("research"), "feat/research", facts)
		if f == nil || f.Code != ReasonCleanupTipMoved {
			t.Fatalf("finding = %+v, want %s", f, ReasonCleanupTipMoved)
		}
		assertContainsAll(t, "tip-moved", f.Message, "local branch feat/research was kept", "moved off the merged commit", "git branch -D feat/research")
		if !isKeptBranchFinding(*f) {
			t.Errorf("tip-moved must be a kept-branch finding")
		}
	})
	t.Run("pr-local-checked-out", func(t *testing.T) {
		git := happyRAGit(t)
		git.worktrees = []gitcli.WorktreeInfo{{Path: "/elsewhere", Branch: featureRefFor("research")}}
		facts := githubcli.MergedFacts{HeadOID: string(git.tip), MergeCommit: strings.Repeat("d", 40)}
		_, _, f := finalizeCleanupLocalRef(ctx, FinalizeDeps{}, git, raContext(t, true), featureRefFor("research"), "feat/research", facts)
		if f == nil || f.Code != ReasonCleanupCheckedOut {
			t.Fatalf("finding = %+v, want %s", f, ReasonCleanupCheckedOut)
		}
		assertContainsAll(t, "pr checked-out", f.Message, "checked out in another worktree", "git branch -D feat/research")
	})
	t.Run("pr-local-unreachable", func(t *testing.T) {
		git := happyRAGit(t)
		git.ancestor = false
		facts := githubcli.MergedFacts{HeadOID: string(git.tip), MergeCommit: strings.Repeat("d", 40)}
		_, _, f := finalizeCleanupLocalRef(ctx, FinalizeDeps{}, git, raContext(t, true), featureRefFor("research"), "feat/research", facts)
		if f == nil || f.Code != ReasonCleanupUnreachable {
			t.Fatalf("finding = %+v, want %s", f, ReasonCleanupUnreachable)
		}
		assertContainsAll(t, "pr unreachable", f.Message, "not on main", "git branch -D feat/research")
	})
	t.Run("pr-remote-moved", func(t *testing.T) {
		git := &keptRemoteGit{raGit: happyRAGit(t), remote: gitcli.ObjectID(strings.Repeat("e", 40))}
		facts := githubcli.MergedFacts{HeadOID: strings.Repeat("a", 40)}
		_, _, _, f := finalizeCleanupRemoteRef(ctx, FinalizeDeps{}, git, githubcli.Repository{}, raContext(t, true), featureRefFor("research"), "feat/research", facts)
		if f == nil || f.Code != ReasonCleanupRemoteMoved {
			t.Fatalf("finding = %+v, want %s", f, ReasonCleanupRemoteMoved)
		}
		assertContainsAll(t, "remote-moved", f.Message, "remote branch feat/research was kept", "git push origin --delete feat/research")
		if !isKeptBranchFinding(*f) {
			t.Errorf("remote-moved must be a kept-branch finding")
		}
	})
}

func TestQuietCleanupReasonsCarryNoDeleteAdvice(t *testing.T) {
	ctx := context.Background()
	drive := func(t *testing.T, git FinalizeCleanupGit, code string) {
		t.Helper()
		res := finalizeCleanupResultsArtifact(ctx, raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), raContext(t, true))
		f := soleFinding(t, res, code)
		if strings.Contains(f.Message, "git branch -D") || strings.Contains(f.Message, "--delete") {
			t.Errorf("%s message carries delete advice: %q", code, f.Message)
		}
		if isKeptBranchFinding(f) {
			t.Errorf("%s must not be a kept-branch finding", code)
		}
		if strings.Contains(res.HumanText(), "git branch -D") {
			t.Errorf("%s HumanText carries delete advice: %q", code, res.HumanText())
		}
	}
	t.Run("ref-probe", func(t *testing.T) {
		git := happyRAGit(t)
		git.resolveErr = errCleanupProbe
		drive(t, git, ReasonCleanupRefProbe)
	})
	t.Run("fetch", func(t *testing.T) {
		git := happyRAGit(t)
		git.fetchErr = errCleanupProbe
		drive(t, git, ReasonCleanupAncestryProbe)
	})
	t.Run("list", func(t *testing.T) {
		git := happyRAGit(t)
		git.listErr = errCleanupProbe
		drive(t, git, ReasonCleanupListProbe)
	})
	t.Run("local-delete", func(t *testing.T) {
		drive(t, &failingDeleteGit{raGit: happyRAGit(t)}, ReasonCleanupLocalDelete)
	})
	for _, code := range []string{ReasonCleanupLeaseRejected, ReasonCleanupRemoteProbe, ReasonCleanupChildProbe, FindingWorkspaceRemnant, ReasonCleanupLocalDelete, ReasonCleanupRefProbe, ReasonCleanupAncestryProbe, ReasonCleanupListProbe} {
		if isKeptBranchFinding(cleanupWarning(code, "x")) {
			t.Errorf("%s must not be a kept-branch finding", code)
		}
	}
}

func TestKeptBranchWarningReusedName(t *testing.T) {
	ra := finalizeBlob(41, "research", "done", "high", "", "deliverable: results-artifact\n")
	other := func(status string) StatusBlob {
		b := finalizeBlob(50, "other", status, "high", prRefFor(50), "")
		b.Data = []byte(strings.Replace(string(b.Data), "branch: feat/other\n", "branch: feat/research\n", 1))
		return b
	}
	t.Run("live-reuse-suppresses-advice", func(t *testing.T) {
		cc := raContextCorpus(t, []StatusBlob{ra, other("in-progress")})
		git := happyRAGit(t)
		git.ancestor = false
		res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), cc)
		f := soleFinding(t, res, ReasonCleanupUnreachable)
		if !strings.Contains(f.Message, "change #50 is now using it") || strings.Contains(f.Message, "git branch -D") {
			t.Errorf("reused name: got %q", f.Message)
		}
		rf := keptBranchWarning(cc, ReasonCleanupRemoteMoved, "feat/research")
		if want := "remote branch feat/research was kept: change #50 is now using it."; rf.Message != want {
			t.Errorf("remote reused name: got %q, want %q", rf.Message, want)
		}
	})
	t.Run("final-record-keeps-advice", func(t *testing.T) {
		cc := raContextCorpus(t, []StatusBlob{ra, other("done")})
		f := keptBranchWarning(cc, ReasonCleanupUnreachable, "feat/research")
		if !strings.Contains(f.Message, "git branch -D feat/research") || strings.Contains(f.Message, "now using it") {
			t.Errorf("final record must not suppress advice: got %q", f.Message)
		}
	})
}

func TestCleanupHumanTextNamesKeptBranch(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		git := happyRAGit(t)
		git.ancestor = false
		res := finalizeCleanupResultsArtifact(context.Background(), raDeps(&raWorkspace{disp: workspace.CleanupCleaned}, git), raContext(t, true))
		if res.Result != ResultBlocked {
			t.Fatalf("result = %q, want %q", res.Result, ResultBlocked)
		}
		assertContainsAll(t, "pending HumanText", res.HumanText(), ReasonCleanupUnreachable, "git branch -D feat/research")
	})
	t.Run("applied", func(t *testing.T) {
		cc := raContext(t, true)
		r := newCleanupResult(OperationFinalizeCleanup, ResultApplied, CleanupOpResult{
			ID: 41, Disposition: CleanupDispChildrenRetargetRequired,
			Findings: []StatusFinding{keptBranchWarning(cc, ReasonCleanupCheckedOut, "feat/research")},
		})
		assertContainsAll(t, "applied HumanText", r.HumanText(), "checked out in another worktree", "git branch -D feat/research")
	})
}
