package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// The Cleanup tests build each proof-gated scenario against the real-Git harness.
// Cleanup removes ONLY the checkout — never a local or remote branch — and only
// when the manifest, live registration, feature-ref attachment, base reachability,
// and an exact clean tracked/untracked delta all prove out. Every blocked case is
// asserted byte-untouched (the colliding artifact hashed before/after); the local
// branch always survives; and probe failures are `failed`, never a false clean.

// cleanupResult runs Cleanup and returns the result, failing on an unexpected
// error only when wantErr is false.
func cleanupOK(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) CleanupResult {
	t.Helper()
	res, err := svc.Cleanup(context.Background(), CleanupRequest{Repository: repo, Target: tgt})
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	return res
}

// TestCleanupReadyClean proves a ready + clean workspace is removed: the
// registration and checkout directory are gone, the manifest is retained as a
// cleaned tombstone that still loads, and the LOCAL BRANCH survives at its tip.
// Every uninvolved worktree is preserved.
func TestCleanupReadyClean(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		created := prepareOK(t, svc, repo, tgt)
		if created.Disposition != PrepareCreated {
			t.Fatalf("prepare disposition = %q; want created", created.Disposition)
		}
		ws := wsPathOf(repo)
		branchTip := localBranchTip(t, r)

		before := r.snapshotAll(t)
		res := cleanupOK(t, svc, repo, tgt)

		if res.Disposition != CleanupCleaned {
			t.Errorf("Disposition = %q; want cleaned", res.Disposition)
		}
		if res.Path != ws {
			t.Errorf("Path = %q; want %q", res.Path, ws)
		}
		// The checkout directory is gone.
		if _, err := os.Lstat(ws); !os.IsNotExist(err) {
			t.Errorf("workspace dir Lstat err = %v; want not-exist (removed)", err)
		}
		// The registration is gone.
		if containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
			t.Errorf("worktree still registered at %q after cleanup", ws)
		}
		// The manifest is a retained cleaned tombstone that still loads.
		m, present, err := loadManifest(metaDirOf(repo, tgt))
		if err != nil || !present {
			t.Fatalf("tombstone loadManifest present=%v err=%v; want present", present, err)
		}
		if m.Phase != PhaseCleaned {
			t.Errorf("tombstone phase = %q; want cleaned", m.Phase)
		}
		// The local branch STILL EXISTS at its unchanged tip: cleanup never deletes it.
		if !branchExists(r.Primary, "feat/"+prepSlug) {
			t.Errorf("feat branch deleted; cleanup must never delete a branch")
		}
		if got := localBranchTip(t, r); got != branchTip {
			t.Errorf("branch tip = %q; want unchanged %q", got, branchTip)
		}
		r.assertAllUnchanged(t, before)
	})
}

// TestCleanupRetryAlreadyClean proves a second Cleanup after a successful one is
// idempotent: the cleaned tombstone plus the absent registration yield
// already-clean, and nothing further changes.
func TestCleanupRetryAlreadyClean(t *testing.T) {
	eachTopology(t, func(t *testing.T, r *wsRepos) {
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)
		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupCleaned {
			t.Fatalf("first Cleanup = %q; want cleaned", res.Disposition)
		}
		beforeTombstone := readFileBytes(t, filepath.Join(metaDirOf(repo, tgt), manifestFileName))
		branchTip := localBranchTip(t, r)

		res := cleanupOK(t, svc, repo, tgt)
		if res.Disposition != CleanupAlreadyClean {
			t.Errorf("retry Disposition = %q; want already-clean", res.Disposition)
		}
		if after := readFileBytes(t, filepath.Join(metaDirOf(repo, tgt), manifestFileName)); after != beforeTombstone {
			t.Errorf("tombstone bytes changed on already-clean retry")
		}
		if got := localBranchTip(t, r); got != branchTip {
			t.Errorf("branch tip = %q; want unchanged %q", got, branchTip)
		}
	})
}

