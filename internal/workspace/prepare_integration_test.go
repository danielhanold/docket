//go:build integration

package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationWorkspaceSetupPrepareFreshUnstacked prepares an unstacked target whose base is the
// fetched integration branch. Origin main is advanced after the clone, leaving
// the primary's origin/main tracking ref stale; the prepared base equals
// origin's CURRENT commit, proving Prepare fetched rather than trusting the
// cached ref. Preservation of every uninvolved worktree is asserted.
func TestIntegrationWorkspaceSetupPrepareFreshUnstacked(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)

		stale := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "origin/main"))
		current := r.advanceMain(t)
		if stale == current {
			t.Fatalf("advanceMain did not move origin main (fixture bug)")
		}
		// The tracking ref is still stale until Prepare fetches.
		if got := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "origin/main")); got != stale {
			t.Fatalf("origin/main tracking ref = %q; want stale %q before Prepare", got, stale)
		}

		base := resolveBase(t, []domain.ChangeSpec{{ID: 7, Status: domain.StatusProposed}}, nil, 7)
		tgt, err := NewTarget(7, prepSlug, base, "feat/"+prepSlug)
		if err != nil {
			t.Fatalf("NewTarget: %v", err)
		}

		before := r.snapshotAll(t)
		ws, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		assertFreshCreated(t, r, repo, ws, current)
		r.assertAllUnchanged(t, before)
	})
}

// TestIntegrationWorkspaceSetupPrepareFreshLiveParentStack prepares a target stacked on a live parent
// whose remote branch is the resolved base; the workspace starts at that
// parent branch's commit, not integration.
func TestIntegrationWorkspaceSetupPrepareFreshLiveParentStack(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		parentTip := r.pushBranch(t, "feat/five", "main")
		svc, repo := r.newService(t)

		base := resolveBase(t, []domain.ChangeSpec{
			{ID: 5, Status: domain.StatusInProgress, Branch: present("feat/five")},
			{ID: 7, Status: domain.StatusProposed, StackedOn: parentOf(5)},
		}, []string{"feat/five"}, 7)
		if base.Branch != "feat/five" {
			t.Fatalf("resolved base branch = %q; want feat/five", base.Branch)
		}
		tgt, err := NewTarget(7, prepSlug, base, "feat/"+prepSlug)
		if err != nil {
			t.Fatalf("NewTarget: %v", err)
		}

		before := r.snapshotAll(t)
		ws, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		if ws.BaseRef != gitcli.RefName("refs/heads/feat/five") {
			t.Errorf("BaseRef = %q; want refs/heads/feat/five", ws.BaseRef)
		}
		assertFreshCreated(t, r, repo, ws, parentTip)
		r.assertAllUnchanged(t, before)
	})
}

// TestIntegrationWorkspaceSetupPrepareFreshDoneParent prepares a target stacked on a DONE parent, which
// resolves terminally to the integration branch: the workspace starts at
// origin main, not at the parent branch.
func TestIntegrationWorkspaceSetupPrepareFreshDoneParent(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		r.pushBranch(t, "feat/five", "main") // exists remotely but is bypassed by rule 3
		svc, repo := r.newService(t)
		mainTip := gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "main"))

		base := resolveBase(t, []domain.ChangeSpec{
			{ID: 5, Status: domain.StatusDone, Branch: present("feat/five")},
			{ID: 7, Status: domain.StatusProposed, StackedOn: parentOf(5)},
		}, []string{"feat/five"}, 7)
		if base.Branch != "main" {
			t.Fatalf("resolved base branch = %q; want main (done parent -> integration)", base.Branch)
		}
		tgt, err := NewTarget(7, prepSlug, base, "feat/"+prepSlug)
		if err != nil {
			t.Fatalf("NewTarget: %v", err)
		}

		before := r.snapshotAll(t)
		ws, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		assertFreshCreated(t, r, repo, ws, mainTip)
		r.assertAllUnchanged(t, before)
	})
}

