package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/gatedrive"
	"github.com/danielhanold/docket/internal/testsupport"
)

// These are change 0446's AC5/AC6 flows over PRODUCTION seams (review finding: the
// earlier acceptance tests used permissive fake accounting — okLaunchReconciler,
// fakeLaunchObserver{Accounted:true} — and probed finalize admission with a bare
// ReserveRawWorktreeExecution, so no test ran the shared census or finalize's real
// admission with unrelated damaged history present). Here:
//
//   - cancellation, success closeout, and resume quiescence run through
//     productionCancelSeams: the real admission store, appLaunchReconciler /
//     appLaunchObserver (Driver.ReconcileEpochLaunches / ObserveEpochLaunches over the
//     real process service), appGateObserver, and appGateStopper;
//   - unrelated corrupt, unsupported-schema, obsolete (lost-linkage, rotated-token),
//     other-worktree, and HALTED drive records plus a corrupt unrelated epoch record are
//     seeded BEFORE cancel/closeout, so the census walks them;
//   - finalize is entered through NewFinalizeGateDriveService(...).Start — the
//     Driver.Start/Admit path finalize's processFinalizeGate uses — and the resumed
//     replacement's gate through NewBuildGateDriveService(...).Start carrying its run
//     epoch, each launching one real /bin/echo run to PASSED.

// seedCensusDrive writes one drive record (the executable schema 4 unless fields
// overrides schema_version) straight into the repository's drive registry. It stands
// in for history an earlier executor left behind; fields are merged over the base.
func seedCensusDrive(t *testing.T, common, id string, fields map[string]any) {
	t.Helper()
	rec := map[string]any{"schema_version": 4, "repo_identity": common}
	for k, v := range fields {
		rec[k] = v
	}
	buf, err := json.Marshal(map[string]any{"generation": "g-" + id, "record": rec})
	if err != nil {
		t.Fatalf("marshal drive record: %v", err)
	}
	writeCensusDriveBytes(t, common, id, buf)
}

func writeCensusDriveBytes(t *testing.T, common, id string, buf []byte) {
	t.Helper()
	dir := filepath.Join(common, "docket", "gate-drives", "v1", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir drive dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "record.json"), buf, 0o600); err != nil {
		t.Fatalf("write drive record: %v", err)
	}
}

