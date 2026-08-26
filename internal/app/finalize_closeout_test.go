package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
)

// This file drives `finalize closeout` over a REAL metadata topology in BOTH
// repository modes (the same bare-remote main/docket matrix the other planning
// integration tests use, so the archive relocation, the derived-view rerenders,
// the whole-repository validation, and the exact-lease push run against real
// Git) plus a recording fake FinalizeGitHub that scripts the merged-PR reprobe a
// hermetic suite cannot reach. Closeout is the atomic terminal metadata
// transaction: a false `done` or a byte-corrupted merged artifact is the risk it
// is built to refuse, so every path proves the transaction landed nothing on a
// refusal and every authored byte outside a generated block survived a success.

// --- fake FinalizeGitHub for closeout -------------------------------------

// fakeCloseoutGitHub answers the GitHub calls `finalize closeout` makes
// (DiscoverRepository, ProbeMerged) from scripted per-number state. Every other
// finalize-half GitHub method panics so an accidental call is loud.
type fakeCloseoutGitHub struct {
	repo githubcli.Repository
	// merged maps a PR number to its authoritative reprobe outcome/facts.
	merged   map[int]closeoutProbe
	probeErr error
	probes   int
}

type closeoutProbe struct {
	outcome githubcli.MergeOutcome
	facts   githubcli.MergedFacts
}

func (f *fakeCloseoutGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return f.repo, nil
}

func (f *fakeCloseoutGitHub) ProbeMerged(_ context.Context, _ githubcli.Repository, number int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	f.probes++
	if f.probeErr != nil {
		return githubcli.MergeUnknown, githubcli.MergedFacts{}, f.probeErr
	}
	p, ok := f.merged[number]
	if !ok {
		// Cleanly not merged.
		return githubcli.MergeNotMergeable, githubcli.MergedFacts{}, nil
	}
	return p.outcome, p.facts, nil
}

func (f *fakeCloseoutGitHub) ViewPullRequest(context.Context, githubcli.Repository, int) (githubcli.PullRequest, error) {
	panic("ViewPullRequest: closeout must not call this")
}
func (f *fakeCloseoutGitHub) FindOpenPullRequestsByHead(context.Context, githubcli.Repository, string) ([]githubcli.PullRequest, error) {
	panic("FindOpenPullRequestsByHead: closeout must not call this")
}
func (f *fakeCloseoutGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: closeout must not call this")
}
func (f *fakeCloseoutGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: closeout must not call this")
}
func (f *fakeCloseoutGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: closeout must not call this")
}
func (f *fakeCloseoutGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeResult, error) {
	panic("MergePullRequest: closeout must not call this")
}

// --- closeout fixture -----------------------------------------------------

const (
	closeoutID  = 5
	closeoutPR  = 7
	closeoutRef = "github.com/acme/widget#7"
)

// closeoutFixture is a real published feature workspace whose parent record is
// implemented, carries the canonical PR reference, and points at a spec (on the
// metadata ref) plus a plan and results (on the integration ref in docket mode,
// on the shared ref in main mode) that each carry a docket:backlink block. It is
// exactly the state `finalize closeout` consumes after a verified merge.
type closeoutFixture struct {
	*rebaseFixture
	specPath    string
	planPath    string
	resultsPath string
	version     string
}

func closeoutRecord(id int, slug, status, pr, specPath, planPath, resultsPath string) string {
	// An implemented/stacked-merged record is coherent only when it carries the
	// claim facts (branch, claimed_at, reconciled) an in-progress base supplies; a
	// proposed base has none, which is what the illegal-source test wants.
	base := status
	coherentClaim := status == "implemented" || status == "stacked-merged"
	if coherentClaim {
		base = "in-progress"
	}
	rec := lifecycleChange(id, slug, base)
	if coherentClaim {
		rec = strings.Replace(rec, "status: in-progress\n", "status: "+status+"\n", 1)
	}
	rec = strings.Replace(rec, "blocked_by:\n", "pr: '"+pr+"'\nblocked_by:\n", 1)
	rec = strings.Replace(rec, "spec:\n", "spec: "+specPath+"\n", 1)
	rec = strings.Replace(rec, "plan:\n", "plan: "+planPath+"\n", 1)
	rec = strings.Replace(rec, "results:\n", "results: "+resultsPath+"\n", 1)
	return rec
}

// artifactWithBacklink renders a minimal spec/plan/results file: a docket:backlink
// managed block targeting the change's active path, then an authored body whose
// bytes must survive closeout unchanged.
func artifactWithBacklink(activePath, heading, body string) string {
	return "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
		"> ↩ **Change 0005 — A change** — `" + activePath + "`\n" +
		"<!-- docket:backlink:end -->\n\n" +
		"# " + heading + "\n\n" + body + "\n"
}