// TestIntegrationWorkspaceSetupPrepareFreshStackedMergedRecurse prepares a target whose immediate parent
// is a branchless stacked-merged change; resolution recurses through it to the
// grandparent's branch, and the workspace starts there.
func TestIntegrationWorkspaceSetupPrepareFreshStackedMergedRecurse(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		grandTip := r.pushBranch(t, "feat/four", "main")
		svc, repo := r.newService(t)

		base := resolveBase(t, []domain.ChangeSpec{
			{ID: 4, Status: domain.StatusInProgress, Branch: present("feat/four")},
			{ID: 5, Status: domain.StatusStackedMerged, StackedOn: parentOf(4)},
			{ID: 7, Status: domain.StatusProposed, StackedOn: parentOf(5)},
		}, []string{"feat/four"}, 7)
		if base.Branch != "feat/four" {
			t.Fatalf("resolved base branch = %q; want feat/four", base.Branch)
		}
		tgt, err := NewTarget(7, prepSlug, base, "feat/"+prepSlug)
		if err != nil {
			t.Fatalf("NewTarget: %v", err)
		}

		before := r.snapshotAll(t)
		ws, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		assertFreshCreated(t, r, repo, ws, grandTip)
		r.assertAllUnchanged(t, before)
	})
}

// TestIntegrationWorkspaceSetupPrepareReturnsReinspectedFacts proves the returned HeadCommit is a value
// read back from Git after creation (the branch tip via ResolveRef), not an
// echo of the requested base — they are equal here, and the assertion pins that
// the reported head is the branch's actual current commit.
func TestIntegrationWorkspaceSetupPrepareReturnsReinspectedFacts(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)

	base := resolveBase(t, []domain.ChangeSpec{{ID: 7, Status: domain.StatusProposed}}, nil, 7)
	tgt, err := NewTarget(7, prepSlug, base, "feat/"+prepSlug)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	ws, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	wantHead := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", string(prepFeatureRef())))
	if ws.HeadCommit != wantHead {
		t.Errorf("HeadCommit = %q; want branch tip %q read back via git", ws.HeadCommit, wantHead)
	}
}

// TestIntegrationWorkspaceSetupPrepareRejectsMismatchedRepository proves an inconsistent Repository (a
// CommonDir belonging to a different repository) is rejected as invalid-input at
// the validate stage, before any directory or branch is created.
func TestIntegrationWorkspaceSetupPrepareRejectsMismatchedRepository(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)

	other := mainModeRepo(t)
	_, otherRepo := other.newService(t)

	bad := repo
	bad.CommonDir = otherRepo.CommonDir

	base := resolveBase(t, []domain.ChangeSpec{{ID: 7, Status: domain.StatusProposed}}, nil, 7)
	tgt, err := NewTarget(7, prepSlug, base, "feat/"+prepSlug)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	_, err = svc.Prepare(context.Background(), PrepareRequest{Repository: bad, Remote: "origin", Target: tgt})
	if err == nil {
		t.Fatalf("Prepare with mismatched repository = nil error; want rejection")
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error %v is not a *Failure", err)
	}
	if f.Kind != KindInvalidInput {
		t.Errorf("Kind = %q; want %q", f.Kind, KindInvalidInput)
	}
	if f.Stage != "validate" {
		t.Errorf("Stage = %q; want validate", f.Stage)
	}
	assertNothingCreated(t, r, repo.CommonDir)
}

// TestIntegrationWorkspaceSetupPrepareFetchFailureCreatesNothing proves a base branch that is absent on
// the remote fails the fetch with an external failure and leaves no checkout,
// branch, or manifest behind (fetch precedes any manifest publication).
func TestIntegrationWorkspaceSetupPrepareFetchFailureCreatesNothing(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)

		// A resolved base whose branch never exists on the remote.
		base := domain.EffectiveBase{Kind: domain.BaseResolved, Branch: "ghostbase"}
		tgt, err := NewTarget(7, prepSlug, base, "feat/"+prepSlug)
		if err != nil {
			t.Fatalf("NewTarget: %v", err)
		}

		before := r.snapshotAll(t)
		_, err = svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		if err == nil {
			t.Fatalf("Prepare against absent remote base = nil error; want failure")
		}
		f, ok := AsFailure(err)
		if !ok {
			t.Fatalf("error %v is not a *Failure", err)
		}
		if f.Kind != KindExternal {
			t.Errorf("Kind = %q; want %q", f.Kind, KindExternal)
		}
		if f.Stage != "fetch" {
			t.Errorf("Stage = %q; want fetch", f.Stage)
		}
		assertNothingCreated(t, r, repo.CommonDir)
		r.assertAllUnchanged(t, before)
	})
}

