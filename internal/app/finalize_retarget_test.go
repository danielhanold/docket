package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/githubcli"
)

// --- fake GitHub seam -----------------------------------------------------

// fakePR is one pull request the retarget fake tracks. Its base and version
// mutate exactly as githubcli.RetargetPullRequest promises, so the fake is a
// behavioral stand-in for that adapter rather than a fixed script.
type fakePR struct {
	number  int
	head    string
	base    string
	version string
}

// retargetCall records one RetargetPullRequest invocation for assertions.
type retargetCall struct {
	number          int
	expectedVersion string
	newBase         string
	edited          bool
}

// fakeRetargetGitHub is a recording, behavioral FinalizeGitHub over an in-memory
// PR registry. FindOpenPullRequestsByHead and RetargetPullRequest share the same
// registry so an edit is visible to a later probe (the retry path). The GitHub
// methods the operation must never call panic, so an accidental call is loud.
type fakeRetargetGitHub struct {
	repo    githubcli.Repository
	repoErr error
	prs     []*fakePR
	findErr map[string]error // head branch -> forced probe error
	retErr  map[int]error    // pr number -> forced unknown error

	finds     []string
	retargets []retargetCall
}

func (f *fakeRetargetGitHub) DiscoverRepository(_ context.Context, _ string) (githubcli.Repository, error) {
	if f.repoErr != nil {
		return githubcli.Repository{}, f.repoErr
	}
	return f.repo, nil
}

func (f *fakeRetargetGitHub) FindOpenPullRequestsByHead(_ context.Context, _ githubcli.Repository, headBranch string) ([]githubcli.PullRequest, error) {
	f.finds = append(f.finds, headBranch)
	if e := f.findErr[headBranch]; e != nil {
		return nil, e
	}
	var out []githubcli.PullRequest
	for _, pr := range f.prs {
		if pr.head == headBranch {
			out = append(out, githubcli.PullRequest{
				Number: pr.number, State: githubcli.StateOpen,
				HeadBranch: pr.head, BaseBranch: pr.base, Version: pr.version,
			})
		}
	}
	return out, nil
}

func (f *fakeRetargetGitHub) RetargetPullRequest(_ context.Context, _ githubcli.Repository, number int, expectedVersion, newBase string) (githubcli.RetargetOutcome, githubcli.PullRequest, error) {
	call := retargetCall{number: number, expectedVersion: expectedVersion, newBase: newBase}
	if e := f.retErr[number]; e != nil {
		f.retargets = append(f.retargets, call)
		return githubcli.RetargetUnknown, githubcli.PullRequest{}, e
	}
	var pr *fakePR
	for _, p := range f.prs {
		if p.number == number {
			pr = p
			break
		}
	}
	if pr == nil {
		f.retargets = append(f.retargets, call)
		return githubcli.RetargetUnknown, githubcli.PullRequest{}, fmt.Errorf("pr %d not found", number)
	}
	// Promised end-state is the idempotency key, checked before the version gate.
	if pr.base == newBase {
		f.retargets = append(f.retargets, call)
		return githubcli.RetargetAlready, snapshotPR(pr), nil
	}
	if expectedVersion == "" || expectedVersion != pr.version {
		f.retargets = append(f.retargets, call)
		return githubcli.RetargetContended, githubcli.PullRequest{}, nil
	}
	pr.base = newBase
	pr.version = pr.version + "+r"
	call.edited = true
	f.retargets = append(f.retargets, call)
	return githubcli.RetargetRetargeted, snapshotPR(pr), nil
}

func snapshotPR(pr *fakePR) githubcli.PullRequest {
	return githubcli.PullRequest{
		Number: pr.number, State: githubcli.StateOpen,
		HeadBranch: pr.head, BaseBranch: pr.base, Version: pr.version,
	}
}

