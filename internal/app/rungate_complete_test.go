package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// These are the successful-run ownership closeout tests (change 0441 Task 7).
// completeSuccessfulRun is the observation-only counterpart of runCancel: on a
// verified keyed run-complete it fences the epoch completing, proves every registered
// obligation terminal WITHOUT stopping anything, retires the released worktree slot,
// and CASes completing→completed — failing closed on any unsettled obligation and
// never relabelling a cancelled/superseded run successful. The tests drive the flow
// over faked observation seams and a real gatedrive admission store, reusing
// rungate_cancel_test.go's fixtures.

// fakeProcessObserver is an injectable processObserver: it answers proven/unproven
// per run dir (falling back to defaultProven), can return a canned error, records
// every handle it observed, and — like the cancel tests' onStop barrier — can inject a
// race via onObserve. It stops nothing.
type fakeProcessObserver struct {
	proven        map[string]bool
	defaultProven bool
	err           error
	calls         []string
	onObserve     func(handle string)
}

func (f *fakeProcessObserver) observeProcessTerminal(runDir string) (bool, error) {
	f.calls = append(f.calls, runDir)
	if f.onObserve != nil {
		f.onObserve(runDir)
	}
	if f.err != nil {
		return false, f.err
	}
	if f.proven != nil {
		if v, ok := f.proven[runDir]; ok {
			return v, nil
		}
	}
	return f.defaultProven, nil
}

// fakeLaunchObserver is an injectable epochLaunchObserver: it records each
// (worktree,epoch) pair, returns a canned report/error, and can inject a race via
// onObserve (a late participant registered after the accounting snapshot but before
// re-enumeration). It settles nothing.
type fakeLaunchObserver struct {
	report    gatedrive.EpochLaunchReport
	err       error
	calls     []string
	onObserve func()
}

func (f *fakeLaunchObserver) observe(worktree, epochID string) (gatedrive.EpochLaunchReport, error) {
	f.calls = append(f.calls, worktree+"|"+epochID)
	if f.onObserve != nil {
		f.onObserve()
	}
	return f.report, f.err
}

// completionFixture is one prepared successfully-finished run: a fully authorized
// active epoch bound to a worktree whose epoch-owned slot is RELEASED (its drives are
// done), a registered coordinator native task carrying recorded terminal evidence,
// every mutation completed, and permissive observation seams (every process proven
// terminal, every launch accounted).
type completionFixture struct {
	repo, key, epochID, worktree, runDir, common string
	store                                        *gatedrive.Store
	observer                                     *fakeProcessObserver
	launchObserver                               *fakeLaunchObserver
}

func (f completionFixture) seams() cancelSeams {
	return cancelSeams{store: f.store, observer: f.observer, launchObserver: f.launchObserver}
}

func newCompletionFixture(t *testing.T) completionFixture {
	t.Helper()
	base := newCancelFixture(t, true)
	// A successful closeout retires only a RELEASED owned slot (the run's drives are
	// done), exactly like cancellation — release the fixture's confirmed slot.
	slot, _, err := base.store.LoadWorktreeExecution(base.worktree)
	if err != nil {
		t.Fatalf("load slot: %v", err)
	}
	if err := base.store.ReleaseWorktreeExecution(base.worktree, slot.ReservationToken); err != nil {
		t.Fatalf("release slot: %v", err)
	}
	must(t, RegisterEpochParticipant(base.repo, base.key, base.epochID,
		EpochParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
	must(t, RecordEpochParticipantTerminal(base.repo, base.key, base.epochID,
		"turn-1", "t1", ParticipantTerminalCompleted))
	return completionFixture{
		repo: base.repo, key: base.key, epochID: base.epochID, worktree: base.worktree,
		runDir: base.runDir, common: base.common, store: base.store,
		observer:       &fakeProcessObserver{defaultProven: true},
		launchObserver: &fakeLaunchObserver{report: gatedrive.EpochLaunchReport{Accounted: true}},
	}
}

// slotReservationToken reads the worktree slot's current reservation token.
func slotReservationToken(t *testing.T, store *gatedrive.Store, worktree string) string {
	t.Helper()
	slot, _, err := store.LoadWorktreeExecution(worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	return slot.ReservationToken
}

// TestCompleteSuccessfulRunHappyPath: a fully settled run closes out — ok, epoch
// EpochCompleted, the owned released slot detached (RunEpochID cleared) with its state
// still released and every history field intact (AC2 history preservation).
func TestCompleteSuccessfulRunHappyPath(t *testing.T) {
	fx := newCompletionFixture(t)
	before, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load before: %v", err)
	}
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCompleted {
		t.Fatalf("epoch state = %q, want completed", st)
	}
	after, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load after: %v", err)
	}
	if after.RunEpochID != "" {
		t.Fatalf("slot RunEpochID = %q, want cleared", after.RunEpochID)
	}
	if string(after.State) != "released" {
		t.Fatalf("slot state = %q, want released", after.State)
	}
	if after.RawRunID != before.RawRunID || after.RawRunDir != before.RawRunDir ||
		after.ExecutionGen != before.ExecutionGen || after.DriveID != before.DriveID ||
		after.Kind != before.Kind {
		t.Fatalf("closeout must preserve history: before=%+v after=%+v", before, after)
	}
}

// TestCompleteSuccessfulRunIdempotentReplay: a second closeout of a completed epoch is
// a no-op success — the stored epoch generation is byte-stable across the replay.
func TestCompleteSuccessfulRunIdempotentReplay(t *testing.T) {
	fx := newCompletionFixture(t)
	if ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key); !ok {
		t.Fatalf("first closeout ok=false reason=%q findings=%v", reason, findings)
	}
	_, genBefore, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("load gen before: %v", err)
	}
	if ok, reason, _ := completeSuccessfulRun(fx.seams(), fx.repo, fx.key); !ok {
		t.Fatalf("replay ok=false reason=%q", reason)
	}
	_, genAfter, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("load gen after: %v", err)
	}
	if genAfter != genBefore {
		t.Fatalf("idempotent replay wrote: generation %q -> %q", genBefore, genAfter)
	}
}

