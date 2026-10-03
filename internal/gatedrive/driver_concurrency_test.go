package gatedrive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// racingProc is a thread-safe ProcessSeam purpose-built to race two same-owner
// advances over one death deterministically: the seeded original run is dead
// (signaled), and its first two observations rendezvous both advances before
// either may persist. Launch only counts — a drive never relaunches (change
// 0493), so it must never be called. All bookkeeping is mutex-guarded so the
// test is clean under -race.
type racingProc struct {
	barrier *sync.WaitGroup // trips once both advances observed the dead run

	mu        sync.Mutex
	launchSeq int
	deathObs  int
}

func newRacingProc() *racingProc {
	var b sync.WaitGroup
	b.Add(2)
	return &racingProc{barrier: &b}
}

func (p *racingProc) Launch(req process.LaunchRequest) (*process.LaunchOutcome, error) {
	defer releaseHandedLock(req)
	p.mu.Lock()
	p.launchSeq++
	id := fmt.Sprintf("relaunch%d", p.launchSeq)
	p.mu.Unlock()
	return &process.LaunchOutcome{RunID: id, RunDir: "/runs/" + id, State: process.StateRunning}, nil
}

func (p *racingProc) Observe(runDir string) (*process.Observation, error) {
	if strings.HasSuffix(runDir, "run1") {
		p.mu.Lock()
		block := p.deathObs < 2
		if block {
			p.deathObs++
		}
		p.mu.Unlock()
		if block {
			p.barrier.Done()
			p.barrier.Wait()
		}
		return &process.Observation{State: process.StateSignaled, RunDir: runDir}, nil
	}
	return &process.Observation{State: process.StateRunning, RunDir: runDir}, nil
}

func (p *racingProc) Stop(runDir, reason string) (*process.StopOutcome, error) {
	// The signaled original is already terminal: an ownership-proven no-op.
	return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
		Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
}

func (p *racingProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	return &process.ReservationResolution{Disposition: "never-launched"}, nil
}

func (p *racingProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	return process.Leftover{Answer: process.LeftoverNone}, nil
}

