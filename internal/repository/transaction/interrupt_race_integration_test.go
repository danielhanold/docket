//go:build integration

package transaction

import (
	"context"
	"strings"
	"testing"
)

// TestRaceIntegrationTxnInterruptCancelInsidePlan proves cancelling the context while the operation
// is planning yields an interrupted disposition and leaves origin untouched — a
// pre-push cancellation never changes the remote.
// Race shard (change 0466): Execute runs in a goroutine while the test goroutine cancels or races it through shared hooks.
func TestRaceIntegrationTxnInterruptCancelInsidePlan(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	base := r.originTip(t)

	ctx, cancel := context.WithCancel(context.Background())
	op := &blockingPlanOp{entered: make(chan struct{})}

	var res Result
	var rerr error
	done := make(chan struct{})
	go func() {
		res, rerr = eng.Execute(ctx, Request{
			Repository: repo, Remote: "origin", TargetRef: r.Target,
			Loader: testLoader{}, Operation: op,
		})
		close(done)
	}()

	<-op.entered
	cancel()
	<-done

	if res.Disposition != DispositionInterrupted {
		t.Fatalf("disposition = %q, want interrupted", res.Disposition)
	}
	assertFailureKind(t, rerr, KindCancelled)
	if r.originTip(t) != base {
		t.Error("origin advanced on a cancellation inside Plan")
	}
}

// TestRaceIntegrationTxnInterruptCancelBetweenCommitAndPush proves that cancelling after the local
// commit exists but before the push leaves origin byte-identical: the push is
// launched with a dead context and never reaches the remote. The barrier clock
// parks the attempt on the setPhase(committed) stamp — after CommitPaths, before
// PushLease — which is the deterministic pre-push seam.
// Race shard (change 0466): Execute runs in a goroutine while the test goroutine cancels or races it through shared hooks.
func TestRaceIntegrationTxnInterruptCancelBetweenCommitAndPush(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	base := r.originTip(t)

	clk := newBarrierClock(3) // 3rd clock read == setPhase(committed), immediately pre-push
	eng, err := NewEngine(client, clk)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	op := createOp(thirdChangePath, thirdChange())

	var res Result
	var rerr error
	done := make(chan struct{})
	go func() {
		res, rerr = eng.Execute(ctx, Request{
			Repository: repo, Remote: "origin", TargetRef: r.Target,
			Loader: testLoader{}, Operation: op,
		})
		close(done)
	}()

	<-clk.reached // the local commit exists; the push has not been launched
	cancel()
	close(clk.release)
	<-done

	if res.Disposition != DispositionInterrupted {
		t.Fatalf("disposition = %q, want interrupted (a pre-push cancel must not apply)", res.Disposition)
	}
	assertFailureKind(t, rerr, KindCancelled)
	if r.originTip(t) != base {
		t.Error("origin advanced despite a cancellation before the push")
	}
}

