package gatedrive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// ---------------------------------------------------------------------------
// Injected seams. Every driver dependency that touches the outside world —
// time, the process supervisor, and git — is faked here so the state machine
// is exercised deterministically with no sleeps for production durations and
// no real repository.
// ---------------------------------------------------------------------------

// fakeClock is a deterministic, manually advanced Clock. Its Now never moves on
// its own; the driver's injected sleep advances it, so a slice bound is reached
// by a fixed number of polls rather than by wall-clock time.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time                  { return c.now }
func (c *fakeClock) Since(t time.Time) time.Duration { return c.now.Sub(t) }
func (c *fakeClock) advance(d time.Duration)         { c.now = c.now.Add(d) }

// fakeProc is a scriptable ProcessSeam. Each method records its call count and
// defers to an injected closure; a nil closure yields a sensible default (a
// fresh running run, a running observation, a performed stop) so a test sets
// only the behavior it cares about.
type fakeProc struct {
	launch  func(process.LaunchRequest) (*process.LaunchOutcome, error)
	observe func(runDir string) (*process.Observation, error)
	stop    func(runDir, reason string) (*process.StopOutcome, error)
	resolve func(root, token string) (*process.ReservationResolution, error)

	launchN, observeN, stopN, resolveN int
}

func (f *fakeProc) Launch(r process.LaunchRequest) (*process.LaunchOutcome, error) {
	f.launchN++
	if f.launch == nil {
		id := fmt.Sprintf("run%d", f.launchN)
		return &process.LaunchOutcome{RunID: id, RunDir: "/runs/" + id, State: process.StateRunning}, nil
	}
	return f.launch(r)
}

func (f *fakeProc) Observe(runDir string) (*process.Observation, error) {
	f.observeN++
	if f.observe == nil {
		return &process.Observation{State: process.StateRunning, RunDir: runDir}, nil
	}
	return f.observe(runDir)
}

func (f *fakeProc) Stop(runDir, reason string) (*process.StopOutcome, error) {
	f.stopN++
	if f.stop == nil {
		return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
	}
	return f.stop(runDir, reason)
}

// ResolveReservation defaults to a proven never-launched verdict — the natural
// outcome of a launch that returned an error with no run dir, so the worktree
// slot is released rather than left blocking. Tests that model a lost launch
// response inject a closure returning "unresolved" or "identified".
func (f *fakeProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	f.resolveN++
	if f.resolve == nil {
		return &process.ReservationResolution{Disposition: "never-launched"}, nil
	}
	return f.resolve(root, token)
}

// ClassifyRun is the legacy-inventory recovery seam. These driver/admission tests
// exercise fresh worktrees with no HALTED legacy history, so the default never
// classifies a run dead; a test that needs otherwise scripts through fakeRecovery.
func (f *fakeProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return process.RecoveryEntry{Disposition: "invalid"}, nil
}

// obs builds a running/terminal observation for a run dir.
func obs(state process.State, runDir string) *process.Observation {
	return &process.Observation{State: state, RunDir: runDir}
}

// fakeGit is a GitSeam whose four reads are fixed strings; changing any one
// changes the computed fingerprint, so a test simulates worktree drift by
// mutating a field between calls. WorktreePaths is empty so ComputeFingerprint
// never walks a real filesystem.
type fakeGit struct {
	head, index, status string
	err                 error
}

func (g *fakeGit) HeadOID(string) (string, error)       { return g.head, g.err }
func (g *fakeGit) IndexEntries(string) ([]byte, error)  { return []byte(g.index), g.err }
func (g *fakeGit) Status(string) ([]byte, error)        { return []byte(g.status), g.err }
func (g *fakeGit) WorktreePaths(string) ([]byte, error) { return nil, g.err }

func stableGit() *fakeGit { return &fakeGit{head: "HEAD1", index: "IDX1", status: "ST1"} }

// ---------------------------------------------------------------------------
// Fixtures.
// ---------------------------------------------------------------------------

const pollTick = time.Millisecond

// newTestDriver wires a driver with injected seams and a short slice of four
// poll ticks; the injected sleep advances the fake clock so a persistently
// running run reaches the slice bound after four polls and returns WAITING.
func newTestDriver(t *testing.T, clk *fakeClock, proc *fakeProc, git GitSeam) (*Driver, *Store) {
	t.Helper()
	store := OpenStore(testsupport.TempDir(t))
	d := NewDriver(store, clk, proc, git)
	d.slice = 4 * pollTick
	d.pollInterval = pollTick
	d.sleep = func(dur time.Duration) { clk.advance(dur) }
	return d, store
}

func startRun() time.Time { return time.Unix(1_000_000, 0).UTC() }

// sampleWorktreeOnce/sampleWorktreeDir back sampleWorktree: a single real,
// canonical, existing directory used as the sample start worktree. Change
// 0375's worktree admission derives its slot key from filepath.EvalSymlinks of the
// worktree root, so a Start now needs a resolvable path (the old "/repo"
// sentinel cannot be symlink-resolved). One shared directory is safe across tests:
// every test owns a fresh Store (a fresh admission root under testsupport.TempDir),
// so its worktree slot is isolated even though the worktree key is shared. The git
// seam is faked in these tests, so ComputeFingerprint never touches the directory —
// only admission's EvalSymlinks does.
var (
	sampleWorktreeOnce sync.Once
	sampleWorktreeDir  string
)

func sampleWorktree() string {
	sampleWorktreeOnce.Do(func() {
		// tempdir-exempt: sample worktree built once under sampleWorktreeOnce and shared across tests for the process lifetime.
		dir, err := os.MkdirTemp("", "gatedrive-sample-worktree-")
		if err != nil {
			panic("gatedrive test: mkdir sample worktree: " + err.Error())
		}
		if resolved, rerr := filepath.EvalSymlinks(dir); rerr == nil {
			dir = resolved
		}
		sampleWorktreeDir = dir
	})
	return sampleWorktreeDir
}

