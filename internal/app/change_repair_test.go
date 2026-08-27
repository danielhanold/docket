package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/workspace"
	"strings"
	"testing"
)

// This file drives `change repair-identity`: the version-pinned single-field
// identity repair the finalize checkpoint hands a human's decision to. The
// clause-by-clause refusals run over fakes (a scripted reader/GitHub seam with a
// recording engine that must never fire); the candidate-branch probe, the
// workspace gate, and the applied write run end-to-end over real bare-remote
// repositories with a fake GitHub seam scripting the viewed PR.

const repairVersion = "1234123412341234123412341234123412341234"

// --- fake FinalizeGitHub for repair ----------------------------------------

// fakeRepairGitHub scripts DiscoverRepository and the exact-number
// ViewPullRequest the repair reads; every other finalize-half GitHub method
// panics so an accidental call is loud.
type fakeRepairGitHub struct {
	repo    githubcli.Repository
	repoErr error
	pr      githubcli.PullRequest
	viewErr error

	viewCalls []int
}

func (f *fakeRepairGitHub) DiscoverRepository(_ context.Context, _ string) (githubcli.Repository, error) {
	return f.repo, f.repoErr
}
func (f *fakeRepairGitHub) ViewPullRequest(_ context.Context, _ githubcli.Repository, number int) (githubcli.PullRequest, error) {
	f.viewCalls = append(f.viewCalls, number)
	return f.pr, f.viewErr
}
func (f *fakeRepairGitHub) ProbeMerged(context.Context, githubcli.Repository, int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	panic("ProbeMerged: repair must not call this")
}
func (f *fakeRepairGitHub) FindOpenPullRequestsByHead(context.Context, githubcli.Repository, string) ([]githubcli.PullRequest, error) {
	panic("FindOpenPullRequestsByHead: repair must not call this")
}
func (f *fakeRepairGitHub) RetargetPullRequest(context.Context, githubcli.Repository, int, string, string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	panic("RetargetPullRequest: repair must not call this")
}
func (f *fakeRepairGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: repair must not call this")
}
func (f *fakeRepairGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: repair must not call this")
}
func (f *fakeRepairGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeResult, error) {
	panic("MergePullRequest: repair must not call this")
}

// --- fake FinalizeWorkspace for the ownership gate -------------------------

// fakeRepairWorkspace scripts the inspected workspace state (or a probe error)
// and records every Inspect call — the sentinel that proves the conflicting-
// workspace check actually executed. Every non-Inspect method panics: repair
// only inspects.
type fakeRepairWorkspace struct {
	inspection   workspace.Inspection
	inspectErr   error
	inspectCalls []workspace.InspectRequest
}

func (f *fakeRepairWorkspace) Inspect(_ context.Context, req workspace.InspectRequest) (workspace.Inspection, error) {
	f.inspectCalls = append(f.inspectCalls, req)
	return f.inspection, f.inspectErr
}
func (f *fakeRepairWorkspace) ReadRebaseReceipt(context.Context, string) (workspace.RebaseReceipt, bool, error) {
	panic("ReadRebaseReceipt: repair must not call this")
}
func (f *fakeRepairWorkspace) WriteRebaseReceipt(context.Context, string, workspace.RebaseReceipt) error {
	panic("WriteRebaseReceipt: repair must not call this")
}
func (f *fakeRepairWorkspace) ClearRebaseReceipt(context.Context, string) error {
	panic("ClearRebaseReceipt: repair must not call this")
}
func (f *fakeRepairWorkspace) PublishRewrite(context.Context, workspace.RewriteRequest) (workspace.RewriteOutcome, error) {
	panic("PublishRewrite: repair must not call this")
}
func (f *fakeRepairWorkspace) PublishHead(context.Context, workspace.PublishRequest) (workspace.PublishResult, error) {
	panic("PublishHead: repair must not call this")
}
func (f *fakeRepairWorkspace) Cleanup(context.Context, workspace.CleanupRequest) (workspace.CleanupResult, error) {
	panic("Cleanup: repair must not call this")
}

// --- fixtures --------------------------------------------------------------

