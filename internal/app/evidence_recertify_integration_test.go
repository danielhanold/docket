//go:build integration

package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// buildVsFinalizeYAML declares DIVERGENT build/finalize commands so a test can
// prove which owner's command a gate resolved (acceptance: differing commands
// prove only the BUILD command runs).
const buildVsFinalizeYAML = "build:\n  gate: local\n  test_command: go test ./build-only\nfinalize:\n  test_command: make finalize-only\n"

// TestIntegrationEvidenceBuildLocalGateResolvesBuildCommandOnly: the BUILD-owned production gate
// resolves build.test_command; the finalize twin resolves finalize.test_command
// from the same pin. Deleting the owner branch in buildDriveService reddens one
// of the two arms.
func TestIntegrationEvidenceBuildLocalGateResolvesBuildCommandOnly(t *testing.T) {
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

// TestIntegrationEvidenceBuildLocalGateFailsClosedWithoutBuildCommand: a config with ONLY
// finalize.test_command set fails the build-owned gate closed (ok=false → the
// caller halts, never a fabricated red) while the finalize twin still resolves.
// This pins the guard's keying on the owner's OWN config key.
func TestIntegrationEvidenceBuildLocalGateFailsClosedWithoutBuildCommand(t *testing.T) {
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

// requireRecordUnchanged fails when a refused recertify wrote the change record
// — the only durable write recertify makes, since it never edits the PR.
func requireRecordUnchanged(t *testing.T, f *rebaseFixture, before string) {
	t.Helper()
	if got := f.remoteRecordBytes(t); got != before {
		t.Fatalf("a refused recertify wrote the change record:\n%s", got)
	}
}

// TestIntegrationEvidenceEvidenceRecertifyRefusesNotImplemented: any non-implemented status is
// blocked before any probe of the gate or record write (acceptance 3).
func TestIntegrationEvidenceEvidenceRecertifyRefusesNotImplemented(t *testing.T) {
	f := setupRebaseFixtureStatus(t, planRepoModes()[0], "in-progress")
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	before := f.remoteRecordBytes(t)
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, &fakeGate{}), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyNotImplemented {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyNotImplemented)
	}
	requireRecordUnchanged(t, f, before)
}

// TestIntegrationEvidenceEvidenceRecertifyRefusesDirtyWorkspace: uncommitted work blocks (never
// gated over, never recorded) — acceptance 3.
func TestIntegrationEvidenceEvidenceRecertifyRefusesDirtyWorkspace(t *testing.T) {
	f, _, deps, wdeps := recertifyFixture(t, &fakeGate{})
	before := f.remoteRecordBytes(t)
	writeRepoFile(t, f.wp, "dirty.txt", "uncommitted\n")
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyWorkspaceDirty {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyWorkspaceDirty)
	}
	requireRecordUnchanged(t, f, before)
}

// TestIntegrationEvidenceEvidenceRecertifyRefusesUnpublishedFollowUp: a local follow-up commit
// that was never pushed disagrees with the remote feature head; the operation
// refuses (publish first through the existing workflow) rather than certify a
// head the PR does not hold.
func TestIntegrationEvidenceEvidenceRecertifyRefusesUnpublishedFollowUp(t *testing.T) {
	f, _, deps, wdeps := recertifyFixture(t, &fakeGate{})
	before := f.remoteRecordBytes(t)
	writeRepoFile(t, f.wp, "followup.txt", "review fix\n")
	runGit(t, f.wp, "add", "-A")
	runGit(t, f.wp, "commit", "-q", "-m", "review fix")
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result == ResultApplied || res.Reason != ReasonRecertifyHeadDisagreement {
		t.Fatalf("result = %s/%s; want a %s refusal", res.Result, res.Reason, ReasonRecertifyHeadDisagreement)
	}
	requireRecordUnchanged(t, f, before)
}

// TestIntegrationEvidenceEvidenceRecertifyRefusesClosedOrMismatchedPR: no open PR for the feature
// head refuses (pr-not-open); an open PR naming a different head refuses
// (head-disagreement). Neither runs the gate — acceptance 3.
func TestIntegrationEvidenceEvidenceRecertifyRefusesClosedOrMismatchedPR(t *testing.T) {
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

// TestIntegrationEvidenceEvidenceRecertifyShape: a non-positive id is an invalid-input shape
// refusal before any probe.
func TestIntegrationEvidenceEvidenceRecertifyShape(t *testing.T) {
	f, _, deps, wdeps := recertifyFixture(t, &fakeGate{})
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: 0})
	if res.Result != ResultInvalidInput {
		t.Fatalf("result = %s; want %s", res.Result, ResultInvalidInput)
	}
}

