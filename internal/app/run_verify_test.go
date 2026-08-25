package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/workspace"
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
	repo := newMainModeRepo(t, nil)
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

// TestRunVerifyPRIdentityForms is the mutation test for the migrated PR-identity
// conjunct: run verify accepts a recorded pr: in EITHER form (canonical URL or
// legacy owner/repo#N shorthand) when its parsed number equals the verified PR's
// number, and flags pr-unverified when the number differs or the recorded value
// is unparseable. The verified PR is number 42 (rvPR).
func TestRunVerifyPRIdentityForms(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	ev := string(prEvidenceBytes(t, f.head))

	cases := []struct {
		name       string
		recorded   string
		wantVerify bool
	}{
		{"url form matches", rvRecordedPRURL(), true},
		{"shorthand form matches", rvRecordedPR(), true},
		{"url form wrong number", "https://github.com/acme/widget/pull/99", false},
		{"shorthand wrong number", prRepo().Spec() + "#99", false},
		{"unparseable recorded pr", "not-a-pr-ref", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps, wdeps, gdeps := f.deps(
				rvRecord(rvPlanPath, rvResultsPath, tc.recorded, "feat/"+rvSlug),
				rvPR(f.head, ev),
			)
			res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
			reasons := unmetReasons(res)
			hasPRUnverified := false
			for _, r := range reasons {
				if r == ReasonRunPRUnverified {
					hasPRUnverified = true
				}
			}
			if tc.wantVerify {
				if res.Verdict != VerdictRunComplete {
					t.Fatalf("recorded %q: verdict = %q, want run-complete (unmet %v)", tc.recorded, res.Verdict, reasons)
				}
			} else if !hasPRUnverified {
				t.Fatalf("recorded %q: expected a pr-unverified conjunct, got unmet %v (verdict %q)", tc.recorded, reasons, res.Verdict)
			}
		})
	}
}

// TestRunVerifyComplete: an implemented change satisfying every postcondition ⇒
// run-complete with no unmet conjuncts.
func TestRunVerifyComplete(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunComplete {
		t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunComplete, unmetReasons(res))
	}
	if len(res.Unmet) != 0 {
		t.Errorf("run-complete carried unmet conjuncts: %v", unmetReasons(res))
	}
	if res.Head != f.head {
		t.Errorf("head = %q, want %q", res.Head, f.head)
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("run-complete exit code = %d, want 0", code)
	}
}

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

