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
// ReconcileEpochLaunches walks the drive registry, attributes each drive to its
// run epoch through resolveDriveEpoch (the SAME linkage the launch paths use), and
// proves — under the per-drive claimant flock and the process seam — whether each
// nonterminal drive of a FENCED epoch has a settled or a still-pending launch. It
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

// seedScopedEpochDrive persists a drive enrolled in a fresh scope carrying epochID,
// so resolveDriveEpoch answers epochID for it. mutate tweaks the seeded record (its
// launch identity, relaunch reservation, or outcome) before it is persisted.
func seedScopedEpochDrive(t *testing.T, store *Store, epochID string, mutate func(*driveRecord)) (id, ownerGen string) {
	t.Helper()
	req := sampleStart()
	sreq := scopeReqFor(req, "")
	sreq.RunEpochID = epochID
	grant, err := store.PrepareScope(sreq)
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	rec := seedRecord(t)
	rec.ScopeID = grant.ScopeID
	if mutate != nil {
		mutate(&rec)
	}
	id, _, err = store.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	return id, rec.OwnerGeneration
}

// TestReconcileSeesPendingReservedDrive proves a drive admitted but never launched
// (a delayed StartAdmitted the fence still has to refuse) is reported pending —
// Accounted=false with a launch-pending finding — for both a scopeless slot-linked
// drive and a scoped drive, even though nothing was ever launched or registered.
func TestReconcileSeesPendingReservedDrive(t *testing.T) {
	t.Run("scopeless", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, _ := newTestDriver(t, clk, proc, stableGit())

		req := sampleStart()
		req.RunEpochID = "e1"
		ticket, err := d.Admit(req) // reserve only; no StartAdmitted
		if err != nil {
			t.Fatalf("Admit: %v", err)
		}

		report, err := d.ReconcileEpochLaunches(req.Worktree, "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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

	t.Run("scoped", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RawRunDir = "" // never launched
			r.RawOwnership = ""
		})

		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
		}
		if report.Accounted {
			t.Fatalf("a reserved-but-unlaunched scoped drive must NOT be accounted, findings=%v", report.Findings)
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
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedScopedEpochDrive(t, store, "e1", nil)

	// Hold the drive's claimant flock from the test: a launch is "in flight".
	claim, busy, err := store.tryRelaunchClaim(id)
	if err != nil || busy {
		t.Fatalf("tryRelaunchClaim = (busy=%v, err=%v), want a free claim", busy, err)
	}
	defer claim.close()

	done := make(chan EpochLaunchReport, 1)
	go func() {
		r, _ := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
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
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "never-launched"}, nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
		r.RelaunchReserved = true
		r.RelaunchToken = "aaaaaaaaaaaaaaaa"
	})
	before, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load before: %v", err)
	}

	report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches: %v", err)
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
		clk := &fakeClock{now: startEpoch()}
		var stopped []string
		proc := &fakeProc{
			stop: func(runDir, reason string) (*process.StopOutcome, error) {
				stopped = append(stopped, runDir)
				return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RawRunDir = replacement // the drive names the replacement
			r.RawOwnership = "replacement"
			r.RelaunchCount = 1
		})

		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{
			stop: func(runDir, reason string) (*process.StopOutcome, error) {
				return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RawRunDir = replacement
			r.RawOwnership = "replacement"
			r.RelaunchCount = 1
		})

		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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
	const admissionToken = "cccccccccccccccc"

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
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RelaunchReserved = true
			r.RelaunchToken = relaunchToken
			r.AdmissionToken = admissionToken
		})
		return id
	}

	t.Run("never-launched-accounts", func(t *testing.T) {
		proc, seen := newProc("never-launched", false)
		d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, proc, stableGit())
		seed(store)
		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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
		d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, proc, stableGit())
		id := seed(store)
		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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
		d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, proc, stableGit())
		id := seed(store)
		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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
