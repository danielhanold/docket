package gatedrive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	launchN, observeN, stopN int
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

func startEpoch() time.Time { return time.Unix(1_000_000, 0).UTC() }

// sampleStart is a well-formed StartRequest for an idempotent suite gate.
func sampleStart() StartRequest {
	return StartRequest{
		RepoDir:             "/repo",
		Worktree:            "/repo",
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
	start := startEpoch()
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
	clk := &fakeClock{now: startEpoch()}
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
	if !doc.Deadline.Equal(startEpoch().Add(30 * time.Minute)) {
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

// TestSeveralWaitingSlicesRetainDriveIdentity proves several WAITING slices keep
// one drive, run, attempt, and fixed deadline: only the same owner advancing the
// same live run.
func TestSeveralWaitingSlicesRetainDriveIdentity(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
		t.Fatalf("identity drift must never be reported as red")
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{} // stays running
	d, _ := newTestDriver(t, clk, proc, stableGit())

	doc, err := d.Start(sampleStart())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Rewind the wall clock behind the last accepted slice clock.
	clk.now = startEpoch().Add(-time.Hour)
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
	if !rec.Deadline.Equal(startEpoch().Add(30 * time.Minute)) {
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
			mutate:    func(rec *driveRecord) { rec.Deadline = startEpoch().Add(-time.Minute) },
			wantCause: "deadline-expired",
		},
		{
			name:      "identity mismatch",
			state:     process.StateSignaled,
			driftGit:  true,
			wantCause: "identity-mismatch",
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
			clk := &fakeClock{now: startEpoch().Add(time.Second)}
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
	clk := &fakeClock{now: startEpoch().Add(time.Second)}
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
	clk := &fakeClock{now: startEpoch().Add(time.Second)}
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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
	clk := &fakeClock{now: startEpoch()}
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

// TestProcessSeamSatisfiedByRealService proves the real process.Service is a
// drop-in ProcessSeam, so Task 7 can wire it directly.
func TestProcessSeamSatisfiedByRealService(t *testing.T) {
	var _ ProcessSeam = (*process.Service)(nil)
}

// ---------------------------------------------------------------------------
// Scoped starts reserve the scope slot durably BEFORE any launch (change 0405
// Task 3). These prove the pinned order pre-checks → fingerprint →
// NewReservedDrive → reserveScopeDrive → Launch → attachLaunch →
// confirmScopeLaunch, and its two fail-closed boundaries (launch failure,
// persist failure) that never launch a duplicate.
// ---------------------------------------------------------------------------

// prepareScopedStart prepares a task scope over store and returns the grant plus
// a StartRequest wired to it (scope id + child capability). The caller drives the
// Start so the fake seams can observe the scope mid-flight.
func prepareScopedStart(t *testing.T, store *Store) (ScopeGrant, StartRequest) {
	t.Helper()
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	return grant, req
}

// scopedTestDriver wires a driver with the short test slice over the given store,
// clock, proc, and git — the newTestDriver body without minting a fresh store, so
// a scoped-start test can share one store with a scope-inspecting fake seam.
func scopedTestDriver(store *Store, clk *fakeClock, proc ProcessSeam, git GitSeam) *Driver {
	d := NewDriver(store, clk, proc, git)
	d.slice = 4 * pollTick
	d.pollInterval = pollTick
	d.sleep = func(dur time.Duration) { clk.advance(dur) }
	return d
}

// TestScopedStartReservesBeforeLaunch proves the scoped Start reserves the scope's
// single slot DURABLY before any process launch: the fake seam's Launch observes
// the scope already holding this drive id in scopeStateReserved, and after Start
// returns the slot is scopeStateLaunched and the drive record carries the launch
// handle.
func TestScopedStartReservesBeforeLaunch(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	grant, req := prepareScopedStart(t, store)

	var atLaunch scopeRecord
	var loadErr error
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			// At launch time the reservation must already be durable in the scope.
			sc, err := store.LoadScope(grant.ScopeID)
			if err != nil {
				loadErr = err
			}
			atLaunch = sc
			return &process.LaunchOutcome{RunID: "run1", RunDir: "/runs/run1", State: process.StateRunning}, nil
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())

	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if loadErr != nil {
		t.Fatalf("LoadScope at launch: %v", loadErr)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("first slice must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	// Reservation preceded the launch: the scope named this drive as reserved.
	if atLaunch.CurrentDriveID != doc.DriveID {
		t.Fatalf("at launch the scope must already name this drive, got %q want %q", atLaunch.CurrentDriveID, doc.DriveID)
	}
	if atLaunch.CurrentDriveState != scopeStateReserved {
		t.Fatalf("at launch the slot must be reserved (not yet launched), got %q", atLaunch.CurrentDriveState)
	}
	// After Start the slot is confirmed launched and the record carries the handle.
	after, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if after.CurrentDriveState != scopeStateLaunched {
		t.Fatalf("after Start the slot must be launched, got %q", after.CurrentDriveState)
	}
	rec, err := store.Load(doc.DriveID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.RawRunDir != "/runs/run1" || rec.RawOwnership != "run1" {
		t.Fatalf("the drive record must carry the launch handle, got dir=%q id=%q", rec.RawRunDir, rec.RawOwnership)
	}
	if proc.launchN != 1 {
		t.Fatalf("scoped Start must launch exactly once, got %d", proc.launchN)
	}
}

// TestScopedStartLaunchFailureFailsClosed proves a launch failure on a scoped
// start fails closed with NO automatic second launch: Start returns a command
// error (not a fabricated verdict), the scope still names the reserved drive, the
// drive record is persisted HALTED/"launch-failed" with its owner retained (a
// terminal-unconsumed record outer recovery can see), and a subsequent Start on
// the scope is refused ErrScopeBusy.
func TestScopedStartLaunchFailureFailsClosed(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	grant, req := prepareScopedStart(t, store)

	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			return nil, fmt.Errorf("gatedrive-test: launch failed")
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())

	if _, err := d.Start(req); err == nil {
		t.Fatalf("a launch failure must be a command failure (error)")
	}
	// The scope still names the reserved drive — never treated as empty.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID == "" || scope.CurrentDriveState != scopeStateReserved {
		t.Fatalf("after a launch failure the slot must stay reserved, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
	}
	// The drive record is persisted HALTED/"launch-failed" with its owner retained.
	rec, err := store.Load(scope.CurrentDriveID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "launch-failed" {
		t.Fatalf("launch failure must persist HALTED/launch-failed, got %s/%q", rec.LastOutcome, rec.LastCause)
	}
	if rec.OwnerGeneration == "" {
		t.Fatalf("a launch-failed record must retain its owner generation for outer recovery")
	}
	if rec.RawRunDir != "" {
		t.Fatalf("a launch that never returned a handle must leave RawRunDir empty, got %q", rec.RawRunDir)
	}
	// No automatic second launch: a subsequent start is refused ErrScopeBusy.
	launchesBefore := proc.launchN
	if _, err := d.Start(req); !isOwnershipKind(err, ErrScopeBusy) {
		t.Fatalf("a subsequent start over a reserved (launch-failed) slot must fail ErrScopeBusy, got %v", err)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a refused subsequent start must not launch, launched %d->%d", launchesBefore, proc.launchN)
	}
}

// TestScopedStartReservationFailureCleansOrphanedReservedDrive proves the change
// 0405 review finding: a scoped start mints a reserved drive record
// (NewReservedDrive) BEFORE reserveScopeDrive wins the scope slot, so a reservation
// rejection must not strand that record. Left behind, its nonterminal outcome makes
// it a spurious FindScopeDriveIDs recovery candidate — and with a sibling result >1
// match fails the outer takeover closed on ambiguity — degrading the very
// crash-recovery path scopes exist to provide. Here the reservation write fails (the
// scope directory is read-only around reserveScopeDrive, after NewReservedDrive has
// already minted the record) and the start must leave ZERO discoverable drive
// records, having launched nothing.
func TestScopedStartReservationFailureCleansOrphanedReservedDrive(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := passObserveProc()
	d := scopedTestDriver(store, clk, proc, stableGit())
	grant, req := prepareScopedStart(t, store)

	// Make the scope directory read-only so reserveScopeDrive's slot write fails
	// AFTER NewReservedDrive has already minted the reserved drive record (the drive
	// root stays writable). The pre-check's LoadScope still reads under r-x.
	scopeDir := filepath.Join(store.scopeRoot, grant.ScopeID)
	if err := os.Chmod(scopeDir, 0o500); err != nil {
		t.Fatalf("chmod scope dir read-only: %v", err)
	}
	_, serr := d.Start(req)
	// Restore perms before any read so the assertions and cleanup work.
	if cerr := os.Chmod(scopeDir, 0o700); cerr != nil {
		t.Fatalf("restore scope dir perms: %v", cerr)
	}
	if serr == nil {
		t.Fatalf("a reservation write failure must fail the start")
	}
	if proc.launchN != 0 {
		t.Fatalf("a reservation failure must never launch, got %d", proc.launchN)
	}

	// The just-minted reserved record was never launch-confirmed into the slot, so
	// removing it severs no live recovery — and it must not linger as a spurious
	// recovery candidate.
	ids, err := store.FindScopeDriveIDs(req.ChangeID, "")
	if err != nil {
		t.Fatalf("FindScopeDriveIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("a failed reservation must leave no orphaned reserved drive candidate, got %v", ids)
	}
	// Belt-and-suspenders: the drive root holds no leftover drive directory at all.
	entries, err := os.ReadDir(store.root)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir drive root: %v", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) != 0 {
		t.Fatalf("a failed reservation must leave no orphaned drive directory, got %v", dirs)
	}
}

// TestScopedStartAttachLaunchFailureStopsOrphan proves that when persisting the
// launch handle (attachLaunch) fails, the freshly launched run is stopped (orphan
// control) and the scope slot is NOT treated as empty — a subsequent start is
// refused rather than launching a duplicate.
func TestScopedStartAttachLaunchFailureStopsOrphan(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	grant, req := prepareScopedStart(t, store)

	const runDir = "/runs/run1"
	var stopped []string
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			sc, err := store.LoadScope(grant.ScopeID)
			if err != nil {
				return nil, err
			}
			// Only when the reservation already precedes the launch (the behavior
			// under test) do we sabotage the record dir so the attachLaunch write
			// fails: make the reserved drive's directory read-only.
			if sc.CurrentDriveID != "" {
				if err := os.Chmod(filepath.Join(store.root, sc.CurrentDriveID), 0o500); err != nil {
					return nil, err
				}
			}
			return &process.LaunchOutcome{RunID: "run1", RunDir: runDir, State: process.StateRunning}, nil
		},
		stop: func(rd, reason string) (*process.StopOutcome, error) {
			stopped = append(stopped, rd)
			return &process.StopOutcome{State: process.StateStopped, RunDir: rd, Performed: true}, nil
		},
	}
	d := scopedTestDriver(store, clk, proc, stableGit())

	if _, err := d.Start(req); err == nil {
		t.Fatalf("a persist (attachLaunch) failure must be a command failure (error)")
	}
	// Restore perms so subsequent reads and temp cleanup work.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID != "" {
		if err := os.Chmod(filepath.Join(store.root, scope.CurrentDriveID), 0o700); err != nil {
			t.Fatalf("restore perms: %v", err)
		}
	}
	// The freshly launched run was stopped (orphan control).
	foundStop := false
	for _, rd := range stopped {
		if rd == runDir {
			foundStop = true
		}
	}
	if !foundStop {
		t.Fatalf("a persist failure must stop the orphaned run %q, stops=%v", runDir, stopped)
	}
	// The scope slot is NOT treated as empty: it stays reserved.
	if scope.CurrentDriveID == "" || scope.CurrentDriveState != scopeStateReserved {
		t.Fatalf("after a persist failure the slot must stay reserved, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
	}
	// A subsequent start is refused (never a duplicate launch).
	launchesBefore := proc.launchN
	if _, err := d.Start(req); !isOwnershipKind(err, ErrScopeBusy) {
		t.Fatalf("a subsequent start over a reserved (persist-failed) slot must fail ErrScopeBusy, got %v", err)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a refused subsequent start must not launch, launched %d->%d", launchesBefore, proc.launchN)
	}
}

// ---------------------------------------------------------------------------
// Successor starts — the sequential handshake (change 0405 Task 4). A scope
// carries a SEQUENCE of task-owned drives through one slot; each successor start
// presents the predecessor receipt (drive id + owner generation) and acknowledges
// exactly that PASSED/FAILED result before launching its own new drive.
// ---------------------------------------------------------------------------

// passObserveProc returns a fakeProc whose every run reports PASSED on the first
// observation, so a scoped Start returns a durable PASSED in one call.
func passObserveProc() *fakeProc {
	return &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return obs(process.StatePassed, runDir), nil
		},
	}
}

// TestScopedSequentialStarts reproduces spec verification 1: with the complete
// identity bundle a FIRST start PASSES (the positive control proving identity is
// not the failure), a receipt-less SECOND start over the launched slot is refused
// with the historical scope-second-live-drive rejection, and a SECOND start that
// presents the predecessor receipt succeeds as a NEW drive with its own id and
// owner generation while the predecessor record survives as history with its owner
// generation cleared.
func TestScopedSequentialStarts(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := passObserveProc()
	d := scopedTestDriver(store, clk, proc, stableGit())
	grant, req := prepareScopedStart(t, store)

	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if first.Outcome != PASSED {
		t.Fatalf("first start must PASS (positive control), got %s (%s)", first.Outcome, first.Cause)
	}

	launchesBefore := proc.launchN
	if _, err := d.Start(req); !isOwnershipKind(err, ErrScopeSecondDrive) {
		t.Fatalf("a receipt-less second start must fail ErrScopeSecondDrive, got %v", err)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a rejected receipt-less second start must not launch, launched %d->%d", launchesBefore, proc.launchN)
	}

	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	second, err := d.Start(succ)
	if err != nil {
		t.Fatalf("successor Start: %v", err)
	}
	if second.Outcome != PASSED {
		t.Fatalf("successor start must PASS, got %s (%s)", second.Outcome, second.Cause)
	}
	if second.DriveID == first.DriveID {
		t.Fatalf("a successor must be a NEW drive, got the predecessor's id %q", second.DriveID)
	}
	if second.Generation == "" || second.Generation == first.Generation {
		t.Fatalf("a successor must carry its own owner generation, got %q (predecessor %q)", second.Generation, first.Generation)
	}

	firstRec, err := store.Load(first.DriveID)
	if err != nil {
		t.Fatalf("Load predecessor: %v", err)
	}
	if firstRec.LastOutcome != PASSED {
		t.Fatalf("the predecessor verdict must remain readable, got %s", firstRec.LastOutcome)
	}
	if firstRec.OwnerGeneration != "" {
		t.Fatalf("acknowledging the predecessor must clear its owner generation, got %q", firstRec.OwnerGeneration)
	}

	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID != second.DriveID || scope.CurrentDriveState != scopeStateLaunched {
		t.Fatalf("the slot must name the successor launched, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
	}
	if scope.PriorDriveID != first.DriveID {
		t.Fatalf("the prior drive must chain to the predecessor, got %q", scope.PriorDriveID)
	}
	if scope.PendingAckDriveID != "" {
		t.Fatalf("a completed successor transition must leave no pending ack, got %q", scope.PendingAckDriveID)
	}
}

// TestScopedSequenceBaselineRedGreen reproduces spec verification 2 (fake seams):
// baseline PASSED, RED FAILED, GREEN PASSED as three distinct drives under one
// scope, with a real worktree edit between RED and GREEN (modeled by changing the
// fake git seam's fingerprint input). Each command executes exactly once, each
// drive keeps its own fingerprint and result, and the slot chains correctly.
func TestScopedSequenceBaselineRedGreen(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	git := stableGit()
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if strings.HasSuffix(runDir, "run2") {
				return obs(process.StateFailed, runDir), nil
			}
			return obs(process.StatePassed, runDir), nil
		},
	}
	d := scopedTestDriver(store, clk, proc, git)
	grant, req := prepareScopedStart(t, store)

	base, err := d.Start(req)
	if err != nil {
		t.Fatalf("baseline Start: %v", err)
	}
	if base.Outcome != PASSED {
		t.Fatalf("baseline must PASS, got %s (%s)", base.Outcome, base.Cause)
	}

	redReq := req
	redReq.PredecessorDriveID = base.DriveID
	redReq.PredecessorOwnerGen = base.Generation
	red, err := d.Start(redReq)
	if err != nil {
		t.Fatalf("RED Start: %v", err)
	}
	if red.Outcome != FAILED {
		t.Fatalf("RED must FAIL, got %s (%s)", red.Outcome, red.Cause)
	}

	git.status = "EDITED-BETWEEN-RED-AND-GREEN"

	greenReq := req
	greenReq.PredecessorDriveID = red.DriveID
	greenReq.PredecessorOwnerGen = red.Generation
	green, err := d.Start(greenReq)
	if err != nil {
		t.Fatalf("GREEN Start: %v", err)
	}
	if green.Outcome != PASSED {
		t.Fatalf("GREEN must PASS, got %s (%s)", green.Outcome, green.Cause)
	}

	ids := map[string]bool{base.DriveID: true, red.DriveID: true, green.DriveID: true}
	if len(ids) != 3 {
		t.Fatalf("baseline/RED/GREEN must be three distinct drives, got ids %v", ids)
	}
	if proc.launchN != 3 {
		t.Fatalf("each of three drives must launch exactly once, got %d launches", proc.launchN)
	}
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.DriveCount != 3 {
		t.Fatalf("the scope must have admitted three drives, got DriveCount=%d", scope.DriveCount)
	}
	if scope.CurrentDriveID != green.DriveID {
		t.Fatalf("the current drive must be GREEN, got %q", scope.CurrentDriveID)
	}
	if scope.PriorDriveID != red.DriveID {
		t.Fatalf("the prior drive must be RED, got %q", scope.PriorDriveID)
	}
	baseRec, _ := store.Load(base.DriveID)
	redRec, _ := store.Load(red.DriveID)
	greenRec, _ := store.Load(green.DriveID)
	if !baseRec.Fingerprint.Equal(redRec.Fingerprint) {
		t.Fatalf("baseline and RED ran over the same worktree; their fingerprints must match")
	}
	if greenRec.Fingerprint.Equal(redRec.Fingerprint) {
		t.Fatalf("the edit between RED and GREEN must change the GREEN drive's fingerprint")
	}
	if baseRec.OwnerGeneration != "" || redRec.OwnerGeneration != "" {
		t.Fatalf("acknowledged predecessors must have their owner generation cleared")
	}
	if greenRec.OwnerGeneration == "" {
		t.Fatalf("the current (unacknowledged) GREEN drive must retain its owner generation")
	}
}

// scopedSuccessorFixture prepares a fresh scope (pinning a gate context) over a new
// store, drives its first drive to a durable PASSED, and returns the driver, store,
// grant, and the base successor request carrying the complete identity bundle plus
// the valid predecessor receipt, together with the process seam. Every run PASSES
// immediately, so a valid successor also PASSES.
func scopedSuccessorFixture(t *testing.T) (*Driver, *Store, ScopeGrant, StartRequest, *fakeProc) {
	t.Helper()
	const gateCtx = "task-4-dispatch-context"
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := passObserveProc()
	d := scopedTestDriver(store, clk, proc, stableGit())

	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, gateCtx))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	req.GateContext = gateCtx

	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if first.Outcome != PASSED {
		t.Fatalf("first start must PASS, got %s (%s)", first.Outcome, first.Cause)
	}

	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	return d, store, grant, succ, proc
}

