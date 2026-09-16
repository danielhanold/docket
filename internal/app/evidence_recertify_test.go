package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/githubcli"
)

// buildVsFinalizeYAML declares DIVERGENT build/finalize commands so a test can
// prove which owner's command a gate resolved (acceptance: differing commands
// prove only the BUILD command runs).
const buildVsFinalizeYAML = "build:\n  gate: local\n  test_command: go test ./build-only\nfinalize:\n  test_command: make finalize-only\n"

// TestBuildLocalGateResolvesBuildCommandOnly: the BUILD-owned production gate
// resolves build.test_command; the finalize twin resolves finalize.test_command
// from the same pin. Deleting the owner branch in buildDriveService reddens one
// of the two arms.
func TestBuildLocalGateResolvesBuildCommandOnly(t *testing.T) {
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), buildVsFinalizeYAML)
	ctx := context.Background()

	bg := NewBuildLocalGate(deps, wdeps).(*processFinalizeGate)
	bsvc, ok := bg.buildDriveService(ctx, repoDir)
	if !ok || bsvc.command != "go test ./build-only" {
		t.Fatalf("build gate resolved (ok=%v, command=%q); want build.test_command %q", ok, commandOf(bsvc), "go test ./build-only")
	}
	fg := NewFinalizeGate(deps, wdeps).(*processFinalizeGate)
	fsvc, ok := fg.buildDriveService(ctx, repoDir)
	if !ok || fsvc.command != "make finalize-only" {
		t.Fatalf("finalize gate resolved (ok=%v, command=%q); want finalize.test_command %q", ok, commandOf(fsvc), "make finalize-only")
	}
}

// commandOf tolerates a nil service in a failure message.
func commandOf(svc *GateDriveService) string {
	if svc == nil {
		return "<nil>"
	}
	return svc.command
}

// TestBuildLocalGateFailsClosedWithoutBuildCommand: a config with ONLY
// finalize.test_command set fails the build-owned gate closed (ok=false → the
// caller halts, never a fabricated red) while the finalize twin still resolves.
// This pins the guard's keying on the owner's OWN config key.
func TestBuildLocalGateFailsClosedWithoutBuildCommand(t *testing.T) {
	yaml := "finalize:\n  test_command: make finalize-only\n"
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), yaml)
	ctx := context.Background()

	bg := NewBuildLocalGate(deps, wdeps).(*processFinalizeGate)
	if _, ok := bg.buildDriveService(ctx, repoDir); ok {
		t.Fatalf("build-owned gate resolved a drive service with no build.test_command; must fail closed")
	}
	fg := NewFinalizeGate(deps, wdeps).(*processFinalizeGate)
	if _, ok := fg.buildDriveService(ctx, repoDir); !ok {
		t.Fatalf("finalize-owned gate must still resolve from finalize.test_command")
	}
}

// greenBlockFor renders a bare canonical green evidence block certifying head
// with the fixture repo's resolved build.test_command ("go test ./..." — see
// buildConfiguredRepo's .docket.yml).
func greenBlockFor(t *testing.T, head string) string {
	t.Helper()
	rec, err := evidence.NewRecord("go test ./...", head, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("evidence.NewRecord: %v", err)
	}
	return evidence.Render(rec)
}

// recertifyFixture assembles the standard recertify scenario over the real-git
// rebase fixture: an implemented change, a clean published feature workspace,
// and one open PR at the current head whose body carries STALE green evidence
// (it names the base tip, an older real commit). gate is injected per test.
func recertifyFixture(t *testing.T, gate FinalizeGate) (*rebaseFixture, *fakePublishGitHub, FinalizeDeps, WorkspaceDeps) {
	t.Helper()
	f := setupRebaseFixture(t, planRepoModes()[0])
	staleBody := greenEvidenceFor(t, f.baseTip) // "Authored prose.\n\n<block>\nMore prose.\n"
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, staleBody)}
	deps := f.finalizeDeps(gh, gate)
	return f, gh, deps, WorkspaceDeps{Service: f.svc}
}

