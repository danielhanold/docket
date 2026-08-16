package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// The Prepare tests exercise the fresh-allocation path against real temporary
// Git repositories with local bare remotes, on both the plain and the
// docket-style topologies. Existing/resume/blocked arms are Task 6.

// prepSlug is the feature slug every fresh-allocation scenario prepares. Its
// derived feature ref is refs/heads/feat/<prepSlug>.
const prepSlug = "fix-the-thing"

func prepFeatureRef() gitcli.RefName { return gitcli.RefName("refs/heads/feat/" + prepSlug) }

// resolveBase wires a real domain.ResolveEffectiveBase outcome — proving the
// service consumes the resolver rather than shadowing its rules — and asserts
// the outcome actually resolved (a fixture bug otherwise).
func resolveBase(t *testing.T, specs []domain.ChangeSpec, branches []string, subject domain.ChangeID) domain.EffectiveBase {
	t.Helper()
	changes := make([]domain.Change, 0, len(specs))
	for _, sp := range specs {
		changes = append(changes, domain.NewChange(sp))
	}
	snap := domain.NewSnapshot(domain.SnapshotSpec{
		Policy:  domain.RepositoryPolicy{IntegrationBranch: "main"},
		Changes: changes,
	})
	set := make(map[string]bool, len(branches))
	for _, b := range branches {
		set[b] = true
	}
	facts := domain.NewBranchFacts(set)
	c, out := snap.Change(subject)
	if out != domain.LookupFound {
		t.Fatalf("Change(%d) = %v; want found", subject, out)
	}
	base := domain.ResolveEffectiveBase(snap, c, facts)
	if base.Kind != domain.BaseResolved {
		t.Fatalf("resolver base kind = %q; want resolved (fixture bug)", base.Kind)
	}
	return base
}

// assertNothingCreated proves a rejected Prepare left no checkout, no local
// feature branch, and no manifest under the real repository.
func assertNothingCreated(t *testing.T, r *wsRepos, commonDir string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(r.Primary, ".worktrees", prepSlug)); !os.IsNotExist(err) {
		t.Errorf(".worktrees/%s exists or is unstattable (%v); want absent", prepSlug, err)
	}
	if branchExists(r.Primary, "feat/"+prepSlug) {
		t.Errorf("local branch feat/%s exists; want absent", prepSlug)
	}
	if commonDir != "" {
		if _, present, err := loadManifest(workspaceDir(commonDir, prepFeatureRef())); err != nil || present {
			t.Errorf("manifest present=%v err=%v; want cleanly absent", present, err)
		}
	}
}

// assertFreshCreated asserts the full created-workspace postcondition: the
// returned disposition and facts, live Git registration at the canonical path on
// the feature branch whose tip is wantBase, and a ready manifest recording the
// exact base commit.
func assertFreshCreated(t *testing.T, r *wsRepos, repo gitcli.Repository, ws Workspace, wantBase gitcli.ObjectID) {
	t.Helper()
	if ws.Disposition != PrepareCreated {
		t.Fatalf("Disposition = %q; want %q", ws.Disposition, PrepareCreated)
	}
	wantPath := filepath.Join(repo.PrimaryWorktree, ".worktrees", prepSlug)
	if ws.Path != wantPath {
		t.Errorf("Path = %q; want %q", ws.Path, wantPath)
	}
	if ws.FeatureRef != prepFeatureRef() {
		t.Errorf("FeatureRef = %q; want %q", ws.FeatureRef, prepFeatureRef())
	}
	if ws.BaseCommit != wantBase {
		t.Errorf("BaseCommit = %q; want %q", ws.BaseCommit, wantBase)
	}
	if ws.HeadCommit != wantBase {
		t.Errorf("HeadCommit = %q; want %q (fresh head == base)", ws.HeadCommit, wantBase)
	}
	if ws.Dirty {
		t.Errorf("Dirty = true; want false on a fresh checkout")
	}

	// Live Git: registered at the canonical path, symbolic HEAD is the feature
	// ref, and the branch tip equals the fetched base.
	if got := symbolicHead(t, wantPath); got != string(prepFeatureRef()) {
		t.Errorf("workspace symbolic HEAD = %q; want %q", got, prepFeatureRef())
	}
	if got := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", string(prepFeatureRef()))); got != wantBase {
		t.Errorf("branch tip = %q; want %q", got, wantBase)
	}
	wl := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
	if !containsWorktreePath(t, wl, wantPath) {
		t.Errorf("worktree list does not register %q:\n%s", wantPath, wl)
	}

	// Manifest advanced to ready with the exact base commit.
	m, present, err := loadManifest(workspaceDir(repo.CommonDir, prepFeatureRef()))
	if err != nil || !present {
		t.Fatalf("loadManifest present=%v err=%v; want present", present, err)
	}
	if m.Phase != PhaseReady {
		t.Errorf("manifest phase = %q; want ready", m.Phase)
	}
	if m.BaseCommit != wantBase {
		t.Errorf("manifest BaseCommit = %q; want %q", m.BaseCommit, wantBase)
	}
	if m.Path != wantPath {
		t.Errorf("manifest Path = %q; want %q", m.Path, wantPath)
	}
}