// TestScopedSuccessorRejectionMatrix reproduces spec verifications 3–4 (start half):
// a successor request with any single identity/context/capability/receipt dimension
// wrong is a typed rejection that launches NOTHING and does NOT consume the
// legitimate predecessor — a correct successor still succeeds afterward.
func TestScopedSuccessorRejectionMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(r *StartRequest)
		want   OwnershipErrorKind
	}{
		{"wrong repo-dir", func(r *StartRequest) { r.RepoDir = "/other-repo" }, ErrScopeIdentityMismatch},
		{"wrong branch", func(r *StartRequest) { r.Branch = "feat/other" }, ErrScopeIdentityMismatch},
		{"wrong worktree", func(r *StartRequest) { r.Worktree = "/other-worktree" }, ErrScopeIdentityMismatch},
		{"wrong change", func(r *StartRequest) { r.ChangeID = "9999" }, ErrScopeIdentityMismatch},
		{"wrong task", func(r *StartRequest) { r.TaskID = "task-99" }, ErrScopeIdentityMismatch},
		{"wrong phase", func(r *StartRequest) { r.Phase = "finalize" }, ErrScopeIdentityMismatch},
		{"wrong gate context", func(r *StartRequest) { r.GateContext = "not-the-context" }, ErrScopeIdentityMismatch},
		{"missing gate context", func(r *StartRequest) { r.GateContext = "" }, ErrScopeIdentityMismatch},
		{"wrong capability", func(r *StartRequest) { r.ChildCapability = "wrong-capability" }, ErrScopeCapabilityMismatch},
		{"missing predecessor id", func(r *StartRequest) { r.PredecessorDriveID = "" }, ErrStalePredecessor},
		{"missing predecessor generation", func(r *StartRequest) { r.PredecessorOwnerGen = "" }, ErrStalePredecessor},
		{"nonexistent predecessor id", func(r *StartRequest) { r.PredecessorDriveID = "ffffffffffffffffffffffffffffffff" }, ErrStalePredecessor},
		{"wrong predecessor generation", func(r *StartRequest) { r.PredecessorOwnerGen = "not-the-owner" }, ErrStalePredecessor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, store, _, base, proc := scopedSuccessorFixture(t)
			predID := base.PredecessorDriveID
			predGen := base.PredecessorOwnerGen
			launchesBefore := proc.launchN

			bad := base
			tc.mutate(&bad)
			if _, err := d.Start(bad); !isOwnershipKind(err, tc.want) {
				t.Fatalf("%s: want %s, got %v", tc.name, tc.want, err)
			}
			if proc.launchN != launchesBefore {
				t.Fatalf("%s: a rejected successor must not launch, launched %d->%d", tc.name, launchesBefore, proc.launchN)
			}
			prec, err := store.Load(predID)
			if err != nil {
				t.Fatalf("Load predecessor: %v", err)
			}
			if prec.OwnerGeneration != predGen {
				t.Fatalf("%s: a rejected successor must not consume the predecessor: owner %q want %q", tc.name, prec.OwnerGeneration, predGen)
			}
			ok, err := d.Start(base)
			if err != nil {
				t.Fatalf("%s: a correct successor after a rejected one must succeed: %v", tc.name, err)
			}
			if ok.Outcome != PASSED || ok.DriveID == predID {
				t.Fatalf("%s: correct successor must be a new PASSED drive, got %s id=%q", tc.name, ok.Outcome, ok.DriveID)
			}
		})
	}
}

