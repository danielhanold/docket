package gatedrive

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	launch   func(process.LaunchRequest) (*process.LaunchOutcome, error)
	observe  func(runDir string) (*process.Observation, error)
	stop     func(runDir, reason string) (*process.StopOutcome, error)
	resolve  func(root, token string) (*process.ReservationResolution, error)
	leftover func(runDir string) (process.Leftover, error)

	// retainLock, when set, leaves the handed worktree lock to the launch
	// closure (a test that emulates a supervisor still holding it); by default
	// Launch closes it after the closure runs, as the real process.Launch closes
	// the caller's copy on every path.
	retainLock bool

	launchN, observeN, stopN, resolveN, leftoverN int
}

func (f *fakeProc) Launch(r process.LaunchRequest) (*process.LaunchOutcome, error) {
	f.launchN++
	if !f.retainLock {
		defer releaseHandedLock(r)
	}
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
// outcome of a launch that returned an error with no run dir. Tests that model a
// lost launch response inject a closure returning "unresolved" or "identified".
func (f *fakeProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	f.resolveN++
	if f.resolve == nil {
		return &process.ReservationResolution{Disposition: "never-launched"}, nil
	}
	return f.resolve(root, token)
}

// ProbeLeftover defaults to none — an exited supervisor whose group is empty —
// so every existing census test keeps its run-terminal finding. Tests that
// model a leftover suite inject a closure.
func (f *fakeProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	f.leftoverN++
	if f.leftover == nil {
		return process.Leftover{Answer: process.LeftoverNone}, nil
	}
	return f.leftover(runDir)
}

// mkWorktree returns a fresh, real directory to stand in for a worktree root
// (no git: the default corpus never starts git).
func mkWorktree(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(testsupport.TempDir(t), "wt")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkWorktree: %v", err)
	}
	return dir
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
	// rootErr, when set, is WorktreeRoot's error (a cwd outside any worktree).
	rootErr error
}

func (g *fakeGit) HeadOID(string) (string, error)       { return g.head, g.err }
func (g *fakeGit) IndexEntries(string) ([]byte, error)  { return []byte(g.index), g.err }
func (g *fakeGit) Status(string) ([]byte, error)        { return []byte(g.status), g.err }
func (g *fakeGit) WorktreePaths(string) ([]byte, error) { return nil, g.err }
func (g *fakeGit) WorktreeRoot(dir string) (string, error) {
	if g.rootErr != nil {
		return "", g.rootErr
	}
	return fakeWorktreeRoot(dir), nil
}

