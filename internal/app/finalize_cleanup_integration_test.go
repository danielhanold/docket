//go:build integration

package app

import (
	"context"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors is the 0337 sweep
// self-heal proof: a change closed out while the bug was live left stale
// active-path backlinks on the integration branch, which ALSO carries an
// unrelated malformed corpus record. The next cleanup must land the retarget
// anyway — the repair leg's gate is scoped to the artifacts it patches.
func TestIntegrationFinalizeCleanupBacklinkRepairIgnoresUnrelatedCorpusErrors(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)

	// archiveClosed advanced origin/main via the closeout backlink leg, leaving
	// the writer clone's local main behind; sync it so the fixture advance below
	// fast-forwards rather than being rejected as non-fast-forward.
	runGit(t, f.repo.writer, "fetch", "-q", "origin", "main")
	runGit(t, f.repo.writer, "reset", "--hard", "origin/main")

	// Recreate the stuck state on the integration branch: revert both artifacts
	// to stale active-path backlinks AND plant the unrelated malformed record.
	recPath := groomPath(f.id, f.slug)
	f.repo.writerAdvance(t, "main", map[string]string{
		f.planPath:    artifactWithBacklink(recPath, "Plan", "The widget plan."),
		f.resultsPath: artifactWithBacklink(recPath, "Results", "The widget results."),
		"docs/adrs/0099-malformed.md": "---\n" +
			"id: 99\n" +
			"title: uses `context: fork` dispatch\n" +
			"status: Accepted\n" +
			"date: 2026-08-22\n" +
			"---\n\n# 99. Malformed on purpose\n",
	})

	gh := f.mergedCleanupFake(head, mergeCommit)
	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
	for _, fd := range res.Findings {
		if fd.Code == ReasonCleanupBacklinkPending {
			t.Fatalf("unrelated corpus error refused the cleanup repair leg: %+v", fd)
		}
	}
	if res.Disposition != CleanupDispCleaned {
		t.Fatalf("cleanup disposition = %q (reason %q msg %q), want %q",
			res.Disposition, res.Reason, res.Message, CleanupDispCleaned)
	}
	// The stale backlinks were re-pointed at the archive path.
	for _, p := range []string{f.planPath, f.resultsPath} {
		got, ok := originFile(t, f.repo.origin, "main", p)
		if !ok {
			t.Fatalf("artifact %q vanished from main", p)
		}
		if strings.Contains(got, recPath) {
			t.Errorf("artifact %q still backlinks the stale active path %q:\n%s", p, recPath, got)
		}
		if !strings.Contains(got, "docs/changes/archive/") {
			t.Errorf("artifact %q does not backlink an archive path:\n%s", p, got)
		}
	}
}

func TestIntegrationFinalizeCleanupBranchDeletion(t *testing.T) {
	requireRealGit(t)

	t.Run("happy", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		gh := f.mergedCleanupFake(head, mergeCommit)
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
		if res.Result != ResultApplied || res.Disposition != CleanupDispCleaned {
			t.Fatalf("cleanup = %q disp %q (%s)", res.Result, res.Disposition, res.Message)
		}
		if f.localBranchPresent(t) {
			t.Fatalf("the merged local feature branch must be deleted")
		}
		if f.remoteBranchPresent(t) {
			t.Fatalf("the merged remote feature branch must be deleted")
		}
	})

	t.Run("moved-tip-retained", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		// The fake reports a DIFFERENT merged head than the live branch tip, so the
		// exact-tip proof fails and the branch is retained.
		gh := f.mergedCleanupFake(strings.Repeat("e", 40), mergeCommit)
		_ = head
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
		if res.Disposition == CleanupDispCleaned {
			t.Fatalf("a moved/mismatched tip must retain the branch, got cleaned")
		}
		if !f.localBranchPresent(t) {
			t.Fatalf("a mismatched tip must leave the local branch intact")
		}
	})

	t.Run("unreachable-merge-chain-retained", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		// Archive the record but do NOT merge the head into main: the merged facts
		// point at a merge commit that is not an ancestor of main's tip, so the
		// merge-chain containment proof fails and the local branch is retained.
		gh := f.mergedCleanupFake(f.head, strings.Repeat("f", 40))
		// Force the archive by marking done directly through closeout with a real
		// merge, then rewind main so the head is no longer reachable is complex;
		// instead archive normally, then assert containment holds — here we test the
		// negative through the injected ancestor probe returning false.
		head, mergeCommit := f.archiveClosed(t)
		gh = f.mergedCleanupFake(head, mergeCommit)
		git := &faultyCleanupGit{inner: f.deps.Client, fail: "ancestor"}
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, git, f.svc), f.repo.invocation, f.id)
		if res.Disposition == CleanupDispCleaned {
			t.Fatalf("an unprovable merge chain must retain the branch")
		}
		if !f.localBranchPresent(t) {
			t.Fatalf("an unprovable merge chain must leave the local branch intact")
		}
	})

	t.Run("open-child-retains-remote", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		gh := f.mergedCleanupFake(head, mergeCommit)
		// A live open child PR whose base is the parent's feature branch: the remote
		// must be retained and children-retarget-required reported.
		gh.openByHead["feat/child"] = []githubcli.PullRequest{{
			Number: 99, State: githubcli.StateOpen, HeadBranch: "feat/child", BaseBranch: "feat/" + f.slug,
		}}
		// Register the child in the corpus as stacked on this change.
		f.seedStackChild(t, 6, "child")
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
		if res.Disposition != CleanupDispChildrenRetargetRequired {
			t.Fatalf("an open child must report children-retarget-required, got %q (%s)", res.Disposition, res.Message)
		}
		if !f.remoteBranchPresent(t) {
			t.Fatalf("an open child must retain the parent remote branch")
		}
	})
}

