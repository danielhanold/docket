package gatedrive

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// ---------------------------------------------------------------------------
// Cancellation's pending-launch accounting (change 0437 Task 6).
// ReconcileRunLaunches walks the drive registry, attributes each drive to its
// run through resolveDriveRun (the SAME linkage the launch paths use), and
// proves — under the per-drive claimant flock and the process seam — whether each
// nonterminal drive of a FENCED run has a settled or a still-pending launch. It
// launches nothing, mutates no drive verdict, and preserves unresolved evidence.
// ---------------------------------------------------------------------------

// reconcileFindingPresent reports whether any finding starts with prefix.
func reconcileFindingPresent(findings []string, prefix string) bool {
	for _, f := range findings {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// seedSlotLinkedRunDrive persists a drive linked to runID the way production links
// a build-owned drive: a worktree execution slot reserved for runID on the drive's
// worktree, with the drive carrying that slot's reservation token. resolveDriveRun
// answers runID for it, and the slot's current token is the census's positive
// current reference to it (change 0446 spec §4). mutate tweaks the record before it
// is persisted.
func seedSlotLinkedRunDrive(t *testing.T, store *Store, worktree, runID string, mutate func(*driveRecord)) (id, ownerGen string) {
	t.Helper()
	rec := seedRecord(t)
	rec.WorktreePath = worktree
	token, err := store.ReserveWorktreeExecution(admissionRecord{
		RepoIdentity: rec.RepoIdentity, WorktreeRoot: worktree, RunID: runID, Kind: "scopeless",
	})
	if err != nil {
		t.Fatalf("ReserveWorktreeExecution: %v", err)
	}
	rec.AdmissionToken = token
	if mutate != nil {
		mutate(&rec)
	}
	id, _, err = store.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	return id, rec.OwnerGeneration
}

// rotateSlotToken loses a slot-linked drive's run linkage the way production
// does: its worktree slot is released and re-reserved for the same run under a
// fresh token, so the drive's AdmissionToken no longer matches and resolveDriveRun
// reports CauseRunLinkLost.
func rotateSlotToken(t *testing.T, store *Store, rec driveRecord, runID string) {
	t.Helper()
	if err := store.ReleaseWorktreeExecution(rec.WorktreePath, rec.AdmissionToken); err != nil {
		t.Fatalf("ReleaseWorktreeExecution: %v", err)
	}
	if _, err := store.ReserveWorktreeExecution(admissionRecord{
		RepoIdentity: rec.RepoIdentity, WorktreeRoot: rec.WorktreePath, RunID: runID, Kind: "scopeless",
	}); err != nil {
		t.Fatalf("ReserveWorktreeExecution (rotate): %v", err)
	}
}

// admitRunDrive admits (reserve only, no launch) a REAL drive on the sample
// worktree for runID through Driver.Admit, so the worktree slot names the run and
// holds the drive's AdmissionToken exactly as production leaves it. It returns the
// ticket.
func admitRunDrive(t *testing.T, d *Driver, runID string) *AdmissionTicket {
	t.Helper()
	req := sampleStart()
	req.RunID = runID
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	return ticket
}

// settleDriveOutcome forces drive id's recorded outcome (a test stand-in for the
// driver persisting a verdict).
func settleDriveOutcome(t *testing.T, store *Store, id string, outcome Outcome) {
	t.Helper()
	if err := store.ownerCAS(id, func(r *driveRecord) error {
		r.LastOutcome = outcome
		return nil
	}); err != nil {
		t.Fatalf("settle %s: %v", id, err)
	}
}

// corruptFile overwrites path with bytes no reader can decode.
func corruptFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("corrupt %s: %v", path, err)
	}
}

// findingFor reports whether findings carry exactly tok+":"+id.
func findingFor(findings []string, tok, id string) bool {
	for _, f := range findings {
		if f == tok+":"+id {
			return true
		}
	}
	return false
}

