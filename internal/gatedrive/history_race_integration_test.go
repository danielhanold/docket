//go:build integration

package gatedrive

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestRaceIntegrationGatedriveConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe is Criterion 5:
// two concurrent Starts on ONE worktree over a store seeded with nonblocking
// legacy history admit exactly one launch (the loser refused ErrWorktreeBusy, never
// a legacy-inventory refusal), and a manual CleanupHistory racing them completes
// without deadlock and leaves every seeded record byte-identical. Run under -race.
// Race shard (change 0466): concurrent Starts and a CleanupHistory race over one legacy-seeded store.
func TestRaceIntegrationGatedriveConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe(t *testing.T) {
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
	reqA := sampleStart()
	reqA.Worktree = wt
	reqB := sampleStart()
	reqB.Worktree = wt
	reqB.ChangeID = "0343"
	reqs := []StartRequest{reqA, reqB}

	proc := &tornDownProc{countingProc: &countingProc{}}
	var barrier sync.WaitGroup
	barrier.Add(2) // only the two Starts fingerprint; CleanupHistory never touches git.
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