// fakeWorktreeRoot stands in for gitcli.DiscoverWorktree's canonical root
// without starting git: the symlink-resolved dir when it exists, else dir
// cleaned.
func fakeWorktreeRoot(dir string) string {
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		return r
	}
	return filepath.Clean(dir)
}

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
// canonical, existing directory used as the sample start worktree. One shared
// directory is safe across tests: every test owns a fresh Store (a fresh lock
// root under testsupport.TempDir), so its worktree lock is isolated even though
// the worktree key is shared. The worktree lock keys on the launch cwd
// (sampleStart's "/repo", resolved by fakeGit.WorktreeRoot), and the git seam is
// faked, so ComputeFingerprint never touches the directory.
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
	// One slice observation, plus the holder note's pre-write liveness check
	// (WorktreeLock.WriteHolder observes the launched run before writing).
	if proc.observeN != 2 {
		t.Fatalf("zero budget must take exactly one slice observation (2 with the holder-note check), took %d", proc.observeN)
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
// Death: a supervisor death always halts (change 0493).
// ---------------------------------------------------------------------------

// deathAfterFirstSlice starts req with its first run live for the first slice;
// once *dead is set, that run observes state, so the next Advance reaches the
// death branch.
func deathAfterFirstSlice(t *testing.T, req StartRequest, state process.State) (*Driver, *Store, *fakeProc, DriveDoc, *bool) {
	t.Helper()
	clk := &fakeClock{now: startRun()}
	dead := false
	proc := &fakeProc{observe: func(runDir string) (*process.Observation, error) {
		if dead && strings.HasSuffix(runDir, "run1") {
			return obs(state, runDir), nil
		}
		return obs(process.StateRunning, runDir), nil
	}}
	d, store := newTestDriver(t, clk, proc, stableGit())
	started, err := d.Start(req)
	if err != nil || started.Outcome != WAITING {
		t.Fatalf("Start = %s (%v), want WAITING", started.Outcome, err)
	}
	return d, store, proc, started, &dead
}

// TestDeathHaltsSupervisorDied (change 0493): every drive whose run dies — one
// started as finalize starts one or as a build drive, signaled or vanished,
// within or past its deadline, worktree unchanged or drifted, with or without
// another gate holding the worktree — HALTs supervisor-died with its one
// launch. Nothing launches again, the worktree lock is never re-taken, and the
// record keeps its first run.
func TestDeathHaltsSupervisorDied(t *testing.T) {
	for _, tc := range []struct {
		name       string
		owner      string
		idempotent bool
		state      process.State
		holdOther  bool
	}{
		{"finalize signaled", "finalize", true, process.StateSignaled, false},
		{"finalize vanished", "finalize", true, process.StateVanished, false},
		{"build signaled", "build", false, process.StateSignaled, false},
		{"build vanished", "build", false, process.StateVanished, false},
		{"another gate took the worktree", "finalize", true, process.StateVanished, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := sampleStart()
			req.Owner = tc.owner
			req.IdempotentSuiteGate = tc.idempotent
			d, store, proc, started, dead := deathAfterFirstSlice(t, req, tc.state)
			if tc.holdOther {
				holdWorktree(t, store, req.Cwd)
			}
			*dead = true

			doc, err := d.Advance(started.DriveID, started.Generation)
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied {
				t.Fatalf("a death = %s/%q, want HALTED/%s", doc.Outcome, doc.Cause, CauseSupervisorDied)
			}
			if proc.launchN != 1 {
				t.Fatalf("a death must never launch again: launches = %d, want 1", proc.launchN)
			}
			rec, err := store.Load(started.DriveID)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if rec.Attempt != 1 || rec.RawRunDir != "/runs/run1" || rec.LastCause != CauseSupervisorDied {
				t.Fatalf("record = attempt %d run %q cause %q, want 1 /runs/run1 %s", rec.Attempt, rec.RawRunDir, rec.LastCause, CauseSupervisorDied)
			}
			if !tc.holdOther && !worktreeFree(t, store, req.Cwd) {
				t.Fatalf("a halted drive must hold no worktree lock")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*driveRecord)
		drift  bool
	}{
		{name: "deadline already past", mutate: func(r *driveRecord) { r.Deadline = startRun().Add(-time.Minute) }},
		{name: "worktree drifted", drift: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git := stableGit()
			proc := &fakeProc{
				observe: func(runDir string) (*process.Observation, error) { return obs(process.StateSignaled, runDir), nil },
				stop: func(runDir, reason string) (*process.StopOutcome, error) {
					return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
						Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
				},
			}
			d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, git)
			rec := seedRecord(t)
			if tc.mutate != nil {
				tc.mutate(&rec)
			}
			id, ownerGen := seedDrive(t, store, rec)
			if tc.drift {
				git.status = "DRIFTED"
			}
			doc, err := d.Advance(id, ownerGen)
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied || proc.launchN != 0 {
				t.Fatalf("a death = %s/%q launches %d, want HALTED/%s and no launch", doc.Outcome, doc.Cause, proc.launchN, CauseSupervisorDied)
			}
		})
	}
}