// TestCompleteSuccessfulRunNeverRelabelsCancellation: a cancelling/cancelled run is
// run-cancelled, a superseded run is stale-run-epoch, and the epoch state is never
// rewritten to a successful one.
func TestCompleteSuccessfulRunNeverRelabelsCancellation(t *testing.T) {
	cases := []struct {
		state  epochState
		reason string
	}{
		{EpochCancelling, "run-cancelled"},
		{EpochCancelled, "run-cancelled"},
		{EpochSuperseded, "stale-run-epoch"},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			fx := newCompletionFixture(t)
			forceEpochState(t, fx.repo, fx.key, tc.state)
			ok, reason, _ := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
			if ok {
				t.Fatalf("state %q closed out successfully", tc.state)
			}
			if reason != tc.reason {
				t.Fatalf("reason = %q, want %q", reason, tc.reason)
			}
			if st := loadEpochState(t, fx.repo, fx.key); st != tc.state {
				t.Fatalf("state %q was relabelled to %q", tc.state, st)
			}
		})
	}
}

// TestCompleteSuccessfulRunBlocksOnEveryUnsettledObligation (AC3): each unsettled
// obligation fails closed — ok false, reason completion-unaccounted, the named
// finding present, the epoch left durably completing, and the slot untouched.
func TestCompleteSuccessfulRunBlocksOnEveryUnsettledObligation(t *testing.T) {
	// each build returns the seams and the located run; the epoch begins active so the
	// closeout drives the real completing fence before it blocks.
	type row struct {
		name    string
		finding string
		build   func(t *testing.T) (seams cancelSeams, repo, key, epochID, worktree string)
	}
	permissiveSeams := func(store *gatedrive.Store) cancelSeams {
		return cancelSeams{store: store,
			observer:       &fakeProcessObserver{defaultProven: true},
			launchObserver: &fakeLaunchObserver{report: gatedrive.EpochLaunchReport{Accounted: true}}}
	}
	registerDoneCoordinator := func(t *testing.T, repo, key, epochID string) {
		must(t, RegisterEpochParticipant(repo, key, epochID, EpochParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
		must(t, RecordEpochParticipantTerminal(repo, key, epochID, "turn-1", "t1", ParticipantTerminalCompleted))
	}
	rows := []row{
		{"native-participant-unobserved", "participant-unobserved:coordinator", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			must(t, RegisterEpochParticipant(fx.repo, fx.key, fx.epochID, EpochParticipant{Kind: "coordinator", NativeHandle: "turn-2"}))
			return fx.seams(), fx.repo, fx.key, fx.epochID, fx.worktree
		}},
		{"live-execution-participant", "process-live:exec-1", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			must(t, RegisterEpochParticipant(fx.repo, fx.key, fx.epochID, EpochParticipant{Kind: "raw-run", NativeHandle: "exec-1"}))
			fx.observer.defaultProven = false
			fx.observer.proven = map[string]bool{fx.runDir: true} // the slot's run stays proven
			return fx.seams(), fx.repo, fx.key, fx.epochID, fx.worktree
		}},
		{"nil-observer", "process-observer-unavailable", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			// An execution participant needs observation; a released owned slot does
			// not (its release is the durable proof — change 0446), so the nil
			// observer is exercised through the participant pass.
			must(t, RegisterEpochParticipant(fx.repo, fx.key, fx.epochID, EpochParticipant{Kind: "raw-run", NativeHandle: "exec-1"}))
			s := fx.seams()
			s.observer = nil
			return s, fx.repo, fx.key, fx.epochID, fx.worktree
		}},
		{"launch-not-accounted", "claim-busy:d9", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			fx.launchObserver.report = gatedrive.EpochLaunchReport{Accounted: false, Findings: []string{"claim-busy:d9"}}
			return fx.seams(), fx.repo, fx.key, fx.epochID, fx.worktree
		}},
		{"nil-launch-observer", "launch-observer-unavailable", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			s := fx.seams()
			s.launchObserver = nil
			return s, fx.repo, fx.key, fx.epochID, fx.worktree
		}},
		{"mutation-pending", "mutation-pending:pr.publish", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			must(t, epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
				r.AdmittedMutations = []AdmittedMutation{{OpKey: "pr.publish", Status: "admitted"}}
				return nil
			}))
			return fx.seams(), fx.repo, fx.key, fx.epochID, fx.worktree
		}},
		{"owned-slot-not-released", "slot-not-released", func(t *testing.T) (cancelSeams, string, string, string, string) {
			base := newCancelFixture(t, true) // slot left executing (not released)
			registerDoneCoordinator(t, base.repo, base.key, base.epochID)
			return permissiveSeams(base.store), base.repo, base.key, base.epochID, base.worktree
		}},
		{"unowned-slot", "slot-ownership-unresolved", func(t *testing.T) (cancelSeams, string, string, string, string) {
			base := newCancelFixture(t, false) // no epoch-owned slot; worktree bound
			// An epoch-less EXECUTING slot not linked to any registered participant.
			tok, err := base.store.ReserveRawWorktreeExecution(base.common, base.worktree, nil)
			if err != nil {
				t.Fatalf("reserve raw: %v", err)
			}
			if err := base.store.ConfirmWorktreeExecution(base.worktree, tok, "run-U", filepath.Join(base.worktree, "run-U")); err != nil {
				t.Fatalf("confirm raw: %v", err)
			}
			registerDoneCoordinator(t, base.repo, base.key, base.epochID)
			return permissiveSeams(base.store), base.repo, base.key, base.epochID, base.worktree
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			seams, repo, key, _, worktree := r.build(t)
			slotEpochBefore := loadSlotEpoch(t, seams.store, worktree)
			ok, reason, findings := completeSuccessfulRun(seams, repo, key)
			if ok {
				t.Fatalf("closed out with an unsettled obligation")
			}
			if reason != "completion-unaccounted" {
				t.Fatalf("reason = %q, want completion-unaccounted (findings=%v)", reason, findings)
			}
			if !hasFinding(findings, r.finding) {
				t.Fatalf("findings = %v, want %q", findings, r.finding)
			}
			if st := loadEpochState(t, repo, key); st != EpochCompleting {
				t.Fatalf("epoch state = %q, want completing (success fence held)", st)
			}
			if epo := loadSlotEpoch(t, seams.store, worktree); epo != slotEpochBefore {
				t.Fatalf("blocked closeout touched the slot: RunEpochID %q -> %q", slotEpochBefore, epo)
			}
		})
	}
}

