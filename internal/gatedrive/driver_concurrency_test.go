package gatedrive

import (
	"errors"
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

// racingProc is a thread-safe ProcessSeam purpose-built to reproduce the
// concurrent-relaunch race deterministically. Its first dead-run observation
// rendezvouses two concurrent advances before either may reserve the relaunch.
// The reservation winner is then the only caller permitted to reach Launch.
// All bookkeeping is mutex/atomic-guarded so the test is clean under -race.
type racingProc struct {
	barrier     *sync.WaitGroup // trips once both advances observed the dead run
	newRunState process.State   // the state a relaunched run reports

	mu        sync.Mutex
	launchSeq int
	deathObs  int
	launched  map[string]bool // relaunch run dirs handed out
	stops     []string        // every runDir passed to Stop, in call order
}

func newRacingProc(newRunState process.State) *racingProc {
	var b sync.WaitGroup
	b.Add(2)
	return &racingProc{
		barrier:     &b,
		newRunState: newRunState,
		launched:    map[string]bool{},
	}
}

func (p *racingProc) Launch(req process.LaunchRequest) (*process.LaunchOutcome, error) {
	defer releaseHandedLock(req)
	p.mu.Lock()
	p.launchSeq++
	id := fmt.Sprintf("relaunch%d", p.launchSeq)
	dir := "/runs/" + id
	p.launched[dir] = true
	p.mu.Unlock()

	return &process.LaunchOutcome{RunID: id, RunDir: dir, State: process.StateRunning}, nil
}

func (p *racingProc) Observe(runDir string) (*process.Observation, error) {
	// The seeded original run is dead (signaled); every relaunched run reports
	// the scripted new-run state.
	if strings.HasSuffix(runDir, "run1") {
		// Both advances must see the vanished original before either can reserve
		// a replacement. This creates the exact pre-reservation race without
		// forcing the loser to call Launch.
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
	return &process.Observation{State: p.newRunState, RunDir: runDir}, nil
}

func (p *racingProc) Stop(runDir, reason string) (*process.StopOutcome, error) {
	p.mu.Lock()
	p.stops = append(p.stops, runDir)
	p.mu.Unlock()

	if strings.HasSuffix(runDir, "run1") {
		// A signaled run is already terminal: an ownership-proven no-op.
		return &process.StopOutcome{State: process.StateSignaled, RunDir: runDir, Performed: false,
			Terminal: &process.Terminal{Kind: "signal", Signal: 9}}, nil
	}
	return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
}

func (p *racingProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	return &process.ReservationResolution{Disposition: "never-launched"}, nil
}

// relaunchStopCount reports how many of the runs THIS proc launched were later
// passed to Stop — i.e. orphan cleanups, as distinct from the death-probe stops
// of the original run.
func (p *racingProc) relaunchStopCount() (n int, stopped map[string]bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	stopped = map[string]bool{}
	for _, s := range p.stops {
		if p.launched[s] {
			stopped[s] = true
			n++
		}
	}
	return n, stopped
}

// liveRelaunchDirs returns the relaunch run dirs this proc launched that were
// never stopped — the still-live owned trees.
func (p *racingProc) liveRelaunchDirs(stopped map[string]bool) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var live []string
	for dir := range p.launched {
		if !stopped[dir] {
			live = append(live, dir)
		}
	}
	return live
}

// TestConcurrentSameOwnerAdvanceRelaunchesOnce proves the single relaunch is
// decided atomically under a relaunch reservation: two concurrent Advance calls that
// present the SAME valid owner generation over a nonterminal record whose child
// has died must together yield EXACTLY ONE relaunch (RelaunchCount==1) and
// exactly one live owned tree. The losing advance reloads the authoritative
// drive state without issuing a backend launch.
func TestConcurrentSameOwnerAdvanceRelaunchesOnce(t *testing.T) {
	cases := []struct {
		name        string
		newRunState process.State
		wantOutcome Outcome
	}{
		{name: "healthy relaunch winner waits", newRunState: process.StateRunning, wantOutcome: WAITING},
		{name: "terminal relaunch winner fails", newRunState: process.StateFailed, wantOutcome: FAILED},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := OpenStore(testsupport.TempDir(t))
			id, ownerGen := seedDrive(t, store, seedRecord(t))

			proc := newRacingProc(tc.newRunState)

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

			for i, e := range errs {
				if e != nil {
					t.Fatalf("advance %d returned an error: %v", i, e)
				}
			}

			if proc.launchSeq != 1 {
				t.Fatalf("the relaunch reservation must allow exactly one backend launch, got %d", proc.launchSeq)
			}

			rec, err := store.Load(id)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if rec.RelaunchCount != 1 {
				t.Fatalf("two concurrent same-owner advances must yield EXACTLY ONE relaunch, got RelaunchCount=%d", rec.RelaunchCount)
			}
			if rec.Attempt != 2 {
				t.Fatalf("exactly one relaunch advances the attempt to 2, got %d", rec.Attempt)
			}
			if rec.LastOutcome != tc.wantOutcome {
				t.Fatalf("settled outcome = %s (%s), want %s", rec.LastOutcome, rec.LastCause, tc.wantOutcome)
			}

			// The losing advance never launches an orphan. The sole launched
			// replacement remains the drive's owned run unless its own terminal
			// outcome already ended it.
			relaunchStops, stopped := proc.relaunchStopCount()
			if relaunchStops != 0 {
				t.Fatalf("a reservation loser must not create an orphan to stop, stopped %d relaunch runs", relaunchStops)
			}
			live := proc.liveRelaunchDirs(stopped)
			if len(live) != 1 {
				t.Fatalf("exactly one live owned relaunch tree must survive, got %d: %v", len(live), live)
			}
			if rec.RawRunDir != live[0] {
				t.Fatalf("the drive must own the surviving relaunch tree, RawRunDir=%q live=%q", rec.RawRunDir, live[0])
			}
		})
	}
}

