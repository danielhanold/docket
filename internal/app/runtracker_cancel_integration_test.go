//go:build integration

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

// These are the `run cancel` operation tests (change 0375 Task 10). run.cancel is
// the coordinator's explicit Stop: it durably fences the run, tears down
// registered tasks/processes and the run's gate drives (the launch census,
// attributed by the run's context hash — change 0490), reconciles admitted
// mutations, and reports cancelled only on full accounting. Most tests drive the
// flow over faked stop/native/census seams and a real gatedrive store rooted at a
// real temp git repo; the change 0490 tests at the end run the production seams
// over a real supervised drive.

// TestIntegrationRunCancelRunCancelHappyPath: an active run whose launch census
// accounts every drive cancels cleanly — disposition cancelled, run cancelled, the
// census run once for the run's context hash, and nothing else stopped (the run
// registered no execution participant).
func TestIntegrationRunCancelRunCancelHappyPath(t *testing.T) {
	fx := newCancelFixture(t)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	recon := okLaunchReconciler()
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: recon}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if res.Result != ResultApplied {
		t.Fatalf("result = %q, want applied", res.Result)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
		t.Fatalf("run state = %q, want cancelled", st)
	}
	if len(recon.calls) != 1 || recon.calls[0] != fx.contextHash {
		t.Fatalf("census calls = %v, want [%s] (the run's context hash)", recon.calls, fx.contextHash)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("stopper calls = %v, want none (no registered execution participant)", stopper.calls)
	}
}

// TestIntegrationRunCancelRunCancelPendingOnUnprovenStop: a registered execution
// participant whose stop is unproven fences the run but leaves it cancelling —
// disposition cancellation-pending with a stop-unproven finding naming it.
func TestIntegrationRunCancelRunCancelPendingOnUnprovenStop(t *testing.T) {
	fx := newCancelFixture(t)
	must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: participantKindRawRun, NativeHandle: fx.runDir}))
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: false}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending", res.Disposition)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelling {
		t.Fatalf("run state = %q, want cancelling (durable fence held)", st)
	}
	if !hasFinding(res.Findings, "stop-unproven:"+fx.runDir) {
		t.Fatalf("findings = %v, want stop-unproven:%s", res.Findings, fx.runDir)
	}
}

// TestIntegrationRunCancelRunCancelAlreadyCancelled: a repeat against a cancelled run is idempotent
// already-cancelled, touching nothing.
func TestIntegrationRunCancelRunCancelAlreadyCancelled(t *testing.T) {
	fx := newCancelFixture(t)
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.State = RunCancelled
		return nil
	}); err != nil {
		t.Fatalf("runRecordCAS: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{}}
	// The terminal path now runs the bounded historical repair, which re-proves
	// quiescence through the launch reconciler; a nil reconciler is unverifiable and
	// refused by design, so an authorized terminal repeat injects okLaunchReconciler().
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionAlreadyCancelled {
		t.Fatalf("disposition = %q, want already-cancelled", res.Disposition)
	}
	if res.Result != ResultNoOp {
		t.Fatalf("result = %q, want no-op", res.Result)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("already-cancelled must stop nothing, got %v", stopper.calls)
	}
}

// TestIntegrationRunCancelRunCancelRefusedWrongRun: a stale run locator is refused with no fence.
func TestIntegrationRunCancelRunCancelRefusedWrongRun(t *testing.T) {
	fx := newCancelFixture(t)
	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}}, fx.repo, fx.key, "not-the-run", "human stop")
	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if !hasFinding(res.Findings, "run-id-mismatch") {
		t.Fatalf("findings = %v, want run-id-mismatch", res.Findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunActive {
		t.Fatalf("a refused cancel must not fence: run state = %q, want active", st)
	}
}

// TestIntegrationRunCancelRunCancelRefusedWrongClaim: an unconfirmed (here, absent) claim binding is
// refused — the run is not a genuinely claimed run.
func TestIntegrationRunCancelRunCancelRefusedWrongClaim(t *testing.T) {
	// Build a fixture WITHOUT confirming a claim.
	repo := newRunTrackerRepo(t)
	common, _ := runTrackerGitCommonDir(repo)
	key, err := MintRunTrackerRecord(repo, RunTrackerRecord{
		Target: runStartStoredTarget, AttemptLimit: 2, Retry: RetryUnused,
		Disposition: "run-started", ParentCap: "parent-cap-raw",
	})
	if err != nil {
		t.Fatalf("MintRunTrackerRecord: %v", err)
	}
	ep, err := MintRunRecord(repo, key, "42")
	if err != nil {
		t.Fatalf("MintRunRecord: %v", err)
	}
	res := runCancel(cancelSeams{store: gatedrive.OpenStore(common), stopper: &fakeCancelStopper{}}, repo, key, ep.RunID, "human stop")
	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if !hasFinding(res.Findings, "claim-unconfirmed") {
		t.Fatalf("findings = %v, want claim-unconfirmed", res.Findings)
	}
	if st := loadRunState(t, repo, key); st != RunActive {
		t.Fatalf("a refused cancel must not fence: run state = %q, want active", st)
	}
}

// TestIntegrationRunCancelRunCancelRefusedWrongRepo: a key that does not locate a record in this
// repository is refused (the repository/locator authority fails closed).
func TestIntegrationRunCancelRunCancelRefusedWrongRepo(t *testing.T) {
	fx := newCancelFixture(t)
	other := newRunTrackerRepo(t)
	otherCommon, _ := runTrackerGitCommonDir(other)
	res := runCancel(cancelSeams{store: gatedrive.OpenStore(otherCommon), stopper: &fakeCancelStopper{}}, other, fx.key, fx.runID, "human stop")
	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused (findings=%v)", res.Disposition, res.Findings)
	}
	// The original run in the real repo is untouched.
	if st := loadRunState(t, fx.repo, fx.key); st != RunActive {
		t.Fatalf("a foreign-repo cancel must not touch the real run: state = %q", st)
	}
}