// TestConcurrentSameOwnerAdvanceOverDeathHaltsOnce (change 0493): two concurrent
// Advance calls presenting the SAME valid owner generation over a drive whose run
// has died both observe the death before either persists (racingProc's barrier),
// and both return the one recorded HALTED supervisor-died verdict with no error.
// Neither launches anything.
func TestConcurrentSameOwnerAdvanceOverDeathHaltsOnce(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	id, ownerGen := seedDrive(t, store, seedRecord(t))
	proc := newRacingProc()

	mkDriver := func() *Driver {
		clk := &fakeClock{now: startRun().Add(time.Second)}
		d := NewDriver(store, clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		return d
	}
	drivers := []*Driver{mkDriver(), mkDriver()}
	docs := make([]DriveDoc, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range drivers {
		go func(i int) {
			defer wg.Done()
			docs[i], errs[i] = drivers[i].Advance(id, ownerGen)
		}(i)
	}
	wg.Wait()

	for i := range docs {
		if errs[i] != nil {
			t.Fatalf("advance %d returned an error: %v", i, errs[i])
		}
		if docs[i].Outcome != HALTED || docs[i].Cause != CauseSupervisorDied {
			t.Fatalf("advance %d = %s/%q, want HALTED/%s", i, docs[i].Outcome, docs[i].Cause, CauseSupervisorDied)
		}
	}
	if proc.launchSeq != 0 {
		t.Fatalf("a death must never launch, got %d launches", proc.launchSeq)
	}
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != CauseSupervisorDied || rec.Attempt != 1 || rec.RawRunDir != "/runs/run1" {
		t.Fatalf("record = %s/%q attempt %d run %q, want HALTED/%s attempt 1 /runs/run1", rec.LastOutcome, rec.LastCause, rec.Attempt, rec.RawRunDir, CauseSupervisorDied)
	}
}

// terminalSettleProc is the shared ProcessSeam core for the deterministic
// loser-after-terminal-settle regression: the seeded original run observes as
// signaled (dead), so the winner settles the drive HALTED supervisor-died within
// its single Advance. Launch only counts — a drive never relaunches (change
// 0493), so it must never be called.
type terminalSettleProc struct {
	mu        sync.Mutex
	launchSeq int
	launched  map[string]bool
	stops     []string
}

func newTerminalSettleProc() *terminalSettleProc {
	return &terminalSettleProc{launched: map[string]bool{}}
}

func (p *terminalSettleProc) Launch(req process.LaunchRequest) (*process.LaunchOutcome, error) {
	defer releaseHandedLock(req)
	p.mu.Lock()
	p.launchSeq++
	id := fmt.Sprintf("relaunch%d", p.launchSeq)
	dir := "/runs/" + id
	p.launched[dir] = true
	p.mu.Unlock()
	return &process.LaunchOutcome{RunID: id, RunDir: dir, State: process.StateRunning}, nil
}

func (p *terminalSettleProc) Observe(runDir string) (*process.Observation, error) {
	if strings.HasSuffix(runDir, "run1") {
		return &process.Observation{State: process.StateSignaled, RunDir: runDir}, nil
	}
	return &process.Observation{State: process.StateFailed, RunDir: runDir}, nil
}

func (p *terminalSettleProc) Stop(runDir, reason string) (*process.StopOutcome, error) {
	p.mu.Lock()
	p.stops = append(p.stops, runDir)
	p.mu.Unlock()
	if strings.HasSuffix(runDir, "run1") {
		return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
			Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
	}
	return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
}

func (p *terminalSettleProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	return &process.ReservationResolution{Disposition: "never-launched"}, nil
}

func (p *terminalSettleProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	return process.Leftover{Answer: process.LeftoverNone}, nil
}

// gatedLoserSeam wraps the shared core for the LOSER driver only: its first
// dead-run observation signals `observing` (proving the loser loaded a
// nonterminal record and entered its slice) and then parks on `gate` until the
// test releases it — after the winner's Advance has fully returned with the
// terminal outcome durably persisted. This forces, deterministically, the
// interleaving where the loser's persist CAS finds a terminal record.
type gatedLoserSeam struct {
	core      *terminalSettleProc
	gate      <-chan struct{}
	observing chan struct{}
	once      sync.Once
}

func (s *gatedLoserSeam) Launch(req process.LaunchRequest) (*process.LaunchOutcome, error) {
	return s.core.Launch(req)
}

func (s *gatedLoserSeam) Observe(runDir string) (*process.Observation, error) {
	if strings.HasSuffix(runDir, "run1") {
		s.once.Do(func() { close(s.observing) })
		<-s.gate
	}
	return s.core.Observe(runDir)
}

func (s *gatedLoserSeam) Stop(runDir, reason string) (*process.StopOutcome, error) {
	return s.core.Stop(runDir, reason)
}

func (s *gatedLoserSeam) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	return s.core.ResolveReservation(root, token)
}

func (s *gatedLoserSeam) ProbeLeftover(runDir string) (process.Leftover, error) {
	return s.core.ProbeLeftover(runDir)
}

// TestLoserAfterTerminalSettleReturnsRecordedState pins the deterministic
// resolution of the 0411 interleaving, as it reads since change 0493: a
// same-owner advance whose persist CAS finds the winner's terminal HALTED
// supervisor-died record must return the authoritative recorded state — never
// the raw errAlreadyTerminal sentinel as an Advance error. Nothing launches and
// the attempt stays 1.
func TestLoserAfterTerminalSettleReturnsRecordedState(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	seeded := seedRecord(t)
	id, ownerGen := seedDrive(t, store, seeded)

	core := newTerminalSettleProc()
	gate := make(chan struct{})
	loserSeam := &gatedLoserSeam{core: core, gate: gate, observing: make(chan struct{})}

	mkDriver := func(seam ProcessSeam) *Driver {
		clk := &fakeClock{now: startRun().Add(time.Second)}
		d := NewDriver(reopenStore(store), clk, seam, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		return d
	}

	type advanceResult struct {
		doc DriveDoc
		err error
	}
	loserDone := make(chan advanceResult, 1)
	go func() {
		doc, err := mkDriver(loserSeam).Advance(id, ownerGen)
		loserDone <- advanceResult{doc: doc, err: err}
	}()

	// The loser is parked inside its slice holding a stale nonterminal record.
	<-loserSeam.observing

	// The winner runs to completion: dead original observed, its tree proven
	// gone, HALTED supervisor-died persisted.
	winnerDoc, winnerErr := mkDriver(core).Advance(id, ownerGen)
	if winnerErr != nil {
		t.Fatalf("winner advance: %v", winnerErr)
	}
	if winnerDoc.Outcome != HALTED || winnerDoc.Cause != CauseSupervisorDied {
		t.Fatalf("winner = %s/%q, want HALTED/%s", winnerDoc.Outcome, winnerDoc.Cause, CauseSupervisorDied)
	}

	// Only now may the loser proceed to its persist attempt.
	close(gate)
	loser := <-loserDone
	if loser.err != nil {
		t.Fatalf("the loser must return recorded state, not an error: %v", loser.err)
	}
	if loser.doc.Outcome != winnerDoc.Outcome {
		t.Fatalf("loser doc outcome = %s (%s), want the winner's %s", loser.doc.Outcome, loser.doc.Cause, winnerDoc.Outcome)
	}
	if loser.doc.Cause != winnerDoc.Cause {
		t.Fatalf("loser doc cause = %q, want the winner's recorded cause %q", loser.doc.Cause, winnerDoc.Cause)
	}

	core.mu.Lock()
	launches := core.launchSeq
	core.mu.Unlock()
	if launches != 0 {
		t.Fatalf("a death must never launch, got %d launches", launches)
	}
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.Attempt != 1 {
		t.Fatalf("Attempt = %d, want 1", rec.Attempt)
	}
	if rec.LastOutcome != HALTED {
		t.Fatalf("recorded outcome = %s (%s), want %s", rec.LastOutcome, rec.LastCause, HALTED)
	}
	if !rec.Deadline.Equal(seeded.Deadline) {
		t.Fatalf("the original deadline must be preserved: got %v, seeded %v", rec.Deadline, seeded.Deadline)
	}
}

// barrierGit blocks its first read (HeadOID, the first call ComputeFingerprint
// makes) on a shared N-party barrier, so concurrent Starts all compute their
// fingerprint BEFORE any tries the worktree lock — maximizing the admission race
// window. The remaining reads are fixed strings so the fingerprint is otherwise
// deterministic.
type barrierGit struct {
	wg   *sync.WaitGroup
	head string
}

func (g *barrierGit) HeadOID(string) (string, error) {
	g.wg.Done()
	g.wg.Wait()
	return g.head, nil
}
func (g *barrierGit) IndexEntries(string) ([]byte, error)     { return []byte("IDX1"), nil }
func (g *barrierGit) Status(string) ([]byte, error)           { return []byte("ST1"), nil }
func (g *barrierGit) WorktreePaths(string) ([]byte, error)    { return nil, nil }
func (g *barrierGit) WorktreeRoot(dir string) (string, error) { return fakeWorktreeRoot(dir), nil }

// countingProc is a minimal thread-safe ProcessSeam that counts launches; every
// launched run stays running so a winning Start reaches WAITING. It is purpose-
// built for the one-worktree admission races, where at most one Start launches.
// Like a live supervisor it RETAINS each handed worktree lock until the test's
// cleanup calls releaseRetained, so a winner keeps the worktree busy.
type countingProc struct {
	mu       sync.Mutex
	launchN  int
	retained []*os.File
}

func (p *countingProc) Launch(req process.LaunchRequest) (*process.LaunchOutcome, error) {
	p.mu.Lock()
	p.launchN++
	n := p.launchN
	if req.WorktreeLock != nil {
		p.retained = append(p.retained, req.WorktreeLock)
	}
	p.mu.Unlock()
	id := fmt.Sprintf("run%d", n)
	return &process.LaunchOutcome{RunID: id, RunDir: "/runs/" + id, State: process.StateRunning}, nil
}

func (p *countingProc) Observe(runDir string) (*process.Observation, error) {
	return &process.Observation{State: process.StateRunning, RunDir: runDir}, nil
}

func (p *countingProc) Stop(runDir, reason string) (*process.StopOutcome, error) {
	return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
}

func (p *countingProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	return &process.ReservationResolution{Disposition: "never-launched"}, nil
}

func (p *countingProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	return process.Leftover{Answer: process.LeftoverNone}, nil
}

func (p *countingProc) launches() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.launchN
}

// releaseRetained closes every worktree lock the fake supervisors kept.
func (p *countingProc) releaseRetained() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, f := range p.retained {
		_ = f.Close()
	}
	p.retained = nil
}

