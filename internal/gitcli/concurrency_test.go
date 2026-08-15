package gitcli

import (
	"bytes"
	"context"
	"sync"
	"testing"
)

// TestConcurrentOperationsShareClientAndSourceSafely proves a Client and an open
// ObjectSource are safe for concurrent use by independent callers, which is how
// the application layer will hold them: one client per process, one source per
// pinned revision, many in-flight reads.
//
// The adapter keeps no mutable per-call state — the sanitized environment and
// the pinned revision are built once and only read afterwards — and this test is
// what holds that property in place. Under `go test -race` it is also the
// detector for any future field that starts being written mid-call; without
// -race it still catches cross-talk, where a concurrent call corrupts another's
// results rather than racing on memory.
//
// Every goroutine's answer is compared against the same serially-computed
// golden, so a result assembled from another goroutine's request is a failure
// even when the race detector sees nothing.
func TestConcurrentOperationsShareClientAndSourceSafely(t *testing.T) {
	requireGit(t)
	repos := newMainModeRepos(t)
	c := newRealClient(t)
	ctx := context.Background()

	repo := mustDiscover(t, c, repos.Invocation)
	commit, err := c.ResolveRef(ctx, repo, "refs/heads/main")
	if err != nil {
		t.Fatalf("ResolveRef: %v", err)
	}
	src, err := c.OpenObjectSource(ctx, repo, Revision{Commit: commit, Remote: "origin", Ref: "refs/heads/main"})
	if err != nil {
		t.Fatalf("OpenObjectSource: %v", err)
	}

	// Serial goldens: whatever a lone caller sees is what every concurrent
	// caller must see.
	wantTree, err := src.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("golden ListTree: %v", err)
	}
	blobPaths := []RepoPath{"README.md", ".docket.yml", "docs/changes/active/0001-a.md"}
	wantBlobs, err := src.ReadBlobs(ctx, blobPaths)
	if err != nil {
		t.Fatalf("golden ReadBlobs: %v", err)
	}

	const goroutines = 8
	const rounds = 3
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*rounds*3)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				gotTree, err := src.ListTree(ctx, nil)
				if err != nil {
					errs <- err
					return
				}
				if !treeEntriesEqual(gotTree, wantTree) {
					errs <- errConcurrentMismatch("ListTree")
					return
				}

				gotBlobs, err := src.ReadBlobs(ctx, blobPaths)
				if err != nil {
					errs <- err
					return
				}
				if !blobResultsEqual(gotBlobs, wantBlobs) {
					errs <- errConcurrentMismatch("ReadBlobs")
					return
				}

				// A second Client surface in flight at the same time, so the
				// shared sanitized environment is read concurrently too.
				gotCommit, err := c.ResolveRef(ctx, repo, "refs/heads/main")
				if err != nil {
					errs <- err
					return
				}
				if gotCommit != commit {
					errs <- errConcurrentMismatch("ResolveRef")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// errConcurrentMismatch names the operation whose concurrent answer diverged
// from the serial golden.
type errConcurrentMismatch string

func (e errConcurrentMismatch) Error() string {
	return string(e) + ": concurrent result differs from the serial golden"
}

// treeEntriesEqual compares two tree listings element-wise; ListTree returns a
// deterministic order, so order is part of the comparison.
func treeEntriesEqual(a, b []TreeEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// blobResultsEqual compares two blob-result slices including exact bytes, so a
// payload assembled from another goroutine's request is caught.
func blobResultsEqual(a, b []BlobResult) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path || a[i].Found != b[i].Found {
			return false
		}
		if a[i].Blob.ObjectID != b[i].Blob.ObjectID || a[i].Blob.Mode != b[i].Blob.Mode {
			return false
		}
		if !bytes.Equal(a[i].Blob.Bytes, b[i].Blob.Bytes) {
			return false
		}
	}
	return true
}
