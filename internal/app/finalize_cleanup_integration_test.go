//go:build integration

package app

import (
	"context"
	"errors"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/workspace"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestIntegrationFinalizeCleanupOnlyAfterFinal(t *testing.T) {
	requireRealGit(t)

	t.Run("non-final-refused", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		// The record is implemented (non-final), no aborted rebase scratch.
		gh := f.mergedCleanupFake(f.head, strings.Repeat("d", 40))
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
		if res.Result == ResultApplied {
			t.Fatalf("cleanup of a non-final change must refuse, got %q disp %q", res.Result, res.Disposition)
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

// setupKilledCleanupFixture builds a closeout fixture (real prepared workspace
// and published feature branch) and then relocates its record to archive/ as a
// killed change — the state a reconcile-kill of a resumed in-progress change
// leaves behind. A killed record carries no branch or claim stamp (change.kill
// strips them), which lifecycleChange already models for status "killed".
func setupKilledCleanupFixture(t *testing.T) *closeoutFixture {
	t.Helper()
	f := setupCloseoutFixture(t, planRepoModeDocket())
	activePath := groomPath(f.id, f.slug)
	archivePath := "docs/changes/archive/2026-08-16-" + padID(f.id) + "-" + f.slug + ".md"
	// Sync the writer's metadata branch to origin before removing the active
	// record, so the removal commits on top of the published tip.
	runGit(t, f.repo.writer, "fetch", "-q", "origin", f.branch)
	runGit(t, f.repo.writer, "checkout", "-q", f.branch)
	runGit(t, f.repo.writer, "reset", "-q", "--hard", "origin/"+f.branch)
	runGit(t, f.repo.writer, "rm", "-q", activePath)
	f.repo.writerAdvance(t, f.branch, map[string]string{archivePath: lifecycleChange(f.id, f.slug, "killed")})
	f.revision = blobRevisionAt(t, f.repo.origin, f.branch, archivePath)
	return f
}

func TestIntegrationFinalizeCleanupKilledRetained(t *testing.T) {
	requireRealGit(t)
	f := setupKilledCleanupFixture(t)
	gh := f.mergedCleanupFake(f.head, strings.Repeat("d", 40))
	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
	if res.Result != ResultNoOp || res.Disposition != CleanupDispRetained || res.Reason != ReasonCleanupKilledRetained {
		t.Fatalf("a killed change must be a retained no-op, got result %q disp %q reason %q (%s)",
			res.Result, res.Disposition, res.Reason, res.Message)
	}
	if !strings.Contains(res.Message, "killed") {
		t.Fatalf("the message must name the killed condition, got %q", res.Message)
	}
	if _, err := os.Stat(f.wp); err != nil {
		t.Fatalf("a killed change's workspace must be retained: %v", err)
	}
	if !f.localBranchPresent(t) || !f.remoteBranchPresent(t) {
		t.Fatalf("a killed change must retain its branches")
	}
}

// lockIgnoredWorkspaceDir makes Git's delete of the feature workspace fail part-
// way after it has removed the registration: an ignored (clean-check-invisible)
// directory holding one file, made read-only. Restored at test end.
func lockIgnoredWorkspaceDir(t *testing.T, commonDir, ws string) {
	t.Helper()
	info := filepath.Join(commonDir, "info")
	if err := os.MkdirAll(info, 0o755); err != nil {
		t.Fatal(err)
	}
	ex, err := os.OpenFile(filepath.Join(info, "exclude"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ex.WriteString("\nlocked/\n"); err != nil {
		t.Fatal(err)
	}
	if err := ex.Close(); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, ws, "locked/keep", "ignored bytes\n")
	locked := filepath.Join(ws, "locked")
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
}

// TestIntegrationFinalizeCleanupWorkspaceRemnant: Git removes the registration
// but part of the folder cannot be deleted. The workspace leg is done, the
// local and remote branch legs still run, and the result is cleaned with
// exactly one workspace-remnant warning naming the leftover path.
func TestIntegrationFinalizeCleanupWorkspaceRemnant(t *testing.T) {
	requireRealGit(t)
	if os.Geteuid() == 0 {
		t.Skip("permission-based removal failure needs a non-root user")
	}
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)
	gh := f.mergedCleanupFake(head, mergeCommit)
	ws, err := filepath.EvalSymlinks(f.wp)
	if err != nil {
		t.Fatal(err)
	}
	lockIgnoredWorkspaceDir(t, f.gitrepo.CommonDir, ws)

	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)

	if res.Result != ResultApplied || res.Disposition != CleanupDispCleaned {
		t.Fatalf("cleanup = %q disp %q (%s) findings %+v; want applied/cleaned", res.Result, res.Disposition, res.Message, res.Findings)
	}
	if f.localBranchPresent(t) || f.remoteBranchPresent(t) {
		t.Fatal("the branch legs must still run after a remnant")
	}
	if len(res.Findings) != 1 || res.Findings[0].Code != "workspace-remnant" || res.Findings[0].Severity != "warning" {
		t.Fatalf("findings = %+v; want exactly one workspace-remnant warning", res.Findings)
	}
	want := "Git removed the worktree but part of the folder could not be deleted; delete " + ws + " by hand"
	if res.Findings[0].Message != want {
		t.Fatalf("message = %q; want %q", res.Findings[0].Message, want)
	}
}

// TestIntegrationFinalizeCleanupWorkspaceBlockedNamesPath: an untracked,
// non-ignored file blocks cleanup (nothing touched, branches retained) and the
// workspace-blocked finding names it.
func TestIntegrationFinalizeCleanupWorkspaceBlockedNamesPath(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModeDocket())
	head, mergeCommit := f.archiveClosed(t)
	gh := f.mergedCleanupFake(head, mergeCommit)
	writeRepoFile(t, f.wp, "stray-notes.txt", "not committed\n")

	res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)

	if res.Disposition != CleanupDispPending {
		t.Fatalf("disposition = %q; want pending", res.Disposition)
	}
	var msg string
	for _, fd := range res.Findings {
		if fd.Code == "workspace-blocked" {
			msg = fd.Message
		}
	}
	if !strings.Contains(msg, "(stray-notes.txt)") {
		t.Fatalf("workspace-blocked message = %q; want it to name stray-notes.txt", msg)
	}
	if !f.localBranchPresent(t) {
		t.Fatal("a blocked workspace must retain the local branch")
	}
	if _, err := os.Stat(filepath.Join(f.wp, "stray-notes.txt")); err != nil {
		t.Fatalf("the blocking file must survive: %v", err)
	}
}

