package transaction

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the interruption matrix from the spec's acceptance boundary:
// lost-response replay, cancellation at three points (inside Plan, between the
// local commit and the push, and before the first fetch), and engine-level
// materialization/verification failures. The single invariant every pre-push
// case proves: origin's ref is byte-identical before and after, and no candidate
// bytes ever reach the remote. Coordination is by channels and a barrier clock,
// never sleeps.

// barrierClock is the pinned instant every attempt stamps with, plus a
// deterministic seam: on its Nth Now() call it blocks until the test releases it.
// The engine reads the clock in a fixed order within one attempt —
// allocate(1), commit author date(2), setPhase(committed)(3), setPhase(pushed)(4)
// — so blocking on call 3 parks the attempt exactly AFTER the local commit exists
// and BEFORE PushLease is invoked. That makes "cancel between commit and push"
// deterministic: the test cancels the context while the engine is parked, so the
// push is launched with an already-dead context and never touches origin. A
// mis-count cannot yield a false green — blocking earlier is still a pre-push
// cancel (remote unchanged), and blocking on call 4 (post-push) would surface as
// an APPLIED disposition the test asserts against.
type barrierClock struct {
	base    time.Time
	target  int
	mu      sync.Mutex
	n       int
	reached chan struct{}
	release chan struct{}
}

func newBarrierClock(target int) *barrierClock {
	return &barrierClock{
		base:    time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC),
		target:  target,
		reached: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (c *barrierClock) Now() time.Time {
	c.mu.Lock()
	c.n++
	n := c.n
	c.mu.Unlock()
	if n == c.target {
		close(c.reached)
		<-c.release
	}
	return c.base
}

// blockingPlanOp blocks inside Plan until the context is cancelled, then returns
// ctx.Err() — the operation-observable barrier for the "cancel inside Plan" case.
type blockingPlanOp struct{ entered chan struct{} }

func (o *blockingPlanOp) Key() OperationKey { return "test.op" }

func (o *blockingPlanOp) Plan(ctx context.Context, _ AttemptState) (MutationPlan, OperationResult, error) {
	close(o.entered)
	<-ctx.Done()
	return MutationPlan{}, OperationResult{}, ctx.Err()
}

// soleCandidateWorktree returns the detached worktree path of the single candidate
// currently allocated under repo's transactions root — used while an attempt is
// parked on the barrier clock, when exactly one candidate exists.
func soleCandidateWorktree(t *testing.T, repo gitcli.Repository) string {
	t.Helper()
	root := transactionsRoot(repo)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read transactions root: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() && isOwnedTransactionID(e.Name()) {
			return filepath.Join(root, e.Name(), worktreeDirName)
		}
	}
	t.Fatal("no candidate worktree found under transactions root")
	return ""
}
