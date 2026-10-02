//go:build integration

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// These are the successful-run ownership closeout tests (change 0441 Task 7).
// completeSuccessfulRun is the observation-only counterpart of runCancel: on a
// verified keyed run-complete it fences the run completing, proves every registered
// obligation terminal WITHOUT stopping anything (the run's drives through the
// observe-only launch census, attributed by its context hash — change 0490), and
// CASes completing→completed — failing closed on any unsettled obligation and
// never relabelling a cancelled/superseded run successful. The tests drive the flow
// over faked observation seams and a real gatedrive store, reusing the run-cancel
// fixtures in runtracker_cancel_helpers_test.go.

// completionFixture is one prepared successfully-finished run: a fully authorized
// active run bound to a worktree, a registered coordinator native task carrying
// recorded terminal evidence, every mutation completed, and permissive observation
// seams (every process proven terminal, every launch accounted).
type completionFixture struct {
	repo, key, runID, worktree, runDir, common, contextHash string
	store                                                   *gatedrive.Store
	observer                                                *fakeProcessObserver
	launchObserver                                          *fakeLaunchObserver
}

func (f completionFixture) seams() cancelSeams {
	return cancelSeams{store: f.store, observer: f.observer, launchObserver: f.launchObserver}
}

func newCompletionFixture(t *testing.T) completionFixture {
	t.Helper()
	base := newCancelFixture(t)
	must(t, RegisterRunParticipant(base.repo, base.key, base.runID,
		RunParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
	must(t, RecordRunParticipantTerminal(base.repo, base.key, base.runID,
		"turn-1", "t1", ParticipantTerminalCompleted))
	return completionFixture{
		repo: base.repo, key: base.key, runID: base.runID, worktree: base.worktree,
		runDir: base.runDir, common: base.common, contextHash: base.contextHash, store: base.store,
		observer:       &fakeProcessObserver{defaultProven: true},
		launchObserver: &fakeLaunchObserver{report: gatedrive.RunLaunchReport{Accounted: true}},
	}
}

// TestIntegrationRunCompletionCompleteSuccessfulRunHappyPath: a fully settled run
// closes out — ok, run RunCompleted, and the observe-only census run once for the
// run's context hash.
func TestIntegrationRunCompletionCompleteSuccessfulRunHappyPath(t *testing.T) {
	fx := newCompletionFixture(t)
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	if calls := fx.launchObserver.calls; len(calls) != 1 || calls[0] != fx.contextHash {
		t.Fatalf("census calls = %v, want [%s] (the run's context hash)", calls, fx.contextHash)
	}
}

// TestIntegrationRunCompletionCompleteSuccessfulRunIdempotentReplay: a second closeout of a completed run is
// a no-op success — the stored run generation is byte-stable across the replay.
func TestIntegrationRunCompletionCompleteSuccessfulRunIdempotentReplay(t *testing.T) {
	fx := newCompletionFixture(t)
	if ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key); !ok {
		t.Fatalf("first closeout ok=false reason=%q findings=%v", reason, findings)
	}
	_, genBefore, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("load gen before: %v", err)
	}
	if ok, reason, _ := completeSuccessfulRun(fx.seams(), fx.repo, fx.key); !ok {
		t.Fatalf("replay ok=false reason=%q", reason)
	}
	_, genAfter, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("load gen after: %v", err)
	}
	if genAfter != genBefore {
		t.Fatalf("idempotent replay wrote: generation %q -> %q", genBefore, genAfter)
	}
}

// TestIntegrationRunCompletionCompleteSuccessfulRunNeverRelabelsCancellation: a cancelling/cancelled run is
// run-cancelled, a superseded run is stale-run-id, and the run state is never
// rewritten to a successful one.
func TestIntegrationRunCompletionCompleteSuccessfulRunNeverRelabelsCancellation(t *testing.T) {
	cases := []struct {
		state  runState
		reason string
	}{
		{RunCancelling, "run-cancelled"},
		{RunCancelled, "run-cancelled"},
		{RunSuperseded, "stale-run-id"},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			fx := newCompletionFixture(t)
			forceRunState(t, fx.repo, fx.key, tc.state)
			ok, reason, _ := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
			if ok {
				t.Fatalf("state %q closed out successfully", tc.state)
			}
			if reason != tc.reason {
				t.Fatalf("reason = %q, want %q", reason, tc.reason)
			}
			if st := loadRunState(t, fx.repo, fx.key); st != tc.state {
				t.Fatalf("state %q was relabelled to %q", tc.state, st)
			}
		})
	}
}

