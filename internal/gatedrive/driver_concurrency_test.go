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

func (p *racingProc) Launch(process.LaunchRequest) (*process.LaunchOutcome, error) {
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

func (p *racingProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return process.RecoveryEntry{Disposition: "invalid"}, nil
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
				clk := &fakeClock{now: startEpoch().Add(time.Second)}
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

func (p *terminalSettleProc) Launch(process.LaunchRequest) (*process.LaunchOutcome, error) {
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

func (p *terminalSettleProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return process.RecoveryEntry{Disposition: "invalid"}, nil
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

func (s *gatedLoserSeam) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return s.core.ClassifyRun(runDir, mark)
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
		clk := &fakeClock{now: startEpoch().Add(time.Second)}
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

func (p *claimWindowProc) Launch(process.LaunchRequest) (*process.LaunchOutcome, error) {
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

func (p *claimWindowProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return process.RecoveryEntry{Disposition: "invalid"}, nil
}

func (p *claimWindowProc) counts() (launches, resolutions int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.launchN, p.resolveN
}

func TestRelaunchReservationHolderCannotBeStolenBeforeLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	// A live worktree slot backs the admission token (an epoch-less slot, as a
	// real scopeless drive holds one), so the epoch-linkage resolution admits the
	// relaunch through the standalone path (change 0437 Task 3).
	wt := mkWorktree(t)
	token, terr := store.ReserveWorktreeExecution(sampleAdmission(wt))
	if terr != nil {
		t.Fatalf("reserve admission: %v", terr)
	}
	rec := seedRecord(t)
	rec.WorktreePath = wt
	rec.AdmissionToken = token
	id, ownerGen := seedDrive(t, store, rec)
	proc := newClaimWindowProc()

	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch().Add(time.Second)}
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
// makes) on a shared 2-party barrier, so two concurrent Starts both pass their
// unlocked pre-check AND compute their fingerprint BEFORE either reserves the
// scope slot — maximizing the reservation race window. The remaining reads are
// fixed strings so the fingerprint is otherwise deterministic.
type barrierGit struct {
	wg   *sync.WaitGroup
	head string
}

func (g *barrierGit) HeadOID(string) (string, error) {
	g.wg.Done()
	g.wg.Wait()
	return g.head, nil
}
func (g *barrierGit) IndexEntries(string) ([]byte, error)  { return []byte("IDX1"), nil }
func (g *barrierGit) Status(string) ([]byte, error)        { return []byte("ST1"), nil }
func (g *barrierGit) WorktreePaths(string) ([]byte, error) { return nil, nil }

// countingProc is a minimal thread-safe ProcessSeam that counts launches; every
// launched run stays running so a winning Start reaches WAITING. It is purpose-
// built for the empty-scope reservation race, where at most one Start launches.
type countingProc struct {
	mu      sync.Mutex
	launchN int
}

func (p *countingProc) Launch(process.LaunchRequest) (*process.LaunchOutcome, error) {
	p.mu.Lock()
	p.launchN++
	n := p.launchN
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

func (p *countingProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return process.RecoveryEntry{Disposition: "invalid"}, nil
}

func (p *countingProc) launches() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.launchN
}

// TestDriverConcurrencyScopedStartReservationRace proves the durable pre-launch
// reservation makes an empty-scope start race admit EXACTLY ONE launch: two
// goroutines Start the same empty scope, rendezvous at the fingerprint barrier
// (both past their pre-check), then contend at reserveScopeDrive — exactly one
// wins and launches, and the loser is refused with a typed ErrScopeBusy /
// ErrScopeSecondDrive and never launches. Run under -race.
func TestDriverConcurrencyScopedStartReservationRace(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability

	proc := &countingProc{}
	var barrier sync.WaitGroup
	barrier.Add(2)
	git := &barrierGit{wg: &barrier, head: "HEAD1"}

	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch()}
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
			docs[i], errs[i] = drivers[i].Start(req)
		}(i)
	}
	wg.Wait()

	// Exactly one launch total — the reservation, not the launch, arbitrates.
	if got := proc.launches(); got != 1 {
		t.Fatalf("two concurrent empty-scope starts must admit EXACTLY ONE launch, got %d", got)
	}
	// Exactly one nil-error winner; the loser is a typed reservation rejection.
	winners := 0
	winIdx := -1
	for i, e := range errs {
		if e == nil {
			winners++
			winIdx = i
			continue
		}
		if !isOwnershipKind(e, ErrScopeBusy) && !isOwnershipKind(e, ErrScopeSecondDrive) {
			t.Fatalf("the losing start must fail ErrScopeBusy or ErrScopeSecondDrive, got %v", e)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one start must win, got %d", winners)
	}
	if docs[winIdx].Outcome != WAITING {
		t.Fatalf("the winning start must WAIT, got %s (%s)", docs[winIdx].Outcome, docs[winIdx].Cause)
	}
	// The persisted scope names exactly the sole winner's launched drive.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID != docs[winIdx].DriveID {
		t.Fatalf("the scope must name the sole winner's drive %q, got %q", docs[winIdx].DriveID, scope.CurrentDriveID)
	}
	if scope.CurrentDriveState != scopeStateLaunched {
		t.Fatalf("the winner's slot must be launched, got %q", scope.CurrentDriveState)
	}
}

// TestScopedStartConcurrentAcrossScopesOneLaunch proves the worktree execution slot
// admits EXACTLY ONE launch when two DIFFERENT scopes race a start on ONE worktree:
// two goroutines rendezvous at the fingerprint barrier (both past their unlocked
// pre-check), then contend at ReserveWorktreeExecution. Exactly one wins and launches;
// the loser is refused ErrWorktreeBusy and never launches. Run under -race.
func TestScopedStartConcurrentAcrossScopesOneLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	_, reqA := prepareScopedStartAt(t, store, sampleWorktree(), "0342")
	_, reqB := prepareScopedStartAt(t, store, sampleWorktree(), "0343")
	reqs := []StartRequest{reqA, reqB}

	proc := &countingProc{}
	var barrier sync.WaitGroup
	barrier.Add(2)
	git := &barrierGit{wg: &barrier, head: "HEAD1"}

	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch()}
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

	// Exactly one launch total — the worktree slot, not the launch, arbitrates.
	if got := proc.launches(); got != 1 {
		t.Fatalf("two scopes racing one worktree must admit EXACTLY ONE launch, got %d", got)
	}
	winners := 0
	winIdx := -1
	for i, e := range errs {
		if e == nil {
			winners++
			winIdx = i
			continue
		}
		if !isOwnershipKind(e, ErrWorktreeBusy) {
			t.Fatalf("the losing cross-scope start must fail ErrWorktreeBusy, got %v", e)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one start must win, got %d", winners)
	}
	if docs[winIdx].Outcome != WAITING {
		t.Fatalf("the winning start must WAIT, got %s (%s)", docs[winIdx].Outcome, docs[winIdx].Cause)
	}
}

