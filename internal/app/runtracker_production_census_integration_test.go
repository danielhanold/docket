//go:build integration

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
// fakeLaunchObserver{Accounted:true} — so no test ran the shared census or
// finalize's real admission with unrelated damaged history present). Here:
//
//   - cancellation, success closeout, and resume quiescence run through
//     productionCancelSeams: the real gatedrive store, appLaunchReconciler /
//     appLaunchObserver (Driver.ReconcileRunLaunches / VerdictRunLaunches over the
//     real process service), appGateObserver, and appGateStopper;
//   - unrelated corrupt, unsupported-schema, obsolete (lost-linkage, rotated-token),
//     other-worktree, and HALTED drive records plus a corrupt unrelated run record are
//     seeded BEFORE cancel/closeout, so the census walks them; beside them sit the
//     run's OWN finished drives (carrying its run context, change 0490) and another
//     run's nonterminal drive, so the census attributes by run context for real
//     (seedRunContextDrives);
//   - finalize is entered through NewFinalizeGateDriveService(...).Start — the
//     Driver.Start/Admit path finalize's processFinalizeGate uses — and the resumed
//     replacement's gate through NewBuildGateDriveService(...).Start carrying its run
//     id, each launching one real /bin/echo run to PASSED.

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
	dir := filepath.Join(common, "docket", "gate-drives", "v2", id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir drive dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "record.json"), buf, 0o600); err != nil {
		t.Fatalf("write drive record: %v", err)
	}
}

// seedUnrelatedDamagedHistory seeds history with no ownership connection to the run
// under test, including records bound to the SAME worktree path by earlier
// generations: a corrupt record, an unsupported-schema record, a HALTED drive on this
// worktree whose run dir is gone, a nonterminal scopeless drive carrying a rotated
// launch token, an old nonterminal drive carrying scope_id (a
// pre-0489 task drive naming a missing scope, read as scopeless), a nonterminal drive
// bound to a removed other worktree, and a corrupt unrelated run record. prefix keeps
// the ids distinct across calls.
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
	badRun := filepath.Join(fx.common, "docket", runTrackerDirName, prefix+"-damaged-unrelated-run")
	if err := os.MkdirAll(badRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(badRun, runRecordFileName), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// seedRunContextDrives seeds drives the production census must attribute by run
// context (change 0490): the run's OWN drives — a PASSED one, a HALTED one, and a
// nonterminal one, each of whose run dirs was removed with its run root (clean
// absence: torn down) — and a nonterminal drive of ANOTHER run whose run dir exists
// but holds no supervisor. Attributing the other run's drive would make the
// production Observe of that dir fail and keep the run pending, so a green census
// proves it was never touched. prefix keeps the ids distinct across calls.
func seedRunContextDrives(t *testing.T, fx cancelFixture, prefix string) {
	t.Helper()
	id := func(n string) string { return prefix + "cccccccccccccccccccccccccccc" + n }
	gone := filepath.Join(testsupport.TempDir(t), "removed-run-root")
	for n, outcome := range map[string]gatedrive.Outcome{"01": gatedrive.PASSED, "02": gatedrive.HALTED, "03": gatedrive.WAITING} {
		seedCensusDrive(t, fx.common, id(n), map[string]any{
			"worktree_path": fx.worktree, "raw_run_dir": filepath.Join(gone, "run-"+n),
			"last_outcome": string(outcome), "run_context_hash": fx.contextHash,
		})
	}
	foreignRunDir := filepath.Join(testsupport.TempDir(t), "foreign-run")
	if err := os.MkdirAll(foreignRunDir, 0o700); err != nil {
		t.Fatalf("mkdir foreign run dir: %v", err)
	}
	seedCensusDrive(t, fx.common, id("04"), map[string]any{
		"worktree_path": fx.worktree, "raw_run_dir": foreignRunDir,
		"last_outcome": string(gatedrive.WAITING), "run_context_hash": runTrackerHashToken("another-run-context"),
	})
}

// neverLaunchedToken is the launch token a seeded never-launched drive carries:
// lowercase hex, as process.ResolveReservation requires.
const neverLaunchedToken = "0491aaaabbbbccccddddeeeeffff0000"

// seedNeverLaunchedDrive seeds the record a tracked gate.drive.start killed between
// Admit and StartAdmitted leaves behind (change 0491, spec Problem fact 5): reserved
// — no outcome, no run dir — carrying its launch token, owner generation, run root,
// and the run's context hash. extra merges further fields (change_id for the outer
// scan).
func seedNeverLaunchedDrive(t *testing.T, common, id, worktree, runRoot, contextHash string, extra map[string]any) {
	t.Helper()
	fields := map[string]any{
		"worktree_path":    worktree,
		"run_root":         runRoot,
		"admission_token":  neverLaunchedToken,
		"owner_generation": "orphan-owner-generation",
		"run_context_hash": contextHash,
	}
	for k, v := range extra {
		fields[k] = v
	}
	seedCensusDrive(t, common, id, fields)
}

// driveOutcome reads one drive's persisted outcome and cause.
func driveOutcome(t *testing.T, store *gatedrive.Store, id string) (gatedrive.Outcome, string) {
	t.Helper()
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("load drive %s: %v", id, err)
	}
	return rec.LastOutcome, rec.LastCause
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
// workspace as repo, worktree, and cwd, no run — and drives it to its terminal.
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
	})
	if got.Result != ResultApplied || got.Drive == nil {
		t.Fatalf("finalize Start on the closed-out worktree refused: result=%s reason=%q stage=%q locator=%q message=%q",
			got.Result, got.Reason, got.Stage, got.Locator, got.Message)
	}
	if out := runDriveToTerminal(t, svc, got); out != gatedrive.PASSED {
		t.Fatalf("finalize drive outcome = %s, want PASSED", out)
	}
}