// terminalSettleProc is the shared ProcessSeam core for the deterministic
// loser-after-terminal-settle regression: the seeded original run observes as
// signaled (dead), and every relaunched run reports StateFailed so the winner
// settles the drive terminally within its single Advance.
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

// gatedLoserSeam wraps the shared core for the LOSER driver only: its first
// dead-run observation signals `observing` (proving the loser loaded a
// nonterminal record and entered its slice) and then parks on `gate` until the
// test releases it — after the winner's Advance has fully returned with the
// terminal outcome durably persisted. This forces, deterministically, the
// interleaving where the loser's reserveRelaunch CAS finds a terminal record.
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

// TestLoserAfterTerminalSettleReturnsRecordedState pins the deterministic
// resolution of the 0411 interleaving: a same-owner advance that loses the
// relaunch race only AFTER the winner has persisted the replacement's terminal
// outcome must return the authoritative recorded state — never the raw
// errAlreadyTerminal sentinel as an Advance error. Exactly one launch, one
// relaunch, one attempt increment, no orphan.
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

	// The winner runs to completion: dead original observed, relaunch reserved
	// and launched, replacement observed StateFailed, FAILED persisted.
	winnerDoc, winnerErr := mkDriver(core).Advance(id, ownerGen)
	if winnerErr != nil {
		t.Fatalf("winner advance: %v", winnerErr)
	}
	if winnerDoc.Outcome != FAILED {
		t.Fatalf("winner outcome = %s, want %s", winnerDoc.Outcome, FAILED)
	}

	// Only now may the loser proceed to its reservation attempt.
	close(gate)
	loser := <-loserDone
	if loser.err != nil {
		t.Fatalf("the reservation loser must return recorded state, not an error: %v", loser.err)
	}
	if loser.doc.Outcome != FAILED {
		t.Fatalf("loser doc outcome = %s (%s), want %s", loser.doc.Outcome, loser.doc.Cause, FAILED)
	}
	if loser.doc.Cause != winnerDoc.Cause {
		t.Fatalf("loser doc cause = %q, want the winner's recorded cause %q", loser.doc.Cause, winnerDoc.Cause)
	}

	if core.launchSeq != 1 {
		t.Fatalf("exactly one backend launch, got %d", core.launchSeq)
	}
	rec, err := store.Load(id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.RelaunchCount != 1 {
		t.Fatalf("RelaunchCount = %d, want 1", rec.RelaunchCount)
	}
	if rec.Attempt != 2 {
		t.Fatalf("Attempt = %d, want 2", rec.Attempt)
	}
	if rec.LastOutcome != FAILED {
		t.Fatalf("recorded outcome = %s (%s), want %s", rec.LastOutcome, rec.LastCause, FAILED)
	}
	if !rec.Deadline.Equal(seeded.Deadline) {
		t.Fatalf("the original deadline must be preserved: got %v, seeded %v", rec.Deadline, seeded.Deadline)
	}
	core.mu.Lock()
	var relaunchStops int
	for _, s := range core.stops {
		if core.launched[s] {
			relaunchStops++
		}
	}
	core.mu.Unlock()
	if relaunchStops != 0 {
		t.Fatalf("the loser must not create or stop an orphan, stopped %d relaunch runs", relaunchStops)
	}
}

