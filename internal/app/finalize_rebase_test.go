package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
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

func TestFinalizeRebaseHappyAndReceipt(t *testing.T) {
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupRebaseFixture(t, m)
			f.advanceBase(t) // the base moves ahead: a real rewrite is required.

			gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
			gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
			deps := f.finalizeDeps(gh, gate)

			res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
				FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})

			if res.Result != ResultApplied || res.Disposition != RebaseDispRebased {
				t.Fatalf("rebase = %q disp %q (reason %q msg %q), want applied/rebased", res.Result, res.Disposition, res.Reason, res.Message)
			}
			// The rewrite actually moved the head.
			if newHead := f.localHead(); newHead == f.head {
				t.Fatalf("the feature head did not move; no rewrite happened")
			}
			// The receipt was written with the exact orig/base identities.
			rec, present, err := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
			if err != nil || !present {
				t.Fatalf("expected an owned receipt after the rewrite (present=%v err=%v)", present, err)
			}
			if rec.OrigHead != f.head {
				t.Errorf("receipt orig head = %q, want the pre-rebase head %q", rec.OrigHead, f.head)
			}
			if rec.BaseRef != string(f.target.BaseRef) {
				t.Errorf("receipt base ref = %q, want %q", rec.BaseRef, f.target.BaseRef)
			}
			if rec.OrigRemoteHead != f.head {
				t.Errorf("receipt orig remote head = %q, want the published head %q", rec.OrigRemoteHead, f.head)
			}
			// The owned recovery refs exist.
			if orig, err := tryGit(f.wp, "rev-parse", "refs/docket/finalize/5/orig"); err != nil || strings.TrimSpace(orig) != f.head {
				t.Errorf("owned orig ref = %q (err %v), want the pre-rebase head", orig, err)
			}
			if _, err := tryGit(f.wp, "rev-parse", "refs/docket/finalize/5/base"); err != nil {
				t.Errorf("owned base ref missing: %v", err)
			}
			// The gate ran (a real rewrite is never skipped) and produced evidence.
			if gate.calls != 1 {
				t.Errorf("gate calls = %d, want exactly 1 (a real rewrite runs the suite)", gate.calls)
			}
			if res.Gate == nil || res.Gate.Compose != gateComposeRan || res.Gate.Outcome != string(FinalizeGatePassed) || res.Gate.Evidence == "" {
				t.Errorf("gate report = %+v, want ran/passed with evidence", res.Gate)
			}
		})
	}
}

// --- TestFinalizeRebasePreconditions --------------------------------------

// TestFinalizeRebasePreconditions proves every precondition refusal leaves the
// receipt unwritten and Git untouched, with a closed reason. Preconditions are
// mode-independent, so the table runs in main mode.
func TestFinalizeRebasePreconditions(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("not-implemented", func(t *testing.T) {
		f := setupRebaseFixtureStatus(t, main, "in-progress")
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseNotImplemented)
		f.receiptAbsent(t)
		if f.localHead() != f.head {
			t.Errorf("a refused precondition moved the feature head")
		}
	})

	t.Run("version-drift", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: "sha256:" + strings.Repeat("b", 64), Head: f.head})
		assertRebaseRefused(t, res, ResultContended, ReasonRebaseVersionDrift)
		f.receiptAbsent(t)
	})

	t.Run("pr-base-mismatch", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		badPR := f.prForHead(f.head, "")
		badPR.BaseBranch = "some-other-branch"
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{badPR}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebasePRBaseMismatch)
		f.receiptAbsent(t)
	})

	t.Run("pr-head-mismatch", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		badPR := f.prForHead(strings.Repeat("c", 40), "")
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{badPR}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebasePRHeadMismatch)
		f.receiptAbsent(t)
	})

	t.Run("dirty-workspace", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		writeRepoFile(t, f.wp, "scratch.txt", "uncommitted\n")
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseWorkspaceDirty)
		f.receiptAbsent(t)
	})

	t.Run("local-head-mismatch", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		// Advance the local head off the expected head, but keep the PR (and req)
		// naming the old head: the workspace head no longer matches the authorization.
		writeRepoFile(t, f.wp, "more.txt", "more work\n")
		runGit(t, f.wp, "add", "-A")
		runGit(t, f.wp, "commit", "-q", "-m", "extra")
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultContended, ReasonRebaseLocalHeadMismatch)
		f.receiptAbsent(t)
	})

	t.Run("remote-head-mismatch", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		// Advance the remote feature head out of band, leaving the local head at the
		// expected head: local and remote no longer agree.
		runGit(t, f.wp, "commit", "-q", "--allow-empty", "-m", "remote-only")
		runGit(t, f.wp, "push", "-q", "origin", "HEAD:refs/heads/feat/"+f.slug)
		runGit(t, f.wp, "reset", "--hard", f.head)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseRemoteHeadMismatch)
		f.receiptAbsent(t)
	})
}

