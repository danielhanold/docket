package app

import (
	"context"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
	"sort"
	"strings"
	"testing"
)

// run verify is the read-only postcondition report for the claim→implemented
// workflow. These tests pin its three closed verdicts (run-complete /
// run-unclaimed / run-incomplete), the mutation table that reddens each promised
// postcondition in turn, and the exit-code contract (every verdict exits 0; only
// operational errors exit non-zero).

const (
	rvSlug        = "widget"
	rvPlanPath    = "docs/superpowers/plans/2026-08-17-widget-plan.md"
	rvResultsPath = "docs/changes/results/0003-widget-results.md"
)

func rvRecordedPR() string { return prRepo().Spec() + "#42" }

// rvRecord renders an implemented change 3 carrying the given linkage. It starts
// from the in-progress shape (which holds branch/claimed_at/reconciled) and flips
// status to implemented so the surviving claim fields are present, exactly as a
// real mark-implemented transition leaves them.
func rvRecord(plan, results, pr, branch string) []byte {
	src := lifecycleChange(3, rvSlug, "in-progress")
	src = strings.Replace(src, "status: in-progress", "status: implemented", 1)
	if plan != "" {
		src = strings.Replace(src, "plan:\n", "plan: '"+plan+"'\n", 1)
	}
	if results != "" {
		src = strings.Replace(src, "results:\n", "results: '"+results+"'\n", 1)
	}
	if pr != "" {
		src = strings.Replace(src, "blocked_by:\n", "pr: '"+pr+"'\nblocked_by:\n", 1)
	}
	if branch != "" {
		src = strings.Replace(src, "branch: feat/"+rvSlug, "branch: "+branch, 1)
	}
	return []byte(src)
}

// rvPR is the single open PR the fake adapter reports for the feature branch.
func rvPR(head, body string) githubcli.PullRequest {
	return githubcli.PullRequest{
		Number: 42, State: githubcli.StateOpen, HeadBranch: "feat/" + rvSlug,
		HeadCommit: head, BaseBranch: "main", Body: body,
	}
}

// rvFixture is a real repo whose invocation clone holds a local feature commit
// (head) carrying the plan and results artifacts, optionally published to the
// bare origin as refs/heads/feat/widget.
type rvFixture struct {
	repo   *gitRepo
	client *gitcli.Client
	pin    StatusPin
	head   string
}

// newRunVerifyFixture builds the real-git fixture. The plan and results blobs are
// committed at head so run verify's blob reads succeed; when publish is true the
// feature head is pushed to origin so the remote-head postcondition holds.
func newRunVerifyFixture(t *testing.T, publish bool) *rvFixture {
	t.Helper()
	requireRealGit(t)
	repo := newWorkingRepo(t, nil)
	client := newGitClient(t)

	runGit(t, repo.invocation, "checkout", "-q", "-b", "feat/"+rvSlug)
	writeRepoFile(t, repo.invocation, rvPlanPath, "# plan\n")
	writeRepoFile(t, repo.invocation, rvResultsPath, "# results\n")
	runGit(t, repo.invocation, "add", "-A")
	runGit(t, repo.invocation, "commit", "-q", "-m", "feature work")
	head := runGit(t, repo.invocation, "rev-parse", "HEAD")
	if publish {
		runGit(t, repo.invocation, "push", "-q", "origin", "feat/"+rvSlug)
	}
	runGit(t, repo.invocation, "checkout", "-q", "main")
	return &rvFixture{repo: repo, client: client, pin: mainPin(t), head: head}
}

// deps assembles the run-verify deps over a fixture: the fake reader supplies the
// implemented corpus record, the fake workspace service reports the local head,
// the fake GitHub adapter reports the PR, and the real client performs the remote
// probe and blob reads.
func (f *rvFixture) deps(record []byte, pr githubcli.PullRequest) (PlanningDeps, WorkspaceDeps, GitHubDeps) {
	reader := &fakeReader{
		pin:    f.pin,
		corpus: []StatusBlob{{Kind: repository.KindChange, Location: repository.LocationActive, Path: groomPath(3, rvSlug), Version: miVersion, Data: record}},
		facts:  domain.NewBranchFacts(nil),
	}
	deps := PlanningDeps{Client: f.client, Reader: reader, Clock: testClock()}
	wdeps := WorkspaceDeps{Service: &fakeWorkspaceService{inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(f.head)}}}
	gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo(), probePRs: []githubcli.PullRequest{pr}}}
	return deps, wdeps, gdeps
}