// containsWorktreePath reports whether the porcelain worktree list registers a
// worktree whose canonical path equals want.
func containsWorktreePath(t *testing.T, porcelain, want string) bool {
	t.Helper()
	for _, line := range splitLines(porcelain) {
		if p, ok := cutPrefix(line, "worktree "); ok {
			cp, err := filepath.EvalSymlinks(p)
			if err != nil {
				continue
			}
			if cp == want {
				return true
			}
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, c := range s {
		if c == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return "", false
}

// TestPrepareFreshUnstacked prepares an unstacked target whose base is the
// fetched integration branch. Origin main is advanced after the clone, leaving
// the primary's origin/main tracking ref stale; the prepared base equals
// origin's CURRENT commit, proving Prepare fetched rather than trusting the
// cached ref. Preservation of every uninvolved worktree is asserted.
func TestPrepareFreshUnstacked(t *testing.T) {
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
		tgt, err := NewTarget(7, prepSlug, base)
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

// TestPrepareFreshLiveParentStack prepares a target stacked on a live parent
// whose remote branch is the resolved base; the workspace starts at that
// parent branch's commit, not integration.
func TestPrepareFreshLiveParentStack(t *testing.T) {
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
		tgt, err := NewTarget(7, prepSlug, base)
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

// TestPrepareFreshDoneParent prepares a target stacked on a DONE parent, which
// resolves terminally to the integration branch: the workspace starts at
// origin main, not at the parent branch.
func TestPrepareFreshDoneParent(t *testing.T) {
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
		tgt, err := NewTarget(7, prepSlug, base)
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

// TestPrepareFreshStackedMergedRecurse prepares a target whose immediate parent
// is a branchless stacked-merged change; resolution recurses through it to the
// grandparent's branch, and the workspace starts there.
func TestPrepareFreshStackedMergedRecurse(t *testing.T) {
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
		tgt, err := NewTarget(7, prepSlug, base)
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

// TestPrepareReturnsReinspectedFacts proves the returned HeadCommit is a value
// read back from Git after creation (the branch tip via ResolveRef), not an
// echo of the requested base — they are equal here, and the assertion pins that
// the reported head is the branch's actual current commit.
func TestPrepareReturnsReinspectedFacts(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)

	base := resolveBase(t, []domain.ChangeSpec{{ID: 7, Status: domain.StatusProposed}}, nil, 7)
	tgt, err := NewTarget(7, prepSlug, base)
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

// TestPrepareRejectsMismatchedRepository proves an inconsistent Repository (a
// CommonDir belonging to a different repository) is rejected as invalid-input at
// the validate stage, before any directory or branch is created.
func TestPrepareRejectsMismatchedRepository(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)

	other := mainModeRepo(t)
	_, otherRepo := other.newService(t)

	bad := repo
	bad.CommonDir = otherRepo.CommonDir

	base := resolveBase(t, []domain.ChangeSpec{{ID: 7, Status: domain.StatusProposed}}, nil, 7)
	tgt, err := NewTarget(7, prepSlug, base)
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

// TestPrepareFetchFailureCreatesNothing proves a base branch that is absent on
// the remote fails the fetch with an external failure and leaves no checkout,
// branch, or manifest behind (fetch precedes any manifest publication).
func TestPrepareFetchFailureCreatesNothing(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)

		// A resolved base whose branch never exists on the remote.
		base := domain.EffectiveBase{Kind: domain.BaseResolved, Branch: "ghostbase"}
		tgt, err := NewTarget(7, prepSlug, base)
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