// TestScopedSuccessorPredecessorStateRejections reproduces spec verification 4
// (predecessor-state half): a successor is refused when its predecessor is not a
// durable reusable result — live (WAITING), HALTED, carrying an outstanding
// handoff, under a closed scope, owned by a superseded generation, or already
// acknowledged. Each rejection launches nothing.
func TestScopedSuccessorPredecessorStateRejections(t *testing.T) {
	t.Run("waiting predecessor not reusable", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		proc := &fakeProc{}
		d := scopedTestDriver(store, clk, proc, stableGit())
		_, req := prepareScopedStart(t, store)
		first, err := d.Start(req)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if first.Outcome != WAITING {
			t.Fatalf("want WAITING first drive, got %s", first.Outcome)
		}
		succ := req
		succ.PredecessorDriveID = first.DriveID
		succ.PredecessorOwnerGen = first.Generation
		launchesBefore := proc.launchN
		if _, err := d.Start(succ); !isOwnershipKind(err, ErrPredecessorNotReusable) {
			t.Fatalf("a WAITING predecessor must reject a successor ErrPredecessorNotReusable, got %v", err)
		}
		if proc.launchN != launchesBefore {
			t.Fatalf("no launch on a rejected successor, launched %d->%d", launchesBefore, proc.launchN)
		}
	})

	t.Run("halted predecessor not reusable", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		proc := &fakeProc{}
		d := scopedTestDriver(store, clk, proc, stableGit())
		_, req := prepareScopedStart(t, store)
		first, err := d.Start(req)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		rec, err := store.Load(first.DriveID)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		rec.LastOutcome = HALTED
		rec.LastCause = "some-halt"
		overwriteDriveRecord(t, store, first.DriveID, rec)
		succ := req
		succ.PredecessorDriveID = first.DriveID
		succ.PredecessorOwnerGen = first.Generation
		launchesBefore := proc.launchN
		if _, err := d.Start(succ); !isOwnershipKind(err, ErrPredecessorNotReusable) {
			t.Fatalf("a HALTED predecessor must reject a successor ErrPredecessorNotReusable, got %v", err)
		}
		if proc.launchN != launchesBefore {
			t.Fatalf("no launch on a rejected successor")
		}
	})

	t.Run("outstanding handoff", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		proc := &fakeProc{}
		d := scopedTestDriver(store, clk, proc, stableGit())
		_, req := prepareScopedStart(t, store)
		first, err := d.Start(req)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if _, err := d.Handoff(first.DriveID, first.Generation); err != nil {
			t.Fatalf("Handoff: %v", err)
		}
		succ := req
		succ.PredecessorDriveID = first.DriveID
		succ.PredecessorOwnerGen = first.Generation
		launchesBefore := proc.launchN
		if _, err := d.Start(succ); !isOwnershipKind(err, ErrHandoffOutstanding) {
			t.Fatalf("an outstanding handoff must reject a successor ErrHandoffOutstanding, got %v", err)
		}
		if proc.launchN != launchesBefore {
			t.Fatalf("no launch on a rejected successor")
		}
	})

	t.Run("closed scope", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		proc := passObserveProc()
		d := scopedTestDriver(store, clk, proc, stableGit())
		grant, req := prepareScopedStart(t, store)
		first, err := d.Start(req)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if err := store.closeScope(grant.ScopeID); err != nil {
			t.Fatalf("closeScope: %v", err)
		}
		succ := req
		succ.PredecessorDriveID = first.DriveID
		succ.PredecessorOwnerGen = first.Generation
		launchesBefore := proc.launchN
		if _, err := d.Start(succ); !isOwnershipKind(err, ErrScopeClosed) {
			t.Fatalf("a closed scope must reject a successor ErrScopeClosed, got %v", err)
		}
		if proc.launchN != launchesBefore {
			t.Fatalf("no launch on a rejected successor")
		}
	})

	t.Run("superseded owner", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		proc := passObserveProc()
		d := scopedTestDriver(store, clk, proc, stableGit())
		_, req := prepareScopedStart(t, store)
		first, err := d.Start(req)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		rec, err := store.Load(first.DriveID)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		rec.OwnerGeneration = "superseded-by-a-takeover"
		overwriteDriveRecord(t, store, first.DriveID, rec)
		succ := req
		succ.PredecessorDriveID = first.DriveID
		succ.PredecessorOwnerGen = first.Generation
		launchesBefore := proc.launchN
		if _, err := d.Start(succ); !isOwnershipKind(err, ErrStalePredecessor) {
			t.Fatalf("a superseded owner must reject a successor ErrStalePredecessor, got %v", err)
		}
		if proc.launchN != launchesBefore {
			t.Fatalf("no launch on a rejected successor")
		}
	})

	t.Run("acknowledged earlier drive", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		store := OpenStore(testsupport.TempDir(t))
		proc := passObserveProc()
		d := scopedTestDriver(store, clk, proc, stableGit())
		_, req := prepareScopedStart(t, store)
		first, err := d.Start(req)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		succ := req
		succ.PredecessorDriveID = first.DriveID
		succ.PredecessorOwnerGen = first.Generation
		if _, err := d.Start(succ); err != nil {
			t.Fatalf("successor Start: %v", err)
		}
		stale := req
		stale.PredecessorDriveID = first.DriveID
		stale.PredecessorOwnerGen = first.Generation
		launchesBefore := proc.launchN
		if _, err := d.Start(stale); !isOwnershipKind(err, ErrStalePredecessor) {
			t.Fatalf("an already-acknowledged predecessor must reject a successor ErrStalePredecessor, got %v", err)
		}
		if proc.launchN != launchesBefore {
			t.Fatalf("no launch on a rejected successor")
		}
	})
}