// TestMixedScopedScopelessOneWorktreeOneLaunch proves the worktree slot is the
// outer admission authority even when only one contender has a recovery scope.
// Both callers pass the fingerprint barrier before either can reserve; exactly
// one may reach the backend Launch.
func TestMixedScopedScopelessOneWorktreeOneLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	_, scoped := prepareScopedStartAt(t, store, sampleWorktree(), "0342")
	scopeless := sampleStart()
	scopeless.Worktree = scoped.Worktree
	scopeless.ChangeID = "0343"
	reqs := []StartRequest{scoped, scopeless}

	proc := &countingProc{}
	var barrier sync.WaitGroup
	barrier.Add(2)
	git := &barrierGit{wg: &barrier, head: "HEAD1"}
	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch()}
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
		t.Fatalf("mixed scoped/scopeless starts on one worktree must launch once, got %d", got)
	}
	assertOneWorktreeStartWinner(t, docs, errs)
}

// TestTwoScopelessStartsOneWorktreeOneLaunch proves that two finalize-style
// starts contend on the same durable worktree slot despite using independent
// private RunRoots.
func TestTwoScopelessStartsOneWorktreeOneLaunch(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	reqA := sampleStart()
	reqB := sampleStart()
	reqB.ChangeID = "0343"
	reqs := []StartRequest{reqA, reqB}

	proc := &countingProc{}
	var barrier sync.WaitGroup
	barrier.Add(2)
	git := &barrierGit{wg: &barrier, head: "HEAD1"}
	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch()}
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
// two scopes on two distinct worktrees both launch concurrently over one shared store.
// Run under -race.
func TestDistinctWorktreesProgressConcurrently(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	wt1 := testsupport.TempDir(t)
	wt2 := testsupport.TempDir(t)
	_, reqA := prepareScopedStartAt(t, store, wt1, "0342")
	_, reqB := prepareScopedStartAt(t, store, wt2, "0343")
	reqs := []StartRequest{reqA, reqB}

	proc := &countingProc{}
	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch()}
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