// TestIntegrationWorkspaceSetupPrepareInvocationMatrix is the CWD/symlink invocation matrix (spec
// §"Real-Git workspace matrix" first bullet). It seeds gitcli.Discover from five
// spellings of the SAME repository — the primary checkout, inside `.docket/`,
// inside another feature worktree, a nested subdirectory, and a symlinked
// spelling of the primary path — and requires every one to resolve ONE canonical
// repository identity and therefore ONE canonical workspace location: the same
// hashed metadata directory and the same checkout path, with the first Prepare
// creating it and every later invocation adopting it as existing. The workspace
// location is derived from Repository.PrimaryWorktree, never from CWD, so a
// caller's directory can never fork it into a second checkout.
func TestIntegrationWorkspaceSetupPrepareInvocationMatrix(t *testing.T) {
	requireGit(t)
	r := docketModeRepo(t) // the topology carrying `.docket/` and a sibling feature worktree
	ctx := context.Background()

	c, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	svc, err := NewService(c)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	tgt := freshTarget(t, 7)

	// A real nested subdirectory beneath the primary checkout (an empty directory
	// is invisible to git status, so it does not perturb any preservation proof).
	nested := filepath.Join(r.Primary, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	// A symlinked spelling of the primary path, in a separate temp root, so the
	// invocation path differs from the canonical one by a real symlink hop (on top
	// of the macOS /tmp -> /private/tmp hop testsupport.TempDir(t) already provides).
	linkParent := testsupport.TempDir(t)
	link := filepath.Join(linkParent, "primary-link")
	if err := os.Symlink(r.Primary, link); err != nil {
		t.Fatal(err)
	}

	invocations := []struct{ name, path string }{
		{"primary", r.Primary},
		{"dot-docket", filepath.Join(r.Primary, ".docket")},
		{"sibling-feature-worktree", filepath.Join(r.Primary, ".worktrees", "other")},
		{"nested-subdir", nested},
		{"symlinked-primary", link},
	}

	var canonical gitcli.Repository
	var wantPath, wantDir string
	for i, inv := range invocations {
		repo, err := c.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: inv.path})
		if err != nil {
			t.Fatalf("Discover from %s (%s): %v", inv.name, inv.path, err)
		}
		if i == 0 {
			canonical = repo
			wantPath = filepath.Join(repo.PrimaryWorktree, ".worktrees", prepSlug)
			wantDir = workspaceDir(repo.CommonDir, tgt.FeatureRef)
		} else if repo != canonical {
			t.Errorf("Discover from %s resolved %+v; want the canonical identity %+v", inv.name, repo, canonical)
		}

		ws, err := svc.Prepare(ctx, PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		if err != nil {
			t.Fatalf("Prepare from %s: %v", inv.name, err)
		}
		wantDisp := PrepareExisting
		if i == 0 {
			wantDisp = PrepareCreated
		}
		if ws.Disposition != wantDisp {
			t.Errorf("Prepare from %s: Disposition = %q; want %q", inv.name, ws.Disposition, wantDisp)
		}
		if ws.Path != wantPath {
			t.Errorf("Prepare from %s: Path = %q; want the one canonical location %q", inv.name, ws.Path, wantPath)
		}
		if got := workspaceDir(repo.CommonDir, tgt.FeatureRef); got != wantDir {
			t.Errorf("Prepare from %s: manifest dir = %q; want %q", inv.name, got, wantDir)
		}
	}

	// Exactly one feature worktree was registered across all five invocations, and
	// exactly one ready manifest exists at the single canonical location.
	wl := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
	if n := countWorktreePathOccurrences(wl, wantPath); n != 1 {
		t.Errorf("feature worktree registered %d times; want exactly one canonical registration:\n%s", n, wl)
	}
	m, present, err := loadManifest(wantDir)
	if err != nil || !present {
		t.Fatalf("loadManifest(%s): present=%v err=%v; want one present manifest", wantDir, present, err)
	}
	if m.Phase != PhaseReady {
		t.Errorf("manifest phase = %q; want ready", m.Phase)
	}
}

