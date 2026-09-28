package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
	"strings"
	"testing"
	"time"
)

// This file drives the finalize rebase state machine and its local-gate
// composition over a REAL feature workspace (a real bare-remote topology, a real
// gitcli.Client, a real workspace.Service with its owned rebase receipt) plus a
// fake FinalizeGitHub for the PR facts and a fake FinalizeGate for the suite
// outcome. The receipt-before-mutation ordering, the owned recovery refs, and the
// foreign-state retention are only meaningful against real Git, so nothing about
// the rebase itself is stubbed; only the two external effects that a hermetic
// suite cannot run (GitHub and the suite process) are injected.

// --- fake FinalizeGitHub --------------------------------------------------

// fakeRebaseGitHub answers the two GitHub calls the rebase operation makes
// (DiscoverRepository, FindOpenPullRequestsByHead) from an in-memory PR registry.
// Every other finalize-half GitHub method panics so an accidental call is loud.
type fakeRebaseGitHub struct {
	repo    githubcli.Repository
	repoErr error
	prs     []githubcli.PullRequest
	findErr error
}

func (f *fakeRebaseGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	if f.repoErr != nil {
		return githubcli.Repository{}, f.repoErr
	}
	return f.repo, nil
}

func (f *fakeRebaseGitHub) ViewPullRequest(context.Context, githubcli.Repository, int) (githubcli.PullRequest, error) {
	panic("ViewPullRequest: rebase must not call this")
}
func (f *fakeRebaseGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, headBranch string) ([]githubcli.PullRequest, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	var out []githubcli.PullRequest
	for _, pr := range f.prs {
		if pr.HeadBranch == headBranch {
			out = append(out, pr)
		}
	}
	return out, nil
}

func (f *fakeRebaseGitHub) ProbeMerged(context.Context, githubcli.Repository, int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	panic("ProbeMerged: rebase must not call this")
}
func (f *fakeRebaseGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: rebase must not call this")
}
func (f *fakeRebaseGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: rebase must not call this")
}
func (f *fakeRebaseGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: rebase must not call this")
}
func (f *fakeRebaseGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeResult, error) {
	panic("MergePullRequest: rebase must not call this")
}

// --- fake FinalizeGate ----------------------------------------------------

// fakeGate is a scripted FinalizeGate: it returns the configured result/error and
// counts its calls, so a test both scripts the gate outcome and asserts whether
// the gate ran at all (the skip path must never call it).
type fakeGate struct {
	result LocalGateResult
	err    error
	calls  int
}

func (g *fakeGate) RunLocalGate(_ context.Context, _ LocalGateRequest) (LocalGateResult, error) {
	g.calls++
	if g.err != nil {
		return LocalGateResult{}, g.err
	}
	return g.result, nil
}

// headEvidenceGate is a passing FinalizeGate that mints green evidence
// certifying the EXACT head each request names, so a recorded publish checkpoint
// verifies against the rebased head. It counts calls like fakeGate so skip paths
// can assert the suite never ran.
type headEvidenceGate struct {
	t     *testing.T
	calls int
}

func (g *headEvidenceGate) RunLocalGate(_ context.Context, req LocalGateRequest) (LocalGateResult, error) {
	g.calls++
	rec, err := evidence.NewRecord("go test ./...", strings.ToLower(req.Head), time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC))
	if err != nil {
		g.t.Fatalf("evidence.NewRecord: %v", err)
	}
	return LocalGateResult{Outcome: FinalizeGatePassed, Evidence: evidence.Render(rec), RunDir: "/run/x"}, nil
}

// seqGate is a FinalizeGate that returns a scripted sequence of results, one per
// call, and records every request it received — so a test can assert the second
// slice RESUMED with the exact continuation the first (WAITING) slice returned
// (driver Advance semantics), not started a fresh drive.
type seqGate struct {
	results []LocalGateResult
	reqs    []LocalGateRequest
}

func (g *seqGate) RunLocalGate(_ context.Context, req LocalGateRequest) (LocalGateResult, error) {
	g.reqs = append(g.reqs, req)
	if len(g.reqs) > len(g.results) {
		return LocalGateResult{}, fmt.Errorf("seqGate: unexpected call %d (only %d scripted)", len(g.reqs), len(g.results))
	}
	return g.results[len(g.reqs)-1], nil
}