// TestDeathUnprovenHaltsUncertainOwnership: a death whose tree cannot be proven
// gone (the probe stop errors) HALTs uncertain-ownership, as before change 0493,
// and launches nothing.
func TestDeathUnprovenHaltsUncertainOwnership(t *testing.T) {
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) { return obs(process.StateSignaled, runDir), nil },
		stop: func(runDir, reason string) (*process.StopOutcome, error) {
			return nil, fmt.Errorf("gatedrive-test: stop cannot prove ownership")
		},
	}
	d, store := newTestDriver(t, &fakeClock{now: startRun().Add(time.Second)}, proc, stableGit())
	id, ownerGen := seedDrive(t, store, seedRecord(t))
	doc, err := d.Advance(id, ownerGen)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != "uncertain-ownership" || proc.launchN != 0 {
		t.Fatalf("an unproven death = %s/%q launches %d, want HALTED/uncertain-ownership and no launch", doc.Outcome, doc.Cause, proc.launchN)
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
// Worktree lock admission (change 0490). Every drive launch site takes the
// canonical worktree's lock before it launches and hands it to the supervisor,
// so one worktree carries at most one live gate and a dead gate frees it with no
// recovery step.
// ---------------------------------------------------------------------------

// releaseHandedLock closes a launch request's handed worktree lock, as the real
// process.Launch does on every path. A fake has no supervisor to keep the lock,
// so closing it emulates one that already exited.
func releaseHandedLock(req process.LaunchRequest) {
	if req.WorktreeLock != nil {
		_ = req.WorktreeLock.Close()
	}
}

// worktreeLockDir is the lock directory a drive whose launch cwd is cwd keys on
// (fakeGit resolves the root through fakeWorktreeRoot).
func worktreeLockDir(store *Store, cwd string) string {
	return filepath.Join(store.lockRoot, worktreeLockKey(fakeWorktreeRoot(cwd)))
}

// worktreeFree reports whether cwd's worktree lock is free right now. It takes
// and closes the lock, so it leaves no hold behind.
func worktreeFree(t *testing.T, store *Store, cwd string) bool {
	t.Helper()
	l, err := store.TryWorktreeLock(fakeWorktreeRoot(cwd), nil)
	if isOwnershipKind(err, ErrWorktreeBusy) {
		return false
	}
	if err != nil {
		t.Fatalf("TryWorktreeLock(%s): %v", cwd, err)
	}
	l.Release()
	return true
}

// holdWorktree takes cwd's worktree lock as another gate would, until cleanup.
func holdWorktree(t *testing.T, store *Store, cwd string) {
	t.Helper()
	l, err := store.TryWorktreeLock(fakeWorktreeRoot(cwd), nil)
	if err != nil {
		t.Fatalf("hold the worktree lock: %v", err)
	}
	t.Cleanup(l.Release)
}

// soleDriveID returns the one drive id the store holds.
func soleDriveID(t *testing.T, store *Store) string {
	t.Helper()
	entries, err := os.ReadDir(store.root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("want exactly one drive record, got %d (err=%v)", len(entries), err)
	}
	return entries[0].Name()
}

var launchTokenShape = regexp.MustCompile("^[0-9a-f]{32}$")

// TestAdmitRefusesWorktreeBusyAndCreatesNoDrive: a worktree whose lock another
// gate holds refuses the admission with a typed worktree-busy at the
// worktree-admission stage, creates no drive, and launches nothing.
func TestAdmitRefusesWorktreeBusyAndCreatesNoDrive(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()
	holdWorktree(t, store, req.Cwd)

	ticket, err := d.Admit(req)
	if ticket != nil {
		t.Fatalf("a busy worktree must not admit, got a ticket")
	}
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrWorktreeBusy || oe.Op != opWorktreeAdmission {
		t.Fatalf("Admit over a held worktree = %v, want worktree-busy at %s", err, opWorktreeAdmission)
	}
	if n := driveRecordCount(t, store); n != 0 {
		t.Fatalf("a busy refusal must create no drive, got %d", n)
	}
	if proc.launchN != 0 {
		t.Fatalf("a busy refusal must launch nothing, launched %d", proc.launchN)
	}
}

// TestAdmitUnresolvedWorktreeIsRefusedNotFree: a launch cwd that resolves to no
// git worktree is a typed worktree-unresolved refusal naming that cwd — never
// read as a free worktree, and never an I/O (internal) failure — and creates no
// drive.
func TestAdmitUnresolvedWorktreeIsRefusedNotFree(t *testing.T) {
	git := stableGit()
	git.rootErr = errors.New("gatedrive-test: worktree root unresolvable")
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, git)
	req := sampleStart()

	ticket, err := d.Admit(req)
	if ticket != nil {
		t.Fatalf("an unresolvable worktree must not admit")
	}
	oe, ok := AsOwnershipError(err)
	if !ok || oe.Kind != ErrWorktreeUnresolved || oe.Op != opWorktreeAdmission {
		t.Fatalf("Admit with an unresolvable root = %v, want a typed %s refusal at %s", err, ErrWorktreeUnresolved, opWorktreeAdmission)
	}
	if oe.Cwd != req.Cwd {
		t.Fatalf("worktree-unresolved refusal Cwd = %q, want the launch cwd %q", oe.Cwd, req.Cwd)
	}
	if _, isStore := AsStoreError(err); isStore {
		t.Fatalf("an unresolvable worktree must not be a store (internal) error: %v", err)
	}
	if n := driveRecordCount(t, store); n != 0 {
		t.Fatalf("an I/O refusal must create no drive, got %d", n)
	}
	if proc.launchN != 0 {
		t.Fatalf("an I/O refusal must launch nothing, launched %d", proc.launchN)
	}
}

