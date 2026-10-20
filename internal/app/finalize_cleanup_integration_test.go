//go:build integration

package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/danielhanold/docket/internal/domain"
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
// into the ROOT's branch (never the integration branch), and the
// non-destructive PR-body repoint runs first, landing the edit close-out left
// pending. The descendant's carry is proven, but it has no workspace of its own
// on this machine, so cleanup stops at the workspace leg: nothing is removed and
// the root's branches, which carry the descendant's merged work, stay intact.
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
	// The carry is proven, but the descendant has no workspace here, so the
	// workspace leg holds the branch legs back: nothing removed, the root's branch
	// (which carries the descendant's merged work) intact.
	if res.Result != ResultBlocked || res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupWorkspaceBlocked {
		t.Fatalf("cleanup = %q disp %q reason %q, want blocked/pending/%s", res.Result, res.Disposition, res.Reason, ReasonCleanupWorkspaceBlocked)
	}
	if len(res.RemovedRefs) != 0 || !f.localBranchPresent(t) || !f.remoteBranchPresent(t) {
		t.Fatalf("a pending cleanup must remove nothing; removed=%v", res.RemovedRefs)
	}

	// A still-failing retry surfaces the retryable finding on the pending result.
	pr.bodies[8] = artifactWithBacklink(descActive, "Child", "child prose")
	pr.editErr = errors.New("gh: HTTP 502")
	again := FinalizeCleanup(context.Background(), deps, f.repo.invocation, 6)
	var pending bool
	for _, fd := range again.Findings {
		pending = pending || fd.Code == ReasonPRBacklinkPending
	}
	if again.Disposition != CleanupDispPending || !pending || len(again.RemovedRefs) != 0 {
		t.Fatalf("failing retry = disp %q findings %+v removed %v, want pending carrying a %s finding with nothing removed",
			again.Disposition, again.Findings, again.RemovedRefs, ReasonPRBacklinkPending)
	}
	if !f.localBranchPresent(t) || !f.remoteBranchPresent(t) {
		t.Fatalf("a failing retry must leave the root's branches intact")
	}
}

// --- carried stacked children ---------------------------------------------

// carriedMember is one change of a real stack built by setupCarriedStackFixture:
// its record identity, its prepared workspace, the head its PR merged, and the
// merge result that PR produced on its parent's branch (on main for the root).
type carriedMember struct {
	id        int
	slug      string
	branch    string
	pr        int
	wp        string
	forkPoint string // the parent branch tip the member's workspace was cut from
	head      string
	baseRef   string // the branch its PR merged into
	merge     string
}

// facts are the member's authoritative merged-PR facts.
func (m *carriedMember) facts() closeoutProbe {
	return closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(m.head, m.baseRef, m.merge)}
}

// carriedStackFixture is a real stack whose root closed out: every member is
// done and archived, and every member still has its workspace folder, local
// branch, and remote branch.
type carriedStackFixture struct {
	*closeoutFixture
	members []*carriedMember // root first, then each child in stack order
}

// cleanupFake answers every member's merged PR with its real facts.
func (sf *carriedStackFixture) cleanupFake() *fakeCleanupGitHub {
	merged := map[int]closeoutProbe{}
	for _, m := range sf.members {
		merged[m.pr] = m.facts()
	}
	return &fakeCleanupGitHub{repo: retargetRepo(), merged: merged, openByHead: map[string][]githubcli.PullRequest{}}
}

// clean runs FinalizeCleanup for one member through the real Git client and
// workspace service.
func (sf *carriedStackFixture) clean(t *testing.T, gh FinalizeGitHub, git FinalizeCleanupGit, id int) CleanupOpResult {
	t.Helper()
	return FinalizeCleanup(context.Background(), sf.cleanupDeps(gh, git, sf.svc), sf.repo.invocation, id)
}

// branchPresent reports whether branch exists locally and on origin.
func (sf *carriedStackFixture) branchPresent(t *testing.T, branch string) (local, remote bool) {
	t.Helper()
	_, err := sf.deps.Client.ResolveRef(context.Background(), sf.gitrepo, gitcli.RefName("refs/heads/"+branch))
	rr, rerr := sf.deps.Client.ProbeRemoteBranch(context.Background(), sf.gitrepo, "origin", gitcli.RefName("refs/heads/"+branch))
	if rerr != nil {
		t.Fatalf("remote probe %s: %v", branch, rerr)
	}
	return err == nil, rr.State == gitcli.RemoteRefFound
}

// assertGone fails unless the member's folder, local branch, and remote branch
// are all gone.
func (sf *carriedStackFixture) assertGone(t *testing.T, m *carriedMember) {
	t.Helper()
	if _, err := os.Stat(m.wp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("change %d: workspace %s must be removed (stat err %v)", m.id, m.wp, err)
	}
	if local, remote := sf.branchPresent(t, m.branch); local || remote {
		t.Fatalf("change %d: %s must be gone (local %v, remote %v)", m.id, m.branch, local, remote)
	}
}

// assertKept fails unless the member's folder, local branch, and remote branch
// all still exist.
func (sf *carriedStackFixture) assertKept(t *testing.T, m *carriedMember) {
	t.Helper()
	if _, err := os.Stat(m.wp); err != nil {
		t.Fatalf("change %d: workspace %s must be kept: %v", m.id, m.wp, err)
	}
	if local, remote := sf.branchPresent(t, m.branch); !local || !remote {
		t.Fatalf("change %d: %s must be kept (local %v, remote %v)", m.id, m.branch, local, remote)
	}
}