// The finalize-half GitHub methods the retarget operation must never touch.
func (f *fakeRetargetGitHub) ProbeMerged(context.Context, githubcli.Repository, int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	panic("ProbeMerged: retarget must not call this")
}
func (f *fakeRetargetGitHub) EnsureComment(context.Context, githubcli.Repository, int, string, string) (githubcli.CommentOutcome, string, error) {
	panic("EnsureComment: retarget must not call this")
}
func (f *fakeRetargetGitHub) FindComment(context.Context, githubcli.Repository, int, string) (bool, string, error) {
	panic("FindComment: retarget must not call this")
}
func (f *fakeRetargetGitHub) MergePullRequest(context.Context, githubcli.Repository, int, githubcli.ObjectRef, bool) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	panic("MergePullRequest: retarget must not call this")
}

// retargetDeps wires the read-only reader, the recording GitHub fake, and a
// recording engine that must never fire (retarget opens no transaction).
func retargetDeps(fake *fakeReader, gh FinalizeGitHub, engine *recordingEngine) FinalizeDeps {
	return FinalizeDeps{
		Planning: PlanningDeps{Reader: fake, Engine: engine, Clock: testClock()},
		GitHub:   gh,
	}
}

func retargetRepo() githubcli.Repository {
	return githubcli.Repository{Host: "github.com", Owner: "acme", Name: "widgets"}
}

// childOutcomeByID finds one child's outcome in the result, or fails.
func childOutcomeByID(t *testing.T, r RetargetChildrenResult, id int) ChildRetargetOutcome {
	t.Helper()
	for _, c := range r.Children {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("child %d not present in result children %+v", id, r.Children)
	return ChildRetargetOutcome{}
}

// --- tests ----------------------------------------------------------------

// TestRetargetChildrenHappy: each authorized open child is probe/act/verified onto
// the parent's effective base (the integration branch for a root parent), and a
// retry adopts the already-retargeted exact PR as an idempotent no-op.
func TestRetargetChildrenHappy(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
		finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
		finalizeBlob(82, "child-b", "implemented", "high", prRefFor(820), "stacked_on: 80\n"),
	}
	gh := &fakeRetargetGitHub{
		repo: retargetRepo(),
		prs: []*fakePR{
			{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810"},
			{number: 820, head: "feat/child-b", base: "feat/root", version: "cv820"},
		},
	}
	engine := &recordingEngine{}
	fake := &fakeReader{pin: pin, corpus: corpus}
	req := RetargetChildrenRequest{
		ID: 80, Version: "blobfin0080",
		Children: []AuthorizedChild{
			{ID: 81, PRNumber: 810, PRVersion: "cv810"},
			{ID: 82, PRNumber: 820, PRVersion: "cv820"},
		},
	}

	got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q message=%q", got.Result, got.Reason, got.Message)
	}
	if got.Disposition != RetargetDispositionRetargeted {
		t.Fatalf("disposition=%q, want retargeted", got.Disposition)
	}
	if got.Base != "main" {
		t.Fatalf("base=%q, want the integration branch main", got.Base)
	}
	for _, id := range []int{81, 82} {
		c := childOutcomeByID(t, got, id)
		if c.Outcome != childOutcomeRetargeted || c.Base != "main" {
			t.Errorf("child %d outcome=%q base=%q, want retargeted onto main", id, c.Outcome, c.Base)
		}
	}
	// The retarget executed no transaction: stacked_on is untouched by construction.
	if len(engine.calls) != 0 {
		t.Fatalf("retarget opened %d transactions, want 0", len(engine.calls))
	}
	// Both child PRs now target the effective base.
	for _, pr := range gh.prs {
		if pr.base != "main" {
			t.Errorf("PR %d base=%q, want main", pr.number, pr.base)
		}
	}

	// Retry: the same authorized set adopts the already-retargeted PRs as a no-op,
	// issuing no further edit.
	editsBefore := countEdits(gh.retargets)
	fake2 := &fakeReader{pin: pin, corpus: corpus}
	retry := FinalizeRetargetChildren(context.Background(), retargetDeps(fake2, gh, engine), "", req)
	if retry.Result != ResultNoOp {
		t.Fatalf("retry result=%q, want no-op (idempotent adopt)", retry.Result)
	}
	if retry.Disposition != RetargetDispositionRetargeted {
		t.Errorf("retry disposition=%q, want retargeted", retry.Disposition)
	}
	for _, id := range []int{81, 82} {
		if c := childOutcomeByID(t, retry, id); c.Outcome != childOutcomeAlready {
			t.Errorf("retry child %d outcome=%q, want already", id, c.Outcome)
		}
	}
	if countEdits(gh.retargets) != editsBefore {
		t.Errorf("retry issued a new edit; edits before=%d after=%d", editsBefore, countEdits(gh.retargets))
	}
}

