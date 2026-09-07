package app

import (
	"context"
	"encoding/json"
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

// TestFinalizeRebaseResolverBudgetSnapshot proves a fresh owned rebase snapshots
// the RESOLVED finalize.resolver_max_attempts into the receipt as a versioned
// budget group (version "1", the resolved non-default limit, used 0, no
// outstanding reservation), and that the conflicted result surfaces the counts in
// both its JSON document and its human text.
func TestFinalizeRebaseResolverBudgetSnapshot(t *testing.T) {
	f, res, _ := beginConflictedWithLimit(t, 2)

	// The receipt carries the versioned budget group at the resolved non-default.
	rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt after fresh rebase: present=%v err=%v", present, err)
	}
	if rec.ResolverBudgetVersion != "1" || rec.ResolverLimit != "2" || rec.ResolverUsed != "0" {
		t.Fatalf("receipt budget = ver %q limit %q used %q, want 1/2/0 (the resolved non-default 2, not the built-in 3)",
			rec.ResolverBudgetVersion, rec.ResolverLimit, rec.ResolverUsed)
	}
	if rec.ResolverReservationToken != "" || rec.ResolverReservationStopped != "" || rec.ResolverContinuationStarted != "" {
		t.Errorf("fresh receipt carried a reservation: token %q stopped %q cont %q",
			rec.ResolverReservationToken, rec.ResolverReservationStopped, rec.ResolverContinuationStarted)
	}

	// The conflicted result carries the counts (2/0/2).
	if res.ResolverLimit != 2 || res.ResolverUsed != 0 || res.ResolverRemaining != 2 {
		t.Fatalf("result counts = %d/%d/%d, want 2/0/2", res.ResolverLimit, res.ResolverUsed, res.ResolverRemaining)
	}
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		ResolverLimit     int `json:"resolver_limit"`
		ResolverRemaining int `json:"resolver_remaining"`
	}
	if err := json.Unmarshal(buf, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.ResolverLimit != 2 || doc.ResolverRemaining != 2 {
		t.Errorf("JSON counts = limit %d remaining %d, want 2/2 (raw %s)", doc.ResolverLimit, doc.ResolverRemaining, buf)
	}
	if !strings.Contains(string(buf), `"resolver_limit":2`) || !strings.Contains(string(buf), `"resolver_remaining":2`) {
		t.Errorf("JSON document missing resolver counts: %s", buf)
	}
	if h := res.HumanText(); !strings.Contains(h, "0/2") || !strings.Contains(h, "2 remaining") {
		t.Errorf("HumanText does not mention the resolver counts: %q", h)
	}
}

