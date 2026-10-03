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
// 0490). ReconcileRunLaunches / VerdictRunLaunches walk the drive registry,
// attribute each drive to the run whose context hash it stores, and prove each
// attributed drive's supervisors gone — under the per-drive claimant flock for a
// nonterminal drive. No worktree lock or holder note is read.
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
	// leftover[dir] is what ProbeLeftover answers for dir (an unlisted dir is
	// none); leftoverErr[dir] makes it fail. probed records every probed dir.
	leftover    map[string]process.Leftover
	leftoverErr map[string]error
	probed      []string
}

func newSupervisors() *supervisors {
	return &supervisors{
		state: map[string]process.State{}, stuck: map[string]bool{},
		leftover: map[string]process.Leftover{}, leftoverErr: map[string]error{},
	}
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
		leftover: func(runDir string) (process.Leftover, error) {
			s.probed = append(s.probed, runDir)
			lo, ok := s.leftover[runDir]
			if !ok {
				lo = process.Leftover{Answer: process.LeftoverNone}
			}
			return lo, s.leftoverErr[runDir]
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
	}{{"reconcile", d.ReconcileRunLaunches}, {"verdict", d.VerdictRunLaunches}}
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

// TestCensusSettlesNeverAttachedFirstLaunch (L9; verdict mode, change 0491): a
// reserved first launch that never attached a run dir resolves its exact launch
// token; a proven never-launched one is settled and accounted in both modes —
// HALTED run-cancelled in cancel mode, HALTED launch-abandoned in verdict mode —
// and neither mode launches or stops anything.
func TestCensusSettlesNeverAttachedFirstLaunch(t *testing.T) {
	// seed gives the drive an existing run root: a missing root short-circuits the
	// resolve (reconcileFirstLaunch's clean-absence rule, change 0491).
	seed := func(t *testing.T, store *Store) (id, runRoot string) {
		runRoot = testsupport.TempDir(t)
		id, _ = seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir, r.RawOwnership = "", ""
			r.AdmissionToken = censusAdmissionToken
			r.RunRoot = runRoot
		})
		return id, runRoot
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
		id, runRoot := seed(t, store)

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted {
			t.Fatalf("a proven never-launched first launch must be accounted, got %+v", report)
		}
		if token != censusAdmissionToken || root != runRoot {
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

	t.Run("verdict-settles-launch-abandoned", func(t *testing.T) {
		var root, token string
		proc := newProc(&root, &token)
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id, runRoot := seed(t, store)

		report, err := d.VerdictRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("VerdictRunLaunches: %v", err)
		}
		if !report.Accounted || len(report.Findings) != 0 {
			t.Fatalf("the keyed verdict must settle a proven never-launched first launch, got %+v", report)
		}
		if token != censusAdmissionToken || root != runRoot {
			t.Fatalf("resolved (%q, %q), want the drive's run root and launch token", root, token)
		}
		after, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if after.LastOutcome != HALTED || after.LastCause != "launch-abandoned" {
			t.Fatalf("want HALTED launch-abandoned, got %v/%q", after.LastOutcome, after.LastCause)
		}
		if proc.launchN != 0 || proc.stopN != 0 {
			t.Fatalf("the verdict census launches and stops nothing: launch=%d stop=%d", proc.launchN, proc.stopN)
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
		r.RunRoot = testsupport.TempDir(t) // an existing root: the token is resolved
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

// TestCensusResolvesLaunchFailedFirstLaunch (review F1): a first launch whose
// process.Launch returned an error is written HALTED "launch-failed" with no run
// dir — but the launch may have spawned a supervisor before losing its response.
// The census must not settle it by its empty run dirs: it resolves the drive's
// launch token. An identified running run is stopped (cancel) or run-live
// (verdict); a proven never-launched launch is settled in both modes without
// rewriting the already-terminal record; an unresolved verdict or a resolve error
// keeps the run pending; a run root that no longer exists is clean absence.
func TestCensusResolvesLaunchFailedFirstLaunch(t *testing.T) {
	seed := func(t *testing.T, store *Store, runRoot string) string {
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir, r.RawOwnership = "", ""
			r.AdmissionToken = censusAdmissionToken
			r.RunRoot = runRoot
			r.LastOutcome = HALTED
			r.LastCause = "launch-failed"
		})
		return id
	}

	t.Run("identified-running", func(t *testing.T) {
		for _, mode := range []string{"reconcile", "verdict"} {
			t.Run(mode, func(t *testing.T) {
				sup := newSupervisors()
				root := testsupport.TempDir(t)
				dir := liveRunDir(t, "run1")
				sup.state[dir] = process.StateRunning
				var gotRoot, gotToken string
				sup.resolve = func(r, token string) (*process.ReservationResolution, error) {
					gotRoot, gotToken = r, token
					return &process.ReservationResolution{Disposition: "identified", RunID: "run1", RunDir: dir, State: process.StateRunning}, nil
				}
				proc := sup.proc()
				d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
				id := seed(t, store, root)

				if mode == "reconcile" {
					report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
					if err != nil {
						t.Fatalf("ReconcileRunLaunches: %v", err)
					}
					if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
						t.Fatalf("a launch-failed drive whose launch token resolves to a running run must have it stopped, got %+v", report)
					}
					if len(sup.stopped) != 1 || sup.stopped[0] != dir {
						t.Fatalf("stopped %v, want exactly [%s]", sup.stopped, dir)
					}
				} else {
					report, err := d.VerdictRunLaunches(capHash(censusCtxA))
					if err != nil {
						t.Fatalf("VerdictRunLaunches: %v", err)
					}
					if report.Accounted || !findingFor(report.Findings, "run-live", id) {
						t.Fatalf("verdict must keep a launch-failed drive's live supervisor pending, got %+v", report)
					}
					if proc.stopN != 0 {
						t.Fatalf("verdict mode must never stop, Stop called %d times", proc.stopN)
					}
				}
				if gotRoot != root || gotToken != censusAdmissionToken {
					t.Fatalf("resolved (%q, %q), want the drive's run root and launch token", gotRoot, gotToken)
				}
				if proc.launchN != 0 {
					t.Fatalf("the census must launch nothing, Launch called %d times", proc.launchN)
				}
			})
		}
	})

	t.Run("never-launched-settled", func(t *testing.T) {
		proc := &fakeProc{resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "never-launched"}, nil
		}}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		id := seed(t, store, testsupport.TempDir(t))
		recPath := filepath.Join(store.root, id, recordFileName)
		before, err := os.ReadFile(recPath)
		if err != nil {
			t.Fatalf("read record: %v", err)
		}
		for _, mode := range censusModes(d) {
			report, err := mode.run(capHash(censusCtxA))
			if err != nil {
				t.Fatalf("%s: %v", mode.name, err)
			}
			if !report.Accounted || len(report.Findings) != 0 {
				t.Fatalf("%s: a launch-failed drive that provably never launched is settled, got %+v", mode.name, report)
			}
		}
		if proc.resolveN != 2 {
			t.Fatalf("each census must resolve the launch token, ResolveReservation called %d times", proc.resolveN)
		}
		after, err := os.ReadFile(recPath)
		if err != nil {
			t.Fatalf("read record: %v", err)
		}
		if string(before) != string(after) {
			t.Fatal("the census must not rewrite an already-terminal launch-failed record")
		}
	})

	pending := map[string]func(root, token string) (*process.ReservationResolution, error){
		"unresolved": func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "unresolved"}, nil
		},
		"resolve-error": func(root, token string) (*process.ReservationResolution, error) {
			return nil, errors.New("census incomplete")
		},
	}
	for name, resolve := range pending {
		t.Run(name, func(t *testing.T) {
			proc := &fakeProc{resolve: resolve}
			d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
			id := seed(t, store, testsupport.TempDir(t))
			for _, mode := range censusModes(d) {
				report, err := mode.run(capHash(censusCtxA))
				if err != nil {
					t.Fatalf("%s: %v", mode.name, err)
				}
				if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) {
					t.Fatalf("%s: an unresolvable launch-failed launch must keep the run pending, got %+v", mode.name, report)
				}
			}
		})
	}

	t.Run("run-root-absent", func(t *testing.T) {
		proc := &fakeProc{resolve: func(root, token string) (*process.ReservationResolution, error) {
			return nil, errors.New("root must be an existing directory")
		}}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		seed(t, store, filepath.Join(testsupport.TempDir(t), "removed-run-root"))
		for _, mode := range censusModes(d) {
			report, err := mode.run(capHash(censusCtxA))
			if err != nil {
				t.Fatalf("%s: %v", mode.name, err)
			}
			if !report.Accounted {
				t.Fatalf("%s: a run root that no longer exists holds no run dir — clean absence, got %+v", mode.name, report)
			}
		}
		if proc.resolveN != 0 {
			t.Fatalf("an absent run root needs no resolve, ResolveReservation called %d times", proc.resolveN)
		}
	})
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
// settled by its label: cancel mode stops it, and verdict mode reports run-live
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

	t.Run("verdict-live", func(t *testing.T) {
		sup := newSupervisors()
		proc := sup.proc()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		dir := liveRunDir(t, "run1")
		sup.state[dir] = process.StateRunning
		id := seed(t, store, dir)

		report, err := d.VerdictRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("VerdictRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "run-live", id) {
			t.Fatalf("verdict must keep a HALTED drive's live supervisor pending, got %+v", report)
		}
		if proc.stopN != 0 {
			t.Fatalf("verdict mode must never stop, Stop called %d times", proc.stopN)
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

// TestCensusProvesOnlyRawRunDir (change 0493): a drive has at most one launch,
// so the census proves only RawRunDir. A HALTED drive whose recorded run dir is
// live is stopped by cancel (never probed for a leftover once stopped) and
// reported run-live by the verdict; a prior_raw_run_dir an old record still
// carries is never observed. An old nonterminal record still journaling a
// relaunch reservation is proven through RawRunDir, and its relaunch token is
// never resolved.
func TestCensusProvesOnlyRawRunDir(t *testing.T) {
	seedHalted := func(t *testing.T, store *Store, raw, prior string) string {
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir = raw
			r.LastOutcome = HALTED
			r.LastCause = CauseSupervisorDied
		})
		stampRecordKeys(t, store, id, map[string]any{"prior_raw_run_dir": prior, "relaunch_count": 1})
		return id
	}
	t.Run("cancel-stops-raw-only", func(t *testing.T) {
		sup := newSupervisors()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		raw, prior := liveRunDir(t, "run1"), liveRunDir(t, "run0")
		sup.state[raw], sup.state[prior] = process.StateRunning, process.StateRunning
		id := seedHalted(t, store, raw, prior)

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
			t.Fatalf("the live RawRunDir must be stopped and accounted, got %+v", report)
		}
		if len(sup.stopped) != 1 || sup.stopped[0] != raw {
			t.Fatalf("stopped %v, want exactly [%s]", sup.stopped, raw)
		}
		if containsString(sup.observed, prior) {
			t.Fatalf("an old prior_raw_run_dir must never be probed; observed %v", sup.observed)
		}
		if containsString(sup.probed, raw) {
			t.Fatalf("a dir the census stopped is never probed for a leftover; probed %v", sup.probed)
		}
	})
	t.Run("verdict-run-live", func(t *testing.T) {
		sup := newSupervisors()
		proc := sup.proc()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		raw, prior := liveRunDir(t, "run1"), liveRunDir(t, "run0")
		sup.state[raw], sup.state[prior] = process.StateRunning, process.StateRunning
		id := seedHalted(t, store, raw, prior)

		report, err := d.VerdictRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("VerdictRunLaunches: %v", err)
		}
		if report.Accounted || !findingFor(report.Findings, "run-live", id) {
			t.Fatalf("verdict must report the live RawRunDir run-live, got %+v", report)
		}
		if proc.stopN != 0 || containsString(sup.observed, prior) {
			t.Fatalf("verdict stops nothing and never probes prior_raw_run_dir: stops=%d observed=%v", proc.stopN, sup.observed)
		}
	})
	t.Run("old-reserved-relaunch-record", func(t *testing.T) {
		sup := newSupervisors()
		proc := sup.proc()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		raw := liveRunDir(t, "run1")
		sup.state[raw] = process.StateRunning
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir = raw
			r.AdmissionToken = censusAdmissionToken
		})
		stampRecordKeys(t, store, id, legacyRelaunchKeys())

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "replacement-stopped", id) {
			t.Fatalf("an old reserved record is proven through RawRunDir, got %+v", report)
		}
		if proc.resolveN != 0 {
			t.Fatalf("an old relaunch token is never resolved, ResolveReservation called %d times", proc.resolveN)
		}
	})
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
			r.RunRoot = testsupport.TempDir(t) // an existing root: the token is resolved
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