// sampleStart is a well-formed StartRequest for an idempotent suite gate.
func sampleStart() StartRequest {
	return StartRequest{
		RepoDir:             "/repo",
		Worktree:            sampleWorktree(),
		ChangeID:            "0342",
		TaskID:              "task-6",
		Phase:               "build",
		Branch:              "feat/x",
		Ref:                 "refs/heads/feat/x",
		Command:             []string{"go test ./..."},
		Cwd:                 "/repo",
		ConfigProvenance:    "config:finalize.test_command",
		Budget:              30 * time.Minute,
		EnvHash:             "envhash",
		RunRoot:             "/repo/.git/docket/gate-runs",
		IdempotentSuiteGate: true,
	}
}

// seedRecord builds a persisted-ready driveRecord whose fingerprint matches the
// stable git seam, so a directly-seeded drive can be advanced without going
// through Start. The caller tweaks the fields a specific transition needs.
func seedRecord(t *testing.T) driveRecord {
	t.Helper()
	fp, err := ComputeFingerprint("/repo", stableGit())
	if err != nil {
		t.Fatalf("ComputeFingerprint: %v", err)
	}
	start := startRun()
	return driveRecord{
		RepoIdentity:        "/repo",
		WorktreePath:        "/repo",
		ChangeID:            "0342",
		TaskID:              "task-6",
		Phase:               "build",
		Branch:              "feat/x",
		Ref:                 "refs/heads/feat/x",
		HeadOID:             fp.Head,
		Fingerprint:         fp,
		Command:             []string{"go test ./..."},
		Cwd:                 "/repo",
		ConfigProvenance:    "config:finalize.test_command",
		Budget:              30 * time.Minute,
		EnvHash:             "envhash",
		RunRoot:             "/repo/.git/docket/gate-runs",
		IdempotentSuiteGate: true,
		StartedAt:           start,
		UpdatedAt:           start,
		Deadline:            start.Add(30 * time.Minute),
		LastClock:           start,
		ProtocolVersion:     ProtocolVersion,
		RawRunDir:           "/runs/run1",
		RawOwnership:        "run1",
		Attempt:             1,
		OwnerGeneration:     "owner-seed",
	}
}

// seedDrive persists rec and returns its id and the owner generation to advance
// with.
func seedDrive(t *testing.T, store *Store, rec driveRecord) (id, ownerGen string) {
	t.Helper()
	id, _, err := store.NewDrive(rec)
	if err != nil {
		t.Fatalf("NewDrive: %v", err)
	}
	return id, rec.OwnerGeneration
}

// ---------------------------------------------------------------------------
// Start / WAITING slices.
// ---------------------------------------------------------------------------

// TestStartLaunchesAndFirstSliceWaits proves Start launches one raw run and,
// when the run stays running across the slice, returns WAITING with a drive id,
// owner generation, attempt 1, and the fixed deadline — and no raw run dir
// (only PASSED exposes it).
func TestStartLaunchesAndFirstSliceWaits(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{} // default: launch running, observe running
	d, store := newTestDriver(t, clk, proc, stableGit())

	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("a run that stays running across the slice must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	if doc.DriveID == "" || doc.Generation == "" {
		t.Fatalf("WAITING doc must carry a drive id and owner generation: %+v", doc)
	}
	if doc.Attempt != 1 {
		t.Fatalf("first attempt must be 1, got %d", doc.Attempt)
	}
	if !doc.Deadline.Equal(startRun().Add(30 * time.Minute)) {
		t.Fatalf("deadline must be start+budget, got %v", doc.Deadline)
	}
	if doc.RawRunDir != "" {
		t.Fatalf("WAITING must not expose a raw run dir, got %q", doc.RawRunDir)
	}
	if proc.launchN != 1 {
		t.Fatalf("Start must launch exactly once, launched %d", proc.launchN)
	}
	// The drive is durably persisted and resumable.
	if _, err := store.Load(doc.DriveID); err != nil {
		t.Fatalf("Start must persist the drive: %v", err)
	}
}

// TestStartCarriesLegacyHistorySummary proves a successful start document carries
// the first-admission legacy recovery summary when legacy history was assessed,
// and carries none on an ordinary start over a store with no legacy records. The
// summary is a diagnostic surface on the START document only.
func TestStartCarriesLegacyHistorySummary(t *testing.T) {
	t.Run("legacy-history-present", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{} // default: launch running, observe running
		d, store := newTestDriver(t, clk, proc, stableGit())
		// A completed v2 drive bound to a since-removed worktree: assessed and
		// counted by the first-admission census, but not blocking this start.
		copyLegacyFixture(t, store, "passed")

		doc, err := d.Start(sampleStart())
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if doc.LegacyHistory == nil {
			t.Fatal("a start over a store carrying legacy history must carry the recovery summary")
		}
		if doc.LegacyHistory.Checked != 1 {
			t.Fatalf("LegacyHistory.Checked = %d, want 1", doc.LegacyHistory.Checked)
		}
	})

	t.Run("no-legacy-records-nil", func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		proc := &fakeProc{}
		d, _ := newTestDriver(t, clk, proc, stableGit())

		doc, err := d.Start(sampleStart())
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if doc.LegacyHistory != nil {
			t.Fatalf("a normal start with no legacy records must carry no summary narration, got %+v", doc.LegacyHistory)
		}
	})
}