// TestCompleteSuccessfulRunSkipsObservingNonReleasedOwnedSlot isolates step (3)'s
// slot-released guard from retirement's own slot-not-released check (defense in
// depth): a non-released owned slot is refused BEFORE its process is observed, so a
// live slot is never probed as if it were settled. Retirement independently refuses a
// non-released owned slot with the same finding, so this observation-order assertion
// is what pins the EARLY guard as load-bearing rather than decoration.
func TestCompleteSuccessfulRunSkipsObservingNonReleasedOwnedSlot(t *testing.T) {
	base := newCancelFixture(t, true) // epoch-owned slot left EXECUTING (not released)
	must(t, RegisterEpochParticipant(base.repo, base.key, base.epochID,
		EpochParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
	must(t, RecordEpochParticipantTerminal(base.repo, base.key, base.epochID,
		"turn-1", "t1", ParticipantTerminalCompleted))
	observer := &fakeProcessObserver{defaultProven: true}
	seams := cancelSeams{store: base.store, observer: observer,
		launchObserver: &fakeLaunchObserver{report: gatedrive.EpochLaunchReport{Accounted: true}}}
	ok, reason, findings := completeSuccessfulRun(seams, base.repo, base.key)
	if ok {
		t.Fatal("closed out over a non-released owned slot")
	}
	if reason != "completion-unaccounted" || !hasFinding(findings, "slot-not-released") {
		t.Fatalf("reason=%q findings=%v, want completion-unaccounted + slot-not-released", reason, findings)
	}
	for _, c := range observer.calls {
		if c == base.runDir {
			t.Fatalf("a non-released owned slot's process was observed (%q); the released guard must short-circuit first", c)
		}
	}
}

// TestCompleteSuccessfulRunSendsNoStops (AC3): closeout observes only — it never calls
// the stop-capable stopper, native-canceller, or reconcile (stop) seam, on either the
// happy path or a blocked path.
func TestCompleteSuccessfulRunSendsNoStops(t *testing.T) {
	assertNoStops := func(t *testing.T, stopper *fakeCancelStopper, native *fakeNativeCanceller, recon *fakeLaunchReconciler) {
		t.Helper()
		if len(stopper.calls) != 0 {
			t.Fatalf("closeout stopped a process: %v", stopper.calls)
		}
		if len(native.calls) != 0 {
			t.Fatalf("closeout cancelled a native task: %v", native.calls)
		}
		if len(recon.calls) != 0 {
			t.Fatalf("closeout drove the stop-capable reconcile seam: %v", recon.calls)
		}
	}
	// Happy path.
	fx := newCompletionFixture(t)
	stopper := &fakeCancelStopper{proven: map[string]bool{}}
	native := &fakeNativeCanceller{}
	recon := okLaunchReconciler()
	seams := cancelSeams{store: fx.store, stopper: stopper, native: native, launches: recon,
		observer: fx.observer, launchObserver: fx.launchObserver}
	if ok, reason, findings := completeSuccessfulRun(seams, fx.repo, fx.key); !ok {
		t.Fatalf("happy path ok=false reason=%q findings=%v", reason, findings)
	}
	assertNoStops(t, stopper, native, recon)

	// Blocked path.
	fx2 := newCompletionFixture(t)
	fx2.launchObserver.report = gatedrive.EpochLaunchReport{Accounted: false, Findings: []string{"claim-busy:x"}}
	stopper2 := &fakeCancelStopper{proven: map[string]bool{}}
	native2 := &fakeNativeCanceller{}
	recon2 := okLaunchReconciler()
	seams2 := cancelSeams{store: fx2.store, stopper: stopper2, native: native2, launches: recon2,
		observer: fx2.observer, launchObserver: fx2.launchObserver}
	if ok, _, _ := completeSuccessfulRun(seams2, fx2.repo, fx2.key); ok {
		t.Fatal("blocked path reported success")
	}
	assertNoStops(t, stopper2, native2, recon2)
}

// TestCompleteSuccessfulRunLateParticipantBlocks (AC3 "late participant"): a
// participant appended after the accounting snapshot but before retirement is caught
// by re-enumeration, blocking the closeout.
func TestCompleteSuccessfulRunLateParticipantBlocks(t *testing.T) {
	fx := newCompletionFixture(t)
	// The observer proves the slot's own run terminal but nothing else.
	fx.observer.defaultProven = false
	fx.observer.proven = map[string]bool{fx.runDir: true}
	// During the accounting pass's launch observation, a pre-fence launch registers a
	// terminal-unproven execution participant directly (the completing fence would
	// refuse the normal registration path — this stands in for that race).
	fx.launchObserver.onObserve = func() {
		_ = epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
			r.Participants = append(r.Participants, EpochParticipant{Kind: "raw-run", NativeHandle: "LATE", RegisteredAt: "x"})
			return nil
		})
	}
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if ok {
		t.Fatal("a late unproven participant did not block the closeout")
	}
	if reason != "completion-unaccounted" {
		t.Fatalf("reason = %q, want completion-unaccounted (findings=%v)", reason, findings)
	}
	if !hasFinding(findings, "process-live:LATE") {
		t.Fatalf("findings = %v, want process-live:LATE", findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCompleting {
		t.Fatalf("epoch state = %q, want completing (blocked, fence held)", st)
	}
}

// TestCompleteSuccessfulRunReplayAfterRetireBeforeComplete (AC6 "interruption
// before/after slot retirement"): a crash between the slot retirement and the
// completing→completed CAS leaves the slot already detached and the epoch still
// completing; a replay accepts the safe prior detachment and finishes to completed.
func TestCompleteSuccessfulRunReplayAfterRetireBeforeComplete(t *testing.T) {
	fx := newCompletionFixture(t)
	token := slotReservationToken(t, fx.store, fx.worktree)
	if err := fx.store.RetireWorktreeExecutionEpoch(fx.worktree, fx.epochID, token); err != nil {
		t.Fatalf("out-of-band retire: %v", err)
	}
	forceEpochState(t, fx.repo, fx.key, EpochCompleting)
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("replay ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCompleted {
		t.Fatalf("epoch state = %q, want completed", st)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != "" {
		t.Fatalf("slot epoch = %q, want still cleared", epo)
	}
}

// TestCompleteSuccessfulRunForeignSuccessorUntouched (AC5/AC6 successor protection): a
// slot carrying a DIFFERENT nonempty RunEpochID (a successor that reserved after safe
// detachment) is left untouched, and the closeout still completes.
func TestCompleteSuccessfulRunForeignSuccessorUntouched(t *testing.T) {
	base := newCancelFixture(t, false) // no epoch-owned slot; worktree bound
	if _, err := base.store.ReserveWorktreeExecutionForEpoch(base.common, base.worktree, "successor-epoch", nil); err != nil {
		t.Fatalf("successor reserve: %v", err)
	}
	must(t, RegisterEpochParticipant(base.repo, base.key, base.epochID, EpochParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
	must(t, RecordEpochParticipantTerminal(base.repo, base.key, base.epochID, "turn-1", "t1", ParticipantTerminalCompleted))
	seams := cancelSeams{store: base.store,
		observer:       &fakeProcessObserver{defaultProven: true},
		launchObserver: &fakeLaunchObserver{report: gatedrive.EpochLaunchReport{Accounted: true}}}
	ok, reason, findings := completeSuccessfulRun(seams, base.repo, base.key)
	if !ok {
		t.Fatalf("ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadEpochState(t, base.repo, base.key); st != EpochCompleted {
		t.Fatalf("epoch state = %q, want completed", st)
	}
	if epo := loadSlotEpoch(t, base.store, base.worktree); epo != "successor-epoch" {
		t.Fatalf("successor slot epoch = %q, want untouched successor-epoch", epo)
	}
	if st := loadSlotState(t, base.store, base.worktree); st != "reserved" {
		t.Fatalf("successor slot state = %q, want reserved (untouched)", st)
	}
}

// TestStandaloneFinalizeAdmissionBlockedThenAdmittedAroundCloseout is AC1/AC2's
// end-to-end integration pin: a standalone finalize gate's worktree admission is
// REFUSED before the successful closeout and ADMITTED after it, at the exact
// admission shape the finalize path composes (GateLaunch reserves via the store's
// standalone entrypoint ReserveRawWorktreeExecution). Before closeout the released
// but still epoch-owned slot presents the finalize gate's empty epoch to the
// reserveWorktreeExecution "RunEpochID != rec.RunEpochID" fence and is refused
// ErrStaleRunEpoch (omission cannot detach a workflow-owned worktree). After
// completeSuccessfulRun retires the slot the same epoch-less reservation admits, and
// a following admitWorkflowMutation on the worktree is unfenced (a usable done
// callback) — proving later workflow mutations on that worktree are not trapped by
// the retired epoch.
func TestStandaloneFinalizeAdmissionBlockedThenAdmittedAroundCloseout(t *testing.T) {
	fx := newCompletionFixture(t)

	// BEFORE closeout: the standalone finalize gate's admission shape presents an
	// empty epoch to a slot the run epoch still owns (released, between drives) and is
	// refused stale-run-epoch.
	if _, err := fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil); func() bool {
		oe, ok := gatedrive.AsOwnershipError(err)
		return !ok || oe.Kind != gatedrive.ErrStaleRunEpoch
	}() {
		t.Fatalf("before closeout: raw reserve must refuse stale-run-epoch, got %v", err)
	}

	// Close out the verified successful run.
	if ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key); !ok {
		t.Fatalf("closeout ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCompleted {
		t.Fatalf("epoch state = %q, want completed", st)
	}

	// AFTER closeout: the same epoch-less reservation now admits (the slot was
	// detached from its retired epoch); release it back so the worktree is idle.
	token, err := fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil)
	if err != nil {
		t.Fatalf("after closeout: raw reserve must admit, got %v", err)
	}
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, token); err != nil {
		t.Fatalf("release standalone reservation: %v", err)
	}

	// A subsequent workflow mutation on the worktree is unfenced: the retired epoch no
	// longer owns it, so admitWorkflowMutation returns a usable done callback.
	done, err := admitWorkflowMutation(fx.worktree, "pr.publish")
	if err != nil || done == nil {
		t.Fatalf("admitWorkflowMutation after closeout: done=%v err=%v, want a usable callback", done, err)
	}
	done(mutationStatusCompleted)
}

// TestOrdinaryReleaseStillRetainsEpochBetweenDrives is AC8's ordinary-release fence
// probe: ReleaseWorktreeExecution on an epoch-owned slot leaves RunEpochID intact, so
// a foreign/epoch-less reserve BETWEEN drives is still refused ErrStaleRunEpoch. Only
// the attributed successful closeout (or an explicit cancellation) detaches the epoch;
// a plain between-drives release never does. This pins the fence the change must NOT
// weaken.
func TestOrdinaryReleaseStillRetainsEpochBetweenDrives(t *testing.T) {
	fx := newCancelFixture(t, true) // confirmed epoch-owned slot
	token := slotReservationToken(t, fx.store, fx.worktree)
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, token); err != nil {
		t.Fatalf("release: %v", err)
	}
	if epo := loadSlotEpoch(t, fx.store, fx.worktree); epo != fx.epochID {
		t.Fatalf("released slot RunEpochID = %q, want retained %q", epo, fx.epochID)
	}
	if st := loadSlotState(t, fx.store, fx.worktree); st != "released" {
		t.Fatalf("slot state = %q, want released", st)
	}
	// A between-drives foreign (epoch-less) reserve is still fenced.
	if _, err := fx.store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil); func() bool {
		oe, ok := gatedrive.AsOwnershipError(err)
		return !ok || oe.Kind != gatedrive.ErrStaleRunEpoch
	}() {
		t.Fatalf("foreign reserve between drives must refuse stale-run-epoch, got %v", err)
	}
}

// countFinding returns how many times token appears in findings.
func countFinding(findings []string, token string) int {
	n := 0
	for _, f := range findings {
		if f == token {
			n++
		}
	}
	return n
}

// TestCompleteSuccessfulRunDoesNotDuplicateFindings pins the diagnostic-noise fix
// (change 0441 review finding): a participant or mutation that stays unsettled across
// step (3)'s fenced-record accounting and step (4)'s reload re-enumeration must appear
// exactly ONCE in the operator-facing CompletionFindings, not twice — while the
// closeout still fails closed (ok=false, completion-unaccounted).
func TestCompleteSuccessfulRunDoesNotDuplicateFindings(t *testing.T) {
	t.Run("participant", func(t *testing.T) {
		fx := newCompletionFixture(t)
		// A registered native participant with no terminal evidence: unsettled on both reads.
		must(t, RegisterEpochParticipant(fx.repo, fx.key, fx.epochID,
			EpochParticipant{Kind: "coordinator", NativeHandle: "turn-2"}))
		ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
		if ok || reason != "completion-unaccounted" {
			t.Fatalf("ok=%v reason=%q, want false/completion-unaccounted (findings=%v)", ok, reason, findings)
		}
		if got := countFinding(findings, "participant-unobserved:coordinator"); got != 1 {
			t.Fatalf("participant-unobserved:coordinator appears %d times, want exactly 1 (findings=%v)", got, findings)
		}
	})
	t.Run("mutation", func(t *testing.T) {
		fx := newCompletionFixture(t)
		must(t, epochCAS(fx.repo, fx.key, func(r *EpochRecord) error {
			r.AdmittedMutations = []AdmittedMutation{{OpKey: "pr.publish", Status: "admitted"}}
			return nil
		}))
		ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
		if ok || reason != "completion-unaccounted" {
			t.Fatalf("ok=%v reason=%q, want false/completion-unaccounted (findings=%v)", ok, reason, findings)
		}
		if got := countFinding(findings, "mutation-pending:pr.publish"); got != 1 {
			t.Fatalf("mutation-pending:pr.publish appears %d times, want exactly 1 (findings=%v)", got, findings)
		}
	})
}

// --- change 0446 Task 8: durable completion facts survive scratch cleanup (spec §5,
// AC6). A released owned slot, an exact matching released slot, or a persisted
// PASSED/FAILED drive record is the sufficient durable proof of an execution's
// teardown; deleting the run's optional scratch directory must not reopen it. HALTED
// is never that proof, and the absence of any durable record still blocks. ---

// errScratchGone stands in for the production observer's failure on a run whose
// scratch directory was removed: process.Observe cannot read what no longer exists.
var errScratchGone = errors.New("run dir removed")

// scratchObserver models the production processObserver over real scratch: a run
// whose directory exists is proven terminal (it finished), a run whose directory
// is gone cannot be observed at all. It records every handle it was asked about.
type scratchObserver struct{ calls []string }

func (o *scratchObserver) observeProcessTerminal(runDir string) (bool, error) {
	o.calls = append(o.calls, runDir)
	if _, err := os.Stat(runDir); err != nil {
		return false, errScratchGone
	}
	return true, nil
}

// seedDriveRecord writes one drive record directly into the repository's drive
// registry (the executable schema, 4) naming runDir as its current raw run and
// outcome as its persisted LastOutcome. It stands in for a drive the supervisor
// already committed terminal; only the fields the durable-proof read keys on are set.
func seedDriveRecord(t *testing.T, common, id, worktree, runDir string, outcome gatedrive.Outcome) {
	t.Helper()
	dir := filepath.Join(common, "docket", "gate-drives", "v1", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir drive dir: %v", err)
	}
	doc := map[string]any{
		"generation": "g-" + id,
		"record": map[string]any{
			"schema_version": 4,
			"repo_identity":  common,
			"worktree_path":  worktree,
			"raw_run_dir":    runDir,
			"last_outcome":   string(outcome),
		},
	}
	buf, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal drive record: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "record.json"), buf, 0o600); err != nil {
		t.Fatalf("write drive record: %v", err)
	}
}

