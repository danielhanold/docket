//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"strings"
	"testing"
)

func TestIntegrationRecordOpsChangeReconcileUnrelatedInvalidRecordProgress(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	repo := newWorkingRepo(t, map[string]string{
		recPath:             reconcilableChange(id, "widget"),
		unrelatedBrokenPath: unrelatedBrokenBytes,
	})
	node := planningDepsFor(t, repo.invocation)

	res := ChangeReconcile(context.Background(), node.deps, node.dir, ChangeReconcileRequest{
		ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath), ReconcileLogEntry: "Reconciled against current reality.\n",
	})
	if res.Result != ResultApplied {
		t.Fatalf("reconcile beside an unrelated unparseable record = %q (disposition %q findings %v), want applied",
			res.Result, res.Disposition, res.Findings)
	}
	rec, _ := originFile(t, repo.origin, "docket", recPath)
	if !strings.Contains(rec, "Reconciled against current reality.") {
		t.Errorf("reconciled record on origin lacks the log entry:\n%s", rec)
	}
	assertUnrelatedBrokenIntact(t, repo)
}

func TestIntegrationRecordOpsChangeReconcileUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	for _, c := range unrelatedRefusalCases(t, id, recPath, reconcilableChange(id, "widget"), reconcilableChange(id, "dupe")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			node := planningDepsFor(t, repo.invocation)
			tip := originTip(t, repo.origin, "docket")

			res := ChangeReconcile(context.Background(), node.deps, node.dir, ChangeReconcileRequest{
				ID: id, Version: blobVersionAt(t, repo.origin, "docket", recPath), ReconcileLogEntry: "Reconciled.\n",
			})
			if res.Result == ResultApplied {
				t.Fatalf("reconcile applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, res.Disposition, res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused reconcile moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}
