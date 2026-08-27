package app

import (
	"context"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
	"strings"
	"testing"
)

// This file drives `finalize merge` over a REAL feature workspace (the same
// bare-remote topology, gitcli.Client, and workspace.Service the rebase/publish
// tests use — so the effective-base resolution, the workspace-head agreement,
// and the merge-commit reachability proof run against real Git) plus a
// recording fake FinalizeGitHub that scripts the merge/reprobe outcomes a
// hermetic suite cannot reach. The expected-head GitHub merge, the authoritative
// reprobe, and the Git reachability proof are the highest-consequence external
// effect in the terminal path, so every conjunct is rechecked from a fresh
// reload immediately before the effect and no merge call is issued once any
// conjunct is falsified.

// --- fake FinalizeGitHub for merge ----------------------------------------

// fakeMergeGitHub answers the GitHub calls `finalize merge` makes
// (DiscoverRepository, ProbeMerged, FindOpenPullRequestsByHead, MergePullRequest)
// from scripted state, and records every MergePullRequest call so a test can
// prove a refused merge issued zero merge calls and a merged/already-merged PR
// issued at most one. Every other finalize-half GitHub method panics so an
// accidental call is loud.
type fakeMergeGitHub struct {
	repo githubcli.Repository

	// ProbeMerged(number) result. Defaults to not-mergeable (cleanly not merged).
	probeOutcome githubcli.MergeOutcome
	probeFacts   githubcli.MergedFacts
	probeErr     error

	// Open PRs by head branch. The parent head resolves the live PR the merge
	// gates on; child heads resolve the open-child probe.
	openByHead map[string][]githubcli.PullRequest
	findErr    error

	// MergePullRequest result and recorded call state.
	mergeOutcome       githubcli.MergeOutcome
	mergeMethod        githubcli.MergeMethod
	mergeRepoMethods   []githubcli.MergeMethod
	mergeBranchMethods []githubcli.MergeMethod
	mergeFacts         githubcli.MergedFacts
	mergeErr           error
	mergeCalls         int
	lastMergeAdmin     bool
	lastMergeHead      githubcli.ObjectRef
	lastMergeNum       int
}

func (f *fakeMergeGitHub) DiscoverRepository(context.Context, string) (githubcli.Repository, error) {
	return f.repo, nil
}

func (f *fakeMergeGitHub) ProbeMerged(_ context.Context, _ githubcli.Repository, _ int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	if f.probeErr != nil {
		return githubcli.MergeUnknown, githubcli.MergedFacts{}, f.probeErr
	}
	if f.probeOutcome == "" {
		return githubcli.MergeNotMergeable, githubcli.MergedFacts{}, nil
	}
	return f.probeOutcome, f.probeFacts, nil
}

func (f *fakeMergeGitHub) ViewPullRequest(context.Context, githubcli.Repository, int) (githubcli.PullRequest, error) {
	panic("ViewPullRequest: merge must not call this")
}
func (f *fakeMergeGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, head string) ([]githubcli.PullRequest, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.openByHead[head], nil
}

func (f *fakeMergeGitHub) MergePullRequest(_ context.Context, _ githubcli.Repository, number int, expectedHead githubcli.ObjectRef, admin bool) (githubcli.MergeResult, error) {
	f.mergeCalls++
	f.lastMergeAdmin = admin
	f.lastMergeHead = expectedHead
	f.lastMergeNum = number
	if f.mergeErr != nil {
		return githubcli.MergeResult{Outcome: githubcli.MergeUnknown}, f.mergeErr
	}
	return githubcli.MergeResult{
		Outcome:       f.mergeOutcome,
		Method:        f.mergeMethod,
		Facts:         f.mergeFacts,
		RepoMethods:   f.mergeRepoMethods,
		BranchMethods: f.mergeBranchMethods,
	}, nil
}

func (f *fakeMergeGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: merge must not call this")
}
func (f *fakeMergeGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: merge must not call this")
}
func (f *fakeMergeGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: merge must not call this")
}

// --- merge fixture --------------------------------------------------------

const mergeCanonicalPRNumber = 7

func mergePRRef() string { return "github.com/acme/widget#7" }

// mergeFixture is a real published feature workspace whose parent record carries
// a canonical PR reference — the exact state `finalize merge` consumes.
type mergeFixture struct {
	*rebaseFixture
	version string // the fresh record blob version after the pr-reference patch
}