// TestSeveralWaitingSlicesRetainDriveIdentity proves several WAITING slices keep
// one drive, run, attempt, and fixed deadline: only the same owner advancing the
// same live run.
func TestSeveralWaitingSlicesRetainDriveIdentity(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{} // stays running
	d, store := newTestDriver(t, clk, proc, stableGit())

	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := doc.Deadline
	for i := 0; i < 3; i++ {
		doc, err = d.Advance(doc.DriveID, doc.Generation)
		if err != nil {
			t.Fatalf("Advance %d: %v", i, err)
		}
		if doc.Outcome != WAITING {
			t.Fatalf("slice %d must WAIT, got %s (%s)", i, doc.Outcome, doc.Cause)
		}
		if doc.Attempt != 1 {
			t.Fatalf("attempt must stay 1 across WAITING slices, got %d", doc.Attempt)
		}
		if !doc.Deadline.Equal(deadline) {
			t.Fatalf("deadline must not move across slices: got %v want %v", doc.Deadline, deadline)
		}
	}
	// Exactly one raw run was ever launched across all the slices.
	if proc.launchN != 1 {
		t.Fatalf("several WAITING slices must not relaunch: launched %d", proc.launchN)
	}
	rec, _ := store.Load(doc.DriveID)
	if rec.RelaunchCount != 0 || rec.Attempt != 1 {
		t.Fatalf("identity drifted across slices: attempt=%d relaunch=%d", rec.Attempt, rec.RelaunchCount)
	}
}

// ---------------------------------------------------------------------------
// Terminal outcomes between slices.
// ---------------------------------------------------------------------------

// TestTerminalPassBetweenSlices proves a pass arriving on a later slice is
// accepted only after the fingerprint revalidates, and exposes the raw run dir.
func TestTerminalPassBetweenSlices(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	running := true
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if running {
				return obs(process.StateRunning, runDir), nil
			}
			return obs(process.StatePassed, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())

	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s", doc.Outcome)
	}
	running = false // the suite finishes green between slices
	doc, err = d.Advance(doc.DriveID, doc.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != PASSED {
		t.Fatalf("a green terminal must PASS, got %s (%s)", doc.Outcome, doc.Cause)
	}
	if doc.RawRunDir == "" {
		t.Fatalf("PASSED must expose the raw run dir for evidence")
	}
	// A PASSED drive is terminal: re-advancing returns the same verdict without
	// re-observing the (already consumed) run.
	before := proc.observeN
	again, err := d.Advance(doc.DriveID, doc.Generation)
	if err != nil {
		t.Fatalf("re-advance: %v", err)
	}
	if again.Outcome != PASSED || again.RawRunDir != doc.RawRunDir {
		t.Fatalf("re-advance of a PASSED drive must be idempotent, got %s", again.Outcome)
	}
	if proc.observeN != before {
		t.Fatalf("re-advancing a terminal drive must not re-drive the run")
	}
	_ = store
}

// TestTerminalFailBetweenSlices proves a red suite is FAILED, distinct from a
// halt.
func TestTerminalFailBetweenSlices(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateFailed, runDir), nil
		},
	}
	d, _ := newTestDriver(t, clk, proc, stableGit())
	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != FAILED {
		t.Fatalf("a red suite must FAIL, got %s (%s)", doc.Outcome, doc.Cause)
	}
	if doc.RawRunDir != "" {
		t.Fatalf("FAILED must not expose a raw run dir")
	}
}

// TestPassFingerprintMismatchHalts proves a green terminal whose worktree drifted
// since drive start is HALTED (stop-if-owned), never converted to red.
func TestPassFingerprintMismatchHalts(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	git := stableGit()
	running := true
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if running {
				return obs(process.StateRunning, runDir), nil
			}
			return obs(process.StatePassed, runDir), nil
		},
	}
	d, _ := newTestDriver(t, clk, proc, git)
	// The drive records its start fingerprint on the first (WAITING) slice.
	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s", doc.Outcome)
	}
	// The worktree drifts, then the suite completes green.
	git.status = "DRIFTED"
	running = false
	doc, err = d.Advance(doc.DriveID, doc.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("a drifted pass must HALT, not %s", doc.Outcome)
	}
	if doc.Outcome == FAILED {
		t.Fatalf("a changed worktree must never be reported as red")
	}
	if doc.RawRunDir != "" {
		t.Fatalf("a HALTED drive must not expose a raw run dir")
	}
	if proc.stopN == 0 {
		t.Fatalf("a drifted pass must stop the owned run")
	}
}

// ---------------------------------------------------------------------------
// Deadline and clock governance.
// ---------------------------------------------------------------------------

// TestZeroBudgetTakesOneObservationThenStopsAndHalts proves a zero budget takes
// exactly one observation of a live run, then stops it and HALTs.
func TestZeroBudgetTakesOneObservationThenStopsAndHalts(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{} // stays running
	d, _ := newTestDriver(t, clk, proc, stableGit())

	req := sampleStart()
	req.Budget = 0
	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("zero budget must HALT a still-live run, got %s", doc.Outcome)
	}
	if proc.observeN != 1 {
		t.Fatalf("zero budget must take exactly one observation, took %d", proc.observeN)
	}
	if proc.stopN == 0 {
		t.Fatalf("zero budget must stop the still-live run")
	}
}

// TestDeadlineExpiryWithLiveRunStopsAndHalts proves an expired deadline over a
// live run stops the tree and HALTs, and earns no relaunch.
func TestDeadlineExpiryWithLiveRunStopsAndHalts(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{} // stays running
	d, _ := newTestDriver(t, clk, proc, stableGit())

	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s", doc.Outcome)
	}
	// Jump the clock forward past the fixed deadline, then advance.
	clk.now = doc.Deadline.Add(time.Minute)
	doc, err = d.Advance(doc.DriveID, doc.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("deadline expiry over a live run must HALT, got %s", doc.Outcome)
	}
	if !strings.HasPrefix(doc.Cause, "deadline-expired") {
		t.Fatalf("cause must name deadline expiry, got %q", doc.Cause)
	}
	if proc.stopN == 0 {
		t.Fatalf("deadline expiry must stop the owned tree")
	}
	if proc.launchN != 1 {
		t.Fatalf("deadline expiry earns no relaunch, launched %d", proc.launchN)
	}
}

