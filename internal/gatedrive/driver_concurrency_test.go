package gatedrive

import (
	"fmt"
	"strings"
	"sync"
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
	rec := seedRecord(t)
	rec.AdmissionToken = "aaaaaaaaaaaaaaaa"
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