// TestRunVerifyIncompleteEnumeratesConjuncts is the spec's own testing rule: each
// row mutates or removes exactly one promised postcondition and expects
// run-incomplete carrying that conjunct's stable reason — asserted as the FULL
// unmet list, not merely non-empty. The happy fixture (TestRunVerifyComplete)
// satisfies all of them.
func TestRunVerifyIncompleteEnumeratesConjuncts(t *testing.T) {
	pub := newRunVerifyFixture(t, true)
	ev := string(prEvidenceBytes(t, pub.head))
	recordedPR := rvRecordedPR()
	ghostPlan := "docs/superpowers/plans/2026-08-17-ghost.md"
	ghostResults := "docs/changes/results/0003-ghost.md"

	rows := []struct {
		name   string
		record []byte
		pr     githubcli.PullRequest
		want   string
	}{
		{
			name:   "missing plan link",
			record: rvRecord("", rvResultsPath, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(pub.head, ev),
			want:   ReasonRunPlanUnlinked,
		},
		{
			name:   "plan file gone at recorded path",
			record: rvRecord(ghostPlan, rvResultsPath, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(pub.head, ev),
			want:   ReasonRunPlanMissing,
		},
		{
			name:   "stale evidence names another head",
			record: rvRecord(rvPlanPath, rvResultsPath, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(pub.head, string(prEvidenceBytes(t, prOtherHead))),
			want:   ReasonRunEvidenceUnverified,
		},
		{
			name:   "PR names another head",
			record: rvRecord(rvPlanPath, rvResultsPath, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(prOtherHead, ev),
			want:   ReasonRunPRUnverified,
		},
		{
			name:   "results identity broken",
			record: rvRecord(rvPlanPath, ghostResults, recordedPR, "feat/"+rvSlug),
			pr:     rvPR(pub.head, ev),
			want:   ReasonRunResultsIdentity,
		},
		{
			name:   "lease contended",
			record: rvRecord(rvPlanPath, rvResultsPath, recordedPR, "feat/other"),
			pr:     rvPR(pub.head, ev),
			want:   ReasonRunLeaseContended,
		},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			deps, wdeps, gdeps := pub.deps(row.record, row.pr)
			res := RunVerify(context.Background(), deps, wdeps, gdeps, pub.repo.invocation, RunVerifyRequest{ID: 3})
			if res.Verdict != VerdictRunIncomplete {
				t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunIncomplete, unmetReasons(res))
			}
			if got := unmetReasons(res); len(got) != 1 || got[0] != row.want {
				t.Fatalf("unmet = %v, want exactly [%s]", got, row.want)
			}
			if code := ExitCode(res.Env().Result); code != 0 {
				t.Errorf("run-incomplete exit code = %d, want 0", code)
			}
		})
	}

	// The remote-head postcondition needs an unpublished feature head: the local
	// head exists but the remote never received it, so the remote is absent.
	t.Run("feature head differs from remote", func(t *testing.T) {
		unpub := newRunVerifyFixture(t, false)
		deps, wdeps, gdeps := unpub.deps(
			rvRecord(rvPlanPath, rvResultsPath, recordedPR, "feat/"+rvSlug),
			rvPR(unpub.head, string(prEvidenceBytes(t, unpub.head))),
		)
		res := RunVerify(context.Background(), deps, wdeps, gdeps, unpub.repo.invocation, RunVerifyRequest{ID: 3})
		if res.Verdict != VerdictRunIncomplete {
			t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunIncomplete, unmetReasons(res))
		}
		if got := unmetReasons(res); len(got) != 1 || got[0] != ReasonRunRemoteHeadMismatch {
			t.Fatalf("unmet = %v, want exactly [%s]", got, ReasonRunRemoteHeadMismatch)
		}
	})
}

// TestRunVerifyOperationalError: an absent id is an operational error, not a
// verdict — it carries no verdict and exits non-zero.
func TestRunVerifyOperationalError(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug), rvPR(f.head, string(prEvidenceBytes(t, f.head))))
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 999})
	if res.Verdict != "" {
		t.Errorf("operational error carried a verdict %q", res.Verdict)
	}
	if code := ExitCode(res.Env().Result); code == 0 {
		t.Errorf("operational error exit code = 0, want non-zero (result %q)", res.Env().Result)
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

// TestRunVerifyWaitingAgreeingChain: a fully-agreeing local receipt chain over an
// in-progress change yields run-waiting, exposing the opaque handoff id and phase
// (never an owner credential), as a success-shaped, exit-0 verdict.
func TestRunVerifyWaitingAgreeingChain(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	reader := fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true}
	deps, wdeps, gdeps := rvWaitingDeps(t, f, reader)

	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunWaiting {
		t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunWaiting, unmetReasons(res))
	}
	if res.HandoffID != "d0opaque" {
		t.Errorf("handoff id = %q, want %q", res.HandoffID, "d0opaque")
	}
	if res.Phase != "build" {
		t.Errorf("phase = %q, want %q", res.Phase, "build")
	}
	if len(res.Unmet) != 0 {
		t.Errorf("run-waiting carried unmet conjuncts: %v", unmetReasons(res))
	}
	if code := ExitCode(res.Env().Result); code != 0 {
		t.Errorf("run-waiting exit code = %d, want 0", code)
	}
}