// TestAbandonAdmissionFreesWorktree: an admitted ticket holds the worktree;
// abandoning it removes the never-launched reserved drive and frees the
// worktree, launches nothing, and leaves a ticket that can never launch.
func TestAbandonAdmissionFreesWorktree(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()

	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if worktreeFree(t, store, req.Cwd) {
		t.Fatalf("an admitted ticket must hold the worktree lock")
	}
	if err := d.AbandonAdmission(ticket); err != nil {
		t.Fatalf("AbandonAdmission: %v", err)
	}
	if !worktreeFree(t, store, req.Cwd) {
		t.Fatalf("an abandoned admission must free the worktree")
	}
	if _, lerr := store.Load(ticket.id); lerr == nil {
		t.Fatalf("an abandoned admission must remove its reserved drive record")
	}
	if _, serr := d.StartAdmitted(ticket); !isOwnershipKind(serr, ErrUnresolvedLaunchTransition) {
		t.Fatalf("an abandoned ticket must never launch, StartAdmitted = %v", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("an abandoned admission must launch nothing, launched %d", proc.launchN)
	}
}

// TestLaunchFailureFreesWorktree: a launch that returns an error (the real
// Launch closes the handed lock on every path) leaves the drive HALTED
// launch-failed as evidence and the worktree free — no reservation resolution
// and no recovery step are needed before the next start admits.
func TestLaunchFailureFreesWorktree(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		return nil, errors.New("gatedrive-test: supervisor spawn failed")
	}}
	d, store := newTestDriver(t, clk, proc, stableGit())
	req := sampleStart()

	if _, err := d.Start(req); err == nil {
		t.Fatalf("a failed launch must be a command failure")
	}
	rec, err := store.Load(soleDriveID(t, store))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "launch-failed" {
		t.Fatalf("failed-launch drive = %s/%q, want HALTED/launch-failed", rec.LastOutcome, rec.LastCause)
	}
	if !worktreeFree(t, store, req.Cwd) {
		t.Fatalf("a failed launch must free the worktree")
	}
	if proc.resolveN != 0 {
		t.Fatalf("a failed first launch needs no reservation resolution, resolved %d", proc.resolveN)
	}
	if _, err := d.Admit(req); err != nil {
		t.Fatalf("the next start must admit with no recovery step: %v", err)
	}
}