// claimWindowProc holds the reservation winner inside Launch so a second
// same-owner Advance deterministically enters the reserve-to-launch window. A
// correct claimant fence makes the second caller return authoritative state
// without resolving the live holder's token or issuing another launch.
type claimWindowProc struct {
	launchEntered chan struct{}
	releaseLaunch chan struct{}

	mu       sync.Mutex
	launchN  int
	resolveN int
}

func newClaimWindowProc() *claimWindowProc {
	return &claimWindowProc{
		launchEntered: make(chan struct{}),
		releaseLaunch: make(chan struct{}),
	}
}

func (p *claimWindowProc) Launch(req process.LaunchRequest) (*process.LaunchOutcome, error) {
	defer releaseHandedLock(req)
	p.mu.Lock()
	p.launchN++
	n := p.launchN
	p.mu.Unlock()
	if n == 1 {
		close(p.launchEntered)
		<-p.releaseLaunch
	}
	return &process.LaunchOutcome{
		RunID:  fmt.Sprintf("relaunch%d", n),
		RunDir: fmt.Sprintf("/runs/relaunch%d", n),
		State:  process.StateRunning,
	}, nil
}

func (p *claimWindowProc) Observe(runDir string) (*process.Observation, error) {
	if strings.HasSuffix(runDir, "run1") {
		return &process.Observation{State: process.StateVanished, RunDir: runDir}, nil
	}
	return &process.Observation{State: process.StateRunning, RunDir: runDir}, nil
}

func (p *claimWindowProc) Stop(runDir, reason string) (*process.StopOutcome, error) {
	return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
}

func (p *claimWindowProc) ResolveReservation(root, token string) (*process.ReservationResolution, error) {
	p.mu.Lock()
	p.resolveN++
	p.mu.Unlock()
	return &process.ReservationResolution{Disposition: "never-launched"}, nil
}

func (p *claimWindowProc) counts() (launches, resolutions int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.launchN, p.resolveN
}