// assertRebaseRefused asserts a result carries the expected protocol result and
// reason, and never a success disposition.
func assertRebaseRefused(t *testing.T, res FinalizeRebaseResult, want Result, reason string) {
	t.Helper()
	if res.Result != want || res.Reason != reason {
		t.Fatalf("refusal = (%q, %q), want (%q, %q) [findings %v msg %q]", res.Result, res.Reason, want, reason, res.Findings, res.Message)
	}
}

// --- TestFinalizeRebaseGateOutcomes ---------------------------------------

// TestFinalizeRebaseGateOutcomes proves the gate composition maps each terminal:
// skip on a no-op with exact-head green evidence, passed to evidence, failed to
// repair work, and every non-decidable observation to a retained halt — never a
// fabricated red.
func TestFinalizeRebaseGateOutcomes(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("skip-on-noop-exact-green", func(t *testing.T) {
		f := setupRebaseFixture(t, main) // base unmoved: the rebase is a no-op.
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, greenEvidenceFor(t, f.head))}}
		gate := &fakeGate{}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispUnchanged || res.Gate == nil || res.Gate.Compose != gateComposeSkipped {
			t.Fatalf("noop+green = disp %q gate %+v, want unchanged/skipped", res.Disposition, res.Gate)
		}
		if gate.calls != 0 {
			t.Errorf("the suite was launched %d time(s) on a skip; want 0", gate.calls)
		}
		if res.Gate.Permit != f.head {
			t.Errorf("skip permit = %q, want the exact evidence head %q", res.Gate.Permit, f.head)
		}
	})

	t.Run("passed-produces-evidence", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultApplied || res.Disposition != RebaseDispRebased || res.Gate.Evidence == "" {
			t.Fatalf("passed = %q disp %q evidence?%v, want applied/rebased with evidence", res.Result, res.Disposition, res.Gate.Evidence != "")
		}
	})

	t.Run("failed-is-repair-work", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateFailed, RunDir: "/run/x"}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultGateFailed || res.Disposition != RebaseDispFailed || res.Reason != ReasonRebaseGateFailed {
			t.Fatalf("failed = (%q, %q, %q), want gate-failed/failed/gate-failed", res.Result, res.Disposition, res.Reason)
		}
		if res.Gate.Evidence != "" {
			t.Errorf("a failed gate produced evidence: %q", res.Gate.Evidence)
		}
	})

	t.Run("halt-at-budget-not-red", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltRunningAtBudget, RunDir: "/run/x"}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultBlocked || res.Disposition != RebaseDispBlocked || res.Reason != ReasonRebaseGateHalted {
			t.Fatalf("halt = (%q, %q, %q), want blocked/blocked/gate-halted", res.Result, res.Disposition, res.Reason)
		}
		if res.Gate.HaltCause != GateHaltRunningAtBudget {
			t.Errorf("halt cause = %q, want running-at-budget", res.Gate.HaltCause)
		}
	})

	t.Run("seam-error-is-halt-unavailable", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{err: errRebaseGateSeam}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Result != ResultBlocked || res.Gate.HaltCause != GateHaltUnavailable {
			t.Fatalf("seam error = %q halt %q, want blocked/unavailable", res.Result, res.Gate.HaltCause)
		}
	})
}