// TestBackwardClockJumpHaltsDriver proves a backward clock jump below the last
// accepted clock — which could lengthen the effective budget — HALTs rather than
// trusting the reading.
func TestBackwardClockJumpHaltsDriver(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{} // stays running
	d, _ := newTestDriver(t, clk, proc, stableGit())

	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Rewind the wall clock behind the last accepted slice clock.
	clk.now = startRun().Add(-time.Hour)
	doc, err = d.Advance(doc.DriveID, doc.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("a backward clock jump must HALT, got %s", doc.Outcome)
	}
	if !strings.Contains(doc.Cause, "clock") {
		t.Fatalf("cause must name the clock, got %q", doc.Cause)
	}
}

// ---------------------------------------------------------------------------
// Fail-closed observations.
// ---------------------------------------------------------------------------

// TestMalformedObservationFailsClosed proves an unreadable observation HALTs;
// only an exact running state is retryable.
func TestMalformedObservationFailsClosed(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		observe: func(string) (*process.Observation, error) {
			return nil, fmt.Errorf("gatedrive-test: unreadable observation")
		},
	}
	d, _ := newTestDriver(t, clk, proc, stableGit())
	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("a malformed observation must HALT, got %s", doc.Outcome)
	}
}

// TestUnknownObservationStateHalts proves an unrecognized native state fails
// closed rather than being coerced into a workflow outcome.
func TestUnknownObservationStateHalts(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.State("gremlin"), runDir), nil
		},
	}
	d, _ := newTestDriver(t, clk, proc, stableGit())
	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("an unknown state must HALT, got %s", doc.Outcome)
	}
}

// TestStoppedNotInitiatedHalts proves a native stopped state the drive did not
// initiate is HALTED, never red.
func TestStoppedNotInitiatedHalts(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateStopped, runDir), nil
		},
	}
	d, _ := newTestDriver(t, clk, proc, stableGit())
	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("an externally stopped run must HALT, got %s", doc.Outcome)
	}
	if doc.Outcome == FAILED {
		t.Fatalf("a stop must never be reported red")
	}
}

// ---------------------------------------------------------------------------
// Death and the single relaunch.
// ---------------------------------------------------------------------------

// TestSignaledDeathRelaunchAdmittedOnce proves a signaled death under all five
// relaunch conditions relaunches exactly once: a second raw run under the same
// drive, deadline, and identity, with attempt and relaunch count advanced. The
// first run's stop no-op is consumed by a re-observe before the relaunch.
func TestSignaledDeathRelaunchAdmittedOnce(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			// launchN was already incremented by the wrapper.
			return nil, nil // replaced below
		},
	}
	proc.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		id := fmt.Sprintf("run%d", proc.launchN)
		return &process.LaunchOutcome{RunID: id, RunDir: "/runs/" + id, State: process.StateRunning}, nil
	}
	proc.observe = func(runDir string) (*process.Observation, error) {
		if strings.HasSuffix(runDir, "run1") {
			return obs(process.StateSignaled, runDir), nil // first tree died
		}
		return obs(process.StateRunning, runDir), nil // relaunched tree is healthy
	}
	proc.stop = func(runDir, reason string) (*process.StopOutcome, error) {
		// signaled run is already terminal: an already-terminal no-op.
		return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
			Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
	}
	d, store := newTestDriver(t, clk, proc, stableGit())

	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("after an admitted relaunch the healthy new run must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	if doc.Attempt != 2 {
		t.Fatalf("an admitted relaunch must advance the attempt to 2, got %d", doc.Attempt)
	}
	if proc.launchN != 2 {
		t.Fatalf("exactly one relaunch (two launches) must occur, got %d", proc.launchN)
	}
	rec, _ := store.Load(doc.DriveID)
	if rec.RelaunchCount != 1 {
		t.Fatalf("relaunch count must be 1, got %d", rec.RelaunchCount)
	}
	if rec.RawRunDir != "/runs/run2" {
		t.Fatalf("the drive must now own the second run, got %q", rec.RawRunDir)
	}
	if rec.PriorRawRunDir != "/runs/run1" {
		t.Fatalf("the dead first attempt must be preserved, got %q", rec.PriorRawRunDir)
	}
	if !rec.Deadline.Equal(startRun().Add(30 * time.Minute)) {
		t.Fatalf("the relaunch must keep the original deadline, got %v", rec.Deadline)
	}
}