// seedUnrelatedDamagedHistory seeds history with no ownership connection to the run
// epoch under test, including records bound to the SAME worktree path by earlier
// generations: a corrupt record, an unsupported-schema record, a HALTED drive on this
// worktree whose run dir is gone, a nonterminal scopeless drive whose admission token
// the slot no longer holds (rotated), a nonterminal scoped drive whose scope record is
// missing (lost linkage), a nonterminal drive bound to a removed other worktree, and a
// corrupt unrelated epoch record. prefix keeps the ids distinct across calls.
func seedUnrelatedDamagedHistory(t *testing.T, fx cancelFixture, prefix string) {
	t.Helper()
	id := func(n string) string { return prefix + "eeeeeeeeeeeeeeeeeeeeeeeeeeee" + n }
	gone := filepath.Join(testsupport.TempDir(t), "gone")
	writeCensusDriveBytes(t, fx.common, id("01"), []byte("{not json"))
	seedCensusDrive(t, fx.common, id("02"), map[string]any{"schema_version": 99, "worktree_path": fx.worktree})
	seedCensusDrive(t, fx.common, id("03"), map[string]any{
		"worktree_path": fx.worktree, "raw_run_dir": filepath.Join(gone, "halted-run"),
		"last_outcome": string(gatedrive.HALTED), "last_cause": "stopped-not-initiated",
	})
	seedCensusDrive(t, fx.common, id("04"), map[string]any{
		"worktree_path": fx.worktree, "raw_run_dir": filepath.Join(gone, "rotated-run"),
		"last_outcome": string(gatedrive.WAITING), "admission_token": "rotated-away-token",
	})
	seedCensusDrive(t, fx.common, id("05"), map[string]any{
		"worktree_path": fx.worktree, "raw_run_dir": filepath.Join(gone, "lost-scope-run"),
		"last_outcome": string(gatedrive.WAITING), "scope_id": prefix + "abababababababababababababab99",
	})
	seedCensusDrive(t, fx.common, id("06"), map[string]any{
		"worktree_path": filepath.Join(gone, "other-worktree"), "raw_run_dir": filepath.Join(gone, "other-run"),
		"last_outcome": string(gatedrive.WAITING),
	})
	badEpoch := filepath.Join(fx.common, "docket", "rungate", prefix+"-damaged-unrelated-epoch")
	if err := os.MkdirAll(badEpoch, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badEpoch, epochRecordFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// finalizeEffFor is a finalize-owned effective config carrying a resolved
// finalize.test_command and the observation budget, so the finalize service's Start
// clears the unresolved-command guard.
func finalizeEffFor(command string) config.Effective {
	eff := config.Effective{}
	eff.GateObservation = config.Value[int]{Value: 30, Provenance: config.Provenance{Layer: config.LayerRepository}}
	eff.Finalize.TestCommand = config.Value[string]{Value: command, Provenance: config.Provenance{Layer: config.LayerRepository}}
	return eff
}

// runDriveToTerminal advances a WAITING drive through svc until it leaves WAITING,
// returning the terminal outcome. /bin/echo finishes in milliseconds; the deadline is
// a safety net, never an expectation.
func runDriveToTerminal(t *testing.T, svc *GateDriveService, got GateDriveResult) gatedrive.Outcome {
	t.Helper()
	doc := got.Drive
	deadline := time.Now().Add(30 * time.Second)
	for doc.Outcome == gatedrive.WAITING {
		if time.Now().After(deadline) {
			t.Fatal("drive never left WAITING within 30s")
		}
		time.Sleep(10 * time.Millisecond)
		next := svc.Advance(doc.DriveID, doc.Generation)
		if next.Drive == nil {
			t.Fatalf("advance produced no drive document: result=%s reason=%q", next.Result, next.Reason)
		}
		doc = next.Drive
	}
	return doc.Outcome
}

// stopRunsUnder ends any run a drive left live under root (test cleanup only).
func stopRunsUnder(root string) {
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		GateStop(filepath.Join(root, e.Name()), "test cleanup")
	}
}

// startFinalizeGate enters finalize's local gate exactly as processFinalizeGate's
// RunLocalGate does for a fresh drive — the finalize-owned service's Start with the
// workspace as repo, worktree, and cwd, no epoch — and drives it to its terminal.
func startFinalizeGate(t *testing.T, fx cancelFixture) {
	t.Helper()
	svc, res, reason := NewFinalizeGateDriveService(fx.common, guardianExecutable(t), finalizeEffFor("/bin/echo finalize"))
	if svc == nil {
		t.Fatalf("finalize gate-drive service was nil: %s %s", res, reason)
	}
	runRoot := filepath.Join(testsupport.TempDir(t), "finalize-runs")
	t.Cleanup(func() { stopRunsUnder(runRoot) })
	got := svc.Start(GateDriveStartRequest{
		RepoDir: fx.worktree, Worktree: fx.worktree, ChangeID: "42",
		Phase: finalizeLocalGatePhase, Cwd: fx.worktree, RunRoot: runRoot,
		IdempotentSuiteGate: true,
	})
	if got.Result != ResultApplied || got.Drive == nil {
		t.Fatalf("finalize Start on the closed-out worktree refused: result=%s reason=%q stage=%q locator=%q message=%q",
			got.Result, got.Reason, got.Stage, got.Locator, got.Message)
	}
	if out := runDriveToTerminal(t, svc, got); out != gatedrive.PASSED {
		t.Fatalf("finalize drive outcome = %s, want PASSED", out)
	}
}

// prepareQuiescentRun is the shared arrangement: a real authorized epoch owning a
// worktree whose slot is RELEASED (its drives are done), a sorted-first cancelled
// never-superseded predecessor bound to the same path, and unrelated damaged history
// seeded before any closeout or cancellation runs.
func prepareQuiescentRun(t *testing.T) cancelFixture {
	t.Helper()
	requireProcessSupervisorHere(t)
	fx := newCancelFixture(t, true)
	slot, _, err := fx.store.LoadWorktreeExecution(fx.worktree)
	if err != nil {
		t.Fatalf("load slot: %v", err)
	}
	if err := fx.store.ReleaseWorktreeExecution(fx.worktree, slot.ReservationToken); err != nil {
		t.Fatalf("release slot: %v", err)
	}
	seedNamedEpoch(t, fx.repo, "0000-cancelled-predecessor", fx.worktree, EpochCancelled)
	seedUnrelatedDamagedHistory(t, fx, "a")
	return fx
}

// requireProcessSupervisorHere skips where the native process supervisor is not
// built (internal/process launches on darwin/linux only).
func requireProcessSupervisorHere(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "darwin", "linux":
	default:
		t.Skipf("native process supervisor is unsupported on %s", runtime.GOOS)
	}
}