// TestIntegrationWorkspaceSetupPrepareExistingIdempotent proves a second Prepare on a ready workspace
// returns `existing` and mutates nothing: commits, staged bytes, dirty tracked
// bytes, and untracked files created between the two calls all survive
// byte-identically, and Dirty is reported true, never repaired.
func TestIntegrationWorkspaceSetupPrepareExistingIdempotent(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)

		first := prepareOK(t, svc, repo, tgt)
		if first.Disposition != PrepareCreated {
			t.Fatalf("first Prepare disposition = %q; want created", first.Disposition)
		}
		ws := wsPathOf(repo)

		// Mutate the workspace between calls: a commit, a staged file, a dirty
		// tracked file, and an untracked file.
		writeWorktreeFile(t, ws, "feature.go", "package feature\n")
		gitOut(t, ws, "add", "feature.go")
		gitOut(t, ws, "commit", "-q", "-m", "feature work")
		committedTip := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))

		writeWorktreeFile(t, ws, "staged.txt", "staged bytes\n")
		gitOut(t, ws, "add", "staged.txt")
		writeWorktreeFile(t, ws, "main.go", "package main // dirtied\n")
		writeWorktreeFile(t, ws, "untracked.txt", "untracked bytes\n")

		beforeWs := snapshotTree(t, ws)
		beforePreserve := r.snapshotAll(t)

		second, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		if err != nil {
			t.Fatalf("second Prepare: %v", err)
		}
		if second.Disposition != PrepareExisting {
			t.Errorf("second Prepare disposition = %q; want existing", second.Disposition)
		}
		if !second.Dirty {
			t.Errorf("second Prepare Dirty = false; want true (dirty reported)")
		}
		if second.HeadCommit != committedTip {
			t.Errorf("HeadCommit = %q; want committed tip %q", second.HeadCommit, committedTip)
		}
		if second.BaseCommit != first.BaseCommit {
			t.Errorf("BaseCommit = %q; want recorded %q", second.BaseCommit, first.BaseCommit)
		}

		// Nothing repaired: the workspace is byte-identical, and every uninvolved
		// worktree is unchanged. Manifest is still ready (not rewritten to nonsense).
		assertUnchanged(t, beforeWs, ws)
		r.assertAllUnchanged(t, beforePreserve)
		if m, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !present || m.Phase != PhaseReady {
			t.Errorf("manifest present=%v phase=%v err=%v; want present ready", present, m.Phase, err)
		}
	})
}

// TestIntegrationWorkspaceSetupPrepareResumeCreateBoth is interrupted-allocation resume arm (i): an
// allocating manifest with no branch and no worktree. Resume creates both at the
// recorded base and advances to ready as `resumed`.
func TestIntegrationWorkspaceSetupPrepareResumeCreateBoth(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		base := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main"))

		writeStateManifest(t, repo, tgt, base, PhaseAllocating)
		if branchExists(r.Primary, "feat/"+prepSlug) {
			t.Fatalf("fixture: branch already exists")
		}

		before := r.snapshotAll(t)
		ws := prepareOK(t, svc, repo, tgt)
		if ws.Disposition != PrepareResumed {
			t.Errorf("disposition = %q; want resumed", ws.Disposition)
		}
		if ws.BaseCommit != base {
			t.Errorf("BaseCommit = %q; want %q", ws.BaseCommit, base)
		}
		if got := localBranchTip(t, r); got != base {
			t.Errorf("branch tip = %q; want base %q", got, base)
		}
		if got := symbolicHead(t, wsPathOf(repo)); got != string(prepFeatureRef()) {
			t.Errorf("workspace symbolic HEAD = %q; want %q", got, prepFeatureRef())
		}
		if m, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !present || m.Phase != PhaseReady {
			t.Errorf("manifest present=%v phase=%v err=%v; want present ready", present, m.Phase, err)
		}
		r.assertAllUnchanged(t, before)
	})
}

// TestIntegrationWorkspaceSetupPrepareResumeAttach is resume arm (ii): an allocating manifest and a
// branch already at the recorded base, but no worktree. Resume attaches the
// existing branch and NEVER moves its tip, even though origin advanced meanwhile.
func TestIntegrationWorkspaceSetupPrepareResumeAttach(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		base := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main"))

		writeStateManifest(t, repo, tgt, base, PhaseAllocating)
		gitOut(t, r.Primary, "branch", "feat/"+prepSlug, string(base))

		// Origin moves forward after the branch was created; resume must not follow.
		moved := r.advanceMain(t)
		if moved == base {
			t.Fatalf("advanceMain did not move origin (fixture bug)")
		}

		before := r.snapshotAll(t)
		ws := prepareOK(t, svc, repo, tgt)
		if ws.Disposition != PrepareResumed {
			t.Errorf("disposition = %q; want resumed", ws.Disposition)
		}
		if got := localBranchTip(t, r); got != base {
			t.Errorf("branch tip = %q; want unchanged base %q (attach must not move it)", got, base)
		}
		if !containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), wsPathOf(repo)) {
			t.Errorf("worktree not registered after attach resume")
		}
		if m, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !present || m.Phase != PhaseReady {
			t.Errorf("manifest present=%v phase=%v err=%v; want present ready", present, m.Phase, err)
		}
		r.assertAllUnchanged(t, before)
	})
}