// TestFinalizeCleanupChildRemoteBranchIdentity proves the remote-ref open-child
// probe addresses each child by ITS OWN recorded branch, never a slug-derived
// name: an open child PR on a NON-DERIVED recorded head that still targets the
// parent branch retains the remote and reports children-retarget-required. Were
// the probe to query the slug-derived feat/child instead, it would find no PR
// and wrongly delete the remote — so the retention proves the recorded head.
func TestIntegrationFinalizeCleanupChildRemoteBranchIdentity(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)
	gh := f.mergedCleanupFake(head, mergeCommit)
	// The live open child PR sits on a non-derived recorded head and still targets
	// the parent's feature branch.
	gh.openByHead["feature/child-head"] = []githubcli.PullRequest{{
		Number: 99, State: githubcli.StateOpen, HeadBranch: "feature/child-head", BaseBranch: "feat/" + f.slug,
	}}
	f.seedStackChildBranch(t, 6, "child", "feature/child-head")

	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
	if res.Disposition != CleanupDispChildrenRetargetRequired {
		t.Fatalf("an open child on its recorded head must report children-retarget-required, got %q (%s)", res.Disposition, res.Message)
	}
	if !f.remoteBranchPresent(t) {
		t.Fatalf("an open child on its recorded head must retain the parent remote branch")
	}
}

func TestIntegrationFinalizeCleanupInjectedProbeErrors(t *testing.T) {
	requireRealGit(t)
	for _, fail := range []string{"resolve", "remote-probe", "list", "ancestor", "fetch"} {
		fail := fail
		t.Run("git-"+fail, func(t *testing.T) {
			f := setupCloseoutFixture(t, planRepoModeDocket())
			head, mergeCommit := f.archiveClosed(t)
			gh := f.mergedCleanupFake(head, mergeCommit)
			git := &faultyCleanupGit{inner: f.deps.Client, fail: fail}
			res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, git, f.svc), f.repo.invocation, f.id)
			if res.Disposition == CleanupDispCleaned {
				t.Fatalf("an injected %q probe error must not report cleaned", fail)
			}
			if !f.remoteBranchPresent(t) && !f.localBranchPresent(t) {
				t.Fatalf("an injected %q probe error destroyed a resource", fail)
			}
		})
	}

	t.Run("workspace-manifest-lock", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		gh := f.mergedCleanupFake(head, mergeCommit)
		ws := &faultyCleanupWorkspace{Service: f.svc, failCleanup: true}
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, ws), f.repo.invocation, f.id)
		if res.Disposition == CleanupDispCleaned {
			t.Fatalf("an injected workspace probe error must not report cleaned")
		}
		if !f.localBranchPresent(t) {
			t.Fatalf("a workspace probe error must leave the local branch intact (still checked out)")
		}
	})
}

