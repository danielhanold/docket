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
func (f *fakeCloseoutGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
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

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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

	first := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
	if first.Result != ResultApplied || first.Disposition != CloseoutDispDoneArchived {
		t.Fatalf("first closeout = %q disp %q", first.Result, first.Disposition)
	}
	tipAfterFirst := originTip(t, f.repo.origin, f.branch)

	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
	if replay.Disposition != CloseoutDispAlready {
		t.Fatalf("stacked replay disposition = %q, want %q", replay.Disposition, CloseoutDispAlready)
	}
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
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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

	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id)
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