// setupCloseoutFixture builds the real published feature workspace, patches the
// parent record to carry the PR reference + artifact pointers, and seeds the
// spec/plan/results artifacts on their mode-correct refs.
func setupCloseoutFixture(t *testing.T, m planRepoMode) *closeoutFixture {
	t.Helper()
	f := setupRebaseFixture(t, m)
	cf := &closeoutFixture{
		rebaseFixture: f,
		specPath:      "docs/superpowers/specs/2026-08-16-widget-design.md",
		planPath:      "docs/superpowers/plans/2026-08-16-widget-plan.md",
		resultsPath:   "docs/changes/results/0005-widget-results.md",
	}
	recPath := groomPath(f.id, f.slug)
	// The record and the spec live on the metadata branch. The plan and results
	// live on the integration branch (main) — in docket mode a genuinely different
	// ref; in main mode the same ref as everything else.
	metaFiles := map[string]string{
		recPath:     closeoutRecord(f.id, f.slug, "implemented", closeoutRef, cf.specPath, cf.planPath, cf.resultsPath),
		cf.specPath: artifactWithBacklink(recPath, "Design", "The widget design."),
	}
	f.repo.writerAdvance(t, f.branch, metaFiles)

	integrationFiles := map[string]string{
		cf.planPath:    artifactWithBacklink(recPath, "Plan", "The widget plan."),
		cf.resultsPath: artifactWithBacklink(recPath, "Results", "The widget results."),
	}
	if m.name == "main" {
		// Same ref: the plan/results join the metadata branch too.
		f.repo.writerAdvance(t, f.branch, integrationFiles)
	} else {
		f.repo.writerAdvance(t, "main", integrationFiles)
	}

	cf.version = blobVersionAt(t, f.repo.origin, f.branch, recPath)
	return cf
}

// closeoutDeps assembles the FinalizeDeps a closeout test drives: the real
// planning seams and workspace service, plus the recording fake GitHub.
func (f *closeoutFixture) closeoutDeps(gh FinalizeGitHub) FinalizeDeps {
	return FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}
}

// mergeIntoBase makes a real merge commit on the integration branch (main) that
// carries the feature head, and returns its id — the authoritative merge commit
// the reachability proof must find reachable from main's tip.
func (f *closeoutFixture) mergeIntoBase(t *testing.T) string {
	t.Helper()
	runGit(t, f.repo.writer, "fetch", "-q", "origin", "feat/"+f.slug)
	runGit(t, f.repo.writer, "checkout", "-q", "main")
	runGit(t, f.repo.writer, "merge", "-q", "--no-ff", "-m", "Merge feat/"+f.slug, "FETCH_HEAD")
	mc := runGit(t, f.repo.writer, "rev-parse", "HEAD")
	runGit(t, f.repo.writer, "push", "-q", "origin", "main")
	return mc
}

// baselineMergedFake returns a fake whose PR #7 reprobes as merged into main at
// the given merge commit and the fixture head.
func (f *closeoutFixture) baselineMergedFake(head, mergeCommit string) *fakeCloseoutGitHub {
	return &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(head, "main", mergeCommit)},
		},
	}
}

// --- TestCloseoutOrdinary -------------------------------------------------

// TestCloseoutOrdinary proves an ordinary verified-integration merge is closed
// out in one transaction: the record is marked done and relocated to the dated
// archive path, its claim is cleared, its updated stamp is the merge date, its
// board is refreshed, and every backlink retargets to the archive path. It runs
// in both metadata modes.
func TestCloseoutOrdinary(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
			if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
				t.Fatalf("closeout = %q disp %q (reason %q msg %q)", res.Result, res.Disposition, res.Reason, res.Message)
			}
			archivePath := "docs/changes/archive/2026-08-18-0005-widget.md"
			if res.ArchivePath != archivePath {
				t.Fatalf("archive path = %q, want %q", res.ArchivePath, archivePath)
			}

			// The active record is gone; the archived record is done, claimless, and
			// stamped with the merge date.
			recPath := groomPath(f.id, f.slug)
			if _, ok := originFile(t, f.repo.origin, f.branch, recPath); ok {
				t.Errorf("active record still present after closeout (presence-encoded state)")
			}
			archived, ok := originFile(t, f.repo.origin, f.branch, archivePath)
			if !ok {
				t.Fatalf("archived record absent at %q", archivePath)
			}
			if !strings.Contains(archived, "status: 'done'") {
				t.Errorf("archived record not done:\n%s", archived)
			}
			if !strings.Contains(archived, "updated: '2026-08-18'") {
				t.Errorf("archived record not stamped with the merge date:\n%s", archived)
			}
			if strings.Contains(archived, "claimed_at: '2026-08-02") {
				t.Errorf("archived record still carries a claim stamp:\n%s", archived)
			}
			// Historical branch/PR fields survive.
			if !strings.Contains(archived, "pr: '"+closeoutRef+"'") {
				t.Errorf("archived record dropped its historical PR field:\n%s", archived)
			}

			// The spec backlink (metadata ref) retargets to the archive path.
			spec, _ := originFile(t, f.repo.origin, f.branch, f.specPath)
			if !strings.Contains(spec, archivePath) || strings.Contains(spec, "`"+recPath+"`") {
				t.Errorf("spec backlink not retargeted to the archive path:\n%s", spec)
			}

			// The plan/results backlinks (integration ref in docket mode) retarget too.
			integrationBranch := f.branch
			if m.name == "docket" {
				integrationBranch = "main"
			}
			for _, p := range []string{f.planPath, f.resultsPath} {
				got, ok := originFile(t, f.repo.origin, integrationBranch, p)
				if !ok {
					t.Fatalf("artifact %q vanished", p)
				}
				if !strings.Contains(got, archivePath) || strings.Contains(got, "`"+recPath+"`") {
					t.Errorf("artifact %q backlink not retargeted:\n%s", p, got)
				}
			}

			// The board is refreshed and current; no feature-branch state is deleted.
			assertBoardMatchesCommitted(t, f.repo.origin, f.branch, f.repo.invocation)
		})
	}
}