// TestEvidenceRecertifyRefusesNotImplemented: any non-implemented status is
// blocked before any probe of the gate or PR edit (acceptance 3).
func TestEvidenceRecertifyRefusesNotImplemented(t *testing.T) {
	f := setupRebaseFixtureStatus(t, planRepoModes()[0], "in-progress")
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, &fakeGate{}), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyNotImplemented {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyNotImplemented)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a refused recertify edited the PR")
	}
}

// TestEvidenceRecertifyRefusesDirtyWorkspace: uncommitted work blocks (never
// gated over, never published) — acceptance 3.
func TestEvidenceRecertifyRefusesDirtyWorkspace(t *testing.T) {
	f, gh, deps, wdeps := recertifyFixture(t, &fakeGate{})
	writeRepoFile(t, f.wp, "dirty.txt", "uncommitted\n")
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyWorkspaceDirty {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyWorkspaceDirty)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a refused recertify edited the PR")
	}
}

// TestEvidenceRecertifyRefusesUnpublishedFollowUp: a local follow-up commit
// that was never pushed disagrees with the remote feature head; the operation
// refuses (publish first through the existing workflow) rather than certify a
// head the PR does not hold.
func TestEvidenceRecertifyRefusesUnpublishedFollowUp(t *testing.T) {
	f, gh, deps, wdeps := recertifyFixture(t, &fakeGate{})
	writeRepoFile(t, f.wp, "followup.txt", "review fix\n")
	runGit(t, f.wp, "add", "-A")
	runGit(t, f.wp, "commit", "-q", "-m", "review fix")
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result == ResultApplied || res.Reason != ReasonRecertifyHeadDisagreement {
		t.Fatalf("result = %s/%s; want a %s refusal", res.Result, res.Reason, ReasonRecertifyHeadDisagreement)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a refused recertify edited the PR")
	}
}

// TestEvidenceRecertifyRefusesClosedOrMismatchedPR: no open PR for the feature
// head refuses (pr-not-open); an open PR naming a different head refuses
// (head-disagreement). Neither runs the gate — acceptance 3.
func TestEvidenceRecertifyRefusesClosedOrMismatchedPR(t *testing.T) {
	// Closed: the fake returns no open PR when State is not open.
	f, gh, deps, wdeps := recertifyFixture(t, &fakeGate{})
	gh.pr.State = githubcli.StateClosed
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyPRNotOpen {
		t.Fatalf("closed PR: result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyPRNotOpen)
	}

	// Mismatched head: the open PR names the base tip, not the feature head.
	f2 := setupRebaseFixture(t, planRepoModes()[0])
	gh2 := &fakePublishGitHub{repo: retargetRepo(), pr: f2.prForHead(f2.baseTip, greenEvidenceFor(t, f2.baseTip))}
	gate2 := &fakeGate{}
	res2 := EvidenceRecertify(context.Background(), f2.finalizeDeps(gh2, gate2), WorkspaceDeps{Service: f2.svc},
		f2.repo.invocation, EvidenceRecertifyRequest{ID: f2.id})
	if res2.Result == ResultApplied || res2.Reason != ReasonRecertifyHeadDisagreement {
		t.Fatalf("mismatched PR head: result = %s/%s; want a %s refusal", res2.Result, res2.Reason, ReasonRecertifyHeadDisagreement)
	}
	if gate2.calls != 0 {
		t.Fatalf("a refused recertify ran the gate")
	}
}

// TestEvidenceRecertifyShape: a non-positive id is an invalid-input shape
// refusal before any probe.
func TestEvidenceRecertifyShape(t *testing.T) {
	f, _, deps, wdeps := recertifyFixture(t, &fakeGate{})
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: 0})
	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %s; want %s", res.Result, ResultInvalidInput)
	}
}