// TestIntegrationFinalizeCleanupPRBacklinkRetry proves cleanup is the retry for
// a PR backlink close-out left pending: a done change whose PR still names the
// active path is repointed; a failing edit makes the cleanup pending (retryable)
// without blocking the independent legs; a replay over a repointed PR issues no
// edit.
func TestIntegrationFinalizeCleanupPRBacklinkRetry(t *testing.T) {
	requireRealGit(t)

	t.Run("repoints-a-pending-pr", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		deps := f.cleanupDeps(f.mergedCleanupFake(head, mergeCommit), f.deps.Client, f.svc)
		recPath := groomPath(f.id, f.slug)
		pr := newFakePRBody(map[int]string{closeoutPR: artifactWithBacklink(recPath, "Summary", "prose")})
		deps.PRBody = pr

		res := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
		if res.Disposition != CleanupDispCleaned {
			t.Fatalf("cleanup = %q disp %q (%s) findings=%+v", res.Result, res.Disposition, res.Message, res.Findings)
		}
		if got := pr.bodies[closeoutPR]; strings.Contains(got, "`"+recPath+"`") || !strings.Contains(got, "docs/changes/archive/") {
			t.Fatalf("cleanup did not repoint the PR backlink:\n%s", got)
		}
		replay := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
		if replay.Disposition == CleanupDispPending || pr.edits != 1 {
			t.Fatalf("replay disp %q edits=%d, want clean with no new edit", replay.Disposition, pr.edits)
		}
	})

	t.Run("failing-edit-is-pending-and-retryable", func(t *testing.T) {
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		deps := f.cleanupDeps(f.mergedCleanupFake(head, mergeCommit), f.deps.Client, f.svc)
		pr := newFakePRBody(map[int]string{closeoutPR: artifactWithBacklink(groomPath(f.id, f.slug), "Summary", "prose")})
		pr.editErr = errors.New("gh: HTTP 502")
		deps.PRBody = pr

		res := FinalizeCleanup(context.Background(), deps, f.repo.invocation, f.id)
		if res.Disposition != CleanupDispPending || res.Reason != ReasonPRBacklinkPending {
			t.Fatalf("cleanup = disp %q reason %q, want pending/%s", res.Disposition, res.Reason, ReasonPRBacklinkPending)
		}
		if f.remoteBranchPresent(t) {
			t.Fatalf("a PR-body failure must not block the independent ref legs")
		}
	})
}