// own read-only epoch pass races ahead of the fence and reads the epoch still live
// (modelled here by injecting no epoch gate, so recoveryEpochRevoked returns false).
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
	recDriver, store := newTestDriver(t, &fakeClock{now: startEpoch()}, recProc, stableGit())
	id, ownerGen := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
		r.RelaunchReserved = true
		r.RelaunchToken = "aaaaaaaaaaaaaaaa"
	})

	// (a) Cancellation reconciles the FENCED epoch's reserved relaunch: proc proves
	// never-launched, so the obligation is accounted. The reconcile fully returns
	// (releasing the per-drive claim) before the Advance below runs.
	report, err := recDriver.ReconcileEpochLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a proven never-launched reserved relaunch must be accounted, findings=%v", report.Findings)
	}

	// (b) A later Advance recovery on the SAME drive, on a fresh store handle (an
	// independent CLI process) whose epoch reads as live. Its seeded run is dead, so
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
	advClk := &fakeClock{now: startEpoch().Add(time.Second)}
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
		d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RelaunchReserved = true
			r.RelaunchToken = "aaaaaaaaaaaaaaaa"
		})
		before, _ := store.Load(id)
		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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
		d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", nil)
		// Corrupt the record so Load fails closed on the walk.
		if err := os.WriteFile(filepath.Join(store.root, id, recordFileName), []byte("{not-json"), 0o600); err != nil {
			t.Fatalf("corrupt record: %v", err)
		}
		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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
		d, store := newTestDriver(t, &fakeClock{now: startEpoch()}, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RawRunDir = "/runs/live"
			r.RawOwnership = "live"
		})
		before, _ := store.Load(id)
		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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

// TestReconcileLostLinkageFailsClosed proves a drive whose epoch linkage
// resolveDriveEpoch cannot resolve (ok==false — here an unreadable scope, i.e.
// CauseEpochUnreadable) is NOT silently skipped as "not this epoch's obligation":
// the census fails closed, keeping Accounted=false with a linkage-unresolved:<id>
// finding, mirroring the record-unreadable leg and the launch paths' refuse/revoke
// treatment of a lost linkage. A fail-OPEN skip here would drop a drive whose
// linkage is lost out of cancellation's pending-launch accounting.
func TestReconcileLostLinkageFailsClosed(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedScopedEpochDrive(t, store, "e1", nil)

	// Sever the drive's epoch linkage: corrupt its scope record so LoadScope fails,
	// making resolveDriveEpoch return ok==false (CauseEpochUnreadable). The drive
	// itself remains a readable, nonterminal record.
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.scopeRoot, rec.ScopeID, recordFileName), []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("corrupt scope record: %v", err)
	}

	report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches: %v", err)
	}
	if report.Accounted {
		t.Fatalf("a drive with lost/unreadable epoch linkage must NOT be silently skipped (fail closed), findings=%v", report.Findings)
	}
	if !reconcileFindingPresent(report.Findings, "linkage-unresolved:"+id) {
		t.Fatalf("findings = %v, want linkage-unresolved:%s", report.Findings, id)
	}
	if proc.launchN != 0 {
		t.Fatalf("reconcile must launch nothing, proc.Launch called %d times", proc.launchN)
	}
}

// TestReconcileReplayConverges proves an unproven stop leaves the run pending, and a
// later replay — once the fake proves teardown — accounts it with no second launch
// (AC5 replay convergence).
func TestReconcileReplayConverges(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
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
	seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
		r.RawRunDir = "/runs/live"
		r.RawOwnership = "live"
	})

	first, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if first.Accounted {
		t.Fatalf("first reconcile must be pending on an unproven stop, findings=%v", first.Findings)
	}

	proven = true
	second, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
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
// Observation-only launch accounting (change 0441 Task 4). ObserveEpochLaunches
// is the SUCCESS-closeout view of one epoch's launch obligations: the same walk,
// epoch linkage, and claimant probe as ReconcileEpochLaunches, but it NEVER stops
// a process, NEVER settles a never-launched reservation terminal, and NEVER
// mutates a record. Shared implementation, one inventory.
// ---------------------------------------------------------------------------

// TestObserveEpochLaunchesNeverStopsOrSettles proves an epoch-linked NONTERMINAL
// drive with an attached run the fake proc reports RUNNING is reported pending
// (run-live) in observe mode, that NO Stop is issued, and that the drive record on
// disk is byte-identical afterward (observation mutates nothing).
func TestObserveEpochLaunchesNeverStopsOrSettles(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateRunning, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
		r.RawRunDir = "/runs/live"
		r.RawOwnership = "live"
	})
	recPath := filepath.Join(store.root, id, recordFileName)
	before, err := os.ReadFile(recPath)
	if err != nil {
		t.Fatalf("read record before: %v", err)
	}

	report, err := d.ObserveEpochLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ObserveEpochLaunches: %v", err)
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