// TestCensusMissingRunRootIsNeverLaunched (change 0491, Decision 4): a first launch
// whose RunRoot does not exist (the launcher died before process.Launch made it, or
// a temp dir was cleaned) never resolves its token — ResolveReservation would refuse
// a missing root forever. Both modes read the clean absence as never launched and
// settle it: run-cancelled in cancel mode, launch-abandoned in verdict mode.
func TestCensusMissingRunRootIsNeverLaunched(t *testing.T) {
	for _, tc := range []struct{ mode, cause string }{
		{"reconcile", "run-cancelled"},
		{"verdict", "launch-abandoned"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			proc := &fakeProc{}
			d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
			missing := filepath.Join(testsupport.TempDir(t), "never-created")
			id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
				r.RawRunDir, r.RawOwnership = "", ""
				r.AdmissionToken = censusAdmissionToken
				r.RunRoot = missing
			})
			run := d.ReconcileRunLaunches
			if tc.mode == "verdict" {
				run = d.VerdictRunLaunches
			}
			report, err := run(capHash(censusCtxA))
			if err != nil {
				t.Fatalf("%s: %v", tc.mode, err)
			}
			if !report.Accounted || len(report.Findings) != 0 {
				t.Fatalf("%s: a missing run root is never launched and settled, got %+v", tc.mode, report)
			}
			if proc.resolveN != 0 {
				t.Fatalf("%s: a missing root must not be resolved, ResolveReservation called %d times", tc.mode, proc.resolveN)
			}
			after, err := store.Load(id)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if after.LastOutcome != HALTED || after.LastCause != tc.cause {
				t.Fatalf("%s: want HALTED %s, got %v/%q", tc.mode, tc.cause, after.LastOutcome, after.LastCause)
			}
		})
	}
}