// unmetReasons projects the result's conjunct list onto its stable reason
// strings, sorted for a stable comparison.
func unmetReasons(res RunVerifyResult) []string {
	out := make([]string, 0, len(res.Unmet))
	for _, u := range res.Unmet {
		out = append(out, u.Reason)
	}
	sort.Strings(out)
	return out
}

// rvProposedDeps builds reader-only deps over a proposed (never-claimed) change 3.
func rvProposedDeps(t *testing.T) PlanningDeps {
	t.Helper()
	reader := &fakeReader{
		pin:    mainPin(t),
		corpus: []StatusBlob{{Kind: repository.KindChange, Location: repository.LocationActive, Path: groomPath(3, rvSlug), Version: miVersion, Data: []byte(lifecycleChange(3, rvSlug, "proposed"))}},
		facts:  domain.NewBranchFacts(nil),
	}
	return PlanningDeps{Reader: reader, Clock: testClock()}
}

// rvRecordedPRURL is the canonical full-URL form of the recorded pr:, the form
// 0344's writer now stamps. Its host/owner/name mirror prRepo().
func rvRecordedPRURL() string { return "https://github.com/acme/widget/pull/42" }

// TestRunVerifyUnclaimed: a proposed change that was never claimed ⇒
// run-unclaimed, reached before any Git or GitHub probe.
func TestRunVerifyUnclaimed(t *testing.T) {
	res := RunVerify(context.Background(), rvProposedDeps(t), WorkspaceDeps{}, GitHubDeps{}, "unused", RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunUnclaimed {
		t.Fatalf("verdict = %q, want %q", res.Verdict, VerdictRunUnclaimed)
	}
	if len(res.Unmet) != 0 {
		t.Errorf("run-unclaimed carried unmet conjuncts: %v", unmetReasons(res))
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("run-unclaimed exit code = %d, want 0", code)
	}
}

// --- run-waiting: local receipt-derived verdict (Task 12) --------------------

// fakeWaitingReader is the injected WaitingReceiptReader seam. It returns a
// preassembled receipt bundle so a test can drive one fully-agreeing chain and
// mutate exactly one receipt dimension per row.
type fakeWaitingReader struct {
	receipt WaitingReceipt
	found   bool
	err     error
}

func (f fakeWaitingReader) Read(_ context.Context, _ string, _ int) (WaitingReceipt, bool, error) {
	return f.receipt, f.found, f.err
}

// rvInProgressRecord renders an in-progress (claimed, not yet implemented)
// change 3 carrying the given linkage — the status a waiting run sits in
// (waiting is private runtime state and never advances the change file).
func rvInProgressRecord(plan, results, branch string) []byte {
	src := lifecycleChange(3, rvSlug, "in-progress")
	if plan != "" {
		src = strings.Replace(src, "plan:\n", "plan: '"+plan+"'\n", 1)
	}
	if results != "" {
		src = strings.Replace(src, "results:\n", "results: '"+results+"'\n", 1)
	}
	if branch != "" {
		src = strings.Replace(src, "branch: feat/"+rvSlug, "branch: "+branch, 1)
	}
	return []byte(src)
}

// rvAgreeingReceipt is the fully-agreeing local receipt chain for the fixture
// head: every independent condition of the run-waiting verdict holds. Each
// mutation row below flips exactly one of these to prove waiting disappears.
func rvAgreeingReceipt(head string) WaitingReceipt {
	fp := gatedrive.Fingerprint{Head: head, Index: "idx", Status: "st", Worktree: "wt", Entries: 2}
	return WaitingReceipt{
		DriveID:             "d0opaque",
		HasUnclaimedHandoff: true,
		ChangeID:            "3",
		TaskID:              "t1",
		Phase:               "build",
		Branch:              "feat/" + rvSlug,
		WorktreePath:        "/some/worktree",
		WorktreeExists:      true,
		DriveHead:           head,
		DriveFingerprint:    fp,
		LiveFingerprint:     fp,
		DeadlineLive:        true,
		TerminalWaiting:     false,
		RawRunMatches:       true,
	}
}

// rvWaitingDeps assembles a published, in-progress fixture whose only unmet
// postcondition is not-implemented (so the reprobe is non-empty and run verify
// reaches the waiting check), wiring the given waiting reader.
func rvWaitingDeps(t *testing.T, f *rvFixture, reader WaitingReceiptReader) (PlanningDeps, WorkspaceDeps, GitHubDeps) {
	t.Helper()
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, rvResultsPath, "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Waiting = reader
	return deps, wdeps, gdeps
}

// TestRunVerifyHaltedPrecedesHandoff: a durable persisted run-halt stays terminal
// even when a fully-agreeing local handoff receipt is present.
func TestRunVerifyHaltedPrecedesHandoff(t *testing.T) {
	src := strings.TrimRight(lifecycleChange(3, "widget", "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"
	corpus := []StatusBlob{{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(3, "widget"),
		Version:  "v3",
		Data:     []byte(src),
	}}
	fake := &fakeReader{pin: docketPin(t), corpus: corpus}
	wdeps := WorkspaceDeps{Waiting: fakeWaitingReader{receipt: rvAgreeingReceipt("deadbeef"), found: true}}
	got := RunVerify(context.Background(), PlanningDeps{Reader: fake, Clock: testClock()},
		wdeps, GitHubDeps{}, "", RunVerifyRequest{ID: 3})
	if got.Verdict != VerdictRunHalted {
		t.Fatalf("verdict = %q, want %q (a persisted halt stays terminal)", got.Verdict, VerdictRunHalted)
	}
}

// TestRunVerifyHaltedVerdict proves `run verify` reports the closed run-halted
// verdict for a change carrying the durable "## Run halted" marker,
// short-circuiting the postcondition reprobe (no git or GitHub is consulted).
func TestRunVerifyHaltedVerdict(t *testing.T) {
	src := strings.TrimRight(lifecycleChange(3, "widget", "in-progress"), "\n") +
		"\n\n## Run halted\n\n### 2026-08-14\n\nPaused.\n"
	corpus := []StatusBlob{{
		Kind:     repository.KindChange,
		Location: repository.LocationActive,
		Path:     groomPath(3, "widget"),
		Version:  "v3",
		Data:     []byte(src),
	}}
	fake := &fakeReader{pin: docketPin(t), corpus: corpus}
	got := RunVerify(context.Background(), PlanningDeps{Reader: fake, Clock: testClock()},
		WorkspaceDeps{}, GitHubDeps{}, "", RunVerifyRequest{ID: 3})
	if got.Verdict != VerdictRunHalted {
		t.Fatalf("verdict=%q reason=%q, want %q", got.Verdict, got.Reason, VerdictRunHalted)
	}
	if got.Result != ResultApplied {
		t.Errorf("result=%q, want a success-shaped verdict envelope", got.Result)
	}
}

// TestRunVerifyAcceptsSkippedEvidenceAtExactHead: a build.gate: off repository's
// PR carries a truthful skipped (build-gate-off) block at the exact feature head.
// run verify's evidence postcondition accepts VerdictSkipped exactly as
// VerdictVerified, so the run is complete. Mirrors TestIntegrationChangeRunVerifyComplete
// with a skipped PR-body block substituted.
func TestRunVerifyAcceptsSkippedEvidenceAtExactHead(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prSkippedEvidenceBytes(t, f.head))),
	)
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunComplete {
		t.Fatalf("verdict = %q, want %q — a skipped PR-body block at the exact head verifies (unmet %v)", res.Verdict, VerdictRunComplete, unmetReasons(res))
	}
}
