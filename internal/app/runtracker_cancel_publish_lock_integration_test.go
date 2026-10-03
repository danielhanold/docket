//go:build integration

package app

import (
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/process"
)

func lockCancelSeams(fx cancelFixture) cancelSeams {
	return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}, launches: okLaunchReconciler()}
}

// TestIntegrationRunCancelKilledPublisherIsAbandoned (spec test 1, Review Focus 5).
func TestIntegrationRunCancelKilledPublisherIsAbandoned(t *testing.T) {
	fx := newCancelFixture(t)
	desc := lockTestWSDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})

	res := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q (findings %v), want cancelled", res.Disposition, res.Findings)
	}
	if countFinding(res.Findings, "mutation-abandoned:"+OperationWorkspacePublish) != 1 || hasFinding(res.Findings, "mutation-pending") {
		t.Fatalf("findings = %v, want exactly one mutation-abandoned and no mutation-pending", res.Findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusUncertain || m.Verified || m.LockToken != token {
		t.Fatalf("stored entry = %+v, want uncertain, unverified, same token", m)
	}
	if _, err := os.Stat(publishLockPathFor(t, fx.repo, fx.key, token)); err != nil {
		t.Fatalf("lock file must never be deleted: %v", err)
	}
	// Repeat: terminal, quiescent, and no second rewrite.
	again := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if again.Disposition != CancelDispositionAlreadyCancelled || countFinding(again.Findings, "mutation-abandoned:"+OperationWorkspacePublish) != 1 {
		t.Fatalf("repeat = %q %v, want already-cancelled with the informational finding", again.Disposition, again.Findings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatal(err)
	}
	if ok, detail := validateResumeQuiescence(cancelSeams{store: fx.store, launches: okLaunchReconciler()}, fx.repo, ep); !ok {
		t.Fatalf("resume quiescence = %q, want quiescent", detail)
	}
	if err := SupersedeCancelledRun(fx.repo, fx.key, "replacement-key"); err != nil {
		t.Fatalf("SupersedeCancelledRun: %v (resume must admit exactly one replacement)", err)
	}
}

// TestIntegrationRunCancelLivePublisherStaysPending (spec test 3, cancel half).
func TestIntegrationRunCancelLivePublisherStaysPending(t *testing.T) {
	fx := newCancelFixture(t)
	desc := lockTestWSDesc()
	token, release := seedPublishLock(t, fx.repo, fx.key, true)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})

	first := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if first.Disposition != CancelDispositionPending || !hasFinding(first.Findings, "mutation-pending:"+OperationWorkspacePublish) ||
		hasFinding(first.Findings, "mutation-abandoned") {
		t.Fatalf("live publisher: %q %v, want cancellation-pending with mutation-pending only", first.Disposition, first.Findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusAdmitted {
		t.Fatalf("settlement touched a live publisher's entry: %q", m.Status)
	}
	release()
	second := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if second.Disposition != CancelDispositionCancelled || !hasFinding(second.Findings, "mutation-abandoned:"+OperationWorkspacePublish) {
		t.Fatalf("after release: %q %v, want cancelled with mutation-abandoned", second.Disposition, second.Findings)
	}
}

// TestIntegrationRunCancelUnprovableJournalEntriesBlock (spec test 4, Review Focus 1–2):
// every entry docket cannot prove abandoned blocks exactly as before change 0494.
func TestIntegrationRunCancelUnprovableJournalEntriesBlock(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T, fx cancelFixture) AdmittedMutation
	}{
		{"no lock token", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted}
		}},
		{"lock file missing", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			tok, err := runToken()
			if err != nil {
				t.Fatal(err)
			}
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: tok}
		}},
		{"probe error", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			tok, _ := seedPublishLock(t, fx.repo, fx.key, false)
			overrideSeam(t, &publishLockProbe, func(string) process.LockProbe { return process.LockProbeUnknown })
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: tok}
		}},
		{"lock path is a directory", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			tok, err := runToken()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(publishLockPathFor(t, fx.repo, fx.key, tok), 0o700); err != nil {
				t.Fatal(err)
			}
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: tok}
		}},
		{"malformed token", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: "../../../../../../../../tmp/xx"}
		}},
		{"unknown status with a free lock", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			tok, _ := seedPublishLock(t, fx.repo, fx.key, false)
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: "bogus", LockToken: tok}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t)
			m := tc.seed(t, fx)
			seedJournal(t, fx.repo, fx.key, m)
			res := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
			if res.Disposition != CancelDispositionPending || !hasFinding(res.Findings, "mutation-pending:"+OperationWorkspacePublish) ||
				hasFinding(res.Findings, "mutation-abandoned") {
				t.Fatalf("%q %v, want cancellation-pending with mutation-pending only", res.Disposition, res.Findings)
			}
			if got := journalEntry(t, fx.repo, fx.key, 0); got.Status != m.Status {
				t.Fatalf("entry status = %q, want untouched %q", got.Status, m.Status)
			}
		})
	}
}

// TestIntegrationRunCancelKilledPublisherThenVerifiedRetrySettles (spec test 5, cancel).
func TestIntegrationRunCancelKilledPublisherThenVerifiedRetrySettles(t *testing.T) {
	fx := newCancelFixture(t)
	desc := lockTestWSDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key,
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token},
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc})
	res := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q (findings %v), want cancelled", res.Disposition, res.Findings)
	}
	if countFinding(res.Findings, "mutation-settled:"+OperationWorkspacePublish) != 1 || hasFinding(res.Findings, "mutation-abandoned") {
		t.Fatalf("findings = %v, want mutation-settled and no mutation-abandoned", res.Findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusCompleted || m.Verified {
		t.Fatalf("original = %+v, want completed and still unverified", m)
	}
}

// TestIntegrationRunCancelPublisherFinishingAfterFenceIsNotAbandoned (Review Focus 4):
// cancel wins while a live publisher is mid-call, and the publisher's own callback
// then resolves it.
func TestIntegrationRunCancelPublisherFinishingAfterFenceIsNotAbandoned(t *testing.T) {
	fx := newCancelFixture(t)
	desc := lockTestWSDesc()
	done, err := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &desc)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	first := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if first.Disposition != CancelDispositionPending || !hasFinding(first.Findings, "mutation-pending:"+OperationWorkspacePublish) {
		t.Fatalf("mid-call cancel = %q %v, want pending on the live publisher", first.Disposition, first.Findings)
	}
	done(mutationStatusCompleted, true)
	second := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if second.Disposition != CancelDispositionCancelled || hasFinding(second.Findings, "mutation-abandoned") || hasFinding(second.Findings, "mutation-pending") {
		t.Fatalf("after the publisher finished: %q %v, want cancelled with no journal finding", second.Disposition, second.Findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusCompleted || !m.Verified {
		t.Fatalf("entry = %+v, want the publisher's own completed+verified outcome", m)
	}
}
