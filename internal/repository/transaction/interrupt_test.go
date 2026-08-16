package transaction

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// TestInterruptLostResponseReplaysOriginalOnce proves a lost response is safe: a
// keyed allocating operation applies once; re-running the SAME request against a
// FRESH engine returns already-applied with the original receipt and commit, adds
// no commit, and leaves exactly one matching receipt in origin history.
func TestInterruptLostResponseReplaysOriginalOnce(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	ctx := context.Background()

	planted := []byte(`{"id":"0003"}`)
	op := createOp(thirdChangePath, thirdChange())
	op.receipt = planted

	res1, err := newEngine(t, client).Execute(ctx, Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: op,
	})
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	if res1.Disposition != DispositionApplied {
		t.Fatalf("first disposition = %q, want applied", res1.Disposition)
	}
	original := res1.AppliedCommit
	tipAfterApply := r.originTip(t)

	// The client observed no response; the caller retries the exact request against a
	// brand-new engine, as a resumed process would.
	replayOp := createOp(thirdChangePath, thirdChange())
	replayOp.receipt = []byte(`{"id":"9999"}`) // a re-run from scratch would surface this
	res2, err := newEngine(t, client).Execute(ctx, Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: replayOp,
	})
	if err != nil {
		t.Fatalf("replay Execute: %v", err)
	}
	if res2.Disposition != DispositionAlreadyApplied {
		t.Fatalf("replay disposition = %q, want already-applied", res2.Disposition)
	}
	if res2.AppliedCommit != original {
		t.Errorf("replay commit = %q, want original %q", res2.AppliedCommit, original)
	}
	if string(res2.Receipt) != string(planted) {
		t.Errorf("replay receipt = %q, want original %q", res2.Receipt, planted)
	}
	if replayOp.calls != 0 {
		t.Errorf("replay planned %d times; a replay must not replan", replayOp.calls)
	}
	if r.originTip(t) != tipAfterApply {
		t.Error("replay added a commit to origin")
	}

	// Exactly one commit in origin history carries this request id.
	cts, err := client.ScanCommitTrailers(ctx, repo, r.originTip(t), []string{trailerRequestID})
	if err != nil {
		t.Fatalf("ScanCommitTrailers: %v", err)
	}
	count := 0
	for _, ct := range cts {
		for _, tr := range ct.Trailers {
			if tr.Key == trailerRequestID && tr.Value == keyReq().RequestID {
				count++
			}
		}
	}
	if count != 1 {
		t.Errorf("matching receipt commits = %d, want exactly 1", count)
	}
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

// TestInterruptCancelInsidePlan proves cancelling the context while the operation
// is planning yields an interrupted disposition and leaves origin untouched — a
// pre-push cancellation never changes the remote.
func TestInterruptCancelInsidePlan(t *testing.T) {
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

// TestInterruptCancelBetweenCommitAndPush proves that cancelling after the local
// commit exists but before the push leaves origin byte-identical: the push is
// launched with a dead context and never reaches the remote. The barrier clock
// parks the attempt on the setPhase(committed) stamp — after CommitPaths, before
// PushLease — which is the deterministic pre-push seam.
func TestInterruptCancelBetweenCommitAndPush(t *testing.T) {
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

// TestInterruptPreCancelledContext proves a context already cancelled before the
// first fetch yields an interrupted result with no allocation and no remote
// change.
func TestInterruptPreCancelledContext(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	base := r.originTip(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := eng.Execute(ctx, Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
	})
	if res.Disposition != DispositionInterrupted {
		t.Fatalf("disposition = %q, want interrupted", res.Disposition)
	}
	assertFailureKind(t, err, KindCancelled)
	if r.originTip(t) != base {
		t.Error("origin advanced on a pre-cancelled context")
	}
	if !transactionsEmpty(t, repo) {
		t.Error("a candidate was allocated despite a pre-cancelled context")
	}
}

// TestInterruptDeltaMismatchDoesNotPush proves an engine-level plan whose declared
// bytes equal the base — so Git sees no delta — fails at the delta guard and never
// pushes; origin is untouched.
func TestInterruptDeltaMismatchDoesNotPush(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	base := r.originTip(t)

	// Replace record 1 with its EXACT current bytes: a non-empty plan producing no
	// actual Git delta — the spec's "plan did not describe reality".
	same := corpusChange(1, "first-change", "proposed")
	op := &scriptedOp{files: []FileMutation{
		{Path: "docs/changes/active/0001-first-change.md", Kind: MutationReplace, Bytes: []byte(same)},
	}}
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: op,
	})
	if err == nil {
		t.Fatal("delta-mismatch plan: want a Go *Failure")
	}
	f := assertMaterializeFailure(t, err, StageVerifyDelta)
	if f.Kind != KindInvalidState {
		t.Errorf("failure kind = %q, want invalid-state", f.Kind)
	}
	if res.Disposition != DispositionFailed {
		t.Errorf("disposition = %q, want failed", res.Disposition)
	}
	if r.originTip(t) != base {
		t.Error("origin advanced on a delta-mismatch failure")
	}
}