// TestReconcileSeesPendingReservedDrive proves a drive admitted but never launched
// (a delayed StartAdmitted the fence still has to refuse) is reported pending —
// Accounted=false with a launch-pending finding — for both a drive admitted through
// Driver.Admit and a hand-seeded slot-linked drive, even though nothing was ever
// launched or registered.
func TestReconcileSeesPendingReservedDrive(t *testing.T) {
	t.Run("scopeless", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, _ := newTestDriver(t, clk, proc, stableGit())

		req := sampleStart()
		req.RunID = "e1"
		ticket, err := d.Admit(req) // reserve only; no StartAdmitted
		if err != nil {
			t.Fatalf("Admit: %v", err)
		}

		report, err := d.ReconcileRunLaunches(req.Worktree, "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted {
			t.Fatalf("a reserved-but-unlaunched drive must NOT be accounted, findings=%v", report.Findings)
		}
		if !reconcileFindingPresent(report.Findings, "launch-pending:"+ticket.id) {
			t.Fatalf("findings = %v, want launch-pending:%s", report.Findings, ticket.id)
		}
		if proc.launchN != 0 {
			t.Fatalf("reconcile must launch nothing, proc.Launch called %d times", proc.launchN)
		}
	})

	t.Run("seeded-slot-linked", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RawRunDir = "" // never launched
			r.RawOwnership = ""
		})

		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted {
			t.Fatalf("a reserved-but-unlaunched slot-linked drive must NOT be accounted, findings=%v", report.Findings)
		}
		if !reconcileFindingPresent(report.Findings, "launch-pending:"+id) {
			t.Fatalf("findings = %v, want launch-pending:%s", report.Findings, id)
		}
	})
}

// TestReconcileBusyClaimIsPending proves a held claimant flock (a launch still in
// flight) is reported claim-busy and Accounted=false, and that reconcile returns
// without waiting on the claim.
func TestReconcileBusyClaimIsPending(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", nil)

	// Hold the drive's claimant flock from the test: a launch is "in flight".
	claim, busy, err := store.tryRelaunchClaim(id)
	if err != nil || busy {
		t.Fatalf("tryRelaunchClaim = (busy=%v, err=%v), want a free claim", busy, err)
	}
	defer claim.close()

	done := make(chan RunLaunchReport, 1)
	go func() {
		r, _ := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		done <- r
	}()
	select {
	case report := <-done:
		if report.Accounted {
			t.Fatalf("a busy claim must NOT be accounted, findings=%v", report.Findings)
		}
		if !reconcileFindingPresent(report.Findings, "claim-busy:"+id) {
			t.Fatalf("findings = %v, want claim-busy:%s", report.Findings, id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reconcile blocked on a busy claim; it must probe nonblocking and return promptly")
	}
	if proc.launchN != 0 {
		t.Fatalf("reconcile must launch nothing, proc.Launch called %d times", proc.launchN)
	}
}

// TestReconcileProvenNeverLaunchedAccounts proves a reserved relaunch the process
// seam proves NEVER launched accounts the obligation and launches nothing, and — to
// close the launch-after-cancel window (spec AC4) — settles the drive terminal HALTED
// "run-cancelled" under the held claim while PRESERVING the consumed reservation (the
// sole relaunch is never refunded), so a later recovery Advance can never launch it.
func TestReconcileProvenNeverLaunchedAccounts(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "never-launched"}, nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
		r.RelaunchReserved = true
		r.RelaunchToken = "aaaaaaaaaaaaaaaa"
	})
	before, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load before: %v", err)
	}

	report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a proven never-launched reservation must be accounted, findings=%v", report.Findings)
	}
	if proc.launchN != 0 {
		t.Fatalf("reconcile must launch nothing, proc.Launch called %d times", proc.launchN)
	}
	after, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load after: %v", err)
	}
	if after.LastOutcome != HALTED || after.LastCause != "run-cancelled" {
		t.Fatalf("reconcile must settle the never-launched reserved relaunch terminal HALTED run-cancelled, got %v/%q", after.LastOutcome, after.LastCause)
	}
	if !after.RelaunchReserved || after.RelaunchToken != before.RelaunchToken || after.RelaunchCount != before.RelaunchCount {
		t.Fatalf("the terminal settle must preserve the consumed reservation (no refund): before=%+v after=%+v", before, after)
	}
}