// TestDeathRelaunchRefusals proves every reason a second launch is refused ends
// in HALTED preserving the dead attempt, and never relaunches.
func TestDeathRelaunchRefusals(t *testing.T) {
	cases := []struct {
		name      string
		state     process.State
		mutate    func(rec *driveRecord)
		driftGit  bool
		stopErr   bool
		wantCause string
	}{
		{
			name:      "not idempotent",
			state:     process.StateSignaled,
			mutate:    func(rec *driveRecord) { rec.IdempotentSuiteGate = false },
			wantCause: "not-idempotent",
		},
		{
			name:      "already relaunched",
			state:     process.StateSignaled,
			mutate:    func(rec *driveRecord) { rec.RelaunchCount = 1; rec.Attempt = 2 },
			wantCause: "relaunch-exhausted",
		},
		{
			name:      "deadline exhausted",
			state:     process.StateSignaled,
			mutate:    func(rec *driveRecord) { rec.Deadline = startRun().Add(-time.Minute) },
			wantCause: "deadline-expired",
		},
		{
			name:      "worktree changed",
			state:     process.StateSignaled,
			driftGit:  true,
			wantCause: "worktree-changed",
		},
		{
			name:      "former tree not proven gone",
			state:     process.StateSignaled,
			stopErr:   true,
			wantCause: "uncertain",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := &fakeClock{now: startRun().Add(time.Second)}
			git := stableGit()
			proc := &fakeProc{
				observe: func(runDir string) (*process.Observation, error) {
					return obs(tc.state, runDir), nil
				},
			}
			if tc.stopErr {
				proc.stop = func(runDir, reason string) (*process.StopOutcome, error) {
					return nil, fmt.Errorf("gatedrive-test: stop cannot prove ownership")
				}
			} else {
				proc.stop = func(runDir, reason string) (*process.StopOutcome, error) {
					return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
						Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
				}
			}
			d, store := newTestDriver(t, clk, proc, git)
			rec := seedRecord(t)
			if tc.mutate != nil {
				tc.mutate(&rec)
			}
			id, ownerGen := seedDrive(t, store, rec)
			if tc.driftGit {
				git.status = "DRIFTED"
			}

			doc, err := d.Advance(id, ownerGen)
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if doc.Outcome != HALTED {
				t.Fatalf("a refused relaunch must HALT, got %s (%s)", doc.Outcome, doc.Cause)
			}
			if doc.Outcome == FAILED {
				t.Fatalf("a death must never be reported red")
			}
			if !strings.Contains(doc.Cause, tc.wantCause) {
				t.Fatalf("cause must name %q, got %q", tc.wantCause, doc.Cause)
			}
			if proc.launchN != 0 {
				t.Fatalf("a refused relaunch must launch nothing, launched %d", proc.launchN)
			}
			// The dead attempt is preserved: the record still names the first run.
			got, _ := store.Load(id)
			if got.RawRunDir != "/runs/run1" {
				t.Fatalf("the dead attempt must be preserved, got %q", got.RawRunDir)
			}
		})
	}
}

// TestVanishedProvenGoneWithoutStop proves a vanished observation already proves
// the tree is gone: the death path consumes it without issuing a stop (there is
// no live tree to stop). With relaunch refused it HALTs.
func TestVanishedProvenGoneWithoutStop(t *testing.T) {
	clk := &fakeClock{now: startRun().Add(time.Second)}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateVanished, runDir), nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	rec := seedRecord(t)
	rec.IdempotentSuiteGate = false // refuse relaunch so we terminate at HALT
	id, ownerGen := seedDrive(t, store, rec)

	doc, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("a vanished run with relaunch refused must HALT, got %s", doc.Outcome)
	}
	if proc.stopN != 0 {
		t.Fatalf("a vanished run needs no stop; issued %d", proc.stopN)
	}
}

// TestSignaledDeathConsumesTerminalViaStopNoOpAndReObserve proves the signaled
// death path proves no tree survives via a stop no-op followed by a re-observe
// before deciding.
func TestSignaledDeathConsumesTerminalViaStopNoOpAndReObserve(t *testing.T) {
	clk := &fakeClock{now: startRun().Add(time.Second)}
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateSignaled, runDir), nil
		},
		stop: func(runDir, reason string) (*process.StopOutcome, error) {
			return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
				Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
		},
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	rec := seedRecord(t)
	rec.IdempotentSuiteGate = false
	id, ownerGen := seedDrive(t, store, rec)

	before := proc.observeN
	doc, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED {
		t.Fatalf("want HALTED, got %s", doc.Outcome)
	}
	if proc.stopN == 0 {
		t.Fatalf("the signaled death path must issue a stop no-op")
	}
	if proc.observeN < before+2 {
		t.Fatalf("the death path must re-observe after the stop no-op")
	}
}

// ---------------------------------------------------------------------------
// Ownership, schema, and resume.
// ---------------------------------------------------------------------------

// TestAdvanceWrongOwnerHalts proves an advance presenting a stale/wrong owner
// generation is HALTED (identity disagreement), never a silent continuation.
func TestAdvanceWrongOwnerHalts(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, _ := newTestDriver(t, clk, proc, stableGit())
	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	got, err := d.Advance(doc.DriveID, "not-the-owner")
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got.Outcome != HALTED {
		t.Fatalf("a wrong owner must HALT, got %s", got.Outcome)
	}
}

// TestAdvanceUnknownDriveIsCommandFailure proves advancing a drive that cannot
// be read is a command failure (an error), not a workflow HALT document.
func TestAdvanceUnknownDriveIsCommandFailure(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	d, _ := newTestDriver(t, clk, &fakeProc{}, stableGit())
	// A well-formed but nonexistent id.
	_, err := d.Advance("00000000000000000000000000000000", "gen")
	if err == nil {
		t.Fatalf("advancing an unreadable drive must be a command failure")
	}
}

// TestSchemaMismatchHalts proves a persisted record with an unknown schema
// version fails closed to HALTED rather than being migrated or advanced.
func TestSchemaMismatchHalts(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Corrupt the on-disk schema version.
	rec := seedRecord(t)
	rec.SchemaVersion = driveSchemaVersion + 999
	buf, _ := json.Marshal(storedRecord{Generation: "x", Record: rec})
	path := filepath.Join(store.root, doc.DriveID, recordFileName)
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	got, err := d.Advance(doc.DriveID, doc.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if got.Outcome != HALTED {
		t.Fatalf("an unknown schema must HALT, got %s", got.Outcome)
	}
}

// TestFreshDriverResumesFromDisk proves the drive record — not in-memory state —
// is the source of truth: a fresh Driver over the same store resumes a WAITING
// drive and consumes the terminal. This is the interruption-between-invocations
// property; each transition is a single atomic record write.
func TestFreshDriverResumesFromDisk(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	running := true
	// The two drivers share one process seam so both observe the same run.
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if running {
				return obs(process.StateRunning, runDir), nil
			}
			return obs(process.StatePassed, runDir), nil
		},
	}
	store := OpenStore(testsupport.TempDir(t))

	driverA := NewDriver(store, clk, proc, stableGit())
	driverA.slice = 4 * pollTick
	driverA.pollInterval = pollTick
	driverA.sleep = func(dur time.Duration) { clk.advance(dur) }

	doc, err := driverA.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s", doc.Outcome)
	}

	// A brand-new driver instance (simulating a fresh CLI process) resumes.
	running = false
	driverB := NewDriver(store, clk, proc, stableGit())
	driverB.slice = 4 * pollTick
	driverB.pollInterval = pollTick
	driverB.sleep = func(dur time.Duration) { clk.advance(dur) }

	resumed, err := driverB.Advance(doc.DriveID, doc.Generation)
	if err != nil {
		t.Fatalf("resume Advance: %v", err)
	}
	if resumed.Outcome != PASSED {
		t.Fatalf("a fresh driver must resume from disk and consume the terminal, got %s", resumed.Outcome)
	}
	if resumed.DriveID != doc.DriveID {
		t.Fatalf("the resumed drive id must match: %q vs %q", resumed.DriveID, doc.DriveID)
	}
}