// TestFinalizeRebaseResolverBudgetRecoveryNoResnapshot proves recoverFromReceipt
// adopts the receipt's stored budget and NEVER re-snapshots the current config: a
// mid-attempt config change (2 -> 5) does not apply to an owned attempt.
func TestFinalizeRebaseResolverBudgetRecoveryNoResnapshot(t *testing.T) {
	f, first, deps := beginConflictedWithLimit(t, 2)
	if first.ResolverLimit != 2 {
		t.Fatalf("fresh conflicted limit = %d, want 2", first.ResolverLimit)
	}
	// The operator raises the cap mid-attempt; the owned attempt must ignore it.
	setResolverConfig(t, f, 5)
	second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if second.Disposition != RebaseDispConflicted {
		t.Fatalf("recovery = disp %q (reason %q), want conflicted", second.Disposition, second.Reason)
	}
	if second.ResolverLimit != 2 {
		t.Fatalf("recovery reported limit %d; the mid-attempt config change to 5 must NOT apply — want the receipt's 2", second.ResolverLimit)
	}
	rec, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if rec.ResolverLimit != "2" || rec.ResolverUsed != "0" {
		t.Errorf("recovery re-snapshotted the receipt budget to limit %q used %q; want the owned 2/0", rec.ResolverLimit, rec.ResolverUsed)
	}
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

// TestFinalizeRebaseResolverBudgetWaitingReloadsForward proves the WAITING
// gate-continuation write copies the resolver-budget group forward from the
// freshly reloaded on-disk receipt, not from a stale in-memory copy.
func TestFinalizeRebaseResolverBudgetWaitingReloadsForward(t *testing.T) {
	f, gh, real := completedBudgetedReceipt(t, func(*workspace.RebaseReceipt) {}) // no gate pair
	ctx := context.Background()

	// Stale = the budget rolled back to its pre-reserve state; the gate pair is
	// empty so the recovery composes the gate afresh.
	stale := real
	stale.ResolverUsed = "0"
	stale.ResolverReservationToken = ""
	stale.ResolverReservationStopped = ""
	wrap := &staleFirstReadWorkspace{FinalizeWorkspace: f.svc, stale: stale}
	deps := FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: wrap,
		Gate: &fakeGate{result: LocalGateResult{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "drive-1", Generation: "gen-1"}}}}

	res := FinalizeRebase(ctx, deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Disposition != RebaseDispWaiting {
		t.Fatalf("waiting slice = %q (reason %q msg %q), want waiting", res.Disposition, res.Reason, res.Message)
	}
	after, present, err := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt after WAITING: present=%v err=%v", present, err)
	}
	if after.GateDriveID != "drive-1" || after.GateOwnerGeneration != "gen-1" {
		t.Fatalf("WAITING did not set the gate pair: %q/%q", after.GateDriveID, after.GateOwnerGeneration)
	}
	assertResolverFieldsEqual(t, "after WAITING set", real, after)
}

// TestFinalizeRebaseResolverBudgetClearReloadsForward proves the terminal
// gate-continuation clear copies the resolver-budget group forward from the
// freshly reloaded on-disk receipt, not from a stale in-memory copy.
func TestFinalizeRebaseResolverBudgetClearReloadsForward(t *testing.T) {
	f, gh, real := completedBudgetedReceipt(t, func(r *workspace.RebaseReceipt) {
		// A recorded WAITING drive so the recovery advances and then CLEARS it.
		r.GateDriveID = "drive-9"
		r.GateOwnerGeneration = "gen-9"
	})
	ctx := context.Background()

	// Stale = the budget rolled back to its pre-reserve state; the gate pair is
	// retained so composeLocalGate advances the recorded drive to its terminal.
	stale := real
	stale.ResolverUsed = "0"
	stale.ResolverReservationToken = ""
	stale.ResolverReservationStopped = ""
	wrap := &staleFirstReadWorkspace{FinalizeWorkspace: f.svc, stale: stale}
	deps := FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: wrap,
		Gate: &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}}

	res := FinalizeRebase(ctx, deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Result != ResultApplied || res.Gate == nil || res.Gate.Evidence == "" {
		t.Fatalf("passed slice = %q gate %+v (reason %q), want applied with evidence", res.Result, res.Gate, res.Reason)
	}
	after, present, err := f.svc.ReadRebaseReceipt(ctx, f.metaDir)
	if err != nil || !present {
		t.Fatalf("receipt after terminal: present=%v err=%v", present, err)
	}
	if after.GateDriveID != "" || after.GateOwnerGeneration != "" {
		t.Fatalf("terminal did not clear the gate pair: %q/%q", after.GateDriveID, after.GateOwnerGeneration)
	}
	assertResolverFieldsEqual(t, "after terminal clear", real, after)
}

// TestFinalizeRebaseGateOffCreatesNoReceipt pins that finalize.gate: off skips
// the rebase entirely — no receipt, hence no resolver budget, is created.
func TestFinalizeRebaseGateOffCreatesNoReceipt(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	writeRepoFile(t, f.repo.invocation, ".docket.local.yml", "finalize:\n  gate: \"off\"\n")
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if res.Result != ResultNoOp || res.Reason != ReasonRebaseGateOff {
		t.Fatalf("gate off = %q reason %q, want no-op/gate-off", res.Result, res.Reason)
	}
	f.receiptAbsent(t)
}