// TestReconcileIdentifiedReplacementStopped proves a relaunch's attached replacement
// — named by the DRIVE record even when the worktree slot still names the
// predecessor — is found via the drive record, stopped through proc, and accounted
// only on PROVEN teardown; an unproven stop keeps the obligation pending.
func TestReconcileIdentifiedReplacementStopped(t *testing.T) {
	const replacement = "/runs/replacement"
	const predecessor = "/runs/predecessor"

	t.Run("proven-stop-accounts", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		var stopped []string
		proc := &fakeProc{
			stop: func(runDir, reason string) (*process.StopOutcome, error) {
				stopped = append(stopped, runDir)
				return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RawRunDir = replacement // the drive names the replacement
			r.RawOwnership = "replacement"
			r.RelaunchCount = 1
		})

		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted {
			t.Fatalf("a proven-stopped replacement must be accounted, findings=%v", report.Findings)
		}
		if !reconcileFindingPresent(report.Findings, "replacement-stopped:"+id) {
			t.Fatalf("findings = %v, want replacement-stopped:%s", report.Findings, id)
		}
		if len(stopped) != 1 || stopped[0] != replacement {
			t.Fatalf("reconcile stopped %v, want exactly [%s] (found via the DRIVE record, not %s)", stopped, replacement, predecessor)
		}
	})

	t.Run("unproven-stop-pending", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{
			stop: func(runDir, reason string) (*process.StopOutcome, error) {
				return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RawRunDir = replacement
			r.RawOwnership = "replacement"
			r.RelaunchCount = 1
		})

		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted {
			t.Fatalf("an unproven replacement stop must keep the run pending, findings=%v", report.Findings)
		}
		if !reconcileFindingPresent(report.Findings, "stop-unproven:"+id) {
			t.Fatalf("findings = %v, want stop-unproven:%s", report.Findings, id)
		}
	})
}

// TestReconcileReservedRelaunchResolved proves a reserved-but-unattached relaunch
// resolves the RELAUNCH token (never the admission token) and settles per the
// resolution: never-launched accounts, identified stops, unresolved stays pending.
func TestReconcileReservedRelaunchResolved(t *testing.T) {
	const relaunchToken = "aaaaaaaaaaaaaaaa"

	newProc := func(disp string, stopPerformed bool) (*fakeProc, *string) {
		seen := new(string)
		return &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				*seen = token
				return &process.ReservationResolution{Disposition: disp, RunID: "rep", RunDir: "/runs/rep"}, nil
			},
			stop: func(runDir, reason string) (*process.StopOutcome, error) {
				return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: stopPerformed}, nil
			},
		}, seen
	}
	seed := func(store *Store) string {
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RelaunchReserved = true
			r.RelaunchToken = relaunchToken // the AdmissionToken stays the slot's own token
		})
		return id
	}

	t.Run("never-launched-accounts", func(t *testing.T) {
		proc, seen := newProc("never-launched", false)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		seed(store)
		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if *seen != relaunchToken {
			t.Fatalf("resolved token = %q, want the RELAUNCH token %q (never the admission token)", *seen, relaunchToken)
		}
		if !report.Accounted {
			t.Fatalf("a never-launched reserved relaunch must be accounted, findings=%v", report.Findings)
		}
	})

	t.Run("identified-stops-and-accounts", func(t *testing.T) {
		proc, seen := newProc("identified", true)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id := seed(store)
		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if *seen != relaunchToken {
			t.Fatalf("resolved token = %q, want the RELAUNCH token %q", *seen, relaunchToken)
		}
		if !report.Accounted || !reconcileFindingPresent(report.Findings, "replacement-stopped:"+id) {
			t.Fatalf("an identified reserved relaunch proven stopped must account, findings=%v", report.Findings)
		}
	})

	t.Run("unresolved-stays-pending", func(t *testing.T) {
		proc, seen := newProc("unresolved", false)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id := seed(store)
		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if *seen != relaunchToken {
			t.Fatalf("resolved token = %q, want the RELAUNCH token %q", *seen, relaunchToken)
		}
		if report.Accounted || !reconcileFindingPresent(report.Findings, "resolution-unresolved:"+id) {
			t.Fatalf("an unresolved reserved relaunch must stay pending, findings=%v", report.Findings)
		}
	})
}