// TestDriverConcurrencySuccessorStartRace reproduces spec verification 5 (successor
// half): two goroutines present the SAME valid predecessor receipt and rendezvous
// at the fingerprint barrier (both past their unlocked pre-check) before contending
// at reserveScopeDrive's scope CAS. Exactly one wins and launches its successor; the
// loser is refused with a typed ErrScopeBusy / ErrStalePredecessor and never
// launches; and the predecessor is retired exactly once (its owner cleared, its
// verdict intact). Run under -race.
func TestDriverConcurrencySuccessorStartRace(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	req := sampleStart()
	grant, err := store.PrepareScope(scopeReqFor(req, ""))
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability

	// Setup: drive the first drive to a durable PASSED with a plain passing seam.
	setupClk := &fakeClock{now: startEpoch()}
	setupProc := &fakeProc{
		observe: func(runDir string) (*process.Observation, error) {
			return &process.Observation{State: process.StatePassed, RunDir: runDir}, nil
		},
	}
	setupDriver := NewDriver(store, setupClk, setupProc, stableGit())
	setupDriver.slice = 4 * pollTick
	setupDriver.pollInterval = pollTick
	setupDriver.sleep = func(dur time.Duration) { setupClk.advance(dur) }
	first, err := setupDriver.Start(req)
	if err != nil {
		t.Fatalf("setup Start: %v", err)
	}
	if first.Outcome != PASSED {
		t.Fatalf("setup first drive must PASS, got %s (%s)", first.Outcome, first.Cause)
	}

	// Race: two successors present the same valid receipt. countingProc runs stay
	// live so the winner WAITs; barrierGit rendezvouses both past their pre-check and
	// fingerprint before either reserves the slot.
	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation

	proc := &countingProc{}
	var barrier sync.WaitGroup
	barrier.Add(2)
	git := &barrierGit{wg: &barrier, head: "HEAD1"}

	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch()}
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
			docs[i], errs[i] = drivers[i].Start(succ)
		}(i)
	}
	wg.Wait()

	// Exactly one launch total — the reservation, not the launch, arbitrates.
	if got := proc.launches(); got != 1 {
		t.Fatalf("two concurrent successor starts must admit EXACTLY ONE launch, got %d", got)
	}
	// Exactly one nil-error winner; the loser is a typed reservation rejection.
	winners := 0
	winIdx := -1
	for i, e := range errs {
		if e == nil {
			winners++
			winIdx = i
			continue
		}
		if !isOwnershipKind(e, ErrScopeBusy) && !isOwnershipKind(e, ErrStalePredecessor) {
			t.Fatalf("the losing successor must fail ErrScopeBusy or ErrStalePredecessor, got %v", e)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one successor must win, got %d", winners)
	}
	if docs[winIdx].Outcome != WAITING {
		t.Fatalf("the winning successor must WAIT, got %s (%s)", docs[winIdx].Outcome, docs[winIdx].Cause)
	}
	if docs[winIdx].DriveID == first.DriveID {
		t.Fatalf("the winning successor must be a NEW drive, got the predecessor's id")
	}

	// The predecessor was retired exactly once: owner cleared, verdict intact.
	firstRec, err := store.Load(first.DriveID)
	if err != nil {
		t.Fatalf("Load predecessor: %v", err)
	}
	if firstRec.OwnerGeneration != "" {
		t.Fatalf("the predecessor must be retired (owner cleared), got %q", firstRec.OwnerGeneration)
	}
	if firstRec.LastOutcome != PASSED {
		t.Fatalf("the predecessor verdict must survive retirement, got %s", firstRec.LastOutcome)
	}

	// The scope names exactly the sole winner's launched successor, chained to the pred.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID != docs[winIdx].DriveID || scope.CurrentDriveState != scopeStateLaunched {
		t.Fatalf("the slot must name the sole winner launched, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
	}
	if scope.PriorDriveID != first.DriveID {
		t.Fatalf("the prior drive must chain to the retired predecessor, got %q", scope.PriorDriveID)
	}
	if scope.PendingAckDriveID != "" {
		t.Fatalf("the completed transition must leave no pending ack, got %q", scope.PendingAckDriveID)
	}
}

// ---------------------------------------------------------------------------
// Deterministic cancel/launch race barriers (change 0437 Task 7, AC2+AC6). A
// fake EpochLaunchGate backed by a mutable, mutex-guarded registry stands in for
// the app gate: it reads the epoch's liveness under the registry mutex and, while
// STILL holding it, runs the driver's durable reserve body — exactly as the
// production gate runs reserve under the held epoch lock. A concurrent fence
// ("cancel") takes the SAME mutex, so it either lands before the liveness read
// (reserve never runs) or after the gate released the lock (it observes the
// durable reservation reserve produced). "cancel" = flip the fake to fenced, then
// run ReconcileEpochLaunches (which consults NO gate; the epoch is already fenced,
// so cancellation may never hold the epoch while probing a per-drive claim). Every
// ordering assertion below is a channel/done-ordering fact — no timing sleep is an
// oracle anywhere.
// ---------------------------------------------------------------------------

// errEpochFenced is the sentinel a fenced fakeEpochRegistry gate refuses with,
// standing in for the app's ErrRunCancelled/ErrStaleRunEpoch fence tokens.
var errEpochFenced = errors.New("gatedrive-test: run epoch fenced (cancelled)")