// TestCompletionSlotReleasedOwnedNoReobservation: a RELEASED slot this epoch owns is
// itself the durable proof of that slot's execution. Its run directory has been
// deleted (scratch cleanup), so re-observing the process could only fail — and the
// closeout must not reopen it: the slot leg is accounted and the observer is never
// asked about the slot's run. Before the fix the re-observation turned the deleted
// scratch into a process-unobserved blocker.
func TestCompletionSlotReleasedOwnedNoReobservation(t *testing.T) {
	fx := newCompletionFixture(t)
	if _, err := os.Stat(fx.runDir); !os.IsNotExist(err) {
		t.Fatalf("precondition: the slot's run dir %q must be absent (err=%v)", fx.runDir, err)
	}
	observer := &scratchObserver{}
	seams := fx.seams()
	seams.observer = observer

	blocked, findings := accountCompletionSlot(seams, EpochRecord{EpochID: fx.epochID, Worktree: fx.worktree})
	if blocked {
		t.Fatalf("a released owned slot blocked on deleted scratch: findings=%v", findings)
	}
	if len(observer.calls) != 0 {
		t.Fatalf("the released owned slot's run was re-observed: %v", observer.calls)
	}

	ok, reason, cfindings := completeSuccessfulRun(seams, fx.repo, fx.key)
	if !ok {
		t.Fatalf("closeout ok=false reason=%q findings=%v", reason, cfindings)
	}
	if len(observer.calls) != 0 {
		t.Fatalf("closeout re-observed a run after scratch cleanup: %v", observer.calls)
	}
}