// TestReconcileNeverLaunchedSettlesTerminalClosingRecoveryLaunchWindow proves the
// launch-after-cancel window (spec AC4) is closed: when reconcile resolves a
// reserved relaunch never-launched UNDER THE HELD CLAIM, it settles the drive
// terminal HALTED "run-cancelled" before releasing the claim, so a SUBSEQUENT
// Advance recovery on the same drive launches NOTHING — even though that recovery's
// own read-only run pass races ahead of the fence and reads the run still live
// (modelled here by injecting no run launch gate, so recoveryRunRevoked returns false).
// Without the settle the recovery would take the freed claim, resolve never-launched
// under the stale revoked=false, and relaunch the dead run AFTER cancellation had
// already completed. The oracle is a strict ordering (reconcile fully returns before
// Advance runs) plus the proc.Launch count — never a timing sleep.
func TestReconcileNeverLaunchedSettlesTerminalClosingRecoveryLaunchWindow(t *testing.T) {
	// The reserved relaunch: a crash-window reservation the recovery path resolves.
	recProc := &fakeProc{
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "never-launched"}, nil
		},
	}
	recDriver, store := newTestDriver(t, &fakeClock{now: startRun()}, recProc, stableGit())
	id, ownerGen := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
		r.RelaunchReserved = true
		r.RelaunchToken = "aaaaaaaaaaaaaaaa"
	})

	// (a) Cancellation reconciles the FENCED run's reserved relaunch: proc proves
	// never-launched, so the obligation is accounted. The reconcile fully returns
	// (releasing the per-drive claim) before the Advance below runs.
	report, err := recDriver.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a proven never-launched reserved relaunch must be accounted, findings=%v", report.Findings)
	}

	// (b) A later Advance recovery on the SAME drive, on a fresh store handle (an
	// independent CLI process) whose run reads as live. Its seeded run is dead, so
	// a NONTERMINAL record would drive its single relaunch and create a process AFTER
	// the completed cancellation. The terminal settle in (a) must forbid that launch.
	advProc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if strings.HasSuffix(runDir, "run1") {
				return obs(process.StateVanished, runDir), nil
			}
			return obs(process.StateRunning, runDir), nil
		},
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			return &process.LaunchOutcome{RunID: "replacement", RunDir: "/runs/replacement", State: process.StateRunning}, nil
		},
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "never-launched"}, nil
		},
	}
	advClk := &fakeClock{now: startRun().Add(time.Second)}
	advDriver := NewDriver(reopenStore(store), advClk, advProc, stableGit())
	advDriver.slice = 4 * pollTick
	advDriver.pollInterval = pollTick
	advDriver.sleep = func(dur time.Duration) { advClk.advance(dur) }

	doc, err := advDriver.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if advProc.launchN != 0 {
		t.Fatalf("a recovery after a completed cancellation must launch NOTHING (AC4 launch-after-cancel window), proc.Launch called %d times", advProc.launchN)
	}
	if doc.Outcome != HALTED || doc.Cause != "run-cancelled" {
		t.Fatalf("the terminal settle must resolve the recovery Advance to HALTED run-cancelled, got %s/%q", doc.Outcome, doc.Cause)
	}
	settled, lerr := store.Load(id)
	if lerr != nil {
		t.Fatalf("Load after: %v", lerr)
	}
	if settled.LastOutcome != HALTED || settled.LastCause != "run-cancelled" {
		t.Fatalf("reconcile must settle the never-launched reserved relaunch terminal HALTED run-cancelled, got %v/%q", settled.LastOutcome, settled.LastCause)
	}
}

// TestReconcileFailuresPreserveEvidence proves a resolution error, an unreadable
// drive record, and a stop error each keep Accounted=false and touch no records
// (publication/read/stop failures preserve unresolved evidence).
func TestReconcileFailuresPreserveEvidence(t *testing.T) {
	t.Run("resolution-error", func(t *testing.T) {
		proc := &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				return nil, errors.New("gatedrive-test: resolve fault")
			},
		}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RelaunchReserved = true
			r.RelaunchToken = "aaaaaaaaaaaaaaaa"
		})
		before, _ := store.Load(id)
		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !reconcileFindingPresent(report.Findings, "resolution-unresolved:"+id) {
			t.Fatalf("a resolve fault must preserve evidence (pending), findings=%v", report.Findings)
		}
		after, _ := store.Load(id)
		if after.LastOutcome != before.LastOutcome {
			t.Fatalf("a resolve fault must not mutate the record: before=%v after=%v", before.LastOutcome, after.LastOutcome)
		}
	})

	t.Run("unreadable-record", func(t *testing.T) {
		proc := &fakeProc{}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", nil)
		// Corrupt the record so Load fails closed on the walk.
		if err := os.WriteFile(filepath.Join(store.root, id, recordFileName), []byte("{not-json"), 0o600); err != nil {
			t.Fatalf("corrupt record: %v", err)
		}
		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !reconcileFindingPresent(report.Findings, "record-unreadable:"+id) {
			t.Fatalf("an unreadable record must fail closed (pending), findings=%v", report.Findings)
		}
	})

	t.Run("stop-error", func(t *testing.T) {
		proc := &fakeProc{
			stop: func(runDir, reason string) (*process.StopOutcome, error) {
				return nil, errors.New("gatedrive-test: stop fault")
			},
		}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RawRunDir = "/runs/live"
			r.RawOwnership = "live"
		})
		before, _ := store.Load(id)
		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !reconcileFindingPresent(report.Findings, "resolution-unresolved:"+id) {
			t.Fatalf("a stop fault must preserve evidence (pending), findings=%v", report.Findings)
		}
		after, _ := store.Load(id)
		if after.LastOutcome != before.LastOutcome {
			t.Fatalf("a stop fault must not mutate the record")
		}
	})
}