// TestInterruptContainmentFailureDoesNotPush proves an engine-level plan whose
// declared file has a non-directory parent component is refused at materialize and
// never pushes, leaving origin and the offending base file untouched.
func TestInterruptContainmentFailureDoesNotPush(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	base := r.originTip(t)

	// README.md is a regular file in the base tree; a create beneath it has a
	// non-directory parent component, which materialize must refuse. It lives
	// outside docs/, so the loader never parses it and the failure is containment,
	// not validation.
	op := &scriptedOp{files: []FileMutation{
		{Path: "README.md/child.md", Kind: MutationCreate, Bytes: []byte("planted\n")},
	}}
	res, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Loader: testLoader{}, Operation: op,
	})
	if err == nil {
		t.Fatal("containment-violating plan: want a Go *Failure")
	}
	assertMaterializeFailure(t, err, StageMaterialize)
	if res.Disposition != DispositionFailed {
		t.Errorf("disposition = %q, want failed", res.Disposition)
	}
	if r.originTip(t) != base {
		t.Error("origin advanced on a containment failure")
	}
	// README.md is still a regular file with its original bytes in the checkout.
	readme := filepath.Join(repo.PrimaryWorktree, "README.md")
	fi, lerr := os.Lstat(readme)
	if lerr != nil || !fi.Mode().IsRegular() {
		t.Errorf("README.md no longer a regular file: mode=%v err=%v", fi.Mode(), lerr)
	}
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

// TestInterruptLiteralLeaseRejectsFresherTrackingRef proves the push lease is
// pinned to the exact base the operation read, not to the clone's remote-tracking
// ref. The barrier clock parks the attempt after the local commit; the test then
// advances origin to a DIVERGENT commit AND updates the engine clone's tracking
// ref to match. A bare/implicit lease would read the fresh tracking ref, find it
// equal to the remote, and force-push — silently clobbering the concurrent
// writer. The literal lease, pinned to the stale base, correctly loses, retries on
// the new base, and both changes converge.
func TestInterruptLiteralLeaseRejectsFresherTrackingRef(t *testing.T) {
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

// TestInterruptAmbiguousPushLandedIsApplied proves the post-push probe: when the
// lease push is rejected but the engine's own commit is nonetheless reachable from
// the advanced remote — the "ambiguous response where the write actually landed" —
// the engine classifies the transaction APPLIED, not failed. The barrier clock
// parks the attempt after the local commit; the test then publishes a DESCENDANT
// of that exact commit to origin (a fast-forward built with commit-tree, touching
// no checkout), so the engine's subsequent lease push loses the lease yet its
// commit is a proven ancestor of the new remote tip.
func TestInterruptAmbiguousPushLandedIsApplied(t *testing.T) {
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