// --- TestFinalizeRebaseGateWaiting ----------------------------------------

// TestFinalizeRebaseGateWaiting proves the slice-bounded driver contract: a
// nonterminal WAITING slice returns a waiting disposition carrying the opaque
// continuation, mints NO evidence, and is NOT routed to integration-repair;
// re-entering the same local-gate phase with that continuation advances the SAME
// drive without repeating the completed rewrite, and a subsequent PASSED slice is
// the only outcome that mints evidence.
func TestFinalizeRebaseGateWaiting(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("waiting-returns-continuation-no-evidence-no-repair", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		cont := GateContinuation{DriveID: "drive-1", Generation: "gen-1"}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGateWaiting, Continuation: cont}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, gate), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if res.Disposition != RebaseDispWaiting {
			t.Fatalf("waiting disposition = %q, want %q (result %q reason %q)", res.Disposition, RebaseDispWaiting, res.Result, res.Reason)
		}
		if res.Result == ResultGateFailed || res.Disposition == RebaseDispFailed {
			t.Errorf("a WAITING slice was routed to repair; waiting is neither repair nor a red terminal")
		}
		if res.Gate == nil || res.Gate.Outcome != string(FinalizeGateWaiting) {
			t.Fatalf("gate report = %+v, want ran/waiting", res.Gate)
		}
		if res.Gate.Evidence != "" {
			t.Errorf("a WAITING slice minted evidence: %q", res.Gate.Evidence)
		}
		if res.Gate.Continuation == nil || *res.Gate.Continuation != cont {
			t.Fatalf("gate continuation = %+v, want %+v", res.Gate.Continuation, cont)
		}
	})

	t.Run("resume-advances-same-drive-without-repeating-rebase-then-mints-on-passed", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t) // the base moved: a real rewrite is required.
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		cont := GateContinuation{DriveID: "drive-9", Generation: "gen-9"}
		gate := &seqGate{results: []LocalGateResult{
			{Outcome: FinalizeGateWaiting, Continuation: cont},
			{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"},
		}}
		deps := f.finalizeDeps(gh, gate)

		first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if first.Disposition != RebaseDispWaiting || first.Gate == nil || first.Gate.Continuation == nil {
			t.Fatalf("first call = disp %q gate %+v, want waiting with a continuation", first.Disposition, first.Gate)
		}
		rewritten := f.localHead()
		recFirst, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)

		// Re-enter the SAME local-gate phase with the continuation the WAITING slice
		// returned. The rebase must not be repeated; only the gate advances.
		second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head, Continuation: *first.Gate.Continuation})
		if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
			t.Fatalf("resume = %q disp %q (reason %q msg %q), want applied/rebased", second.Result, second.Disposition, second.Reason, second.Message)
		}
		if second.Gate == nil || second.Gate.Evidence == "" {
			t.Fatalf("resume produced no evidence on PASSED: %+v", second.Gate)
		}
		// The completed rewrite was NOT repeated: the head and the owned attempt token
		// are unchanged across the re-entry.
		if f.localHead() != rewritten {
			t.Fatalf("resume repeated the rewrite: %q -> %q", rewritten, f.localHead())
		}
		recSecond, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
		if recSecond.Attempt != recFirst.Attempt {
			t.Errorf("resume minted a new rebase attempt %q; want the owned %q", recSecond.Attempt, recFirst.Attempt)
		}
		// The gate saw exactly two slices: the first STARTED (no continuation), the
		// second RESUMED with the exact continuation (Advance semantics).
		if len(gate.reqs) != 2 {
			t.Fatalf("gate slices = %d, want 2 (one per re-entry)", len(gate.reqs))
		}
		if gate.reqs[0].Continuation != (GateContinuation{}) {
			t.Errorf("the first slice carried a continuation %+v; it must start a fresh drive", gate.reqs[0].Continuation)
		}
		if gate.reqs[1].Continuation != cont {
			t.Errorf("resume slice continuation = %+v, want the first slice's %+v", gate.reqs[1].Continuation, cont)
		}
	})
}