// TestReconcileLostLinkageFailsClosed is change 0437's lost-linkage regression
// (change 0446 spec §4): a hand-seeded drive whose run linkage is lost (its worktree
// slot re-reserved under a fresh token, CauseRunLinkLost) that no surviving current
// reference names is informational history, not a repository-wide veto on the run.
func TestReconcileLostLinkageFailsClosed(t *testing.T) {
	t.Run("hand-seeded-orphan-is-history", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", nil)
		rec, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		rotateSlotToken(t, store, rec, "e1")
		if _, ok, cause := d.resolveDriveRun(rec); ok || cause != CauseRunLinkLost {
			t.Fatalf("fixture must lose the drive's linkage, resolveDriveRun = (ok=%v, cause=%q)", ok, cause)
		}

		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted {
			t.Fatalf("an orphan no current reference names must not veto the run, findings=%v", report.Findings)
		}
		if !findingFor(report.Findings, "history-unattributed", id) || reconcileFindingPresent(report.Findings, "linkage-unresolved:") {
			t.Fatalf("findings = %v, want informational history-unattributed:%s and no linkage-unresolved", report.Findings, id)
		}
	})
}

// ---------------------------------------------------------------------------
// Run launch census attribution (change 0446 Task 5, spec §4). The census
// accounts the TARGET run's obligations through current references — the target
// worktree slot's reservation — and never inherits all history: an
// unreadable or unlinked record nothing current names is an informational
// history-unattributed finding, while a named one still fails the run closed.
// ---------------------------------------------------------------------------

// TestCensusTerminalSettledBeforeLinkage proves rule 1: a terminal (here HALTED)
// drive's launch axis is settled BEFORE its linkage is resolved, so a rotated slot
// token does not turn a finished drive into linkage-unresolved.
func TestCensusTerminalSettledBeforeLinkage(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
	id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
		r.LastOutcome = HALTED
		r.LastCause = "stopped-not-initiated"
	})
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rotateSlotToken(t, store, rec, "e1")

	report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a terminal drive's launch axis is settled regardless of lost linkage, findings=%v", report.Findings)
	}
	for _, f := range report.Findings {
		if strings.HasSuffix(f, ":"+id) {
			t.Fatalf("a terminal drive must produce no finding, got %v", report.Findings)
		}
	}
}

// TestCensusUnreferencedCorruptRecordInformational proves rule 3's diagnostic half:
// a corrupt record no current reference names is history-unattributed and does not
// clear Accounted — with no slot at all, and with an occupied run slot whose token
// a READABLE drive holds (so no unreadable record is a candidate holder).
func TestCensusUnreferencedCorruptRecordInformational(t *testing.T) {
	seedUnrelatedCorrupt := func(t *testing.T, store *Store) string {
		t.Helper()
		id, _, err := store.NewDrive(seedRecord(t))
		if err != nil {
			t.Fatalf("NewDrive: %v", err)
		}
		corruptFile(t, filepath.Join(store.root, id, recordFileName))
		return id
	}

	t.Run("no-slot", func(t *testing.T) {
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
		id := seedUnrelatedCorrupt(t, store)
		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted {
			t.Fatalf("an unreferenced corrupt record must not veto the run, findings=%v", report.Findings)
		}
		if !findingFor(report.Findings, "history-unattributed", id) {
			t.Fatalf("findings = %v, want history-unattributed:%s", report.Findings, id)
		}
	})

	t.Run("slot-holder-readable", func(t *testing.T) {
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
		ticket := admitRunDrive(t, d, "e1")
		settleDriveOutcome(t, store, ticket.id, PASSED) // the slot's holder is readable and settled
		id := seedUnrelatedCorrupt(t, store)
		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted {
			t.Fatalf("a corrupt record the resolved slot does not name must not veto the run, findings=%v", report.Findings)
		}
		if !findingFor(report.Findings, "history-unattributed", id) {
			t.Fatalf("findings = %v, want history-unattributed:%s", report.Findings, id)
		}
	})
}