// repairRecord renders an in-progress change record whose recorded branch is
// overridden to branch (empty leaves the canonical feat/<slug>).
func repairRecord(id int, slug, branch string) string {
	src := lifecycleChange(id, slug, "in-progress")
	if branch != "" {
		src = strings.Replace(src, "branch: feat/"+slug, "branch: "+branch, 1)
	}
	return src
}

// repairBlob wraps repairRecord as a corpus StatusBlob at version.
func repairBlob(id int, slug, branch, version string) StatusBlob {
	return StatusBlob{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(id, slug),
		Version:  version,
		Data:     []byte(repairRecord(id, slug, branch)),
	}
}

// repairFakeDeps wires a fake reader over a single change blob, a recording
// engine that must never fire on a refusal, and a scripted GitHub seam. The Git
// client is nil: every fake-driven refusal predates the first gitcli call.
func repairFakeDeps(t *testing.T, blob StatusBlob, gh *fakeRepairGitHub) (FinalizeDeps, *recordingEngine) {
	t.Helper()
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{blob}}
	engine := &recordingEngine{}
	deps := FinalizeDeps{
		Planning: PlanningDeps{Engine: engine, Reader: reader, Clock: testClock()},
		GitHub:   gh,
	}
	return deps, engine
}

// repairGitHub builds the scripted GitHub seam returning one PR by head branch.
func repairGitHub(headBranch string) *fakeRepairGitHub {
	return &fakeRepairGitHub{
		repo: githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widget"},
		pr:   githubcli.PullRequest{Number: 7, State: githubcli.StateOpen, HeadBranch: headBranch, HeadCommit: prHead, BaseBranch: "main"},
	}
}

func assertRepairRefused(t *testing.T, res RepairIdentityResult, wantResult Result, wantReason string, engine *recordingEngine) {
	t.Helper()
	if res.Result != wantResult {
		t.Fatalf("result = %q, want %q (reason %q, msg %q)", res.Result, wantResult, res.Reason, res.Message)
	}
	if res.Reason != wantReason {
		t.Errorf("reason = %q, want %q", res.Reason, wantReason)
	}
	if len(engine.calls) != 0 {
		t.Errorf("a refused repair opened %d transactions, want 0", len(engine.calls))
	}
}

// --- clause 1: stale version ------------------------------------------------

// TestRepairStaleVersionRefused proves clause 1: a change record whose current
// version no longer equals the approved ExpectVersion lost the race and is
// refused as stale-evidence, opening no transaction.
func TestRepairStaleVersionRefused(t *testing.T) {
	deps, engine := repairFakeDeps(t, repairBlob(3, "widget", "", "differentversion0000000000000000000000000"), repairGitHub("feat/widget"))
	res := RepairIdentity(context.Background(), deps, "/repo", RepairIdentityRequest{
		ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/widget",
	})
	assertRepairRefused(t, res, ResultContended, RepairStaleEvidence, engine)
}

// --- clause 2: adopt-pr-head evidence drift ---------------------------------