// --- fixture --------------------------------------------------------------

// errRebaseGateSeam is the injected unrecoverable gate-seam failure.
var errRebaseGateSeam = errors.New("gate seam boom")

// rebaseFixture is a real feature workspace ready to rebase: a published feature
// head on refs/heads/feat/<slug> over origin/main, an implemented record on the
// metadata branch, and the resolved deps/target/paths a test drives.
type rebaseFixture struct {
	t            *testing.T
	repo         *gitRepo
	deps         PlanningDeps
	svc          *workspace.Service
	gitrepo      gitcli.Repository
	target       workspace.Target
	wp           string
	head         string
	baseTip      string
	version      string
	metaDir      string
	id           int
	slug         string
	branch       string // metadata branch of the mode
	baseAdvances int
}

const (
	rebaseFixtureID   = 5
	rebaseFixtureSlug = "widget"
)

// setupRebaseFixture builds the real feature workspace for one metadata mode with
// an implemented record.
func setupRebaseFixture(t *testing.T, m planRepoMode) *rebaseFixture {
	return setupRebaseFixtureStatus(t, m, "implemented")
}

// setupRebaseFixtureStatus builds the fixture with the record at a given
// lifecycle status (the workspace is prepared regardless, so a not-implemented
// precondition can be exercised over a real feature branch).
func setupRebaseFixtureStatus(t *testing.T, m planRepoMode, status string) *rebaseFixture {
	t.Helper()
	requireRealGit(t)
	id, slug := rebaseFixtureID, rebaseFixtureSlug
	recPath := groomPath(id, slug)
	repo := buildConfiguredRepo(t, m, recPath, lifecycleChange(id, slug, status))

	node := planningDepsFor(t, repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	ctx := context.Background()
	gitrepo, err := node.deps.Client.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: repo.invocation})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	base := domain.EffectiveBase{Kind: domain.BaseResolved, Branch: "main"}
	target, err := workspace.NewTarget(domain.ChangeID(id), slug, base, "feat/"+slug)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	ws, err := svc.Prepare(ctx, workspace.PrepareRequest{Repository: gitrepo, Remote: "origin", Target: target})
	if err != nil {
		t.Fatalf("workspace prepare: %v", err)
	}
	wp := ws.Path
	baseTip := runGit(t, wp, "rev-parse", "HEAD")

	// A real feature commit advances the head off the base.
	writeRepoFile(t, wp, "feature.txt", "feature work\n")
	runGit(t, wp, "add", "-A")
	runGit(t, wp, "commit", "-q", "-m", "implement the feature")
	head := runGit(t, wp, "rev-parse", "HEAD")

	if _, err := svc.PublishHead(ctx, workspace.PublishRequest{Repository: gitrepo, Remote: "origin", Target: target}); err != nil {
		t.Fatalf("publish head: %v", err)
	}

	return &rebaseFixture{
		t:       t,
		repo:    repo,
		deps:    node.deps,
		svc:     svc,
		gitrepo: gitrepo,
		target:  target,
		wp:      wp,
		head:    head,
		baseTip: baseTip,
		version: blobVersionAt(t, repo.origin, m.branch, recPath),
		metaDir: workspace.MetaDir(gitrepo.CommonDir, target.FeatureRef),
		id:      id,
		slug:    slug,
		branch:  m.branch,
	}
}

// finalizeDeps assembles the FinalizeDeps a rebase test drives: the real planning
// seams and workspace service, the fake GitHub, and the fake gate.
func (f *rebaseFixture) finalizeDeps(gh FinalizeGitHub, gate FinalizeGate) FinalizeDeps {
	return FinalizeDeps{
		Planning:  f.deps,
		GitHub:    gh,
		Workspace: f.svc,
		Gate:      gate,
	}
}

// freshFinalizeDeps builds a fully independent FinalizeDeps over the SAME on-disk
// repository as the fixture: its own gitcli.Client, its own StatusReader, its own
// workspace.Service, and fresh fakes — no in-process state shared with any other
// deps. It models a separate OS process (docket runs one operation per process, so
// gitStatusReader is constructed fresh per operation). Two of these racing share
// only the on-disk repo and its per-workspace flock (keyed on the metaDir), so a
// concurrency test proves the file lock — not shared Go memory — is what serializes
// admission, and never trips the race detector on a reader field no real deployment
// shares.
func (f *rebaseFixture) freshFinalizeDeps(t *testing.T) FinalizeDeps {
	t.Helper()
	node := planningDepsFor(t, f.repo.invocation)
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	return FinalizeDeps{Planning: node.deps, GitHub: gh, Workspace: svc, Gate: gate}
}