// fakeEpochRegistry is a mutable, mutex-guarded stand-in for the app's run-epoch
// registry. The EpochLaunchGate it produces holds the registry mutex across the
// liveness read AND the reserve body — modelling the production epoch lock held
// across reserve — so a concurrent fence serializes against it: the fence lands
// strictly before the read (reserve never runs) or strictly after reserve's
// durable decision. A fenced epoch's gate refuses errEpochFenced WITHOUT running
// reserve (the EpochLaunchGate contract: a validation failure never calls reserve).
type fakeEpochRegistry struct {
	mu     sync.Mutex
	fenced map[string]bool
}

// fence flips epochID to fenced. It takes the same mutex the gate body holds, so
// it can only land in the serialization windows the gate leaves open.
func (r *fakeEpochRegistry) fence(epochID string) {
	r.mu.Lock()
	if r.fenced == nil {
		r.fenced = map[string]bool{}
	}
	r.fenced[epochID] = true
	r.mu.Unlock()
}

func (r *fakeEpochRegistry) gate() EpochLaunchGate {
	return func(epochID, _ string, reserve func() error) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.fenced[epochID] {
			return errEpochFenced // fenced: refuse without running reserve
		}
		return reserve()
	}
}

// TestBarrierCancelBeforeAdmit proves the fence-first outcome: a fence that lands
// before Admit's liveness read makes Admit refuse, reserving nothing durable, and
// a subsequent reconcile has nothing to account (Accounted, no findings).
func TestBarrierCancelBeforeAdmit(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	reg := &fakeEpochRegistry{}
	d.SetEpochLaunchGate(reg.gate())

	// The fence lands FIRST.
	reg.fence("e1")

	req := sampleStart()
	req.RunEpochID = "e1"
	ticket, err := d.Admit(req)
	if !errors.Is(err, errEpochFenced) {
		t.Fatalf("a fence before Admit must refuse, got ticket=%v err=%v", ticket, err)
	}
	if ticket != nil {
		t.Fatalf("a refused admission returns no ticket, got %+v", ticket)
	}
	if _, _, lerr := store.LoadWorktreeExecution(req.Worktree); !storeErrIs(lerr, ErrNotFound) {
		t.Fatalf("a fenced Admit must reserve no worktree slot, LoadWorktreeExecution err = %v", lerr)
	}
	if n := driveRecordCount(t, store); n != 0 {
		t.Fatalf("a fenced Admit must mint no drive record, got %d", n)
	}
	if proc.launchN != 0 {
		t.Fatalf("a fenced Admit must launch nothing, proc.Launch called %d times", proc.launchN)
	}

	report, err := d.ReconcileEpochLaunches(req.Worktree, "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches: %v", err)
	}
	if !report.Accounted || len(report.Findings) != 0 {
		t.Fatalf("a fence before any admission has nothing to account, got %+v", report)
	}
}

// TestBarrierCancelBetweenAdmitAndStartAdmitted proves the fence that lands after
// Admit's return refuses the delayed launch: reconcile sees the reserved drive
// pending (launch-pending) until StartAdmitted's refusal settles the record
// terminal, then a replay accounts it. No process is ever launched.
func TestBarrierCancelBetweenAdmitAndStartAdmitted(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	proc := &fakeProc{}
	d, store := newTestDriver(t, clk, proc, stableGit())
	reg := &fakeEpochRegistry{}
	d.SetEpochLaunchGate(reg.gate())

	req := sampleStart()
	req.RunEpochID = "e1"
	ticket, err := d.Admit(req) // epoch live: the reservation is durable
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}

	// The fence lands AFTER Admit's return (the serialization window between the
	// two phases). The durable reservation already exists.
	reg.fence("e1")

	// Reconcile sees the reserved-but-unlaunched drive pending until the refusal.
	pending, err := d.ReconcileEpochLaunches(req.Worktree, "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches (pre-refusal): %v", err)
	}
	if pending.Accounted || !reconcileFindingPresent(pending.Findings, "launch-pending:"+ticket.id) {
		t.Fatalf("a fenced-but-unrefused reservation must be pending, got %+v", pending)
	}

	// StartAdmitted observes the fence: it refuses, launches nothing, and settles
	// the delayed ticket's drive HALTED run-cancelled with the slot released.
	if _, serr := d.StartAdmitted(ticket); !errors.Is(serr, errEpochFenced) {
		t.Fatalf("StartAdmitted under a fence must refuse, got %v", serr)
	}
	if proc.launchN != 0 {
		t.Fatalf("a fenced StartAdmitted must launch nothing, proc.Launch called %d times", proc.launchN)
	}
	rec, lerr := store.Load(ticket.id)
	if lerr != nil {
		t.Fatalf("Load: %v", lerr)
	}
	if rec.LastOutcome != HALTED || rec.LastCause != "run-cancelled" {
		t.Fatalf("the refusal must settle the drive HALTED run-cancelled, got %v/%q", rec.LastOutcome, rec.LastCause)
	}

	// A replay now accounts the obligation (the record is terminal).
	settled, err := d.ReconcileEpochLaunches(req.Worktree, "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches (post-refusal): %v", err)
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
	reg := &fakeEpochRegistry{}
	clk := &fakeClock{now: startEpoch()}

	dead := false
	launchEntered := make(chan struct{})
	releaseLaunch := make(chan struct{})
	var launchCount int32
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
		if n == 2 { // the replacement launch: reserve committed, the claim is HELD
			close(launchEntered)
			<-releaseLaunch
		}
		return &process.LaunchOutcome{RunID: id, RunDir: "/runs/" + id, State: process.StateRunning}, nil
	}
	d := scopedTestDriver(store, clk, proc, stableGit())
	d.SetEpochLaunchGate(reg.gate())

	// A scope-bound first start over live epoch e1 WAITs (run1 running, slot executing).
	req, started := startScopedWaitingWithEpoch(t, d, store, "e1")

	// The run dies; its single automatic relaunch is authorized under the live gate,
	// then parks in proc.Launch (reserve committed inside the gate; the claim held).
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

	// The fence lands NOW — after authorization, during the parked launch.
	reg.fence("e1")

	// A concurrent reconcile (an independent CLI process: its own store handle and
	// process seam) reports the held claim as pending work, and returns promptly.
	recDone := make(chan EpochLaunchReport, 1)
	go func() {
		dr := scopedTestDriver(reopenStore(store), &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
		r, _ := dr.ReconcileEpochLaunches(req.Worktree, "e1")
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

	// A replay now identifies and stops the replacement, and only THEN accounts.
	var stopped []string
	recProc := &fakeProc{
		stop: func(runDir, reason string) (*process.StopOutcome, error) {
			stopped = append(stopped, runDir)
			return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
		},
		resolve: func(root, token string) (*process.ReservationResolution, error) {
			return &process.ReservationResolution{Disposition: "identified", RunID: "run2", RunDir: "/runs/run2", State: process.StateRunning}, nil
		},
	}
	dr := scopedTestDriver(reopenStore(store), &fakeClock{now: startEpoch()}, recProc, stableGit())
	replay, err := dr.ReconcileEpochLaunches(req.Worktree, "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches (replay): %v", err)
	}
	if !replay.Accounted {
		t.Fatalf("cancellation completes only once the replacement is identified and stopped, got %+v", replay)
	}
	if len(stopped) == 0 {
		t.Fatalf("the replay must stop the identified replacement, stopped nothing")
	}
}