// TestTwoScopelessStartsOneWorktreeOneLaunch proves that two finalize-style
// starts contend on the same worktree lock despite using independent private
// RunRoots: exactly one launches, and the loser is refused worktree-busy while
// the winner's supervisor holds the lock.
func TestTwoScopelessStartsOneWorktreeOneLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	reqA := sampleStart()
	reqA.RunRoot = filepath.Join(testsupport.TempDir(t), "runs-a")
	reqB := sampleStart()
	reqB.ChangeID = "0343"
	reqB.RunRoot = filepath.Join(testsupport.TempDir(t), "runs-b")
	reqs := []StartRequest{reqA, reqB}

	proc := &countingProc{}
	t.Cleanup(proc.releaseRetained)
	var barrier sync.WaitGroup
	barrier.Add(2)
	git := &barrierGit{wg: &barrier, head: "HEAD1"}
	mkDriver := func() *Driver {
		clk := &fakeClock{now: startRun()}
		d := NewDriver(store, clk, proc, git)
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		return d
	}
	drivers := []*Driver{mkDriver(), mkDriver()}

	docs := make([]DriveDoc, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range drivers {
		go func(i int) {
			defer wg.Done()
			docs[i], errs[i] = drivers[i].Start(reqs[i])
		}(i)
	}
	wg.Wait()

	if got := proc.launches(); got != 1 {
		t.Fatalf("two scopeless starts on one worktree must launch once, got %d", got)
	}
	assertOneWorktreeStartWinner(t, docs, errs)
}