// TestRelaunchCrashBetweenReserveAndLaunchRecovers proves that the durable
// relaunch reservation survives a process restart. A clean census permits the
// one reserved replacement to launch, an identified replacement is attached
// without another launch, and every uncertain census halts the drive closed.
func TestRelaunchCrashBetweenReserveAndLaunchRecovers(t *testing.T) {
	for _, tc := range []struct {
		name          string
		resolution    *process.ReservationResolution
		wantOutcome   Outcome
		wantCause     string
		wantLaunches  int
		wantRelaunch  int
		wantAttempt   int
		wantReserved  bool
		identifiedRun string
	}{
		{
			name:         "never launched starts the reserved replacement",
			resolution:   &process.ReservationResolution{Disposition: "never-launched"},
			wantOutcome:  WAITING,
			wantLaunches: 1,
			wantRelaunch: 1,
			wantAttempt:  2,
		},
		{
			name: "identified replacement attaches without another launch",
			resolution: &process.ReservationResolution{
				Disposition: "identified", RunID: "run2", RunDir: "/runs/run2", State: process.StateRunning,
			},
			wantOutcome:   WAITING,
			wantLaunches:  0,
			wantRelaunch:  1,
			wantAttempt:   2,
			identifiedRun: "/runs/run2",
		},
		{
			name:         "unresolved replacement halts closed",
			resolution:   &process.ReservationResolution{Disposition: "unresolved"},
			wantOutcome:  HALTED,
			wantCause:    "launch-unconfirmed",
			wantLaunches: 0,
			wantRelaunch: 0,
			wantAttempt:  1,
			wantReserved: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := OpenStore(testsupport.TempDir(t))
			// A live no-run-record worktree slot backs the admission token, as a real
			// scopeless drive holds one, so the run-linkage resolution admits the
			// crash-window recovery through the standalone path (change 0437 Task 3).
			wt := mkWorktree(t)
			token, terr := store.ReserveWorktreeExecution(sampleAdmission(wt))
			if terr != nil {
				t.Fatalf("reserve admission: %v", terr)
			}
			rec := seedRecord(t)
			rec.WorktreePath = wt
			rec.AdmissionToken = token
			rec.RelaunchToken = "bbbbbbbbbbbbbbbb"
			id, ownerGen := seedDrive(t, store, rec)
			if err := store.ownerCAS(id, func(r *driveRecord) error {
				r.RelaunchReserved = true
				return nil
			}); err != nil {
				t.Fatalf("reserve relaunch: %v", err)
			}

			proc := &fakeProc{
				launch: func(req process.LaunchRequest) (*process.LaunchOutcome, error) {
					if req.ReservationToken != rec.RelaunchToken || req.ReservationToken == rec.AdmissionToken {
						t.Fatalf("replacement token = %q, want unique relaunch token %q", req.ReservationToken, rec.RelaunchToken)
					}
					return &process.LaunchOutcome{RunID: "run2", RunDir: "/runs/run2", State: process.StateRunning}, nil
				},
				resolve: func(root, token string) (*process.ReservationResolution, error) {
					if root != rec.RunRoot || token != rec.RelaunchToken {
						t.Fatalf("ResolveReservation(%q, %q), want (%q, %q)", root, token, rec.RunRoot, rec.RelaunchToken)
					}
					return tc.resolution, nil
				},
				observe: func(runDir string) (*process.Observation, error) {
					if runDir == "/runs/run1" {
						return obs(process.StateVanished, runDir), nil
					}
					return obs(process.StateRunning, runDir), nil
				},
			}
			clk := &fakeClock{now: startRun().Add(time.Second)}
			d := NewDriver(reopenStore(store), clk, proc, stableGit())
			d.slice = pollTick
			d.pollInterval = pollTick
			d.sleep = func(d time.Duration) { clk.advance(d) }

			doc, err := d.Advance(id, ownerGen)
			if err != nil {
				t.Fatalf("Advance after restart: %v", err)
			}
			if doc.Outcome != tc.wantOutcome || doc.Cause != tc.wantCause {
				t.Fatalf("outcome = %s/%q, want %s/%q", doc.Outcome, doc.Cause, tc.wantOutcome, tc.wantCause)
			}
			if proc.launchN != tc.wantLaunches {
				t.Fatalf("launches = %d, want %d", proc.launchN, tc.wantLaunches)
			}
			got, err := store.Load(id)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got.RelaunchCount != tc.wantRelaunch || got.Attempt != tc.wantAttempt || got.RelaunchReserved != tc.wantReserved {
				t.Fatalf("record = relaunch=%d attempt=%d reserved=%v, want %d/%d/%v", got.RelaunchCount, got.Attempt, got.RelaunchReserved, tc.wantRelaunch, tc.wantAttempt, tc.wantReserved)
			}
			if tc.identifiedRun != "" && got.RawRunDir != tc.identifiedRun {
				t.Fatalf("identified replacement was not attached: RawRunDir=%q want %q", got.RawRunDir, tc.identifiedRun)
			}
		})
	}
}