// mergeParentRecord is the parent lifecycle record carrying a canonical PR
// reference, and optionally an appended authored body section (e.g. a durable
// "## Finalize blocked" marker).
func mergeParentRecord(id int, slug, status, pr, extraBody string) string {
	rec := lifecycleChange(id, slug, status)
	rec = strings.Replace(rec, "blocked_by:\n", "pr: '"+pr+"'\nblocked_by:\n", 1)
	if extraBody != "" {
		rec += "\n" + extraBody + "\n"
	}
	return rec
}

// childRecord is a direct stack child (stacked_on the parent) with a canonical
// PR reference of its own.
func childRecord(id int, slug string, parent int, pr string) string {
	rec := lifecycleChange(id, slug, "implemented")
	rec = strings.Replace(rec, "stacked_on:\n", "stacked_on: "+itoaTest(parent)+"\n", 1)
	rec = strings.Replace(rec, "blocked_by:\n", "pr: '"+pr+"'\nblocked_by:\n", 1)
	return rec
}

// patchParent rewrites the parent record on the metadata branch and returns the
// fresh blob version.
func (f *mergeFixture) patchParent(t *testing.T, status, pr, extraBody string) string {
	t.Helper()
	f.repo.writerAdvance(t, f.branch, map[string]string{groomPath(f.id, f.slug): mergeParentRecord(f.id, f.slug, status, pr, extraBody)})
	f.version = blobVersionAt(t, f.repo.origin, f.branch, groomPath(f.id, f.slug))
	return f.version
}

// setupMergeFixture builds the real published feature workspace and patches the
// parent record to carry the canonical PR reference merge gates on.
func setupMergeFixture(t *testing.T, m planRepoMode) *mergeFixture {
	t.Helper()
	f := setupRebaseFixture(t, m)
	mf := &mergeFixture{rebaseFixture: f}
	mf.patchParent(t, "implemented", mergePRRef(), "")
	return mf
}

// mergeDeps assembles the FinalizeDeps a merge test drives: the real planning
// seams and workspace service, plus the recording fake GitHub.
func (f *mergeFixture) mergeDeps(gh FinalizeGitHub) FinalizeDeps {
	return FinalizeDeps{Planning: f.deps, GitHub: gh, Workspace: f.svc}
}

// parentPR is the canonical open PR for the parent feature head: number 7,
// non-draft, targeting main, carrying green evidence for the given head.
func (f *mergeFixture) parentPR(head string, body string) githubcli.PullRequest {
	return githubcli.PullRequest{
		Number: mergeCanonicalPRNumber, URL: "https://example.test/pr/7", State: githubcli.StateOpen,
		HeadBranch: "feat/" + f.slug, HeadCommit: head, BaseBranch: "main",
		Title: "Add the widget", Body: body, Version: "sha256:" + strings.Repeat("d", 64),
	}
}

// baselineFake returns a fake whose parent PR passes every conjunct: open,
// non-draft, number 7, at the fixture head, base main, green evidence.
func (f *mergeFixture) baselineFake(t *testing.T) *fakeMergeGitHub {
	t.Helper()
	return &fakeMergeGitHub{
		repo:       retargetRepo(),
		openByHead: map[string][]githubcli.PullRequest{"feat/" + f.slug: {f.parentPR(f.head, greenEvidenceFor(t, f.head))}},
	}
}

// mergeFeatureIntoBase creates a real merge commit on origin's integration
// branch (main) that carries the feature head, and returns its object id — the
// authoritative merge commit the reachability proof must find reachable.
func (f *mergeFixture) mergeFeatureIntoBase(t *testing.T) string {
	t.Helper()
	runGit(t, f.repo.writer, "fetch", "-q", "origin", "feat/"+f.slug)
	runGit(t, f.repo.writer, "checkout", "-q", "main")
	runGit(t, f.repo.writer, "merge", "-q", "--no-ff", "-m", "Merge feat/"+f.slug, "FETCH_HEAD")
	m := runGit(t, f.repo.writer, "rev-parse", "HEAD")
	runGit(t, f.repo.writer, "push", "-q", "origin", "main")
	return m
}

// mergedFactsFor builds the authoritative merged facts a merge/reprobe returns.
func mergedFactsFor(head, base, mergeCommit string) githubcli.MergedFacts {
	return githubcli.MergedFacts{
		HeadOID: head, BaseRef: base, MergedAtUTC: "2026-08-18T12:00:00Z",
		MergeCommit: mergeCommit, Version: "sha256:" + strings.Repeat("d", 64),
	}
}