// TestIntegrationEvidenceEvidenceRecertifyHappyPath: stale evidence at an older head becomes
// verified evidence for the exact current head in the change record's
// build-evidence section; the open PR's description is never edited; the
// result is applied/green on the SAME open PR (acceptance 1).
func TestIntegrationEvidenceEvidenceRecertifyHappyPath(t *testing.T) {
	gate := &fakeGate{}
	f, gh, deps, wdeps := recertifyFixture(t, gate)
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}
	prBody := gh.pr.Body

	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultApplied || res.Outcome != RecertifyOutcomeGreen {
		t.Fatalf("result = %s/%s outcome %q (reason %q: %s); want applied green", res.Result, res.Reason, res.Outcome, res.Reason, res.Message)
	}
	if res.Head != f.head || res.Number != 1 || !strings.HasSuffix(res.Reference, "#1") {
		t.Fatalf("result names head %q PR %d (%q); want %q PR 1", res.Head, res.Number, res.Reference, f.head)
	}
	if gh.ensNext != 0 || gh.pr.Body != prBody {
		t.Fatalf("recertify edited the PR description (%d EnsurePullRequest calls)", gh.ensNext)
	}
	// The recertified record is durable in the change record's evidence section.
	want, err := evidence.Extract([]byte(greenBlockFor(t, f.head)))
	if err != nil {
		t.Fatalf("evidence.Extract: %v", err)
	}
	if got := f.remoteRecordEvidence(t); got != want {
		t.Fatalf("recorded evidence = %+v, want the recertified record %+v", got, want)
	}
}

// TestIntegrationEvidenceEvidenceRecertifyAdvancesOneDriveAcrossWaiting: WAITING is nonterminal —
// the operation re-enters the SAME drive (continuation threaded) until a
// terminal, and only then publishes. Deleting the loop's continuation
// threading reddens the continuation asserts.
func TestIntegrationEvidenceEvidenceRecertifyAdvancesOneDriveAcrossWaiting(t *testing.T) {
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

// TestIntegrationEvidenceEvidenceRecertifyGateFailureAndHalt: a red suite is gate-failed (repair
// work) and a halt is blocked — neither writes the record (acceptance 3), and a
// WAITING with no continuation fails closed instead of spinning.
func TestIntegrationEvidenceEvidenceRecertifyGateFailureAndHalt(t *testing.T) {
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
			f, _, deps, wdeps := recertifyFixture(t, gate)
			before := f.remoteRecordBytes(t)
			res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
			if res.Result != tc.want || res.Reason != tc.reason {
				t.Fatalf("result = %s/%s; want %s/%s", res.Result, res.Reason, tc.want, tc.reason)
			}
			requireRecordUnchanged(t, f, before)
		})
	}
}

// TestIntegrationEvidenceEvidenceRecertifyGateOffRecordsSkipped: build.gate off mints truthful
// skipped evidence, records it in the change record, and completes as skipped
// WITHOUT editing the PR (acceptance 2).
func TestIntegrationEvidenceEvidenceRecertifyGateOffRecordsSkipped(t *testing.T) {
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
	// The skipped record's durable home is the change record's evidence section.
	got := f.remoteRecordEvidence(t)
	if got.Result != evidence.ResultSkipped || got.Reason != evidence.ReasonBuildGateOff || got.Head != f.head {
		t.Fatalf("recorded evidence = %+v, want skipped/%s at %s", got, evidence.ReasonBuildGateOff, f.head)
	}
}