// TestIntegrationWorkspaceSetupPrepareResumeVerifyOnly is resume arm (iii): an allocating manifest with
// the branch AND a registered worktree already present, carrying a post-creation
// commit and dirty bytes. Resume verifies and advances to ready only; the commit
// and dirty bytes survive.
func TestIntegrationWorkspaceSetupPrepareResumeVerifyOnly(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		base := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main"))
		ws := wsPathOf(repo)

		writeStateManifest(t, repo, tgt, base, PhaseAllocating)
		gitOut(t, r.Primary, "branch", "feat/"+prepSlug, string(base))
		gitOut(t, r.Primary, "worktree", "add", "--", ws, "feat/"+prepSlug)

		// Post-creation commit and dirty bytes.
		writeWorktreeFile(t, ws, "resumed.go", "package resumed\n")
		gitOut(t, ws, "add", "resumed.go")
		gitOut(t, ws, "commit", "-q", "-m", "post-creation commit")
		postTip := gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
		writeWorktreeFile(t, ws, "dirty.txt", "dirty\n")

		beforeWs := snapshotTree(t, ws)

		out := prepareOK(t, svc, repo, tgt)
		if out.Disposition != PrepareResumed {
			t.Errorf("disposition = %q; want resumed", out.Disposition)
		}
		if out.HeadCommit != postTip {
			t.Errorf("HeadCommit = %q; want post-creation tip %q", out.HeadCommit, postTip)
		}
		if !out.Dirty {
			t.Errorf("Dirty = false; want true (post-creation dirty reported)")
		}
		if got := localBranchTip(t, r); got != postTip {
			t.Errorf("branch tip = %q; want post-creation %q (commit preserved)", got, postTip)
		}
		assertUnchanged(t, beforeWs, ws)
		if m, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !present || m.Phase != PhaseReady {
			t.Errorf("manifest present=%v phase=%v err=%v; want present ready", present, m.Phase, err)
		}
	})
}

// TestIntegrationWorkspaceSetupPrepareResumeBranchOffBaseBlocked is blocked case (c): a branch created by
// this manifest that no longer contains the recorded base commit (the base is a
// commit the branch does not reach). Prepare is blocked and byte-untouched: the
// branch is never reset and no worktree is created.
func TestIntegrationWorkspaceSetupPrepareResumeBranchOffBaseBlocked(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)

	c0 := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main"))
	// A base commit that the branch (at c0) does NOT contain: advance origin and
	// fetch the new commit into the primary's object store, then record it as base.
	c1 := r.advanceMain(t)
	gitOut(t, r.Primary, "fetch", "-q", "origin", "main")
	if c1 == c0 {
		t.Fatalf("advanceMain did not move origin (fixture bug)")
	}

	writeStateManifest(t, repo, tgt, c1, PhaseAllocating)
	gitOut(t, r.Primary, "branch", "feat/"+prepSlug, string(c0)) // branch at c0, base is c1

	before := r.snapshotAll(t)
	beforeManifest := readFileBytes(t, filepath.Join(metaDirOf(repo, tgt), manifestFileName))

	out := prepareOK(t, svc, repo, tgt)
	if out.Disposition != PrepareBlocked {
		t.Errorf("disposition = %q; want blocked", out.Disposition)
	}
	if got := localBranchTip(t, r); got != c0 {
		t.Errorf("branch tip = %q; want unchanged %q (never reset)", got, c0)
	}
	if _, err := os.Lstat(wsPathOf(repo)); !os.IsNotExist(err) {
		t.Errorf("workspace path exists (%v); want none created", err)
	}
	if after := readFileBytes(t, filepath.Join(metaDirOf(repo, tgt), manifestFileName)); after != beforeManifest {
		t.Errorf("manifest bytes changed on blocked resume")
	}
	r.assertAllUnchanged(t, before)
}

