//go:build integration

package app

import (
	"context"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// completeEngineDeps wires ChangeComplete over a real repository (Discover needs
// one) with an empty corpus, so no replay short-circuits the transaction, and a
// recording engine scripted to return res.
func completeEngineDeps(t *testing.T, res transaction.Result) (PlanningDeps, string, *recordingEngine) {
	t.Helper()
	requireRealGit(t)
	repoDir := newWorkingRepo(t, nil).invocation
	engine := &recordingEngine{result: res}
	reader := &fakeChangeReader{pin: mainModePin([]string{"inline"})}
	return PlanningDeps{Client: newGitClient(t), Engine: engine, Reader: reader, Clock: testClock()}, repoDir, engine
}

// TestIntegrationChangeRuntimeCompleteRevisionMismatchContends: a stale
// submitted revision loses the exact-blob pin, which the engine reports as
// contended; the result carries the contended disposition and no archive path.
func TestIntegrationChangeRuntimeCompleteRevisionMismatchContends(t *testing.T) {
	deps, repoDir, engine := completeEngineDeps(t, transaction.Result{Disposition: transaction.DispositionContended})

	res := ChangeComplete(context.Background(), deps, repoDir, validCompleteRequest())

	if res.Result != ResultContended || res.Disposition != CompleteDispContended {
		t.Fatalf("result = (%q, %q), want (contended, contended); findings %v", res.Result, res.Disposition, res.Findings)
	}
	if res.ArchivePath != "" || res.Revision != "" {
		t.Errorf("a contended completion reported a write: archive %q revision %q", res.ArchivePath, res.Revision)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	exp := engine.calls[0].Expected
	if len(exp) != 1 || exp[0].Path != gitcli.RepoPath(groomPath(3, miSlug)) ||
		exp[0].Revision.Kind != transaction.RevisionBlob || exp[0].Revision.ObjectID != gitcli.ObjectID(blobV) {
		t.Errorf("engine expectation = %+v, want the exact submitted blob pin", exp)
	}
}

// TestIntegrationChangeRuntimeCompleteUsesScopedValidation: the completion runs
// under the named change's validation scope, so an unrelated record's findings
// cannot veto it.
func TestIntegrationChangeRuntimeCompleteUsesScopedValidation(t *testing.T) {
	deps, repoDir, engine := completeEngineDeps(t, transaction.Result{
		Disposition:   transaction.DispositionApplied,
		AppliedCommit: "cafebabecafebabecafebabecafebabecafebabe",
		Receipt:       mustMarshal(t, changeCompleteReceipt{ArchivePath: killArchivePath(3, miSlug), ID: 3, Op: OperationChangeComplete}),
	})

	res := ChangeComplete(context.Background(), deps, repoDir, validCompleteRequest())

	if res.Result != ResultApplied || res.Disposition != CompleteDispCompleted {
		t.Fatalf("result = (%q, %q), want (applied, completed); findings %v", res.Result, res.Disposition, res.Findings)
	}
	if len(engine.calls) != 1 {
		t.Fatalf("engine calls = %d, want 1", len(engine.calls))
	}
	req := engine.calls[0]
	if req.Scope == nil {
		t.Fatal("the transaction request carries no validation scope; want changeScope for the named change")
	}
	if req.Operation.Key() != OperationChangeComplete {
		t.Errorf("operation key = %q, want %q", req.Operation.Key(), OperationChangeComplete)
	}
	if want := "change 0003 completed (archived " + killArchivePath(3, miSlug) + ") — cafebabecafebabecafebabecafebabecafebabe"; res.HumanText() != want {
		t.Errorf("HumanText = %q, want %q", res.HumanText(), want)
	}
}
