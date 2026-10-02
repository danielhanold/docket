package gatedrive

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// ---------------------------------------------------------------------------
// The run launch census (change 0437 Task 6; attribution by run context, change
// 0490). ReconcileRunLaunches / ObserveRunLaunches walk the drive registry,
// attribute each drive to the run whose context hash it stores, and prove each
// attributed drive's supervisors gone — under the per-drive claimant flock for a
// nonterminal drive. No worktree slot is read.
// ---------------------------------------------------------------------------

const (
	censusCtxA = "ctx-A" // the run under test
	censusCtxB = "ctx-B" // another run in the same repository
)

// censusAdmissionToken is a well-formed launch (admission) token a seeded first
// launch carries; the census resolves exactly this reservation.
const censusAdmissionToken = "bbbbbbbbbbbbbbbb"

// reconcileFindingPresent reports whether any finding starts with prefix.
func reconcileFindingPresent(findings []string, prefix string) bool {
	for _, f := range findings {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
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

// seedRunDrive persists a drive started inside the run whose raw context is ctx
// (RunContextHash = capHash(ctx)), the way a tracked gate.drive.start stores it.
// seedRecord's RawRunDir names a path that does not exist; mutate sets the run
// dirs, outcome, and reservation state a case needs.
func seedRunDrive(t *testing.T, store *Store, ctx string, mutate func(*driveRecord)) (id, ownerGen string) {
	t.Helper()
	rec := seedRecord(t)
	if ctx != "" {
		rec.RunContextHash = capHash(ctx)
	}
	if mutate != nil {
		mutate(&rec)
	}
	id, _, err := store.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	return id, rec.OwnerGeneration
}

// liveRunDir creates a run dir that exists on disk, so the census must Observe it
// rather than treat it as clean absence.
func liveRunDir(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(testsupport.TempDir(t), name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	return dir
}

// supervisors scripts the process seam over real run dirs: state[dir] is what
// Observe reports (an unlisted dir observes vanished), and Stop moves a running
// dir to stopped unless the dir is stuck. It records every observe and stop.
type supervisors struct {
	state    map[string]process.State
	stuck    map[string]bool
	observed []string
	stopped  []string
	resolve  func(root, token string) (*process.ReservationResolution, error)
}

func newSupervisors() *supervisors {
	return &supervisors{state: map[string]process.State{}, stuck: map[string]bool{}}
}

func (s *supervisors) proc() *fakeProc {
	return &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			s.observed = append(s.observed, runDir)
			st, ok := s.state[runDir]
			if !ok {
				st = process.StateVanished
			}
			return obs(st, runDir), nil
		},
		stop: func(runDir, reason string) (*process.StopOutcome, error) {
			s.stopped = append(s.stopped, runDir)
			if s.state[runDir] == process.StateRunning && !s.stuck[runDir] {
				s.state[runDir] = process.StateStopped
			}
			return &process.StopOutcome{State: s.state[runDir], RunDir: runDir, Performed: true}, nil
		},
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			if s.resolve == nil {
				return &process.ReservationResolution{Disposition: "never-launched"}, nil
			}
			return s.resolve(root, token)
		},
	}
}

// censusModes runs both census entry points over one driver.
func censusModes(d *Driver) []struct {
	name string
	run  func(string) (RunLaunchReport, error)
} {
	return []struct {
		name string
		run  func(string) (RunLaunchReport, error)
	}{{"reconcile", d.ReconcileRunLaunches}, {"observe", d.ObserveRunLaunches}}
}

// TestCensusStopsRunningDriveOfTheRun (L9): a nonterminal drive of the run whose
// supervisor observes running is stopped, re-observed not running, and accounted.
func TestCensusStopsRunningDriveOfTheRun(t *testing.T) {
	sup := newSupervisors()
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
	dir := liveRunDir(t, "run1")
	sup.state[dir] = process.StateRunning
	id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = dir })

	report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
		t.Fatalf("a running drive of the run must be stopped and accounted, got %+v", report)
	}
	if len(sup.stopped) != 1 || sup.stopped[0] != dir {
		t.Fatalf("stopped %v, want exactly [%s]", sup.stopped, dir)
	}
}

