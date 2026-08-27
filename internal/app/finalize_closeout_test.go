package app

import (
	"context"
	"github.com/danielhanold/docket/internal/githubcli"
	"strings"
	"testing"
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

// --- TestCloseoutIdempotent -----------------------------------------------

// --- TestCloseoutRefusals -------------------------------------------------

// --- TestCloseoutStackedMerged --------------------------------------------

// --- TestCloseoutRootCarry ------------------------------------------------

// --- TestCloseoutBacklinkLegDocketMode ------------------------------------

// --- TestCloseoutNeverEditsAuthoredBytes ----------------------------------

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

	// --- TestCloseoutBacklinkLegIgnoresUnrelatedCorpusErrors ------------------

	// --- TestCloseoutBacklinkPendingFindingNamesTheCause ----------------------
