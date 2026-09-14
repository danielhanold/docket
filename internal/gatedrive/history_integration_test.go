package gatedrive

// End-to-end acceptance tests for the legacy-history recovery composition (change
// 0428). Unlike history_test.go (which unit-tests the classifier and the reserve
// chokepoint in isolation), these drive the WHOLE admission → drive path against a
// legacy-seeded store, proving two spec acceptance criteria the unit tests cannot
// vouch for on their own:
//
//   - Criterion 5 (concurrency): two concurrent Starts over one worktree of a
//     legacy-seeded store admit EXACTLY ONE launch, and a concurrent manual
//     CleanupHistory racing them neither deadlocks (it holds no admission/scope/
//     drive lock) nor mutates any seeded record byte.
//   - Criterion 7 (negative space): an ordinary FAILED or post-launch HALTED drive
//     outcome triggers NO legacy census/cleanup and NO second launch — the census
//     is strictly a first-admission event, never an outcome-driven one.

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// tornDownProc is countingProc whose legacy-recovery seam reports every recorded
// group already torn down, so a seeded HALTED legacy record classifies nonblocking
// and admission over the legacy-seeded store is arbitrated purely by the worktree
// execution slot (never refused on the seeded history). Launches are counted
// through the embedded countingProc so a shared instance aggregates across drivers.
type tornDownProc struct {
	*countingProc
}

func (p *tornDownProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	return process.RecoveryEntry{Disposition: "already-abandoned"}, nil
}

// TestConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe is Criterion 5:
// two concurrent scoped Starts on ONE worktree over a store seeded with nonblocking
// legacy history admit exactly one launch (the loser refused ErrWorktreeBusy, never
// a legacy-inventory refusal), and a manual CleanupHistory racing them completes
// without deadlock and leaves every seeded record byte-identical. Run under -race.
func TestConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe(t *testing.T) {
	store := OpenStore(testsupport.TempDir(t))

	// Seed nonblocking legacy history: two completed drives plus a HALTED drive the
	// recovery seam reports already torn down. None blocks admission, so the two
	// concurrent starts contend solely on the worktree execution slot.
	seeded := []string{"passed", "failed", "halted"}
	before := map[string][]byte{}
	for _, name := range seeded {
		id := copyLegacyFixture(t, store, name)
		p := filepath.Join(store.root, id, recordFileName)
		buf, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read seeded %s record: %v", name, err)
		}
		before[p] = buf
	}

	wt := sampleWorktree()
	_, reqA := prepareScopedStartAt(t, store, wt, "0342")
	_, reqB := prepareScopedStartAt(t, store, wt, "0343")
	reqs := []StartRequest{reqA, reqB}

	proc := &tornDownProc{countingProc: &countingProc{}}
	var barrier sync.WaitGroup
	barrier.Add(2) // only the two Starts fingerprint; CleanupHistory never touches git.
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

	// A concurrent manual history cleanup racing the two starts must neither hang
	// (it acquires no admission/scope/drive lock, so it cannot deadlock against the
	// starts' lock chain) nor mutate any record. It runs with apply=true (non
	// -dry-run) so it exercises the recovery-write path, yet the seeded records are
	// never rewritten (any marker the process layer would write lands in a run dir,
	// never in record.json — and here every group reports already torn down).
	cleanup := mkDriver()
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		for i := 0; i < 25; i++ {
			if _, err := cleanup.CleanupHistory(HistoryCleanupRequest{}); err != nil {
				t.Errorf("concurrent CleanupHistory returned an error: %v", err)
				return
			}
		}
	}()

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
	<-cleanupDone // completing at all is the no-deadlock proof.

	// Exactly one launch total: the worktree slot arbitrates over the legacy-seeded
	// store; the seeded nonblocking history never adds a launch or a refusal.
	if got := proc.launches(); got != 1 {
		t.Fatalf("two concurrent starts over a legacy-seeded store must launch EXACTLY once, got %d", got)
	}
	assertOneWorktreeStartWinner(t, docs, errs)

	// The concurrent cleanup neither deleted nor rewrote any seeded record.
	for p, want := range before {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("seeded record %s vanished after a concurrent cleanup: %v", p, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("seeded record %s was mutated by a concurrent cleanup", p)
		}
	}
}

// censusCountingProc counts every legacy-recovery seam consultation so a test can
// prove a drive OUTCOME never triggers the first-admission census. Launch/Observe/
// Stop/ResolveReservation are the scriptable fakeProc; only ClassifyRun is counted.
type censusCountingProc struct {
	*fakeProc
	classifyN int
}

func (p *censusCountingProc) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	p.classifyN++
	return process.RecoveryEntry{Disposition: "invalid"}, nil
}

// newCensusDriver wires a short-slice driver over a fresh store and the given
// census-counting seam, mirroring newTestDriver for a seam type that is not
// *fakeProc.
func newCensusDriver(t *testing.T, clk *fakeClock, proc *censusCountingProc) *Driver {
	t.Helper()
	d := NewDriver(OpenStore(testsupport.TempDir(t)), clk, proc, stableGit())
	d.slice = 4 * pollTick
	d.pollInterval = pollTick
	d.sleep = func(dur time.Duration) { clk.advance(dur) }
	return d
}

// TestOutcomeTriggersNoLegacyCensusOrSecondStart is Criterion 7's negative space: a
// FAILED suite verdict and a post-launch HALTED (deadline) outcome each launch
// exactly once and consult the legacy-recovery seam ZERO times. The census is a
// first-admission-only event (it runs under the worktree slot lock solely on the
// slot's first reservation), so no drive outcome may re-run it. The HALTED case is
// the stronger guard: the drive persists its own HALTED record before returning, so
// any spurious outcome-driven re-inventory would enumerate that record and consult
// the seam on its recorded run dir — which classifyN==0 forbids.
func TestOutcomeTriggersNoLegacyCensusOrSecondStart(t *testing.T) {
	t.Run("failed", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &censusCountingProc{fakeProc: &fakeProc{
			observe: func(runDir string) (*process.Observation, error) {
				return obs(process.StateFailed, runDir), nil
			},
		}}
		d := newCensusDriver(t, clk, proc)
		doc, err := d.Start(sampleStart())
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if doc.Outcome != FAILED {
			t.Fatalf("a red suite must FAIL, got %s (%s)", doc.Outcome, doc.Cause)
		}
		if proc.launchN != 1 {
			t.Fatalf("a FAILED outcome must not launch a second run, launches=%d", proc.launchN)
		}
		if proc.classifyN != 0 {
			t.Fatalf("a FAILED outcome must trigger NO legacy census/cleanup, seam consulted %d times", proc.classifyN)
		}
	})

	t.Run("halted", func(t *testing.T) {
		clk := &fakeClock{now: startEpoch()}
		proc := &censusCountingProc{fakeProc: &fakeProc{}} // stays running → deadline HALT
		d := newCensusDriver(t, clk, proc)
		req := sampleStart()
		req.Budget = 0 // deadline == start: HALT after one observation, stop-if-owned.
		doc, err := d.Start(req)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		if doc.Outcome != HALTED {
			t.Fatalf("a zero-budget drive must HALT, got %s (%s)", doc.Outcome, doc.Cause)
		}
		if proc.launchN != 1 {
			t.Fatalf("a HALTED outcome must not launch a second run, launches=%d", proc.launchN)
		}
		if proc.classifyN != 0 {
			t.Fatalf("a HALTED outcome must trigger NO legacy census/cleanup, seam consulted %d times", proc.classifyN)
		}
	})
}