func countEdits(calls []retargetCall) int {
	n := 0
	for _, c := range calls {
		if c.edited {
			n++
		}
	}
	return n
}

// TestRetargetChildrenNewChildBlocks: a child open in the live graph but absent
// from the authorized set is contended, and NO edit is issued.
func TestRetargetChildrenNewChildBlocks(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
		finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
		finalizeBlob(83, "child-c", "implemented", "high", prRefFor(830), "stacked_on: 80\n"), // concurrently added
	}
	gh := &fakeRetargetGitHub{
		repo: retargetRepo(),
		prs: []*fakePR{
			{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810"},
			{number: 830, head: "feat/child-c", base: "feat/root", version: "cv830"},
		},
	}
	engine := &recordingEngine{}
	fake := &fakeReader{pin: pin, corpus: corpus}
	req := RetargetChildrenRequest{
		ID: 80, Version: "blobfin0080",
		Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}}, // 83 not authorized
	}

	got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
	if got.Result != ResultContended || got.Disposition != RetargetDispositionContended {
		t.Fatalf("result=%q disposition=%q, want contended", got.Result, got.Disposition)
	}
	if got.Reason != ReasonRetargetNewChild {
		t.Errorf("reason=%q, want %q", got.Reason, ReasonRetargetNewChild)
	}
	if countEdits(gh.retargets) != 0 {
		t.Fatalf("a blocked authorization edited a PR; edits=%d", countEdits(gh.retargets))
	}
	if len(gh.retargets) != 0 {
		t.Errorf("RetargetPullRequest was called %d time(s) on a blocked authorization; want 0", len(gh.retargets))
	}
	if len(engine.calls) != 0 {
		t.Errorf("opened %d transactions, want 0", len(engine.calls))
	}
}