// prForHead builds an open PR for the feature head, targeting main, with the
// given body (empty for no evidence).
func (f *rebaseFixture) prForHead(head, body string) githubcli.PullRequest {
	return githubcli.PullRequest{
		Number: 1, State: githubcli.StateOpen, HeadBranch: "feat/" + f.slug,
		HeadCommit: head, BaseBranch: "main", Version: "sha256:" + strings.Repeat("a", 64), Body: body,
	}
}

// receiptAbsent asserts no owned rebase receipt exists.
func (f *rebaseFixture) receiptAbsent(t *testing.T) {
	t.Helper()
	_, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil {
		t.Fatalf("ReadRebaseReceipt: %v", err)
	}
	if present {
		t.Fatalf("a receipt was written when the operation should have left Git untouched")
	}
}

// localHead reads the current feature-workspace head.
func (f *rebaseFixture) localHead() string {
	return runGit(f.t, f.wp, "rev-parse", "HEAD")
}

// advanceBase adds a non-conflicting commit to origin/main so a fresh feature
// head no longer sits on the base — forcing a real rewrite. Each call writes a
// distinct file so repeated advances always produce a fresh commit.
func (f *rebaseFixture) advanceBase(t *testing.T) string {
	t.Helper()
	f.baseAdvances++
	name := "base-advance-" + itoaTest(f.baseAdvances) + ".txt"
	return f.repo.writerAdvance(t, "main", map[string]string{name: "downstream base work\n"})
}

// greenEvidenceFor renders a green build-evidence block certifying head, for
// embedding in a PR body.
func greenEvidenceFor(t *testing.T, head string) string {
	t.Helper()
	rec, err := evidence.NewRecord("go test ./...", head, time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("evidence.NewRecord: %v", err)
	}
	return "Authored prose.\n\n" + evidence.Render(rec) + "\nMore prose.\n"
}

// --- TestGateDecision -----------------------------------------------------

func TestGateDecision(t *testing.T) {
	const head = "abc123"
	cases := []struct {
		name         string
		noop         bool
		evidenceHead string
		currentHead  string
		green        bool
		wantSkip     bool
	}{
		{"noop-exact-green-skips", true, head, head, true, true},
		{"noop-green-moved-head-runs", true, "deadbeef", head, true, false},
		{"noop-no-evidence-runs", true, "", head, false, false},
		{"noop-malformed-evidence-runs", true, "", head, false, false},
		{"real-rebase-even-with-exact-green-runs", false, head, head, true, false},
		{"noop-green-empty-head-runs", true, "", head, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A matching, non-empty command isolates the head/no-op/green axes this
			// table exercises; the command axis is TestGateDecisionRequiresCommandByteEquality's.
			skip, permit := gateDecision(tc.noop, tc.evidenceHead, tc.currentHead, tc.green, "go test ./...", "go test ./...")
			if skip != tc.wantSkip {
				t.Fatalf("gateDecision skip = %v, want %v", skip, tc.wantSkip)
			}
			if skip && permit != tc.evidenceHead {
				t.Fatalf("skip permit = %q, want the exact evidence head %q", permit, tc.evidenceHead)
			}
			if !skip && permit != "" {
				t.Fatalf("a run decision named a permit %q; want none", permit)
			}
		})
	}
}

// TestGateDecisionRequiresCommandByteEquality pins the command axis of the
// finalize suite-skip waiver: a no-op rebase with exact-head GREEN evidence
// still runs the suite unless the recorded command is byte-equal to the
// currently resolved finalize.test_command, and an empty-vs-empty command is a
// vacuous match that must NOT skip. Skipped build evidence (green=false) never
// waives finalize.
func TestGateDecisionRequiresCommandByteEquality(t *testing.T) {
	head := strings.Repeat("ab", 20)
	cases := []struct {
		name               string
		green              bool
		evCmd, resolvedCmd string
		wantSkip           bool
	}{
		{"green same command skips", true, "go test ./...", "go test ./...", true},
		{"green differing command runs the suite", true, "go test ./...", "make check", false},
		{"green empty resolved command runs (never a vacuous match)", true, "", "", false},
		{"skipped evidence never waives finalize", false, "", "go test ./...", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			skip, permit := gateDecision(true, head, head, c.green, c.evCmd, c.resolvedCmd)
			if skip != c.wantSkip {
				t.Errorf("skip = %v, want %v", skip, c.wantSkip)
			}
			if skip && permit != head {
				t.Errorf("permit = %q, want the head", permit)
			}
		})
	}
}

