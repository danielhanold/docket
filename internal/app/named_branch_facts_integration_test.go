//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_contextprobe.sh (prefix ^TestIntegrationContextProbe).

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// --- real-git callers: claim, workspace context, merge context, clear-block --

// namedFactsRepo seeds B (from bSrc, plus an optional in-progress parent 4 on
// feat/b-parent) beside an unrelated stacked pair A (20 in-progress on
// feat/a-parent, 21 stacked on it).
func namedFactsRepo(t *testing.T, bSrc string, withParent bool) *gitRepo {
	t.Helper()
	files := map[string]string{
		groomPath(3, "widget"):    bSrc,
		groomPath(20, "a-parent"): lifecycleChange(20, "a-parent", "in-progress"),
		groomPath(21, "a-child"):  stackedOn(lifecycleChange(21, "a-child", "proposed"), 20),
	}
	if withParent {
		files[groomPath(4, "b-parent")] = lifecycleChange(4, "b-parent", "in-progress")
		files[groomPath(3, "widget")] = stackedOn(bSrc, 4)
	}
	return newWorkingRepo(t, files)
}

func assertPoisonRefusal(t *testing.T, what string, result Result, reason, msg string) {
	t.Helper()
	if result != ResultExternalFailed || reason != ReasonStatusExternal || !strings.Contains(msg, poisonProbe) {
		t.Fatalf("%s with B's own parent unprobeable = %q (%s: %s), want the external probe failure", what, result, reason, msg)
	}
}

func TestIntegrationContextProbeChangeClaimProbesOnlyOwnStack(t *testing.T) {
	requireRealGit(t)
	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		repo := namedFactsRepo(t, claimableChange(3, "widget"), false)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/a-parent")
		res := ChangeClaim(context.Background(), node.deps, node.dir,
			ChangeClaimRequest{ID: 3, Version: blobVersionAt(t, repo.origin, "docket", groomPath(3, "widget"))})
		if res.Result != ResultApplied {
			t.Fatalf("claim beside an unprobeable unrelated stack = %q (disposition %q findings %v), want applied", res.Result, res.Disposition, res.Findings)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		repo := namedFactsRepo(t, claimableChange(3, "widget"), true)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/b-parent")
		res := ChangeClaim(context.Background(), node.deps, node.dir,
			ChangeClaimRequest{ID: 3, Version: blobVersionAt(t, repo.origin, "docket", groomPath(3, "widget"))})
		msg := ""
		for _, f := range res.Findings {
			msg += f.Message
		}
		if res.Result != ResultExternalFailed || !strings.Contains(msg, poisonProbe) {
			t.Fatalf("claim with B's own parent unprobeable = %q (findings %v), want the external probe failure", res.Result, res.Findings)
		}
	})
}

func TestIntegrationContextProbeWorkspaceContextProbesOnlyOwnStack(t *testing.T) {
	requireRealGit(t)
	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), false)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/a-parent")
		if _, r := loadWorkspaceContext(context.Background(), node.deps, node.dir, 3, OperationWorkspaceInspect); r != nil {
			t.Fatalf("workspace context beside an unprobeable unrelated stack refused: %s: %s", r.Reason, r.Message)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), true)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/b-parent")
		_, r := loadWorkspaceContext(context.Background(), node.deps, node.dir, 3, OperationWorkspaceInspect)
		if r == nil {
			t.Fatal("workspace context with B's own parent unprobeable resolved; want the external probe failure")
		}
		assertPoisonRefusal(t, "workspace context", r.Result, r.Reason, r.Message)
	})
}

func TestIntegrationContextProbeMergeContextProbesOnlyOwnStack(t *testing.T) {
	requireRealGit(t)
	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), false)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/a-parent")
		if _, r := loadMergeContext(context.Background(), FinalizeDeps{Planning: node.deps}, node.dir, 3); r != nil {
			t.Fatalf("merge context beside an unprobeable unrelated stack refused: %s: %s", r.Reason, r.Message)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), true)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, "feat/b-parent")
		_, r := loadMergeContext(context.Background(), FinalizeDeps{Planning: node.deps}, node.dir, 3)
		if r == nil {
			t.Fatal("merge context with B's own parent unprobeable resolved; want the external probe failure")
		}
		assertPoisonRefusal(t, "merge context", r.Result, r.Reason, r.Message)
	})
}

func TestIntegrationContextProbeFinalizeClearBlockProbesOnlyOwnStack(t *testing.T) {
	requireRealGit(t)
	probeErr := errors.New("workspace probe reached")
	run := func(t *testing.T, withParent bool, poison string) BlockResult {
		t.Helper()
		repo := namedFactsRepo(t, lifecycleChange(3, "widget", "in-progress"), withParent)
		node := planningDepsFor(t, repo.invocation)
		node.deps.Reader = poisoned(node.deps.Reader, poison)
		deps := FinalizeDeps{
			Planning:  node.deps,
			GitHub:    repairGitHub("feat/widget"),
			Workspace: &fakeRepairWorkspace{inspectErr: probeErr},
		}
		return FinalizeClearBlock(context.Background(), deps, node.dir, ClearBlockRequest{
			ID: 3, Version: blobVersionAt(t, repo.origin, "docket", groomPath(3, "widget")), Head: prHead, PRNumber: 7,
		})
	}
	t.Run("unrelated-poison-does-not-block", func(t *testing.T) {
		// The reprobe proceeds past branch facts to the (scripted-failing)
		// workspace probe: the unrelated stack's branch was never asked for.
		if r := run(t, false, "feat/a-parent"); r.Reason != ReasonBlockWorkspaceProbe {
			t.Fatalf("clear-block beside an unprobeable unrelated stack = %q (%s: %s), want it to reach the workspace probe", r.Result, r.Reason, r.Message)
		}
	})
	t.Run("own-ancestor-poison-refuses", func(t *testing.T) {
		r := run(t, true, "feat/b-parent")
		assertPoisonRefusal(t, "clear-block", r.Result, r.Reason, r.Message)
	})
}