// TestRepairStaleHeadRefused proves clause 2: when the PR's reported head branch
// no longer matches the approved ExpectHead, the repair refuses as
// stale-evidence before any Git work.
func TestRepairStaleHeadRefused(t *testing.T) {
	deps, engine := repairFakeDeps(t, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/actual"))
	res := RepairIdentity(context.Background(), deps, "/repo", RepairIdentityRequest{
		ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/approved",
	})
	assertRepairRefused(t, res, ResultContended, RepairStaleEvidence, engine)
}

// TestRepairViewErrorIsUnknownNotApplied proves a PR view error is pr-unknown —
// an errored read is never laundered into a clean absence or a write.
func TestRepairViewErrorIsUnknownNotApplied(t *testing.T) {
	gh := repairGitHub("feat/widget")
	gh.viewErr = errors.New("gh pr view: network boom")
	deps, engine := repairFakeDeps(t, repairBlob(3, "widget", "", repairVersion), gh)
	res := RepairIdentity(context.Background(), deps, "/repo", RepairIdentityRequest{
		ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/widget",
	})
	assertRepairRefused(t, res, ResultExternalFailed, RepairPRUnknown, engine)
	if len(gh.viewCalls) != 1 || gh.viewCalls[0] != 7 {
		t.Errorf("view calls = %v, want exactly the recorded number 7", gh.viewCalls)
	}
}

// --- clause 3: adopt-pr proof-of-identity -----------------------------------

// TestRepairAdoptPRRequiresHeadEqualsRecorded proves clause 3: the supplied PR
// only proves identity when its head equals the recorded branch. A parse
// failure is invalid-request; a recorded branch that drifted from the approved
// one, or a PR head that does not equal the recorded branch, is stale-evidence.
func TestRepairAdoptPRRequiresHeadEqualsRecorded(t *testing.T) {
	t.Run("unparseable-ref-is-invalid-request", func(t *testing.T) {
		deps, engine := repairFakeDeps(t, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/widget"))
		res := RepairIdentity(context.Background(), deps, "/repo", RepairIdentityRequest{
			ID: 3, ExpectVersion: repairVersion, AdoptPR: "not-a-pr-reference", ExpectBranch: "feat/widget",
		})
		assertRepairRefused(t, res, ResultInvalidInput, RepairInvalidRequest, engine)
	})

	t.Run("recorded-branch-drifted", func(t *testing.T) {
		// The record carries feat/widget, but the human approved feat/other.
		deps, engine := repairFakeDeps(t, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/widget"))
		res := RepairIdentity(context.Background(), deps, "/repo", RepairIdentityRequest{
			ID: 3, ExpectVersion: repairVersion, AdoptPR: "https://github.com/acme/widget/pull/7", ExpectBranch: "feat/other",
		})
		assertRepairRefused(t, res, ResultContended, RepairStaleEvidence, engine)
	})

	t.Run("pr-head-not-recorded-branch", func(t *testing.T) {
		// Recorded/approved branch feat/widget, but the PR's head is feat/elsewhere:
		// the supplied PR does not prove identity.
		deps, engine := repairFakeDeps(t, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/elsewhere"))
		res := RepairIdentity(context.Background(), deps, "/repo", RepairIdentityRequest{
			ID: 3, ExpectVersion: repairVersion, AdoptPR: "https://github.com/acme/widget/pull/7", ExpectBranch: "feat/widget",
		})
		assertRepairRefused(t, res, ResultContended, RepairStaleEvidence, engine)
	})
}

// --- request shape ----------------------------------------------------------

// TestRepairInvalidRequestShape proves the request-shape gate: not exactly one
// mode, or a mode missing its evidence, is invalid-request with no work done.
func TestRepairInvalidRequestShape(t *testing.T) {
	deps, engine := repairFakeDeps(t, repairBlob(3, "widget", "", repairVersion), repairGitHub("feat/widget"))
	for _, tc := range []struct {
		name string
		req  RepairIdentityRequest
	}{
		{"neither-mode", RepairIdentityRequest{ID: 3, ExpectVersion: repairVersion}},
		{"both-modes", RepairIdentityRequest{ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/widget", AdoptPR: "x#1", ExpectBranch: "feat/widget"}},
		{"head-mode-missing-head", RepairIdentityRequest{ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectPRNumber: 7}},
		{"head-mode-missing-number", RepairIdentityRequest{ID: 3, ExpectVersion: repairVersion, AdoptPRHead: true, ExpectHead: "feat/widget"}},
		{"pr-mode-missing-branch", RepairIdentityRequest{ID: 3, ExpectVersion: repairVersion, AdoptPR: "x#1"}},
		{"empty-version", RepairIdentityRequest{ID: 3, AdoptPRHead: true, ExpectPRNumber: 7, ExpectHead: "feat/widget"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := RepairIdentity(context.Background(), deps, "/repo", tc.req)
			assertRepairRefused(t, res, ResultInvalidInput, RepairInvalidRequest, engine)
		})
	}
}

// --- clause 5: writes exactly the approved field ----------------------------

// repairPlanFor runs the repair op's Plan closure over a fake tree so a test can
// inspect the patched record bytes directly (mirrors implementedPlanFor).
func repairPlanFor(t *testing.T, files map[string]string, op changeRepairOp) transaction.MutationPlan {
	t.Helper()
	tree := newFakeTree(files)
	loader := newPlanningLoader(op.eff)
	before, err := loader.Load(context.Background(), tree)
	if err != nil {
		t.Fatalf("loader.Load: %v", err)
	}
	if before.Report.HasErrors() {
		t.Fatalf("before-state has errors: %v", before.Report.Findings())
	}
	plan, opRes, err := op.Plan(context.Background(), transaction.AttemptState{Base: tree.Revision(), State: before, Tree: tree})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	return plan
}

func repairOp(field, value string) changeRepairOp {
	return changeRepairOp{
		changeID:   3,
		field:      field,
		value:      value,
		eff:        planningTestConfig(nil),
		clock:      testClock(),
		link:       render.LinkContext{MetadataBranch: "main"},
		changesDir: "docs/changes",
	}
}

// frontmatterLines returns the lines of a record's first --- … --- block.
func frontmatterLines(t *testing.T, record string) []string {
	t.Helper()
	parts := strings.SplitN(record, "---\n", 3)
	if len(parts) < 3 {
		t.Fatalf("record has no frontmatter block:\n%s", record)
	}
	return strings.Split(strings.TrimRight(parts[1], "\n"), "\n")
}

// changedFrontmatterLines returns the frontmatter lines present in after but not
// before (the additions/replacements the write introduced).
func changedFrontmatterLines(t *testing.T, before, after string) []string {
	t.Helper()
	beforeSet := map[string]bool{}
	for _, l := range frontmatterLines(t, before) {
		beforeSet[l] = true
	}
	var changed []string
	for _, l := range frontmatterLines(t, after) {
		if !beforeSet[l] {
			changed = append(changed, l)
		}
	}
	return changed
}

// TestRepairWritesOnlyTheApprovedField proves clause 5: the write touches
// exactly one identity field plus the updated stamp, in either mode — diffed
// frontmatter line-for-line against the before-state.
func TestRepairWritesOnlyTheApprovedField(t *testing.T) {
	recPath := groomPath(3, "widget")
	before := repairRecord(3, "widget", "")

	t.Run("branch", func(t *testing.T) {
		plan := repairPlanFor(t, map[string]string{recPath: before}, repairOp("branch", "feat/renamed"))
		after := lifecycleRecordBytes(t, plan, recPath)
		changed := changedFrontmatterLines(t, before, after)
		wantChanged := map[string]bool{"branch: 'feat/renamed'": true, "updated: '2026-08-16'": true}
		for _, l := range changed {
			if !wantChanged[l] {
				t.Errorf("unexpected frontmatter change %q; only branch + updated may change\nafter:\n%s", l, after)
			}
		}
		if len(changed) != 2 {
			t.Errorf("changed %d frontmatter lines %v, want exactly branch + updated", len(changed), changed)
		}
	})

	t.Run("pr", func(t *testing.T) {
		plan := repairPlanFor(t, map[string]string{recPath: before}, repairOp("pr", "https://github.com/acme/widget/pull/7"))
		after := lifecycleRecordBytes(t, plan, recPath)
		changed := changedFrontmatterLines(t, before, after)
		wantChanged := map[string]bool{"pr: 'https://github.com/acme/widget/pull/7'": true, "updated: '2026-08-16'": true}
		for _, l := range changed {
			if !wantChanged[l] {
				t.Errorf("unexpected frontmatter change %q; only pr + updated may change\nafter:\n%s", l, after)
			}
		}
		if len(changed) != 2 {
			t.Errorf("changed %d frontmatter lines %v, want exactly pr + updated", len(changed), changed)
		}
	})
}

// --- real-git gates ---------------------------------------------------------

// repairRealDeps wires the real planning client over dir with a fake reader
// (scripting the corpus/version) and recording engine, plus the scripted GitHub
// and workspace seams — so the candidate-branch probe and workspace inspect hit
// real Git while the transaction is observed, never fired.
func repairRealDeps(t *testing.T, dir string, blob StatusBlob, gh *fakeRepairGitHub, ws FinalizeWorkspace) (FinalizeDeps, *recordingEngine) {
	t.Helper()
	client := newGitClient(t)
	engine := &recordingEngine{}
	deps := FinalizeDeps{
		Planning:  PlanningDeps{Client: client, Engine: engine, Reader: &fakeReader{pin: mainPin(t), corpus: []StatusBlob{blob}}, Clock: testClock()},
		GitHub:    gh,
		Workspace: ws,
	}
	return deps, engine
}