// TestCensusReferencedCorruptRecordBlocks proves rule 3's fail-closed half on a REAL
// admitted drive: a record a current reference names still keeps the run
// unaccounted with its exact locator — through the occupied run slot whose token no
// readable drive holds. The reference is established from the slot side, never by
// reading the corrupt record.
func TestCensusReferencedCorruptRecordBlocks(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	ticket := admitRunDrive(t, d, "e1")
	corruptFile(t, filepath.Join(store.root, ticket.id, recordFileName))

	report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if report.Accounted {
		t.Fatalf("a corrupt record named by current ownership must fail closed, findings=%v", report.Findings)
	}
	if !findingFor(report.Findings, "record-unreadable", ticket.id) {
		t.Fatalf("findings = %v, want record-unreadable:%s", report.Findings, ticket.id)
	}
}

// TestCensusSchema2HistoricalTerminalSettles proves rule 2: a supported schema-2
// record the executable reader refuses is read through loadHistoricalDrive — a
// terminal one is settled history (no finding at all), a nonterminal unreferenced
// one is informational — never record-unreadable.
func TestCensusSchema2HistoricalTerminalSettles(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	passed := copyLegacyFixture(t, store, "passed")
	halted := copyLegacyFixture(t, store, "halted")
	waiting := copyLegacyFixture(t, store, "waiting")

	report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("supported schema-2 history must not veto the run, findings=%v", report.Findings)
	}
	for _, id := range []string{passed, halted} {
		for _, f := range report.Findings {
			if strings.HasSuffix(f, ":"+id) {
				t.Fatalf("a terminal schema-2 record is settled history with no finding, got %v", report.Findings)
			}
		}
	}
	if !findingFor(report.Findings, "history-unattributed", waiting) {
		t.Fatalf("findings = %v, want history-unattributed:%s", report.Findings, waiting)
	}
}

// TestCensusSupersededRunStillEnumerates proves rule 5 (AC5's superseded branch):
// an empty worktreeRoot is not proof of quiescence — the census still walks the
// registry and accounts the run's slot-linked drives (resolveDriveRun reads each
// drive's own worktree slot), skipping only the census's slot-side references (the
// app layer supplies a superseded run's replacement worktree for those).
func TestCensusSupersededRunStillEnumerates(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
		r.RawRunDir = "" // reserved, never launched
		r.RawOwnership = ""
	})

	for _, mode := range []struct {
		name string
		run  func(string, string) (RunLaunchReport, error)
	}{{"reconcile", d.ReconcileRunLaunches}, {"observe", d.ObserveRunLaunches}} {
		report, err := mode.run("", "e1")
		if err != nil {
			t.Fatalf("%s: %v", mode.name, err)
		}
		if report.Accounted {
			t.Fatalf("%s: a superseded run's unaccounted slot-linked drive must not account vacuously, findings=%v", mode.name, report.Findings)
		}
		if !findingFor(report.Findings, "launch-pending", id) {
			t.Fatalf("%s: findings = %v, want launch-pending:%s", mode.name, report.Findings, id)
		}
	}
}

// TestCensusRotatedTokenScopelessIsHistorical proves rule 6: an older nonterminal
// scopeless drive whose AdmissionToken the slot no longer holds has lost launch
// authority and is history-unattributed, while the CURRENT token holder carries the
// run's live obligation and is accounted on its own.
func TestCensusRotatedTokenScopelessIsHistorical(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	older := admitRunDrive(t, d, "e1")
	if err := store.ReleaseWorktreeExecution(sampleWorktree(), older.token); err != nil {
		t.Fatalf("ReleaseWorktreeExecution: %v", err)
	}
	current := admitRunDrive(t, d, "e1") // the slot now holds a new token

	report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !findingFor(report.Findings, "history-unattributed", older.id) || findingFor(report.Findings, "linkage-unresolved", older.id) {
		t.Fatalf("findings = %v, want the rotated-token drive informational (history-unattributed:%s)", report.Findings, older.id)
	}
	if report.Accounted || !findingFor(report.Findings, "launch-pending", current.id) {
		t.Fatalf("the current token holder must still carry its pending obligation, got %+v", report)
	}

	settleDriveOutcome(t, store, current.id, PASSED)
	settled, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileRunLaunches (settled): %v", err)
	}
	if !settled.Accounted {
		t.Fatalf("once the current holder settles, the rotated-token history must not keep the run unaccounted, findings=%v", settled.Findings)
	}
}

// TestReconcileReplayConverges proves an unproven stop leaves the run pending, and a
// later replay — once the fake proves teardown — accounts it with no second launch
// (AC5 replay convergence).
func TestReconcileReplayConverges(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proven := false
	proc := &fakeProc{
		stop: func(runDir, reason string) (*process.StopOutcome, error) {
			if proven {
				return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
			}
			return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false}, nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
		r.RawRunDir = "/runs/live"
		r.RawOwnership = "live"
	})

	first, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if first.Accounted {
		t.Fatalf("first reconcile must be pending on an unproven stop, findings=%v", first.Findings)
	}

	proven = true
	second, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if !second.Accounted {
		t.Fatalf("replay must account once teardown proves, findings=%v", second.Findings)
	}
	if proc.launchN != 0 {
		t.Fatalf("reconcile must never launch, proc.Launch called %d times", proc.launchN)
	}
}