// TestCleanupDirtyNeverRemoved is the observable data-loss contract: a dirty
// workspace is NEVER removed and its bytes survive. It encodes the reason Cleanup
// removes via the non-forcing RemoveWorktreeClean (git rechecks cleanliness at the
// destructive boundary) rather than a preflight check plus a forced removal.
func TestCleanupDirtyNeverRemoved(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)

	// A dirty tracked file plus an untracked file: the checkout carries unsaved work.
	writeWorktreeFile(t, ws, "main.go", "package main // unsaved edit\n")
	writeWorktreeFile(t, ws, "scratch.txt", "unsaved untracked bytes\n")
	beforeWs := snapshotTree(t, ws)

	res := cleanupOK(t, svc, repo, tgt)
	if res.Disposition != CleanupBlocked {
		t.Errorf("Disposition = %q; want blocked", res.Disposition)
	}
	// NEVER removed: directory, registration, and dirty bytes all survive.
	if _, err := os.Stat(ws); err != nil {
		t.Errorf("workspace dir stat err = %v; want present (never removed)", err)
	}
	if !containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
		t.Errorf("registration removed for a dirty workspace; must be preserved")
	}
	assertUnchanged(t, beforeWs, ws)
	if m, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !present || m.Phase != PhaseReady {
		t.Errorf("manifest present=%v phase=%v err=%v; want present ready (not advanced)", present, m.Phase, err)
	}
}