func assertOneWorktreeStartWinner(t *testing.T, docs []DriveDoc, errs []error) {
	t.Helper()
	winners := 0
	for i, err := range errs {
		if err == nil {
			winners++
			if docs[i].Outcome != WAITING {
				t.Fatalf("winner %d must WAIT, got %s (%s)", i, docs[i].Outcome, docs[i].Cause)
			}
			continue
		}
		if !isOwnershipKind(err, ErrWorktreeBusy) {
			t.Fatalf("loser %d must fail ErrWorktreeBusy, got %v", i, err)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one start must win, got %d", winners)
	}
}

// TestDistinctWorktreesProgressConcurrently proves separate worktrees do NOT contend:
// two starts on two distinct worktrees both launch concurrently over one shared store.
// Run under -race.
func TestDistinctWorktreesProgressConcurrently(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	wt1 := testsupport.TempDir(t)
	wt2 := testsupport.TempDir(t)
	reqA := sampleStart()
	reqA.Worktree, reqA.Cwd = wt1, wt1
	reqB := sampleStart()
	reqB.Worktree, reqB.Cwd = wt2, wt2
	reqB.ChangeID = "0343"
	reqs := []StartRequest{reqA, reqB}

	proc := &countingProc{}
	t.Cleanup(proc.releaseRetained)
	mkDriver := func() *Driver {
		clk := &fakeClock{now: startRun()}
		d := NewDriver(store, clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		return d
	}
	drivers := []*Driver{mkDriver(), mkDriver()}

	docs := make([]DriveDoc, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range drivers {
		go func(i int) {
			defer wg.Done()
			docs[i], errs[i] = drivers[i].Start(reqs[i])
		}(i)
	}
	wg.Wait()

	if got := proc.launches(); got != 2 {
		t.Fatalf("distinct worktrees must both launch, got %d launches", got)
	}
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("start %d on a distinct worktree must succeed, got %v", i, errs[i])
		}
		if docs[i].Outcome != WAITING {
			t.Fatalf("start %d must WAIT, got %s (%s)", i, docs[i].Outcome, docs[i].Cause)
		}
	}
}

// ---------------------------------------------------------------------------
// Deterministic cancel/launch race barriers (change 0437 Task 7, AC2+AC6). The
// census is driven directly: "cancel" = run ReconcileRunLaunches over the run's
// context hash at a chosen point in the launch. There is no run gate to fence —
// change 0491 deleted the run launch check — so what keeps a cancellation honest
// is the census itself: a held per-drive claim is pending work, a proven
// never-launched launch is settled terminal, and a launched run is stopped
// before the census accounts. Every ordering assertion below is a
// channel/done-ordering fact — no timing sleep is an oracle anywhere.
// ---------------------------------------------------------------------------

// TestBarrierCancelBetweenAdmitAndStartAdmitted proves a census that runs after
// Admit's return refuses the delayed launch: reconcile resolves the reserved first
// launch through its launch token, proves it never launched, and settles the drive
// HALTED run-cancelled under the held claim (change 0490); the delayed
// StartAdmitted then refuses over the settled record, and a replay still
// accounts. No process is ever launched.
func TestBarrierCancelBetweenAdmitAndStartAdmitted(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())

	req := sampleStart()
	req.RunContext = "ctx-e1"
	// An existing run root, so the census resolves the launch token rather than
	// reading a missing root as never launched (change 0491).
	req.RunRoot = testsupport.TempDir(t)
	ticket, err := d.Admit(req) // the reservation is durable
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	// The cancellation lands AFTER Admit's return (the window between the two
	// phases). The durable reservation already exists.
	// Reconcile proves the reserved first launch never launched (the fake resolves
	// never-launched) and settles it terminal before the delayed launch runs.
	first, err := d.ReconcileRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches (pre-refusal): %v", err)
	}
	if !first.Accounted {
		t.Fatalf("a proven never-launched first launch must be accounted, got %+v", first)
	}
	rec, lerr := store.Load(ticket.id)
	if lerr != nil {
		t.Fatalf("Load: %v", lerr)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "run-cancelled" {
		t.Fatalf("reconcile must settle the drive HALTED run-cancelled, got %v/%q", rec.LastOutcome, rec.LastCause)
	}

	// StartAdmitted observes the settled record: it refuses and launches nothing.
	_, serr := d.StartAdmitted(ticket)
	if oe, ok := AsOwnershipError(serr); !ok || oe.Kind != ErrUnresolvedLaunchTransition {
		t.Fatalf("StartAdmitted over a settled record must refuse ErrUnresolvedLaunchTransition, got %v", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("a refused StartAdmitted must launch nothing, proc.Launch called %d times", proc.launchN)
	}
	if rec, lerr = store.Load(ticket.id); lerr != nil || rec.LastOutcome != HALTED || rec.LastCause != "run-cancelled" {
		t.Fatalf("the drive must stay HALTED run-cancelled, got %v/%q (err=%v)", rec.LastOutcome, rec.LastCause, lerr)
	}

	// A replay still accounts the obligation (the record is terminal).
	settled, err := d.ReconcileRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches (post-refusal): %v", err)
	}
	if !settled.Accounted {
		t.Fatalf("a replay after the refusal settled the record must account, got %+v", settled)
	}
}