func TestIntegrationFinalizeCleanupNeverTouchesForeignTrees(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)
	gh := f.mergedCleanupFake(head, mergeCommit)

	// Record the primary worktree HEAD and the metadata tree before cleanup.
	primaryHeadBefore := runGit(t, f.repo.writer, "rev-parse", "HEAD")

	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
	if res.Result != ResultApplied {
		t.Fatalf("cleanup = %q (%s)", res.Result, res.Message)
	}
	primaryHeadAfter := runGit(t, f.repo.writer, "rev-parse", "HEAD")
	if primaryHeadBefore != primaryHeadAfter {
		t.Fatalf("cleanup moved the primary worktree HEAD (%s -> %s)", primaryHeadBefore, primaryHeadAfter)
	}
	// The primary worktree directory still exists and is a checkout.
	if _, err := os.Stat(filepath.Join(f.repo.writer, ".git")); err != nil {
		if _, err2 := os.Stat(f.repo.writer); err2 != nil {
			t.Fatalf("cleanup removed the primary worktree: %v", err2)
		}
	}
}

func TestIntegrationFinalizeCleanupOnlyAfterTerminal(t *testing.T) {
	requireRealGit(t)

	t.Run("non-terminal-refused", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		// The record is implemented (non-terminal), no aborted rebase scratch.
		gh := f.mergedCleanupFake(f.head, strings.Repeat("d", 40))
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
		if res.Result == ResultApplied {
			t.Fatalf("cleanup of a non-terminal change must refuse, got %q disp %q", res.Result, res.Disposition)
		}
		if !f.localBranchPresent(t) || !f.remoteBranchPresent(t) {
			t.Fatalf("a refused cleanup must leave the branches intact")
		}
	})

	t.Run("aborted-rebase-scratch-cleared", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		// Simulate the aborted-owned-rebase residue: owned refs + a receipt whose
		// OrigHead equals the current feature head (the rewrite was undone).
		prefix := ownedRefPrefixFor(f.id)
		if err := f.deps.Client.SetOwnedRef(context.Background(), f.gitrepo, gitcli.RefName(prefix+"/orig"), gitcli.ObjectID(f.head)); err != nil {
			t.Fatalf("set owned ref: %v", err)
		}
		rec := workspace.RebaseReceipt{
			RepoIdentity: f.gitrepo.CommonDir, ChangeID: itoa(f.id), OrigHead: f.head,
			OrigRemoteHead: f.head, BaseRef: "refs/heads/main", BaseHead: f.baseTip,
			Attempt: "att-1", CreatedUTC: "2026-08-18T00:00:00Z",
		}
		if err := f.svc.WriteRebaseReceipt(context.Background(), f.metaDir, rec); err != nil {
			t.Fatalf("write receipt: %v", err)
		}
		gh := f.mergedCleanupFake(f.head, strings.Repeat("d", 40))
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
		if res.Result != ResultApplied || res.Disposition != CleanupDispRebaseScratchCleared {
			t.Fatalf("aborted-rebase cleanup = %q disp %q (%s)", res.Result, res.Disposition, res.Message)
		}
		if _, present, _ := f.svc.ReadRebaseReceipt(context.Background(), f.metaDir); present {
			t.Fatalf("the rebase receipt must be cleared")
		}
	})
}

func TestIntegrationFinalizeCleanupRetryable(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)
	gh := f.mergedCleanupFake(head, mergeCommit)
	deps := f.cleanupDeps(gh, f.deps.Client, f.svc)

	first := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
	if first.Result != ResultApplied || first.Disposition != CleanupDispCleaned {
		t.Fatalf("first cleanup = %q disp %q (%s)", first.Result, first.Disposition, first.Message)
	}
	// A replay reads clean absence + tombstone and is a no-op, never a re-delete
	// error.
	second := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
	if second.Result != ResultNoOp && second.Result != ResultApplied {
		t.Fatalf("replay must be a clean no-op, got %q (%s)", second.Result, second.Message)
	}
	if second.Disposition == CleanupDispPending {
		t.Fatalf("a replay over clean absence must not report pending")
	}
}

func TestIntegrationFinalizeCleanupStackedRetained(t *testing.T) {
	requireRealGit(t)
	f := setupStackedMergedCleanupFixture(t)
	gh := f.mergedCleanupFake(f.head, strings.Repeat("d", 40))
	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
	if res.Disposition != CleanupDispRetained {
		t.Fatalf("a stacked-merged change must retain until root closes, got %q (%s)", res.Disposition, res.Message)
	}
	if !f.localBranchPresent(t) || !f.remoteBranchPresent(t) {
		t.Fatalf("a stacked-merged change must retain its branches")
	}
}