// TestCensusSupervisorAlreadyGoneIsAccounted (L9): a supervisor that already
// vanished — or was signaled — is torn down; no stop is issued in either mode.
func TestCensusSupervisorAlreadyGoneIsAccounted(t *testing.T) {
	for _, st := range []process.State{process.StateVanished, process.StateSignaled} {
		t.Run(string(st), func(t *testing.T) {
			sup := newSupervisors()
			proc := sup.proc()
			d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
			dir := liveRunDir(t, "run1")
			sup.state[dir] = st
			id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = dir })

			for _, mode := range censusModes(d) {
				report, err := mode.run(capHash(censusCtxA))
				if err != nil {
					t.Fatalf("%s: %v", mode.name, err)
				}
				if !report.Accounted || !findingFor(report.Findings, "run-terminal", id) {
					t.Fatalf("%s: a %s supervisor is torn down, got %+v", mode.name, st, report)
				}
			}
			if proc.stopN != 0 {
				t.Fatalf("an exited supervisor must never be stopped, Stop called %d times", proc.stopN)
			}
		})
	}
}

// TestCensusSettlesNeverAttachedFirstLaunch (L9): a reserved first launch that
// never attached a run dir resolves its exact launch token; a proven never-launched
// one is settled HALTED run-cancelled in cancel mode and accounted. In observe mode
// the same drive is launch-pending and its record is left untouched.
func TestCensusSettlesNeverAttachedFirstLaunch(t *testing.T) {
	seed := func(t *testing.T, store *Store) string {
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir, r.RawOwnership = "", ""
			r.AdmissionToken = censusAdmissionToken
		})
		return id
	}
	newProc := func(gotRoot, gotToken *string) *fakeProc {
		return &fakeProc{resolve: func(root, token string) (*process.ReservationResolution, error) {
			*gotRoot, *gotToken = root, token
			return &process.ReservationResolution{Disposition: "never-launched"}, nil
		}}
	}

	t.Run("cancel-settles", func(t *testing.T) {
		var root, token string
		proc := newProc(&root, &token)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id := seed(t, store)

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted {
			t.Fatalf("a proven never-launched first launch must be accounted, got %+v", report)
		}
		if token != censusAdmissionToken || root != seedRecord(t).RunRoot {
			t.Fatalf("resolved (%q, %q), want the drive's run root and launch token", root, token)
		}
		after, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if after.LastOutcome != HALTED || after.LastCause != "run-cancelled" {
			t.Fatalf("the never-launched first launch must settle HALTED run-cancelled, got %v/%q", after.LastOutcome, after.LastCause)
		}
		if proc.launchN != 0 {
			t.Fatalf("the census must launch nothing, Launch called %d times", proc.launchN)
		}
	})

	t.Run("observe-pending-unchanged", func(t *testing.T) {
		var root, token string
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, newProc(&root, &token), stableGit())
		id := seed(t, store)
		recPath := filepath.Join(store.root, id, recordFileName)
		before, err := os.ReadFile(recPath)
		if err != nil {
			t.Fatalf("read record: %v", err)
		}

		report, err := d.ObserveRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ObserveRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "launch-pending", id) {
			t.Fatalf("observe must report the never-launched first launch pending, got %+v", report)
		}
		after, err := os.ReadFile(recPath)
		if err != nil {
			t.Fatalf("read record: %v", err)
		}
		if string(before) != string(after) {
			t.Fatal("observe mode must not mutate the drive record")
		}
	})
}

