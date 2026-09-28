package workspace

import (
	"context"
	"testing"

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
