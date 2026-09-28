package transaction

import (
	"github.com/danielhanold/docket/internal/gitcli"
)

// This file covers PruneReport itself: an empty sweep, deterministic ordering, and
// that recovery works identically under the docket-mode topology (a linked .docket
// worktree present) as under main mode. The full ownership-and-recovery matrix
// lives in recovery_test.go.

// hasCleanupPending reports whether warnings names id's cleanup-pending marker.
func hasCleanupPending(warnings []string, id string) bool {
	want := "cleanup-pending: " + id
	for _, w := range warnings {
		if w == want {
			return true
		}
	}
	return false
}

// hasWorktreeNamed reports whether any registration's path basename equals name.
func hasWorktreeNamed(infos []gitcli.WorktreeInfo, name string) bool {
	for _, info := range infos {
		if base := baseName(info.Path); base == name {
			return true
		}
	}
	return false
}

// baseName returns the final path element without importing path/filepath twice in
// assertions; it keeps this file's helper self-contained.
func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}