// TestIntegrationRunCancelFencesBeforeStopping: a participant that registers between the fence and
// the stop (a launch admitted before the fence won) is caught by the post-stop
// re-enumeration, keeping the cancellation pending.
func TestIntegrationRunCancelFencesBeforeStopping(t *testing.T) {
	fx := newCancelFixture(t)
	// A raw-run participant present at entry; stopping it proves teardown.
	if err := RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: "raw-run", NativeHandle: "R1"}); err != nil {
		t.Fatalf("RegisterRunParticipant P1: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{"R1": true, fx.runDir: true}}
	// The barrier: when P1 is stopped (after the fence), a racing launch registers
	// P2 directly (the run is cancelling, so the normal registration path would be
	// refused — this stands in for a launch admitted before the fence).
	stopper.onStop = func(runDir string) {
		if runDir != "R1" {
			return
		}
		_ = runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
			r.Participants = append(r.Participants, RunParticipant{Kind: "raw-run", NativeHandle: "R2", RegisteredAt: "x"})
			return nil
		})
	}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (the racing P2 must be caught)", res.Disposition)
	}
	if !hasFinding(res.Findings, "unaccounted-participant:R2") {
		t.Fatalf("findings = %v, want unaccounted-participant:R2", res.Findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelling {
		t.Fatalf("run state = %q, want cancelling", st)
	}
}

// TestIntegrationRunCancelRepeatResumesCleanup: a first cancel fences and leaves the run pending
// on an unproven stop; a repeat against the cancelling run resumes cleanup (no
// re-fence, no authority restore) and completes to cancelled when the stop proves.
func TestIntegrationRunCancelRepeatResumesCleanup(t *testing.T) {
	fx := newCancelFixture(t)
	must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: participantKindRawRun, NativeHandle: fx.runDir}))
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: false}}

	first := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")
	if first.Disposition != CancelDispositionPending {
		t.Fatalf("first disposition = %q, want cancellation-pending", first.Disposition)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelling {
		t.Fatalf("after first cancel run state = %q, want cancelling", st)
	}

	// The teardown now proves; a repeat resumes cleanup on the cancelling run.
	stopper.proven[fx.runDir] = true
	second := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")
	if second.Disposition != CancelDispositionCancelled {
		t.Fatalf("second disposition = %q, want cancelled (findings=%v)", second.Disposition, second.Findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
		t.Fatalf("after repeat run state = %q, want cancelled", st)
	}
}

// TestIntegrationRunCancelPendingOnUncompletedMutation: an admitted-not-completed mutation keeps
// the cancellation pending even when every process teardown proves — no premature
// cancelled.
func TestIntegrationRunCancelPendingOnUncompletedMutation(t *testing.T) {
	fx := newCancelFixture(t)
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{{OpKey: "pr.publish", Status: "admitted"}}
		return nil
	}); err != nil {
		t.Fatalf("runRecordCAS seed mutation: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (uncompleted mutation)", res.Disposition)
	}
	if !hasFinding(res.Findings, "mutation-pending:pr.publish") {
		t.Fatalf("findings = %v, want mutation-pending:pr.publish", res.Findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelling {
		t.Fatalf("run state = %q, want cancelling", st)
	}
}

// TestIntegrationRunCancelSettlesUncertainPublicationWithIdenticalRetry (change 0444 acceptance
// 1): an uncertain PR publication plus a later completed identical retry — with
// every process teardown proven — lets cancellation durably complete the original
// entry and report cancelled; the terminal run is then quiescent for resume and
// SupersedeCancelledRun admits exactly one replacement.
func TestIntegrationRunCancelSettlesUncertainPublicationWithIdenticalRetry(t *testing.T) {
	fx := newCancelFixture(t)
	desc := MutationPublication{
		RepoHost: "github.com", RepoOwner: "o", RepoName: "r",
		HeadRef: "fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationPRPublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationPRPublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q (findings %v), want cancelled", res.Disposition, res.Findings)
	}
	if hasFinding(res.Findings, "mutation-pending") {
		t.Fatalf("findings = %v, must not report the settled mutation pending", res.Findings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatal("the ORIGINAL record must be durably completed, not merely the result string")
	}
	// Resume path: the terminal run is quiescent and admits its one replacement.
	if ok, detail := validateResumeQuiescence(cancelSeams{store: fx.store, launches: okLaunchReconciler()}, fx.repo, ep); !ok {
		t.Fatalf("resume quiescence = %q, want quiescent after settlement", detail)
	}
	if err := SupersedeCancelledRun(fx.repo, fx.key, "replacement-key"); err != nil {
		t.Fatalf("SupersedeCancelledRun: %v (resume must admit exactly one replacement)", err)
	}
}

// TestIntegrationRunCancelStaysPendingWithoutCompletedIdenticalRetry (change 0444 acceptance 2):
// a workspace publication settles analogously, and an uncertain entry with NO
// completed identical retry keeps cancellation-pending — then a subsequent
// identical completed retry lets the SAME pending cancellation finish (acceptance
// 4 tail).
func TestIntegrationRunCancelStaysPendingWithoutCompletedIdenticalRetry(t *testing.T) {
	fx := newCancelFixture(t)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}

	first := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop")
	if first.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (no completed identical retry)", first.Disposition)
	}
	if !hasFinding(first.Findings, "mutation-pending:"+OperationWorkspacePublish) {
		t.Fatalf("findings = %v, want mutation-pending:workspace.publish", first.Findings)
	}

	// A subsequent identical successful retry lands in the journal; the SAME repeat
	// cancel now converges.
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = append(r.AdmittedMutations,
			AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc})
		return nil
	}); err != nil {
		t.Fatalf("append retry: %v", err)
	}
	second := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop")
	if second.Disposition != CancelDispositionCancelled {
		t.Fatalf("repeat disposition = %q (findings %v), want cancelled", second.Disposition, second.Findings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatal("the ORIGINAL workspace record must be durably completed after the repeat cancel")
	}
}

// TestIntegrationRunCancelNativeAdapterAbsentIsFindingNotSilence: with no native adapter wired, a
// native participant yields an explicit finding while the process teardown still
// accounts the run to cancelled.
func TestIntegrationRunCancelNativeAdapterAbsentIsFindingNotSilence(t *testing.T) {
	fx := newCancelFixture(t)
	if err := RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: "coordinator", NativeHandle: "turn-1"}); err != nil {
		t.Fatalf("RegisterRunParticipant: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, native: nil, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled", res.Disposition)
	}
	if !hasFinding(res.Findings, "native-cancel-unavailable") {
		t.Fatalf("an absent adapter must produce a finding, not silence: findings=%v", res.Findings)
	}
}