// --- TestCloseoutIdempotent -----------------------------------------------

// TestCloseoutIdempotent proves a replay after a response-lost success is a
// verified no-op keyed on the promised archive record, never a second commit.
func TestCloseoutIdempotent(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	first := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if first.Result != ResultApplied || first.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("first closeout = %q disp %q", first.Result, first.Disposition)
	}
	tipAfterFirst := originTip(t, f.repo.origin, f.branch)

	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if replay.Disposition != CloseoutDispAlready {
		t.Fatalf("replay disposition = %q, want %q (result %q)", replay.Disposition, CloseoutDispAlready, replay.Result)
	}
	if replay.Result == ResultApplied {
		t.Fatalf("replay reported a fresh apply; want a no-op")
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != tipAfterFirst {
		t.Errorf("replay produced a second commit: %q -> %q", tipAfterFirst, tip)
	}
}

// --- TestCloseoutRefusals -------------------------------------------------

// TestCloseoutRefusals proves an open PR, an unknown probe, a destination
// mismatch, and an illegal source status each refuse with a closed disposition
// and land nothing on the metadata ref.
func TestCloseoutRefusals(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("open-pr-not-merged", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		gh := &fakeCloseoutGitHub{repo: retargetRepo(), merged: map[int]closeoutProbe{}} // #7 not merged
		before := originTip(t, f.repo.origin, f.branch)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("a not-merged PR reported success %q", res.Result)
		}
		if res.Disposition != CloseoutDispBlocked {
			t.Errorf("disposition = %q, want %q", res.Disposition, CloseoutDispBlocked)
		}
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("a refusal moved the metadata ref: %q -> %q", before, after)
		}
	})

	t.Run("probe-unknown-retains", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		gh := &fakeCloseoutGitHub{repo: retargetRepo(), probeErr: errors.New("gh probe boom")}
		before := originTip(t, f.repo.origin, f.branch)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Disposition != CloseoutDispUnknown {
			t.Fatalf("disposition = %q, want %q (result %q)", res.Disposition, CloseoutDispUnknown, res.Result)
		}
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("an unknown probe moved the metadata ref: %q -> %q", before, after)
		}
	})

	t.Run("unreachable-merge-commit-contended", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		// Merged facts name main + a real object (the feature head) that is NOT
		// reachable from main's tip: a present-but-unreachable answer, contended.
		gh := f.baselineMergedFake(f.head, f.head)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultContended || res.Disposition != CloseoutDispContended {
			t.Fatalf("unreachable merge = %q disp %q, want contended", res.Result, res.Disposition)
		}
	})

	t.Run("illegal-source-status", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		// A proposed record is not a legal closeout source.
		recPath := groomPath(f.id, f.slug)
		f.repo.writerAdvance(t, f.branch, map[string]string{
			recPath: closeoutRecord(f.id, f.slug, "proposed", closeoutRef, f.specPath, f.planPath, f.resultsPath),
		})
		mergeCommit := f.mergeIntoBase(t)
		gh := f.baselineMergedFake(f.head, mergeCommit)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("closeout of a proposed record reported success %q", res.Result)
		}
		if res.Disposition != CloseoutDispBlocked {
			t.Errorf("disposition = %q, want %q", res.Disposition, CloseoutDispBlocked)
		}
	})
}

// --- TestCloseoutStackedMerged --------------------------------------------

