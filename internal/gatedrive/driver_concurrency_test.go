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
// concurrent-relaunch race deterministically. Its Launch rendezvouses two
// concurrent advances at a barrier so BOTH reach the relaunch Launch (each
// having loaded a record with RelaunchCount==0) before either proceeds to the
// serializing store CAS — the exact interleaving the finding describes. All
// bookkeeping is mutex/atomic-guarded so the test is clean under -race.
type racingProc struct {
	barrier     *sync.WaitGroup // trips once both advances have launched
	newRunState process.State   // the state a relaunched run reports

	mu        sync.Mutex
	launchSeq int
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

	// Rendezvous: hold this launch until the concurrent advance has also
	// launched, guaranteeing both relaunches escape the CAS before either
	// commits — the double-launch window the fix must close.
	p.barrier.Done()
	p.barrier.Wait()

	return &process.LaunchOutcome{RunID: id, RunDir: dir, State: process.StateRunning}, nil
}

func (p *racingProc) Observe(runDir string) (*process.Observation, error) {
	// The seeded original run is dead (signaled); every relaunched run reports
	// the scripted new-run state.
	if strings.HasSuffix(runDir, "run1") {
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
// decided atomically under the ownership CAS: two concurrent Advance calls that
// present the SAME valid owner generation over a nonterminal record whose child
// has died must together yield EXACTLY ONE relaunch (RelaunchCount==1) and
// exactly one live owned tree. The losing advance — which also launched a fresh
// run outside the lock — must NOT commit a second relaunch and must STOP its
// orphaned run so no duplicate/leaked suite tree survives. Both the nonterminal
// (WAITING) winner and the terminal (FAILED) winner exercise the loser-cleanup
// path (errRelaunchRaceLost and errAlreadyTerminal respectively).
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

			// Both advances launched a candidate outside the CAS — inherent to the
			// pre-lock launch and the point of the race.
			if proc.launchSeq != 2 {
				t.Fatalf("both concurrent advances must launch a relaunch candidate, got %d launches", proc.launchSeq)
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

			// The losing advance must STOP its orphaned relaunch — exactly one such
			// cleanup, and no leaked live tree beyond the winner's.
			relaunchStops, stopped := proc.relaunchStopCount()
			if relaunchStops != 1 {
				t.Fatalf("the losing advance must stop its orphaned relaunch (exactly one), stopped %d relaunch runs", relaunchStops)
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