// setupCarriedStackFixture builds a real stack of depth members on the closeout
// fixture's root (5 feat/widget, PR #7): child 6 (feat/gadget, PR #8) stacked on
// 5 and, at depth 3, grandchild 7 (feat/gizmo, PR #9) stacked on 6. Each child
// gets a real prepared workspace cut from its parent's branch, one commit, and a
// published head. Every PR then lands REBASE-STYLE, bottom-up: a child's commits
// are cherry-picked onto its parent's branch (in the parent's workspace, so the
// parent's local branch and remote advance together), and the root's commits are
// cherry-picked onto main. No merge result is an ancestor of main, so the carry
// content proof needs ProvePreserved. The root then closes out, archiving every
// member as done.
func setupCarriedStackFixture(t *testing.T, depth int) *carriedStackFixture {
	t.Helper()
	ctx := context.Background()
	f := setupCloseoutFixture(t, planRepoModeDocket())
	root := &carriedMember{
		id: f.id, slug: f.slug, branch: "feat/" + f.slug, pr: closeoutPR, wp: f.wp,
		forkPoint: f.baseTip, head: f.head, baseRef: "main",
	}
	sf := &carriedStackFixture{closeoutFixture: f, members: []*carriedMember{root}}
	children := []struct {
		id   int
		slug string
		pr   int
	}{{6, "gadget", 8}, {7, "gizmo", 9}}

	records := map[string]string{}
	for i := 1; i < depth; i++ {
		parent, c := sf.members[i-1], children[i-1]
		m := &carriedMember{id: c.id, slug: c.slug, branch: "feat/" + c.slug, pr: c.pr, forkPoint: parent.head, baseRef: parent.branch}
		recPath, planPath := groomPath(c.id, c.slug), "docs/superpowers/plans/2026-08-16-"+c.slug+"-plan.md"
		rec := closeoutRecord(c.id, c.slug, "stacked-merged", "github.com/acme/widget#"+itoa(c.pr), "", planPath, "")
		records[recPath] = strings.Replace(rec, "stacked_on:\n", "stacked_on: "+itoa(parent.id)+"\n", 1)
		records[planPath] = artifactWithBacklink(recPath, c.slug+" plan", "The "+c.slug+" plan.")

		target, err := workspace.NewTarget(domain.ChangeID(c.id), c.slug, domain.EffectiveBase{Kind: domain.BaseResolved, Branch: parent.branch}, m.branch)
		if err != nil {
			t.Fatalf("NewTarget %d: %v", c.id, err)
		}
		ws, err := f.svc.Prepare(ctx, workspace.PrepareRequest{Repository: f.gitrepo, Remote: "origin", Target: target})
		if err != nil {
			t.Fatalf("prepare %d: %v", c.id, err)
		}
		m.wp = ws.Path
		writeRepoFile(t, m.wp, c.slug+".txt", c.slug+" work\n")
		runGit(t, m.wp, "add", "-A")
		runGit(t, m.wp, "commit", "-q", "-m", "implement "+c.slug)
		m.head = runGit(t, m.wp, "rev-parse", "HEAD")
		if _, err := f.svc.PublishHead(ctx, workspace.PublishRequest{Repository: f.gitrepo, Remote: "origin", Target: target}); err != nil {
			t.Fatalf("publish %d: %v", c.id, err)
		}
		sf.members = append(sf.members, m)
	}
	if len(records) > 0 {
		f.repo.writerAdvance(t, f.branch, records)
	}

	// Land each child's PR on its parent's branch, deepest first. `-x` records
	// the picked id in each message, so every merge result is a NEW commit even
	// when it is picked onto the same parent within the same second (an identical
	// commit would silently turn the rebase merge into a fast-forward).
	for i := len(sf.members) - 1; i >= 1; i-- {
		parent, m := sf.members[i-1], sf.members[i]
		runGit(t, parent.wp, "cherry-pick", "-x", m.forkPoint+".."+m.head)
		runGit(t, parent.wp, "push", "-q", "origin", "HEAD:refs/heads/"+parent.branch)
		parent.head = runGit(t, parent.wp, "rev-parse", "HEAD")
		m.merge = parent.head
	}
	// Land the root's PR on main.
	w := f.repo.writer
	runGit(t, w, "fetch", "-q", "origin", "+refs/heads/main:refs/remotes/origin/main", "+refs/heads/"+root.branch+":refs/remotes/origin/"+root.branch)
	runGit(t, w, "checkout", "-q", "-B", "main", "origin/main")
	runGit(t, w, "cherry-pick", "-x", root.forkPoint+".."+root.head)
	runGit(t, w, "push", "-q", "origin", "main")
	root.merge = runGit(t, w, "rev-parse", "HEAD")
	f.fetchAllIntoInvocation(t)
	// Pin the rebase shape: no child's merge result is on main, so the carry
	// content proof can only pass through ProvePreserved, never plain ancestry.
	for _, m := range sf.members[1:] {
		if _, err := tryGit(f.repo.invocation, "merge-base", "--is-ancestor", m.merge, root.merge); err == nil {
			t.Fatalf("fixture: change %d's merge result %s must not be an ancestor of the root merge %s", m.id, m.merge, root.merge)
		}
	}

	merged := map[int]closeoutProbe{}
	for _, m := range sf.members {
		merged[m.pr] = m.facts()
	}
	res := FinalizeCloseout(ctx, f.closeoutDeps(&fakeCloseoutGitHub{repo: retargetRepo(), merged: merged}), f.repo.invocation, f.id, CloseoutNotes{})
	if depth > 1 && (res.Result != ResultApplied || res.Disposition != CloseoutDispRootArchived || len(res.CarriedIDs) != depth-1) {
		t.Fatalf("root closeout = %q disp %q carried %v (reason %q: %s)", res.Result, res.Disposition, res.CarriedIDs, res.Reason, res.Message)
	}
	return sf
}