// TestCloseoutStackedMerged proves a change whose verified PR destination is its
// live parent's branch is marked stacked-merged IN PLACE — not archived, its
// feature branch and workspace retained — and the board is rerendered.
func TestCloseoutStackedMerged(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)

	// A live parent (id 4) the fixture child (id 5) stacks on; the child's PR
	// merged into the parent's feature branch feat/parent.
	recPath := groomPath(f.id, f.slug)
	child := closeoutRecord(f.id, f.slug, "implemented", closeoutRef, f.specPath, f.planPath, f.resultsPath)
	child = strings.Replace(child, "stacked_on:\n", "stacked_on: 4\n", 1)
	f.repo.writerAdvance(t, f.branch, map[string]string{
		groomPath(4, "parent"): lifecycleChange(4, "parent", "in-progress"),
		recPath:                child,
	})

	gh := &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/parent", strings.Repeat("a", 40))},
		},
	}
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispStackedMerged {
		t.Fatalf("closeout = %q disp %q (reason %q msg %q)", res.Result, res.Disposition, res.Reason, res.Message)
	}

	// The record is edited in place, not archived.
	rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
	if !ok {
		t.Fatalf("stacked-merged record was archived away from %q", recPath)
	}
	if !strings.Contains(rec, "status: 'stacked-merged'") {
		t.Errorf("record not stacked-merged:\n%s", rec)
	}
	if _, ok := originFile(t, f.repo.origin, f.branch, "docs/changes/archive/2026-08-18-0005-widget.md"); ok {
		t.Errorf("a stacked-merged closeout archived the record")
	}
	assertBoardMatchesCommitted(t, f.repo.origin, f.branch, f.repo.invocation)

	// A replay is a verified no-op keyed on the promised stacked-merged state.
	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if replay.Disposition != CloseoutDispAlready {
		t.Fatalf("stacked replay disposition = %q, want %q", replay.Disposition, CloseoutDispAlready)
	}
}

// TestCloseoutStackedParentBranchIdentity proves the stacked-merged path reads
// the live PARENT's OWN recorded branch (never a slug-derived name): a merge
// into the parent's non-derived recorded branch takes the in-place stacked path,
// and a live parent whose record carries no branch fails closed to invalid-state
// with the child record left untouched.
func TestCloseoutStackedParentBranchIdentity(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	seedChild := func(t *testing.T, f *closeoutFixture, parentRecord string) string {
		t.Helper()
		recPath := groomPath(f.id, f.slug)
		child := closeoutRecord(f.id, f.slug, "implemented", closeoutRef, f.specPath, f.planPath, f.resultsPath)
		child = strings.Replace(child, "stacked_on:\n", "stacked_on: 4\n", 1)
		f.repo.writerAdvance(t, f.branch, map[string]string{
			groomPath(4, "parent"): parentRecord,
			recPath:                child,
		})
		return recPath
	}

	t.Run("non-derived-parent-branch-honored", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		parent := strings.Replace(lifecycleChange(4, "parent", "in-progress"), "branch: feat/parent\n", "branch: feature/live-parent\n", 1)
		recPath := seedChild(t, f, parent)
		gh := &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feature/live-parent", strings.Repeat("a", 40))},
			},
		}
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultApplied || res.Disposition != CloseoutDispStackedMerged {
			t.Fatalf("closeout = %q disp %q (reason %q msg %q); the parent's recorded non-derived branch must anchor the stacked path", res.Result, res.Disposition, res.Reason, res.Message)
		}
		rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
		if !ok || !strings.Contains(rec, "status: 'stacked-merged'") {
			t.Errorf("record not marked stacked-merged in place:\n%s", rec)
		}
	})

	t.Run("missing-parent-branch-refuses-untouched", func(t *testing.T) {
		f := setupCloseoutFixture(t, m)
		parent := strings.Replace(lifecycleChange(4, "parent", "in-progress"), "branch: feat/parent\n", "", 1)
		recPath := seedChild(t, f, parent)
		gh := &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feature/live-parent", strings.Repeat("a", 40))},
			},
		}
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultInvalidState {
			t.Fatalf("closeout = %q disp %q reason %q, want invalid-state on a live parent with no recorded branch", res.Result, res.Disposition, res.Reason)
		}
		rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
		if !ok || strings.Contains(rec, "stacked-merged") {
			t.Errorf("a refused stacked closeout must leave the child record untouched (never marked stacked-merged):\n%s", rec)
		}
	})
}

// --- TestCloseoutRootCarry ------------------------------------------------