// TestIntegrationWorkspaceSetupPrepareBlockedMatrix walks the fresh-path blocked matrix: each colliding
// artifact with no matching manifest yields PrepareBlocked and is left
// byte-untouched (pre-Go in-flight work is never adopted).
func TestIntegrationWorkspaceSetupPrepareBlockedMatrix(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		t.Run("target-dir-no-manifest", func(t *testing.T) {
			r := freshTopology(t, r)
			svc, repo := r.newService(t)
			tgt := freshTarget(t, 7)
			writeWorktreeFile(t, wsPathOf(repo), "leftover.txt", "prior bytes\n")
			collidePath := filepath.Join(wsPathOf(repo), "leftover.txt")
			before := readFileBytes(t, collidePath)

			assertBlocked(t, svc, repo, tgt)
			if after := readFileBytes(t, collidePath); after != before {
				t.Errorf("colliding directory bytes changed")
			}
			assertNoManifest(t, repo, tgt)
		})

		t.Run("foreign-registration", func(t *testing.T) {
			r := freshTopology(t, r)
			svc, repo := r.newService(t)
			tgt := freshTarget(t, 7)
			// A foreign detached worktree squatting the target path.
			gitOut(t, r.Primary, "worktree", "add", "--detach", "--", wsPathOf(repo), "main")
			collidePath := filepath.Join(wsPathOf(repo), "main.go")
			before := readFileBytes(t, collidePath)

			assertBlocked(t, svc, repo, tgt)
			if !containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), wsPathOf(repo)) {
				t.Errorf("foreign registration removed; must be preserved (never force-removed)")
			}
			if after := readFileBytes(t, collidePath); after != before {
				t.Errorf("foreign worktree bytes changed")
			}
			assertNoManifest(t, repo, tgt)
		})

		t.Run("local-branch-no-manifest", func(t *testing.T) {
			r := freshTopology(t, r)
			svc, repo := r.newService(t)
			tgt := freshTarget(t, 7)
			gitOut(t, r.Primary, "branch", "feat/"+prepSlug, "main")
			before := localBranchTip(t, r)

			assertBlocked(t, svc, repo, tgt)
			if got := localBranchTip(t, r); got != before {
				t.Errorf("local branch tip changed %q -> %q", before, got)
			}
			assertNoManifest(t, repo, tgt)
		})

		t.Run("remote-branch-no-manifest", func(t *testing.T) {
			r := freshTopology(t, r)
			svc, repo := r.newService(t)
			tgt := freshTarget(t, 7)
			r.pushBranch(t, "feat/"+prepSlug, "main")
			before := gitcli.ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/feat/"+prepSlug))

			assertBlocked(t, svc, repo, tgt)
			if got := gitcli.ObjectID(gitOut(t, r.Origin, "rev-parse", "refs/heads/feat/"+prepSlug)); got != before {
				t.Errorf("remote branch tip changed %q -> %q", before, got)
			}
			if branchExists(r.Primary, "feat/"+prepSlug) {
				t.Errorf("remote branch was adopted locally; must not be")
			}
			assertNoManifest(t, repo, tgt)
		})

		t.Run("malformed-manifest", func(t *testing.T) {
			r := freshTopology(t, r)
			svc, repo := r.newService(t)
			tgt := freshTarget(t, 7)
			if err := os.MkdirAll(metaDirOf(repo, tgt), 0o700); err != nil {
				t.Fatal(err)
			}
			mpath := filepath.Join(metaDirOf(repo, tgt), manifestFileName)
			if err := os.WriteFile(mpath, []byte("{ this is not json"), 0o600); err != nil {
				t.Fatal(err)
			}
			before := readFileBytes(t, mpath)

			assertBlocked(t, svc, repo, tgt)
			if after := readFileBytes(t, mpath); after != before {
				t.Errorf("malformed manifest bytes changed")
			}
		})

		t.Run("foreign-commondir-manifest", func(t *testing.T) {
			r := freshTopology(t, r)
			svc, repo := r.newService(t)
			tgt := freshTarget(t, 7)
			other := mainModeRepo(t)
			_, otherRepo := other.newService(t)
			// A structurally valid manifest owned by a DIFFERENT repository.
			foreign := Manifest{
				Schema: manifestSchemaVersion, ID: workspaceID(tgt.FeatureRef), CommonDir: otherRepo.CommonDir,
				ChangeID: tgt.ChangeID, Slug: tgt.Slug, FeatureRef: tgt.FeatureRef, BaseRef: tgt.BaseRef,
				BaseCommit: gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main")),
				Path:       wsPathOf(repo), Phase: PhaseReady,
				CreatedUTC: time.Now().UTC().Format(time.RFC3339), UpdatedUTC: time.Now().UTC().Format(time.RFC3339),
			}
			if err := writeManifest(metaDirOf(repo, tgt), foreign); err != nil {
				t.Fatalf("writeManifest(foreign): %v", err)
			}
			mpath := filepath.Join(metaDirOf(repo, tgt), manifestFileName)
			before := readFileBytes(t, mpath)

			assertBlocked(t, svc, repo, tgt)
			if after := readFileBytes(t, mpath); after != before {
				t.Errorf("foreign-commondir manifest bytes changed")
			}
		})
	})
}

