package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/githubcli"
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
	mergeOutcome   githubcli.MergeOutcome
	mergeFacts     githubcli.MergedFacts
	mergeErr       error
	mergeCalls     int
	lastMergeAdmin bool
	lastMergeHead  githubcli.ObjectRef
	lastMergeNum   int
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

func (f *fakeMergeGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, head string) ([]githubcli.PullRequest, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.openByHead[head], nil
}

func (f *fakeMergeGitHub) MergePullRequest(_ context.Context, _ githubcli.Repository, number int, expectedHead githubcli.ObjectRef, admin bool) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	f.mergeCalls++
	f.lastMergeAdmin = admin
	f.lastMergeHead = expectedHead
	f.lastMergeNum = number
	if f.mergeErr != nil {
		return githubcli.MergeUnknown, githubcli.MergedFacts{}, f.mergeErr
	}
	return f.mergeOutcome, f.mergeFacts, nil
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

// TestFinalizeMergeConjunctAssembly proves the pure conjunct assembly maps each
// falsified input to exactly its closed token and holds only when every input is
// satisfied. This is the exhaustive per-field oracle; the operation-level test
// proves the recheck-before-effect wiring.
func TestFinalizeMergeConjunctAssembly(t *testing.T) {
	const head = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	good := mergeConjunctInputs{
		status:            domain.StatusImplemented,
		canonicalPRNumber: 7, prNumber: 7,
		reqHead: head, prHead: head, remoteHead: head, localHead: head,
		prState: githubcli.StateOpen, prDraft: false,
		prBase: "main", effectiveBase: "main",
		gateOff: false, evidenceGreen: true, evidenceHead: head,
		explicitID: true, requireApproval: false,
		unretargetedOpenChildren: 0,
		versionMatches:           true, finalizeBlocked: false,
	}
	if got := mergeConjuncts(good).FirstFailure(); got != "" {
		t.Fatalf("a fully-satisfied input failed conjunct %q", got)
	}

	cases := []struct {
		name  string
		mut   func(in *mergeConjunctInputs)
		token string
	}{
		{"not-implemented", func(in *mergeConjunctInputs) { in.status = domain.StatusInProgress }, "not-implemented"},
		{"pr-identity", func(in *mergeConjunctInputs) { in.prNumber = 9 }, "pr-identity-mismatch"},
		{"head-pr", func(in *mergeConjunctInputs) { in.prHead = "deadbeef" }, "head-moved"},
		{"head-remote", func(in *mergeConjunctInputs) { in.remoteHead = "deadbeef" }, "head-moved"},
		{"head-local", func(in *mergeConjunctInputs) { in.localHead = "deadbeef" }, "head-moved"},
		{"closed", func(in *mergeConjunctInputs) { in.prState = githubcli.StateClosed }, "not-open-nondraft"},
		{"draft", func(in *mergeConjunctInputs) { in.prDraft = true }, "not-open-nondraft"},
		{"base", func(in *mergeConjunctInputs) { in.prBase = "develop" }, "base-mismatch"},
		{"gate-no-evidence", func(in *mergeConjunctInputs) { in.evidenceGreen = false }, "gate-unsatisfied"},
		{"gate-stale-evidence", func(in *mergeConjunctInputs) { in.evidenceHead = "other" }, "gate-unsatisfied"},
		{"approval", func(in *mergeConjunctInputs) { in.explicitID = false; in.requireApproval = true }, "approval-required"},
		{"open-children", func(in *mergeConjunctInputs) { in.unretargetedOpenChildren = 1 }, "open-children"},
		{"superseded-version", func(in *mergeConjunctInputs) { in.versionMatches = false }, "superseded"},
		{"superseded-blocked", func(in *mergeConjunctInputs) { in.explicitID = false; in.finalizeBlocked = true }, "superseded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := good
			tc.mut(&in)
			if got := mergeConjuncts(in).FirstFailure(); got != tc.token {
				t.Fatalf("FirstFailure = %q, want %q", got, tc.token)
			}
		})
	}

	// The overridable conjuncts: an explicit id satisfies approval and a
	// finalize-blocked marker, but never a superseding version.
	t.Run("explicit-id-overrides-approval", func(t *testing.T) {
		in := good
		in.explicitID = true
		in.requireApproval = true
		if got := mergeConjuncts(in).FirstFailure(); got != "" {
			t.Fatalf("explicit id did not satisfy approval: %q", got)
		}
	})
	t.Run("explicit-id-overrides-blocked", func(t *testing.T) {
		in := good
		in.explicitID = true
		in.finalizeBlocked = true
		if got := mergeConjuncts(in).FirstFailure(); got != "" {
			t.Fatalf("explicit id did not satisfy the finalize-blocked marker: %q", got)
		}
	})
	t.Run("explicit-id-never-overrides-version", func(t *testing.T) {
		in := good
		in.explicitID = true
		in.versionMatches = false
		if got := mergeConjuncts(in).FirstFailure(); got != "superseded" {
			t.Fatalf("explicit id wrongly overrode a superseding version: %q", got)
		}
	})
}