// TestEvidenceRecertifyHappyPath: stale evidence at an older head becomes
// verified evidence for the exact current head on the SAME open PR; authored
// body bytes survive; the result is applied/green (acceptance 1).
func TestEvidenceRecertifyHappyPath(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}

	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultApplied || res.Outcome != RecertifyOutcomeGreen {
		t.Fatalf("result = %s/%s outcome %q (reason %q: %s); want applied green", res.Result, res.Reason, res.Outcome, res.Reason, res.Message)
	}
	if res.Head != f.head || res.Number != 1 {
		t.Fatalf("result names head %q PR %d; want %q PR 1", res.Head, res.Number, f.head)
	}
	if gh.ensNext != 1 {
		t.Fatalf("EnsurePullRequest calls = %d; want exactly 1", gh.ensNext)
	}
	body := gh.lastEnsuredBody()
	if evidence.Verify([]byte(body), f.head) != evidence.VerdictVerified {
		t.Fatalf("converged PR body does not verify green for the current head:\n%s", body)
	}
	if !strings.Contains(body, "Authored prose.") || !strings.Contains(body, "More prose.") {
		t.Fatalf("authored PR body bytes were not preserved:\n%s", body)
	}
	if gh.ensLast.ExpectedHead != f.head || gh.ensLast.ExpectedVersion == "" {
		t.Fatalf("PR edit was not pinned to head+version: %+v", gh.ensLast)
	}
}

// TestEvidenceRecertifyAdvancesOneDriveAcrossWaiting: WAITING is nonterminal —
// the operation re-enters the SAME drive (continuation threaded) until a
// terminal, and only then publishes. Deleting the loop's continuation
// threading reddens the continuation asserts.
func TestEvidenceRecertifyAdvancesOneDriveAcrossWaiting(t *testing.T) {
	f0 := setupRebaseFixture(t, planRepoModes()[0])
	gate := &seqGate{results: []LocalGateResult{
		{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "d1", Generation: "g1"}},
		{Outcome: FinalizeGateWaiting, Continuation: GateContinuation{DriveID: "d1", Generation: "g1"}},
		{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f0.head), RunDir: "/run/x"},
	}}
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f0.prForHead(f0.head, greenEvidenceFor(t, f0.baseTip))}
	res := EvidenceRecertify(context.Background(), f0.finalizeDeps(gh, gate), WorkspaceDeps{Service: f0.svc},
		f0.repo.invocation, EvidenceRecertifyRequest{ID: f0.id})
	if res.Result != ResultApplied {
		t.Fatalf("result = %s/%s: %s; want applied", res.Result, res.Reason, res.Message)
	}
	if len(gate.reqs) != 3 || gate.reqs[0].Continuation.DriveID != "" || gate.reqs[1].Continuation.DriveID != "d1" || gate.reqs[2].Continuation.DriveID != "d1" {
		t.Fatalf("continuation threading = %+v; want empty, then d1, then d1", gate.reqs)
	}
}

// TestEvidenceRecertifyGateFailureAndHalt: a red suite is gate-failed (repair
// work) and a halt is blocked — neither touches the PR (acceptance 3), and a
// WAITING with no continuation fails closed instead of spinning.
func TestEvidenceRecertifyGateFailureAndHalt(t *testing.T) {
	cases := []struct {
		name   string
		result LocalGateResult
		err    error
		want   Result
		reason string
	}{
		{"failed", LocalGateResult{Outcome: FinalizeGateFailed, RunDir: "/run/red"}, nil, ResultGateFailed, ReasonRecertifyGateFailed},
		{"halted", LocalGateResult{Outcome: FinalizeGateHalted, HaltCause: GateHaltRunningAtBudget}, nil, ResultBlocked, ReasonRecertifyGateHalted},
		{"seam error", LocalGateResult{}, errors.New("seam down"), ResultBlocked, ReasonRecertifyGateHalted},
		{"waiting without continuation", LocalGateResult{Outcome: FinalizeGateWaiting}, nil, ResultBlocked, ReasonRecertifyGateHalted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gate := &fakeGate{result: tc.result, err: tc.err}
			f, gh, deps, wdeps := recertifyFixture(t, gate)
			res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
			if res.Result != tc.want || res.Reason != tc.reason {
				t.Fatalf("result = %s/%s; want %s/%s", res.Result, res.Reason, tc.want, tc.reason)
			}
			if gh.ensNext != 0 {
				t.Fatalf("a non-passed gate edited the PR")
			}
		})
	}
}