// TestRetargetChildrenVersionDrift: a changed PR version is contended; an ambiguous
// child head (two open PRs) is contended; a probe error is unknown. In every case
// there is no parent-merge-enabling success and no improper edit.
func TestRetargetChildrenVersionDrift(t *testing.T) {
	pin := docketPin(t)
	base := []StatusBlob{
		finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
		finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
	}

	t.Run("changed-pr-version", func(t *testing.T) {
		gh := &fakeRetargetGitHub{repo: retargetRepo(), prs: []*fakePR{
			{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810-NEW"},
		}}
		engine := &recordingEngine{}
		fake := &fakeReader{pin: pin, corpus: base}
		req := RetargetChildrenRequest{ID: 80, Version: "blobfin0080",
			Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810-STALE"}}}
		got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
		if got.Result == ResultApplied || got.Result == ResultNoOp {
			t.Fatalf("stale PR version produced a merge-enabling success: %q", got.Result)
		}
		if got.Disposition != RetargetDispositionContended {
			t.Fatalf("disposition=%q, want contended", got.Disposition)
		}
		if countEdits(gh.retargets) != 0 {
			t.Errorf("stale PR version was edited; edits=%d", countEdits(gh.retargets))
		}
	})

	t.Run("ambiguous-child-pr", func(t *testing.T) {
		gh := &fakeRetargetGitHub{repo: retargetRepo(), prs: []*fakePR{
			{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810"},
			{number: 811, head: "feat/child-a", base: "feat/root", version: "cv811"}, // second open PR, same head
		}}
		engine := &recordingEngine{}
		fake := &fakeReader{pin: pin, corpus: base}
		req := RetargetChildrenRequest{ID: 80, Version: "blobfin0080",
			Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}}}
		got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
		if got.Result != ResultContended || got.Disposition != RetargetDispositionContended {
			t.Fatalf("result=%q disposition=%q, want contended", got.Result, got.Disposition)
		}
		if got.Reason != ReasonRetargetAmbiguousChildPR {
			t.Errorf("reason=%q, want %q", got.Reason, ReasonRetargetAmbiguousChildPR)
		}
		if len(gh.retargets) != 0 {
			t.Errorf("an ambiguous head issued %d retarget calls; want 0", len(gh.retargets))
		}
	})

	t.Run("probe-error-is-unknown", func(t *testing.T) {
		gh := &fakeRetargetGitHub{repo: retargetRepo(),
			prs:     []*fakePR{{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810"}},
			findErr: map[string]error{"feat/child-a": fmt.Errorf("gh probe timed out")},
		}
		engine := &recordingEngine{}
		fake := &fakeReader{pin: pin, corpus: base}
		req := RetargetChildrenRequest{ID: 80, Version: "blobfin0080",
			Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}}}
		got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
		if got.Result != ResultExternalFailed || got.Disposition != RetargetDispositionUnknown {
			t.Fatalf("result=%q disposition=%q, want external-failed/unknown", got.Result, got.Disposition)
		}
		if len(gh.retargets) != 0 {
			t.Errorf("an unknown probe issued %d retarget calls; want 0", len(gh.retargets))
		}
	})
}

// TestRetargetChildrenParentVersionDrift: a parent record whose live version no
// longer matches the pinned authorization version is contended, before any
// external effect.
func TestRetargetChildrenParentVersionDrift(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
		finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
	}
	gh := &fakeRetargetGitHub{repo: retargetRepo(), prs: []*fakePR{
		{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810"},
	}}
	engine := &recordingEngine{}
	fake := &fakeReader{pin: pin, corpus: corpus}
	req := RetargetChildrenRequest{ID: 80, Version: "blobfin-STALE",
		Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}}}

	got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
	if got.Result != ResultContended || got.Reason != ReasonRetargetVersionDrift {
		t.Fatalf("result=%q reason=%q, want contended/%s", got.Result, got.Reason, ReasonRetargetVersionDrift)
	}
	if len(gh.finds) != 0 || len(gh.retargets) != 0 {
		t.Errorf("a stale parent version reached external probes: finds=%d retargets=%d", len(gh.finds), len(gh.retargets))
	}
}

// TestRetargetChildrenLeavesStackedOn: the operation executes no transaction — no
// metadata is written, so stacked_on is untouched by design — while still
// retargeting the authorized open child.
func TestRetargetChildrenLeavesStackedOn(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
		finalizeBlob(81, "child-a", "implemented", "high", prRefFor(810), "stacked_on: 80\n"),
	}
	gh := &fakeRetargetGitHub{repo: retargetRepo(), prs: []*fakePR{
		{number: 810, head: "feat/child-a", base: "feat/root", version: "cv810"},
	}}
	engine := &recordingEngine{}
	fake := &fakeReader{pin: pin, corpus: corpus}
	req := RetargetChildrenRequest{ID: 80, Version: "blobfin0080",
		Children: []AuthorizedChild{{ID: 81, PRNumber: 810, PRVersion: "cv810"}}}

	got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
	if got.Result != ResultApplied {
		t.Fatalf("result=%q reason=%q", got.Result, got.Reason)
	}
	if len(engine.calls) != 0 {
		t.Fatalf("retarget opened %d transactions; it must write no metadata", len(engine.calls))
	}
	if countEdits(gh.retargets) != 1 {
		t.Errorf("expected exactly one PR edit, got %d", countEdits(gh.retargets))
	}
}