// TestCloseoutRootCarry proves a stack root merged to integration archives the
// root plus every proven carried descendant in ONE transaction using the root's
// merge date for every filename; a single unproven descendant keeps the root
// recoverable with zero descendant writes.
func TestCloseoutRootCarry(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0] // main mode: one ref carries every backlink

	descPlan := "docs/superpowers/plans/2026-08-16-gadget-plan.md"

	// carriedDescendant returns a descendant record (id 6, gadget) stacked on the
	// root (id 5) with the given status.
	seed := func(t *testing.T, descStatus string) *closeoutFixture {
		f := setupCloseoutFixture(t, m)
		recPath := groomPath(6, "gadget")
		desc := closeoutRecord(6, "gadget", descStatus, "github.com/acme/widget#8", "", descPlan, "")
		desc = strings.Replace(desc, "stacked_on:\n", "stacked_on: 5\n", 1)
		f.repo.writerAdvance(t, f.branch, map[string]string{
			recPath:  desc,
			descPlan: artifactWithBacklink(recPath, "Gadget plan", "The gadget plan."),
		})
		return f
	}

	t.Run("all-proven-archives-root-and-descendants", func(t *testing.T) {
		f := seed(t, "stacked-merged")
		mergeCommit := f.mergeIntoBase(t)
		gh := &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "main", mergeCommit)},
				8:          {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(strings.Repeat("c", 40), "feat/widget", strings.Repeat("b", 40))},
			},
		}
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result != ResultApplied || res.Disposition != CloseoutDispRootArchived {
			t.Fatalf("root carry = %q disp %q (reason %q msg %q)", res.Result, res.Disposition, res.Reason, res.Message)
		}
		// Both records archived under the ROOT's merge date.
		for _, p := range []string{
			"docs/changes/archive/2026-08-18-0005-widget.md",
			"docs/changes/archive/2026-08-18-0006-gadget.md",
		} {
			archived, ok := originFile(t, f.repo.origin, f.branch, p)
			if !ok {
				t.Fatalf("archived record absent at %q", p)
			}
			if !strings.Contains(archived, "status: 'done'") {
				t.Errorf("archived record %q not done:\n%s", p, archived)
			}
		}
		if got := res.CarriedIDs; len(got) != 1 || got[0] != 6 {
			t.Errorf("carried ids = %v, want [6]", got)
		}
		assertBoardMatchesCommitted(t, f.repo.origin, f.branch, f.repo.invocation)
	})

	t.Run("one-unproven-descendant-keeps-root-recoverable", func(t *testing.T) {
		f := seed(t, "implemented") // NOT stacked-merged: carry unproven
		mergeCommit := f.mergeIntoBase(t)
		gh := &fakeCloseoutGitHub{
			repo: retargetRepo(),
			merged: map[int]closeoutProbe{
				closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "main", mergeCommit)},
			},
		}
		before := originTip(t, f.repo.origin, f.branch)
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("an unproven descendant let the root close out: %q", res.Result)
		}
		if res.Disposition != CloseoutDispChildrenRetargetRequired {
			t.Errorf("disposition = %q, want %q", res.Disposition, CloseoutDispChildrenRetargetRequired)
		}
		// Zero writes: neither record moved.
		if after := originTip(t, f.repo.origin, f.branch); after != before {
			t.Errorf("an unproven root carry moved the metadata ref: %q -> %q", before, after)
		}
		if _, ok := originFile(t, f.repo.origin, f.branch, groomPath(f.id, f.slug)); !ok {
			t.Errorf("the recoverable root was archived away")
		}
	})
}

// --- TestCloseoutBacklinkLegDocketMode ------------------------------------

// TestCloseoutBacklinkLegDocketMode proves the docket-mode split: the metadata
// transaction lands the archived record + spec + board on the metadata ref, and a
// SEPARATE isolated integration-ref commit patches only the merged plan/results
// backlinks — never a metadata record, never an authored byte.
func TestCloseoutBacklinkLegDocketMode(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[1] // docket
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	metaBefore := originTip(t, f.repo.origin, "docket")
	mainBefore := originTip(t, f.repo.origin, "main")

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}
	// No terminal-backlink-pending finding: the leg landed.
	for _, fd := range res.Findings {
		if fd.Code == ReasonCloseoutBacklinkPending {
			t.Fatalf("the backlink leg did not land: %+v", fd)
		}
	}

	// Both refs advanced, on separate commits.
	metaAfter := originTip(t, f.repo.origin, "docket")
	mainAfter := originTip(t, f.repo.origin, "main")
	if metaAfter == metaBefore {
		t.Errorf("the metadata ref did not advance")
	}
	if mainAfter == mainBefore {
		t.Errorf("the integration ref did not advance (no backlink leg)")
	}

	// The integration-ref commit touched ONLY the merged plan/results — no metadata
	// record crossed onto the integration branch.
	got := originCommitPaths(t, f.repo.origin, mainAfter)
	want := []string{f.planPath, f.resultsPath}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("integration-ref commit changed %v, want exactly %v", got, want)
	}
}

// --- TestCloseoutNeverEditsAuthoredBytes ----------------------------------