// TestCensusRunRootProbeErrorStaysUnresolved (change 0491, Review Focus 1): a run
// root whose Lstat fails for a reason other than not-exist is unknown, not absent —
// it stays resolution-unresolved in both modes and the record is never settled
// (learning probe-error-is-not-clean-absence).
func TestCensusRunRootProbeErrorStaysUnresolved(t *testing.T) {
	file := filepath.Join(testsupport.TempDir(t), "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	notDir := filepath.Join(file, "root") // Lstat fails ENOTDIR, not ErrNotExist
	for _, mode := range []string{"reconcile", "verdict"} {
		t.Run(mode, func(t *testing.T) {
			d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
			id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
				r.RawRunDir, r.RawOwnership = "", ""
				r.AdmissionToken = censusAdmissionToken
				r.RunRoot = notDir
			})
			run := d.ReconcileRunLaunches
			if mode == "verdict" {
				run = d.VerdictRunLaunches
			}
			report, err := run(capHash(censusCtxA))
			if err != nil {
				t.Fatalf("%s: %v", mode, err)
			}
			if report.Accounted || !findingFor(report.Findings, "resolution-unresolved", id) {
				t.Fatalf("%s: a run-root probe error must stay unresolved, got %+v", mode, report)
			}
			after, err := store.Load(id)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if isTerminalOutcome(after.LastOutcome) {
				t.Fatalf("%s: a probe error must never settle the record, got %v/%q", mode, after.LastOutcome, after.LastCause)
			}
		})
	}
}