// TestEvidenceRecertifyGateOffRecordsSkipped: build.gate off mints truthful
// skipped evidence and completes as skipped WITHOUT editing the PR block —
// evidence.Upsert is green-only by design and this change preserves evidence
// rendering (acceptance 2).
func TestEvidenceRecertifyGateOffRecordsSkipped(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	// Config resolves from the pinned default-branch tip (origin/main), never the
	// invocation working tree (loadOperationalContext reads readPinnedOptionalBlob
	// at defaultRev), so the gate-off policy must be published to origin/main.
	f.repo.writerAdvance(t, "main", map[string]string{
		".docket.yml": "integration_branch: main\nbuild:\n  gate: 'off'\nfinalize:\n  test_command: 'go test ./...'\n",
	})

	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultApplied || res.Outcome != RecertifyOutcomeSkipped {
		t.Fatalf("result = %s/%s outcome %q: %s; want applied skipped", res.Result, res.Reason, res.Outcome, res.Message)
	}
	if gate.calls != 0 {
		t.Fatalf("gate ran under build.gate: off")
	}
	if gh.ensNext != 0 {
		t.Fatalf("a skipped recertify edited the PR block")
	}
}

// TestEvidenceRecertifyRefusesUnconfiguredGate: a local build gate with no
// build.test_command refuses; no suite, no PR edit (acceptance 2).
func TestEvidenceRecertifyRefusesUnconfiguredGate(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	// Config resolves from the pinned default-branch tip (origin/main), never the
	// invocation working tree, so the build-command-absent policy (gate defaults
	// local) must be published to origin/main.
	f.repo.writerAdvance(t, "main", map[string]string{
		".docket.yml": "integration_branch: main\nfinalize:\n  test_command: 'go test ./...'\n",
	})

	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result = %s/%s; want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if gate.calls != 0 || gh.ensNext != 0 {
		t.Fatalf("an unconfigured gate ran the suite or edited the PR")
	}
}

// movingGate commits to the feature worktree DURING the gate and then reports
// PASSED for the pre-move head — the moved-HEAD publication hazard.
type movingGate struct {
	f  *rebaseFixture
	ev string
}

func (g *movingGate) RunLocalGate(context.Context, LocalGateRequest) (LocalGateResult, error) {
	writeRepoFile(g.f.t, g.f.wp, "late-edit.txt", "moved under the gate\n")
	runGit(g.f.t, g.f.wp, "add", "-A")
	runGit(g.f.t, g.f.wp, "commit", "-q", "-m", "late edit")
	return LocalGateResult{Outcome: FinalizeGatePassed, Evidence: g.ev, RunDir: "/run/x"}, nil
}

// TestEvidenceRecertifyRefusesHeadMovedUnderGate: a HEAD that moved between
// the gate and the publish can never publish (acceptance 3). The recheck's
// local-vs-remote leg catches it (the late commit is unpublished).
func TestEvidenceRecertifyRefusesHeadMovedUnderGate(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gate := &movingGate{f: f, ev: greenBlockFor(t, f.head)}
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, gate), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result == ResultApplied || res.Result == ResultNoOp {
		t.Fatalf("a moved head published evidence: %s/%s", res.Result, res.Reason)
	}
	if res.Reason != ReasonRecertifyHeadDisagreement && res.Reason != ReasonRecertifyIdentityDrift {
		t.Fatalf("reason = %q; want a head-disagreement/identity-drift refusal", res.Reason)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a moved head reached the PR edit")
	}
}