// TestIntegrationRunCancelNeverChargesOrResets: cancellation touches neither the change-owned
// suite budget nor the run-tracker retry markers, and resets no run-tracker-record retry state.
func TestIntegrationRunCancelNeverChargesOrResets(t *testing.T) {
	fx := newCancelFixture(t)

	// Seed a consumed retry marker and a reserved suite attempt.
	if ok, err := ConsumeRunTrackerRetry(fx.repo, fx.key, 1, 2); err != nil || !ok {
		t.Fatalf("ConsumeRunTrackerRetry = (%v, %v), want (true, nil)", ok, err)
	}
	retryBefore, err := RunTrackerRetryUsage(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("RunTrackerRetryUsage: %v", err)
	}
	budgetKey := gatedrive.SuiteBudgetKey{RepoIdentity: fx.common, ChangeID: "42", Phase: "build"}
	if _, _, err := fx.store.ReserveSuiteAttempt(budgetKey, 3); err != nil {
		t.Fatalf("ReserveSuiteAttempt: %v", err)
	}
	usedBefore, limitBefore, err := fx.store.SuiteBudgetUsage(budgetKey)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage: %v", err)
	}

	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled", res.Disposition)
	}

	retryAfter, err := RunTrackerRetryUsage(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("RunTrackerRetryUsage after: %v", err)
	}
	if retryAfter != retryBefore {
		t.Fatalf("retry markers changed: before=%d after=%d — cancellation must not touch retry state", retryBefore, retryAfter)
	}
	usedAfter, limitAfter, err := fx.store.SuiteBudgetUsage(budgetKey)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage after: %v", err)
	}
	if usedAfter != usedBefore || limitAfter != limitBefore {
		t.Fatalf("suite budget changed: before=(%d,%d) after=(%d,%d) — cancellation charges no attempt", usedBefore, limitBefore, usedAfter, limitAfter)
	}
	rec, err := LoadRunTrackerRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord after: %v", err)
	}
	if rec.Retry != RetryConsumed {
		t.Fatalf("gate record Retry = %q, want consumed (unchanged)", rec.Retry)
	}
	if rec.AttemptLimit != 2 {
		t.Fatalf("gate record AttemptLimit = %d, want 2 (unchanged)", rec.AttemptLimit)
	}
}

// TestIntegrationRunCancelPendingWhileLaunchObligationUnresolved: even with every process teardown
// proven, a run-linked launch obligation the reconciler reports unsettled keeps
// the cancellation pending (a completed replacement must never first appear after a
// completed cancellation), surfacing the reconciler's findings.
func TestIntegrationRunCancelPendingWhileLaunchObligationUnresolved(t *testing.T) {
	fx := newCancelFixture(t)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	recon := &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{
		Accounted: false,
		Findings:  []string{"launch-pending:d1"},
	}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: recon}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (an unsettled launch obligation)", res.Disposition)
	}
	if !hasFinding(res.Findings, "launch-pending:d1") {
		t.Fatalf("findings = %v, want the reconciler's launch-pending:d1 surfaced", res.Findings)
	}
	if len(recon.calls) != 1 || recon.calls[0] != fx.contextHash {
		t.Fatalf("reconciler calls = %v, want [%s] (the run's context hash)", recon.calls, fx.contextHash)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelling {
		t.Fatalf("run state = %q, want cancelling (durable fence held)", st)
	}
}

// TestIntegrationRunCancelCompletesWhenLaunchObligationsSettle: with the reconciler reporting every
// launch obligation accounted and the rest of the accounting green, cancellation
// completes to cancelled.
func TestIntegrationRunCancelCompletesWhenLaunchObligationsSettle(t *testing.T) {
	fx := newCancelFixture(t)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	recon := okLaunchReconciler()
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: recon}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if len(recon.calls) != 1 || recon.calls[0] != fx.contextHash {
		t.Fatalf("reconciler calls = %v, want [%s] (the run's context hash)", recon.calls, fx.contextHash)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
		t.Fatalf("run state = %q, want cancelled", st)
	}
}

// TestIntegrationRunCancelReconcilerUnavailableFailsClosed: a nil launch reconciler is not silence
// — it is a finding and a fail-closed pending, mirroring the nil-stopper rule.
func TestIntegrationRunCancelReconcilerUnavailableFailsClosed(t *testing.T) {
	fx := newCancelFixture(t)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: nil}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionPending {
		t.Fatalf("disposition = %q, want cancellation-pending (nil reconciler fails closed)", res.Disposition)
	}
	if !hasFinding(res.Findings, "launch-reconciler-unavailable") {
		t.Fatalf("findings = %v, want launch-reconciler-unavailable", res.Findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelling {
		t.Fatalf("run state = %q, want cancelling", st)
	}
}

// TestIntegrationRunCancelRunContextUnreadableFailsClosed (change 0490): the launch
// census attributes a run's drives by the context hash its run-tracker record
// carries, so a record that cannot be read names no hash — never the empty hash
// that would account vacuously. Both the stop-capable teardown and the
// observation-only closeout fail closed with run-context-unreadable, without
// running the census at all.
func TestIntegrationRunCancelRunContextUnreadableFailsClosed(t *testing.T) {
	fx := newCancelFixture(t)
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	recPath := filepath.Join(fx.common, "docket", runTrackerDirName, fx.key, runTrackerRecordFileName)
	if err := os.WriteFile(recPath, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("corrupt run-tracker record: %v", err)
	}

	recon := okLaunchReconciler()
	ok, findings, terr := reconcileRunTeardown(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: recon}, fx.repo, fx.key, ep)
	if terr != nil {
		t.Fatalf("reconcileRunTeardown: %v", terr)
	}
	if ok || !hasFinding(findings, "run-context-unreadable") {
		t.Fatalf("teardown = (%v, %v), want unaccounted with run-context-unreadable", ok, findings)
	}
	if len(recon.calls) != 0 {
		t.Fatalf("the census must not run without a run context, calls = %q", recon.calls)
	}

	observer := &fakeLaunchObserver{report: gatedrive.RunLaunchReport{Accounted: true}}
	blocked, lf := accountCompletionLaunches(cancelSeams{launchObserver: observer}, fx.repo, fx.key)
	if !blocked || !hasFinding(lf, "run-context-unreadable") {
		t.Fatalf("closeout launches = (%v, %v), want blocked with run-context-unreadable", blocked, lf)
	}
	if len(observer.calls) != 0 {
		t.Fatalf("the closeout census must not run without a run context, calls = %q", observer.calls)
	}
}