// TestPRBodyEvidenceReportsCommandAndGreenFlag proves prBodyEvidence surfaces
// the recorded command and reports green ONLY for a green record — a skipped
// (build-gate-off) block is not green and carries no command, so it can never
// waive finalize's local gate through gateDecision.
func TestPRBodyEvidenceReportsCommandAndGreenFlag(t *testing.T) {
	head := strings.Repeat("ab", 20)
	green, err := evidence.NewRecord("go test ./...", head, time.Now())
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	h, cmd, isGreen := prBodyEvidence(githubcli.PullRequest{Body: evidence.Render(green)})
	if !isGreen || cmd != "go test ./..." || h != head {
		t.Errorf("green record: head/cmd/green = %q/%q/%v, want %q/%q/true", h, cmd, isGreen, head, "go test ./...")
	}
	skipped, err := evidence.NewSkippedRecord(head, time.Now())
	if err != nil {
		t.Fatalf("NewSkippedRecord: %v", err)
	}
	sh, scmd, sGreen := prBodyEvidence(githubcli.PullRequest{Body: evidence.Render(skipped)})
	if sGreen {
		t.Errorf("skipped record reported green; skipped evidence never waives finalize")
	}
	if scmd != "" {
		t.Errorf("skipped command = %q, want empty", scmd)
	}
	if sh != head {
		t.Errorf("skipped head = %q, want %q", sh, head)
	}
}

// --- TestFinalizeRebaseHappyAndReceipt ------------------------------------

// --- TestFinalizeRebasePreconditions --------------------------------------

// assertRebaseRefused asserts a result carries the expected protocol result and
// reason, and never a success disposition.
func assertRebaseRefused(t *testing.T, res FinalizeRebaseResult, want Result, reason string) {
	t.Helper()
	if res.Result != want || res.Reason != reason {
		t.Fatalf("refusal = (%q, %q), want (%q, %q) [findings %v msg %q]", res.Result, res.Reason, want, reason, res.Findings, res.Message)
	}
}

// --- TestFinalizeRebaseGateOutcomes ---------------------------------------

// --- TestFinalizeRebaseGateWaiting ----------------------------------------

// --- TestFinalizeRebaseResponseLossRecovery -------------------------------

// --- TestFinalizeRebaseForeignStateBlocked --------------------------------

// --- conflict fixture + continue/abort ------------------------------------

// setupConflictedRebase drives a fixture into a live, owned conflicted rebase: the
// base advances with a conflicting edit to the feature's file, so BeginRebase
// stops at that path. It returns the fixture, the conflicted result, and the owned
// attempt token.
func setupConflictedRebase(t *testing.T, m planRepoMode) (*rebaseFixture, FinalizeRebaseResult, FinalizeDeps) {
	t.Helper()
	f := setupRebaseFixture(t, m)
	// The base edits the same file the feature added — a guaranteed add/add conflict.
	f.repo.writerAdvance(t, "main", map[string]string{"feature.txt": "conflicting base content\n"})
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Disposition != RebaseDispConflicted || res.Attempt == "" {
		t.Fatalf("expected a conflicted rebase with an attempt token, got disp %q reason %q msg %q", res.Disposition, res.Reason, res.Message)
	}
	if len(res.UnmergedPaths) == 0 {
		t.Fatalf("conflicted result carried no unmerged paths")
	}
	return f, res, deps
}