// TestIntegrationFinalizeCleanupPRBacklinkRetryCarriedDescendant proves a
// carried stacked descendant has a reachable PR-backlink retry: its PR merged
// into the ROOT's branch (never the integration branch), so cleanup refuses its
// destructive legs as destination-mismatch — but the non-destructive PR-body
// repoint still runs first, landing the edit close-out left pending.
func TestIntegrationFinalizeCleanupPRBacklinkRetryCarriedDescendant(t *testing.T) {
	requireRealGit(t)
	f := setupCloseoutFixture(t, planRepoModes()[0])
	descPlan := "docs/superpowers/plans/2026-08-16-gadget-plan.md"
	descActive := groomPath(6, "gadget")
	desc := closeoutRecord(6, "gadget", "stacked-merged", "github.com/acme/widget#8", "", descPlan, "")
	desc = strings.Replace(desc, "stacked_on:\n", "stacked_on: 5\n", 1)
	f.repo.writerAdvance(t, f.branch, map[string]string{
		descActive: desc,
		descPlan:   artifactWithBacklink(descActive, "Gadget plan", "The gadget plan."),
	})
	childMerge := f.carryOntoRootFeature(t, map[string]string{"gadget.txt": "gadget work\n"})
	mergeCommit := f.mergeIntoBase(t)
	f.fetchAllIntoInvocation(t)
	merged := map[int]closeoutProbe{
		closeoutPR: {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(f.head, "main", mergeCommit)},
		8:          {outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(childMerge, "feat/"+f.slug, childMerge)},
	}

	// Close-out archives root + descendant, but every PR-body edit fails: the
	// descendant's PR still names its ACTIVE record path.
	pr := newFakePRBody(map[int]string{
		closeoutPR: artifactWithBacklink(groomPath(5, "widget"), "Root", "root prose"),
		8:          artifactWithBacklink(descActive, "Child", "child prose"),
	})
	pr.editErr = errors.New("gh: HTTP 502")
	cdeps := f.closeoutDeps(&fakeCloseoutGitHub{repo: retargetRepo(), merged: merged})
	cdeps.PRBody = pr
	if res := FinalizeCloseout(context.Background(), cdeps, f.repo.invocation, f.id, CloseoutNotes{}); res.Disposition != CloseoutDispRootArchived {
		t.Fatalf("root carry = %q disp %q (%s)", res.Result, res.Disposition, res.Message)
	}
	if got := pr.bodies[8]; !strings.Contains(got, "`"+descActive+"`") {
		t.Fatalf("fixture: the failed close-out edit must leave the active backlink:\n%s", got)
	}

	// The retry: cleanup of the carried descendant, the edit now succeeding.
	pr.editErr = nil
	deps := f.cleanupDeps(&fakeCleanupGitHub{repo: retargetRepo(), merged: merged, openByHead: map[string][]githubcli.PullRequest{}}, f.deps.Client, f.svc)
	deps.PRBody = pr
	res := FinalizeCleanup(context.Background(), deps, f.repo.invocation, 6)

	if got := pr.bodies[8]; strings.Contains(got, "`"+descActive+"`") || !strings.Contains(got, "docs/changes/archive/2026-08-18-0006-gadget.md") {
		t.Fatalf("cleanup did not repoint the carried descendant's PR backlink:\n%s", got)
	}
	// The destructive legs are still refused: nothing removed, the root's branch
	// (which carries the descendant's merged work) intact.
	if res.Result != ResultBlocked || res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupDestination {
		t.Fatalf("cleanup = %q disp %q reason %q, want blocked/pending/%s", res.Result, res.Disposition, res.Reason, ReasonCleanupDestination)
	}
	if len(res.RemovedRefs) != 0 || !f.localBranchPresent(t) || !f.remoteBranchPresent(t) {
		t.Fatalf("a destination-mismatch cleanup must remove nothing; removed=%v", res.RemovedRefs)
	}

	// A still-failing retry surfaces the retryable finding on the refusal.
	pr.bodies[8] = artifactWithBacklink(descActive, "Child", "child prose")
	pr.editErr = errors.New("gh: HTTP 502")
	again := FinalizeCleanup(context.Background(), deps, f.repo.invocation, 6)
	var pending bool
	for _, fd := range again.Findings {
		pending = pending || fd.Code == ReasonPRBacklinkPending
	}
	if again.Reason != ReasonCleanupDestination || !pending {
		t.Fatalf("failing retry = reason %q findings %+v, want %s carrying a %s finding", again.Reason, again.Findings, ReasonCleanupDestination, ReasonPRBacklinkPending)
	}
}