// TestCleanupBlockedMatrix walks every unproven/dirty condition. Each leaves the
// colliding artifact byte-untouched, keeps the local branch, and never advances
// the manifest to a tombstone.
func TestCleanupBlockedMatrix(t *testing.T) {
	t.Run("staged-file", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)
		ws := wsPathOf(repo)
		writeWorktreeFile(t, ws, "staged.txt", "staged bytes\n")
		gitOut(t, ws, "add", "staged.txt")
		before := snapshotTree(t, ws)

		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked", res.Disposition)
		}
		assertUnchanged(t, before, ws)
		assertReadyManifestKept(t, r, repo, tgt)
	})

	t.Run("untracked-file", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)
		ws := wsPathOf(repo)
		writeWorktreeFile(t, ws, "untracked.txt", "untracked bytes\n")
		before := snapshotTree(t, ws)

		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked", res.Disposition)
		}
		assertUnchanged(t, before, ws)
		assertReadyManifestKept(t, r, repo, tgt)
	})

	t.Run("unresolved-conflict", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		base := prepareOK(t, svc, repo, tgt).BaseCommit
		ws := wsPathOf(repo)

		// Build a real merge conflict inside the workspace: a divergent branch and
		// the feature branch each touch main.go differently, then merge fails.
		gitOut(t, ws, "checkout", "-q", "-b", "conflictor", string(base))
		writeWorktreeFile(t, ws, "main.go", "package main // conflictor side\n")
		gitOut(t, ws, "add", "main.go")
		gitOut(t, ws, "commit", "-q", "-m", "conflictor change")
		gitOut(t, ws, "checkout", "-q", string(prepFeatureRef()))
		writeWorktreeFile(t, ws, "main.go", "package main // feature side\n")
		gitOut(t, ws, "add", "main.go")
		gitOut(t, ws, "commit", "-q", "-m", "feature change")
		if _, err := gitTry(ws, "merge", "conflictor"); err == nil {
			t.Fatalf("fixture: merge did not conflict")
		}
		before := snapshotTree(t, ws)

		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked for an unresolved conflict", res.Disposition)
		}
		assertUnchanged(t, before, ws)
		if !containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
			t.Errorf("registration removed for a conflicted workspace; must be preserved")
		}
	})

	t.Run("detached-head", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)
		ws := wsPathOf(repo)
		// Detach HEAD out-of-band; the tree stays clean but is no longer on the ref.
		gitOut(t, ws, "checkout", "-q", "--detach", "HEAD")
		before := snapshotTree(t, ws)

		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked for a detached HEAD", res.Disposition)
		}
		assertUnchanged(t, before, ws)
		if !branchExists(r.Primary, "feat/"+prepSlug) {
			t.Errorf("feat branch deleted; must be preserved")
		}
	})

	t.Run("moved-head-base-unreachable", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)

		// Rewrite the recorded base to a commit the feature head does not reach (a
		// later origin-main commit, fetched into the object store): the workspace's
		// head no longer reaches the recorded base — a moved-HEAD mismatch.
		c1 := r.advanceMain(t)
		gitOut(t, r.Primary, "fetch", "-q", "origin", "main")
		m, present, err := loadManifest(metaDirOf(repo, tgt))
		if err != nil || !present {
			t.Fatalf("loadManifest present=%v err=%v", present, err)
		}
		m.BaseCommit = c1
		if err := writeManifest(metaDirOf(repo, tgt), m); err != nil {
			t.Fatalf("writeManifest(rewritten base): %v", err)
		}
		ws := wsPathOf(repo)
		before := snapshotTree(t, ws)

		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked for an unreachable recorded base", res.Disposition)
		}
		assertUnchanged(t, before, ws)
		if !containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
			t.Errorf("registration removed; must be preserved")
		}
	})

	t.Run("registration-path-mismatch", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		prepareOK(t, svc, repo, tgt)
		ws := wsPathOf(repo)

		// Deregister the checkout and re-attach the same branch elsewhere: the ready
		// manifest's recorded path is no longer registered.
		gitOut(t, r.Primary, "worktree", "remove", "--force", "--", ws)
		elsewhere := filepath.Join(repo.PrimaryWorktree, ".worktrees", "moved")
		gitOut(t, r.Primary, "worktree", "add", "--", elsewhere, "feat/"+prepSlug)
		beforeManifest := readFileBytes(t, filepath.Join(metaDirOf(repo, tgt), manifestFileName))

		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked for a registration/path mismatch", res.Disposition)
		}
		if !containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), elsewhere) {
			t.Errorf("relocated registration removed; cleanup must only touch the recorded path")
		}
		if after := readFileBytes(t, filepath.Join(metaDirOf(repo, tgt), manifestFileName)); after != beforeManifest {
			t.Errorf("manifest bytes changed on a blocked mismatch")
		}
	})

	t.Run("missing-manifest", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)

		res := cleanupOK(t, svc, repo, tgt)
		if res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked for a missing manifest", res.Disposition)
		}
		if _, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || present {
			t.Errorf("manifest present=%v err=%v; want none (cleanup wrote nothing)", present, err)
		}
	})

	t.Run("foreign-commondir-manifest", func(t *testing.T) {
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		other := mainModeRepo(t)
		_, otherRepo := other.newService(t)
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

		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked for a foreign-CommonDir manifest", res.Disposition)
		}
		if after := readFileBytes(t, mpath); after != before {
			t.Errorf("foreign manifest bytes changed")
		}
	})

	t.Run("allocating-partial", func(t *testing.T) {
		// An allocating manifest is an unproven partial: the monotonic phase chain
		// refuses allocating->cleaned, so cleanup blocks rather than tombstoning it.
		r := mainModeRepo(t)
		svc, repo := r.newService(t)
		tgt := freshTarget(t, 7)
		base := gitcli.ObjectID(gitOut(t, r.Primary, "rev-parse", "main"))
		writeStateManifest(t, repo, tgt, base, PhaseAllocating)
		mpath := filepath.Join(metaDirOf(repo, tgt), manifestFileName)
		before := readFileBytes(t, mpath)

		if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupBlocked {
			t.Errorf("Disposition = %q; want blocked for an allocating partial", res.Disposition)
		}
		if after := readFileBytes(t, mpath); after != before {
			t.Errorf("allocating manifest bytes changed")
		}
	})
}