// TestRunBackedDeathHaltsAndCensusAccountsOneLaunch (change 0493): a drive
// started inside a run whose supervisor dies halts supervisor-died with its one
// launch — no replacement exists for a cancellation to race — and run.cancel's
// census then accounts it from that one recorded run dir, stopping nothing.
func TestRunBackedDeathHaltsAndCensusAccountsOneLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	clk := &fakeClock{now: startRun()}
	dead := false
	runsRoot := testsupport.TempDir(t) // the census probes run dirs that exist on disk
	var launchCount int32
	proc := &fakeProc{observe: func(runDir string) (*process.Observation, error) {
		if dead {
			return obs(process.StateSignaled, runDir), nil
		}
		return obs(process.StateRunning, runDir), nil
	}}
	proc.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		n := atomic.AddInt32(&launchCount, 1)
		id := fmt.Sprintf("run%d", n)
		runDir := filepath.Join(runsRoot, id)
		if err := os.MkdirAll(runDir, 0o700); err != nil {
			return nil, err
		}
		return &process.LaunchOutcome{RunID: id, RunDir: runDir, State: process.StateRunning}, nil
	}
	d := storeTestDriver(store, clk, proc, stableGit())

	req := sampleStart()
	req.RunContext = "ctx-e1"
	started, serr := d.Start(req)
	if serr != nil || started.Outcome != WAITING {
		t.Fatalf("run-backed first slice must WAIT, got %+v (err=%v)", started, serr)
	}
	dead = true
	doc, err := d.Advance(started.DriveID, started.Generation)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc.Outcome != HALTED || doc.Cause != CauseSupervisorDied {
		t.Fatalf("a run-backed death = %s/%q, want HALTED/%s", doc.Outcome, doc.Cause, CauseSupervisorDied)
	}
	if got := atomic.LoadInt32(&launchCount); got != 1 {
		t.Fatalf("a death must never launch again, launches=%d", got)
	}

	sup := newSupervisors()
	run1 := filepath.Join(runsRoot, "run1")
	sup.state[run1] = process.StateSignaled
	dr := storeTestDriver(reopenStore(store), &fakeClock{now: startRun()}, sup.proc(), stableGit())
	report, err := dr.ReconcileRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted || !findingFor(report.Findings, "run-terminal", started.DriveID) {
		t.Fatalf("the halted drive's one dead run must account run-terminal, got %+v", report)
	}
	if len(sup.stopped) != 0 || len(sup.observed) != 1 || sup.observed[0] != run1 {
		t.Fatalf("the census must observe only %s and stop nothing: observed %v stopped %v", run1, sup.observed, sup.stopped)
	}
}