// --- TestFinalizeMergeConjunctsRechecked ----------------------------------

// TestFinalizeMergeConjunctsRechecked proves the operation rechecks each merge
// conjunct from a FRESH reload immediately before the effect: a falsified field
// refuses with that field's closed token and issues zero merge calls.
func TestFinalizeMergeConjunctsRechecked(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	// Cases achievable by perturbing the live fake/request over a shared baseline
	// fixture (no metadata rewrite needed).
	t.Run("pr-identity-mismatch", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug] = []githubcli.PullRequest{func() githubcli.PullRequest {
			pr := f.parentPR(f.head, greenEvidenceFor(t, f.head))
			pr.Number = 9 // not the canonical #7
			return pr
		}()}
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "pr-identity-mismatch")
	})

	t.Run("head-moved", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		other := strings.Repeat("b", 40)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, other, true, false))
		assertMergeRefusal(t, res, gh, "head-moved")
	})

	t.Run("draft", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].Draft = true
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "not-open-nondraft")
	})

	t.Run("base-mismatch", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].BaseBranch = "develop"
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "base-mismatch")
	})

	t.Run("gate-unsatisfied", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].Body = "" // no green evidence; gate is local
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "gate-unsatisfied")
	})

	// Metadata-shaped cases: a stale version, a durable finalize-blocked marker,
	// and a not-implemented status.
	t.Run("superseded-version", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		req := mergeReq(f, f.head, true, false)
		req.Version = "sha256:" + strings.Repeat("f", 64) // stale; explicit id never overrides a version
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, req)
		assertMergeRefusal(t, res, gh, "superseded")
	})

	t.Run("superseded-finalize-blocked", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.patchParent(t, "implemented", mergePRRef(), "## Finalize blocked\n\nBlocked pending a decision.")
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, false, false))
		assertMergeRefusal(t, res, gh, "superseded")
	})

	t.Run("not-implemented", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.patchParent(t, "proposed", mergePRRef(), "")
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "not-implemented")
	})

	t.Run("unretargeted-open-child", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.repo.writerAdvance(t, f.branch, map[string]string{groomPath(6, "gadget"): childRecord(6, "gadget", f.id, "github.com/acme/widget#8")})
		gh := f.baselineFake(t)
		// The child's open PR still targets the parent's feature branch: unretargeted.
		gh.openByHead["feat/gadget"] = []githubcli.PullRequest{{
			Number: 8, State: githubcli.StateOpen, HeadBranch: "feat/gadget",
			HeadCommit: strings.Repeat("c", 40), BaseBranch: "feat/" + f.slug,
			Version: "sha256:" + strings.Repeat("e", 64),
		}}
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "open-children")
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

