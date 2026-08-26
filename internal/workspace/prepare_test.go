package workspace

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

// TestPrepareReturnsReinspectedFacts proves the returned HeadCommit is a value
// read back from Git after creation (the branch tip via ResolveRef), not an
// echo of the requested base — they are equal here, and the assertion pins that
// the reported head is the branch's actual current commit.
func TestPrepareReturnsReinspectedFacts(t *testing.T) {
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

// TestPrepareFetchFailureCreatesNothing proves a base branch that is absent on
// the remote fails the fetch with an external failure and leaves no checkout,
// branch, or manifest behind (fetch precedes any manifest publication).
func TestPrepareFetchFailureCreatesNothing(t *testing.T) {
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

// ---------------------------------------------------------------------------
// Task 6: existing / resume / blocked matrix.
//
// These tests construct the on-disk states a crash or a collision leaves and
// assert Prepare's disposition and its byte-for-byte preservation guarantees.
// Blocked is a value disposition (PrepareBlocked, nil error); a probe that
// cannot see a resource is an error. Manual manifests are published through the
// production writeManifest so a constructed state is one loadManifest accepts.
// ---------------------------------------------------------------------------

// TestPrepareInvocationMatrix is the CWD/symlink invocation matrix (spec
// §"Real-Git workspace matrix" first bullet). It seeds gitcli.Discover from five
// spellings of the SAME repository — the primary checkout, inside `.docket/`,
// inside another feature worktree, a nested subdirectory, and a symlinked
// spelling of the primary path — and requires every one to resolve ONE canonical
// repository identity and therefore ONE canonical workspace location: the same
// hashed metadata directory and the same checkout path, with the first Prepare
// creating it and every later invocation adopting it as existing. The workspace
// location is derived from Repository.PrimaryWorktree, never from CWD, so a
// caller's directory can never fork it into a second checkout.
func TestPrepareInvocationMatrix(t *testing.T) {
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
	// of the macOS /tmp -> /private/tmp hop t.TempDir() already provides).
	linkParent := t.TempDir()
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

// countWorktreePathOccurrences counts how many registered worktrees resolve to
// want, canonicalizing each porcelain path through every symlink hop.
func countWorktreePathOccurrences(porcelain, want string) int {
	n := 0
	for _, line := range splitLines(porcelain) {
		p, ok := cutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		if canon, err := filepath.EvalSymlinks(p); err == nil && canon == want {
			n++
		}
	}
	return n
}

// freshTarget builds the unstacked target every matrix scenario prepares.
func freshTarget(t *testing.T, id int) Target {
	t.Helper()
	base := resolveBase(t, []domain.ChangeSpec{{ID: domain.ChangeID(id), Status: domain.StatusProposed}}, nil, domain.ChangeID(id))
	tgt, err := NewTarget(domain.ChangeID(id), prepSlug, base, "feat/"+prepSlug)
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	return tgt
}

// prepareOK runs Prepare and fails the test on any error, returning the result.
func prepareOK(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) Workspace {
	t.Helper()
	ws, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return ws
}

// wsPathOf is the canonical checkout path a target's workspace lands at.
func wsPathOf(repo gitcli.Repository) string {
	return filepath.Join(repo.PrimaryWorktree, ".worktrees", prepSlug)
}

// metaDirOf is the hashed workspace metadata directory for a target.
func metaDirOf(repo gitcli.Repository, tgt Target) string {
	return workspaceDir(repo.CommonDir, tgt.FeatureRef)
}

// writeStateManifest publishes a manifest in the target's metadata directory at
// the requested phase and recorded base, via the production writer. It is how a
// crash-left partial state is constructed for the resume tests.
func writeStateManifest(t *testing.T, repo gitcli.Repository, tgt Target, base gitcli.ObjectID, phase Phase) {
	t.Helper()
	m := Manifest{
		Schema:     manifestSchemaVersion,
		ID:         workspaceID(tgt.FeatureRef),
		CommonDir:  repo.CommonDir,
		ChangeID:   tgt.ChangeID,
		Slug:       tgt.Slug,
		FeatureRef: tgt.FeatureRef,
		BaseRef:    tgt.BaseRef,
		BaseCommit: base,
		Path:       wsPathOf(repo),
		Phase:      phase,
		CreatedUTC: time.Now().UTC().Format(time.RFC3339),
		UpdatedUTC: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeManifest(metaDirOf(repo, tgt), m); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
}

// localBranchTip returns refs/heads/feat/<slug>'s tip in the primary clone.
func localBranchTip(t *testing.T, r *wsRepos) gitcli.ObjectID {
	t.Helper()
	return gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", string(prepFeatureRef())))
}

// TestPrepareExistingIdempotent proves a second Prepare on a ready workspace
// returns `existing` and mutates nothing: commits, staged bytes, dirty tracked
// bytes, and untracked files created between the two calls all survive
// byte-identically, and Dirty is reported true, never repaired.
func TestPrepareExistingIdempotent(t *testing.T) {
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

// TestPrepareResumeCreateBoth is interrupted-allocation resume arm (i): an
// allocating manifest with no branch and no worktree. Resume creates both at the
// recorded base and advances to ready as `resumed`.
func TestPrepareResumeCreateBoth(t *testing.T) {
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

// TestPrepareResumeAttach is resume arm (ii): an allocating manifest and a
// branch already at the recorded base, but no worktree. Resume attaches the
// existing branch and NEVER moves its tip, even though origin advanced meanwhile.
func TestPrepareResumeAttach(t *testing.T) {
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

// TestPrepareResumeVerifyOnly is resume arm (iii): an allocating manifest with
// the branch AND a registered worktree already present, carrying a post-creation
// commit and dirty bytes. Resume verifies and advances to ready only; the commit
// and dirty bytes survive.
func TestPrepareResumeVerifyOnly(t *testing.T) {
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

// TestPrepareResumeBranchOffBaseBlocked is blocked case (c): a branch created by
// this manifest that no longer contains the recorded base commit (the base is a
// commit the branch does not reach). Prepare is blocked and byte-untouched: the
// branch is never reset and no worktree is created.
func TestPrepareResumeBranchOffBaseBlocked(t *testing.T) {
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

// readFileBytes reads a file as a string, failing the test on error.
func readFileBytes(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestPrepareBlockedMatrix walks the fresh-path blocked matrix: each colliding
// artifact with no matching manifest yields PrepareBlocked and is left
// byte-untouched (pre-Go in-flight work is never adopted).
func TestPrepareBlockedMatrix(t *testing.T) {
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

// freshTopology rebuilds the same fixture kind as r for an isolated subtest, so
// each blocked-matrix case runs against its own repository.
func freshTopology(t *testing.T, r *wsRepos) *wsRepos {
	t.Helper()
	if len(r.Preserve) > 1 {
		return docketModeRepo(t)
	}
	return mainModeRepo(t)
}

// assertBlocked runs Prepare and asserts a PrepareBlocked disposition with no error.
func assertBlocked(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) {
	t.Helper()
	out, err := svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
	if err != nil {
		t.Fatalf("Prepare = error %v; want blocked disposition", err)
	}
	if out.Disposition != PrepareBlocked {
		t.Errorf("disposition = %q; want blocked", out.Disposition)
	}
}

// assertNoManifest asserts Prepare published no manifest of its own.
func assertNoManifest(t *testing.T, repo gitcli.Repository, tgt Target) {
	t.Helper()
	if _, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || present {
		t.Errorf("manifest present=%v err=%v; want cleanly absent (none published)", present, err)
	}
}

// TestPrepareConcurrentSameTarget proves two Prepares of the SAME target
// serialize on the operation lock and yield exactly one created plus one
// existing, with exactly one branch and one registration.
func TestPrepareConcurrentSameTarget(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)

	var wg sync.WaitGroup
	results := make([]Workspace, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgt})
		}(i)
	}
	close(start)
	wg.Wait()

	created, existing := 0, 0
	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d Prepare: %v", i, errs[i])
		}
		switch results[i].Disposition {
		case PrepareCreated:
			created++
		case PrepareExisting, PrepareResumed:
			existing++
		default:
			t.Errorf("goroutine %d disposition = %q; want created/existing/resumed", i, results[i].Disposition)
		}
	}
	if created != 1 || existing != 1 {
		t.Errorf("dispositions: created=%d existing/resumed=%d; want 1 and 1", created, existing)
	}

	// Exactly one branch and one registration for the target.
	wl := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
	if n := countPathOccurrences(wl, wsPathOf(repo)); n != 1 {
		t.Errorf("registrations at target path = %d; want 1", n)
	}
	if !branchExists(r.Primary, "feat/"+prepSlug) {
		t.Errorf("feat branch missing after concurrent prepare")
	}
}

// countPathOccurrences counts porcelain "worktree <path>" lines whose canonical
// path equals want.
func countPathOccurrences(porcelain, want string) int {
	n := 0
	for _, line := range splitLines(porcelain) {
		if p, ok := cutPrefix(line, "worktree "); ok {
			if cp, err := filepath.EvalSymlinks(p); err == nil && cp == want {
				n++
			}
		}
	}
	return n
}

// TestPrepareConcurrentDistinctTargets proves two Prepares of DIFFERENT targets
// proceed concurrently (distinct operation locks) and both create successfully.
func TestPrepareConcurrentDistinctTargets(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)

	baseA := resolveBase(t, []domain.ChangeSpec{{ID: 7, Status: domain.StatusProposed}}, nil, 7)
	tgtA, err := NewTarget(7, "alpha-slug", baseA, "feat/alpha-slug")
	if err != nil {
		t.Fatalf("NewTarget A: %v", err)
	}
	baseB := resolveBase(t, []domain.ChangeSpec{{ID: 8, Status: domain.StatusProposed}}, nil, 8)
	tgtB, err := NewTarget(8, "beta-slug", baseB, "feat/beta-slug")
	if err != nil {
		t.Fatalf("NewTarget B: %v", err)
	}

	var wg sync.WaitGroup
	var outA, outB Workspace
	var errA, errB error
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		outA, errA = svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgtA})
	}()
	go func() {
		defer wg.Done()
		<-start
		outB, errB = svc.Prepare(context.Background(), PrepareRequest{Repository: repo, Remote: "origin", Target: tgtB})
	}()
	close(start)
	wg.Wait()

	if errA != nil || errB != nil {
		t.Fatalf("distinct-target Prepare errors: A=%v B=%v", errA, errB)
	}
	if outA.Disposition != PrepareCreated || outB.Disposition != PrepareCreated {
		t.Errorf("dispositions A=%q B=%q; want both created", outA.Disposition, outB.Disposition)
	}
}

// TestPrepareProbeFailureCreatesNothing injects a probe failure at the remote
// feature-ref inventory step: a git wrapper that fails `ls-remote` (the only
// inventory probe used by no earlier Prepare step, so identity discovery, base
// fetch, and the local-ref probe all still succeed and the failure lands exactly
// at ProbeRemoteBranch). An errored probe is an external failure, NEVER clean
// absence, so nothing is created (learnings: probe-error-is-not-clean-absence).
func TestPrepareProbeFailureCreatesNothing(t *testing.T) {
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

// writeFailingGit writes an executable git wrapper that forwards to the real git
// on PATH except for the named subcommand, which it fails with exit 1. The
// wrapper is invoked by absolute path, so PATH still resolves the real git.
func writeFailingGit(t *testing.T, failSubcommand string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "git")
	script := "#!/bin/sh\nif [ \"$1\" = \"" + failSubcommand + "\" ]; then\n  echo \"fake git: $1 disabled for test\" >&2\n  exit 1\nfi\nexec git \"$@\"\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// newServiceWithGit builds a Service whose gitcli.Client uses the given git
// executable, discovering the canonical Repository through that same client.
func (r *wsRepos) newServiceWithGit(t *testing.T, exe string) (*Service, gitcli.Repository) {
	t.Helper()
	c, err := gitcli.NewClient(gitcli.WithExecutable(exe))
	if err != nil {
		t.Fatalf("gitcli.NewClient(WithExecutable): %v", err)
	}
	svc, err := NewService(c)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	repo, err := c.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: r.Primary})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return svc, repo
}
