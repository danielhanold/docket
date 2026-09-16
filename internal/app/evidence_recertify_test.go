package app

import (
	"context"
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