// TestCensusStopsIdentifiedFirstLaunch (L9): a first launch whose response was
// lost but whose reservation resolves to an identified, running run is stopped and
// accounted.
func TestCensusStopsIdentifiedFirstLaunch(t *testing.T) {
	sup := newSupervisors()
	dir := liveRunDir(t, "run1")
	sup.state[dir] = process.StateRunning
	sup.resolve = func(root, token string) (*process.ReservationResolution, error) {
		if token != censusAdmissionToken {
			t.Errorf("resolved token %q, want the launch token", token)
		}
		return &process.ReservationResolution{Disposition: "identified", RunID: "run1", RunDir: dir, State: process.StateRunning}, nil
	}
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
	id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
		r.RawRunDir, r.RawOwnership = "", ""
		r.AdmissionToken = censusAdmissionToken
	})

	report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
		t.Fatalf("an identified first launch must be stopped and accounted, got %+v", report)
	}
	if len(sup.stopped) != 1 || sup.stopped[0] != dir {
		t.Fatalf("stopped %v, want exactly [%s]", sup.stopped, dir)
	}
}

// TestCensusPendingOnClaimBusy (L9): a held claimant flock (a launch in flight)
// is claim-busy and not accounted in either mode, and the census returns without
// waiting on the claim.
func TestCensusPendingOnClaimBusy(t *testing.T) {
	proc := &fakeProc{}
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
	id, _ := seedRunDrive(t, store, censusCtxA, nil)

	claim, busy, err := store.tryRelaunchClaim(id)
	if err != nil || busy {
		t.Fatalf("tryRelaunchClaim = (busy=%v, err=%v), want a free claim", busy, err)
	}
	defer claim.close()

	for _, mode := range censusModes(d) {
		done := make(chan RunLaunchReport, 1)
		go func() {
			r, _ := mode.run(capHash(censusCtxA))
			done <- r
		}()
		select {
		case report := <-done:
			if report.Accounted || !findingFor(report.Findings, "claim-busy", id) {
				t.Fatalf("%s: a busy claim must be pending claim-busy:%s, got %+v", mode.name, id, report)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the census blocked on a busy claim; it must probe nonblocking", mode.name)
		}
	}
	if proc.launchN != 0 || proc.stopN != 0 || proc.observeN != 0 {
		t.Fatalf("a busy drive must not be launched, stopped, or observed: launch=%d stop=%d observe=%d", proc.launchN, proc.stopN, proc.observeN)
	}
}

// TestCensusStopsLiveSupervisorOfHaltedDrive (Review Focus 1): a HALTED drive
// whose supervisor is still running (deadline-expired-stop-unproven) is not
// settled by its label: cancel mode stops it, and observe mode reports run-live
// and keeps the run unaccounted.
func TestCensusStopsLiveSupervisorOfHaltedDrive(t *testing.T) {
	seed := func(t *testing.T, store *Store, dir string) string {
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir = dir
			r.LastOutcome = HALTED
			r.LastCause = "deadline-expired-stop-unproven"
		})
		return id
	}

	t.Run("cancel-stops", func(t *testing.T) {
		sup := newSupervisors()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		dir := liveRunDir(t, "run1")
		sup.state[dir] = process.StateRunning
		id := seed(t, store, dir)

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
			t.Fatalf("a HALTED drive's live supervisor must be stopped, got %+v", report)
		}
		if len(sup.stopped) != 1 || sup.stopped[0] != dir {
			t.Fatalf("stopped %v, want exactly [%s]", sup.stopped, dir)
		}
	})

	t.Run("observe-live", func(t *testing.T) {
		sup := newSupervisors()
		proc := sup.proc()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		dir := liveRunDir(t, "run1")
		sup.state[dir] = process.StateRunning
		id := seed(t, store, dir)

		report, err := d.ObserveRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ObserveRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "run-live", id) {
			t.Fatalf("observe must keep a HALTED drive's live supervisor pending, got %+v", report)
		}
		if proc.stopN != 0 {
			t.Fatalf("observe mode must never stop, Stop called %d times", proc.stopN)
		}
	})

	t.Run("stop-does-not-take", func(t *testing.T) {
		sup := newSupervisors()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		dir := liveRunDir(t, "run1")
		sup.state[dir] = process.StateRunning
		sup.stuck[dir] = true
		id := seed(t, store, dir)

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "run-live", id) {
			t.Fatalf("a supervisor still running after the stop must keep cancel pending, got %+v", report)
		}
	})
}