// TestCensusVerdictLeavesBusyClaimPending (change 0491, Review Focus 2): a launcher
// still alive and holding the drive's claim (StartAdmitted mid-launch) is pending
// work. The keyed verdict reports claim-busy, settles nothing, and never waits.
func TestCensusVerdictLeavesBusyClaimPending(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
		r.RawRunDir, r.RawOwnership = "", ""
		r.AdmissionToken = censusAdmissionToken
		r.RunRoot = testsupport.TempDir(t)
	})
	held, busy, err := store.tryRelaunchClaim(id)
	if err != nil || busy {
		t.Fatalf("precondition: take the claim: busy=%v err=%v", busy, err)
	}
	defer held.close()

	report, err := d.VerdictRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("VerdictRunLaunches: %v", err)
	}
	if report.Accounted || !findingFor(report.Findings, "claim-busy", id) {
		t.Fatalf("a held claim must read claim-busy, got %+v", report)
	}
	after, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if isTerminalOutcome(after.LastOutcome) {
		t.Fatalf("the verdict must not settle a drive whose launcher holds the claim, got %v", after.LastOutcome)
	}
}

// TestCensusVerdictSettlesOnlyItsOwnRun (change 0491, Review Focus 3): the keyed
// verdict settles only drives attributed to its own run context; another run's
// never-launched drive is untouched.
func TestCensusVerdictSettlesOnlyItsOwnRun(t *testing.T) {
	d, store := newTestDriver(t, &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
	other, _ := seedRunDrive(t, store, censusCtxB, func(r *driveRecord) {
		r.RawRunDir, r.RawOwnership = "", ""
		r.AdmissionToken = censusAdmissionToken
		r.RunRoot = testsupport.TempDir(t)
	})
	report, err := d.VerdictRunLaunches(capHash(censusCtxA))
	if err != nil {
		t.Fatalf("VerdictRunLaunches: %v", err)
	}
	if !report.Accounted || len(report.Findings) != 0 {
		t.Fatalf("a run with no drives of its own accounts vacuously, got %+v", report)
	}
	after, err := store.Load(other)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if isTerminalOutcome(after.LastOutcome) {
		t.Fatalf("another run's drive must not be settled, got %v/%q", after.LastOutcome, after.LastCause)
	}
}

// containsString reports whether xs holds s.
func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// TestCensusReportsLeftoverOfDeadSupervisor (change 0492): a dead supervisor
// whose recorded group still has members is STILL torn down — the drive stays
// settled in both modes and nothing is stopped — but the finding names the
// surviving group, tree-survives:<drive>:<pgid>, instead of run-terminal. This
// holds for a nonterminal drive and for the HALTED drive a killed supervisor's
// slice actually leaves behind.
//
// Mutation check: making supervisorGone return false on a leftover must turn
// this red (the failure posture: a leftover never changes the outcome).
func TestCensusReportsLeftoverOfDeadSupervisor(t *testing.T) {
	for _, st := range []process.State{process.StateVanished, process.StateSignaled, process.StateStopped} {
		for _, halted := range []bool{false, true} {
			name := string(st)
			if halted {
				name += "/halted"
			}
			t.Run(name, func(t *testing.T) {
				sup := newSupervisors()
				proc := sup.proc()
				d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
				dir := liveRunDir(t, "run1")
				sup.state[dir] = st
				sup.leftover[dir] = process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}
				id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
					r.RawRunDir = dir
					if halted {
						r.LastOutcome = HALTED
						r.LastCause = CauseSupervisorDied
					}
				})

				for _, mode := range censusModes(d) {
					report, err := mode.run(capHash(censusCtxA))
					if err != nil {
						t.Fatalf("%s: %v", mode.name, err)
					}
					if !report.Accounted || !findingFor(report.Findings, "tree-survives", id+":4242") {
						t.Fatalf("%s: a leftover is settled and reported tree-survives:%s:4242, got %+v", mode.name, id, report)
					}
					if findingFor(report.Findings, "run-terminal", id) {
						t.Fatalf("%s: tree-survives replaces run-terminal for the drive, got %+v", mode.name, report)
					}
				}
				if proc.stopN != 0 {
					t.Fatalf("a leftover is never stopped, Stop called %d times", proc.stopN)
				}
			})
		}
	}
}