// ---------------------------------------------------------------------------
// Observation-only launch accounting (change 0441 Task 4). ObserveRunLaunches
// is the SUCCESS-closeout view of one run's launch obligations: the same walk,
// run linkage, and claimant probe as ReconcileRunLaunches, but it NEVER stops
// a process, NEVER settles a never-launched reservation terminal, and NEVER
// mutates a record. Shared implementation, one inventory.
// ---------------------------------------------------------------------------

// TestObserveRunLaunchesNeverStopsOrSettles proves a run-linked NONTERMINAL
// drive with an attached run the fake proc reports RUNNING is reported pending
// (run-live) in observe mode, that NO Stop is issued, and that the drive record on
// disk is byte-identical afterward (observation mutates nothing).
func TestObserveRunLaunchesNeverStopsOrSettles(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateRunning, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
		r.RawRunDir = "/runs/live"
		r.RawOwnership = "live"
	})
	recPath := filepath.Join(store.root, id, recordFileName)
	before, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("read record before: %v", err)
	}

	report, err := d.ObserveRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ObserveRunLaunches: %v", err)
	}
	if report.Accounted {
		t.Fatalf("a live attached run must NOT be accounted in observe mode, findings=%v", report.Findings)
	}
	if !reconcileFindingPresent(report.Findings, "run-live:"+id) {
		t.Fatalf("findings = %v, want run-live:%s", report.Findings, id)
	}
	if proc.stopN != 0 {
		t.Fatalf("observe mode must never stop a process, proc.Stop called %d times", proc.stopN)
	}
	after, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("read record after: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("observe mode must not mutate the drive record:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestObserveRunLaunchesAccountsProvenTerminalRun proves an attached run the fake
// proc proves STOPPED is accounted (run-terminal) in observe mode with no Stop call.
func TestObserveRunLaunchesAccountsProvenTerminalRun(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateStopped, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
		r.RawRunDir = "/runs/gone"
		r.RawOwnership = "gone"
	})

	report, err := d.ObserveRunLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ObserveRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a proven-terminal run must be accounted in observe mode, findings=%v", report.Findings)
	}
	if !reconcileFindingPresent(report.Findings, "run-terminal:"+id) {
		t.Fatalf("findings = %v, want run-terminal:%s", report.Findings, id)
	}
	if proc.stopN != 0 {
		t.Fatalf("observe mode must never stop a process, proc.Stop called %d times", proc.stopN)
	}
}

// TestObserveRunLaunchesKeepsNeverLaunchedPending proves observe mode reports a
// reserved-never-launched drive AND a bare (unlaunched, unreserved) drive as pending
// (launch-pending) and — unlike reconcile — NEVER settles the reservation terminal:
// the record's outcome is unchanged (the completing launch-gate refusal settles it
// later, and a replay then accounts).
func TestObserveRunLaunchesKeepsNeverLaunchedPending(t *testing.T) {
	t.Run("reserved-never-launched", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				return &process.ReservationResolution{Disposition: "never-launched"}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RelaunchReserved = true
			r.RelaunchToken = "aaaaaaaaaaaaaaaa"
		})
		before, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load before: %v", err)
		}

		report, err := d.ObserveRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ObserveRunLaunches: %v", err)
		}
		if report.Accounted {
			t.Fatalf("observe must not account a never-launched reservation, findings=%v", report.Findings)
		}
		if !reconcileFindingPresent(report.Findings, "launch-pending:"+id) {
			t.Fatalf("findings = %v, want launch-pending:%s", report.Findings, id)
		}
		after, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load after: %v", err)
		}
		if after.LastOutcome != before.LastOutcome || isTerminalOutcome(after.LastOutcome) {
			t.Fatalf("observe must never settle the reservation terminal: before=%v after=%v", before.LastOutcome, after.LastOutcome)
		}
	})

	t.Run("bare-reservation", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RawRunDir = ""
			r.RawOwnership = ""
		})
		before, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load before: %v", err)
		}

		report, err := d.ObserveRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ObserveRunLaunches: %v", err)
		}
		if report.Accounted {
			t.Fatalf("observe must not account a bare never-launched drive, findings=%v", report.Findings)
		}
		if !reconcileFindingPresent(report.Findings, "launch-pending:"+id) {
			t.Fatalf("findings = %v, want launch-pending:%s", report.Findings, id)
		}
		after, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load after: %v", err)
		}
		if after.LastOutcome != before.LastOutcome {
			t.Fatalf("observe must not mutate the record: before=%v after=%v", before.LastOutcome, after.LastOutcome)
		}
	})
}