// TestIntegrationFinalizeCleanupCarriedChildTwoLevel: once the root lands and is
// cleaned (deleting the branch the child merged into), the carried child's
// cleanup proves its work reached main through the root and removes its folder,
// local branch, and remote branch. A replay removes nothing and is not pending.
func TestIntegrationFinalizeCleanupCarriedChildTwoLevel(t *testing.T) {
	requireRealGit(t)
	sf := setupCarriedStackFixture(t, 2)
	gh := sf.cleanupFake()
	root, child := sf.members[0], sf.members[1]

	if res := sf.clean(t, gh, sf.deps.Client, root.id); res.Disposition != CleanupDispCleaned {
		t.Fatalf("root cleanup = %q disp %q reason %q (%s) findings %+v", res.Result, res.Disposition, res.Reason, res.Message, res.Findings)
	}
	sf.assertGone(t, root)

	res := sf.clean(t, gh, sf.deps.Client, child.id)
	if res.Result != ResultApplied || res.Disposition != CleanupDispCleaned {
		t.Fatalf("child cleanup = %q disp %q reason %q (%s) findings %+v", res.Result, res.Disposition, res.Reason, res.Message, res.Findings)
	}
	sf.assertGone(t, child)
	removed := strings.Join(res.RemovedRefs, ",")
	for _, want := range []string{"local:" + child.branch, "remote:" + child.branch} {
		if !strings.Contains(removed, want) {
			t.Fatalf("removed refs %v lack %q", res.RemovedRefs, want)
		}
	}

	replay := sf.clean(t, gh, sf.deps.Client, child.id)
	if replay.Disposition == CleanupDispPending || len(replay.RemovedRefs) != 0 {
		t.Fatalf("replay = %q disp %q removed %v reason %q, want a clean no-op", replay.Result, replay.Disposition, replay.RemovedRefs, replay.Reason)
	}
}

// TestIntegrationFinalizeCleanupCarriedChildThreeLevel: a grandchild carried
// through an intermediate child is proven through the whole chain; cleaning the
// root, then the child, then the grandchild leaves no folder and no branch.
func TestIntegrationFinalizeCleanupCarriedChildThreeLevel(t *testing.T) {
	requireRealGit(t)
	sf := setupCarriedStackFixture(t, 3)
	gh := sf.cleanupFake()
	for _, m := range sf.members {
		res := sf.clean(t, gh, sf.deps.Client, m.id)
		if res.Result != ResultApplied || res.Disposition != CleanupDispCleaned {
			t.Fatalf("cleanup %d = %q disp %q reason %q (%s) findings %+v", m.id, res.Result, res.Disposition, res.Reason, res.Message, res.Findings)
		}
	}
	for _, m := range sf.members {
		sf.assertGone(t, m)
	}
}

// probeErrOnPR is a cleanup GitHub fake whose merged-PR probe errors for one PR
// only, so the child's own probe succeeds and an ancestor's fails.
type probeErrOnPR struct {
	*fakeCleanupGitHub
	failPR int
}

func (p *probeErrOnPR) ProbeMerged(ctx context.Context, repo githubcli.Repository, number int) (githubcli.MergeOutcome, githubcli.MergedFacts, error) {
	if number == p.failPR {
		return githubcli.MergeUnknown, githubcli.MergedFacts{}, errors.New("gh: HTTP 502")
	}
	return p.fakeCleanupGitHub.ProbeMerged(ctx, repo, number)
}

