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
			skip, permit := gateDecision(tc.noop, tc.evidenceHead, tc.currentHead, tc.green)
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