// TestIntegrationRunCancelRunCancelPublicEntry: the public RunCancel composes production seams and, over
// a run with no gate drive and no native adapter, refuses cleanly when authority
// is wrong (here a wrong run) — proving the public signature is wired.
func TestIntegrationRunCancelRunCancelPublicEntry(t *testing.T) {
	fx := newCancelFixture(t)
	res := RunCancel(context.Background(), PlanningDeps{}, WorkspaceDeps{}, fx.repo, fx.key, "wrong-run", "human stop")
	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused", res.Disposition)
	}
	if res.Operation != OperationRunCancel {
		t.Fatalf("operation = %q, want %q", res.Operation, OperationRunCancel)
	}
}

// TestIntegrationRunCancelInterruptedBeforeFinalWriteConverges (AC5): the teardown
// was fully accounted but cancelled was never persisted (a crash before the final
// write leaves the run fenced cancelling); the retry revalidates the proof and
// finishes the run transition.
func TestIntegrationRunCancelInterruptedBeforeFinalWriteConverges(t *testing.T) {
	fx := newCancelFixture(t)
	must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: participantKindRawRun, NativeHandle: fx.runDir}))
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	// Reconstruct the crash state directly: the run record CAS has no seam, so the
	// fence and the full teardown are driven here, leaving the run cancelling.
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error { r.State = RunCancelling; return nil }); err != nil {
		t.Fatalf("fence: %v", err)
	}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ok, f, terr := reconcileRunTeardown(seams, fx.repo, fx.key, ep); terr != nil || !ok {
		t.Fatalf("teardown = (%v,%v,%v), want accounted", ok, f, terr)
	}
	// Crash happened here: run still cancelling. The retry:
	res := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("retry disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
		t.Fatalf("run state = %q, want cancelled", st)
	}
}

// TestIntegrationRunCancelConcurrentReplayIsIdempotent (AC4): two sequential replays of a
// completed cancellation are no-ops (already-cancelled) leaving the run cancelled.
func TestIntegrationRunCancelConcurrentReplayIsIdempotent(t *testing.T) {
	fx := newCancelFixture(t)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	seams := cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}
	if res := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("first = %q, want cancelled", res.Disposition)
	}
	for i := 0; i < 2; i++ {
		res := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop")
		if res.Disposition != CancelDispositionAlreadyCancelled {
			t.Fatalf("replay %d = %q, want already-cancelled (findings=%v)", i, res.Disposition, res.Findings)
		}
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
		t.Fatalf("run state = %q, want cancelled and stable", st)
	}
}

// TestIntegrationRunCancelTerminalRepairSupersededIsAlreadyCancelled (AC6): a repeat cancel of a
// SUPERSEDED run with nothing live re-runs the census for the run's own context
// hash and is the idempotent already-cancelled, never regressing the superseded
// state.
func TestIntegrationRunCancelTerminalRepairSupersededIsAlreadyCancelled(t *testing.T) {
	fx := newCancelFixture(t)
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error { r.State = RunSuperseded; return nil }); err != nil {
		t.Fatalf("force superseded: %v", err)
	}
	recon := okLaunchReconciler()
	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: recon}, fx.repo, fx.key, fx.runID, "human repair")
	if res.Disposition != CancelDispositionAlreadyCancelled || res.Result != ResultNoOp {
		t.Fatalf("result = (%q,%q), want (already-cancelled,no-op) (findings=%v)", res.Disposition, res.Result, res.Findings)
	}
	if len(recon.calls) != 1 || recon.calls[0] != fx.contextHash {
		t.Fatalf("census calls = %v, want [%s] (the run's own context hash)", recon.calls, fx.contextHash)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunSuperseded {
		t.Fatalf("run state = %q, want superseded (never regressed)", st)
	}
}

// TestIntegrationRunCancelTerminalRepairRefusesUnsafeHistories (AC6): each unsafe terminal history is
// refused with a specific finding, with NO run mutation, and never
// cancellation-pending over durable terminal state.
func TestIntegrationRunCancelTerminalRepairRefusesUnsafeHistories(t *testing.T) {
	cases := []struct {
		name    string
		arrange func(t *testing.T, fx cancelFixture) cancelSeams
		finding string
	}{
		{"busy-claim-unresolved-relaunch", func(t *testing.T, fx cancelFixture) cancelSeams {
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{},
				launches: &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{Accounted: false, Findings: []string{"claim-busy:d1"}}}}
		}, "claim-busy:d1"},
		{"live-drive", func(t *testing.T, fx cancelFixture) cancelSeams {
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{},
				launches: &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{Accounted: false, Findings: []string{"run-live:d1"}}}}
		}, "run-live:d1"},
		{"contradictory-mutation", func(t *testing.T, fx cancelFixture) cancelSeams {
			if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
				r.AdmittedMutations = []AdmittedMutation{{OpKey: "pr.publish", Status: "admitted"}}
				return nil
			}); err != nil {
				t.Fatalf("seed mutation: %v", err)
			}
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
		}, "mutation-pending:pr.publish"},
		{"unreadable-launch-evidence", func(t *testing.T, fx cancelFixture) cancelSeams {
			return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{},
				launches: &fakeLaunchReconciler{err: fmt.Errorf("injected")}}
		}, "launch-reconcile-failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t)
			seams := tc.arrange(t, fx)
			if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error { r.State = RunCancelled; return nil }); err != nil {
				t.Fatalf("force cancelled: %v", err)
			}
			res := runCancel(seams, fx.repo, fx.key, fx.runID, "human repair")
			if res.Disposition != CancelDispositionRefused {
				t.Fatalf("disposition = %q, want refused (findings=%v)", res.Disposition, res.Findings)
			}
			if !hasFinding(res.Findings, tc.finding) {
				t.Fatalf("findings = %v, want %q", res.Findings, tc.finding)
			}
			if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
				t.Fatalf("run state = %q, want cancelled (no regression, no revival)", st)
			}
		})
	}
}

