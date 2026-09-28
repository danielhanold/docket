//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_evidence.sh (prefix ^TestIntegrationEvidence).

import (
	"context"
	"strings"
	"testing"
)

// TestIntegrationEvidenceRunVerifyMissingResultsIsUnmetConjunct: an otherwise-complete implemented
// run whose change carries no linked results artifact is NOT complete — a green
// PR plus verified evidence and a tracked plan can never certify a run with no
// durable results (change 0410, criterion 1). The missing-results conjunct
// (results-unlinked) is enumerated on run-incomplete.
func TestIntegrationEvidenceRunVerifyMissingResultsIsUnmetConjunct(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, "", rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunIncomplete {
		t.Fatalf("verdict = %q, want %q (missing results must not certify complete; unmet %v)", res.Verdict, VerdictRunIncomplete, unmetReasons(res))
	}
	if got := unmetReasons(res); len(got) != 1 || got[0] != ReasonRunResultsUnlinked {
		t.Fatalf("unmet = %v, want exactly [%s]", got, ReasonRunResultsUnlinked)
	}
}

// TestIntegrationEvidenceRunVerifyInvalidResultsContentIsUnmetConjunct: a linked results path that
// resolves to a tracked regular file whose FINAL content contract fails (a
// whole-section filler body) is an unmet results-content-invalid conjunct whose
// Observed detail names the offending path.
func TestIntegrationEvidenceRunVerifyInvalidResultsContentIsUnmetConjunct(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsInvalidPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunIncomplete {
		t.Fatalf("verdict = %q, want %q (unmet %v)", res.Verdict, VerdictRunIncomplete, unmetReasons(res))
	}
	if got := unmetReasons(res); len(got) != 1 || got[0] != ReasonRunResultsInvalid {
		t.Fatalf("unmet = %v, want exactly [%s]", got, ReasonRunResultsInvalid)
	}
	var observed string
	for _, u := range res.Unmet {
		if u.Reason == ReasonRunResultsInvalid {
			observed = u.Observed
		}
	}
	if !strings.HasPrefix(observed, rvResultsInvalidPath) {
		t.Fatalf("observed = %q, want it to name path %q", observed, rvResultsInvalidPath)
	}
}

// TestIntegrationEvidenceRunVerifyWaitingSurvivesMissingResults: the missing-results conjunct adds
// to unmet without suppressing a valid local waiting receipt. Waiting evaluation
// runs precisely because unmet is nonempty, so an in-progress run with a
// fully-agreeing handoff and NO results still reports run-waiting (change 0410
// preserves waiting precedence).
func TestIntegrationEvidenceRunVerifyWaitingSurvivesMissingResults(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvInProgressRecord(rvPlanPath, "", "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	wdeps.Waiting = fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true}
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunWaiting {
		t.Fatalf("verdict = %q, want %q (a valid waiting receipt outranks the missing-results conjunct; unmet %v)", res.Verdict, VerdictRunWaiting, unmetReasons(res))
	}
}

// TestIntegrationEvidenceRunVerifyAcceptsSkippedEvidenceAtExactHead: a build.gate: off repository's
// PR carries a truthful skipped (build-gate-off) block at the exact feature head.
// run verify's evidence postcondition accepts VerdictSkipped exactly as
// VerdictVerified, so the run is complete. Mirrors TestIntegrationChangeRuntimeRunVerifyComplete
// with a skipped PR-body block substituted.
func TestIntegrationEvidenceRunVerifyAcceptsSkippedEvidenceAtExactHead(t *testing.T) {
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