// TestIntegrationWorkspaceSetupPrepareFreshBlockedByStaleRegistration pins change 0368's fresh-allocation
// tightening: a worktree registration at the intended target path whose directory
// has been removed — a STALE registration — blocks a fresh allocation instead of
// being skip-matched into a silent create. Pre-change, inventoryForFresh proved
// registration absence via registeredAt, which SKIPS an uncanonicalizable path
// (the stale registration's directory is gone); classifyRegistrationAbsence now
// recognizes it fail-closed (regPresent lexically, or regUnresolved), so Prepare
// returns PrepareBlocked, force-removes nothing, and publishes no manifest. Every
// earlier fresh-path leg (local feature ref, remote ref, target path) is left
// clean, so the block is attributable to the registration alone.
func TestIntegrationWorkspaceSetupPrepareFreshBlockedByStaleRegistration(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)

	// Register a worktree at the intended path on a THROWAWAY branch (never the
	// feature ref), then remove its directory: the registration survives in the
	// git metadata while its path can no longer be canonicalized.
	gitOut(t, r.Primary, "worktree", "add", "-b", "throwaway/stale-reg", wsPathOf(repo), "main")
	if err := os.RemoveAll(wsPathOf(repo)); err != nil {
		t.Fatal(err)
	}
	// The registration must still be recorded (RemoveAll does not prune it), or
	// the fixture cannot exercise the stale arm. Match the raw recorded path — the
	// containsWorktreePath helper canonicalizes, which fails on the removed dir.
	beforeList := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
	if !strings.Contains(beforeList, "worktree "+wsPathOf(repo)+"\n") {
		t.Fatalf("stale registration not recorded after RemoveAll; cannot exercise the stale arm:\n%s", beforeList)
	}

	assertBlocked(t, svc, repo, tgt)

	// The stale registration is preserved (never force-removed) and no manifest
	// was published — the fresh allocation is refused, byte-untouched.
	afterList := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
	if !strings.Contains(afterList, "worktree "+wsPathOf(repo)+"\n") {
		t.Errorf("stale registration removed; must be preserved (never force-removed):\n%s", afterList)
	}
	assertNoManifest(t, repo, tgt)
}

// TestIntegrationWorkspaceSetupPrepareProbeFailureCreatesNothing injects a probe failure at the remote
// feature-ref inventory step: a git wrapper that fails `ls-remote` (the only
// inventory probe used by no earlier Prepare step, so identity discovery, base
// fetch, and the local-ref probe all still succeed and the failure lands exactly
// at ProbeRemoteBranch). An errored probe is an external failure, NEVER clean
// absence, so nothing is created (learnings: probe-error-is-not-clean-absence).
func TestIntegrationWorkspaceSetupPrepareProbeFailureCreatesNothing(t *testing.T) {
	r := mainModeRepo(t)
	fakeGit := writeFailingGit(t, "ls-remote")
	svc, repo := r.newServiceWithGit(t, fakeGit)
	tgt := freshTarget(t, 7)

	before := r.snapshotAll(t)
	_, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
	if err == nil {
		t.Fatalf("Prepare with failing ls-remote = nil error; want external failure")
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error %v is not a *Failure", err)
	}
	if f.Kind != KindExternal {
		t.Errorf("Kind = %q; want external", f.Kind)
	}
	if f.Stage != "inventory" {
		t.Errorf("Stage = %q; want inventory", f.Stage)
	}
	assertNothingCreated(t, r, repo.CommonDir)
	r.assertAllUnchanged(t, before)
}
