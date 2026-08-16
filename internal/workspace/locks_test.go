package workspace

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// A file lock establishes no happens-before edge the Go race detector
// recognizes, so mutual exclusion cannot be proven with an unguarded shared
// counter — that reports a data race even when the lock serializes correctly.
// These tests instead prove exclusion two race-clean ways: a deterministic
// non-blocking probe (tryOperationLock reports held while the lock is out), and
// an atomic in-section occupancy counter whose observed maximum must stay 1.

// occupancy runs `workers` goroutines that all pass through fn while holding the
// per-workspace operation lock, released together by a start channel barrier. It
// returns the greatest number of goroutines observed inside the locked section at
// once (1 when the lock serializes) and the number that completed.
func occupancy(t *testing.T, dir string, workers int) (maxInside int32, completed int32) {
	t.Helper()
	var inside, maxSeen, done int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			release, err := acquireOperationLock(dir)
			if err != nil {
				t.Errorf("acquireOperationLock: %v", err)
				return
			}
			n := atomic.AddInt32(&inside, 1)
			for {
				m := atomic.LoadInt32(&maxSeen)
				if n <= m || atomic.CompareAndSwapInt32(&maxSeen, m, n) {
					break
				}
			}
			// A little work widens the window a broken lock would overlap in.
			var acc int
			for k := 0; k < 20000; k++ {
				acc += k
			}
			_ = acc
			atomic.AddInt32(&inside, -1)
			atomic.AddInt32(&done, 1)
			release()
		}()
	}
	close(start)
	wg.Wait()
	return atomic.LoadInt32(&maxSeen), atomic.LoadInt32(&done)
}

// TestOperationLockMutualExclusion proves the per-workspace operation lock
// serializes: with many contending goroutines, at most one is ever inside the
// locked section, and all complete.
func TestOperationLockMutualExclusion(t *testing.T) {
	commonDir := t.TempDir()
	dir := workspaceDir(commonDir, gitcli.RefName("refs/heads/feat/x"))

	const workers = 8
	maxInside, completed := occupancy(t, dir, workers)
	if maxInside != 1 {
		t.Fatalf("max goroutines inside the locked section = %d, want 1 (lock did not serialize)", maxInside)
	}
	if completed != workers {
		t.Fatalf("completed = %d, want %d", completed, workers)
	}
}

// TestTryOperationLock proves tryOperationLock reports held while the blocking
// lock is out, and free (acquirable) after release. This is the deterministic,
// race-clean exclusion guard: if acquireOperationLock did not truly lock, the
// probe would find the lock free while it is held.
func TestTryOperationLock(t *testing.T) {
	commonDir := t.TempDir()
	dir := workspaceDir(commonDir, gitcli.RefName("refs/heads/feat/x"))

	release, err := acquireOperationLock(dir)
	if err != nil {
		t.Fatalf("acquireOperationLock: %v", err)
	}

	rel, held, err := tryOperationLock(dir)
	if err != nil {
		t.Fatalf("tryOperationLock while held: %v", err)
	}
	if !held {
		t.Fatalf("tryOperationLock reported free while the blocking lock is out")
	}
	if rel != nil {
		t.Fatalf("tryOperationLock returned a release func for a held lock")
	}

	release()

	rel, held, err = tryOperationLock(dir)
	if err != nil {
		t.Fatalf("tryOperationLock after release: %v", err)
	}
	if held {
		t.Fatalf("tryOperationLock reported held after release")
	}
	if rel == nil {
		t.Fatalf("tryOperationLock returned no release func for a free lock")
	}
	rel()
}

// TestOperationLockOrdered proves a blocked second acquire observes the first
// release: the waiter's acquisition is recorded only after the holder appended
// its release event, so the guarded order log ends [holder-released,
// waiter-acquired]. Coordination is channels only — no sleeps.
func TestOperationLockOrdered(t *testing.T) {
	commonDir := t.TempDir()
	dir := workspaceDir(commonDir, gitcli.RefName("refs/heads/feat/x"))

	var mu sync.Mutex
	var order []string
	record := func(s string) {
		mu.Lock()
		order = append(order, s)
		mu.Unlock()
	}

	rel1, err := acquireOperationLock(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	started := make(chan struct{})
	acquired := make(chan struct{})
	go func() {
		close(started)
		rel2, err := acquireOperationLock(dir) // blocks until rel1()
		if err != nil {
			t.Errorf("second acquire: %v", err)
			close(acquired)
			return
		}
		record("waiter-acquired")
		close(acquired)
		rel2()
	}()

	<-started
	record("holder-released")
	rel1()
	<-acquired

	if len(order) != 2 || order[0] != "holder-released" || order[1] != "waiter-acquired" {
		t.Fatalf("event order = %v, want [holder-released waiter-acquired]", order)
	}
}

// TestOperationLockDifferentDirsIndependent proves distinct workspace dirs do
// NOT serialize: the second lock is taken while the first is still held, without
// deadlock.
func TestOperationLockDifferentDirsIndependent(t *testing.T) {
	commonDir := t.TempDir()
	dirA := workspaceDir(commonDir, gitcli.RefName("refs/heads/feat/a"))
	dirB := workspaceDir(commonDir, gitcli.RefName("refs/heads/feat/b"))

	relA, err := acquireOperationLock(dirA)
	if err != nil {
		t.Fatalf("acquire A: %v", err)
	}
	relB, err := acquireOperationLock(dirB) // must not block on A
	if err != nil {
		t.Fatalf("acquire B while holding A: %v", err)
	}
	relB()
	relA()
}

// TestRegistryLockReleaseUnblocks proves the workspaces-root registry lock is
// exclusive and that releasing it unblocks a waiter: the waiter's blocked acquire
// completes only after the holder releases.
func TestRegistryLockReleaseUnblocks(t *testing.T) {
	commonDir := t.TempDir()
	root := workspacesRoot(commonDir)

	rel1, err := acquireRegistryLock(root)
	if err != nil {
		t.Fatalf("first registry acquire: %v", err)
	}

	acquired := make(chan struct{})
	started := make(chan struct{})
	go func() {
		close(started)
		rel2, err := acquireRegistryLock(root) // blocks until rel1()
		if err != nil {
			t.Errorf("second registry acquire: %v", err)
			close(acquired)
			return
		}
		close(acquired)
		rel2()
	}()

	<-started
	// While held, the waiter must not have completed its acquire.
	select {
	case <-acquired:
		t.Fatal("registry waiter acquired while the lock was held")
	default:
	}
	rel1()
	<-acquired // release unblocks the waiter
}