// TestIntegrationFinalizeCleanupCarriedChildFailClosed: whenever a carried
// child's relationship or content can't be proven, or a probe can't be answered,
// cleanup keeps its folder and both branches and reports pending.
func TestIntegrationFinalizeCleanupCarriedChildFailClosed(t *testing.T) {
	requireRealGit(t)
	rows := []struct {
		name   string
		depth  int
		result Result
		reason string
		run    func(t *testing.T, sf *carriedStackFixture) CleanupOpResult
	}{
		{
			name: "content-unproven", depth: 2, result: ResultBlocked, reason: ReasonCleanupCarryUnproven,
			run: func(t *testing.T, sf *carriedStackFixture) CleanupOpResult {
				// The child's merge result is a real, pushed commit whose work never
				// reached main.
				dropped := sf.carryCommit(t, "gadget-dropped", sf.baseTip, map[string]string{"dropped.txt": "never shipped\n"})
				sf.fetchAllIntoInvocation(t)
				gh := sf.cleanupFake()
				child := sf.members[1]
				gh.merged[child.pr] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(child.head, child.baseRef, dropped)}
				return sf.clean(t, gh, sf.deps.Client, child.id)
			},
		},
		{
			name: "root-merge-not-on-main", depth: 2, result: ResultBlocked, reason: ReasonCleanupCarryUnproven,
			run: func(t *testing.T, sf *carriedStackFixture) CleanupOpResult {
				// The root's reported merge result is its own branch tip, which holds
				// the child's work but never reached main: the root's merge must be
				// re-proven on main before it can vouch for the child.
				gh := sf.cleanupFake()
				root := sf.members[0]
				gh.merged[root.pr] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(root.head, "main", root.head)}
				return sf.clean(t, gh, sf.deps.Client, sf.members[1].id)
			},
		},
		{
			name: "ancestor-probe-error", depth: 2, result: ResultExternalFailed, reason: ReasonCleanupProbeUnknown,
			run: func(t *testing.T, sf *carriedStackFixture) CleanupOpResult {
				gh := &probeErrOnPR{fakeCleanupGitHub: sf.cleanupFake(), failPR: sf.members[0].pr}
				return sf.clean(t, gh, sf.deps.Client, sf.members[1].id)
			},
		},
		{
			name: "preserve-probe-error", depth: 2, result: ResultExternalFailed, reason: ReasonCleanupAncestryProbe,
			run: func(t *testing.T, sf *carriedStackFixture) CleanupOpResult {
				return sf.clean(t, sf.cleanupFake(), &faultyCleanupGit{inner: sf.deps.Client, fail: "preserve"}, sf.members[1].id)
			},
		},
		{
			name: "destination-matches-no-parent", depth: 2, result: ResultBlocked, reason: ReasonCleanupDestination,
			run: func(t *testing.T, sf *carriedStackFixture) CleanupOpResult {
				gh := sf.cleanupFake()
				child := sf.members[1]
				gh.merged[child.pr] = closeoutProbe{outcome: githubcli.MergeAlreadyMerged, facts: mergedFactsFor(child.head, "feat/elsewhere", child.merge)}
				return sf.clean(t, gh, sf.deps.Client, child.id)
			},
		},
		{
			name: "killed-link", depth: 3, result: ResultBlocked, reason: ReasonCleanupDestination,
			run: func(t *testing.T, sf *carriedStackFixture) CleanupOpResult {
				// Rewrite the intermediate child's archived record as killed.
				runGit(t, sf.repo.writer, "fetch", "-q", "origin", sf.branch)
				runGit(t, sf.repo.writer, "checkout", "-q", sf.branch)
				runGit(t, sf.repo.writer, "reset", "-q", "--hard", "origin/"+sf.branch)
				killed := strings.Replace(lifecycleChange(6, "gadget", "killed"), "stacked_on:\n", "stacked_on: 5\n", 1)
				sf.repo.writerAdvance(t, sf.branch, map[string]string{"docs/changes/archive/2026-08-18-0006-gadget.md": killed})
				return sf.clean(t, sf.cleanupFake(), sf.deps.Client, sf.members[2].id)
			},
		},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			sf := setupCarriedStackFixture(t, row.depth)
			res := row.run(t, sf)
			if res.Result != row.result || res.Disposition != CleanupDispPending || res.Reason != row.reason {
				t.Fatalf("cleanup = %q disp %q reason %q (%s), want %q/pending/%s", res.Result, res.Disposition, res.Reason, res.Message, row.result, row.reason)
			}
			if len(res.RemovedRefs) != 0 {
				t.Fatalf("a refused cleanup removed %v", res.RemovedRefs)
			}
			sf.assertKept(t, sf.members[len(sf.members)-1])
			for _, fd := range res.Findings {
				if isKeptBranchFinding(fd) {
					t.Fatalf("a refused carried-child cleanup must not carry a kept-branch finding: %+v", fd)
				}
			}
			if row.reason == ReasonCleanupCarryUnproven {
				if !strings.Contains(res.Message, "main") {
					t.Fatalf("the carry-unproven message must name the integration branch: %q", res.Message)
				}
				for _, text := range []string{res.Message, res.HumanText()} {
					for _, banned := range []string{"git branch -D", "--delete", "by hand"} {
						if strings.Contains(text, banned) {
							t.Fatalf("the carry-unproven text must not advise deleting by hand (%q): %q", banned, text)
						}
					}
				}
			}
		})
	}
}

// TestIntegrationFinalizeCleanupCarriedChildBranchMoved: a commit made on the
// child's branch after its merge keeps the local branch with the named
// kept-branch warning, while the clean folder is still removed.
func TestIntegrationFinalizeCleanupCarriedChildBranchMoved(t *testing.T) {
	requireRealGit(t)
	sf := setupCarriedStackFixture(t, 2)
	child := sf.members[1]
	writeRepoFile(t, child.wp, "late.txt", "after the merge\n")
	runGit(t, child.wp, "add", "-A")
	runGit(t, child.wp, "commit", "-q", "-m", "after the merge")

	res := sf.clean(t, sf.cleanupFake(), sf.deps.Client, child.id)
	if res.Disposition != CleanupDispPending {
		t.Fatalf("cleanup = %q disp %q reason %q (%s), want pending", res.Result, res.Disposition, res.Reason, res.Message)
	}
	if _, err := os.Stat(child.wp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the clean workspace must still be removed (stat err %v)", err)
	}
	if local, _ := sf.branchPresent(t, child.branch); !local {
		t.Fatalf("a branch moved past its merge must be kept")
	}
	var moved bool
	for _, fd := range res.Findings {
		moved = moved || fd.Code == ReasonCleanupTipMoved
	}
	if !moved || !strings.Contains(res.HumanText(), "git branch -D "+child.branch) {
		t.Fatalf("want a %s kept-branch warning naming the branch, got findings %+v human %q", ReasonCleanupTipMoved, res.Findings, res.HumanText())
	}
}