// TestBarrierCancelBetweenLaunchAndAttach proves a fence during a scopeless
// StartAdmitted's launch-to-attach window (the process exists, the claim held
// across launch+attach) reports claim-busy pending, then a replay accounts once
// the run is attached and stopped — and, crucially, that once a replay reports
// accounted, a subsequent start on the fenced epoch refuses and launches nothing.
func TestBarrierCancelBetweenLaunchAndAttach(t *testing.T) {
	reg := &fakeEpochRegistry{}
	clk := &fakeClock{now: startEpoch()}
	entered := make(chan struct{})
	release := make(chan struct{})
	proc := &fakeProc{}
	// proc.Launch records the run then parks BEFORE returning to the driver — the
	// process is created but attach has not run, and the claim is held across both.
	proc.launch = func(process.LaunchRequest) (*process.LaunchOutcome, error) {
		close(entered)
		<-release
		return &process.LaunchOutcome{RunID: "run1", RunDir: "/runs/run1", State: process.StateRunning}, nil
	}
	d, store := newTestDriver(t, clk, proc, stableGit())
	d.SetEpochLaunchGate(reg.gate())

	req := sampleStart()
	req.RunEpochID = "e1"
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

	recDone := make(chan EpochLaunchReport, 1)
	go func() {
		dr := scopedTestDriver(reopenStore(store), &fakeClock{now: startEpoch()}, &fakeProc{}, stableGit())
		r, _ := dr.ReconcileEpochLaunches(req.Worktree, "e1")
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

	// A replay identifies and stops the attached run, then accounts.
	var stopped []string
	recProc := &fakeProc{
		stop: func(runDir, reason string) (*process.StopOutcome, error) {
			stopped = append(stopped, runDir)
			return &process.StopOutcome{State: process.StateStopped, RunDir: runDir, Performed: true}, nil
		},
	}
	dr := scopedTestDriver(reopenStore(store), &fakeClock{now: startEpoch()}, recProc, stableGit())
	replay, err := dr.ReconcileEpochLaunches(req.Worktree, "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches (replay): %v", err)
	}
	if !replay.Accounted {
		t.Fatalf("a replay after attach+stop must account, got %+v", replay)
	}
	if len(stopped) != 1 || stopped[0] != "/runs/run1" {
		t.Fatalf("the replay must stop the identified run /runs/run1, stopped %v", stopped)
	}

	// No launch after an accounted reconcile: a subsequent start on the fenced epoch
	// refuses and proc.Launch's call count is final.
	launchesBefore := proc.launchN
	next := sampleStart()
	next.RunEpochID = "e1"
	if _, nerr := d.Start(next); !errors.Is(nerr, errEpochFenced) {
		t.Fatalf("a start on the fenced epoch must refuse, got %v", nerr)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("no launch may occur after an accounted reconcile, launches %d->%d", launchesBefore, proc.launchN)
	}
}