// TestCensusChecksPriorRunDir: a relaunched drive records both its replacement
// (RawRunDir) and its first attempt (PriorRawRunDir); a live supervisor in either
// is stopped.
func TestCensusChecksPriorRunDir(t *testing.T) {
	sup := newSupervisors()
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
	replacement, prior := liveRunDir(t, "run2"), liveRunDir(t, "run1")
	sup.state[replacement] = process.StateRunning
	sup.state[prior] = process.StateRunning
	id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
		r.RawRunDir, r.PriorRawRunDir = replacement, prior
		r.RelaunchCount = 1
	})

	report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
		t.Fatalf("both run dirs proven gone must account, got %+v", report)
	}
	if len(sup.stopped) != 2 || sup.stopped[0] != replacement || sup.stopped[1] != prior {
		t.Fatalf("stopped %v, want [%s %s]", sup.stopped, replacement, prior)
	}
}

// TestCensusRunDirAbsentIsTornDownButProbeErrorIsPending (Review Focus 2): a run
// dir that no longer exists is clean absence — accounted with no Observe call —
// while an existing run dir whose Observe errors is unprovable and keeps the run
// pending (a probe error is never clean absence).
func TestCensusRunDirAbsentIsTornDownButProbeErrorIsPending(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		proc := &fakeProc{}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		gone := filepath.Join(testsupport.TempDir(t), "removed-run")
		seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = gone })

		for _, mode := range censusModes(d) {
			report, err := mode.run(capHash(censusCtxA))
			if err != nil {
				t.Fatalf("%s: %v", mode.name, err)
			}
			if !report.Accounted {
				t.Fatalf("%s: an absent run dir is torn down, got %+v", mode.name, report)
			}
		}
		if proc.observeN != 0 || proc.stopN != 0 {
			t.Fatalf("an absent run dir must not be observed or stopped: observe=%d stop=%d", proc.observeN, proc.stopN)
		}
	})

	t.Run("probe-error", func(t *testing.T) {
		proc := &fakeProc{observe: func(string) (*process.Observation, error) {
			return nil, errors.New("gatedrive-test: observe fault")
		}}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		dir := liveRunDir(t, "run1")
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = dir })

		for _, mode := range censusModes(d) {
			report, err := mode.run(capHash(censusCtxA))
			if err != nil {
				t.Fatalf("%s: %v", mode.name, err)
			}
			if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) {
				t.Fatalf("%s: a probe error must keep the run pending, got %+v", mode.name, report)
			}
		}
		if proc.stopN != 0 {
			t.Fatalf("an unprovable probe must not be followed by a stop, Stop called %d times", proc.stopN)
		}
	})

	t.Run("unknown-state", func(t *testing.T) {
		proc := &fakeProc{observe: func(runDir string) (*process.Observation, error) {
			return obs("", runDir), nil
		}}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		dir := liveRunDir(t, "run1")
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = dir })

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) {
			t.Fatalf("an unknown observed state proves nothing, got %+v", report)
		}
	})
}