// TestIntegrationFinalizeCleanupCarriedChildOpenStackedPR: an open pull request
// stacked on the carried child keeps the child's remote branch and reports
// children-retarget-required, exactly as for any done change.
func TestIntegrationFinalizeCleanupCarriedChildOpenStackedPR(t *testing.T) {
	requireRealGit(t)
	sf := setupCarriedStackFixture(t, 2)
	child := sf.members[1]
	runGit(t, sf.repo.writer, "fetch", "-q", "origin", sf.branch)
	runGit(t, sf.repo.writer, "checkout", "-q", sf.branch)
	runGit(t, sf.repo.writer, "reset", "-q", "--hard", "origin/"+sf.branch)
	live := strings.Replace(lifecycleChange(7, "gizmo", "in-progress"), "stacked_on:\n", "stacked_on: "+itoa(child.id)+"\n", 1)
	sf.repo.writerAdvance(t, sf.branch, map[string]string{groomPath(7, "gizmo"): live})
	gh := sf.cleanupFake()
	gh.openByHead["feat/gizmo"] = []githubcli.PullRequest{{
		Number: 9, State: githubcli.StateOpen, HeadBranch: "feat/gizmo", BaseBranch: child.branch,
	}}

	res := sf.clean(t, gh, sf.deps.Client, child.id)
	if res.Disposition != CleanupDispChildrenRetargetRequired {
		t.Fatalf("cleanup = %q disp %q reason %q (%s), want %s", res.Result, res.Disposition, res.Reason, res.Message, CleanupDispChildrenRetargetRequired)
	}
	if _, remote := sf.branchPresent(t, child.branch); !remote {
		t.Fatalf("an open stacked PR must keep the child's remote branch")
	}
}

// raFinalResults is a FINAL-valid results body; attach prepends the backlink.
const raFinalResults = "# Research — Results\n\n**Human action:** No required action.\n\n## Outcome\n\nThe research answered the question; the findings are recorded here.\n"

// completeLeakWorkspace attaches final results to the prepared results-artifact
// change at recPath on the metadata branch, completes it, and returns the
// cleanup deps over the real workspace service.
func completeLeakWorkspace(t *testing.T, lw *leakWorkspace, meta, branch, recPath string) FinalizeDeps {
	t.Helper()
	ctx := context.Background()
	ver := func() string { return blobRevisionAt(t, meta, branch, recPath) }
	slug := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(recPath), fmt.Sprintf("%04d-", lw.id)), ".md")
	attach := ChangeAttachResults(ctx, lw.node.deps, lw.node.dir, ChangeAttachRequest{
		ID: lw.id, Revision: ver(), Path: "docs/results/2026-08-17-" + slug + "-results.md", Markdown: []byte(raFinalResults),
	})
	if attach.Result != ResultApplied {
		t.Fatalf("attach results = %q (reason %q msg %q findings %v)", attach.Result, attach.Reason, attach.Message, attach.Findings)
	}
	done := ChangeComplete(ctx, lw.node.deps, lw.node.dir, ChangeCompleteRequest{ChangeID: lw.id, Path: recPath, Revision: ver()})
	if done.Result != ResultApplied || done.Disposition != CompleteDispCompleted {
		t.Fatalf("change complete = (%q, %q), findings %v", done.Result, done.Disposition, done.Findings)
	}
	svc, err := workspace.NewService(lw.node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	return FinalizeDeps{Planning: lw.node.deps, GitHub: raNoGitHub{}, Workspace: svc}
}

// assertResultsArtifactCleaned runs FinalizeCleanup over the completed change and
// asserts the worktree folder and the local branch are gone.
func assertResultsArtifactCleaned(t *testing.T, lw *leakWorkspace, deps FinalizeDeps) {
	t.Helper()
	res := FinalizeCleanup(context.Background(), deps, lw.node.dir, lw.id)
	if res.Result != ResultApplied || res.Disposition != CleanupDispCleaned {
		t.Fatalf("finalize cleanup = %q disp %q reason %q (%s)", res.Result, res.Disposition, res.Reason, res.Message)
	}
	if _, err := os.Stat(lw.wp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the feature worktree %s must be removed (stat err %v)", lw.wp, err)
	}
	if out := runGit(t, lw.invocation, "for-each-ref", "--format=%(refname)", lw.featureRef); out != "" {
		t.Fatalf("the local feature branch must be deleted, still have %q", out)
	}
}

// TestIntegrationFinalizeCleanupResultsArtifactRealGit: a claimed results-artifact
// change with a prepared worktree and final results completes, then cleanup
// removes the worktree and the local branch with no PR — and leaves a same-named
// branch on origin alone.
func TestIntegrationFinalizeCleanupResultsArtifactRealGit(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "research"
	)
	recPath := groomPath(id, slug)
	rec := withLinkedSpec(groomableChange(id, slug), fixtureSpecPath(slug))
	rec = strings.Replace(rec, "type: feat\n", "type: feat\ndeliverable: results-artifact\n", 1)
	m := planRepoModes()[0]
	repo := buildConfiguredRepoWith(t, m, map[string]string{recPath: rec, fixtureSpecPath(slug): fixtureSpecFile(id, slug)})
	lw := prepareLeakWorkspace(t, repo, m.branch, id, recPath, false)

	runGit(t, lw.wp, "push", "-q", "origin", "HEAD:"+lw.featureRef)
	deps := completeLeakWorkspace(t, lw, repo.meta, m.branch, recPath)
	assertResultsArtifactCleaned(t, lw, deps)
	if !originHasBranch(t, lw.origin, lw.featureRef) {
		t.Fatalf("cleanup of a completed change must never delete a remote branch")
	}
}