// TestRaceIntegrationTxnInterruptLiteralLeaseRejectsFresherTrackingRef proves the push lease is
// pinned to the exact base the operation read, not to the clone's remote-tracking
// ref. The barrier clock parks the attempt after the local commit; the test then
// advances origin to a DIVERGENT commit AND updates the engine clone's tracking
// ref to match. A bare/implicit lease would read the fresh tracking ref, find it
// equal to the remote, and force-push — silently clobbering the concurrent
// writer. The literal lease, pinned to the stale base, correctly loses, retries on
// the new base, and both changes converge.
// Race shard (change 0466): Execute runs in a goroutine while the test goroutine cancels or races it through shared hooks.
func TestRaceIntegrationTxnInterruptLiteralLeaseRejectsFresherTrackingRef(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)

	clk := newBarrierClock(3)
	eng, err := NewEngine(client, clk)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	op := createOp(thirdChangePath, thirdChange())
	var res Result
	var rerr error
	done := make(chan struct{})
	go func() {
		res, rerr = eng.Execute(context.Background(), Request{
			Repository: repo, Remote: "origin", TargetRef: r.Target,
			Loader: testLoader{}, Operation: op,
		})
		close(done)
	}()

	<-clk.reached
	// Origin advances to a divergent commit (a different record), and the engine
	// clone's remote-tracking ref is fast-forwarded to it. A bare lease would now be
	// satisfied and force-clobber; the literal lease pinned to the stale base must
	// not be.
	writerRec := "docs/changes/active/0008-writer-b.md"
	r.advanceOrigin(t, writerRec, corpusChange(8, "writer-b", "proposed"))
	hgitOut(t, repo.PrimaryWorktree, "fetch", "-q", "origin", r.short())
	close(clk.release)
	<-done

	if rerr != nil {
		t.Fatalf("Execute: %v", rerr)
	}
	if res.Disposition != DispositionApplied {
		t.Fatalf("disposition = %q, want applied (literal lease loses then reapplies)", res.Disposition)
	}
	if res.Attempts != 2 {
		t.Errorf("attempts = %d, want 2 (one literal-lease loss then apply)", res.Attempts)
	}
	// Both changes converged: the concurrent writer's record was NOT clobbered.
	names := hgitOut(t, r.Origin, "ls-tree", "-r", "--name-only", string(r.Target))
	for _, want := range []string{thirdChangePath, writerRec} {
		if !strings.Contains(names, want) {
			t.Errorf("origin missing %q — the concurrent writer's change was clobbered:\n%s", want, names)
		}
	}
}

// TestRaceIntegrationTxnInterruptAmbiguousPushLandedIsApplied proves the post-push probe: when the
// lease push is rejected but the engine's own commit is nonetheless reachable from
// the advanced remote — the "ambiguous response where the write actually landed" —
// the engine classifies the transaction APPLIED, not failed. The barrier clock
// parks the attempt after the local commit; the test then publishes a DESCENDANT
// of that exact commit to origin (a fast-forward built with commit-tree, touching
// no checkout), so the engine's subsequent lease push loses the lease yet its
// commit is a proven ancestor of the new remote tip.
// Race shard (change 0466): Execute runs in a goroutine while the test goroutine cancels or races it through shared hooks.
func TestRaceIntegrationTxnInterruptAmbiguousPushLandedIsApplied(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)

	clk := newBarrierClock(3) // park on setPhase(committed): the commit exists, push has not run
	eng, err := NewEngine(client, clk)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	var res Result
	var rerr error
	done := make(chan struct{})
	go func() {
		res, rerr = eng.Execute(context.Background(), Request{
			Repository: repo, Remote: "origin", TargetRef: r.Target,
			Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
		})
		close(done)
	}()

	<-clk.reached
	// The engine's commit exists in the candidate worktree (shared object store with
	// the invocation clone). Publish a descendant of it to origin so the pending push
	// loses its lease but the commit stays reachable from the new tip.
	wt := soleCandidateWorktree(t, repo)
	engineCommit := hgitOut(t, wt, "rev-parse", "HEAD")
	tree := hgitOut(t, repo.PrimaryWorktree, "rev-parse", engineCommit+"^{tree}")
	descendant := hgitOut(t, repo.PrimaryWorktree, "commit-tree", tree, "-p", engineCommit, "-m", "ambiguous descendant")
	hgitOut(t, repo.PrimaryWorktree, "push", "origin", descendant+":"+string(r.Target))
	close(clk.release)
	<-done

	if rerr != nil {
		t.Fatalf("Execute returned a Go error: %v", rerr)
	}
	if res.Disposition != DispositionApplied {
		t.Fatalf("disposition = %q, want applied (the commit landed and is reachable)", res.Disposition)
	}
	if string(res.AppliedCommit) != engineCommit {
		t.Errorf("applied commit = %q, want the engine's commit %q", res.AppliedCommit, engineCommit)
	}
	if got := string(r.originTip(t)); got != descendant {
		t.Errorf("origin tip = %q, want the published descendant %q", got, descendant)
	}
	if !transactionsEmpty(t, repo) {
		t.Error("transactions root not empty after an applied ambiguous push")
	}
}