// TestEvidenceRecertifyRefusesForeignCommandEvidence: evidence recording a
// command other than the currently resolved build.test_command cannot publish —
// the changed-configuration face of acceptance 3.
func TestEvidenceRecertifyRefusesForeignCommandEvidence(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	foreign, err := evidence.NewRecord("make other-suite", f.head, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("evidence.NewRecord: %v", err)
	}
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: evidence.Render(foreign), RunDir: "/run/x"}
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyIdentityDrift {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyIdentityDrift)
	}
	if gh.ensNext != 0 {
		t.Fatalf("foreign-command evidence reached the PR edit")
	}
}

// TestEvidenceRecertifyRefusesWrongHeadEvidence: gate evidence naming another
// head is stale at verification and never published.
func TestEvidenceRecertifyRefusesWrongHeadEvidence(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.baseTip), RunDir: "/run/x"}
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultInvalidState || res.Reason != ReasonRecertifyEvidenceUnverified {
		t.Fatalf("result = %s/%s; want invalid-state/%s", res.Result, res.Reason, ReasonRecertifyEvidenceUnverified)
	}
	if gh.ensNext != 0 {
		t.Fatalf("unverified evidence reached the PR edit")
	}
}

// flakyEnsureGitHub reports the first N EnsurePullRequest calls as UNKNOWN
// (an unverifiable external effect) and then delegates to the real fake.
type flakyEnsureGitHub struct {
	*fakePublishGitHub
	unknowns int
}

func (f *flakyEnsureGitHub) EnsurePullRequest(ctx context.Context, req githubcli.EnsurePullRequestRequest) (githubcli.EnsureResult, error) {
	if f.unknowns > 0 {
		f.unknowns--
		return githubcli.EnsureResult{Disposition: githubcli.EnsureUnknown}, nil
	}
	return f.fakePublishGitHub.EnsurePullRequest(ctx, req)
}

// dispositionEnsureGitHub forces every EnsurePullRequest call to report a fixed
// disposition (with no error), letting a test drive publishRecertifiedEvidence's
// terminal disposition mapping directly.
type dispositionEnsureGitHub struct {
	*fakePublishGitHub
	disp githubcli.EnsureDisposition
}

func (f *dispositionEnsureGitHub) EnsurePullRequest(_ context.Context, req githubcli.EnsurePullRequestRequest) (githubcli.EnsureResult, error) {
	f.fakePublishGitHub.ensNext++
	f.fakePublishGitHub.ensLast = req
	return githubcli.EnsureResult{Disposition: f.disp}, nil
}

// TestEvidenceRecertifyEditContended: a PASSED gate whose PR edit comes back
// EnsureContended (the PR diverged under the update) is refused as
// contended/pr-edit-contended and reports NO completion — the contended arm of
// publishRecertifiedEvidence's disposition mapping. Swapping the mapped result
// to applied/green reddens the assert.
func TestEvidenceRecertifyEditContended(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	inner := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gh := &dispositionEnsureGitHub{fakePublishGitHub: inner, disp: githubcli.EnsureContended}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}}
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, gate), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultContended || res.Reason != ReasonRecertifyEditContended {
		t.Fatalf("result = %s/%s; want %s/%s", res.Result, res.Reason, ResultContended, ReasonRecertifyEditContended)
	}
	if res.Result == ResultApplied || res.Result == ResultNoOp || res.Outcome != "" {
		t.Fatalf("a contended edit reported completion: %s/%s outcome %q", res.Result, res.Reason, res.Outcome)
	}
	if inner.ensNext != 1 {
		t.Fatalf("EnsurePullRequest calls = %d; want exactly 1 (no second mutation)", inner.ensNext)
	}
}