func TestRelaunchReservationHolderCannotBeStolenBeforeLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	wt := mkWorktree(t)
	rec := seedRecord(t)
	rec.WorktreePath = wt
	id, ownerGen := seedDrive(t, store, rec)
	proc := newClaimWindowProc()

	mkDriver := func() *Driver {
		clk := &fakeClock{now: startRun().Add(time.Second)}
		// Separate Store values model independent CLI processes; ownership is
		// carried by the persisted record and kernel flock, not Go memory.
		d := NewDriver(reopenStore(store), clk, proc, stableGit())
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		return d
	}

	type advanceResult struct {
		doc DriveDoc
		err error
	}
	winnerResult := make(chan advanceResult, 1)
	go func() {
		doc, err := mkDriver().Advance(id, ownerGen)
		winnerResult <- advanceResult{doc: doc, err: err}
	}()

	select {
	case <-proc.launchEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("reservation winner did not enter Launch")
	}

	loserResult := make(chan advanceResult, 1)
	go func() {
		doc, err := mkDriver().Advance(id, ownerGen)
		loserResult <- advanceResult{doc: doc, err: err}
	}()

	var loser advanceResult
	select {
	case loser = <-loserResult:
	case <-time.After(2 * time.Second):
		close(proc.releaseLaunch)
		t.Fatal("competing Advance did not return while the reservation holder was launching")
	}
	close(proc.releaseLaunch)

	var winner advanceResult
	select {
	case winner = <-winnerResult:
	case <-time.After(2 * time.Second):
		t.Fatal("reservation winner did not finish after Launch was released")
	}
	if winner.err != nil || loser.err != nil {
		t.Fatalf("Advance errors: winner=%v loser=%v", winner.err, loser.err)
	}
	launches, resolutions := proc.counts()
	if launches != 1 {
		t.Fatalf("reserve-to-launch window admitted %d backend launches, want exactly 1", launches)
	}
	if resolutions != 0 {
		t.Fatalf("live reservation holder was treated as crash recovery %d times", resolutions)
	}
	recorded, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if recorded.RelaunchCount != 1 || recorded.Attempt != 2 {
		t.Fatalf("recorded relaunch/attempt = %d/%d, want 1/2", recorded.RelaunchCount, recorded.Attempt)
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
// Deterministic cancel/launch race barriers (change 0437 Task 7, AC2+AC6). A
// fake RunLaunchGate backed by a mutable, mutex-guarded registry stands in for
// the app gate: it reads the run's liveness under the registry mutex and, while
// STILL holding it, runs the driver's durable reserve body — exactly as the
// production gate runs reserve under the held run lock. A concurrent fence
// ("cancel") takes the SAME mutex, so it either lands before the liveness read
// (reserve never runs) or after the gate released the lock (it observes the
// durable reservation reserve produced). "cancel" = flip the fake to fenced, then
// run ReconcileRunLaunches over the run's context hash (which consults NO gate;
// the run is already fenced, so cancellation may never hold the run while probing
// a per-drive claim). Every ordering assertion below is a channel/done-ordering
// fact — no timing sleep is an oracle anywhere.
// ---------------------------------------------------------------------------

// errRunFenced is the sentinel a fenced fakeRunRegistry gate refuses with,
// standing in for the app's ErrRunCancelled/ErrStaleRunID fence tokens.
var errRunFenced = errors.New("gatedrive-test: run fenced (cancelled)")

// fakeRunRegistry is a mutable, mutex-guarded stand-in for the app's run
// registry. The RunLaunchGate it produces holds the registry mutex across the
// liveness read AND the reserve body — modelling the production run lock held
// across reserve — so a concurrent fence serializes against it: the fence lands
// strictly before the read (reserve never runs) or strictly after reserve's
// durable decision. A fenced run's gate refuses errRunFenced WITHOUT running
// reserve (the RunLaunchGate contract: a validation failure never calls reserve).
type fakeRunRegistry struct {
	mu     sync.Mutex
	fenced map[string]bool
}

// fence flips runID to fenced. It takes the same mutex the gate body holds, so
// it can only land in the serialization windows the gate leaves open.
func (r *fakeRunRegistry) fence(runID string) {
	r.mu.Lock()
	if r.fenced == nil {
		r.fenced = map[string]bool{}
	}
	r.fenced[runID] = true
	r.mu.Unlock()
}

func (r *fakeRunRegistry) gate() RunLaunchGate {
	return func(runID, _ string, reserve func() error) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.fenced[runID] {
			return errRunFenced // fenced: refuse without running reserve
		}
		return reserve()
	}
}