// --- TestFinalizeRebaseResponseLossRecovery -------------------------------

// TestFinalizeRebaseResponseLossRecovery proves a replay after a completed rewrite
// (a lost response) adopts the same outcome from the receipt, the owned refs, the
// head, and the ancestry — and never rebases a different head.
func TestFinalizeRebaseResponseLossRecovery(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]
	f := setupRebaseFixture(t, main)
	f.advanceBase(t)
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)

	first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if first.Disposition != RebaseDispRebased {
		t.Fatalf("first call = %q, want rebased", first.Disposition)
	}
	rewritten := f.localHead()
	recFirst, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)

	// The response was lost; the same request is replayed. It must recover, not
	// rebase again.
	second := FinalizeRebase(context.Background(), deps, f.repo.invocation,
		FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
	if second.Result != ResultApplied || second.Disposition != RebaseDispRebased {
		t.Fatalf("replay = %q disp %q, want applied/rebased", second.Result, second.Disposition)
	}
	if f.localHead() != rewritten {
		t.Fatalf("the replay rebased a different head: %q -> %q", rewritten, f.localHead())
	}
	if second.Head != rewritten {
		t.Errorf("replay head = %q, want the already-rewritten head %q", second.Head, rewritten)
	}
	recSecond, _, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir)
	if recSecond.Attempt != recFirst.Attempt {
		t.Errorf("the replay minted a new attempt token %q; want the owned %q", recSecond.Attempt, recFirst.Attempt)
	}
}

// --- TestFinalizeRebaseForeignStateBlocked --------------------------------

// TestFinalizeRebaseForeignStateBlocked proves a moved base (a resumed rewrite
// whose base drifted) and a pre-existing foreign rebase are both retained and
// blocked, never reset or adopted.
func TestFinalizeRebaseForeignStateBlocked(t *testing.T) {
	requireRealGit(t)
	main := planRepoModes()[0]

	t.Run("moved-base", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		f.advanceBase(t)
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenEvidenceFor(t, f.head), RunDir: "/run/x"}}
		deps := f.finalizeDeps(gh, gate)
		first := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		if first.Disposition != RebaseDispRebased {
			t.Fatalf("first call = %q, want rebased", first.Disposition)
		}
		rewritten := f.localHead()

		// The base moves again under the recorded attempt.
		f.advanceBase(t)
		res := FinalizeRebase(context.Background(), deps, f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseMovedBase)
		if f.localHead() != rewritten {
			t.Errorf("a moved-base refusal reset the head: %q -> %q", rewritten, f.localHead())
		}
	})

	t.Run("foreign-rebase-in-progress", func(t *testing.T) {
		f := setupRebaseFixture(t, main)
		// A foreign rebase, started outside this operation, is left conflicted.
		f.repo.writerAdvance(t, "main", map[string]string{"feature.txt": "base-version conflicting\n"})
		runGit(t, f.wp, "fetch", "-q", "origin", "main")
		_, _ = tryGit(f.wp, "rebase", "origin/main") // conflicts, leaving a rebase in progress
		gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
		res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
			FinalizeRebaseRequest{ID: f.id, Version: f.version, Head: f.head})
		assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseForeignInProgress)
		f.receiptAbsent(t)
	})
}

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