// TestEvidenceRecertifyEditUnrecognizedDisposition: a PASSED gate whose PR edit
// returns an unrecognized (zero-value) disposition falls to the mapping's
// default arm — internal-error/status-internal-error — and reports NO
// completion. Deleting the default arm (so it fell through to applied) reddens
// the assert.
func TestEvidenceRecertifyEditUnrecognizedDisposition(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	inner := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gh := &dispositionEnsureGitHub{fakePublishGitHub: inner, disp: githubcli.EnsureDisposition("")}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}}
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, gate), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultInternalError || res.Reason != ReasonStatusInternalError {
		t.Fatalf("result = %s/%s; want %s/%s", res.Result, res.Reason, ResultInternalError, ReasonStatusInternalError)
	}
	if res.Result == ResultApplied || res.Result == ResultNoOp || res.Outcome != "" {
		t.Fatalf("an unrecognized disposition reported completion: %s/%s outcome %q", res.Result, res.Reason, res.Outcome)
	}
}

// dirtyingGate leaves an untracked, non-ignored file in the feature worktree
// DURING the gate and then reports PASSED — the post-gate-dirty publication
// hazard (a build command that does not clean up after itself).
type dirtyingGate struct {
	f  *rebaseFixture
	ev string
}

func (g *dirtyingGate) RunLocalGate(context.Context, LocalGateRequest) (LocalGateResult, error) {
	writeRepoFile(g.f.t, g.f.wp, "scratch.txt", "left behind by the build command\n")
	return LocalGateResult{Outcome: FinalizeGatePassed, Evidence: g.ev, RunDir: "/run/x"}, nil
}

// TestEvidenceRecertifyRefusesDirtyAfterGate: an untracked, non-ignored file the
// build command leaves in the worktree during a PASSED gate flips the
// pre-publish cleanliness recheck to workspace-dirty and refuses to publish; the
// PR is never edited. Pins the whole-predicate recheck's clean-worktree leg
// (documented as the clean-worktree caveat in the guide).
func TestEvidenceRecertifyRefusesDirtyAfterGate(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gate := &dirtyingGate{f: f, ev: greenBlockFor(t, f.head)}
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, gate), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyWorkspaceDirty {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyWorkspaceDirty)
	}
	if gh.ensNext != 0 {
		t.Fatalf("a dirty post-gate worktree reached the PR edit")
	}
}

// TestEvidenceRecertifyEditFailureThenRetry: an uncertain PR edit is NOT
// completion; a later invocation converges the SAME PR and preserves authored
// content (acceptance 4).
func TestEvidenceRecertifyEditFailureThenRetry(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	inner := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gh := &flakyEnsureGitHub{fakePublishGitHub: inner, unknowns: 1}
	gate := &fakeGate{result: LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}}
	deps := f.finalizeDeps(gh, gate)
	wdeps := WorkspaceDeps{Service: f.svc}

	first := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if first.Result == ResultApplied || first.Result == ResultNoOp {
		t.Fatalf("an unverified PR edit was reported as completion: %s/%s", first.Result, first.Reason)
	}
	if first.Reason != ReasonRecertifyEditUnknown {
		t.Fatalf("first reason = %q; want %s", first.Reason, ReasonRecertifyEditUnknown)
	}

	second := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if second.Result != ResultApplied || second.Number != 1 {
		t.Fatalf("retry = %s/%s PR %d: %s; want applied on the same PR 1", second.Result, second.Reason, second.Number, second.Message)
	}
	body := inner.pr.Body
	if evidence.Verify([]byte(body), f.head) != evidence.VerdictVerified {
		t.Fatalf("retried PR body does not verify for the current head:\n%s", body)
	}
	if !strings.Contains(body, "Authored prose.") || !strings.Contains(body, "More prose.") {
		t.Fatalf("authored PR content was not preserved across the retry:\n%s", body)
	}
}
