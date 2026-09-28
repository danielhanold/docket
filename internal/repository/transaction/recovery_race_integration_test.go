//go:build integration

package transaction

import (
	"context"
	"path/filepath"
	"testing"
)

// TestRaceIntegrationTxnPruneRegistryLockSerializesAllocation proves the registry lock prevents
// PruneAbandoned from ever observing a half-published candidate directory. An
// allocator holds the registry lock across the whole mkdir→lock→manifest critical
// section; the prune, started while the lock is held, cannot list until the
// allocator releases — by which point the candidate is complete and live-locked, so
// it reports "live", never the "malformed" a half-published dir would yield. The
// ordering is enforced by channels and the lock, never a sleep.
// Race shard (change 0466): a goroutine holds the registry lock while PruneAbandoned contends for it.
func TestRaceIntegrationTxnPruneRegistryLockSerializesAllocation(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, _, repo := recoveryEngine(t, r)
	root := transactionsRoot(repo)

	inside := make(chan struct{})
	proceed := make(chan struct{})
	allocDone := make(chan struct{})
	var liveLock *fileLock
	var candID string

	go func() {
		_ = withRegistryLock(root, func() error {
			// Registry lock held. Announce, then wait until the prune is known to be
			// contending before publishing anything.
			close(inside)
			<-proceed
			id, err := newTransactionID()
			if err != nil {
				return err
			}
			candID = id
			candRoot := filepath.Join(root, id)
			if err := mkdirMode(candRoot, txnDirMode); err != nil {
				return err
			}
			if err := mkdirMode(filepath.Join(candRoot, hooksDirName), txnDirMode); err != nil {
				return err
			}
			lk, err := acquireLock(filepath.Join(candRoot, liveLockName), false)
			if err != nil {
				return err
			}
			liveLock = lk
			return writeManifestAtomic(candRoot, baseValidManifest(id, repo.CommonDir))
		})
		close(allocDone)
	}()

	<-inside
	reportCh := make(chan PruneReport, 1)
	errCh := make(chan error, 1)
	go func() {
		rep, err := eng.PruneAbandoned(context.Background(), repo)
		errCh <- err
		reportCh <- rep
	}()

	// The prune goroutine is now blocked on the registry lock (it started after the
	// allocator signalled `inside` and holds it). Release the allocator; only then
	// can the prune list — and it must see a complete, live-locked candidate.
	close(proceed)
	<-allocDone

	if err := <-errCh; err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	rep := <-reportCh
	defer func() { _ = liveLock.release() }()

	if len(rep.Entries) != 1 {
		t.Fatalf("report entries = %+v, want exactly one", rep.Entries)
	}
	e := rep.Entries[0]
	if e.ID != candID || e.Verdict != verdictLive {
		t.Errorf("entry = %+v, want live entry for %s (a half-published dir would read malformed)", e, candID)
	}
}