// TestCensusNoLeftoverKeepsRunTerminal (change 0492): none, unclear, and a probe
// error all keep today's run-terminal:<drive>. The error row answers leftover
// WITH an error — the census trusts an answer only when the probe succeeded.
func TestCensusNoLeftoverKeepsRunTerminal(t *testing.T) {
	for _, tc := range []struct {
		name string
		lo   process.Leftover
		err  error
	}{
		{"none", process.Leftover{Answer: process.LeftoverNone, PGID: 4242}, nil},
		{"unclear", process.Leftover{Answer: process.LeftoverUnclear, PGID: 4242}, nil},
		{"error", process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}, errors.New("probe failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sup := newSupervisors()
			d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
			dir := liveRunDir(t, "run1")
			sup.state[dir] = process.StateVanished
			sup.leftover[dir] = tc.lo
			if tc.err != nil {
				sup.leftoverErr[dir] = tc.err
			}
			id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = dir })

			for _, mode := range censusModes(d) {
				report, err := mode.run(capHash(censusCtxA))
				if err != nil {
					t.Fatalf("%s: %v", mode.name, err)
				}
				if !report.Accounted || !findingFor(report.Findings, "run-terminal", id) {
					t.Fatalf("%s: %s keeps run-terminal:%s, got %+v", mode.name, tc.name, id, report)
				}
				if reconcileFindingPresent(report.Findings, "tree-survives:") {
					t.Fatalf("%s: %s must never report tree-survives, got %+v", mode.name, tc.name, report)
				}
			}
		})
	}
}