// TestCensusIgnoresOtherRunsDrives (L10): a running drive of another run (a
// different context hash) and a running drive started outside any run, in the
// same worktree, are never observed or stopped when reconciling the run; only the
// run's own drive is.
func TestCensusIgnoresOtherRunsDrives(t *testing.T) {
	sup := newSupervisors()
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
	mine, theirs, untracked := liveRunDir(t, "mine"), liveRunDir(t, "theirs"), liveRunDir(t, "untracked")
	for _, dir := range []string{mine, theirs, untracked} {
		sup.state[dir] = process.StateRunning
	}
	id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = mine })
	other, _ := seedRunDrive(t, store, censusCtxB, func(r *driveRecord) { r.RawRunDir = theirs })
	raw, _ := seedRunDrive(t, store, "", func(r *driveRecord) { r.RawRunDir = untracked })

	report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
		t.Fatalf("the run's own drive must be stopped and accounted, got %+v", report)
	}
	for _, dir := range append(append([]string{}, sup.observed...), sup.stopped...) {
		if dir == theirs || dir == untracked {
			t.Fatalf("the census touched a drive that is not the run's (%s): observed=%v stopped=%v", dir, sup.observed, sup.stopped)
		}
	}
	for _, f := range report.Findings {
		if strings.HasSuffix(f, ":"+other) || strings.HasSuffix(f, ":"+raw) {
			t.Fatalf("another run's drive must produce no finding, got %v", report.Findings)
		}
	}
	if sup.state[theirs] != process.StateRunning || sup.state[untracked] != process.StateRunning {
		t.Fatal("another run's gate must still be running")
	}
}

// TestCensusUnreadableRecordIsInformational: a corrupt record is named by
// nothing (its context hash cannot be read), so it is history-unattributed and the
// run stays accounted in both modes.
func TestCensusUnreadableRecordIsInformational(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	id, _ := seedRunDrive(t, store, censusCtxA, nil)
	corruptFile(t, filepath.Join(store.root, id, recordFileName))

	for _, mode := range censusModes(d) {
		report, err := mode.run(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("%s: %v", mode.name, err)
		}
		if !report.Accounted || !findingFor(report.Findings, "history-unattributed", id) {
			t.Fatalf("%s: an unreadable record is informational history, got %+v", mode.name, report)
		}
	}
}

// TestCensusPassedFailedAreSettled: the run's PASSED and FAILED drives are settled
// by their verdict — accounted with no Observe, no Stop, and no finding, even when
// their run dirs still exist.
func TestCensusPassedFailedAreSettled(t *testing.T) {
	proc := &fakeProc{}
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
	for _, out := range []Outcome{PASSED, FAILED} {
		dir := liveRunDir(t, string(out))
		seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir = dir
			r.LastOutcome = out
		})
	}

	for _, mode := range censusModes(d) {
		report, err := mode.run(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("%s: %v", mode.name, err)
		}
		if !report.Accounted || len(report.Findings) != 0 {
			t.Fatalf("%s: PASSED/FAILED drives are settled, got %+v", mode.name, report)
		}
	}
	if proc.observeN != 0 || proc.stopN != 0 {
		t.Fatalf("a settled verdict needs no probe: observe=%d stop=%d", proc.observeN, proc.stopN)
	}
}

// TestCensusEmptyContextAccountsVacuously: a run with no context hash names no
// drive, so the census accounts vacuously without walking the registry (a corrupt
// record that a walk would report stays unreported).
func TestCensusEmptyContextAccountsVacuously(t *testing.T) {
	proc := &fakeProc{}
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
	seedRunDrive(t, store, "", func(r *driveRecord) { r.RawRunDir = liveRunDir(t, "run1") })
	corrupt, _ := seedRunDrive(t, store, censusCtxA, nil)
	corruptFile(t, filepath.Join(store.root, corrupt, recordFileName))

	for _, mode := range censusModes(d) {
		report, err := mode.run("")
		if err != nil {
			t.Fatalf("%s: %v", mode.name, err)
		}
		if !report.Accounted || len(report.Findings) != 0 {
			t.Fatalf("%s: an empty context accounts vacuously, got %+v", mode.name, report)
		}
	}
	if proc.observeN != 0 || proc.stopN != 0 {
		t.Fatalf("an empty context probes nothing: observe=%d stop=%d", proc.observeN, proc.stopN)
	}
}