// TestIntegrationRunCompletionCompleteSuccessfulRunBlocksOnEveryUnsettledObligation (AC3): each unsettled
// obligation fails closed — ok false, reason completion-unaccounted, the named
// finding present, and the run left durably completing.
func TestIntegrationRunCompletionCompleteSuccessfulRunBlocksOnEveryUnsettledObligation(t *testing.T) {
	// each build returns the seams and the located run; the run begins active so the
	// closeout drives the real completing fence before it blocks.
	type row struct {
		name    string
		finding string
		build   func(t *testing.T) (seams cancelSeams, repo, key, runID, worktree string)
	}
	rows := []row{
		{"native-participant-unobserved", "participant-unobserved:coordinator", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: "coordinator", NativeHandle: "turn-2"}))
			return fx.seams(), fx.repo, fx.key, fx.runID, fx.worktree
		}},
		{"live-execution-participant", "process-live:exec-1", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: "raw-run", NativeHandle: "exec-1"}))
			fx.observer.defaultProven = false
			return fx.seams(), fx.repo, fx.key, fx.runID, fx.worktree
		}},
		{"nil-observer", "process-observer-unavailable", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			// The nil observer is exercised through the participant pass.
			must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID, RunParticipant{Kind: "raw-run", NativeHandle: "exec-1"}))
			s := fx.seams()
			s.observer = nil
			return s, fx.repo, fx.key, fx.runID, fx.worktree
		}},
		{"launch-not-accounted", "claim-busy:d9", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			fx.launchObserver.report = gatedrive.RunLaunchReport{Accounted: false, Findings: []string{"claim-busy:d9"}}
			return fx.seams(), fx.repo, fx.key, fx.runID, fx.worktree
		}},
		{"nil-launch-observer", "launch-observer-unavailable", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			s := fx.seams()
			s.launchObserver = nil
			return s, fx.repo, fx.key, fx.runID, fx.worktree
		}},
		{"mutation-pending", "mutation-pending:pr.publish", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			must(t, runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
				r.AdmittedMutations = []AdmittedMutation{{OpKey: "pr.publish", Status: "admitted"}}
				return nil
			}))
			return fx.seams(), fx.repo, fx.key, fx.runID, fx.worktree
		}},
		{"run-live-drive", "run-live:d7", func(t *testing.T) (cancelSeams, string, string, string, string) {
			fx := newCompletionFixture(t)
			fx.launchObserver.report = gatedrive.RunLaunchReport{Accounted: false, Findings: []string{"run-live:d7"}}
			return fx.seams(), fx.repo, fx.key, fx.runID, fx.worktree
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			seams, repo, key, _, _ := r.build(t)
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
			if st := loadRunState(t, repo, key); st != RunCompleting {
				t.Fatalf("run state = %q, want completing (success fence held)", st)
			}
		})
	}
}