// TestCleanupProbeFailureIsFailedNotClean injects a git that fails `worktree`
// during cleanup, so ListWorktrees errors. An errored registration probe is a
// `failed` error, NEVER a false already-clean, and the workspace is fully intact
// (learnings: probe-error-is-not-clean-absence — the 0309 review's defect class).
func TestCleanupProbeFailureIsFailedNotClean(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)
	ws := wsPathOf(repo)
	before := snapshotTree(t, ws)

	// Build a second service whose git fails `worktree list`. The repo is reused
	// from the real discovery above (discovery itself uses `git worktree list`, so
	// it must not run through the failing binary).
	failGit := writeFailingGit(t, "worktree")
	fc, err := gitcli.NewClient(gitcli.WithExecutable(failGit))
	if err != nil {
		t.Fatalf("NewClient(failing): %v", err)
	}
	failSvc, err := NewService(fc)
	if err != nil {
		t.Fatalf("NewService(failing): %v", err)
	}

	res, err := failSvc.Cleanup(context.Background(), CleanupRequest{Repository: repo, Target: tgt})
	if err == nil {
		t.Fatalf("Cleanup with failing worktree probe = nil error; want failed")
	}
	if res.Disposition == CleanupAlreadyClean || res.Disposition == CleanupCleaned {
		t.Errorf("Disposition = %q; a probe error must never read as clean/removed", res.Disposition)
	}
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error %v is not a *Failure", err)
	}
	if f.Kind != KindExternal {
		t.Errorf("Kind = %q; want external", f.Kind)
	}
	// Fully intact: directory, registration, and manifest all survive unchanged.
	assertUnchanged(t, before, ws)
	if !containsWorktreePath(t, gitOut(t, r.Primary, "worktree", "list", "--porcelain"), ws) {
		t.Errorf("registration removed after a probe failure; must be intact")
	}
	if m, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !present || m.Phase != PhaseReady {
		t.Errorf("manifest present=%v phase=%v err=%v; want present ready", present, m.Phase, err)
	}
}

// TestCleanupNeverPrunes proves cleanup never runs a global `git worktree prune`:
// a second, stale-looking-but-registered worktree (its directory deleted) still
// appears in the worktree list after a cleanup of the target.
func TestCleanupNeverPrunes(t *testing.T) {
	r := mainModeRepo(t)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	prepareOK(t, svc, repo, tgt)

	// A second valid registration whose directory is then removed on disk, so a
	// `git worktree prune` would deregister it. Cleanup must not.
	prunable := filepath.Join(repo.PrimaryWorktree, ".worktrees", "prunable")
	gitOut(t, r.Primary, "worktree", "add", "-b", "feat/prunable", prunable, "main")
	if err := os.RemoveAll(prunable); err != nil {
		t.Fatal(err)
	}

	if res := cleanupOK(t, svc, repo, tgt); res.Disposition != CleanupCleaned {
		t.Fatalf("Cleanup = %q; want cleaned", res.Disposition)
	}
	// The prunable registration still appears: no global prune ran. The directory
	// is deleted, so this checks the raw registration line (a canonicalizing check
	// cannot resolve a path whose directory no longer exists).
	if !registeredLine(gitOut(t, r.Primary, "worktree", "list", "--porcelain"), prunable) {
		t.Errorf("prunable registration disappeared; cleanup must never `git worktree prune`")
	}
}

// registeredLine reports whether the porcelain worktree list contains a
// `worktree <path>` stanza header for exactly path. Unlike containsWorktreePath
// it does not canonicalize, so it can assert on a registration whose on-disk
// directory has been removed (a prunable entry).
func registeredLine(porcelain, path string) bool {
	for _, line := range splitLines(porcelain) {
		if p, ok := cutPrefix(line, "worktree "); ok && p == path {
			return true
		}
	}
	return false
}

// assertReadyManifestKept asserts the target's manifest is still present and
// ready (never advanced to a tombstone) and its branch survives.
func assertReadyManifestKept(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target) {
	t.Helper()
	if m, present, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !present || m.Phase != PhaseReady {
		t.Errorf("manifest present=%v phase=%v err=%v; want present ready", present, m.Phase, err)
	}
	if !branchExists(r.Primary, "feat/"+prepSlug) {
		t.Errorf("feat branch deleted; must be preserved")
	}
}