// TestIntegrationRunCancelGuardianReapsButNeverFinalizes: the death guardian's
// fence+reap stops the run's executions but leaves the run CANCELLING — only an
// authorized run.cancel finalizes it cancelled. The contract is proven against
// the shared reconcileRunTeardown, which the guardian composes and which performs
// no run transition; the cancelling→cancelled CAS stays exclusively in runCancel.
func TestIntegrationRunCancelGuardianReapsButNeverFinalizes(t *testing.T) {
	fx := newCancelFixture(t)
	must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: participantKindRawRun, NativeHandle: fx.runDir}))
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	recon := okLaunchReconciler()
	// guardianFenceAndReap composes productionCancelSeams, whose stopper/reconciler
	// reach the real process service. Drive its exact sequence with injected seams
	// instead: fence, then the SAME teardown accounting, and assert what the
	// guardian contract asserts — a reap, no finalize.
	if ferr := runRecordCAS(fx.repo, fx.key, func(rec *RunRecord) error {
		if rec.State == RunActive {
			rec.State = RunCancelling
		}
		return nil
	}); ferr != nil {
		t.Fatalf("fence: %v", ferr)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ok, f, terr := reconcileRunTeardown(cancelSeams{store: fx.store, stopper: stopper, launches: recon}, fx.repo, fx.key, ep); terr != nil || !ok {
		t.Fatalf("guardian reap = (%v,%v,%v), want accounted", ok, f, terr)
	}
	if len(stopper.calls) != 1 || len(recon.calls) != 1 || recon.calls[0] != fx.contextHash {
		t.Fatalf("guardian reap stopped %v and censused %v, want the participant and the run's context", stopper.calls, recon.calls)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelling {
		t.Fatalf("run state = %q, want cancelling (guardian never finalizes)", st)
	}
	// The authorized completion then finalizes.
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: recon}, fx.repo, fx.key, fx.runID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("authorized completion = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
}

// TestIntegrationRunCancelDoesNotUnfenceOldRunLaunches (AC8): after cancellation the
// OLD run's fresh start is still refused by the 437 launch gate (runLaunchGate) —
// a completed cancellation never revives the cancelled run's launch authority,
// and the refused gate never runs the admission body.
func TestIntegrationRunCancelDoesNotUnfenceOldRunLaunches(t *testing.T) {
	fx := newCancelFixture(t)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	if res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("cancel = %q, want cancelled", res.Disposition)
	}
	gate := runLaunchGate(fx.common)
	reserveRan := false
	err := gate(fx.runID, fx.worktree, func() error { reserveRan = true; return nil })
	if err == nil {
		t.Fatal("the cancelled run's launch authorization must be refused after cancellation")
	}
	if reserveRan {
		t.Fatal("the refused gate must never run the admission body")
	}
}

// TestIntegrationRunCancelRepairChargesNothing (AC8): terminal repair — like cancellation — touches
// neither the suite budget nor the run-tracker retry markers. This mirrors
// TestIntegrationRunCancelNeverChargesOrResets (same seeding and asserts) with a
// repeat cancel of a run already forced cancelled placed between the seeding and
// the accounting-neutrality asserts.
func TestIntegrationRunCancelRepairChargesNothing(t *testing.T) {
	fx := newCancelFixture(t)

	// Seed a consumed retry marker and a reserved suite attempt.
	if ok, err := ConsumeRunTrackerRetry(fx.repo, fx.key, 1, 2); err != nil || !ok {
		t.Fatalf("ConsumeRunTrackerRetry = (%v, %v), want (true, nil)", ok, err)
	}
	retryBefore, err := RunTrackerRetryUsage(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("RunTrackerRetryUsage: %v", err)
	}
	budgetKey := gatedrive.SuiteBudgetKey{RepoIdentity: fx.common, ChangeID: "42", Phase: "build"}
	if _, _, err := fx.store.ReserveSuiteAttempt(budgetKey, 3); err != nil {
		t.Fatalf("ReserveSuiteAttempt: %v", err)
	}
	usedBefore, limitBefore, err := fx.store.SuiteBudgetUsage(budgetKey)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage: %v", err)
	}

	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error { r.State = RunCancelled; return nil }); err != nil {
		t.Fatalf("force cancelled: %v", err)
	}

	res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human repair")
	if res.Disposition != CancelDispositionAlreadyCancelled {
		t.Fatalf("disposition = %q, want already-cancelled (findings=%v)", res.Disposition, res.Findings)
	}

	retryAfter, err := RunTrackerRetryUsage(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("RunTrackerRetryUsage after: %v", err)
	}
	if retryAfter != retryBefore {
		t.Fatalf("retry markers changed: before=%d after=%d — repair must not touch retry state", retryBefore, retryAfter)
	}
	usedAfter, limitAfter, err := fx.store.SuiteBudgetUsage(budgetKey)
	if err != nil {
		t.Fatalf("SuiteBudgetUsage after: %v", err)
	}
	if usedAfter != usedBefore || limitAfter != limitBefore {
		t.Fatalf("suite budget changed: before=(%d,%d) after=(%d,%d) — repair charges no attempt", usedBefore, limitBefore, usedAfter, limitAfter)
	}
	rec, err := LoadRunTrackerRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord after: %v", err)
	}
	if rec.Retry != RetryConsumed {
		t.Fatalf("gate record Retry = %q, want consumed (unchanged)", rec.Retry)
	}
	if rec.AttemptLimit != 2 {
		t.Fatalf("gate record AttemptLimit = %d, want 2 (unchanged)", rec.AttemptLimit)
	}
}

// TestIntegrationRunCancelRunCancelWinsFromCompletingRun: an explicit human cancellation WINS from a
// completing (successful, mid-closeout) run — the fence flips completing→cancelling
// and the ordinary teardown/accounting runs to a proven cancellation, never a
// completing/completed relabelling. Completion then loses (change 0441): its
// completing→completed CAS refuses once this fence lands.
func TestIntegrationRunCancelRunCancelWinsFromCompletingRun(t *testing.T) {
	fx := newCancelFixture(t)
	forceRunState(t, fx.repo, fx.key, RunCompleting)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	st := loadRunState(t, fx.repo, fx.key)
	if st != RunCancelled && st != RunCancelling {
		t.Fatalf("run state = %q, want cancelling or cancelled — never completing/completed", st)
	}
}

// TestIntegrationRunCancelRunCancelRefusesCompletedRun: a completed (successfully closed-out) run
// cannot be cancelled — a no-op refusal carrying the completed-run explanation, never
// a state regression and never cancellation-pending over durable terminal state
// (change 0441).
func TestIntegrationRunCancelRunCancelRefusesCompletedRun(t *testing.T) {
	fx := newCancelFixture(t)
	forceRunState(t, fx.repo, fx.key, RunCompleted)
	stopper := &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}
	res := runCancel(cancelSeams{store: fx.store, stopper: stopper, launches: okLaunchReconciler()}, fx.repo, fx.key, fx.runID, "human stop")

	if res.Disposition != CancelDispositionRefused {
		t.Fatalf("disposition = %q, want refused (findings=%v)", res.Disposition, res.Findings)
	}
	if !hasFinding(res.Findings, "run-completed") {
		t.Fatalf("findings = %v, want a run-completed finding", res.Findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed unchanged (no regression)", st)
	}
	if len(stopper.calls) != 0 {
		t.Fatalf("a refused cancel of a completed run must stop nothing, got %v", stopper.calls)
	}
}