// TestIntegrationRunCompletionCompleteSuccessfulRunSendsNoStops (AC3): closeout observes only — it never calls
// the stop-capable stopper, native-canceller, or reconcile (stop) seam, on either the
// happy path or a blocked path.
func TestIntegrationRunCompletionCompleteSuccessfulRunSendsNoStops(t *testing.T) {
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
	fx2.launchObserver.report = gatedrive.RunLaunchReport{Accounted: false, Findings: []string{"claim-busy:x"}}
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

// TestIntegrationRunCompletionCompleteSuccessfulRunLateParticipantBlocks (AC3 "late participant"): a
// participant appended after the accounting snapshot but before the completing
// write is caught by re-enumeration, blocking the closeout.
func TestIntegrationRunCompletionCompleteSuccessfulRunLateParticipantBlocks(t *testing.T) {
	fx := newCompletionFixture(t)
	// The observer proves no execution terminal.
	fx.observer.defaultProven = false
	// During the accounting pass's launch observation, a pre-fence launch registers a
	// terminal-unproven execution participant directly (the completing fence would
	// refuse the normal registration path — this stands in for that race).
	fx.launchObserver.onObserve = func() {
		_ = runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
			r.Participants = append(r.Participants, RunParticipant{Kind: "raw-run", NativeHandle: "LATE", RegisteredAt: "x"})
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
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleting {
		t.Fatalf("run state = %q, want completing (blocked, fence held)", st)
	}
}

// TestIntegrationRunCompletionCompleteSuccessfulRunReplayBeforeCompleteConverges
// (AC6 "interruption before the completing write"): a crash before the
// completing→completed CAS leaves the run completing; a replay re-proves the
// obligations and finishes to completed.
func TestIntegrationRunCompletionCompleteSuccessfulRunReplayBeforeCompleteConverges(t *testing.T) {
	fx := newCompletionFixture(t)
	forceRunState(t, fx.repo, fx.key, RunCompleting)
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("replay ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
}

// TestIntegrationRunCompletionCloseoutLeavesLaterMutationsUnfenced (AC1/AC2): after
// the successful closeout a workflow mutation on the run's worktree is unfenced —
// the completed run no longer owns it, so admitWorkflowMutation returns a usable
// done callback — proving later workflow mutations on that worktree are not trapped
// by the finished run.
func TestIntegrationRunCompletionCloseoutLeavesLaterMutationsUnfenced(t *testing.T) {
	fx := newCompletionFixture(t)
	if ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key); !ok {
		t.Fatalf("closeout ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	done, err := admitWorkflowMutation(fx.worktree, "pr.publish", nil)
	if err != nil || done == nil {
		t.Fatalf("admitWorkflowMutation after closeout: done=%v err=%v, want a usable callback", done, err)
	}
	done(mutationStatusCompleted, false)
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if len(ep.AdmittedMutations) != 0 {
		t.Fatalf("the completed run journaled a later mutation: %+v", ep.AdmittedMutations)
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

// TestIntegrationRunCompletionCompleteSuccessfulRunDoesNotDuplicateFindings pins the diagnostic-noise fix
// (change 0441 review finding): a participant or mutation that stays unsettled across
// step (3)'s fenced-record accounting and step (4)'s reload re-enumeration must appear
// exactly ONCE in the operator-facing CompletionFindings, not twice — while the
// closeout still fails closed (ok=false, completion-unaccounted).
func TestIntegrationRunCompletionCompleteSuccessfulRunDoesNotDuplicateFindings(t *testing.T) {
	t.Run("participant", func(t *testing.T) {
		fx := newCompletionFixture(t)
		// A registered native participant with no terminal evidence: unsettled on both reads.
		must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID,
			RunParticipant{Kind: "coordinator", NativeHandle: "turn-2"}))
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
		must(t, runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
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
// AC6). A persisted PASSED/FAILED drive record is the sufficient durable proof of an
// execution's teardown; deleting the run's optional scratch directory must not
// reopen it. HALTED is never that proof, and the absence of any durable record
// still blocks. ---

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
	dir := filepath.Join(common, "docket", "gate-drives", "v2", id)
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

// TestIntegrationRunCompletionCompletionParticipantDurableProof: an execution participant whose direct
// observation fails because its scratch is gone is accounted by an EXACT matching
// durable record — the one persisted PASSED/FAILED drive naming it. HALTED, a
// missing record, an ambiguous record, and a live observation all keep it
// blocking.
func TestIntegrationRunCompletionCompletionParticipantDurableProof(t *testing.T) {
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
			ep := RunRecord{RunID: fx.runID, Worktree: fx.worktree,
				Participants: []RunParticipant{{Kind: participantKindRawRun, NativeHandle: handle}}}
			blocked, findings := accountCompletionParticipants(seams, ep)
			if blocked != r.blocked {
				t.Fatalf("blocked=%v findings=%v, want blocked=%v", blocked, findings, r.blocked)
			}
			if r.blocked && len(findings) == 0 {
				t.Fatal("a blocked participant must name its unsettled run")
			}
		})
	}
}

// TestIntegrationRunCompletionCompleteThenScratchCleanupThenFinalizeAdmits (AC6): a successful run whose
// first closeout was held by an in-flight mutation has its optional scratch removed
// before the closeout is repeated; the repeat still completes on the durable fact
// that its gate's drive PASSED. Then — with a cancelled, never-superseded
// predecessor run bound to the same path in a directory that sorts first, and
// unrelated damaged drive and run history present — the worktree's lock, the one
// admission every gate start (finalize's included) takes, is free.
func TestIntegrationRunCompletionCompleteThenScratchCleanupThenFinalizeAdmits(t *testing.T) {
	fx := newCompletionFixture(t)
	if err := os.MkdirAll(fx.runDir, 0o755); err != nil {
		t.Fatalf("create the run's scratch: %v", err)
	}
	// The run's gate drive PASSED, naming the run's scratch as its run dir.
	seedDriveRecord(t, fx.common, "0446cccccccccccccccccccccccccc09", fx.worktree, fx.runDir, gatedrive.PASSED)
	must(t, RegisterRunParticipant(fx.repo, fx.key, fx.runID,
		RunParticipant{Kind: participantKindGateScope, NativeHandle: fx.runDir}))
	seams := fx.seams()
	seams.observer = &scratchObserver{}

	// The first closeout is held by a genuinely owned in-flight mutation.
	seedPendingRunMutation(t, fx.repo, fx.key)
	if ok, reason, findings := completeSuccessfulRun(seams, fx.repo, fx.key); ok || !hasFinding(findings, "mutation-pending:") {
		t.Fatalf("first closeout ok=%v reason=%q findings=%v, want held by mutation-pending", ok, reason, findings)
	}

	// Scratch cleanup, then the mutation settles and the closeout is repeated.
	if err := os.RemoveAll(fx.runDir); err != nil {
		t.Fatalf("remove scratch: %v", err)
	}
	reconcilePendingRunMutations(t, fx.repo, fx.key)
	if ok, reason, findings := completeSuccessfulRun(seams, fx.repo, fx.key); !ok {
		t.Fatalf("closeout after scratch cleanup ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}

	// A cancelled never-superseded predecessor bound to the same path sorts first.
	seedNamedRun(t, fx.repo, "0000-cancelled-predecessor", fx.worktree, RunCancelled)
	// Unrelated damaged history: a corrupt drive record and a corrupt run record.
	badDrive := filepath.Join(fx.common, "docket", "gate-drives", "v2", "0446dddddddddddddddddddddddddd01")
	if err := os.MkdirAll(badDrive, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badDrive, "record.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	badRun := filepath.Join(fx.common, "docket", runTrackerDirName, "ffff-damaged-unrelated")
	if err := os.MkdirAll(badRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badRun, runRecordFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Finalize's gate admission is the worktree lock on the canonical root.
	root, store, ok := resolveWorktreeAdmission(fx.worktree)
	if !ok {
		t.Fatal("resolveWorktreeAdmission: the fixture worktree did not resolve")
	}
	lock, err := store.TryWorktreeLock(root, nil)
	if err != nil {
		t.Fatalf("finalize gate admission on the completed worktree refused: %v", err)
	}
	lock.Release()
}

// TestIntegrationRunCompletionCompleteSuccessfulRunSettlesUncertainPublication (change 0444 acceptance 3): the
// REAL attributed closeout path settles an uncertain publication proven by a later
// completed identical retry, then completes the run — surfacing the settlement token
// in the returned findings, and sending no stop, cancelling no native task, and never
// driving the stop-capable launch seam (the same no-stop proof as
// TestIntegrationRunCompletionCompleteSuccessfulRunSendsNoStops).
func TestIntegrationRunCompletionCompleteSuccessfulRunSettlesUncertainPublication(t *testing.T) {
	fx := newCompletionFixture(t)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	stopper := &fakeCancelStopper{proven: map[string]bool{}}
	native := &fakeNativeCanceller{}
	recon := okLaunchReconciler()
	seams := cancelSeams{store: fx.store, stopper: stopper, native: native, launches: recon,
		observer: fx.observer, launchObserver: fx.launchObserver}

	ok, reason, findings := completeSuccessfulRun(seams, fx.repo, fx.key)
	if !ok {
		t.Fatalf("closeout blocked: reason=%q findings=%v; a settled journal must complete", reason, findings)
	}
	if countFinding(findings, "mutation-settled:"+OperationWorkspacePublish) != 1 {
		t.Fatalf("findings = %v, want exactly one mutation-settled:%s surfaced", findings, OperationWorkspacePublish)
	}
	if hasFinding(findings, "mutation-pending") {
		t.Fatalf("findings = %v, must not report the settled mutation pending", findings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunCompleted {
		t.Fatalf("state = %q, want completed", ep.State)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusCompleted {
		t.Fatal("the original entry must be durably settled by the closeout")
	}
	if len(stopper.calls) != 0 || len(native.calls) != 0 || len(recon.calls) != 0 {
		t.Fatalf("closeout must stop nothing: stops=%v native=%v reconcile=%v",
			stopper.calls, native.calls, recon.calls)
	}
}

// TestIntegrationRunCompletionCompleteSuccessfulRunStillBlocksWithoutRetry (change 0444): an uncertain
// publication with no completed identical retry keeps the closeout blocked
// (completion-unaccounted) and the entry uncertain — change 0441's fail-closed
// accounting is not weakened by settlement.
func TestIntegrationRunCompletionCompleteSuccessfulRunStillBlocksWithoutRetry(t *testing.T) {
	fx := newCompletionFixture(t)
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
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if ok || reason != "completion-unaccounted" {
		t.Fatalf("ok=%v reason=%q findings=%v, want blocked completion-unaccounted", ok, reason, findings)
	}
	if !hasFinding(findings, "mutation-pending:"+OperationWorkspacePublish) {
		t.Fatalf("findings = %v, want mutation-pending:%s", findings, OperationWorkspacePublish)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.State != RunCompleting {
		t.Fatalf("state = %q, want completing (the success fence holds while blocked)", ep.State)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusUncertain {
		t.Fatalf("unmatched entry status = %q, want still uncertain", ep.AdmittedMutations[0].Status)
	}
}

// TestIntegrationRunCompletionCompleteSuccessfulRunBlocksOnUnverifiedRetry (change 0444 review blocker):
// an identical retry that completed WITHOUT verifying its postcondition (contended,
// refused, internally failed — journaled completed, verified false) is no evidence,
// so the attributed closeout stays blocked and the original stays uncertain.
func TestIntegrationRunCompletionCompleteSuccessfulRunBlocksOnUnverifiedRetry(t *testing.T) {
	fx := newCompletionFixture(t)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if ok || reason != "completion-unaccounted" {
		t.Fatalf("ok=%v reason=%q findings=%v, want blocked completion-unaccounted", ok, reason, findings)
	}
	if hasFinding(findings, "mutation-settled") {
		t.Fatalf("findings = %v; an unverified retry must settle nothing", findings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ep.AdmittedMutations[0].Status != mutationStatusUncertain {
		t.Fatalf("original status = %q, want still uncertain", ep.AdmittedMutations[0].Status)
	}
}

// TestIntegrationRunCompletionReadOnlyPathsNeverSettle (change 0444): the read-only verification predicates
// report the pending truth of a settleable pair but write NOTHING — the durable
// record is byte-identical after they run (RunVerify and unattributed verdicts consume
// these same predicates). Settlement is a write, and only cancellation and the
// attributed keyed closeout may write.
func TestIntegrationRunCompletionReadOnlyPathsNeverSettle(t *testing.T) {
	fx := newCancelFixture(t)
	desc := MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	if err := runRecordCAS(fx.repo, fx.key, func(r *RunRecord) error {
		r.State = RunCancelled
		r.AdmittedMutations = []AdmittedMutation{
			{OpKey: OperationWorkspacePublish, Status: mutationStatusUncertain, Publication: &desc},
			{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
	recPath := filepath.Join(fx.common, "docket", runTrackerDirName, fx.key, runRecordFileName)
	before, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	ep, _, lerr := LoadRunRecord(fx.repo, fx.key)
	if lerr != nil {
		t.Fatalf("LoadRunRecord: %v", lerr)
	}
	seams := cancelSeams{launches: okLaunchReconciler()}
	if quiescent, vf := verifyTerminalRunQuiescence(seams, fx.repo, ep); quiescent ||
		!hasFinding(vf, "mutation-pending:"+OperationWorkspacePublish) {
		t.Fatalf("verifyTerminalRunQuiescence = %v %v, want the unsettled entry reported pending", quiescent, vf)
	}
	if rok, detail := validateResumeQuiescence(seams, fx.repo, ep); rok {
		t.Fatalf("validateResumeQuiescence ok (detail %q), want the unsettled entry to block", detail)
	}
	after, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("re-read record: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("read-only verification paths must not write the run record")
	}
}