// TestBarrierCancelBeforeAdmit proves the fence-first outcome: a fence that lands
// before Admit's liveness read makes Admit refuse, reserving nothing durable, and
// a subsequent reconcile has nothing to account (Accounted, no findings).
func TestBarrierCancelBeforeAdmit(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	reg := &fakeRunRegistry{}
	d.SetRunLaunchGate(reg.gate())

	// The fence lands FIRST.
	reg.fence("e1")

	req := sampleStart()
	req.RunID = "e1"
	req.RunContext = "ctx-e1"
	ticket, err := d.Admit(req)
	if !errors.Is(err, errRunFenced) {
		t.Fatalf("a fence before Admit must refuse, got ticket=%v err=%v", ticket, err)
	}
	if ticket != nil {
		t.Fatalf("a refused admission returns no ticket, got %+v", ticket)
	}
	if !worktreeFree(t, store, req.Cwd) {
		t.Fatalf("a fenced Admit must leave the worktree lock free")
	}
	if n := driveRecordCount(t, store); n != 0 {
		t.Fatalf("a fenced Admit must mint no drive record, got %d", n)
	}
	if proc.launchN != 0 {
		t.Fatalf("a fenced Admit must launch nothing, proc.Launch called %d times", proc.launchN)
	}

	report, err := d.ReconcileRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches: %v", err)
	}
	if !report.Accounted || len(report.Findings) != 0 {
		t.Fatalf("a fence before any admission has nothing to account, got %+v", report)
	}
}