// TestCensusReservedRelaunchResolvesRelaunchToken: a reserved-but-unattached
// relaunch resolves the RELAUNCH token (never the launch token): never-launched
// settles HALTED run-cancelled preserving the consumed reservation (cancel) or
// stays launch-pending (observe); identified is stopped; unresolved stays pending.
func TestCensusReservedRelaunchResolvesRelaunchToken(t *testing.T) {
	const relaunchToken = "aaaaaaaaaaaaaaaa"
	seed := func(t *testing.T, store *Store) string {
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.AdmissionToken = censusAdmissionToken
			r.RelaunchReserved = true
			r.RelaunchToken = relaunchToken
		})
		return id
	}
	resolving := func(sup *supervisors, disp, runDir string, seen *string) {
		sup.resolve = func(root, token string) (*process.ReservationResolution, error) {
			*seen = token
			return &process.ReservationResolution{Disposition: disp, RunID: "rep", RunDir: runDir}, nil
		}
	}

	t.Run("never-launched-settles", func(t *testing.T) {
		var seen string
		sup := newSupervisors()
		resolving(sup, "never-launched", "", &seen)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		id := seed(t, store)
		before, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if seen != relaunchToken || !report.Accounted {
			t.Fatalf("resolved %q accounted=%v, want the relaunch token and accounted (findings=%v)", seen, report.Accounted, report.Findings)
		}
		after, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if after.LastOutcome != HALTED || after.LastCause != "run-cancelled" {
			t.Fatalf("want HALTED run-cancelled, got %v/%q", after.LastOutcome, after.LastCause)
		}
		if !after.RelaunchReserved || after.RelaunchToken != before.RelaunchToken || after.RelaunchCount != before.RelaunchCount {
			t.Fatalf("the settle must preserve the consumed reservation: before=%+v after=%+v", before, after)
		}
	})

	t.Run("never-launched-observe-pending", func(t *testing.T) {
		var seen string
		sup := newSupervisors()
		resolving(sup, "never-launched", "", &seen)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		id := seed(t, store)

		report, err := d.ObserveRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ObserveRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "launch-pending", id) {
			t.Fatalf("observe must keep a never-launched relaunch pending, got %+v", report)
		}
		after, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if isTerminalOutcome(after.LastOutcome) {
			t.Fatalf("observe must never settle the reservation, got %v", after.LastOutcome)
		}
	})

	t.Run("identified-stops", func(t *testing.T) {
		var seen string
		sup := newSupervisors()
		dir := liveRunDir(t, "rep")
		sup.state[dir] = process.StateRunning
		resolving(sup, "identified", dir, &seen)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		id := seed(t, store)

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if seen != relaunchToken || !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
			t.Fatalf("resolved %q, want an identified relaunch stopped and accounted, got %+v", seen, report)
		}
	})

	t.Run("unresolved-pending", func(t *testing.T) {
		var seen string
		sup := newSupervisors()
		resolving(sup, "unresolved", "", &seen)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		id := seed(t, store)

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) {
			t.Fatalf("an unresolved relaunch must stay pending, got %+v", report)
		}
	})
}