// TestFinalizeMergeExplicitIDOverrides proves an explicit id satisfies the
// finalize-blocked skip but never overrides wrong PR identity, an unsafe stack,
// or the repair sign-off (gate), and never a superseding version.
func TestFinalizeMergeExplicitIDOverrides(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("explicit-id-merges-past-finalize-blocked", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.patchParent(t, "implemented", mergePRRef(), "## Finalize blocked\n\nBlocked pending a decision.")
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultApplied || res.Merge == nil {
			t.Fatalf("explicit id did not merge past the finalize-blocked marker: %q (reason %q)", res.Result, res.Reason)
		}
		if gh.mergeCalls != 1 {
			t.Fatalf("merge calls = %d, want 1", gh.mergeCalls)
		}
	})

	// Without an explicit id, the same finalize-blocked marker refuses.
	t.Run("auto-refuses-finalize-blocked", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		f.patchParent(t, "implemented", mergePRRef(), "## Finalize blocked\n\nBlocked pending a decision.")
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, false, false))
		assertMergeRefusal(t, res, gh, "superseded")
	})

	t.Run("explicit-id-does-not-override-pr-identity", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].Number = 9
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "pr-identity-mismatch")
	})

	t.Run("explicit-id-does-not-override-gate", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.openByHead["feat/"+f.slug][0].Body = ""
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		assertMergeRefusal(t, res, gh, "gate-unsatisfied")
	})

	t.Run("explicit-id-does-not-override-superseding-version", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		req := mergeReq(f, f.head, true, false)
		req.Version = "sha256:" + strings.Repeat("f", 64)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, req)
		assertMergeRefusal(t, res, gh, "superseded")
	})
}

// --- TestFinalizeMergeAdminGate -------------------------------------------

// TestFinalizeMergeAdminGate proves --admin is honored only on an attended,
// explicitly-named run, is never inferred, and that a denial stays denied
// (never retried with admin).
func TestFinalizeMergeAdminGate(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("admin-honored-with-explicit-id", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, true))
		if res.Result != ResultApplied || res.Merge == nil {
			t.Fatalf("admin merge with explicit id = %q (reason %q), want applied", res.Result, res.Reason)
		}
		if !gh.lastMergeAdmin {
			t.Fatalf("admin flag was not passed through to the merge effect")
		}
	})

	t.Run("admin-refused-without-explicit-id", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, false, true))
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("admin without explicit id reported success %q", res.Result)
		}
		if res.Reason != ReasonMergeAdminNotAuthorized {
			t.Fatalf("reason = %q, want %q", res.Reason, ReasonMergeAdminNotAuthorized)
		}
		if gh.mergeCalls != 0 {
			t.Fatalf("admin-without-explicit-id issued %d merge call(s); want 0", gh.mergeCalls)
		}
	})

	t.Run("denied-stays-denied-no-admin-retry", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeDenied
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Disposition != MergeDispDenied {
			t.Fatalf("disposition = %q, want %q (result %q)", res.Disposition, MergeDispDenied, res.Result)
		}
		if res.Merge != nil {
			t.Fatalf("a denial carried a VerifiedMerge")
		}
		if gh.mergeCalls != 1 {
			t.Fatalf("a denial issued %d merge call(s); want exactly 1 (no admin retry)", gh.mergeCalls)
		}
		if gh.lastMergeAdmin {
			t.Fatalf("a denial was retried with admin; admin must never be inferred")
		}
	})
}

// --- TestFinalizeMergeVerification ----------------------------------------