// TestFinalizeRebaseContinueValidatesReport proves the resolver report is verified
// against the live unmerged set: a wrong attempt, a non-resolved disposition, and
// a path outside the unmerged set all refuse without staging, while a valid report
// stages exactly the named paths and completes the rebase.
func TestFinalizeRebaseContinueValidatesReport(t *testing.T) {
	requireRealGit(t)
	f, conflicted, deps := setupConflictedRebase(t, planRepoModes()[0])
	attempt := conflicted.Attempt
	ctx := context.Background()

	goodReport := ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved, ConflictedPaths: []string{"feature.txt"}}

	// A wrong attempt token refuses.
	wrong := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, "not-the-attempt", goodReport)
	assertRebaseRefused(t, wrong, ResultBlocked, ReasonRebaseAttemptMismatch)

	// A non-resolved report refuses (route through abort).
	stuck := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverStuck, ConflictedPaths: []string{"feature.txt"}})
	assertRebaseRefused(t, stuck, ResultInvalidInput, ReasonRebaseReportDisposition)

	// A path outside the live unmerged set refuses.
	badPaths := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverResolved, ConflictedPaths: []string{"not-conflicted.txt"}})
	assertRebaseRefused(t, badPaths, ResultInvalidInput, ReasonRebaseReportPaths)

	// The rebase is still live and conflicted: the refusals staged nothing.
	if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseConflicted {
		t.Fatalf("a refused continue changed the rebase state to %q", st.Disposition)
	}

	// The resolver resolves the file; a valid report stages exactly it and completes.
	writeRepoFile(t, f.wp, "feature.txt", "reconciled content\n")
	done := FinalizeRebaseContinue(ctx, deps, f.repo.invocation, f.id, attempt, goodReport)
	if done.Result != ResultApplied || done.Disposition != RebaseDispRebased {
		t.Fatalf("valid continue = %q disp %q (reason %q msg %q), want applied/rebased", done.Result, done.Disposition, done.Reason, done.Message)
	}
	if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseUnchanged {
		t.Errorf("the rebase did not complete; state %q", st.Disposition)
	}
}

// TestFinalizeRebaseAbortVerifiesRestore proves abort proves the owned attempt,
// restores the recorded original head, clears the owned scratch, and returns a
// blocked disposition recommending the human finalize-block — with the report body
// never echoed.
func TestFinalizeRebaseAbortVerifiesRestore(t *testing.T) {
	requireRealGit(t)
	f, conflicted, deps := setupConflictedRebase(t, planRepoModes()[0])
	attempt := conflicted.Attempt
	ctx := context.Background()

	// A wrong attempt cannot abort someone else's rewrite.
	wrong := FinalizeRebaseAbort(ctx, deps, f.repo.invocation, f.id, "not-the-attempt",
		ResolverReport{ChangeID: f.id, Attempt: "not-the-attempt", Disposition: ResolverStuck})
	assertRebaseRefused(t, wrong, ResultBlocked, ReasonRebaseAttemptMismatch)

	secret := "SECRET internal path detail that must never be echoed"
	res := FinalizeRebaseAbort(ctx, deps, f.repo.invocation, f.id, attempt,
		ResolverReport{ChangeID: f.id, Attempt: attempt, Disposition: ResolverStuck, Summary: secret, RecommendedAction: secret})
	if res.Result != ResultApplied || res.Disposition != RebaseDispBlocked {
		t.Fatalf("abort = %q disp %q, want applied/blocked", res.Result, res.Disposition)
	}
	if strings.Contains(res.Message, secret) {
		t.Errorf("the abort result echoed the report body: %q", res.Message)
	}
	// The original head is restored and the rebase is no longer in progress.
	if f.localHead() != f.head {
		t.Errorf("abort did not restore the original head: %q != %q", f.localHead(), f.head)
	}
	if st, _ := f.deps.Client.RebaseState(ctx, f.wp); st.Disposition != gitcli.RebaseUnchanged {
		t.Errorf("a rebase is still in progress after abort: %q", st.Disposition)
	}
	// The owned scratch is cleared.
	f.receiptAbsent(t)
	if _, err := tryGit(f.wp, "rev-parse", "--verify", "refs/docket/finalize/5/orig"); err == nil {
		t.Errorf("the owned orig ref survived abort")
	}
}