// TestCloseoutNeverEditsAuthoredBytes proves the authored content of the merged
// plan/results outside the docket:backlink block is byte-identical after a
// closeout.
func TestCloseoutNeverEditsAuthoredBytes(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
			if res.Result != ResultApplied {
				t.Fatalf("closeout did not apply: %q (reason %q)", res.Result, res.Reason)
			}

			integrationBranch := f.branch
			if m.name == "docket" {
				integrationBranch = "main"
			}
			for _, p := range []string{f.planPath, f.resultsPath} {
				got, ok := originFile(t, f.repo.origin, integrationBranch, p)
				if !ok {
					t.Fatalf("artifact %q vanished", p)
				}
				// The authored body after the backlink block survives verbatim.
				_, body, found := strings.Cut(got, "<!-- docket:backlink:end -->\n")
				if !found {
					t.Fatalf("artifact %q lost its backlink block:\n%s", p, got)
				}
				if !strings.HasPrefix(body, "\n# ") {
					t.Errorf("artifact %q authored body was disturbed:\n%q", p, body)
				}
			}
		})
	}
}

// --- closeout notes -------------------------------------------------------

// closeoutTestNotes is the both-category request the spec's rendering example uses.
func closeoutTestNotes() CloseoutNotes {
	return CloseoutNotes{
		VerificationOutcomes: []string{"Production health check passed after deployment"},
		LateFindings:         []string{"The upgrade guide should mention the legacy config cleanup"},
	}
}

const closeoutWantNotesSection = "## Closeout notes\n\n" +
	"### Verification\n\n" +
	"- Production health check passed after deployment\n\n" +
	"### Late findings\n\n" +
	"- The upgrade guide should mention the legacy config cleanup\n"

// TestCloseoutNotesLandWithArchive proves notes land in the SAME transaction as
// the ordinary archive, as the final section, in both repository modes — and
// that the lifecycle transition still happened.
func TestCloseoutNotesLandWithArchive(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
			if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
				t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
			}
			archived, ok := originFile(t, f.repo.origin, f.branch, res.ArchivePath)
			if !ok {
				t.Fatalf("archived record absent at %q", res.ArchivePath)
			}
			if !strings.HasSuffix(archived, closeoutWantNotesSection) {
				t.Errorf("archived record does not END with the notes section:\n%s", archived)
			}
			if !strings.Contains(archived, "status: 'done'") {
				t.Errorf("notes landed without the lifecycle transition:\n%s", archived)
			}
		})
	}
}

// TestCloseoutNoNotesEmitsNoSection pins the byte-for-byte-today promise.
func TestCloseoutNoNotesEmitsNoSection(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied {
		t.Fatalf("closeout = %q", res.Result)
	}
	archived, _ := originFile(t, f.repo.origin, f.branch, res.ArchivePath)
	if strings.Contains(archived, "## Closeout notes") {
		t.Errorf("no-notes closeout conjured a notes section:\n%s", archived)
	}
}

// TestCloseoutNotesInvalidInputMutatesNothing: empty-after-trim, control
// characters, marker text, and an oversized entry each refuse before any
// probe or transaction — the change stays implemented and the tip unmoved.
func TestCloseoutNotesInvalidInputMutatesNothing(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	tipBefore := originTip(t, f.repo.origin, f.branch)

	bad := []CloseoutNotes{
		{VerificationOutcomes: []string{"   "}},
		{LateFindings: []string{"bell\x07"}},
		{LateFindings: []string{"crlf\r\nmid"}}, // interior CR survives trimming; must be rejected
		{VerificationOutcomes: []string{"<!-- docket:backlink:start -->"}},
		{VerificationOutcomes: []string{strings.Repeat("a", maxAuthoredMarkdownBytes+1)}},
	}
	for i, n := range bad {
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, n)
		if res.Result != ResultInvalidInput {
			t.Fatalf("bad[%d] result = %q, want invalid input", i, res.Result)
		}
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != tipBefore {
		t.Errorf("an invalid-notes request moved the metadata tip: %q -> %q", tipBefore, tip)
	}
	if gh.probes != 0 {
		t.Errorf("an invalid-notes request reached the PR probe (%d probes)", gh.probes)
	}
}

// TestCloseoutNotesReplayAndFrozen: an identical-notes retry replays as a
// no-op with no second commit; a different-notes retry is refused with
// terminal-notes-frozen and moves nothing.
func TestCloseoutNotesReplayAndFrozen(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	first := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if first.Result != ResultApplied {
		t.Fatalf("first closeout = %q", first.Result)
	}
	tipAfterFirst := originTip(t, f.repo.origin, f.branch)

	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if replay.Disposition != CloseoutDispAlready || replay.Result == ResultApplied {
		t.Fatalf("identical-notes replay = %q disp %q, want no-op already", replay.Result, replay.Disposition)
	}

	different := closeoutTestNotes()
	different.LateFindings = []string{"a different late finding"}
	refused := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, different)
	if refused.Reason != ReasonCloseoutNotesFrozen {
		t.Fatalf("different-notes retry reason = %q, want %q (result %q disp %q)",
			refused.Reason, ReasonCloseoutNotesFrozen, refused.Result, refused.Disposition)
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != tipAfterFirst {
		t.Errorf("a refused retry produced a commit")
	}
	archived, _ := originFile(t, f.repo.origin, f.branch, first.ArchivePath)
	if !strings.HasSuffix(archived, closeoutWantNotesSection) {
		t.Errorf("terminal record's notes changed after the refused retry:\n%s", archived)
	}
}

