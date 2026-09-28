//go:build integration

package transaction

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestIntegrationTxnRecoveryInterruptLostResponseReplaysOriginalOnce proves a lost response is safe: a
// keyed allocating operation applies once; re-running the SAME request against a
// FRESH engine returns already-applied with the original receipt and commit, adds
// no commit, and leaves exactly one matching receipt in origin history.
func TestIntegrationTxnRecoveryInterruptLostResponseReplaysOriginalOnce(t *testing.T) {
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

// TestIntegrationTxnRecoveryInterruptPreCancelledContext proves a context already cancelled before the
// first fetch yields an interrupted result with no allocation and no remote
// change.
func TestIntegrationTxnRecoveryInterruptPreCancelledContext(t *testing.T) {
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

// TestIntegrationTxnRecoveryInterruptDeltaMismatchDoesNotPush proves an engine-level plan whose declared
// bytes equal the base — so Git sees no delta — fails at the delta guard and never
// pushes; origin is untouched.
func TestIntegrationTxnRecoveryInterruptDeltaMismatchDoesNotPush(t *testing.T) {
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

// TestIntegrationTxnRecoveryInterruptContainmentFailureDoesNotPush proves an engine-level plan whose
// declared file has a non-directory parent component is refused at materialize and
// never pushes, leaving origin and the offending base file untouched.
func TestIntegrationTxnRecoveryInterruptContainmentFailureDoesNotPush(t *testing.T) {
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