// TestIntegrationEvidenceEvidenceRecertifyRefusesUnconfiguredGate: a local build gate with no
// build.test_command refuses; no suite, no record write (acceptance 2).
func TestIntegrationEvidenceEvidenceRecertifyRefusesUnconfiguredGate(t *testing.T) {
	gate := &fakeGate{}
	f, _, deps, wdeps := recertifyFixture(t, gate)
	// Config resolves from the pinned default-branch tip (origin/main), never the
	// invocation working tree, so the build-command-absent policy (gate defaults
	// local) must be published to origin/main.
	f.repo.writerAdvance(t, "main", map[string]string{
		".docket.yml": "integration_branch: main\nfinalize:\n  test_command: 'go test ./...'\n",
	})
	before := f.remoteRecordBytes(t)

	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result = %s/%s; want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if gate.calls != 0 {
		t.Fatalf("an unconfigured gate ran the suite")
	}
	requireRecordUnchanged(t, f, before)
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

// TestIntegrationEvidenceEvidenceRecertifyRefusesHeadMovedUnderGate: a HEAD that moved between
// the gate and the publish can never publish (acceptance 3). The recheck's
// local-vs-remote leg catches it (the late commit is unpublished).
func TestIntegrationEvidenceEvidenceRecertifyRefusesHeadMovedUnderGate(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gate := &movingGate{f: f, ev: greenBlockFor(t, f.head)}
	before := f.remoteRecordBytes(t)
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, gate), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result == ResultApplied || res.Result == ResultNoOp {
		t.Fatalf("a moved head published evidence: %s/%s", res.Result, res.Reason)
	}
	if res.Reason != ReasonRecertifyHeadDisagreement && res.Reason != ReasonRecertifyCertifiedInputChanged {
		t.Fatalf("reason = %q; want a head-disagreement/certified-input-changed refusal", res.Reason)
	}
	requireRecordUnchanged(t, f, before)
}

// TestIntegrationEvidenceEvidenceRecertifyRefusesForeignCommandEvidence: evidence recording a
// command other than the currently resolved build.test_command cannot publish —
// the changed-configuration face of acceptance 3.
func TestIntegrationEvidenceEvidenceRecertifyRefusesForeignCommandEvidence(t *testing.T) {
	gate := &fakeGate{}
	f, _, deps, wdeps := recertifyFixture(t, gate)
	foreign, err := evidence.NewRecord("make other-suite", f.head, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("evidence.NewRecord: %v", err)
	}
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: evidence.Render(foreign), RunDir: "/run/x"}
	before := f.remoteRecordBytes(t)
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != "certified-input-changed" {
		t.Fatalf("result = %s/%s; want blocked/certified-input-changed", res.Result, res.Reason)
	}
	requireRecordUnchanged(t, f, before)
}

// TestIntegrationEvidenceEvidenceRecertifyRefusesWrongHeadEvidence: gate evidence naming another
// head is stale at verification and never recorded.
func TestIntegrationEvidenceEvidenceRecertifyRefusesWrongHeadEvidence(t *testing.T) {
	gate := &fakeGate{}
	f, _, deps, wdeps := recertifyFixture(t, gate)
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.baseTip), RunDir: "/run/x"}
	before := f.remoteRecordBytes(t)
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultInvalidState || res.Reason != ReasonRecertifyEvidenceUnverified {
		t.Fatalf("result = %s/%s; want invalid-state/%s", res.Result, res.Reason, ReasonRecertifyEvidenceUnverified)
	}
	requireRecordUnchanged(t, f, before)
}

// TestIntegrationEvidenceEvidenceRecertifyRecordContended: a PASSED gate whose
// record write races a concurrent writer that moved the change record between
// recertify's read and its exact-revision transaction is refused as
// contended/record-evidence-contended, reports NO completion, and leaves the
// concurrent writer's record untouched.
func TestIntegrationEvidenceEvidenceRecertifyRecordContended(t *testing.T) {
	gate := &fakeGate{}
	f, _, deps, wdeps := recertifyFixture(t, gate)
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}
	var advanced string
	deps.Planning.Engine = &recordWriteInterposer{inner: f.deps.Engine, before: func() {
		advanced = f.remoteRecordBytes(t) + "\nA concurrent writer's prose.\n"
		f.repo.writerAdvance(t, f.branch, map[string]string{groomPath(f.id, f.slug): advanced})
	}}
	res := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultContended || res.Reason != ReasonRecertifyRecordContended {
		t.Fatalf("result = %s/%s (%s); want %s/%s", res.Result, res.Reason, res.Message, ResultContended, ReasonRecertifyRecordContended)
	}
	if res.Outcome != "" {
		t.Fatalf("a contended record write reported completion: outcome %q", res.Outcome)
	}
	if advanced == "" {
		t.Fatalf("the record write was never attempted")
	}
	if got := f.remoteRecordBytes(t); got != advanced {
		t.Fatalf("the contended recertify touched the concurrent writer's record:\n%s", got)
	}
}