// TestRelaunchReservationNotRefundedOnUncertainty proves an uncertain reserved
// replacement permanently consumes the drive's sole relaunch. Even if a later
// probe would report clean absence, the terminal launch-unconfirmed outcome
// remains authoritative and no new backend launch is permitted.
func TestRelaunchReservationNotRefundedOnUncertainty(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	rec := seedRecord(t)
	rec.AdmissionToken = "reservation-token"
	rec.RelaunchToken = "bbbbbbbbbbbbbbbb"
	id, ownerGen := seedDrive(t, store, rec)
	if err := store.ownerCAS(id, func(r *driveRecord) error {
		r.RelaunchReserved = true
		return nil
	}); err != nil {
		t.Fatalf("reserve relaunch: %v", err)
	}

	resolution := "unresolved"
	proc := &fakeProc{
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: resolution}, nil
		},
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StateVanished, runDir), nil
		},
	}
	d := NewDriver(reopenStore(store), &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())

	first, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("first Advance: %v", err)
	}
	if first.Outcome != HALTED || first.Cause != "launch-unconfirmed" {
		t.Fatalf("uncertain replacement must halt unresolved, got %s/%q", first.Outcome, first.Cause)
	}
	resolution = "never-launched"
	second, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("second Advance: %v", err)
	}
	if second.Outcome != HALTED || second.Cause != "launch-unconfirmed" {
		t.Fatalf("reservation must not be refunded after uncertainty, got %s/%q", second.Outcome, second.Cause)
	}
	if proc.launchN != 0 || proc.resolveN != 1 {
		t.Fatalf("a consumed uncertain reservation must not launch or re-resolve, launches=%d resolves=%d", proc.launchN, proc.resolveN)
	}
	got, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !got.RelaunchReserved || got.RelaunchCount != 0 {
		t.Fatalf("uncertainty must retain the consumed reservation, got reserved=%v relaunch=%d", got.RelaunchReserved, got.RelaunchCount)
	}
}

// TestProcessSeamSatisfiedByRealService proves the real process.Service is a
// drop-in ProcessSeam, so Task 7 can wire it directly.
func TestProcessSeamSatisfiedByRealService(t *testing.T) {
	var _ ProcessSeam = (*process.Service)(nil)
}

// storeTestDriver wires a driver with the short test slice over the given store,
// clock, proc, and git — the newTestDriver body without minting a fresh store, so
// a test can share one store with a fake seam that inspects it mid-flight.
func storeTestDriver(store *Store, clk *fakeClock, proc ProcessSeam, git GitSeam) *Driver {
	d := NewDriver(store, clk, proc, git)
	d.slice = 4 * pollTick
	d.pollInterval = pollTick
	d.sleep = func(dur time.Duration) { clk.advance(dur) }
	return d
}

// ---------------------------------------------------------------------------
// Worktree execution slot admission (change 0375 Task 3). A start reserves the
// canonical worktree's single execution slot before launch, so one worktree
// carries at most one reserved-or-running top-level gate run.
// ---------------------------------------------------------------------------