// TestBarrierSameScopeFirstStartContention proves the initial-start peer race is
// unaffected by the epoch gate: two same-scope, same-epoch first starts rendezvous
// past their fingerprint pre-check, then contend; exactly one wins and launches,
// the loser is refused typed and launches nothing, and the loser releases nothing
// the winner adopted. Run under -race.
func TestBarrierSameScopeFirstStartContention(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))
	req := sampleStart()
	sreq := scopeReqFor(req, "")
	sreq.RunEpochID = "e1"
	grant, err := store.PrepareScope(sreq)
	if err != nil {
		t.Fatalf("PrepareScope: %v", err)
	}
	req.ScopeID = grant.ScopeID
	req.ChildCapability = grant.ChildCapability
	req.RunEpochID = "e1"

	reg := &fakeEpochRegistry{}
	proc := &countingProc{}
	var barrier sync.WaitGroup
	barrier.Add(2)
	git := &barrierGit{wg: &barrier, head: "HEAD1"}

	mkDriver := func() *Driver {
		clk := &fakeClock{now: startEpoch()}
		d := NewDriver(store, clk, proc, git)
		d.slice = 4 * pollTick
		d.pollInterval = pollTick
		d.sleep = func(dur time.Duration) { clk.advance(dur) }
		d.SetEpochLaunchGate(reg.gate())
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
			docs[i], errs[i] = drivers[i].Start(req)
		}(i)
	}
	wg.Wait()

	if got := proc.launches(); got != 1 {
		t.Fatalf("two same-scope same-epoch first starts must launch EXACTLY once, got %d", got)
	}
	winners, winIdx := 0, -1
	for i, e := range errs {
		if e == nil {
			winners++
			winIdx = i
			continue
		}
		if !isOwnershipKind(e, ErrScopeBusy) && !isOwnershipKind(e, ErrScopeSecondDrive) {
			t.Fatalf("the losing start must fail ErrScopeBusy or ErrScopeSecondDrive, got %v", e)
		}
	}
	if winners != 1 {
		t.Fatalf("exactly one start must win, got %d", winners)
	}
	if docs[winIdx].Outcome != WAITING {
		t.Fatalf("the winning start must WAIT, got %s (%s)", docs[winIdx].Outcome, docs[winIdx].Cause)
	}
	// The loser released nothing the winner adopted: the scope names the winner's
	// launched drive, and the worktree slot is the winner's executing reservation.
	scope, err := store.LoadScope(grant.ScopeID)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if scope.CurrentDriveID != docs[winIdx].DriveID || scope.CurrentDriveState != scopeStateLaunched {
		t.Fatalf("the scope must name the sole winner launched, got id=%q state=%q", scope.CurrentDriveID, scope.CurrentDriveState)
	}
	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if slot.State != admissionExecuting {
		t.Fatalf("the winner's worktree slot must be executing, got %q", slot.State)
	}
}

// TestSameScopeFirstStartLateLoserDoesNotRotate deterministically pins the losing
// interleaving of TestBarrierSameScopeFirstStartContention: a receipt-less first
// start that passed precheckScopedStart before the winner published the scope, and
// whose worktree admission runs only AFTER the winner confirmed the slot to
// executing. Rotation is successor-only, so the late loser must be refused typed
// ErrScopeSecondDrive with the winner's executing reservation untouched — same
// state, same token, same ExecutionGen.
func TestSameScopeFirstStartLateLoserDoesNotRotate(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{}
	d := scopedTestDriver(store, clk, proc, stableGit())
	_, req := prepareScopedStart(t, store)

	// The winner: a full first start that launches and confirms its slot.
	doc, err := d.Start(req)
	if err != nil {
		t.Fatalf("winner Start: %v", err)
	}
	if doc.Outcome != WAITING {
		t.Fatalf("winner must WAIT, got %s (%s)", doc.Outcome, doc.Cause)
	}
	before, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	if before.State != admissionExecuting {
		t.Fatalf("precondition: the winner's slot must be executing, got %q", before.State)
	}

	// The late loser: the SAME receipt-less request reaches worktree admission only
	// now. Calling admitScopedWorktree directly models the loser that already passed
	// its precheck against the then-empty scope; admission must refuse typed and
	// must not rotate the winner's live reservation.
	_, _, _, _, _, aerr := d.admitScopedWorktree(req)
	if !isOwnershipKind(aerr, ErrScopeSecondDrive) {
		t.Fatalf("a late receipt-less first start must refuse ErrScopeSecondDrive, got %v", aerr)
	}
	after, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after refusal: %v", err)
	}
	if after.State != admissionExecuting || after.ReservationToken != before.ReservationToken || after.ExecutionGen != before.ExecutionGen {
		t.Fatalf("the winner's executing reservation must be untouched: state %q->%q, token changed=%v, gen %d->%d",
			before.State, after.State, after.ReservationToken != before.ReservationToken, before.ExecutionGen, after.ExecutionGen)
	}
}