// TestIntegrationRunCancelRepairTerminalRunRemovedWorktree (change 0446 spec §5,
// AC2): a terminal run whose feature worktree directory was REMOVED is still
// repaired to the idempotent already-cancelled — the census is attributed by the
// run's context hash, never by its worktree path.
func TestIntegrationRunCancelRepairTerminalRunRemovedWorktree(t *testing.T) {
	fx := newCancelFixture(t)
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error { r.State = RunCancelled; return nil }); err != nil {
		t.Fatalf("force cancelled: %v", err)
	}
	if err := os.RemoveAll(fx.worktree); err != nil {
		t.Fatalf("remove worktree: %v", err)
	}
	recon := okLaunchReconciler()
	seams := cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: recon}
	for i := 0; i < 2; i++ {
		if res := runCancel(seams, fx.repo, fx.key, fx.runID, "human repair"); res.Disposition != CancelDispositionAlreadyCancelled {
			t.Fatalf("repair %d = %q, want already-cancelled (findings=%v)", i, res.Disposition, res.Findings)
		}
	}
	if len(recon.calls) != 2 || recon.calls[0] != fx.contextHash {
		t.Fatalf("census calls = %v, want the run's context hash on each repair", recon.calls)
	}
}

// supersedeFixtureRun confirms the fixture run cancelled and supersedes it with a
// freshly minted replacement run key whose run binds replacementWorktree ("" leaves
// the replacement run unbound), mirroring armResumeReplacement's order. The
// superseded run's own Worktree is cleared by SupersedeCancelledRun. It returns
// the replacement's run key.
func supersedeFixtureRun(t *testing.T, fx cancelFixture, replacementWorktree string, mintReplacementRun bool) string {
	t.Helper()
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error { r.State = RunCancelled; return nil }); err != nil {
		t.Fatalf("force cancelled: %v", err)
	}
	replKey := mintTestRunKey(t, fx.repo)
	if err := SupersedeCancelledRun(fx.repo, fx.key, replKey); err != nil {
		t.Fatalf("SupersedeCancelledRun: %v", err)
	}
	if mintReplacementRun {
		if _, err := MintRunRecord(fx.repo, replKey, ""); err != nil {
			t.Fatalf("MintRunRecord(replacement): %v", err)
		}
		if replacementWorktree != "" {
			if err := runRecordCAS(fx.repo, replKey, func(r *RunRecord) error { r.Worktree = replacementWorktree; return nil }); err != nil {
				t.Fatalf("bind replacement worktree: %v", err)
			}
		}
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunSuperseded || ep.Worktree != "" {
		t.Fatalf("superseded fixture = state %q worktree %q, want superseded with a cleared worktree", ep.State, ep.Worktree)
	}
	return replKey
}

// TestIntegrationRunCancelTerminalRepairSupersededCensusesOwnContext (change 0490): a
// SUPERSEDED run has an empty Worktree, which the census never needs — terminal
// repair runs it over the predecessor's own run context, so a live drive of the
// predecessor is still found (refused, never inferred safe), while the
// replacement's worktree binding is irrelevant either way.
func TestIntegrationRunCancelTerminalRepairSupersededCensusesOwnContext(t *testing.T) {
	t.Run("quiescent-already-cancelled", func(t *testing.T) {
		fx := newCancelFixture(t)
		supersedeFixtureRun(t, fx, fx.worktree, true)
		launches := okLaunchReconciler()
		res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: launches}, fx.repo, fx.key, fx.runID, "human repair")
		if res.Disposition != CancelDispositionAlreadyCancelled {
			t.Fatalf("disposition = %q, want already-cancelled (findings=%v)", res.Disposition, res.Findings)
		}
		if len(launches.calls) != 1 || launches.calls[0] != fx.contextHash {
			t.Fatalf("census calls = %v, want exactly [%s] (the predecessor's own run context)", launches.calls, fx.contextHash)
		}
		if st := loadRunState(t, fx.repo, fx.key); st != RunSuperseded {
			t.Fatalf("run state = %q, want superseded (never regressed)", st)
		}
	})
	t.Run("live-predecessor-drive-refused", func(t *testing.T) {
		fx := newCancelFixture(t)
		supersedeFixtureRun(t, fx, "", false) // replacement run never minted: irrelevant now
		launches := &fakeLaunchReconciler{report: gatedrive.RunLaunchReport{Accounted: false, Findings: []string{"run-live:d1"}}}
		res := runCancel(cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: launches}, fx.repo, fx.key, fx.runID, "human repair")
		if res.Disposition != CancelDispositionRefused || !hasFinding(res.Findings, "run-live:d1") {
			t.Fatalf("result = %q %v, want refused run-live:d1", res.Disposition, res.Findings)
		}
		if len(launches.calls) != 1 || launches.calls[0] != fx.contextHash {
			t.Fatalf("census calls = %v, want exactly [%s]", launches.calls, fx.contextHash)
		}
	})
}

// tornResumeFixture reproduces a TORN resume the way armResumeReplacement leaves it:
// the replacement's outer scope is prepared (binding the feature worktree) and its
// gate record minted, the confirmed-cancelled predecessor is superseded reserving that
// key, and then the start fails before (neverMinted) or after (unbound) MintRunRecord
// — so no replacement run binds a worktree. It returns the replacement run key.
func tornResumeFixture(t *testing.T, fx cancelFixture, neverMinted bool) string {
	t.Helper()
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error { r.State = RunCancelled; return nil }); err != nil {
		t.Fatalf("force cancelled: %v", err)
	}
	grant, err := fx.store.PrepareScope(gatedrive.ScopeRequest{ChangeID: "42", Worktree: fx.worktree})
	if err != nil {
		t.Fatalf("PrepareScope(replacement): %v", err)
	}
	replKey, err := MintRunTrackerRecord(fx.repo, RunTrackerRecord{
		Target:       runStartStoredTarget,
		AttemptLimit: 1,
		Retry:        RetryUnused,
		Disposition:  "run-started",
		ScopeID:      grant.ScopeID,
		ParentCap:    grant.ParentCapability,
	})
	if err != nil {
		t.Fatalf("MintRunTrackerRecord(replacement): %v", err)
	}
	if err := SupersedeCancelledRun(fx.repo, fx.key, replKey); err != nil {
		t.Fatalf("SupersedeCancelledRun: %v", err)
	}
	if !neverMinted {
		if _, err := MintRunRecord(fx.repo, replKey, ""); err != nil {
			t.Fatalf("MintRunRecord(replacement): %v", err)
		}
	}
	return replKey
}