// TestBarrierCancelBetweenLaunchAndAttach proves a cancellation during a
// scopeless StartAdmitted's launch-to-attach window (the process exists, the
// claim held across launch+attach) reports claim-busy pending, then a replay
// accounts once the run is attached and stopped.
func TestBarrierCancelBetweenLaunchAndAttach(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	entered := make(chan struct{})
	release := make(chan struct{})
	proc := &fakeProc{}
	run1 := filepath.Join(testsupport.TempDir(t), "run1")
	if err := os.MkdirAll(run1, 0o700); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	// proc.Launch records the run then parks BEFORE returning to the driver — the
	// process is created but attach has not run, and the claim is held across both.
	proc.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		close(entered)
		<-release
		return &process.LaunchOutcome{RunID: "run1", RunDir: run1, State: process.StateRunning}, nil
	}
	d, store := newTestDriver(t, clk, proc, stableGit())

	req := sampleStart()
	req.RunContext = "ctx-e1"
	ticket, err := d.Admit(req)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	startDone := make(chan struct {
		doc DriveDoc
		err error
	}, 1)
	go func() {
		doc, serr := d.StartAdmitted(ticket)
		startDone <- struct {
			doc DriveDoc
			err error
		}{doc, serr}
	}()

	<-entered // launch in flight: the process exists, attach pending, claim held

	// The cancellation lands during the launch-to-attach window.
	recDone := make(chan RunLaunchReport, 1)
	go func() {
		dr := storeTestDriver(reopenStore(store), &fakeClock{now: startRun()}, &fakeProc{}, stableGit())
		r, _ := dr.ReconcileRunLaunches(capHash(req.RunContext))
		recDone <- r
	}()
	select {
	case report := <-recDone:
		if report.Accounted {
			t.Fatalf("a launch in flight (held claim) must not be accounted, got %+v", report)
		}
		if !reconcileFindingPresent(report.Findings, "claim-busy:"+ticket.id) {
			t.Fatalf("findings = %v, want claim-busy:%s", report.Findings, ticket.id)
		}
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("reconcile blocked on a busy claim; it must probe nonblocking and return promptly")
	}

	// Release: the run attaches, the claim frees, StartAdmitted returns WAITING.
	close(release)
	res := <-startDone
	if res.err != nil {
		t.Fatalf("StartAdmitted: %v", res.err)
	}
	if res.doc.Outcome != WAITING {
		t.Fatalf("a healthy attached run WAITs, got %s/%s", res.doc.Outcome, res.doc.Cause)
	}

	// A replay stops the attached run (its supervisor is running), then accounts.
	sup := newSupervisors()
	sup.state[run1] = process.StateRunning
	dr := storeTestDriver(reopenStore(store), &fakeClock{now: startRun()}, sup.proc(), stableGit())
	replay, err := dr.ReconcileRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches (replay): %v", err)
	}
	if !replay.Accounted {
		t.Fatalf("a replay after attach+stop must account, got %+v", replay)
	}
	if len(sup.stopped) != 1 || sup.stopped[0] != run1 {
		t.Fatalf("the replay must stop the attached run %s, stopped %v", run1, sup.stopped)
	}
}