// TestBarrierCancelBetweenAdmitAndStartAdmitted proves the fence that lands after
// Admit's return refuses the delayed launch: reconcile resolves the reserved first
// launch through its launch token, proves it never launched, and settles the drive
// HALTED run-cancelled under the held claim (change 0490); the delayed
// StartAdmitted then refuses, and a replay still accounts. No process is ever
// launched.
func TestBarrierCancelBetweenAdmitAndStartAdmitted(t *testing.T) {
	clk := &fakeClock{now: startRun()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	reg := &fakeRunRegistry{}
	d.SetRunLaunchGate(reg.gate())

	req := sampleStart()
	req.RunID = "e1"
	req.RunContext = "ctx-e1"
	ticket, err := d.Admit(req) // run live: the reservation is durable
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	// The fence lands AFTER Admit's return (the serialization window between the
	// two phases). The durable reservation already exists.
	reg.fence("e1")

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

	// StartAdmitted observes the fence: it refuses and launches nothing.
	if _, serr := d.StartAdmitted(ticket); !errors.Is(serr, errRunFenced) {
		t.Fatalf("StartAdmitted under a fence must refuse, got %v", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("a fenced StartAdmitted must launch nothing, proc.Launch called %d times", proc.launchN)
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

// TestBarrierCancelBetweenAuthorizationAndLaunch proves the spec's second race
// outcome: a fence that lands AFTER a relaunch won its authorization (reserve
// committed, the per-drive claim held) but before proc.Launch cannot make
// cancellation complete while the launch is in flight — a concurrent reconcile
// reports claim-busy pending. The replacement process CAN be created after the
// fence, yet cancellation only completes once a replay identifies and stops it.
func TestBarrierCancelBetweenAuthorizationAndLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	reg := &fakeRunRegistry{}
	clk := &fakeClock{now: startRun()}

	dead := false
	launchEntered := make(chan struct{})
	releaseLaunch := make(chan struct{})
	var launchCount int32
	// The census proves supervisors gone over run dirs that exist on disk, so each
	// launch's run dir is a real directory.
	runsRoot := testsupport.TempDir(t)
	proc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			if dead && strings.HasSuffix(runDir, "run1") {
				return obs(process.StateSignaled, runDir), nil
			}
			return obs(process.StateRunning, runDir), nil
		},
	}
	proc.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		n := atomic.AddInt32(&launchCount, 1)
		id := fmt.Sprintf("run%d", n)
		runDir := filepath.Join(runsRoot, id)
		if err := os.MkdirAll(runDir, 0o700); err != nil {
			return nil, err
		}
		if n == 2 { // the replacement launch: reserve committed, the claim is HELD
			close(launchEntered)
			<-releaseLaunch
		}
		return &process.LaunchOutcome{RunID: id, RunDir: runDir, State: process.StateRunning}, nil
	}
	d := storeTestDriver(store, clk, proc, stableGit())
	d.SetRunLaunchGate(reg.gate())

	// A run-backed first start over live run e1 WAITs (run1 running).
	req := sampleStart()
	req.RunID = "e1"
	req.RunContext = "ctx-e1"
	started, serr := d.Start(req)
	if serr != nil || started.Outcome != WAITING {
		t.Fatalf("run-backed first slice must WAIT, got %+v (err=%v)", started, serr)
	}

	// The run dies; its single automatic relaunch is reserved under the drive's
	// claim, then parks in proc.Launch (reserve committed; the claim held).
	dead = true
	advance := make(chan struct {
		doc DriveDoc
		err error
	}, 1)
	go func() {
		doc, err := d.Advance(started.DriveID, started.Generation)
		advance <- struct {
			doc DriveDoc
			err error
		}{doc, err}
	}()

	<-launchEntered // the replacement launch is parked: reserve committed, claim held

	// The fence lands NOW — after the reservation, during the parked launch.
	reg.fence("e1")

	// A concurrent reconcile (an independent CLI process: its own store handle and
	// process seam) reports the held claim as pending work, and returns promptly.
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
		if !reconcileFindingPresent(report.Findings, "claim-busy:"+started.DriveID) {
			t.Fatalf("findings = %v, want claim-busy:%s", report.Findings, started.DriveID)
		}
	case <-time.After(5 * time.Second):
		close(releaseLaunch)
		t.Fatal("reconcile blocked on a busy claim; it must probe nonblocking and return promptly")
	}

	// Release the parked launch: the replacement attaches, the claim frees.
	close(releaseLaunch)
	res := <-advance
	if res.err != nil {
		t.Fatalf("Advance: %v", res.err)
	}
	if res.doc.Outcome != WAITING {
		t.Fatalf("an authorized relaunch's healthy new run must WAIT, got %s/%s", res.doc.Outcome, res.doc.Cause)
	}
	if got := atomic.LoadInt32(&launchCount); got != 2 {
		t.Fatalf("the replacement process must have been created after the fence, launches=%d", got)
	}

	// A replay now stops the attached replacement (its supervisor is running), and
	// only THEN accounts.
	sup := newSupervisors()
	replacement := filepath.Join(runsRoot, "run2")
	sup.state[replacement] = process.StateRunning
	sup.state[filepath.Join(runsRoot, "run1")] = process.StateSignaled
	dr := storeTestDriver(reopenStore(store), &fakeClock{now: startRun()}, sup.proc(), stableGit())
	replay, err := dr.ReconcileRunLaunches(capHash(req.RunContext))
	if err != nil {
		t.Fatalf("ReconcileRunLaunches (replay): %v", err)
	}
	if !replay.Accounted {
		t.Fatalf("cancellation completes only once the replacement is stopped, got %+v", replay)
	}
	if len(sup.stopped) != 1 || sup.stopped[0] != replacement {
		t.Fatalf("the replay must stop exactly the replacement %s, stopped %v", replacement, sup.stopped)
	}
}

// TestBarrierCancelBetweenLaunchAndAttach proves a fence during a scopeless
// StartAdmitted's launch-to-attach window (the process exists, the claim held
// across launch+attach) reports claim-busy pending, then a replay accounts once
// the run is attached and stopped — and, crucially, that once a replay reports
// accounted, a subsequent start on the fenced run refuses and launches nothing.
func TestBarrierCancelBetweenLaunchAndAttach(t *testing.T) {
	reg := &fakeRunRegistry{}
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
	d.SetRunLaunchGate(reg.gate())

	req := sampleStart()
	req.RunID = "e1"
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

	// The fence lands during the launch-to-attach window.
	reg.fence("e1")

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

	// No launch after an accounted reconcile: a subsequent start on the fenced run
	// refuses and proc.Launch's call count is final.
	launchesBefore := proc.launchN
	next := sampleStart()
	next.RunID = "e1"
	if _, nerr := d.Start(next); !errors.Is(nerr, errRunFenced) {
		t.Fatalf("a start on the fenced run must refuse, got %v", nerr)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("no launch may occur after an accounted reconcile, launches %d->%d", launchesBefore, proc.launchN)
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