// TestIntegrationRunCancelTerminalRepairTornResumeConverges (change 0446 spec "Repeated cancellation,
// completion, and admission after safe reconciliation converge using existing
// operations"; change 0490): a torn resume — the predecessor superseded, the
// replacement run never minted, never bound, unreadable, or looping back — is never
// a dead end. Terminal repair reads none of the replacement chain: it censuses the
// predecessor's own run context, so every shape converges on the idempotent
// already-cancelled.
func TestIntegrationRunCancelTerminalRepairTornResumeConverges(t *testing.T) {
	for _, tc := range []struct {
		name    string
		arrange func(t *testing.T, fx cancelFixture)
	}{
		{"replacement-never-minted", func(t *testing.T, fx cancelFixture) { tornResumeFixture(t, fx, true) }},
		{"replacement-unbound", func(t *testing.T, fx cancelFixture) { tornResumeFixture(t, fx, false) }},
		{"replacement-corrupt", func(t *testing.T, fx cancelFixture) {
			replKey := tornResumeFixture(t, fx, false)
			dir, err := runKeyDir(fx.repo, replKey, "test")
			if err != nil {
				t.Fatalf("runKeyDir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, runRecordFileName), []byte("{not json"), 0o600); err != nil {
				t.Fatalf("corrupt replacement run: %v", err)
			}
		}},
		{"replacement-cyclic", func(t *testing.T, fx cancelFixture) {
			replKey := tornResumeFixture(t, fx, false)
			if err := runRecordCAS(fx.repo, replKey, func(r *RunRecord) error {
				r.State = RunSuperseded
				r.ReplacementReserved = fx.key // loops back to the predecessor
				return nil
			}); err != nil {
				t.Fatalf("loop the chain: %v", err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t)
			tc.arrange(t, fx)
			launches := okLaunchReconciler()
			seams := cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: launches}
			for i := 0; i < 2; i++ {
				if res := runCancel(seams, fx.repo, fx.key, fx.runID, "human repair"); res.Disposition != CancelDispositionAlreadyCancelled {
					t.Fatalf("repair %d = %q, want already-cancelled (findings=%v)", i, res.Disposition, res.Findings)
				}
			}
			if len(launches.calls) != 2 || launches.calls[0] != fx.contextHash || launches.calls[1] != fx.contextHash {
				t.Fatalf("census calls = %v, want the predecessor's own run context [%s] on each repair", launches.calls, fx.contextHash)
			}
			if st := loadRunState(t, fx.repo, fx.key); st != RunSuperseded {
				t.Fatalf("run state = %q, want superseded (never regressed)", st)
			}
		})
	}
}

// TestIntegrationRunCancelRemovedWorktreeRunCancels (change 0446 spec AC2): cancelling
// an ACTIVE run whose feature worktree directory was removed needs no worktree
// path: the census is attributed by run context, so the cancel completes and a
// repeat is the idempotent no-op.
func TestIntegrationRunCancelRemovedWorktreeRunCancels(t *testing.T) {
	fx := newCancelFixture(t)
	if err := os.RemoveAll(fx.worktree); err != nil {
		t.Fatalf("remove worktree: %v", err)
	}
	seams := cancelSeams{store: fx.store, stopper: &fakeCancelStopper{}, launches: okLaunchReconciler()}
	if res := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if again := runCancel(seams, fx.repo, fx.key, fx.runID, "human stop"); again.Disposition != CancelDispositionAlreadyCancelled {
		t.Fatalf("repeat = %q, want already-cancelled (findings=%v)", again.Disposition, again.Findings)
	}
}

// --- change 0490: cancel over the production seams and a real supervised drive ---

// startRunDrive starts one build drive INSIDE the fixture's run — carrying the
// run's id and its raw run context, so the launch census attributes it — through
// the production build gate-drive service over the real process supervisor. Start
// observes the suite for up to one slice, so it runs on a goroutine; the returned
// channel receives its result. Supervisors the test spawns are reaped (so a stop
// can prove their group gone) and any run left live is stopped at cleanup.
func startRunDrive(t *testing.T, fx cancelFixture, command string) (string, <-chan GateDriveResult) {
	t.Helper()
	requireProcessSupervisorHere(t)
	svc, res, reason := NewBuildGateDriveService(fx.common, guardianExecutable(t), buildEffWithMaxAttempts(command, 4))
	if svc == nil {
		t.Fatalf("build gate-drive service was nil: %s %s", res, reason)
	}
	runRoot := filepath.Join(testsupport.TempDir(t), "run-drives")
	reapRunSupervisors(t, runRoot)
	t.Cleanup(func() { stopRunsUnder(runRoot) })
	done := make(chan GateDriveResult, 1)
	go func() {
		done <- svc.Start(GateDriveStartRequest{
			RepoDir: fx.worktree, Worktree: fx.worktree, ChangeID: "42", TaskID: "task-1",
			Phase: "build", Branch: "fix/x", Ref: "refs/heads/fix/x", Cwd: fx.worktree,
			RunRoot: runRoot, RunID: fx.runID, RunContext: cancelFixtureRunContext,
		})
	}()
	return runRoot, done
}

// runManifest is the slice of a run's manifest.json these tests read.
type runManifest struct {
	Phase         string `json:"phase"`
	SupervisorPID int    `json:"supervisor_pid"`
	PGID          int    `json:"pgid"`
}

// awaitRunPhase waits until runRoot holds exactly one native run whose manifest has
// reached a phase in want, and returns its run dir and manifest.
func awaitRunPhase(t *testing.T, runRoot string, want ...string) (string, runManifest) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		for _, name := range mustDirEntries(runRoot) {
			if !gateRunIDRe.MatchString(name) {
				continue
			}
			dir := filepath.Join(runRoot, name)
			raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
			if err != nil {
				continue
			}
			var m runManifest
			if json.Unmarshal(raw, &m) != nil || m.SupervisorPID <= 1 {
				continue
			}
			for _, w := range want {
				if m.Phase == w {
					return dir, m
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no run under %s reached phase %v within 30s", runRoot, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// awaitStart receives the backgrounded Start's result, failing on a wedge.
func awaitStart(t *testing.T, done <-chan GateDriveResult) GateDriveResult {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(60 * time.Second):
		t.Fatal("the drive's Start never returned")
		return GateDriveResult{}
	}
}

// onlyDriveID returns the id of the single drive in the fixture's registry.
func onlyDriveID(t *testing.T, fx cancelFixture) string {
	t.Helper()
	drives := mustDirEntries(filepath.Join(fx.common, "docket", "gate-drives", "v2"))
	if len(drives) != 1 {
		t.Fatalf("want exactly one drive, got %v", drives)
	}
	return drives[0]
}

// TestIntegrationRunCancelFreesWorktreeForNextStart (Review Focus 4): a tracked
// run's drive runs a sleeping suite under the real supervisor, which holds the
// worktree's lock. run.cancel over the production seams finds the drive by the
// run's context hash, stops its supervisor, and reports cancelled; the worktree's
// lock is then free for the next start with no recovery, release, or retirement
// step.
func TestIntegrationRunCancelFreesWorktreeForNextStart(t *testing.T) {
	fx := newCancelFixture(t)
	runRoot, done := startRunDrive(t, fx, "sleep 60")
	awaitRunPhase(t, runRoot, "running")
	root, store, ok := resolveWorktreeAdmission(fx.worktree)
	if !ok {
		t.Fatal("resolveWorktreeAdmission: the fixture worktree did not resolve")
	}
	if held, err := store.TryWorktreeLock(root, nil); err == nil {
		held.Release()
		t.Fatal("precondition: the running drive's supervisor must hold the worktree lock")
	}
	id := onlyDriveID(t, fx)

	res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.runID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if !hasFinding(res.Findings, "replacement-stopped:"+id) {
		t.Fatalf("findings = %v, want replacement-stopped:%s (the census stopped the run's drive)", res.Findings, id)
	}
	awaitStart(t, done)

	// The supervisor releases the worktree lock as the last thing it does, just after
	// live.lock (L7), so a proven-stopped run's lock is free at most a moment later.
	// The bounded wait below absorbs that exit, never a recovery step.
	deadline := time.Now().Add(5 * time.Second)
	for {
		next, err := store.TryWorktreeLock(root, nil)
		if err == nil {
			next.Release()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("after cancelled, the next start's worktree lock is still refused: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled: when the run's
// drive's supervisor is already gone — killed outright (vanished), or exited after
// recording its suite's signal death (signaled) — cancel reaches cancelled, never
// cancellation-pending: teardown proof is "the supervisor is gone", and every
// state but running proves it, for the census and for a registered execution
// participant's stop alike.
func TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		kill    bool // SIGKILL the supervisor alone once the suite runs
	}{
		{"vanished", "sleep 60", true},
		{"signaled", "kill -KILL $$", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t)
			runRoot, done := startRunDrive(t, fx, tc.command)
			var runDir string
			if tc.kill {
				dir, m := awaitRunPhase(t, runRoot, "running")
				runDir = dir
				// The orphaned suite outlives its killed supervisor (an accepted loss,
				// change 0492): end its group at cleanup.
				t.Cleanup(func() { _ = syscall.Kill(-m.PGID, syscall.SIGKILL) })
				if err := syscall.Kill(m.SupervisorPID, syscall.SIGKILL); err != nil {
					t.Fatalf("kill the supervisor: %v", err)
				}
			}
			awaitStart(t, done) // the drive's slice sees the death and HALTs
			if runDir == "" {
				runDir, _ = awaitRunPhase(t, runRoot, "terminal")
			}
			if obs := GateObserve(runDir); obs.State == "running" {
				t.Fatalf("precondition: the supervisor must be gone, observed %q", obs.State)
			}
			// The run also registered the run as an execution participant, so the
			// stopper's proof rule (supervisorGone) is exercised beside the census's.
			must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID,
				RunParticipant{Kind: participantKindGateScope, NativeHandle: runDir}))

			res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.runID, "human stop")
			if res.Disposition != CancelDispositionCancelled {
				t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
			}
			if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
				t.Fatalf("run state = %q, want cancelled", st)
			}
		})
	}
}