// TestCompletionUnreleasedOwnedSlotStillBlocks: an owned slot still EXECUTING is a
// live obligation — no durable release exists, so the slot leg blocks
// slot-not-released exactly as before (unchanged safety).
func TestCompletionUnreleasedOwnedSlotStillBlocks(t *testing.T) {
	base := newCancelFixture(t, true) // epoch-owned slot left executing
	seams := cancelSeams{store: base.store, observer: &scratchObserver{},
		launchObserver: &fakeLaunchObserver{report: gatedrive.EpochLaunchReport{Accounted: true}}}
	blocked, findings := accountCompletionSlot(seams, EpochRecord{EpochID: base.epochID, Worktree: base.worktree})
	if !blocked || !hasFinding(findings, "slot-not-released") {
		t.Fatalf("blocked=%v findings=%v, want blocked slot-not-released", blocked, findings)
	}
}

// TestCompletionParticipantDurableProof: an execution participant whose direct
// observation fails because its scratch is gone is accounted by an EXACT matching
// durable record — the released slot recording that run, or the one persisted
// PASSED/FAILED drive naming it. HALTED, a missing record, an ambiguous record, a
// released slot recording a different run, an unreleased slot, and a live
// observation all keep it blocking.
func TestCompletionParticipantDurableProof(t *testing.T) {
	const (
		idA = "0446cccccccccccccccccccccccccc01"
		idB = "0446cccccccccccccccccccccccccc02"
	)
	type setup func(t *testing.T, fx completionFixture) (handle string, seams cancelSeams)
	gone := func(fx completionFixture) cancelSeams {
		s := fx.seams()
		s.observer = &fakeProcessObserver{err: errScratchGone}
		return s
	}
	rows := []struct {
		name    string
		blocked bool
		build   setup
	}{
		{"released slot records the run", false, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			return fx.runDir, gone(fx)
		}},
		{"persisted PASSED drive", false, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			h := filepath.Join(fx.worktree, "run-P")
			seedDriveRecord(t, fx.common, idA, fx.worktree, h, gatedrive.PASSED)
			return h, gone(fx)
		}},
		{"persisted FAILED drive", false, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			h := filepath.Join(fx.worktree, "run-F")
			seedDriveRecord(t, fx.common, idA, fx.worktree, h, gatedrive.FAILED)
			return h, gone(fx)
		}},
		{"HALTED drive is never execution proof", true, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			h := filepath.Join(fx.worktree, "run-H")
			seedDriveRecord(t, fx.common, idA, fx.worktree, h, gatedrive.HALTED)
			return h, gone(fx)
		}},
		{"WAITING drive is not terminal", true, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			h := filepath.Join(fx.worktree, "run-W")
			seedDriveRecord(t, fx.common, idA, fx.worktree, h, gatedrive.WAITING)
			return h, gone(fx)
		}},
		{"no durable record", true, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			return filepath.Join(fx.worktree, "run-none"), gone(fx)
		}},
		{"ambiguous drive records", true, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			h := filepath.Join(fx.worktree, "run-2")
			seedDriveRecord(t, fx.common, idA, fx.worktree, h, gatedrive.PASSED)
			seedDriveRecord(t, fx.common, idB, fx.worktree, h, gatedrive.PASSED)
			return h, gone(fx)
		}},
		{"released slot records a different run", true, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			return filepath.Join(fx.worktree, "run-other"), gone(fx)
		}},
		{"live observation contradicts a PASSED record", true, func(t *testing.T, fx completionFixture) (string, cancelSeams) {
			h := filepath.Join(fx.worktree, "run-L")
			seedDriveRecord(t, fx.common, idA, fx.worktree, h, gatedrive.PASSED)
			s := fx.seams()
			s.observer = &fakeProcessObserver{defaultProven: false}
			return h, s
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			fx := newCompletionFixture(t)
			handle, seams := r.build(t, fx)
			ep := EpochRecord{EpochID: fx.epochID, Worktree: fx.worktree,
				Participants: []EpochParticipant{{Kind: participantKindRawRun, NativeHandle: handle}}}
			blocked, findings := accountCompletionParticipants(seams, ep)
			if blocked != r.blocked {
				t.Fatalf("blocked=%v findings=%v, want blocked=%v", blocked, findings, r.blocked)
			}
			if r.blocked && len(findings) == 0 {
				t.Fatal("a blocked participant must name its unsettled run")
			}
		})
	}

	t.Run("unreleased slot recording the run", func(t *testing.T) {
		base := newCancelFixture(t, true) // slot still executing base.runDir
		seams := cancelSeams{store: base.store, observer: &fakeProcessObserver{err: errScratchGone}}
		ep := EpochRecord{EpochID: base.epochID, Worktree: base.worktree,
			Participants: []EpochParticipant{{Kind: participantKindGateScope, NativeHandle: base.runDir}}}
		if blocked, findings := accountCompletionParticipants(seams, ep); !blocked {
			t.Fatalf("an unreleased slot was accepted as execution proof: findings=%v", findings)
		}
	})
}

