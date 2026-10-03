//go:build integration

package app

import (
	"context"
	"testing"
)

// TestIntegrationRunCompletionKilledPublisherIsAbandonedAtCloseout (spec test 2).
func TestIntegrationRunCompletionKilledPublisherIsAbandonedAtCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	desc := lockTestPRDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("closeout blocked: reason=%q findings=%v", reason, findings)
	}
	if countFinding(findings, "mutation-abandoned:"+OperationPRPublish) != 1 || hasFinding(findings, "mutation-pending") {
		t.Fatalf("findings = %v, want exactly one mutation-abandoned:pr.publish (deduplicated) and no mutation-pending", findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("state = %q, want completed", st)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusUncertain {
		t.Fatalf("keyed closeout left the entry %q, want uncertain (journal kept truthful)", m.Status)
	}
}

// TestIntegrationRunCompletionVerdictRunCompleteReportsAbandonedPublication (spec test 2,
// through the real keyed verdict).
func TestIntegrationRunCompletionVerdictRunCompleteReportsAbandonedPublication(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	desc := lockTestPRDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})
	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (findings %v)", got, want, res.CompletionFindings)
	}
	if countFinding(res.CompletionFindings, "mutation-abandoned:"+OperationPRPublish) != 1 {
		t.Fatalf("CompletionFindings = %v, want mutation-abandoned:pr.publish", res.CompletionFindings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil || ep.State != RunCompleted {
		t.Fatalf("run state = %q (err %v), want completed", ep.State, err)
	}
}

// TestIntegrationRunCompletionLivePublisherBlocksCloseout (spec test 3, closeout half).
func TestIntegrationRunCompletionLivePublisherBlocksCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	desc := lockTestPRDesc()
	token, release := seedPublishLock(t, fx.repo, fx.key, true)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if ok || reason != "completion-unaccounted" || !hasFinding(findings, "mutation-pending:"+OperationPRPublish) {
		t.Fatalf("live publisher: ok=%v reason=%q findings=%v, want completion-unaccounted with mutation-pending", ok, reason, findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleting {
		t.Fatalf("state = %q, want completing (the success fence holds)", st)
	}
	release()
	ok, reason, findings = completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok || !hasFinding(findings, "mutation-abandoned:"+OperationPRPublish) {
		t.Fatalf("after release: ok=%v reason=%q findings=%v, want completed with mutation-abandoned", ok, reason, findings)
	}
}

// TestIntegrationRunCompletionUnprovableEntryBlocksCloseout (spec test 4, closeout).
func TestIntegrationRunCompletionUnprovableEntryBlocksCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	tok, err := runToken() // a token with no lock file
	if err != nil {
		t.Fatal(err)
	}
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, LockToken: tok})
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if ok || reason != "completion-unaccounted" || !hasFinding(findings, "mutation-pending:"+OperationPRPublish) || hasFinding(findings, "mutation-abandoned") {
		t.Fatalf("missing lock file: ok=%v reason=%q findings=%v, want blocked mutation-pending only", ok, reason, findings)
	}
}

// TestIntegrationRunCompletionKilledPublisherThenVerifiedRetrySettlesAtCloseout (spec test 5).
func TestIntegrationRunCompletionKilledPublisherThenVerifiedRetrySettlesAtCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	desc := lockTestWSDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key,
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token},
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc})
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("closeout blocked: reason=%q findings=%v", reason, findings)
	}
	if countFinding(findings, "mutation-settled:"+OperationWorkspacePublish) != 1 || hasFinding(findings, "mutation-abandoned") {
		t.Fatalf("findings = %v, want mutation-settled and no mutation-abandoned", findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusCompleted {
		t.Fatalf("original = %q, want completed", m.Status)
	}
}