// TestCheckpointDecision pins the pure publish-checkpoint reuse policy: reuse
// requires verified evidence and full equality on tested head, base head,
// command, gate policy, and PR number.
func TestCheckpointDecision(t *testing.T) {
	head := strings.Repeat("ab", 20)
	base := strings.Repeat("cd", 20)
	good := publishCheckpoint{Head: head, BaseHead: base, Command: "go test ./...", Gate: "local", PRNumber: "7", Evidence: "block"}
	cases := []struct {
		name        string
		cp          publishCheckpoint
		currentHead string
		liveBase    string
		resolvedCmd string
		resolvedGt  string
		prNumber    int
		verified    bool
		want        bool
	}{
		{"all-match-reuses", good, head, base, "go test ./...", "local", 7, true, true},
		{"unverified-evidence-runs", good, head, base, "go test ./...", "local", 7, false, false},
		{"moved-head-runs", good, strings.Repeat("ef", 20), base, "go test ./...", "local", 7, true, false},
		{"moved-base-runs", good, head, strings.Repeat("ef", 20), "go test ./...", "local", 7, true, false},
		{"changed-command-runs", good, head, base, "make check", "local", 7, true, false},
		{"empty-resolved-command-runs", publishCheckpoint{Head: head, BaseHead: base, Command: "", Gate: "local", PRNumber: "7", Evidence: "block"}, head, base, "", "local", 7, true, false},
		{"changed-gate-policy-runs", good, head, base, "go test ./...", "off", 7, true, false},
		{"different-pr-runs", good, head, base, "go test ./...", "local", 8, true, false},
		{"zero-pr-runs", good, head, base, "go test ./...", "local", 0, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkpointDecision(tc.cp, tc.currentHead, tc.liveBase, tc.resolvedCmd, tc.resolvedGt, tc.prNumber, tc.verified); got != tc.want {
				t.Fatalf("checkpointDecision = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPublishCheckpointOf proves the presence probe: a fully-set checkpoint is
// extracted; any empty member reads as absent.
func TestPublishCheckpointOf(t *testing.T) {
	full := workspace.RebaseReceipt{
		PublishCheckpointHead:     strings.Repeat("ab", 20),
		PublishCheckpointBaseHead: strings.Repeat("cd", 20),
		PublishCheckpointCommand:  "go test ./...",
		PublishCheckpointGate:     "local",
		PublishCheckpointPRNumber: "7",
		PublishCheckpointEvidence: "block",
	}
	if cp, ok := publishCheckpointOf(full); !ok || cp.Head != full.PublishCheckpointHead || cp.Evidence != "block" {
		t.Fatalf("publishCheckpointOf(full) = (%+v, %v), want present with the receipt's fields", cp, ok)
	}
	missing := full
	missing.PublishCheckpointEvidence = ""
	if _, ok := publishCheckpointOf(missing); ok {
		t.Fatalf("publishCheckpointOf with an empty member reported present; want absent")
	}
	if _, ok := publishCheckpointOf(workspace.RebaseReceipt{}); ok {
		t.Fatalf("publishCheckpointOf(zero) reported present; want absent")
	}
}

// --- resolver-budget snapshot (change 0349) -------------------------------

// setResolverConfig writes a repository-local `.docket.local.yml` that resolves
// finalize.resolver_max_attempts through the repository-local layer (scopeAny),
// so a test can exercise a NON-DEFAULT resolved cap without touching the pinned
// committed .docket.yml.
func setResolverConfig(t *testing.T, f *rebaseFixture, limit int) {
	t.Helper()
	writeRepoFile(t, f.repo.invocation, ".docket.local.yml",
		fmt.Sprintf("finalize:\n  resolver_max_attempts: %d\n", limit))
}

// beginConflictedWithLimit drives a fresh owned rebase into a live conflict with
// the resolved cap set to limit through the repository-local layer, returning the
// fixture, the conflicted begin result, and the deps.
func beginConflictedWithLimit(t *testing.T, limit int) (*rebaseFixture, FinalizeRebaseResult, FinalizeDeps) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	// A conflicting base edit guarantees BeginRebase stops at the feature file.
	f.repo.writerAdvance(t, "main", map[string]string{"feature.txt": "conflicting base content\n"})
	setResolverConfig(t, f, limit)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Disposition != RebaseDispConflicted {
		t.Fatalf("fresh rebase = disp %q (reason %q msg %q), want conflicted", res.Disposition, res.Reason, res.Message)
	}
	return f, res, deps
}

// staleFirstReadWorkspace wraps a FinalizeWorkspace to prove every
// gate-continuation receipt rewrite reloads the resolver-budget group from disk
// (change 0349's write-forward rule). Its FIRST ReadRebaseReceipt serves a STALE
// copy — the budget as it stood before a reserve/continue advanced it — so the
// operation's in-memory `rec` is stale; every later read (the write path's
// reload) delegates to the real store. A write path that copies the six resolver
// fields forward from the reloaded receipt survives; one that trusts the stale
// in-memory copy silently loses the advanced budget and reddens the test.
type staleFirstReadWorkspace struct {
	FinalizeWorkspace
	served bool
	stale  workspace.RebaseReceipt
}

func (w *staleFirstReadWorkspace) ReadRebaseReceipt(ctx context.Context, dir string) (workspace.RebaseReceipt, bool, error) {
	if !w.served {
		w.served = true
		return w.stale, true, nil
	}
	return w.FinalizeWorkspace.ReadRebaseReceipt(ctx, dir)
}

// assertResolverFieldsEqual fails unless the six resolver-budget fields of got
// match want byte-identically.
func assertResolverFieldsEqual(t *testing.T, when string, want, got workspace.RebaseReceipt) {
	t.Helper()
	if got.ResolverBudgetVersion != want.ResolverBudgetVersion ||
		got.ResolverLimit != want.ResolverLimit ||
		got.ResolverUsed != want.ResolverUsed ||
		got.ResolverReservationToken != want.ResolverReservationToken ||
		got.ResolverReservationStopped != want.ResolverReservationStopped ||
		got.ResolverContinuationStarted != want.ResolverContinuationStarted {
		t.Fatalf("%s: resolver fields drifted:\n got ver=%q limit=%q used=%q tok=%q stopped=%q cont=%q\nwant ver=%q limit=%q used=%q tok=%q stopped=%q cont=%q",
			when,
			got.ResolverBudgetVersion, got.ResolverLimit, got.ResolverUsed, got.ResolverReservationToken, got.ResolverReservationStopped, got.ResolverContinuationStarted,
			want.ResolverBudgetVersion, want.ResolverLimit, want.ResolverUsed, want.ResolverReservationToken, want.ResolverReservationStopped, want.ResolverContinuationStarted)
	}
}

// completedBudgetedReceipt drives a fresh real rewrite to a valid completed
// receipt, then seeds its resolver-budget group with a consumed opportunity and
// an outstanding reservation (as the reserve/continue path would), returning the
// fixture, the fake GitHub, and the seeded on-disk receipt.
func completedBudgetedReceipt(t *testing.T, seed func(*workspace.RebaseReceipt)) (*rebaseFixture, *fakeRebaseGitHub, workspace.RebaseReceipt) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.advanceBase(t) // a real rewrite (never a no-op) so the gate composes.
	ctx := context.Background()
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	first := FinalizeRebase(ctx, f.finalizeDeps(gh, &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}),
		f.repo.invocation, FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if first.Disposition != RebaseDispRebased {
		t.Fatalf("first rebase = %q, want rebased", first.Disposition)
	}
	rec, present, err := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt after first rebase: present=%v err=%v", present, err)
	}
	rec.ResolverBudgetVersion = "1"
	rec.ResolverLimit = "3"
	rec.ResolverUsed = "1"
	rec.ResolverReservationToken = "tok-1"
	rec.ResolverReservationStopped = strings.Repeat("a", 40)
	seed(&rec)
	if err := f.svc.WriteRebaseReceipt(ctx, f.metaDir, rec); err != nil {
		t.Fatalf("seed budgeted receipt: %v", err)
	}
	real, _, _ := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	return f, gh, real
}

// --- reservation-verified continue (change 0349, Task 7) -------------------

var errStageSeamBoom = errors.New("stage-and-continue seam boom")

// stageSeam wraps the real continue Git seam so a continue test can COUNT
// StageAndContinueRebase calls, capture the receipt's continuation-started marker
// AT staging time (the durable mark must land before the Git mutation), and — when
// scripted — return a synthetic next-conflict/rebased status without a real
// multi-commit fixture. RebaseState and StoppedRebaseCommit delegate to the real
// client so the reservation's stopped-commit verification runs against live Git.
type stageSeam struct {
	FinalizeContinueGit
	f          *rebaseFixture
	calls      int
	contAtCall string               // ResolverContinuationStarted read when staging is invoked
	script     *gitcli.RebaseStatus // when non-nil, returned instead of delegating to real Git
	scriptErr  error
}

func (g *stageSeam) StageAndContinueRebase(ctx context.Context, dir string, paths []string) (gitcli.RebaseStatus, error) {
	g.calls++
	if g.f != nil {
		rec, _, _ := g.f.svc.ReadRebaseReceipt(ctx, g.f.metaDir)
		g.contAtCall = rec.ResolverContinuationStarted
	}
	if g.scriptErr != nil {
		return gitcli.RebaseStatus{}, g.scriptErr
	}
	if g.script != nil {
		return *g.script, nil
	}
	return g.FinalizeContinueGit.StageAndContinueRebase(ctx, dir, paths)
}

// errReconcileWrite is the injected durable-write failure for the
// reservation-reconciliation sites (change 0411).
var errReconcileWrite = errors.New("reconcile write boom")

// reconcileFailWorkspace wraps the real FinalizeWorkspace and faults
// WriteRebaseReceipt after `allow` successful writes while `fail` is set, so a
// test can let the continuation-started marker land durably and then fail only
// the reservation reconciliation. Clearing `fail` restores writes for the
// recovery retry. Every other seam method delegates to the embedded service.
type reconcileFailWorkspace struct {
	FinalizeWorkspace
	allow  int  // writes that pass through before faulting
	fail   bool // fault writes past allow while set
	writes int  // observed write count
}

func (w *reconcileFailWorkspace) WriteRebaseReceipt(ctx context.Context, dir string, r workspace.RebaseReceipt) error {
	w.writes++
	if w.fail && w.writes > w.allow {
		return errReconcileWrite
	}
	return w.FinalizeWorkspace.WriteRebaseReceipt(ctx, dir, r)
}

// reserveOnConflict drives a fresh owned rebase into a live conflict with the given
// resolver cap, then durably reserves ONE dispatch (real FinalizeResolverReserve),
// returning the fixture, deps, the owned attempt, the reserved token, and the live
// stopped commit the reservation is bound to (used == 1 afterwards).
func reserveOnConflict(t *testing.T, limit int) (f *rebaseFixture, deps FinalizeDeps, attempt, token, stopped string) {
	t.Helper()
	f, begin, deps := beginConflictedWithLimit(t, limit)
	attempt = begin.Attempt
	stopped = liveStoppedCommit(t, f)
	res := FinalizeResolverReserve(context.Background(), deps, f.repo.invocation, f.id, attempt)
	if res.Disposition != ReserveReserved || res.Reservation == "" {
		t.Fatalf("reserve on conflict = disp %q token %q (reason %q), want reserved with a token", res.Disposition, res.Reservation, res.Reason)
	}
	return f, deps, attempt, res.Reservation, stopped
}

// TestMapDriveOutcomeCarriesRefusalDetail proves a Start/Advance command failure
// that carried a typed refusal (no drive document) maps to a halted
// LocalGateResult that PRESERVES reason/message/stage/locator beside the coarse
// unavailable cause — and that a detail-less failure keeps the exact current
// generic shape.
func TestMapDriveOutcomeCarriesRefusalDetail(t *testing.T) {
	g := &processFinalizeGate{}
	out := GateDriveResult{
		Envelope: NewEnvelope(OperationGateDriveStart, ResultInvalidInput),
		Reason:   "worktree-busy",
		Message:  "a raw gate run occupies this worktree's execution slot; ...",
		Stage:    stageWorktreeAdmission,
		Locator:  "incumbent-run:0123456789abcdef0123456789abcdef",
	}
	got := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, out)
	if got.Outcome != FinalizeGateHalted || got.HaltCause != GateHaltUnavailable {
		t.Fatalf("outcome/cause = %v/%q", got.Outcome, got.HaltCause)
	}
	if got.HaltReason != "worktree-busy" || got.HaltStage != stageWorktreeAdmission ||
		got.HaltLocator != "incumbent-run:0123456789abcdef0123456789abcdef" || got.HaltMessage == "" {
		t.Fatalf("refusal detail dropped: %+v", got)
	}
	if got.RunDir != "" {
		t.Fatalf("run_dir must not carry incumbent facts: %q", got.RunDir)
	}
	bare := g.mapDriveOutcome(context.Background(), LocalGateRequest{}, GateDriveResult{})
	if bare.HaltReason != "" || bare.HaltMessage != "" || bare.HaltStage != "" || bare.HaltLocator != "" {
		t.Fatalf("detail-less failure grew detail: %+v", bare)
	}
}
