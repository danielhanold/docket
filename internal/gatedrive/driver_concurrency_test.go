package gatedrive

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
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
			store := OpenStore(t.TempDir())
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
