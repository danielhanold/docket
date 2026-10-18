//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"testing"
)

func TestIntegrationRecordOpsChangeLifecycleUnrelatedInvalidRecordProgress(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	rows := []struct {
		name, from, want string
		run              func(node realNode, revision string) ChangeLifecycleResult
	}{
		{name: "block", from: "in-progress", want: "blocked", run: func(node realNode, revision string) ChangeLifecycleResult {
			return ChangeBlock(context.Background(), node.deps, node.dir,
				ChangeBlockRequest{ChangeID: id, Path: recPath, Revision: revision, Reason: "waiting on upstream"})
		}},
		{name: "defer", from: "proposed", want: "deferred", run: func(node realNode, revision string) ChangeLifecycleResult {
			return ChangeDefer(context.Background(), node.deps, node.dir,
				ChangeDeferRequest{ChangeID: id, Path: recPath, Revision: revision, WhyDeferred: "Parked pending a decision.\n"})
		}},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			repo := newWorkingRepo(t, map[string]string{
				recPath:             lifecycleChange(id, "widget", r.from),
				unrelatedBrokenPath: unrelatedBrokenBytes,
			})
			node := planningDepsFor(t, repo.invocation)
			res := r.run(node, blobRevisionAt(t, repo.origin, "docket", recPath))
			if res.Result != ResultApplied || res.Status != r.want {
				t.Fatalf("%s beside an unrelated unparseable record = %q status %q (findings %v), want applied %q",
					r.name, res.Result, res.Status, res.Findings, r.want)
			}
			assertUnrelatedBrokenIntact(t, repo)
		})
	}
}

func TestIntegrationRecordOpsChangeLifecycleUnrelatedInvalidRecordRefusals(t *testing.T) {
	requireRealGit(t)
	const id = 3
	recPath := groomPath(id, "widget")
	for _, c := range unrelatedRefusalCases(t, id, recPath, lifecycleChange(id, "widget", "in-progress"), lifecycleChange(id, "dupe", "in-progress")) {
		t.Run(c.name, func(t *testing.T) {
			repo := newWorkingRepo(t, c.files)
			node := planningDepsFor(t, repo.invocation)
			tip := originTip(t, repo.origin, "docket")

			res := ChangeBlock(context.Background(), node.deps, node.dir, ChangeBlockRequest{
				ChangeID: id, Path: recPath, Revision: blobRevisionAt(t, repo.origin, "docket", recPath), Reason: "waiting on upstream",
			})
			if res.Result == ResultApplied {
				t.Fatalf("block applied despite %s; want a refusal", c.name)
			}
			assertRefusalBeyondUnrelated(t, "", res.Findings)
			if got := originTip(t, repo.origin, "docket"); got != tip {
				t.Errorf("a refused block moved the metadata branch %s -> %s", tip, got)
			}
		})
	}
}

// A kill of an in-progress stack parent whose stacked child is already built is
// refused end to end through the real engine: invalid-state with the
// built-stack-descendants finding, no metadata commit, and both records left
// byte-for-byte as they were.
func TestIntegrationRecordOpsChangeKillRefusesBuiltStackedChild(t *testing.T) {
	requireRealGit(t)
	parentPath := groomPath(3, "widget")
	childPath := groomPath(4, "child")
	repo := newWorkingRepo(t, map[string]string{
		parentPath: lifecycleChange(3, "widget", "in-progress"),
		childPath:  builtChild("implemented"),
	})
	node := planningDepsFor(t, repo.invocation)
	tip := originTip(t, repo.origin, "docket")
	parentBefore, _ := originFile(t, repo.origin, "docket", parentPath)
	childBefore, _ := originFile(t, repo.origin, "docket", childPath)

	res := ChangeKill(context.Background(), node.deps, node.dir, ChangeKillRequest{
		ChangeID: 3, Path: parentPath, Revision: blobRevisionAt(t, repo.origin, "docket", parentPath),
		WhyKilled: "Dropped.\n",
	})
	if res.Result != ResultInvalidState {
		t.Fatalf("kill over a built stacked child = %q (findings %v), want invalid-state", res.Result, res.Findings)
	}
	found := false
	for _, f := range res.Findings {
		if f.Code == "built-stack-descendants" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing built-stack-descendants finding; got %v", res.Findings)
	}
	if got := originTip(t, repo.origin, "docket"); got != tip {
		t.Errorf("a refused kill moved the metadata branch %s -> %s", tip, got)
	}
	if got, ok := originFile(t, repo.origin, "docket", parentPath); !ok || got != parentBefore {
		t.Errorf("parent record changed by a refused kill (present %v)", ok)
	}
	if got, ok := originFile(t, repo.origin, "docket", childPath); !ok || got != childBefore {
		t.Errorf("child record changed by a refused kill (present %v)", ok)
	}
}