// TestStartReservesWorktreeSlot proves a start reserves the worktree execution
// slot and, once its run is launch-confirmed and still live (WAITING), the slot is
// executing and carries the drive's raw run identity — the durable locator a later
// cancellation or recovery resolves the worktree by. (A PASSED/FAILED terminal
// RELEASES the slot; that lifecycle is proven by TestStartReleasesSlotOnTerminal.)
func TestStartReservesWorktreeSlot(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{} // runs stay live so the drive WAITs and holds the slot
	d := storeTestDriver(store, clk, proc, stableGit())
	req := sampleStart()

	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("start over a live run must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}

	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionExecuting {
		t.Fatalf("while a drive runs the worktree slot must be executing, got %q", slot.State)
	}
	rec, err := store.Load(doc.DriveID)
	if err != nil {
		t.Fatalf("Load drive: %v", err)
	}
	if slot.RawRunID != rec.RawOwnership || slot.RawRunDir != rec.RawRunDir {
		t.Fatalf("the slot must carry the drive's raw run identity, slot=(%q,%q) drive=(%q,%q)",
			slot.RawRunID, slot.RawRunDir, rec.RawOwnership, rec.RawRunDir)
	}
	if slot.Kind != "scopeless" {
		t.Fatalf("the slot kind must be scopeless, got %q", slot.Kind)
	}
	// The drive carries the admission token it launched under (threaded into the raw
	// launch and used by recovery), and it is never a capability.
	if rec.AdmissionToken == "" || rec.AdmissionToken != slot.ReservationToken {
		t.Fatalf("the drive must carry the slot's admission token, drive=%q slot=%q", rec.AdmissionToken, slot.ReservationToken)
	}
}

// TestStartReleasesSlotOnTerminal proves the per-drive release: a drive that
// reaches a PASSED terminal frees the worktree execution slot (the supervisor
// reports PASSED only after tearing the child down), so a LATER drive for a
// DIFFERENT change can admit onto the same worktree. Without the release a
// finished drive would fence the worktree forever.
func TestStartReleasesSlotOnTerminal(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	proc := passObserveProc() // every run PASSES on first observation
	d := storeTestDriver(store, clk, proc, stableGit())

	reqA := sampleStart()
	a, err := d.Start(reqA)
	if err != nil {
		t.Fatalf("drive A Start: %v", err)
	}
	if a.Outcome != PASSED {
		t.Fatalf("drive A must PASS, got %s (%s)", a.Outcome, a.Cause)
	}
	// The passed drive released the slot: the worktree is idle again.
	slot, _, err := store.LoadWorktreeExecution(reqA.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionReleased {
		t.Fatalf("a PASSED drive must release the worktree slot, got %q", slot.State)
	}
	// A drive for a different change now admits onto the same worktree and PASSES too.
	reqB := sampleStart()
	reqB.ChangeID = "0343"
	b, err := d.Start(reqB)
	if err != nil {
		t.Fatalf("drive B Start over a released worktree slot: %v", err)
	}
	if b.Outcome != PASSED {
		t.Fatalf("drive B must PASS over the reused worktree, got %s (%s)", b.Outcome, b.Cause)
	}
	if b.DriveID == a.DriveID {
		t.Fatalf("the second start must be a NEW drive, got the first drive's id")
	}
}

// TestAbandonAdmissionReleasesSlotAndRemovesReservedDrive proves the collapsed
// AbandonAdmission shape: an admission the caller decides not to launch removes
// its never-launched RESERVED drive record and releases the worktree slot it
// reserved, so a later start admits onto the worktree — and nothing launches.
func TestAbandonAdmissionReleasesSlotAndRemovesReservedDrive(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()

	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if err := d.AbandonAdmission(ticket); err != nil {
		t.Fatalf("AbandonAdmission: %v", err)
	}
	if proc.launchN != 0 {
		t.Fatalf("an abandoned admission must launch nothing, got %d", proc.launchN)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionReleased {
		t.Fatalf("an abandoned admission must release its slot, got %q", slot.State)
	}
	if _, lerr := store.Load(ticket.id); lerr == nil {
		t.Fatalf("an abandoned admission must remove its reserved drive record")
	}
	if _, err := d.Admit(req); err != nil {
		t.Fatalf("a later start must admit over the released slot: %v", err)
	}
}

// TestScopelessStartReservesBeforeLaunch proves that the finalize-style,
// scopeless start owns a durable worktree slot before it asks the
// process backend to launch. Its private RunRoot is only the supervisor's
// allocation directory; worktree admission is keyed by Worktree.
func TestScopelessStartReservesBeforeLaunch(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	req := sampleStart()

	var atLaunch admissionRecord
	var loadErr error
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			atLaunch, _, loadErr = store.LoadWorktreeExecution(req.Worktree)
			return &process.LaunchOutcome{RunID: "run1", RunDir: "/runs/run1", State: process.StateRunning}, nil
		},
	}
	d := storeTestDriver(store, clk, proc, stableGit())

	doc, err := d.Start(req)
	if loadErr != nil {
		t.Fatalf("LoadWorktreeExecution at launch: %v", loadErr)
	}
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if atLaunch.State != admissionReserved {
		t.Fatalf("at launch the scopeless worktree slot must already be reserved, got %q", atLaunch.State)
	}
	if atLaunch.Kind != "scopeless" || atLaunch.ScopeID != "" {
		t.Fatalf("at launch the slot must be scopeless with no scope, got kind=%q scope=%q", atLaunch.Kind, atLaunch.ScopeID)
	}
	if atLaunch.ReservationToken == "" {
		t.Fatalf("at launch the slot must carry a reservation token")
	}
	if doc.Outcome != WAITING {
		t.Fatalf("scopeless start over a live run must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after Start: %v", err)
	}
	if slot.State != admissionExecuting || slot.RawRunID != "run1" || slot.RawRunDir != "/runs/run1" {
		t.Fatalf("after launch the slot must be executing with the run handle, got state=%q id=%q dir=%q", slot.State, slot.RawRunID, slot.RawRunDir)
	}
}

// TestScopelessPersistFailureReleasesOnProvenStop proves the post-launch
// attach failure has no ambiguous-free path: a proven owned stop releases the
// worktree slot, while a stop the seam cannot prove leaves it unresolved.
func TestScopelessPersistFailureReleasesOnProvenStop(t *testing.T) {
	for name, stop := range map[string]func(string, string) (*process.StopOutcome, error){
		"proven stop releases": func(runDir, reason string) (*process.StopOutcome, error) {
			return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
		},
		"unproven stop leaves unresolved": func(string, string) (*process.StopOutcome, error) {
			return nil, fmt.Errorf("gatedrive-test: stop ownership unproven")
		},
	} {
		t.Run(name, func(t *testing.T) {
			clk := &fakeClock{now: startRun()}
			store := OpenStore(testsupport.TempDir(t))
			req := sampleStart()
			var recordDir string
			proc := &fakeProc{
				launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
					entries, err := os.ReadDir(store.root)
					if err != nil {
						return nil, err
					}
					if len(entries) != 1 || !entries[0].IsDir() {
						t.Fatalf("the reserved drive must exist before launch, got entries=%v", entries)
					}
					recordDir = filepath.Join(store.root, entries[0].Name())
					if err := os.Chmod(recordDir, 0o500); err != nil {
						return nil, fmt.Errorf("chmod reserved drive: %w", err)
					}
					return &process.LaunchOutcome{RunID: "run1", RunDir: "/runs/run1", State: process.StateRunning}, nil
				},
				stop: stop,
			}
			d := storeTestDriver(store, clk, proc, stableGit())

			if _, err := d.Start(req); err == nil {
				t.Fatalf("attach failure must return an error")
			}
			if recordDir == "" {
				t.Fatalf("launch never found the reserved drive record")
			}
			if err := os.Chmod(recordDir, 0o700); err != nil {
				t.Fatalf("restore reserved drive permissions: %v", err)
			}
			slot, _, err := store.LoadWorktreeExecution(req.Worktree)
			if err != nil {
				t.Fatalf("LoadWorktreeExecution: %v", err)
			}
			want := admissionReleased
			if name == "unproven stop leaves unresolved" {
				want = admissionUnresolved
			}
			if slot.State != want {
				t.Fatalf("persist failure slot state = %q, want %q", slot.State, want)
			}
		})
	}
}

// passObserveProc returns a fakeProc whose every run reports PASSED on the first
// observation, so a Start returns a durable PASSED in one call.
func passObserveProc() *fakeProc {
	return &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StatePassed, runDir), nil
		},
	}
}