// TestRetargetChildrenSkipsTerminalChildren: stacked-merged and done children do
// not block the parent merge and are never probed or edited; only the open,
// non-terminal authorized child is retargeted.
func TestRetargetChildrenSkipsTerminalChildren(t *testing.T) {
	pin := docketPin(t)
	corpus := []StatusBlob{
		finalizeBlob(80, "root", "implemented", "high", prRefFor(800), ""),
		finalizeBlob(81, "child-a", "stacked-merged", "high", prRefFor(810), "stacked_on: 80\n"),
		finalizeBlob(82, "child-b", "done", "high", prRefFor(820), "stacked_on: 80\n"),
		finalizeBlob(83, "child-c", "implemented", "high", prRefFor(830), "stacked_on: 80\n"),
	}
	gh := &fakeRetargetGitHub{repo: retargetRepo(), prs: []*fakePR{
		{number: 830, head: "feat/child-c", base: "feat/root", version: "cv830"},
	}}
	engine := &recordingEngine{}
	fake := &fakeReader{pin: pin, corpus: corpus}
	req := RetargetChildrenRequest{ID: 80, Version: "blobfin0080",
		Children: []AuthorizedChild{{ID: 83, PRNumber: 830, PRVersion: "cv830"}}}

	got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
	if got.Result != ResultApplied || got.Disposition != RetargetDispositionRetargeted {
		t.Fatalf("result=%q disposition=%q, want applied/retargeted", got.Result, got.Disposition)
	}
	// The terminal children never had their heads probed.
	for _, head := range gh.finds {
		if head == "feat/child-a" || head == "feat/child-b" {
			t.Errorf("terminal child head %q was probed; terminal children must be skipped", head)
		}
	}
	// The terminal children are surfaced as skipped, not omitted; only 830 edited.
	if c := childOutcomeByID(t, got, 81); c.Outcome != childOutcomeSkippedDone {
		t.Errorf("child 81 outcome=%q, want skipped-terminal", c.Outcome)
	}
	if c := childOutcomeByID(t, got, 82); c.Outcome != childOutcomeSkippedDone {
		t.Errorf("child 82 outcome=%q, want skipped-terminal", c.Outcome)
	}
	if c := childOutcomeByID(t, got, 83); c.Outcome != childOutcomeRetargeted {
		t.Errorf("child 83 outcome=%q, want retargeted", c.Outcome)
	}
	if countEdits(gh.retargets) != 1 {
		t.Errorf("expected exactly one edit (child 83), got %d", countEdits(gh.retargets))
	}
}

// TestRetargetChildrenShapeRefusals: request-shape violations are invalid-input
// carrying findings, reaching no external seam, and the document normalizes its
// collections (never null).
func TestRetargetChildrenShapeRefusals(t *testing.T) {
	pin := docketPin(t)
	gh := &fakeRetargetGitHub{repo: retargetRepo()}
	engine := &recordingEngine{}
	fake := &fakeReader{pin: pin, corpus: nil}

	// Missing version + a malformed child (id<=0, pr_number<=0, empty version) +
	// duplicate child id.
	req := RetargetChildrenRequest{ID: 80, Version: "",
		Children: []AuthorizedChild{
			{ID: 0, PRNumber: 0, PRVersion: ""},
			{ID: 81, PRNumber: 810, PRVersion: "cv810"},
			{ID: 81, PRNumber: 811, PRVersion: "cv811"},
		}}
	got := FinalizeRetargetChildren(context.Background(), retargetDeps(fake, gh, engine), "", req)
	if got.Result != ResultInvalidInput {
		t.Fatalf("result=%q, want invalid-input", got.Result)
	}
	if len(got.Findings) == 0 {
		t.Fatalf("shape refusal carried no findings")
	}
	if len(gh.finds) != 0 || len(gh.retargets) != 0 {
		t.Errorf("shape refusal reached an external seam: finds=%d retargets=%d", len(gh.finds), len(gh.retargets))
	}
	if fake.pinCount != 0 {
		t.Errorf("shape refusal pinned context %d times, want 0", fake.pinCount)
	}
	buf, _ := json.Marshal(got)
	if strings.Contains(string(buf), "null") {
		t.Errorf("null leaked into protocol document: %s", buf)
	}
	codes := map[string]bool{}
	for _, f := range got.Findings {
		codes[f.Code] = true
	}
	for _, want := range []string{"empty-version", "invalid-child_id", "invalid-child_pr_number", "empty-child_pr_version", "duplicate-child_id"} {
		if !codes[want] {
			t.Errorf("missing shape finding %q; got %v", want, codes)
		}
	}
}