// TestObserveEpochLaunchesAccountsProvenTerminalRun proves an attached run the fake
// proc proves STOPPED is accounted (run-terminal) in observe mode with no Stop call.
func TestObserveEpochLaunchesAccountsProvenTerminalRun(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateStopped, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
		r.RawRunDir = "/runs/gone"
		r.RawOwnership = "gone"
	})

	report, err := d.ObserveEpochLaunches(sampleWorktree(), "e1")
	if err != nil {
		t.Fatalf("ObserveEpochLaunches: %v", err)
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

// TestObserveEpochLaunchesKeepsNeverLaunchedPending proves observe mode reports a
// reserved-never-launched drive AND a bare (unlaunched, unreserved) drive as pending
// (launch-pending) and — unlike reconcile — NEVER settles the reservation terminal:
// the record's outcome is unchanged (the completing launch-gate refusal settles it
// later, and a replay then accounts).
func TestObserveEpochLaunchesKeepsNeverLaunchedPending(t *testing.T) {
	t.Run("reserved-never-launched", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				return &process.ReservationResolution{Disposition: "never-launched"}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RelaunchReserved = true
			r.RelaunchToken = "aaaaaaaaaaaaaaaa"
		})
		before, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load before: %v", err)
		}

		report, err := d.ObserveEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ObserveEpochLaunches: %v", err)
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
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RawRunDir = ""
			r.RawOwnership = ""
		})
		before, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load before: %v", err)
		}

		report, err := d.ObserveEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ObserveEpochLaunches: %v", err)
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

// TestObserveEpochLaunchesBusyClaimIsPending proves a held claimant flock is reported
// claim-busy and Accounted=false in observe mode, and that observe probes nonblocking
// (returns without waiting on the claim) and stops nothing.
func TestObserveEpochLaunchesBusyClaimIsPending(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	id, _ := seedScopedEpochDrive(t, store, "e1", nil)

	claim, busy, err := store.tryRelaunchClaim(id)
	if err != nil || busy {
		t.Fatalf("tryRelaunchClaim = (busy=%v, err=%v), want a free claim", busy, err)
	}
	defer claim.close()

	done := make(chan EpochLaunchReport, 1)
	go func() {
		r, _ := d.ObserveEpochLaunches(sampleWorktree(), "e1")
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

// TestReconcileEpochLaunchesBehaviorUnchanged is the regression pin: after factoring
// the shared body, the reconcile (cancel) mode still STOPS an identified run and
// still SETTLES a proven never-launched reservation terminal HALTED "run-cancelled".
func TestReconcileEpochLaunchesBehaviorUnchanged(t *testing.T) {
	t.Run("identified-run-stopped", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		var stopped []string
		proc := &fakeProc{
			stop: func(runDir, reason string) (*process.StopOutcome, error) {
				stopped = append(stopped, runDir)
				return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RawRunDir = "/runs/replacement"
			r.RawOwnership = "replacement"
			r.RelaunchCount = 1
		})

		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
		}
		if !report.Accounted || !reconcileFindingPresent(report.Findings, "replacement-stopped:"+id) {
			t.Fatalf("reconcile must still stop and account an identified run, findings=%v", report.Findings)
		}
		if len(stopped) != 1 || stopped[0] != "/runs/replacement" {
			t.Fatalf("reconcile stopped %v, want exactly [/runs/replacement]", stopped)
		}
	})

	t.Run("never-launched-settles-terminal", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &fakeProc{
			resolve: func(root, token string) (*process.ReservationResolution, error) {
				return &process.ReservationResolution{Disposition: "never-launched"}, nil
			},
		}
		d, store := newTestDriver(t, clk, proc, stableGit())
		id, _ := seedScopedEpochDrive(t, store, "e1", func(r *driveRecord) {
			r.RelaunchReserved = true
			r.RelaunchToken = "aaaaaaaaaaaaaaaa"
		})

		report, err := d.ReconcileEpochLaunches(sampleWorktree(), "e1")
		if err != nil {
			t.Fatalf("ReconcileEpochLaunches: %v", err)
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
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())

	req := sampleStart()
	req.RunEpochID = "e1"
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	// Release the slot (it preserves the reservation token + epoch), but the drive
	// itself never launched and remains nonterminal.
	if rerr := store.ReleaseWorktreeExecution(req.Worktree, ticket.token); rerr != nil {
		t.Fatalf("ReleaseWorktreeExecution: %v", rerr)
	}

	report, err := d.ReconcileEpochLaunches(req.Worktree, "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches: %v", err)
	}
	if report.Accounted {
		t.Fatalf("a released slot alone must not settle a pending launch, findings=%v", report.Findings)
	}
	if !reconcileFindingPresent(report.Findings, "launch-pending:"+ticket.id) {
		t.Fatalf("findings = %v, want launch-pending:%s", report.Findings, ticket.id)
	}
}