// TestProductionCensusCompleteThenFinalize (AC6): the successful closeout runs the
// production observation census with unrelated damaged history present, the run's
// scratch is gone, and finalize then enters through its real gate-drive Start.
func TestProductionCensusCompleteThenFinalize(t *testing.T) {
	fx := prepareQuiescentRun(t)
	must(t, RegisterEpochParticipant(fx.repo, fx.key, fx.epochID,
		EpochParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
	must(t, RecordEpochParticipantTerminal(fx.repo, fx.key, fx.epochID,
		"turn-1", "t1", ParticipantTerminalCompleted))
	// The released slot's run is also a registered gate-scope participant whose
	// scratch directory no longer exists: the production observer cannot observe it,
	// so only the durable released-slot fact accounts it.
	must(t, RegisterEpochParticipant(fx.repo, fx.key, fx.epochID,
		EpochParticipant{Kind: participantKindGateScope, NativeHandle: fx.runDir}))
	if _, err := os.Stat(fx.runDir); !os.IsNotExist(err) {
		t.Fatalf("precondition: the run's scratch %q must be absent (err=%v)", fx.runDir, err)
	}

	ok, reason, findings := completeSuccessfulRun(productionCancelSeams(fx.repo), fx.repo, fx.key)
	if !ok {
		t.Fatalf("production closeout over unrelated history ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochCompleted {
		t.Fatalf("epoch state = %q, want completed", st)
	}
	startFinalizeGate(t, fx)
}

// TestProductionCensusCancelThenFinalize (AC5/AC6): an otherwise quiescent epoch
// cancels through the production reconciliation census with unrelated damaged
// history present — repeated cancellation converges — and finalize then enters
// through its real gate-drive Start.
func TestProductionCensusCancelThenFinalize(t *testing.T) {
	fx := prepareQuiescentRun(t)
	res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.epochID, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("production cancel over unrelated history = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if again := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.epochID, "human stop"); again.Disposition != CancelDispositionAlreadyCancelled {
		t.Fatalf("repeated cancel = %q, want already-cancelled (findings=%v)", again.Disposition, again.Findings)
	}
	startFinalizeGate(t, fx)
}

// TestProductionCensusCancelResumeStartsReplacementGate (AC5): after a production
// cancellation, NEW unrelated history lands, then resume re-proves quiescence through
// the production census, reserves exactly one replacement, and the replacement's
// build gate — carrying its run epoch — starts through the real service and passes.
func TestProductionCensusCancelResumeStartsReplacementGate(t *testing.T) {
	fx := prepareQuiescentRun(t)
	if res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, fx.epochID, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("production cancel = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	seedUnrelatedDamagedHistory(t, fx, "b") // history added between cancellation and resume

	oldEp, _, err := LoadEpochRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadEpochRecord: %v", err)
	}
	if ok, detail := validateResumeQuiescence(productionCancelSeams(fx.repo), fx.repo, oldEp, fx.worktree); !ok {
		t.Fatalf("production resume quiescence over unrelated history refused: %s", detail)
	}

	svc, res, reason := NewBuildGateDriveService(fx.common, guardianExecutable(t), buildEffWithMaxAttempts("/bin/echo build", 4))
	if svc == nil {
		t.Fatalf("build gate-drive service was nil: %s %s", res, reason)
	}
	sdeps := GateScopeDeps{Prepare: func(req gatedrive.ScopeRequest) (gatedrive.ScopeGrant, error) {
		g := svc.PrepareScope(req)
		if g.Result != ResultApplied {
			t.Fatalf("outer PrepareScope: %s (%s)", g.Result, g.Reason)
		}
		return gatedrive.ScopeGrant{ScopeID: g.ScopeID, ChildCapability: g.ChildCapability, ParentCapability: g.ParentCapability}, nil
	}}
	armed := armResumeReplacement(fx.repo, sdeps, fx.key, resumeReplacementParams{
		attributedID: 42, scopeChangeID: "42", branch: "fix/x", worktree: fx.worktree, attemptLimit: 2,
	})
	if !armed.Armed || armed.Epoch == "" {
		t.Fatalf("resume did not arm a replacement: %+v", armed)
	}
	if st := loadEpochState(t, fx.repo, fx.key); st != EpochSuperseded {
		t.Fatalf("old epoch state = %q, want superseded", st)
	}

	runRoot := filepath.Join(testsupport.TempDir(t), "build-runs")
	t.Cleanup(func() { stopRunsUnder(runRoot) })
	scope := svc.PrepareScope(gatedrive.ScopeRequest{
		RepoIdentity: fx.worktree, Worktree: fx.worktree, ChangeID: "42", TaskID: "task-1",
		Phase: "build", Branch: "fix/x", RunEpochID: armed.Epoch,
	})
	if scope.Result != ResultApplied {
		t.Fatalf("replacement PrepareScope: %s (%s)", scope.Result, scope.Reason)
	}
	got := svc.Start(GateDriveStartRequest{
		RepoDir: fx.worktree, Worktree: fx.worktree, ChangeID: "42", TaskID: "task-1",
		Phase: "build", Branch: "fix/x", Ref: "refs/heads/fix/x", Cwd: fx.worktree,
		RunRoot: runRoot, ScopeID: scope.ScopeID, ChildCapability: scope.ChildCapability,
		RunEpochID: armed.Epoch, IdempotentSuiteGate: true,
	})
	if got.Result != ResultApplied || got.Drive == nil {
		t.Fatalf("replacement gate Start refused: result=%s reason=%q stage=%q locator=%q message=%q",
			got.Result, got.Reason, got.Stage, got.Locator, got.Message)
	}
	if out := runDriveToTerminal(t, svc, got); out != gatedrive.PASSED {
		t.Fatalf("replacement drive outcome = %s, want PASSED", out)
	}
}