// TestIntegrationRunCancelRepeatOnTerminalRunRerunsCensus: a repeat cancel of an
// already-cancelled run re-runs the production census for the run's own context
// hash. With only settled drives of the run it is the idempotent already-cancelled;
// once a drive of the run whose supervisor cannot be proven gone exists, the same
// repeat is refused naming that drive — never cancellation-pending over durable
// terminal state, and the run stays cancelled.
func TestIntegrationRunCancelRepeatOnTerminalRunRerunsCensus(t *testing.T) {
	requireProcessSupervisorHere(t)
	fx := newCancelFixture(t)
	gone := filepath.Join(testsupport.TempDir(t), "removed-run-root")
	seedCensusDrive(t, fx.common, "abababababababababababababab0001", map[string]any{
		"worktree_path": fx.worktree, "raw_run_dir": filepath.Join(gone, "run-1"),
		"last_outcome": string(gatedrive.PASSED), "run_context_hash": fx.contextHash,
	})
	forceRunState(t, fx.repo, fx.key, RunCancelled)

	if res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.runID, "human repair"); res.Disposition != CancelDispositionAlreadyCancelled {
		t.Fatalf("repeat over settled drives = %q, want already-cancelled (findings=%v)", res.Disposition, res.Findings)
	}

	// A nonterminal drive of the run whose run dir exists but whose supervisor cannot
	// be observed: the census cannot prove it gone.
	unproven := filepath.Join(testsupport.TempDir(t), "unobservable-run")
	if err := os.MkdirAll(unproven, 0o700); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	const liveID = "abababababababababababababab0002"
	seedCensusDrive(t, fx.common, liveID, map[string]any{
		"worktree_path": fx.worktree, "raw_run_dir": unproven,
		"last_outcome": string(gatedrive.WAITING), "run_context_hash": fx.contextHash,
	})
	res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.runID, "human repair")
	if res.Disposition != CancelDispositionRefused || !hasFinding(res.Findings, "resolution-unresolved:"+liveID) {
		t.Fatalf("repeat over an unprovable drive = %q %v, want refused resolution-unresolved:%s", res.Disposition, res.Findings, liveID)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
		t.Fatalf("run state = %q, want cancelled (never regressed)", st)
	}
}
