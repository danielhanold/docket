//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_evidence.sh (prefix ^TestIntegrationEvidence).

import (
	"strings"
	"testing"

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