// TestFinalizeMergeVerification proves the authoritative post-merge verification:
// a reachable merge commit yields a verified merge; a divergent head/base is
// contended; an unobservable reprobe is unknown; none but the reachable proof
// permits closeout.
func TestFinalizeMergeVerification(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]

	t.Run("reachable-merge-commit-verifies", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		mergeCommit := f.mergeFeatureIntoBase(t)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultApplied || res.Disposition != MergeDispMerged || res.Merge == nil {
			t.Fatalf("verified merge = %q disp %q merge %v (reason %q)", res.Result, res.Disposition, res.Merge, res.Reason)
		}
		vm := res.Merge
		if vm.PRNumber != mergeCanonicalPRNumber || vm.HeadOID != f.head || vm.BaseRef != "main" || vm.MergeCommit != mergeCommit {
			t.Fatalf("VerifiedMerge facts = %+v, want number 7 head %s base main mergeCommit %s", vm, f.head, mergeCommit)
		}
		if vm.MergedAtUTC == "" {
			t.Fatalf("VerifiedMerge carried no mergedAt")
		}
	})

	t.Run("head-divergence-is-contended", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(strings.Repeat("b", 40), "main", strings.Repeat("a", 40))
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultContended || res.Merge != nil {
			t.Fatalf("head divergence = %q merge %v, want contended and no VerifiedMerge", res.Result, res.Merge)
		}
	})

	t.Run("base-divergence-is-contended", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "develop", strings.Repeat("a", 40))
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultContended || res.Merge != nil {
			t.Fatalf("base divergence = %q merge %v, want contended and no VerifiedMerge", res.Result, res.Merge)
		}
	})

	t.Run("unobservable-reprobe-is-unknown", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeUnknown
		gh.mergeErr = errors.New("gh merge transport boom")
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Disposition != MergeDispUnknown || res.Merge != nil {
			t.Fatalf("unobservable merge = disp %q merge %v (result %q), want unknown and no VerifiedMerge", res.Disposition, res.Merge, res.Result)
		}
	})

	t.Run("present-but-unreachable-merge-commit-is-contended", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		// The feature head is a real object in the shared store but is NOT merged
		// into the destination, so it is present yet not reachable from the base
		// tip — a clean unreachable answer, distinct from an absent object.
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		gh.mergeFacts = mergedFactsFor(f.head, "main", f.head)
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Result != ResultContended || res.Merge != nil {
			t.Fatalf("unreachable merge commit = %q merge %v (reason %q), want contended and no VerifiedMerge", res.Result, res.Merge, res.Reason)
		}
		if res.Reason != ReasonMergeUnreachable {
			t.Fatalf("reason = %q, want %q", res.Reason, ReasonMergeUnreachable)
		}
	})

	t.Run("reported-merge-commit-absent-is-unknown", func(t *testing.T) {
		f := setupMergeFixture(t, m)
		gh := f.baselineFake(t)
		gh.mergeOutcome = githubcli.MergeMerged
		// A well-formed object id that is not in the destination's object graph:
		// the reachability probe cannot observe it, so the result is unknown.
		gh.mergeFacts = mergedFactsFor(f.head, "main", strings.Repeat("a", 40))
		res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
		if res.Merge != nil {
			t.Fatalf("an unverifiable merge commit produced a VerifiedMerge")
		}
		if res.Result == ResultApplied || res.Result == ResultNoOp {
			t.Fatalf("an unverifiable merge commit reported success %q", res.Result)
		}
	})
}

// --- TestFinalizeMergeAlreadyMergedNoop ------------------------------------

// TestFinalizeMergeAlreadyMergedNoop proves an already-merged exact PR is a
// verified no-op regardless of who merged it, issuing no second merge.
func TestFinalizeMergeAlreadyMergedNoop(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupMergeFixture(t, m)
	mergeCommit := f.mergeFeatureIntoBase(t)
	gh := f.baselineFake(t)
	gh.probeOutcome = githubcli.MergeAlreadyMerged
	gh.probeFacts = mergedFactsFor(f.head, "main", mergeCommit)

	res := FinalizeMerge(context.Background(), f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
	if res.Result != ResultNoOp || res.Disposition != MergeDispAlreadyMerged || res.Merge == nil {
		t.Fatalf("already-merged = %q disp %q merge %v (reason %q), want no-op/already-merged verified", res.Result, res.Disposition, res.Merge, res.Reason)
	}
	if res.Merge.MergeCommit != mergeCommit {
		t.Fatalf("VerifiedMerge merge commit = %q, want %q", res.Merge.MergeCommit, mergeCommit)
	}
	if gh.mergeCalls != 0 {
		t.Fatalf("an already-merged PR issued %d merge call(s); want 0 (never a second merge)", gh.mergeCalls)
	}
}