// TestCloseoutNotesStackedInPlace proves the stacked-merged in-place path also
// carries notes into the terminal record, with the same replay/frozen semantics.
func TestCloseoutNotesStackedInPlace(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)

	recPath := groomPath(f.id, f.slug)
	child := closeoutRecord(f.id, f.slug, "implemented", closeoutRef, f.specPath, f.planPath, f.resultsPath)
	child = strings.Replace(child, "stacked_on:\n", "stacked_on: 4\n", 1)
	f.repo.writerAdvance(t, f.branch, map[string]string{
		groomPath(4, "parent"): lifecycleChange(4, "parent", "in-progress"),
		recPath:                child,
	})

	gh := &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "feat/parent", strings.Repeat("a", 40))},
		},
	}
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if res.Result != ResultApplied || res.Disposition != CloseoutDispStackedMerged {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}

	rec, ok := originFile(t, f.repo.origin, f.branch, recPath)
	if !ok {
		t.Fatalf("stacked-merged record was archived away from %q", recPath)
	}
	if !strings.Contains(rec, "status: 'stacked-merged'") {
		t.Errorf("record not stacked-merged:\n%s", rec)
	}
	if !strings.HasSuffix(rec, closeoutWantNotesSection) {
		t.Errorf("in-place record does not END with the notes section:\n%s", rec)
	}

	// Identical-notes replay is a no-op; different notes are frozen.
	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if replay.Disposition != CloseoutDispAlready || replay.Result == ResultApplied {
		t.Fatalf("stacked identical-notes replay = %q disp %q, want no-op already", replay.Result, replay.Disposition)
	}
	different := closeoutTestNotes()
	different.LateFindings = []string{"a different late finding"}
	refused := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, different)
	if refused.Reason != ReasonCloseoutNotesFrozen {
		t.Fatalf("stacked different-notes retry reason = %q, want %q (result %q)", refused.Reason, ReasonCloseoutNotesFrozen, refused.Result)
	}
}

// TestCloseoutNotesRootCarryNoPropagation proves root notes land ONLY on the
// root: a carried descendant's own notes survive root archival, and the root's
// notes never propagate onto the descendant.
func TestCloseoutNotesRootCarryNoPropagation(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0] // main mode: one ref carries every backlink

	descPlan := "docs/superpowers/plans/2026-08-16-gadget-plan.md"
	f := setupCloseoutFixture(t, m)
	childPath := groomPath(6, "gadget")
	desc := closeoutRecord(6, "gadget", "implemented", "github.com/acme/widget#8", "", descPlan, "")
	desc = strings.Replace(desc, "stacked_on:\n", "stacked_on: 5\n", 1)
	f.repo.writerAdvance(t, f.branch, map[string]string{
		childPath: desc,
		descPlan:  artifactWithBacklink(childPath, "Gadget plan", "The gadget plan."),
	})

	// Establish the ROOT's verified merge into integration up front (in main mode
	// the metadata ref IS main, so this must land before any closeout advances it).
	mergeCommit := f.mergeIntoBase(t)

	// First: close out the CHILD (id 6) as stacked-merged into the root's branch,
	// carrying its OWN notes.
	childNotes := CloseoutNotes{LateFindings: []string{"child-owned note"}}
	ghChild := &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			8: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(strings.Repeat("c", 40), "feat/widget", strings.Repeat("b", 40))},
		},
	}
	childRes := FinalizeCloseout(context.Background(), f.closeoutDeps(ghChild), f.repo.invocation, 6, childNotes)
	if childRes.Result != ResultApplied || childRes.Disposition != CloseoutDispStackedMerged {
		t.Fatalf("child closeout = %q disp %q (reason %q)", childRes.Result, childRes.Disposition, childRes.Reason)
	}

	// Then: carry the ROOT (id 5) with DIFFERENT notes.
	ghRoot := &fakeCloseoutGitHub{
		repo: retargetRepo(),
		merged: map[int]closeoutProbe{
			closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "main", mergeCommit)},
			8:          {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(strings.Repeat("c", 40), "feat/widget", strings.Repeat("b", 40))},
		},
	}
	rootRes := FinalizeCloseout(context.Background(), f.closeoutDeps(ghRoot), f.repo.invocation, f.id, closeoutTestNotes())
	if rootRes.Result != ResultApplied || rootRes.Disposition != CloseoutDispRootArchived {
		t.Fatalf("root carry = %q disp %q (reason %q)", rootRes.Result, rootRes.Disposition, rootRes.Reason)
	}

	// (a) the archived ROOT record ends with the root's notes section.
	rootArchived, ok := originFile(t, f.repo.origin, f.branch, "docs/changes/archive/2026-08-18-0005-widget.md")
	if !ok {
		t.Fatalf("root archived record absent")
	}
	if !strings.HasSuffix(rootArchived, closeoutWantNotesSection) {
		t.Errorf("archived ROOT record does not end with its notes section:\n%s", rootArchived)
	}

	childArchived, ok := originFile(t, f.repo.origin, f.branch, "docs/changes/archive/2026-08-18-0006-gadget.md")
	if !ok {
		t.Fatalf("child archived record absent")
	}
	// (b) the root's notes never propagated onto the descendant.
	if strings.Contains(childArchived, "Production health check") {
		t.Errorf("root notes propagated to the carried child:\n%s", childArchived)
	}
	// (c) the child's own notes survived root archival.
	if !strings.Contains(childArchived, "child-owned note") {
		t.Errorf("child's own notes did not survive root archival:\n%s", childArchived)
	}
}