// TestSameWorktreeRaceAcrossOwnersRawAndAliasOneWinner (change 0446 spec AC7,
// re-targeted onto the worktree lock by change 0490) extends the pairwise
// one-worktree races above to the full contender set at once: two starts for
// different changes, a start whose launch cwd is a SYMLINK ALIAS of the
// worktree, and a raw launcher taking the worktree lock directly all rendezvous
// past their unlocked pre-checks and contend for the one lock. Exactly one wins
// (at most one backend launch; the raw winner launches none), every loser is
// refused worktree-busy, and the winner still holds the lock through either
// spelling. Several rounds widen the interleavings; run under -race.
func TestSameWorktreeRaceAcrossOwnersRawAndAliasOneWinner(t *testing.T) {
	for round := 0; round < 8; round++ {
		t.Run(fmt.Sprintf("round-%d", round), func(t *testing.T) {
			store := OpenStore(testsupport.TempDir(t))
			wt := mkWorktree(t)
			alias := filepath.Join(testsupport.TempDir(t), "alias")
			if err := os.Symlink(wt, alias); err != nil {
				t.Fatal(err)
			}
			first := sampleStart()
			first.Worktree, first.Cwd = wt, wt
			second := sampleStart()
			second.Worktree, second.Cwd = wt, wt
			second.ChangeID = "0343"
			viaAlias := sampleStart()
			viaAlias.Worktree, viaAlias.Cwd = alias, alias
			viaAlias.ChangeID = "0344"
			reqs := []StartRequest{first, second, viaAlias}

			proc := &countingProc{}
			t.Cleanup(proc.releaseRetained)
			var barrier sync.WaitGroup
			barrier.Add(len(reqs) + 1)
			git := &barrierGit{wg: &barrier, head: "HEAD1"}

			errs := make([]error, len(reqs)+1)
			var rawLock *WorktreeLock
			var wg sync.WaitGroup
			for i := range reqs {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					clk := &fakeClock{now: startRun()}
					_, errs[i] = storeTestDriver(store, clk, proc, git).Start(reqs[i])
				}(i)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				barrier.Done()
				barrier.Wait()
				rawLock, errs[len(reqs)] = store.TryWorktreeLock(fakeWorktreeRoot(wt), nil)
			}()
			wg.Wait()
			t.Cleanup(rawLock.Release)

			winners := 0
			for i, err := range errs {
				if err == nil {
					winners++
					continue
				}
				if !isOwnershipKind(err, ErrWorktreeBusy) {
					t.Fatalf("contender %d must lose worktree-busy, got %v", i, err)
				}
			}
			if winners != 1 {
				t.Fatalf("exactly one contender may win the worktree, got %d (errs=%v)", winners, errs)
			}
			rawWon := errs[len(reqs)] == nil
			wantLaunches := 1
			if rawWon {
				wantLaunches = 0
			}
			if got := proc.launches(); got != wantLaunches {
				t.Fatalf("launches = %d, want %d (raw won: %v)", got, wantLaunches, rawWon)
			}
			for _, spelling := range []string{wt, alias} {
				if worktreeFree(t, store, spelling) {
					t.Fatalf("the winner must still hold the worktree through %q", spelling)
				}
			}
		})
	}
}