// TestRunVerifyWaitingTerminalOverridesDeadline: an expired deadline still yields
// run-waiting WHEN a durable terminal result is waiting to be consumed — the one
// admitted exception to the live-deadline condition.
func TestRunVerifyWaitingTerminalOverridesDeadline(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	rcpt := rvAgreeingReceipt(f.head)
	rcpt.DeadlineLive = false
	rcpt.TerminalWaiting = true
	deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rcpt, found: true})

	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunWaiting {
		t.Fatalf("verdict = %q, want %q", res.Verdict, VerdictRunWaiting)
	}
}

// TestRunVerifyWaitingMutationsDisappear is the spec's mutation rule: flip exactly
// one receipt dimension of the agreeing chain and prove waiting disappears —
// falling through to the ordinary run-incomplete verdict. A found=false / errored
// reader (missing local state, e.g. another machine) also never invents waiting.
func TestRunVerifyWaitingMutationsDisappear(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	base := rvAgreeingReceipt(f.head)

	rows := []struct {
		name    string
		mutate  func(*WaitingReceipt)
		found   bool
		readErr error
	}{
		{name: "head drift (drive vs workspace)", mutate: func(r *WaitingReceipt) { r.DriveHead = "0000000000000000000000000000000000000000" }, found: true},
		{name: "head drift (live vs drive)", mutate: func(r *WaitingReceipt) { r.LiveFingerprint.Head = "0000000000000000000000000000000000000000" }, found: true},
		{name: "fingerprint drift", mutate: func(r *WaitingReceipt) { r.LiveFingerprint.Worktree = "drifted" }, found: true},
		{name: "claimed handoff", mutate: func(r *WaitingReceipt) { r.HasUnclaimedHandoff = false }, found: true},
		{name: "expired deadline without terminal", mutate: func(r *WaitingReceipt) { r.DeadlineLive = false; r.TerminalWaiting = false }, found: true},
		{name: "mismatched raw run", mutate: func(r *WaitingReceipt) { r.RawRunMatches = false }, found: true},
		{name: "broken chain: change id mismatch", mutate: func(r *WaitingReceipt) { r.ChangeID = "99" }, found: true},
		{name: "broken chain: empty drive id", mutate: func(r *WaitingReceipt) { r.DriveID = "" }, found: true},
		{name: "broken chain: empty phase", mutate: func(r *WaitingReceipt) { r.Phase = "" }, found: true},
		{name: "worktree missing", mutate: func(r *WaitingReceipt) { r.WorktreeExists = false }, found: true},
		{name: "recorded branch mismatch", mutate: func(r *WaitingReceipt) { r.Branch = "feat/other" }, found: true},
		{name: "missing local state (not found)", mutate: func(r *WaitingReceipt) {}, found: false},
		{name: "reader error", mutate: func(r *WaitingReceipt) {}, found: true, readErr: errors.New("store unreadable")},
	}

	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			rcpt := base
			row.mutate(&rcpt)
			deps, wdeps, gdeps := rvWaitingDeps(t, f, fakeWaitingReader{receipt: rcpt, found: row.found, err: row.readErr})
			res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
			if res.Verdict == VerdictRunWaiting {
				t.Fatalf("mutation %q still reported run-waiting", row.name)
			}
			if res.Verdict != VerdictRunIncomplete {
				t.Fatalf("mutation %q verdict = %q, want %q", row.name, res.Verdict, VerdictRunIncomplete)
			}
		})
	}
}

// TestRunVerifyCompletePrecedesStaleHandoff: when every completed-run
// postcondition holds, run-complete wins even though a fully-agreeing local
// handoff receipt is present.
func TestRunVerifyCompletePrecedesStaleHandoff(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Waiting = fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true}

	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunComplete {
		t.Fatalf("verdict = %q, want %q (a completed run must outrank a stale handoff)", res.Verdict, VerdictRunComplete)
	}
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
