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

// TestIntegrationEvidenceRunVerifyMissingResultsIsUnmetCondition: an otherwise-complete implemented
// run whose change carries no linked results artifact is NOT complete — a green
// PR plus verified evidence and a tracked plan can never certify a run with no
// durable results (change 0410, criterion 1). The missing-results condition
// (results-unlinked) is enumerated on run-incomplete.
func TestIntegrationEvidenceRunVerifyMissingResultsIsUnmetCondition(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvWithEvidence(t, rvRecord(rvPlanPath, "", rvRecordedPR(), "feat/"+rvSlug), prEvidenceBytes(t, f.head)),
		rvPR(f.head, prBodyNoEvidence),
	)
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunIncomplete {
		t.Fatalf("verdict = %q, want %q (missing results must not certify complete; unmet %v)", res.Verdict, VerdictRunIncomplete, unmetReasons(res))
	}
	if got := unmetReasons(res); len(got) != 1 || got[0] != ReasonRunResultsUnlinked {
		t.Fatalf("unmet = %v, want exactly [%s]", got, ReasonRunResultsUnlinked)
	}
}

// TestIntegrationEvidenceRunVerifyInvalidResultsContentIsUnmetCondition: a linked results path that
// resolves to a tracked regular file whose FINAL content contract fails (a
// whole-section filler body) is an unmet results-content-invalid condition whose
// Observed detail names the offending path.
func TestIntegrationEvidenceRunVerifyInvalidResultsContentIsUnmetCondition(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvWithEvidence(t, rvRecord(rvPlanPath, rvResultsInvalidPath, rvRecordedPR(), "feat/"+rvSlug), prEvidenceBytes(t, f.head)),
		rvPR(f.head, prBodyNoEvidence),
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

// TestIntegrationEvidenceRunVerifyWaitingSurvivesMissingResults: the missing-results condition adds
// to unmet without suppressing a valid local waiting receipt. Waiting evaluation
// runs precisely because unmet is nonempty, so an in-progress run with a
// fully-agreeing handoff and NO results still reports run-waiting (change 0410
// preserves waiting precedence).
func TestIntegrationEvidenceRunVerifyWaitingSurvivesMissingResults(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvWithEvidence(t, rvInProgressRecord(rvPlanPath, "", "feat/"+rvSlug), prEvidenceBytes(t, f.head)),
		rvPR(f.head, prBodyNoEvidence),
	)
	wdeps.Waiting = fakeWaitingReader{receipt: rvAgreeingReceipt(f.head), found: true}
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunWaiting {
		t.Fatalf("verdict = %q, want %q (a valid waiting receipt outranks the missing-results condition; unmet %v)", res.Verdict, VerdictRunWaiting, unmetReasons(res))
	}
}

// TestIntegrationEvidenceRunVerifyAcceptsSkippedEvidenceAtExactHead: a build.gate: off repository's
// change record carries a truthful skipped (build-gate-off) block at the exact
// feature head. run verify's evidence postcondition accepts VerdictSkipped exactly
// as VerdictVerified, so the run is complete. Mirrors
// TestIntegrationChangeRuntimeRunVerifyComplete with a skipped record block substituted.
func TestIntegrationEvidenceRunVerifyAcceptsSkippedEvidenceAtExactHead(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvWithEvidence(t, rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug), prSkippedEvidenceBytes(t, f.head)),
		rvPR(f.head, prBodyNoEvidence),
	)
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunComplete {
		t.Fatalf("verdict = %q, want %q — a skipped record block at the exact head verifies (unmet %v)", res.Verdict, VerdictRunComplete, unmetReasons(res))
	}
}

// TestIntegrationEvidenceRunVerifyEvidenceReadsRecordNotPR: run verify reads the
// durable build evidence from the change record's "## Build evidence" section
// only. A green block for the exact head in the PR description, with no record
// section, is run-incomplete with exactly evidence-unverified — there is no
// compatibility read of PR-body evidence.
func TestIntegrationEvidenceRunVerifyEvidenceReadsRecordNotPR(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	deps, wdeps, gdeps := f.deps(
		rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug),
		rvPR(f.head, string(prEvidenceBytes(t, f.head))),
	)
	res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
	if res.Verdict != VerdictRunIncomplete {
		t.Fatalf("verdict = %q, want %q (PR-body evidence must not certify; unmet %v)", res.Verdict, VerdictRunIncomplete, unmetReasons(res))
	}
	if got := unmetReasons(res); len(got) != 1 || got[0] != ReasonRunEvidenceUnverified {
		t.Fatalf("unmet = %v, want exactly [%s]", got, ReasonRunEvidenceUnverified)
	}
}

// TestIntegrationEvidenceRunVerifyEvidenceIndependentOfPRCount: with no open PR
// the record's evidence is still evaluated on its own — a verified record
// section adds no evidence-unverified condition beside pr-unverified, and a
// missing one adds exactly one.
func TestIntegrationEvidenceRunVerifyEvidenceIndependentOfPRCount(t *testing.T) {
	f := newRunVerifyFixture(t, true)
	for _, tc := range []struct {
		name   string
		record []byte
		want   []string
	}{
		{"verified record", rvWithEvidence(t, rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug), prEvidenceBytes(t, f.head)), []string{ReasonRunPRUnverified}},
		{"no record evidence", rvRecord(rvPlanPath, rvResultsPath, rvRecordedPR(), "feat/"+rvSlug), []string{ReasonRunEvidenceUnverified, ReasonRunPRUnverified}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, wdeps, _ := f.deps(tc.record, rvPR(f.head, prBodyNoEvidence))
			gdeps := GitHubDeps{Service: &fakeGitHub{repo: prRepo()}}
			res := RunVerify(context.Background(), deps, wdeps, gdeps, f.repo.invocation, RunVerifyRequest{ID: 3})
			if got := unmetReasons(res); strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("unmet = %v, want %v", got, tc.want)
			}
		})
	}
}