// TestObserveRunLaunchesBusyClaimIsPending proves a held claimant flock is reported
// claim-busy and Accounted=false in observe mode, and that observe probes nonblocking
// (returns without waiting on the claim) and stops nothing.
func TestObserveRunLaunchesBusyClaimIsPending(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", nil)

	claim, busy, err := store.tryRelaunchClaim(id)
	if err != nil || busy {
		t.Fatalf("tryRelaunchClaim = (busy=%v, err=%v), want a free claim", busy, err)
	}
	defer claim.close()

	done := make(chan RunLaunchReport, 1)
	go func() {
		r, _ := d.ObserveRunLaunches(sampleWorktree(), "e1")
		done <- r
	}()
	select {
	case report := <-done:
		if report.Accounted {
			t.Fatalf("a busy claim must NOT be accounted, findings=%v", report.Findings)
		}
		if !reconcileFindingPresent(report.Findings, "claim-busy:"+id) {
			t.Fatalf("findings = %v, want claim-busy:%s", report.Findings, id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("observe blocked on a busy claim; it must probe nonblocking and return promptly")
	}
	if proc.stopN != 0 {
		t.Fatalf("observe mode must never stop, proc.Stop called %d times", proc.stopN)
	}
}

// TestReconcileRunLaunchesBehaviorUnchanged is the regression pin: after factoring
// the shared body, the reconcile (cancel) mode still STOPS an identified run and
// still SETTLES a proven never-launched reservation terminal HALTED "run-cancelled".
func TestReconcileRunLaunchesBehaviorUnchanged(t *testing.T) {
	t.Run("identified-run-stopped", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		var stopped []string
		proc := &fakeProc{
			stop: func(runDir, reason string) (*process.StopOutcome, error) {
				stopped = append(stopped, runDir)
				return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RawRunDir = "/runs/replacement"
			r.RawOwnership = "replacement"
			r.RelaunchCount = 1
		})

		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted || !reconcileFindingPresent(report.Findings, "replacement-stopped:"+id) {
			t.Fatalf("reconcile must still stop and account an identified run, findings=%v", report.Findings)
		}
		if len(stopped) != 1 || stopped[0] != "/runs/replacement" {
			t.Fatalf("reconcile stopped %v, want exactly [/runs/replacement]", stopped)
		}
	})

	t.Run("never-launched-settles-terminal", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				return &process.ReservationResolution{Disposition: "never-launched"}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedSlotLinkedRunDrive(t, store, sampleWorktree(), "e1", func(r *driveRecord) {
			r.RelaunchReserved = true
			r.RelaunchToken = "aaaaaaaaaaaaaaaa"
		})

		report, err := d.ReconcileRunLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted {
			t.Fatalf("reconcile must account a proven never-launched reservation, findings=%v", report.Findings)
		}
		after, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load after: %v", err)
		}
		if after.LastOutcome != HALTED || after.LastCause != "run-cancelled" {
			t.Fatalf("reconcile must settle never-launched terminal HALTED run-cancelled, got %v/%q", after.LastOutcome, after.LastCause)
		}
	})
}

// TestReconcileReleasedSlotWithPendingDriveNotAccounted proves a released worktree
// slot alone never settles a launch obligation: a scopeless drive whose slot was
// released but that never launched is still reported pending.
func TestReconcileReleasedSlotWithPendingDriveNotAccounted(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())

	req := sampleStart()
	req.RunID = "e1"
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	// Release the slot (it preserves the reservation token + run), but the drive
	// itself never launched and remains nonterminal.
	if rerr := store.ReleaseWorktreeExecution(req.Worktree, ticket.token); rerr != nil {
		t.Fatalf("ReleaseWorktreeExecution: %v", rerr)
	}

	report, err := d.ReconcileRunLaunches(req.Worktree, "e1")
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if report.Accounted {
		t.Fatalf("a released slot alone must not settle a pending launch, findings=%v", report.Findings)
	}
	if !reconcileFindingPresent(report.Findings, "launch-pending:"+ticket.id) {
		t.Fatalf("findings = %v, want launch-pending:%s", report.Findings, ticket.id)
	}
}
