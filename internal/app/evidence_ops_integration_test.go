//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_evidence.sh (prefix ^TestIntegrationEvidence).

import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

// --- record: build-owned gate policy (change 0374) ---------------------------

// TestIntegrationEvidenceEvidenceRecordBuildGateOffMintsSkipped: build.gate: off is an explicit
// no-gate policy. No run is observed (RunDir empty), the head must still be the
// current feature head, and the block is the truthful skipped record.
func TestIntegrationEvidenceEvidenceRecordBuildGateOffMintsSkipped(t *testing.T) {
	res := evidenceRecordWithConfig(t, "build:\n  gate: \"off\"\n",
		EvidenceRecordRequest{ID: 7, Head: currentFeatureHead(t)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %v (%s), want applied", res.Result, res.Reason)
	}
	if res.Outcome != "skipped" || !strings.Contains(res.Block, "build-gate-off") {
		t.Errorf("outcome/block = %q/%q, want skipped/build-gate-off", res.Outcome, res.Block)
	}
	if res.Command != "" {
		t.Errorf("a skipped record carries no command, got %q", res.Command)
	}
}

// TestIntegrationEvidenceEvidenceRecordUnconfiguredBuildCommandIsTypedSetupRefusal: a local build
// gate with no build.test_command is a typed setup refusal that names the
// remedy command — never a fabricated empty command.
func TestIntegrationEvidenceEvidenceRecordUnconfiguredBuildCommandIsTypedSetupRefusal(t *testing.T) {
	res := evidenceRecordWithConfig(t, "build:\n  gate: local\n",
		EvidenceRecordRequest{ID: 7, Head: currentFeatureHead(t), RunDir: testsupport.TempDir(t)})
	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result/reason = %v/%s, want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if !strings.Contains(res.Message, "docket repository configure-tests") {
		t.Errorf("message %q must name the setup remedy", res.Message)
	}
}

// TestIntegrationEvidenceEvidenceRecordRecordsBuildCommandNotFinalize: the divergent-command
// fixture. EvidenceRecord is build-owned, so the recorded command is
// build.test_command — swapping the source to finalize.test_command reddens
// this test (the guard the divergent fixture exists for).
func TestIntegrationEvidenceEvidenceRecordRecordsBuildCommandNotFinalize(t *testing.T) {
	res := evidenceRecordPassedRun(t, "build:\n  gate: local\n  test_command: go test ./build-only\nfinalize:\n  test_command: make finalize-only\n")
	if res.Result != ResultApplied {
		t.Fatalf("result = %v (%s: %s), want applied", res.Result, res.Reason, res.Message)
	}
	if res.Command != "go test ./build-only" {
		t.Errorf("recorded command = %q; evidence must record build.test_command", res.Command)
	}
}

// --- record: per-request owner (change 0517) ---------------------------------

// Mixed-owner fixtures: build and finalize ALWAYS carry different commands.
// With identical commands the build-owned and finalize-owned paths produce the
// same record, so only a mixed fixture can tell them apart (learning
// shared-resource-keeps-first-owner-assumptions).
const (
	ownerBuildCmd        = "build-cmd"
	ownerFinalizeCmd     = "finalize-cmd"
	ownerMixedLocal      = "build:\n  gate: local\n  test_command: build-cmd\nfinalize:\n  gate: local\n  test_command: finalize-cmd\n"
	ownerBuildOff        = "build:\n  gate: \"off\"\n  test_command: build-cmd\nfinalize:\n  gate: local\n  test_command: finalize-cmd\n"
	ownerBuildEmpty      = "build:\n  gate: local\nfinalize:\n  gate: local\n  test_command: finalize-cmd\n"
	ownerFinalizeEmpty   = "build:\n  gate: local\n  test_command: build-cmd\nfinalize:\n  gate: local\n"
	ownerFinalizeGateOff = "build:\n  gate: local\n  test_command: build-cmd\nfinalize:\n  gate: \"off\"\n  test_command: finalize-cmd\n"
)

// evidenceRecordPassedRunAs runs EvidenceRecord over a real passed gate run dir
// at the current feature head, under the given YAML overlay and owner.
func evidenceRecordPassedRunAs(t *testing.T, yaml, owner string) EvidenceOpResult {
	t.Helper()
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), yaml)
	return EvidenceRecord(context.Background(), deps, wdeps, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: passedRunDir(t), Head: evidenceHead, Owner: owner})
}