// failingRecordWriteEngine fails the first N record-evidence transactions
// without touching the metadata branch and delegates every other call to the
// real engine.
type failingRecordWriteEngine struct {
	inner interface {
		Execute(ctx context.Context, req transaction.Request) (transaction.Result, error)
	}
	failures int
}

func (e *failingRecordWriteEngine) Execute(ctx context.Context, req transaction.Request) (transaction.Result, error) {
	if _, ok := req.Operation.(recordEvidenceOp); ok && e.failures > 0 {
		e.failures--
		return transaction.Result{Disposition: transaction.DispositionFailed}, errors.New("record write boom")
	}
	return e.inner.Execute(ctx, req)
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

// TestIntegrationEvidenceEvidenceRecertifyRefusesDirtyAfterGate: an untracked, non-ignored file the
// build command leaves in the worktree during a PASSED gate flips the
// pre-publish cleanliness recheck to workspace-dirty and refuses to publish; the
// record is never written. Pins the whole-predicate recheck's clean-worktree leg
// (documented as the clean-worktree caveat in the guide).
func TestIntegrationEvidenceEvidenceRecertifyRefusesDirtyAfterGate(t *testing.T) {
	f := setupRebaseFixture(t, planRepoModes()[0])
	gh := &fakePublishGitHub{repo: retargetRepo(), pr: f.prForHead(f.head, greenEvidenceFor(t, f.baseTip))}
	gate := &dirtyingGate{f: f, ev: greenBlockFor(t, f.head)}
	before := f.remoteRecordBytes(t)
	res := EvidenceRecertify(context.Background(), f.finalizeDeps(gh, gate), WorkspaceDeps{Service: f.svc},
		f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonRecertifyWorkspaceDirty {
		t.Fatalf("result = %s/%s; want blocked/%s", res.Result, res.Reason, ReasonRecertifyWorkspaceDirty)
	}
	requireRecordUnchanged(t, f, before)
}

// TestIntegrationEvidenceEvidenceRecertifyRecordFailureThenRetry: a record write
// that does not apply is NOT completion; a later invocation converges the
// change record's build-evidence section for the same head on the same PR
// (acceptance 4).
func TestIntegrationEvidenceEvidenceRecertifyRecordFailureThenRetry(t *testing.T) {
	gate := &fakeGate{}
	f, _, deps, wdeps := recertifyFixture(t, gate)
	gate.result = LocalGateResult{Outcome: FinalizeGatePassed, Evidence: greenBlockFor(t, f.head), RunDir: "/run/x"}
	deps.Planning.Engine = &failingRecordWriteEngine{inner: f.deps.Engine, failures: 1}
	before := f.remoteRecordBytes(t)

	first := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if first.Result == ResultApplied || first.Result == ResultNoOp {
		t.Fatalf("a failed record write was reported as completion: %s/%s", first.Result, first.Reason)
	}
	if first.Reason != ReasonRecertifyRecordFailed {
		t.Fatalf("first reason = %q; want %s", first.Reason, ReasonRecertifyRecordFailed)
	}
	if got := f.remoteRecordBytes(t); got != before {
		t.Fatalf("a failed record write changed the record:\n%s", got)
	}

	second := EvidenceRecertify(context.Background(), deps, wdeps, f.repo.invocation, EvidenceRecertifyRequest{ID: f.id})
	if second.Result != ResultApplied || second.Number != 1 {
		t.Fatalf("retry = %s/%s PR %d: %s; want applied on the same PR 1", second.Result, second.Reason, second.Number, second.Message)
	}
	want, err := evidence.Extract([]byte(greenBlockFor(t, f.head)))
	if err != nil {
		t.Fatalf("evidence.Extract: %v", err)
	}
	if got := f.remoteRecordEvidence(t); got != want {
		t.Fatalf("recorded evidence after retry = %+v, want %+v", got, want)
	}
}