// TestBarrierSuccessorUnderCancel proves the successor path under a mid-flight
// fence: a fenced successor start refuses without launching, and it leaves the slot
// EITHER the predecessor's executing reservation (refused before rotation) OR
// released (refused after rotation) — both legal, and neither leaks a
// reserved-but-unreleased rotation. Cancellation then has nothing to chase.
func TestBarrierSuccessorUnderCancel(t *testing.T) {
	clk := &fakeClock{now: startEpoch()}
	store := OpenStore(testsupport.TempDir(t))
	proc := &fakeProc{} // WAIT: executing slot
	reg := &fakeEpochRegistry{}
	d := scopedTestDriver(store, clk, proc, stableGit())
	d.SetEpochLaunchGate(reg.gate())
	_, req := prepareScopedStart(t, store)
	req.RunEpochID = "e1"

	first, err := d.Start(req)
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if first.Outcome != WAITING {
		t.Fatalf("first start must WAIT, got %s (%s)", first.Outcome, first.Cause)
	}
	predSlot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution: %v", err)
	}
	oldToken := predSlot.ReservationToken
	// Terminal-before-release window: the successor would reach the executing arm.
	if err := store.ownerCAS(first.DriveID, func(r *driveRecord) error {
		r.LastOutcome = PASSED
		return nil
	}); err != nil {
		t.Fatalf("settle predecessor terminal: %v", err)
	}

	// The fence lands mid-flight, before the successor starts.
	reg.fence("e1")

	launchesBefore := proc.launchN
	succ := req
	succ.PredecessorDriveID = first.DriveID
	succ.PredecessorOwnerGen = first.Generation
	if _, serr := d.Start(succ); !errors.Is(serr, errEpochFenced) {
		t.Fatalf("a fenced successor start must refuse, got %v", serr)
	}
	if proc.launchN != launchesBefore {
		t.Fatalf("a fenced successor must never launch, launched %d->%d", launchesBefore, proc.launchN)
	}

	slot, _, err := store.LoadWorktreeExecution(req.Worktree)
	if err != nil {
		t.Fatalf("LoadWorktreeExecution after refusal: %v", err)
	}
	switch slot.State {
	case admissionExecuting:
		if slot.ReservationToken != oldToken {
			t.Fatalf("before-rotation refusal must keep the predecessor's executing reservation, token changed")
		}
	case admissionReleased:
		// after-rotation-then-released: legal, nothing leaked.
	default:
		t.Fatalf("a fenced successor must leave the slot executing (unrotated) or released, got %q", slot.State)
	}

	// Cancellation has nothing to chase: the predecessor is terminal (accounted by
	// slot/participant teardown), and the successor created no drive.
	report, err := d.ReconcileEpochLaunches(req.Worktree, "e1")
	if err != nil {
		t.Fatalf("ReconcileEpochLaunches: %v", err)
	}
	if !report.Accounted {
		t.Fatalf("a fenced successor leaves nothing pending, got %+v", report)
	}
}

// --- change 0446 spec §3 / AC7: finished-incumbent reconciliation under races ---

// TestConcurrentAdmitsOverFinishedIncumbentAdmitOnce races several Admits over ONE
// proven-finished incumbent (a PASSED drive whose release was interrupted). Every
// racer may reconcile the incumbent, but the slot CAS with the exact expected token
// and state lets only one reconciliation release it and the reserve under the slot
// lock admits exactly one execution; every loser is refused with the winner's slot
// intact.
func TestConcurrentAdmitsOverFinishedIncumbentAdmitOnce(t *testing.T) {
	seam := &incumbentSeam{}
	d, store := newIncumbentDriver(t, seam)
	req := incumbentStart(t)
	finishedDriveIncumbent(t, d, store, seam, req)

	const racers = 4
	var (
		start   sync.WaitGroup
		done    sync.WaitGroup
		mu      sync.Mutex
		tickets []*AdmissionTicket
		refused int
	)
	start.Add(1)
	for i := 0; i < racers; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			ticket, err := d.Admit(req)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if oe, ok := AsOwnershipError(err); !ok || (oe.Kind != ErrWorktreeBusy && oe.Kind != ErrUnresolvedExecution) {
					t.Errorf("a losing racer must be refused worktree-busy/unresolved, got %v", err)
				}
				refused++
				return
			}
			tickets = append(tickets, ticket)
		}()
	}
	start.Done()
	done.Wait()

	if len(tickets) != 1 || refused != racers-1 {
		t.Fatalf("admitted %d, refused %d; want exactly one admission over one finished incumbent", len(tickets), refused)
	}
	slot := mustSlot(t, store, req.Worktree)
	if slot.State != admissionReserved || slot.ReservationToken != tickets[0].token {
		t.Fatalf("the winner's reservation must hold the slot: state=%s", slot.State)
	}
	launches := seam.launchN
	if _, err := d.StartAdmitted(tickets[0]); err != nil {
		t.Fatalf("StartAdmitted: %v", err)
	}
	if seam.launchN != launches+1 {
		t.Fatalf("launches = %d, want exactly one", seam.launchN-launches)
	}
}