// assertGreenCommand fails unless res is an applied green record naming want,
// both in the result field and inside the rendered canonical block.
func assertGreenCommand(t *testing.T, res EvidenceOpResult, want string) {
	t.Helper()
	if res.Result != ResultApplied {
		t.Fatalf("result = %v (%s: %s), want applied", res.Result, res.Reason, res.Message)
	}
	if res.Outcome != string(evidence.ResultGreen) || res.Command != want {
		t.Fatalf("outcome/command = %q/%q, want green/%q", res.Outcome, res.Command, want)
	}
	rec, err := evidence.Extract([]byte(res.Block))
	if err != nil {
		t.Fatalf("block does not parse: %v\n%s", err, res.Block)
	}
	if rec.Result != evidence.ResultGreen || rec.Command != want {
		t.Fatalf("block record = %s/%q, want green/%q", rec.Result, rec.Command, want)
	}
}

// TestIntegrationEvidenceRecordFinalizeOwnerCertifiesFinalizeCommand: owner
// finalize records finalize.test_command whatever the build settings say —
// (a) build.gate off is NOT skipped, (b) a divergent build command is not
// recorded, (c) an empty build.test_command is not refused — and finalize.gate
// is not consulted.
func TestIntegrationEvidenceRecordFinalizeOwnerCertifiesFinalizeCommand(t *testing.T) {
	for _, tc := range []struct{ name, yaml string }{
		{"build-gate-off", ownerBuildOff},
		{"both-local", ownerMixedLocal},
		{"build-command-empty", ownerBuildEmpty},
		{"finalize-gate-off-not-consulted", ownerFinalizeGateOff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertGreenCommand(t, evidenceRecordPassedRunAs(t, tc.yaml, EvidenceOwnerFinalize), ownerFinalizeCmd)
		})
	}
}

// TestIntegrationEvidenceRecordFinalizeOwnerUnconfiguredCommandRefused: an
// empty finalize.test_command is a typed setup refusal naming the FINALIZE key.
func TestIntegrationEvidenceRecordFinalizeOwnerUnconfiguredCommandRefused(t *testing.T) {
	res := evidenceRecordWithConfig(t, ownerFinalizeEmpty, EvidenceRecordRequest{
		ID: 7, Head: currentFeatureHead(t), RunDir: testsupport.TempDir(t), Owner: EvidenceOwnerFinalize})
	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result/reason = %v/%s, want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if !strings.Contains(res.Message, "finalize.test_command") || strings.Contains(res.Message, "build.") {
		t.Errorf("message %q must name finalize.test_command and no build.* key", res.Message)
	}
	if !strings.Contains(res.Message, "docket repository configure-tests") {
		t.Errorf("message %q must name the setup remedy", res.Message)
	}
}

// TestIntegrationEvidenceRecordFinalizeOwnerMissingRunDirNamesNoBuildGate:
// owner finalize with no --run is missing-run-dir, and the message does not
// claim a build gate setting (Review Focus 2).
func TestIntegrationEvidenceRecordFinalizeOwnerMissingRunDirNamesNoBuildGate(t *testing.T) {
	res := evidenceRecordWithConfig(t, ownerBuildOff, EvidenceRecordRequest{
		ID: 7, Head: currentFeatureHead(t), Owner: EvidenceOwnerFinalize})
	if res.Result != ResultInvalidInput || res.Reason != ReasonEvidenceMissingRunDir {
		t.Fatalf("result/reason = %v/%s, want invalid-input/%s", res.Result, res.Reason, ReasonEvidenceMissingRunDir)
	}
	if strings.Contains(res.Message, "build.") {
		t.Errorf("finalize-owned message %q must not name a build.* key", res.Message)
	}
}

