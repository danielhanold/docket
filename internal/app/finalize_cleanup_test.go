package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
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
// DiscoverRepository, ProbeMerged (the terminal reprobe), and
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
// change reaches done+archived — exactly the terminal state cleanup consumes.
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

// --- TestFinalizeCleanupOnlyAfterTerminal ---------------------------------

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
// stacked-merged (a terminal-but-retained state).
func setupStackedMergedCleanupFixture(t *testing.T) *closeoutFixture {
	t.Helper()
	f := setupCloseoutFixture(t, planRepoModeDocket())
	recPath := groomPath(f.id, f.slug)
	src := closeoutRecord(f.id, f.slug, "stacked-merged", closeoutRef, f.specPath, f.planPath, f.resultsPath)
	f.repo.writerAdvance(t, f.branch, map[string]string{recPath: src})
	f.version = blobVersionAt(t, f.repo.origin, f.branch, recPath)
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
		dir := t.TempDir()
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
	root := t.TempDir()
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

// planRepoModeDocket returns the docket metadata mode (the mode where the plan
// and results live on a genuinely different integration ref, exercising the
// backlink-repair leg).
func planRepoModeDocket() planRepoMode {
	for _, m := range planRepoModes() {
		if m.name == "docket" {
			return m
		}
	}
	panic("docket mode not found")
}

// --- TestCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors ----------------