// TestLaunchHandsLockAndWritesHolder: admission precedes launch — at Launch the
// reserved drive exists and the worktree is held by the handed lock — the launch
// carries the drive-minted 32-hex launch token, and after the launch the holder
// note names the drive, its run dir, its change, and its owner.
func TestLaunchHandsLockAndWritesHolder(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	var store *Store
	var (
		handed, heldAtLaunch bool
		reservedAtLaunch     int
		token                string
	)
	proc := &fakeProc{}
	proc.launch = func(r process.LaunchRequest) (*process.LaunchOutcome, error) {
		handed = r.WorktreeLock != nil
		token = r.ReservationToken
		heldAtLaunch = !worktreeFree(t, store, r.Cwd)
		reservedAtLaunch = driveRecordCount(t, store)
		return &process.LaunchOutcome{RunID: "run1", RunDir: "/runs/run1", State: process.StateRunning}, nil
	}
	d, s := newTestDriver(t, clk, proc, stableGit())
	store = s
	req := sampleStart()
	req.Owner = "build"

	doc, err := d.Start(req)
	if err != nil || doc.Outcome != WAITING {
		t.Fatalf("Start = %s (%v), want WAITING", doc.Outcome, err)
	}
	if !handed || !heldAtLaunch {
		t.Fatalf("the launch must carry the held worktree lock: handed=%v held=%v", handed, heldAtLaunch)
	}
	if reservedAtLaunch != 1 {
		t.Fatalf("the reserved drive must exist before the launch, found %d", reservedAtLaunch)
	}
	rec, err := store.Load(doc.DriveID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !launchTokenShape.MatchString(token) || rec.AdmissionToken != token {
		t.Fatalf("launch token %q must be the drive's own 32-hex admission_token %q", token, rec.AdmissionToken)
	}
	note, ok := readHolderNote(worktreeLockDir(store, req.Cwd))
	if !ok {
		t.Fatalf("the launch must write the holder note")
	}
	if note.Kind != "drive" || note.DriveID != doc.DriveID || note.RunDir != "/runs/run1" ||
		note.ChangeID != req.ChangeID || note.Owner != "build" || note.WrittenAt.IsZero() {
		t.Fatalf("holder note = %+v, want drive %s run /runs/run1 change %s owner build", note, doc.DriveID, req.ChangeID)
	}
}

// TestStartAdmittedRefusesConsumedTicket: a ticket launches at most once. A
// second StartAdmitted over an already-launched ticket — whose lock now belongs
// to the supervisor — refuses and launches nothing.
func TestStartAdmittedRefusesConsumedTicket(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, _ := newTestDriver(t, clk, proc, stableGit())

	ticket, err := d.Admit(sampleStart())
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	if doc, err := d.StartAdmitted(ticket); err != nil || doc.Outcome != WAITING {
		t.Fatalf("first StartAdmitted = %s (%v), want WAITING", doc.Outcome, err)
	}
	if _, err := d.StartAdmitted(ticket); !isOwnershipKind(err, ErrUnresolvedLaunchTransition) {
		t.Fatalf("a consumed ticket must refuse, got %v", err)
	}
	if proc.launchN != 1 {
		t.Fatalf("a consumed ticket must never launch again, launched %d", proc.launchN)
	}
}

// TestScopelessAttachFailureStopsFreshRun: a launch whose handle cannot be
// attached stops the fresh run (its supervisor's exit then frees the worktree)
// and fails the start — never an automatic second launch.
func TestScopelessAttachFailureStopsFreshRun(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	store := OpenStore(testsupport.TempDir(t))
	var recordDir string
	proc := &fakeProc{
		launch: func(process.LaunchRequest) (*process.LaunchOutcome, error) {
			recordDir = filepath.Join(store.root, soleDriveID(t, store))
			if err := os.Chmod(recordDir, 0o500); err != nil {
				return nil, fmt.Errorf("chmod reserved drive: %w", err)
			}
			return &process.LaunchOutcome{RunID: "run1", RunDir: "/runs/run1", State: process.StateRunning}, nil
		},
	}
	var stopped []string
	proc.stop = func(runDir, _ string) (*process.StopOutcome, error) {
		stopped = append(stopped, runDir)
		return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
	}
	d := storeTestDriver(store, clk, proc, stableGit())

	_, err := d.Start(sampleStart())
	if recordDir != "" {
		_ = os.Chmod(recordDir, 0o700)
	}
	if err == nil {
		t.Fatalf("an attach failure must fail the start")
	}
	if len(stopped) != 1 || stopped[0] != "/runs/run1" {
		t.Fatalf("an attach failure must stop exactly the fresh run, stopped %v", stopped)
	}
	if proc.launchN != 1 {
		t.Fatalf("an attach failure must not launch again, launched %d", proc.launchN)
	}
}

// TestCorruptReservedDriveNeverLaunches: a drive record corrupted in the window
// between Admit and StartAdmitted refuses the delayed launch typed, launches
// nothing, and frees the worktree; advancing a corrupt live drive never
// launches again.
func TestCorruptReservedDriveNeverLaunches(t *testing.T) {
	t.Run("reserved-before-launch", func(t *testing.T) {
		proc := &fakeProc{}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		req := sampleStart()
		ticket, err := d.Admit(req)
		if err != nil {
			t.Fatalf("Admit: %v", err)
		}
		corruptFile(t, filepath.Join(store.root, ticket.id, recordFileName))

		if _, err := d.StartAdmitted(ticket); !isStoreKind(err, ErrCorruptRecord) {
			t.Fatalf("the delayed launch of a corrupt reserved drive must refuse typed, got %v", err)
		}
		if proc.launchN != 0 {
			t.Fatalf("a corrupt reserved drive must never launch, got %d launches", proc.launchN)
		}
		if !worktreeFree(t, store, req.Cwd) {
			t.Fatalf("a refused launch must free the worktree")
		}
	})
	t.Run("corrupt-live-drive", func(t *testing.T) {
		proc := &fakeProc{}
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
		doc, err := d.Start(sampleStart())
		if err != nil || doc.Outcome != WAITING {
			t.Fatalf("Start: doc=%+v err=%v", doc, err)
		}
		corruptFile(t, filepath.Join(store.root, doc.DriveID, recordFileName))

		launches := proc.launchN
		if adv, err := d.Advance(doc.DriveID, doc.Generation); err == nil && adv.Outcome != HALTED {
			t.Fatalf("advancing a corrupt drive must halt or fail, got %+v", adv)
		}
		if proc.launchN != launches {
			t.Fatal("a corrupt drive must never launch again")
		}
	})
}

// TestTerminalDocAlwaysExposesRunRoot: every terminal document — PASSED,
// FAILED, and HALTED alike — exposes the drive's run root so its owner removes
// it; a live (WAITING) drive never does.
func TestTerminalDocAlwaysExposesRunRoot(t *testing.T) {
	for _, outcome := range []Outcome{PASSED, FAILED, HALTED} {
		t.Run(string(outcome), func(t *testing.T) {
			clk := &fakeClock{now: startRun()}
			d, store := newTestDriver(t, clk, &fakeProc{}, stableGit())
			rec := seedRecord(t)
			rec.LastOutcome = outcome
			if outcome == HALTED {
				rec.LastCause = "uncertain-ownership"
			}
			id, owner := seedDrive(t, store, rec)
			doc, err := d.Advance(id, owner)
			if err != nil {
				t.Fatalf("Advance: %v", err)
			}
			if doc.Outcome != outcome || doc.RunRoot != rec.RunRoot {
				t.Fatalf("%s document run root = %q, want %q", doc.Outcome, doc.RunRoot, rec.RunRoot)
			}
		})
	}
	t.Run(string(WAITING), func(t *testing.T) {
		clk := &fakeClock{now: startRun()}
		d, _ := newTestDriver(t, clk, &fakeProc{}, stableGit())
		doc, err := d.Start(sampleStart())
		if err != nil || doc.Outcome != WAITING || doc.RunRoot != "" {
			t.Fatalf("a WAITING document must retain its run root: %s root=%q (%v)", doc.Outcome, doc.RunRoot, err)
		}
	})
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

// driveRecordCount reports how many drive records the store holds. The store
// mints nothing until the first NewDrive/NewReservedDrive, so an absent root is
// zero — a gate refusal that reserves nothing leaves the root absent.
func driveRecordCount(t *testing.T, store *Store) int {
	t.Helper()
	entries, err := os.ReadDir(store.root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("ReadDir drive root: %v", err)
	}
	return len(entries)
}