func mergeReq(f *mergeFixture, head string, explicit, admin bool) FinalizeMergeRequest {
	return FinalizeMergeRequest{ID: f.id, Version: f.version, Head: head, Admin: admin, ExplicitID: explicit}
}

// --- TestMergeConjuncts (pure) --------------------------------------------

// --- TestFinalizeMergeConjunctsRechecked ----------------------------------

// TestProbeUnretargetedOpenChildrenBranchIdentity proves the open-child gate
// probes each child by ITS OWN recorded branch (never a slug-derived name): a
// non-derived recorded head is the branch queried, and a child whose record
// carries no branch is returned as an error the caller retains as unknown — so
// the parent merge is never issued.
func TestProbeUnretargetedOpenChildrenBranchIdentity(t *testing.T) {
	pin := docketPin(t)
	repo := retargetRepo()
	const parentBranch = "feat/root"

	build := func(t *testing.T, childBranchLine string) (domain.Snapshot, domain.Change) {
		t.Helper()
		root := finalizeBlob(80, "root", "implemented", "high", prRefFor(800), "")
		child := finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n")
		child.Data = []byte(strings.Replace(string(child.Data), "branch: feat/child-a\n", childBranchLine, 1))
		reader := &fakeReader{pin: pin, corpus: []StatusBlob{root, child}}
		inputs, _ := parseCorpus(reader.corpus)
		b, err := repository.BuildSnapshot(repository.BuildInput{Config: reader.pin.Config.Effective, Documents: inputs})
		if err != nil {
			t.Fatalf("BuildSnapshot: %v", err)
		}
		parent, out := b.Snapshot.Change(80)
		if out != domain.LookupFound {
			t.Fatalf("parent 80 not found in snapshot (%v)", out)
		}
		return b.Snapshot, parent
	}

	t.Run("non-derived-recorded-head-probed", func(t *testing.T) {
		snap, parent := build(t, "branch: feature/child-head\n")
		gh := &fakeMergeGitHub{repo: repo, openByHead: map[string][]githubcli.PullRequest{
			"feature/child-head": {{Number: 8, State: githubcli.StateOpen, HeadBranch: "feature/child-head", BaseBranch: parentBranch}},
		}}
		open, err := probeUnretargetedOpenChildren(context.Background(), FinalizeDeps{GitHub: gh}, repo, snap, parent, parentBranch)
		if err != nil {
			t.Fatalf("probe error: %v", err)
		}
		if len(open) != 1 || open[0] != 81 {
			t.Errorf("open children = %v, want [81] found by its recorded head feature/child-head", open)
		}
	})

	t.Run("missing-branch-is-unknown-error", func(t *testing.T) {
		snap, parent := build(t, "")
		gh := &fakeMergeGitHub{repo: repo, openByHead: map[string][]githubcli.PullRequest{}}
		if _, err := probeUnretargetedOpenChildren(context.Background(), FinalizeDeps{GitHub: gh}, repo, snap, parent, parentBranch); err == nil {
			t.Fatal("a child with no recorded branch must return an error (retained as unknown, no merge), got nil")
		}
	})
}

// assertMergeRefusal asserts a merge refusal carries the reason token, produced
// no VerifiedMerge, and issued zero merge calls.
func assertMergeRefusal(t *testing.T, res FinalizeMergeResult, gh *fakeMergeGitHub, token string) {
	t.Helper()
	if res.Reason != token {
		t.Fatalf("refusal reason = %q, want %q (result %q msg %q)", res.Reason, token, res.Result, res.Message)
	}
	if res.Result == ResultApplied || res.Result == ResultNoOp {
		t.Fatalf("a conjunct refusal reported a success result %q", res.Result)
	}
	if res.Merge != nil {
		t.Fatalf("a conjunct refusal carried a VerifiedMerge")
	}
	if gh.mergeCalls != 0 {
		t.Fatalf("a conjunct refusal issued %d merge call(s); want 0", gh.mergeCalls)
	}
}

// --- TestFinalizeMergeExplicitIDOverrides ---------------------------------

// --- TestFinalizeMergeAdminGate -------------------------------------------

// --- TestFinalizeMergeVerification ----------------------------------------

// --- TestFinalizeMergeAlreadyMergedNoop ------------------------------------

// --- TestFinalizeMergeMethodMapping ---------------------------------------