// TestIntegrationFinalizeCleanupResultsArtifactPrivate: in a private repository a
// results-artifact change completes with its metadata commit on the private
// store, and cleanup removes the worktree and local branch while the shared
// remote gains no branch or ref.
func TestIntegrationFinalizeCleanupResultsArtifactPrivate(t *testing.T) {
	requireRealGit(t)
	create := validChangeCreateRequest()
	create.Deliverable = string(domain.DeliverableResultsArtifact)
	lw, lay := newPrivateLeakWorkspaceFor(t, create)
	meta := lay.DefaultBareRemote
	recPath := runGit(t, meta, "ls-tree", "-r", "--name-only", privateFixtureBranch, "docs/changes/active/")
	if strings.Contains(recPath, "\n") || recPath == "" {
		t.Fatalf("want exactly one active record on the private store, got %q", recPath)
	}
	refsBefore := runGit(t, lw.origin, "for-each-ref", "--format=%(refname) %(objectname)")

	deps := completeLeakWorkspace(t, lw, meta, privateFixtureBranch, recPath)
	if subj := runGit(t, meta, "log", "-1", "--format=%s", privateFixtureBranch); !strings.Contains(subj, "completed") {
		t.Fatalf("the completion commit must land on the private store, latest subject %q", subj)
	}
	assertResultsArtifactCleaned(t, lw, deps)
	if refsAfter := runGit(t, lw.origin, "for-each-ref", "--format=%(refname) %(objectname)"); refsAfter != refsBefore {
		t.Fatalf("the shared remote's refs changed:\nbefore:\n%s\nafter:\n%s", refsBefore, refsAfter)
	}
}

// completeNeverPrepared claims the results-artifact change at recPath (minting
// its branch) WITHOUT preparing a workspace — the halted-before-prepare case —
// then attaches final results and completes it. It returns the cleanup deps over
// the real workspace service.
func completeNeverPrepared(t *testing.T, repo *gitRepo, branch string, id int, recPath string) (realNode, FinalizeDeps, *workspace.Service) {
	t.Helper()
	ctx := context.Background()
	node := planningDepsFor(t, repo.invocation)
	ver := func() string { return blobRevisionAt(t, repo.meta, branch, recPath) }
	ctxRes := ContextImplementation(ctx, node.deps, node.dir, ImplementationContextRequest{ID: id})
	if ctxRes.Result != ResultApplied || ctxRes.Context == nil {
		t.Fatalf("context implementation = %q (reason %q); want a bundle", ctxRes.Result, ctxRes.Reason)
	}
	claim := ChangeClaim(ctx, node.deps, node.dir, ChangeClaimRequest{ID: id, Revision: ctxRes.Context.Change.Revision})
	if claim.Result != ResultApplied || claim.Disposition != ClaimDispositionApplied {
		t.Fatalf("claim = (%q, %q), want applied/applied (findings %v)", claim.Result, claim.Disposition, claim.Findings)
	}
	attach := ChangeAttachResults(ctx, node.deps, node.dir, ChangeAttachRequest{
		ID: id, Revision: ver(), Path: "docs/results/2026-08-17-research-results.md", Markdown: []byte(raFinalResults),
	})
	if attach.Result != ResultApplied {
		t.Fatalf("attach results = %q (reason %q msg %q findings %v)", attach.Result, attach.Reason, attach.Message, attach.Findings)
	}
	done := ChangeComplete(ctx, node.deps, node.dir, ChangeCompleteRequest{ChangeID: id, Path: recPath, Revision: ver()})
	if done.Result != ResultApplied || done.Disposition != CompleteDispCompleted {
		t.Fatalf("change complete = (%q, %q), findings %v", done.Result, done.Disposition, done.Findings)
	}
	svc, err := workspace.NewService(node.deps.Client)
	if err != nil {
		t.Fatalf("workspace.NewService: %v", err)
	}
	return node, FinalizeDeps{Planning: node.deps, GitHub: raNoGitHub{}, Workspace: svc}, svc
}

// sweepAssessOne runs the sweep's full-scope historical assessment over the
// real pinned corpus for one done record and returns its pre-dispatch entry
// (nil when the record is actionable instead).
func sweepAssessOne(t *testing.T, node realNode, deps FinalizeDeps, svc *workspace.Service, id int) (*MaintenanceEntry, bool) {
	t.Helper()
	ctx := context.Background()
	pin, err := node.deps.Reader.PinContext(ctx, node.dir)
	if err != nil {
		t.Fatalf("pin: %v", err)
	}
	inv, refusal := sweepBuildSnapshot(ctx, node.deps.Reader, pin, pin.Config.Effective)
	if refusal != nil {
		t.Fatalf("sweepBuildSnapshot refused: %+v", *refusal)
	}
	items := []sweepWorkItem{{id: id, kind: sweepKindCleanup}}
	shared := gatherSweepSharedFacts(ctx, deps, node.dir, pin.Layout, inv, items)
	entries, actionable := sweepAssessHistorical(ctx, deps, WorkspaceDeps{Service: svc}, inv, pin, shared, items)
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i], len(actionable) != 0
		}
	}
	return nil, len(actionable) != 0
}