// TestIntegrationEvidenceRecordFinalizeOwnerHeadMismatchRefused: the new
// finalize branch still goes through verifyFeatureHead (Review Focus 3).
func TestIntegrationEvidenceRecordFinalizeOwnerHeadMismatchRefused(t *testing.T) {
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), ownerMixedLocal)
	res := EvidenceRecord(context.Background(), deps, wdeps, repoDir, EvidenceRecordRequest{
		ID: 7, RunDir: passedRunDir(t), Head: evidenceOtherHead, Owner: EvidenceOwnerFinalize})
	if res.Reason != ReasonEvidenceHeadMismatch {
		t.Fatalf("reason = %s (%v), want %s", res.Reason, res.Result, ReasonEvidenceHeadMismatch)
	}
}

// TestIntegrationEvidenceRecordBuildOwnerAndOmittedStayBuild: owner build and
// an omitted owner behave exactly as before 0517 on the mixed fixture — green
// naming build-cmd under a local build gate, skipped under build.gate: off.
func TestIntegrationEvidenceRecordBuildOwnerAndOmittedStayBuild(t *testing.T) {
	for _, tc := range []struct{ name, owner string }{
		{"omitted", ""},
		{"build", EvidenceOwnerBuild},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertGreenCommand(t, evidenceRecordPassedRunAs(t, ownerMixedLocal, tc.owner), ownerBuildCmd)

			off := evidenceRecordWithConfig(t, ownerBuildOff,
				EvidenceRecordRequest{ID: 7, Head: currentFeatureHead(t), Owner: tc.owner})
			if off.Result != ResultApplied || off.Outcome != string(evidence.ResultSkipped) {
				t.Fatalf("gate-off result/outcome = %v/%q (%s), want applied/skipped", off.Result, off.Outcome, off.Reason)
			}
			if off.Command != "" || !strings.Contains(off.Block, evidence.ReasonBuildGateOff) {
				t.Errorf("gate-off command/block = %q/%q, want empty/%s", off.Command, off.Block, evidence.ReasonBuildGateOff)
			}
		})
	}
}

// TestIntegrationEvidenceFinalizeGatePassCertifiesWithSeamOwner (change 0517):
// a PASSED drive in the finalize.rebase built-in gate certifies with the
// seam's own owner. The finalize seam — and the zero-value seam, which the
// seam treats as finalize — records finalize-cmd, green even under
// build.gate: off; the build-owned recertify seam still records build-cmd.
func TestIntegrationEvidenceFinalizeGatePassCertifiesWithSeamOwner(t *testing.T) {
	for _, tc := range []struct{ name, owner, yaml, want string }{
		{"finalize-seam", gateOwnerFinalize, ownerMixedLocal, ownerFinalizeCmd},
		{"zero-value-seam-is-finalize", "", ownerMixedLocal, ownerFinalizeCmd},
		{"finalize-seam-build-gate-off", gateOwnerFinalize, ownerBuildOff, ownerFinalizeCmd},
		{"recertify-build-seam", gateOwnerBuild, ownerMixedLocal, ownerBuildCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), tc.yaml)
			g := &processFinalizeGate{planning: deps, wdeps: wdeps, owner: tc.owner}
			removeRoot := true
			doc := &gatedrive.DriveDoc{Outcome: gatedrive.PASSED, RawRunDir: passedRunDir(t)}
			res := g.mapTerminalDrive(context.Background(),
				LocalGateRequest{RepoDir: repoDir, ID: 7, Head: evidenceHead}, doc, &removeRoot)
			if res.Outcome != FinalizeGatePassed {
				t.Fatalf("outcome = %s (halt %s), want passed", res.Outcome, res.HaltCause)
			}
			rec, err := evidence.Extract([]byte(res.Evidence))
			if err != nil {
				t.Fatalf("evidence block does not parse: %v\n%s", err, res.Evidence)
			}
			if rec.Result != evidence.ResultGreen || rec.Command != tc.want {
				t.Fatalf("evidence = %s/%q, want green/%q", rec.Result, rec.Command, tc.want)
			}
		})
	}
}