// --- TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors ------------------

// TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors is the 0337 regression:
// the integration branch carries a pre-existing corpus record the mutation
// never touches whose bytes fail document.Parse (an ADR with an unquoted
// colon-space title — the live ADR-0024 trigger). The backlink-only patch must
// LAND anyway: the leg's gate is scoped to the artifacts it patches, not the
// health of the integration branch's partial corpus.
func TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	// Pre-existing, mutation-unrelated corpus error on the integration branch.
	f.repo.writerAdvance(t, "main", map[string]string{
		"docs/adrs/0099-malformed.md": "---\n" +
			"id: 99\n" +
			"title: uses `context: fork` dispatch\n" +
			"status: Accepted\n" +
			"date: 2026-08-22\n" +
			"---\n\n# 99. Malformed on purpose\n",
	})
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	mainBefore := originTip(t, f.repo.origin, "main")

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}
	// The leg LANDED: no pending finding, and the integration ref advanced.
	for _, fd := range res.Findings {
		if fd.Code == ReasonCloseoutBacklinkPending {
			t.Fatalf("unrelated corpus error refused the backlink leg: %+v", fd)
		}
	}
	mainAfter := originTip(t, f.repo.origin, "main")
	if mainAfter == mainBefore {
		t.Fatalf("the integration ref did not advance (no backlink leg)")
	}
	// The leg's commit touched exactly the plan/results — the malformed record
	// and every other corpus byte are untouched.
	got := originCommitPaths(t, f.repo.origin, mainAfter)
	want := []string{f.planPath, f.resultsPath}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("integration-ref commit changed %v, want exactly %v", got, want)
	}
	// The retarget itself happened: the plan now backlinks the archive path.
	plan, ok := originFile(t, f.repo.origin, "main", f.planPath)
	if !ok {
		t.Fatalf("plan artifact vanished from main")
	}
	if !strings.Contains(plan, res.ArchivePath) {
		t.Errorf("plan backlink does not point at the archive path %q:\n%s", res.ArchivePath, plan)
	}
	if !strings.Contains(plan, "# Plan\n\nThe widget plan.") {
		t.Errorf("authored plan body disturbed:\n%s", plan)
	}
}

// --- TestCloseoutBacklinkPendingFindingNamesTheCause ----------------------

// TestCloseoutBacklinkPendingFindingNamesTheCause is the 0337 diagnosability
// proof (spec D): when the leg still cannot land — here an IN-SCOPE failure,
// the targeted plan artifact's own bytes fail document.Parse — the
// terminal-backlink-pending finding carries the typed cause (the offending
// artifact path), never a bare coarse token. The change itself still closes
// out done+archived: the leg stays best-effort.
func TestCloseoutBacklinkPendingFindingNamesTheCause(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	// Corrupt the targeted plan artifact on the integration branch: malformed
	// frontmatter fails document.Parse, an in-scope condition even after the
	// gate is scoped to the patched artifacts.
	f.repo.writerAdvance(t, "main", map[string]string{
		f.planPath: "---\ntitle: uses `context: fork` dispatch\n---\n\n" +
			artifactWithBacklink(groomPath(f.id, f.slug), "Plan", "The widget plan."),
	})
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
	}
	var pending *StatusFinding
	for i, fd := range res.Findings {
		if fd.Code == ReasonCloseoutBacklinkPending {
			pending = &res.Findings[i]
		}
	}
	if pending == nil {
		t.Fatalf("an in-scope malformed artifact did not leave the leg pending: %+v", res.Findings)
	}
	// The finding names the cause: the exact offending artifact path, and the
	// typed detail separator — proof it carries more than the old coarse token.
	if !strings.Contains(pending.Message, f.planPath) {
		t.Errorf("pending finding does not name the offending artifact:\n%s", pending.Message)
	}
	if !strings.Contains(pending.Message, "): ") {
		t.Errorf("pending finding carries no typed detail (still cause-free): %q", pending.Message)
	}
}