// prepareQuiescentRun is the shared arrangement: a real authorized run owning a
// worktree whose drives are done (seedRunContextDrives), a sorted-first cancelled
// never-superseded predecessor bound to the same path, and unrelated damaged history
// seeded before any closeout or cancellation runs.
func prepareQuiescentRun(t *testing.T) cancelFixture {
	t.Helper()
	requireProcessSupervisorHere(t)
	fx := newCancelFixture(t)
	seedNamedRun(t, fx.repo, "0000-cancelled-predecessor", fx.worktree, RunCancelled)
	seedUnrelatedDamagedHistory(t, fx, "a")
	seedRunContextDrives(t, fx, "a")
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

// TestIntegrationRunCompletionProductionCensusCompleteThenFinalize (AC6): the successful closeout runs the
// production observation census with unrelated damaged history present, the run's
// scratch is gone, and finalize then enters through its real gate-drive Start.
func TestIntegrationRunCompletionProductionCensusCompleteThenFinalize(t *testing.T) {
	fx := prepareQuiescentRun(t)
	must(t, RegisterRunParticipant(fx.repo, fx.key,
		RunParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
	must(t, RecordRunParticipantTerminal(fx.repo, fx.key,
		"turn-1", "t1", ParticipantTerminalCompleted))
	// The run's gate is also a registered gate-scope participant whose scratch
	// directory no longer exists: the production observer cannot observe it, so only
	// the durable fact that its drive PASSED accounts it.
	seedCensusDrive(t, fx.common, "accccccccccccccccccccccccccccc09", map[string]any{
		"worktree_path": fx.worktree, "raw_run_dir": fx.runDir,
		"last_outcome": string(gatedrive.PASSED), "run_context_hash": fx.contextHash,
	})
	must(t, RegisterRunParticipant(fx.repo, fx.key,
		RunParticipant{Kind: participantKindGateScope, NativeHandle: fx.runDir}))
	if _, err := os.Stat(fx.runDir); !os.IsNotExist(err) {
		t.Fatalf("precondition: the run's scratch %q must be absent (err=%v)", fx.runDir, err)
	}

	ok, reason, findings := completeSuccessfulRun(productionCancelSeams(fx.repo), fx.repo, fx.key)
	if !ok {
		t.Fatalf("production closeout over unrelated history ok=false reason=%q findings=%v", reason, findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
	startFinalizeGate(t, fx)
}

// TestIntegrationRunCompletionProductionCensusCancelThenFinalize (AC5/AC6): an otherwise quiescent run
// cancels through the production reconciliation census with unrelated damaged
// history present — repeated cancellation converges — and finalize then enters
// through its real gate-drive Start.
func TestIntegrationRunCompletionProductionCensusCancelThenFinalize(t *testing.T) {
	fx := prepareQuiescentRun(t)
	res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("production cancel over unrelated history = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	if again := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, "human stop"); again.Disposition != CancelDispositionAlreadyCancelled {
		t.Fatalf("repeated cancel = %q, want already-cancelled (findings=%v)", again.Disposition, again.Findings)
	}
	startFinalizeGate(t, fx)
}

// TestIntegrationRunCompletionProductionCensusCancelResumeStartsReplacementGate (AC5): after a production
// cancellation, NEW unrelated history lands, then resume re-proves quiescence through
// the production census, reserves exactly one replacement, and the replacement's
// build gate — carrying its run — starts through the real service and passes.
func TestIntegrationRunCompletionProductionCensusCancelResumeStartsReplacementGate(t *testing.T) {
	fx := prepareQuiescentRun(t)
	if res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, "human stop"); res.Disposition != CancelDispositionCancelled {
		t.Fatalf("production cancel = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
	}
	seedUnrelatedDamagedHistory(t, fx, "b") // history added between cancellation and resume

	oldEp, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if ok, detail := validateResumeQuiescence(productionCancelSeams(fx.repo), fx.repo, oldEp); !ok {
		t.Fatalf("production resume quiescence over unrelated history refused: %s", detail)
	}

	svc, res, reason := NewBuildGateDriveService(fx.common, guardianExecutable(t), buildEffWithMaxAttempts("/bin/echo build", 4))
	if svc == nil {
		t.Fatalf("build gate-drive service was nil: %s %s", res, reason)
	}
	sdeps := RunTrackerScopeDeps{Prepare: gatedrive.OpenStore(fx.common).PrepareScope}
	started := armResumeReplacement(fx.repo, sdeps, fx.key, resumeReplacementParams{
		attributedID: 42, scopeChangeID: "42", branch: "fix/x", worktree: fx.worktree, attemptLimit: 2,
	})
	if !started.Started || started.Key == "" {
		t.Fatalf("resume did not start a replacement: %+v", started)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunSuperseded {
		t.Fatalf("old run state = %q, want superseded", st)
	}

	runRoot := filepath.Join(testsupport.TempDir(t), "build-runs")
	t.Cleanup(func() { stopRunsUnder(runRoot) })
	got := svc.Start(GateDriveStartRequest{
		RepoDir: fx.worktree, Worktree: fx.worktree, ChangeID: "42", TaskID: "task-1",
		Phase: "build", Branch: "fix/x", Ref: "refs/heads/fix/x", Cwd: fx.worktree,
		RunRoot: runRoot,
	})
	if got.Result != ResultApplied || got.Drive == nil {
		t.Fatalf("replacement gate Start refused: result=%s reason=%q stage=%q locator=%q message=%q",
			got.Result, got.Reason, got.Stage, got.Locator, got.Message)
	}
	if out := runDriveToTerminal(t, svc, got); out != gatedrive.PASSED {
		t.Fatalf("replacement drive outcome = %s, want PASSED", out)
	}
}

// TestIntegrationRunCompletionProductionCensusHaltedLiveDriveBlocks: the successful
// closeout's production census is attributed by the run's context hash. With only
// settled drives of the run (PASSED, plus removed run dirs) it completes; a HALTED
// drive of the run whose supervisor still runs — a HALT label is never proof of
// teardown — blocks it completion-unaccounted with run-live:<id>, observing only:
// the live run is never stopped and the run stays completing.
func TestIntegrationRunCompletionProductionCensusHaltedLiveDriveBlocks(t *testing.T) {
	registerDone := func(t *testing.T, fx cancelFixture) {
		must(t, RegisterRunParticipant(fx.repo, fx.key,
			RunParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
		must(t, RecordRunParticipantTerminal(fx.repo, fx.key,
			"turn-1", "t1", ParticipantTerminalCompleted))
	}
	t.Run("settled-drives-complete", func(t *testing.T) {
		fx := prepareQuiescentRun(t)
		registerDone(t, fx)
		if ok, reason, findings := completeSuccessfulRun(productionCancelSeams(fx.repo), fx.repo, fx.key); !ok {
			t.Fatalf("closeout over settled drives ok=false reason=%q findings=%v", reason, findings)
		}
	})
	t.Run("halted-drive-live-supervisor-blocks", func(t *testing.T) {
		fx := prepareQuiescentRun(t)
		registerDone(t, fx)
		// A live raw run outside any worktree (so it holds no worktree lock), named by
		// a HALTED drive of the run.
		liveRoot := testsupport.TempDir(t)
		reapRunSupervisors(t, liveRoot) // so the cleanup stop proves the group gone promptly
		live := GateLaunch(liveRoot, testsupport.TempDir(t), []string{"/bin/sleep", "60"})
		t.Cleanup(func() { GateStop(live.RunDir, "test cleanup") })
		if live.Result != ResultApplied || live.RunDir == "" {
			t.Fatalf("launch: result=%s reason=%q", live.Result, live.Reason)
		}
		const haltedID = "acccccccccccccccccccccccccccccc8"
		seedCensusDrive(t, fx.common, haltedID, map[string]any{
			"worktree_path": fx.worktree, "raw_run_dir": live.RunDir,
			"last_outcome": string(gatedrive.HALTED), "last_cause": "deadline-expired-stop-unproven",
			"run_context_hash": fx.contextHash,
		})
		ok, reason, findings := completeSuccessfulRun(productionCancelSeams(fx.repo), fx.repo, fx.key)
		if ok || reason != "completion-unaccounted" || !hasFinding(findings, "run-live:"+haltedID) {
			t.Fatalf("closeout = ok %v reason %q findings %v, want completion-unaccounted with run-live:%s", ok, reason, findings, haltedID)
		}
		if st := loadRunState(t, fx.repo, fx.key); st != RunCompleting {
			t.Fatalf("run state = %q, want completing (the success fence holds while blocked)", st)
		}
		if obs := GateObserve(live.RunDir); obs.State != "running" {
			t.Fatalf("the stop-free closeout stopped the live run: state %q", obs.State)
		}
	})
}

// TestIntegrationRunCompletionProductionCensusMissingRunRoot (change 0491, spec T3):
// a never-attached first launch whose run root does not exist used to read
// resolution-unresolved forever, so cancel stayed cancellation-pending, the closeout
// blocked, and resume quiescence refused. Each now reads it as never launched.
func TestIntegrationRunCompletionProductionCensusMissingRunRoot(t *testing.T) {
	const orphan = "adddddddddddddddddddddddddd00491"
	seed := func(t *testing.T, fx cancelFixture) {
		missing := filepath.Join(testsupport.TempDir(t), "never-created-run-root")
		seedNeverLaunchedDrive(t, fx.common, orphan, fx.worktree, missing, fx.contextHash, nil)
	}
	t.Run("cancel-settles-run-cancelled", func(t *testing.T) {
		fx := prepareQuiescentRun(t)
		seed(t, fx)
		res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, "human stop")
		if res.Disposition != CancelDispositionCancelled {
			t.Fatalf("cancel = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
		}
		if out, cause := driveOutcome(t, fx.store, orphan); out != gatedrive.HALTED || cause != "run-cancelled" {
			t.Fatalf("drive = %s/%q, want HALTED run-cancelled", out, cause)
		}
	})
	t.Run("closeout-settles-launch-abandoned", func(t *testing.T) {
		fx := prepareQuiescentRun(t)
		must(t, RegisterRunParticipant(fx.repo, fx.key,
			RunParticipant{Kind: "coordinator", NativeHandle: "turn-1"}))
		must(t, RecordRunParticipantTerminal(fx.repo, fx.key,
			"turn-1", "t1", ParticipantTerminalCompleted))
		seed(t, fx)
		if ok, reason, findings := completeSuccessfulRun(productionCancelSeams(fx.repo), fx.repo, fx.key); !ok {
			t.Fatalf("closeout ok=false reason=%q findings=%v", reason, findings)
		}
		if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
			t.Fatalf("run state = %q, want completed", st)
		}
		if out, cause := driveOutcome(t, fx.store, orphan); out != gatedrive.HALTED || cause != "launch-abandoned" {
			t.Fatalf("drive = %s/%q, want HALTED launch-abandoned", out, cause)
		}
	})
	t.Run("resume-quiescence-admits", func(t *testing.T) {
		fx := prepareQuiescentRun(t)
		if res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, "human stop"); res.Disposition != CancelDispositionCancelled {
			t.Fatalf("cancel = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
		}
		seed(t, fx) // left behind after the cancel, as a delayed launcher would
		oldEp, _, err := LoadRunRecord(fx.repo, fx.key)
		if err != nil {
			t.Fatalf("LoadRunRecord: %v", err)
		}
		if ok, detail := validateResumeQuiescence(productionCancelSeams(fx.repo), fx.repo, oldEp); !ok {
			t.Fatalf("resume quiescence refused over a missing run root: %s", detail)
		}
	})
}