// TestCompleteThenScratchCleanupThenFinalizeAdmits (AC6): a successful run whose
// first closeout was held by an in-flight mutation has its optional scratch removed
// before the closeout is repeated; the repeat still completes on the durable release
// facts. Then — with a cancelled, never-superseded predecessor epoch bound to the
// same path in a directory that sorts first, and unrelated damaged drive and epoch
// history present — the finalize gate's admission on that worktree, composed exactly
// as GateLaunch composes it, admits.
func TestCompleteThenScratchCleanupThenFinalizeAdmits(t *testing.T) {
	fx := newCompletionFixture(t)
	if err := os.MkdirAll(fx.runDir, 0o755); err != nil {
		t.Fatalf("create the run's scratch: %v", err)
	}
	must(t, RegisterEpochParticipant(fx.repo, fx.key, fx.epochID,
		EpochParticipant{Kind: participantKindGateScope, NativeHandle: fx.runDir}))
	seams := fx.seams()
	seams.observer = &scratchObserver{}

	// The first closeout is held by a genuinely owned in-flight mutation.
	seedPendingEpochMutation(t, fx.repo, fx.key)
	if ok, reason, findings := completeSuccessfulRun(seams, fx.repo, fx.key); ok || !hasFinding(findings, "mutation-pending:") {
		t.Fatalf("first closeout ok=%v reason=%q findings=%v, want held by mutation-pending", ok, reason, findings)
	}

	// Scratch cleanup, then the mutation settles and the closeout is repeated.
	if err := os.RemoveAll(fx.runDir); err != nil {
		t.Fatalf("remove scratch: %v", err)
	}
	reconcilePendingEpochMutations(t, fx.repo, fx.key)
	if ok, reason, findings := completeSuccessfulRun(seams, fx.repo, fx.key); !ok {
		t.Fatalf("closeout after scratch cleanup ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCompleted {
		t.Fatalf("epoch state = %q, want completed", st)
	}

	// A cancelled never-superseded predecessor bound to the same path sorts first.
	seedNamedEpoch(t, fx.repo, "0000-cancelled-predecessor", fx.worktree, EpochCancelled)
	// Unrelated damaged history: a corrupt drive record and a corrupt epoch record.
	badDrive := filepath.Join(fx.common, "docket", "gate-drives", "v1", "0446dddddddddddddddddddddddddd01")
	if err := os.MkdirAll(badDrive, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDrive, "record.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	badEpoch := filepath.Join(fx.common, "docket", "rungate", "ffff-damaged-unrelated")
	if err := os.MkdirAll(badEpoch, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badEpoch, epochRecordFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Finalize's gate admission, composed as GateLaunch composes it.
	store := gatedrive.OpenStore(fx.common)
	store.SetEpochSettledResolver(epochSettledResolver(fx.common))
	if refusal, refused := rawStaleEpochRefusal(store, fx.worktree); refused {
		t.Fatalf("finalize admission refused at the epoch fence: %+v", refusal)
	}
	token, err := store.ReserveRawWorktreeExecution(fx.common, fx.worktree, nil)
	if err != nil {
		t.Fatalf("finalize gate admission on the completed worktree refused: %v", err)
	}
	if err := store.ReleaseWorktreeExecution(fx.worktree, token); err != nil {
		t.Fatalf("release finalize reservation: %v", err)
	}
}