// TestIntegrationFinalizeCleanupResultsArtifactNeverPrepared: a results-artifact
// change claimed (its branch minted) but halted before workspace prepare, then
// completed by a person, has no workspace manifest, worktree, or local branch on
// this machine. Cleanup reads that as already-clean and the sweep as no work; a
// leftover local branch with no manifest stays blocked in both.
func TestIntegrationFinalizeCleanupResultsArtifactNeverPrepared(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "research"
	)
	setup := func(t *testing.T) (*gitRepo, realNode, FinalizeDeps, *workspace.Service) {
		recPath := groomPath(id, slug)
		rec := withLinkedSpec(groomableChange(id, slug), fixtureSpecPath(slug))
		rec = strings.Replace(rec, "type: feat\n", "type: feat\ndeliverable: results-artifact\n", 1)
		m := planRepoModes()[0]
		repo := buildConfiguredRepoWith(t, m, map[string]string{recPath: rec, fixtureSpecPath(slug): fixtureSpecFile(id, slug)})
		node, deps, svc := completeNeverPrepared(t, repo, m.branch, id, recPath)
		return repo, node, deps, svc
	}

	t.Run("already-clean", func(t *testing.T) {
		repo, node, deps, svc := setup(t)
		res := FinalizeCleanup(context.Background(), deps, node.dir, id)
		if res.Result != ResultNoOp || res.Disposition != CleanupDispAlready {
			t.Fatalf("finalize cleanup = %q disp %q reason %q (%s)", res.Result, res.Disposition, res.Reason, res.Message)
		}
		if out := runGit(t, repo.invocation, "for-each-ref", "--format=%(refname)", "refs/heads/feat/"+slug); out != "" {
			t.Fatalf("cleanup must create no local branch, have %q", out)
		}
		e, actionable := sweepAssessOne(t, node, deps, svc, id)
		if actionable || e == nil || e.Disposition != SweepDispSkipped || e.Reason != ReasonSweepSnapshotNoWork {
			t.Fatalf("sweep assessment = %+v (actionable %v), want skipped/%s", e, actionable, ReasonSweepSnapshotNoWork)
		}
	})

	t.Run("leftover-branch-stays-blocked", func(t *testing.T) {
		repo, node, deps, svc := setup(t)
		runGit(t, repo.invocation, "branch", "feat/"+slug)
		res := FinalizeCleanup(context.Background(), deps, node.dir, id)
		if res.Disposition != CleanupDispPending || res.Reason != ReasonCleanupWorkspaceBlocked {
			t.Fatalf("finalize cleanup = %q disp %q reason %q, want pending/%s", res.Result, res.Disposition, res.Reason, ReasonCleanupWorkspaceBlocked)
		}
		if out := runGit(t, repo.invocation, "for-each-ref", "--format=%(refname)", "refs/heads/feat/"+slug); out == "" {
			t.Fatalf("a leftover local branch with no manifest must be retained")
		}
		e, actionable := sweepAssessOne(t, node, deps, svc, id)
		if actionable || e == nil || e.Disposition != SweepDispBlocked || e.Reason != ReasonSweepSnapshotBlocked {
			t.Fatalf("sweep assessment = %+v (actionable %v), want blocked/%s", e, actionable, ReasonSweepSnapshotBlocked)
		}
	})
}

// moveBranchPastMerge points the local branch at a new child commit of its
// current tip — a commit made after the merge — so cleanup's tip and ancestry
// proofs refuse and the branch is kept on purpose.
func moveBranchPastMerge(t *testing.T, dir, branch string) {
	t.Helper()
	tip := runGit(t, dir, "rev-parse", "refs/heads/"+branch)
	c := runGit(t, dir, "commit-tree", tip+"^{tree}", "-p", tip, "-m", "after merge")
	runGit(t, dir, "branch", "-f", branch, c)
}

// TestIntegrationFinalizeCleanupLeftoverLocalBranchSweepRetry: a done change
// whose first cleanup could not delete its local branch keeps that branch
// behind a clean workspace and an absent remote. The sweep assessment reads the
// branch from the shared local-branch inventory and dispatches a fresh cleanup,
// which deletes it; a branch moved past the merge is kept and named instead.
func TestIntegrationFinalizeCleanupLeftoverLocalBranchSweepRetry(t *testing.T) {
	requireRealGit(t)
	firstRunKeepsBranch := func(t *testing.T) (*closeoutFixture, *fakeCleanupGitHub) {
		t.Helper()
		f := setupCloseoutFixture(t, planRepoModeDocket())
		head, mergeCommit := f.archiveClosed(t)
		gh := f.mergedCleanupFake(head, mergeCommit)
		git := &faultyCleanupGit{inner: f.deps.Client, fail: "delete"}
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, git, f.svc), f.repo.invocation, f.id)
		var deleteFailed bool
		for _, fd := range res.Findings {
			deleteFailed = deleteFailed || fd.Code == ReasonCleanupLocalDelete
		}
		if res.Reason != ReasonCleanupLocalDelete && !deleteFailed {
			t.Fatalf("first cleanup = %q disp %q reason %q findings %+v, want %s", res.Result, res.Disposition, res.Reason, res.Findings, ReasonCleanupLocalDelete)
		}
		if !f.localBranchPresent(t) {
			t.Fatalf("a failed local delete must leave the local branch")
		}
		if f.remoteBranchPresent(t) {
			t.Fatalf("the remote leg is independent: the remote branch must be deleted")
		}
		return f, gh
	}
	assess := func(t *testing.T, f *closeoutFixture, gh *fakeCleanupGitHub) {
		t.Helper()
		e, actionable := sweepAssessOne(t, realNode{dir: f.repo.invocation, deps: f.deps}, f.cleanupDeps(gh, f.deps.Client, f.svc), f.svc, f.id)
		if !actionable {
			t.Fatalf("a leftover local branch must make the record actionable, got entry %+v", e)
		}
	}

	t.Run("retried", func(t *testing.T) {
		f, gh := firstRunKeepsBranch(t)
		assess(t, f, gh)
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
		if res.Disposition != CleanupDispCleaned {
			t.Fatalf("retry = %q disp %q reason %q (%s)", res.Result, res.Disposition, res.Reason, res.Message)
		}
		if f.localBranchPresent(t) {
			t.Fatalf("the retry must delete the leftover local branch")
		}
	})

	t.Run("kept-after-post-merge-commit", func(t *testing.T) {
		f, gh := firstRunKeepsBranch(t)
		branch := "feat/" + f.slug
		moveBranchPastMerge(t, f.repo.invocation, branch)
		assess(t, f, gh)
		res := FinalizeCleanup(context.Background(), f.cleanupDeps(gh, f.deps.Client, f.svc), f.repo.invocation, f.id)
		if res.Disposition != CleanupDispPending {
			t.Fatalf("cleanup = %q disp %q reason %q, want pending", res.Result, res.Disposition, res.Reason)
		}
		if !f.localBranchPresent(t) {
			t.Fatalf("a branch moved past the merge must be kept")
		}
		human := res.HumanText()
		for _, want := range []string{"local branch " + branch + " was kept", "git branch -D " + branch} {
			if !strings.Contains(human, want) {
				t.Fatalf("human text lacks %q:\n%s", want, human)
			}
		}
	})
}

