//go:build integration

package transaction

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestRaceIntegrationTxnSetPhaseAtomicUnderConcurrentReads rewrites the phase many times while a
// reader goroutine parses the manifest in a tight loop. Because publication is a
// same-directory temp+rename, every read observes a complete document — a naive
// in-place rewrite would let the reader catch a truncated file.
// Race shard (change 0466): a reader goroutine polls the manifest while the phase is rewritten.
func TestRaceIntegrationTxnSetPhaseAtomicUnderConcurrentReads(t *testing.T) {
	_, repo := newTxnRepo(t)
	c, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
	if err != nil {
		t.Fatalf("allocateCandidate: %v", err)
	}
	defer func() { _ = c.live.release() }()

	manifestPath := filepath.Join(c.root, "manifest.json")
	stop := make(chan struct{})
	readerErr := make(chan error, 1)
	go func() {
		for {
			select {
			case <-stop:
				readerErr <- nil
				return
			default:
			}
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				readerErr <- fmt.Errorf("read: %w", err)
				return
			}
			var m manifest
			if err := json.Unmarshal(data, &m); err != nil {
				readerErr <- fmt.Errorf("partial/parse: %w", err)
				return
			}
		}
	}()

	phases := []phase{phaseReady, phaseCommitted, phasePushed, phaseAllocating}
	const iters = 400
	var last phase
	for i := 0; i < iters; i++ {
		last = phases[i%len(phases)]
		if err := c.setPhase(txnTestClock, last); err != nil {
			close(stop)
			<-readerErr
			t.Fatalf("setPhase: %v", err)
		}
	}
	close(stop)
	if err := <-readerErr; err != nil {
		t.Fatalf("concurrent reader saw a bad manifest: %v", err)
	}

	if got := readManifestFile(t, c.root).Phase; got != last {
		t.Errorf("final phase = %q, want %q", got, last)
	}
}

// TestRaceIntegrationTxnRegistryLockAllocationExcludesConcurrentAllocation proves two allocations
// contending on the same transactions root both succeed and produce distinct
// candidate directories — the registry lock serializes them without deadlock.
// Race shard (change 0466): two goroutines allocate candidates under the registry lock.
func TestRaceIntegrationTxnRegistryLockAllocationExcludesConcurrentAllocation(t *testing.T) {
	_, repo := newTxnRepo(t)

	start := make(chan struct{})
	type res struct {
		c   *candidate
		err error
	}
	results := make(chan res, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			c, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
			results <- res{c, err}
		}()
	}
	close(start)

	var got []*candidate
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil {
			t.Fatalf("concurrent allocateCandidate: %v", r.err)
		}
		got = append(got, r.c)
	}
	defer func() {
		for _, c := range got {
			_ = c.live.release()
		}
	}()
	if got[0].id == got[1].id {
		t.Errorf("concurrent allocations collided on id %q", got[0].id)
	}
}