// TestReconcileNeverLaunchedSettlesTerminalClosingRecoveryLaunchWindow proves the
// launch-after-cancel window (spec AC4) is closed: when reconcile resolves a
// reserved relaunch never-launched UNDER THE HELD CLAIM, it settles the drive
// terminal HALTED "run-cancelled" before releasing the claim, so a SUBSEQUENT
// Advance recovery on the same drive launches NOTHING — even though that recovery
// checks no run at all (the relaunch crosses no run launch gate since change 0490,
// so this terminal settle is what closes the window). The oracle is a strict
// ordering (reconcile fully returns before Advance runs) plus the proc.Launch count
// — never a timing sleep.
func TestReconcileNeverLaunchedSettlesTerminalClosingRecoveryLaunchWindow(t *testing.T) {
	recProc := &fakeProc{
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "never-launched"}, nil
		},
	}
	recDriver, store := newTestDriver(t, &fakeClock{now: startRun()}, recProc, stableGit())
	id, ownerGen := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
		r.RelaunchReserved = true
		r.RelaunchToken = "aaaaaaaaaaaaaaaa"
	})

	report, err := recDriver.ReconcileRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a proven never-launched reserved relaunch must be accounted, findings=%v", report.Findings)
	}

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
		t.Fatalf("a recovery after a completed cancellation must launch NOTHING (AC4), proc.Launch called %d times", advProc.launchN)
	}
	if doc.Outcome != HALTED || doc.Cause != "run-cancelled" {
		t.Fatalf("the terminal settle must resolve the recovery Advance to HALTED run-cancelled, got %s/%q", doc.Outcome, doc.Cause)
	}
}

// TestCensusFailuresPreserveEvidence: a resolution error and a stop error each
// keep the run pending and leave the drive record's outcome untouched.
func TestCensusFailuresPreserveEvidence(t *testing.T) {
	t.Run("resolution-error", func(t *testing.T) {
		proc := &fakeProc{resolve: func(root, token string) (*process.ReservationResolution, error) {
			return nil, errors.New("gatedrive-test: resolve fault")
		}}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir, r.RawOwnership = "", ""
			r.AdmissionToken = censusAdmissionToken
		})
		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) {
			t.Fatalf("a resolve fault must keep the run pending, got %+v", report)
		}
		if after, _ := store.Load(id); isTerminalOutcome(after.LastOutcome) {
			t.Fatalf("a resolve fault must not settle the record, got %v", after.LastOutcome)
		}
	})

	t.Run("stop-error", func(t *testing.T) {
		dir := liveRunDir(t, "run1")
		proc := &fakeProc{stop: func(runDir, reason string) (*process.StopOutcome, error) {
			return nil, errors.New("gatedrive-test: stop fault")
		}}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = dir })
		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) {
			t.Fatalf("a stop fault must keep the run pending, got %+v", report)
		}
	})

	t.Run("missing-launch-token", func(t *testing.T) {
		proc := &fakeProc{}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir, r.RawOwnership = "", ""
		})
		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) || proc.resolveN != 0 {
			t.Fatalf("an unattached launch with no token is unresolvable, got %+v (resolves=%d)", report, proc.resolveN)
		}
	})
}

// TestCensusSchema2HistoricalTerminalSettles: a supported schema-2 record the
// executable reader refuses is read through loadHistoricalDrive — a terminal one is
// settled history (no finding at all), a nonterminal one is informational — never
// record-unreadable.
func TestCensusSchema2HistoricalTerminalSettles(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	passed := copyLegacyFixture(t, store, "passed")
	halted := copyLegacyFixture(t, store, "halted")
	waiting := copyLegacyFixture(t, store, "waiting")

	report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
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

// TestCensusReplayConverges: a supervisor still running after the stop leaves the
// run pending; a later replay — once the stop takes — accounts it with no launch.
func TestCensusReplayConverges(t *testing.T) {
	sup := newSupervisors()
	proc := sup.proc()
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
	dir := liveRunDir(t, "run1")
	sup.state[dir] = process.StateRunning
	sup.stuck[dir] = true
	seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = dir })

	first, err := d.ReconcileRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if first.Accounted {
		t.Fatalf("first reconcile must be pending while the supervisor runs, findings=%v", first.Findings)
	}

	sup.stuck[dir] = false
	second, err := d.ReconcileRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if !second.Accounted {
		t.Fatalf("replay must account once the supervisor is gone, findings=%v", second.Findings)
	}
	if proc.launchN != 0 {
		t.Fatalf("the census must never launch, proc.Launch called %d times", proc.launchN)
	}
}