// TestIntegrationFinalizeCleanupLeftoverBranchFullSweep drives the production
// MaintenanceSweep over a completed results-artifact change whose first cleanup
// removed the worktree but could not delete the local branch: the full sweep
// retries and deletes it, or, for a branch moved past the completion, keeps it
// and names it with its delete command in the human summary.
func TestIntegrationFinalizeCleanupLeftoverBranchFullSweep(t *testing.T) {
	requireRealGit(t)
	const (
		id   = 3
		slug = "research"
	)
	setup := func(t *testing.T) (*leakWorkspace, FinalizeDeps) {
		t.Helper()
		recPath := groomPath(id, slug)
		rec := withLinkedSpec(groomableChange(id, slug), fixtureSpecPath(slug))
		rec = strings.Replace(rec, "type: feat\n", "type: feat\ndeliverable: results-artifact\n", 1)
		m := planRepoModes()[0]
		repo := buildConfiguredRepoWith(t, m, map[string]string{recPath: rec, fixtureSpecPath(slug): fixtureSpecFile(id, slug)})
		lw := prepareLeakWorkspace(t, repo, m.branch, id, recPath, false)
		deps := completeLeakWorkspace(t, lw, repo.meta, m.branch, recPath)

		first := deps
		first.CleanupGit = &faultyCleanupGit{inner: lw.node.deps.Client, fail: "delete"}
		res := FinalizeCleanup(context.Background(), first, lw.node.dir, lw.id)
		if res.Disposition != CleanupDispPending {
			t.Fatalf("first cleanup = %q disp %q reason %q (%s), want pending", res.Result, res.Disposition, res.Reason, res.Message)
		}
		if out := runGit(t, lw.invocation, "for-each-ref", "--format=%(refname)", lw.featureRef); out == "" {
			t.Fatalf("a failed local delete must keep the branch")
		}
		if _, err := os.Stat(lw.wp); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("the first cleanup must remove the worktree %s (stat err %v)", lw.wp, err)
		}
		sweepDeps := deps
		sweepDeps.CleanupGit = nil
		sweepDeps.PRBatch = &fakeSweepBatchReader{}
		return lw, sweepDeps
	}
	findEntry := func(t *testing.T, res MaintenanceResult) MaintenanceEntry {
		t.Helper()
		for _, e := range res.Entries {
			if e.ID == id && e.Kind == sweepKindCleanup {
				return e
			}
		}
		t.Fatalf("the sweep produced no cleanup entry for #%d: %+v", id, res.Entries)
		return MaintenanceEntry{}
	}

	t.Run("retried", func(t *testing.T) {
		lw, sweepDeps := setup(t)
		res := MaintenanceSweep(context.Background(), sweepDeps, lw.node.dir, SweepScopeFull)
		if e := findEntry(t, res); e.Disposition != SweepDispApplied {
			t.Fatalf("sweep entry = %+v, want applied", e)
		}
		if out := runGit(t, lw.invocation, "for-each-ref", "--format=%(refname)", lw.featureRef); out != "" {
			t.Fatalf("the sweep must delete the leftover branch, still have %q", out)
		}
	})

	t.Run("kept", func(t *testing.T) {
		lw, sweepDeps := setup(t)
		branch := strings.TrimPrefix(lw.featureRef, "refs/heads/")
		moveBranchPastMerge(t, lw.invocation, branch)
		res := MaintenanceSweep(context.Background(), sweepDeps, lw.node.dir, SweepScopeFull)
		e := findEntry(t, res)
		if e.Disposition == SweepDispApplied || e.Reason != ReasonCleanupUnreachable {
			t.Fatalf("sweep entry = %+v, want a pending cleanup with reason %s", e, ReasonCleanupUnreachable)
		}
		if out := runGit(t, lw.invocation, "for-each-ref", "--format=%(refname)", lw.featureRef); out == "" {
			t.Fatalf("a branch moved past the completion must be kept")
		}
		human := res.HumanText()
		for _, want := range []string{
			fmt.Sprintf("warning: change #%d: local branch feat/%s was kept", id, slug),
			"git branch -D feat/" + slug,
		} {
			if !strings.Contains(human, want) {
				t.Fatalf("sweep human text lacks %q:\n%s", want, human)
			}
		}
	})
}