// TestReconcileSuccessorRaceLeavesSuccessorUntouched: a successor reservation lands
// between reconciliation's probe and its CAS (the probe proved the OLD incumbent
// finished). The CAS with the old expected token fails, reconciliation re-evaluates
// the ACTUAL incumbent once — never releasing it with the newly observed token — and
// the admission is refused with the successor's slot byte-identical.
func TestReconcileSuccessorRaceLeavesSuccessorUntouched(t *testing.T) {
	seam := &incumbentSeam{}
	d, store := newIncumbentDriver(t, seam)
	req := incumbentStart(t)
	old := finishedDriveIncumbent(t, d, store, seam, req)

	fired := 0
	var successor string
	incumbentApplyHook = func() {
		fired++
		if fired > 1 {
			return
		}
		tok, err := store.rotateWorktreeExecutionForSuccessor(req.Worktree, old)
		if err != nil {
			t.Errorf("successor rotation: %v", err)
		}
		successor = tok
	}
	t.Cleanup(func() { incumbentApplyHook = nil })

	ticket, err := d.Admit(req)
	if ticket != nil {
		t.Fatalf("the admission must not win over a successor that raced the settle")
	}
	requireIncumbentRefusal(t, err, store, req.Worktree, successor, admissionReserved)
	if fired != 1 {
		t.Fatalf("apply hook fired %d times; the successor must be re-evaluated, never released on its new token", fired)
	}
}

// TestSameWorktreeRaceAcrossOwnersRawAndAliasOneWinner (change 0446 spec AC7)
// extends the pairwise one-worktree races above to the full contender set at once:
// a scoped start, a scopeless start, a scopeless start through a SYMLINK ALIAS of
// the worktree, and a participating raw reservation all rendezvous past their
// unlocked pre-checks and contend for the one worktree slot. Exactly one wins
// (at most one backend launch; the raw winner launches none), every loser is a
// typed worktree refusal, and the slot names exactly the winner's reservation.
// Several rounds widen the interleavings; run under -race.
func TestSameWorktreeRaceAcrossOwnersRawAndAliasOneWinner(t *testing.T) {
	for round := 0; round < 8; round++ {
		t.Run(fmt.Sprintf("round-%d", round), func(t *testing.T) {
			store := OpenStore(testsupport.TempDir(t))
			wt := mkWorktree(t)
			alias := filepath.Join(testsupport.TempDir(t), "alias")
			if err := os.Symlink(wt, alias); err != nil {
				t.Fatal(err)
			}
			_, scoped := prepareScopedStartAt(t, store, wt, "0342")
			scopeless := sampleStart()
			scopeless.Worktree = wt
			scopeless.ChangeID = "0343"
			viaAlias := sampleStart()
			viaAlias.Worktree = alias
			viaAlias.ChangeID = "0344"
			reqs := []StartRequest{scoped, scopeless, viaAlias}

			proc := &countingProc{}
			var barrier sync.WaitGroup
			barrier.Add(len(reqs) + 1)
			git := &barrierGit{wg: &barrier, head: "HEAD1"}

			errs := make([]error, len(reqs)+1)
			var rawToken string
			var wg sync.WaitGroup
			for i := range reqs {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					clk := &fakeClock{now: startEpoch()}
					_, errs[i] = scopedTestDriver(store, clk, proc, git).Start(reqs[i])
				}(i)
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				barrier.Done()
				barrier.Wait()
				rawToken, errs[len(reqs)] = store.ReserveRawWorktreeExecution("repo-x", wt, proc)
			}()
			wg.Wait()

			winners := 0
			for i, err := range errs {
				if err == nil {
					winners++
					continue
				}
				if !isOwnershipKind(err, ErrWorktreeBusy) && !isOwnershipKind(err, ErrUnresolvedExecution) {
					t.Fatalf("contender %d must lose with a typed worktree refusal, got %v", i, err)
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
			slot, _, err := store.LoadWorktreeExecution(alias)
			if err != nil {
				t.Fatalf("load slot through the alias: %v", err)
			}
			if rawWon && (slot.Kind != "raw" || slot.ReservationToken != rawToken) {
				t.Fatalf("the raw winner's reservation must hold the slot, got kind %q", slot.Kind)
			}
			if !rawWon && slot.State != admissionExecuting {
				t.Fatalf("a driven winner's slot must be executing, got %s", slot.State)
			}
		})
	}
}
