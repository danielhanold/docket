//go:build integration

package gitcli

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"os"
	"path/filepath"
	"testing"
)

// worktreeOracle returns the canonical (Abs + every-symlink-hop) toplevel and
// absolute git dir git itself reports for path, computed by invoking real git
// directly — an oracle independent of the adapter under test.
func worktreeOracle(t *testing.T, path string) (root, gitDir string) {
	t.Helper()
	return canon(t, gitOut(t, path, "rev-parse", "--show-toplevel")),
		canon(t, gitOut(t, path, "rev-parse", "--absolute-git-dir"))
}

// TestIntegrationRepoDiscoverWorktreeContainingIdentity proves DiscoverWorktree resolves the
// working tree CONTAINING the invocation path — identical from the primary root
// and any nested directory beneath it, and (unlike Discover) resolving each
// linked worktree to ITSELF rather than the primary. Every answer is checked
// against git's own canonicalized report.
func TestIntegrationRepoDiscoverWorktreeContainingIdentity(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()

	check := func(t *testing.T, path string) WorktreeIdentity {
		t.Helper()
		wantRoot, wantGitDir := worktreeOracle(t, path)
		id, err := c.DiscoverWorktree(ctx, DiscoverOptions{InvocationPath: path})
		if err != nil {
			t.Fatalf("DiscoverWorktree(%q) error: %v", path, err)
		}
		if id.Root != wantRoot {
			t.Errorf("DiscoverWorktree(%q) Root = %q, want %q", path, id.Root, wantRoot)
		}
		if id.GitDir != wantGitDir {
			t.Errorf("DiscoverWorktree(%q) GitDir = %q, want %q", path, id.GitDir, wantGitDir)
		}
		return id
	}

	t.Run("main mode: root and nested dir share one identity", func(t *testing.T) {
		r := newMainModeRepos(t)
		fromRoot := check(t, r.Invocation)
		fromNested := check(t, filepath.Join(r.Invocation, "docs/changes"))
		if fromRoot != fromNested {
			t.Errorf("nested-dir identity %+v != root identity %+v", fromNested, fromRoot)
		}
	})

	t.Run("docket mode: linked worktree resolves to itself", func(t *testing.T) {
		r := newDocketModeRepos(t)
		primary := check(t, r.Invocation)

		docketWT := filepath.Join(r.Invocation, ".docket")
		featWT := filepath.Join(r.Invocation, ".worktrees/feat-x")

		docketID := check(t, docketWT)
		featID := check(t, featWT)

		// Per-worktree ownership isolation: a linked worktree is its OWN root
		// with its OWN git dir, never the primary's.
		if docketID.Root == primary.Root {
			t.Errorf(".docket Root = %q resolved to the primary root, want the linked worktree", docketID.Root)
		}
		if docketID.GitDir == primary.GitDir {
			t.Errorf(".docket GitDir = %q resolved to the primary git dir, want the per-worktree git dir", docketID.GitDir)
		}
		// The linked worktree's git dir lives under the primary's .git/worktrees.
		wantParent := canon(t, filepath.Join(r.Invocation, ".git", "worktrees"))
		if got := filepath.Dir(docketID.GitDir); got != wantParent {
			t.Errorf(".docket GitDir parent = %q, want %q", got, wantParent)
		}
		if featID.Root == primary.Root || featID.Root == docketID.Root {
			t.Errorf("feature worktree Root = %q not distinct from primary/.docket", featID.Root)
		}
	})
}

// TestIntegrationRepoDiscoverWorktreeThroughSymlink proves a path reached through an extra
// symlink resolves to the same canonical identity as the real path. The fixture
// lives under testsupport.TempDir(t), so on macOS this already exercises /tmp -> /private/tmp.
func TestIntegrationRepoDiscoverWorktreeThroughSymlink(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)

	direct, err := c.DiscoverWorktree(ctx, DiscoverOptions{InvocationPath: r.Invocation})
	if err != nil {
		t.Fatalf("DiscoverWorktree(direct) error: %v", err)
	}

	link := filepath.Join(testsupport.TempDir(t), "linktoroot")
	if err := os.Symlink(r.Invocation, link); err != nil {
		t.Fatal(err)
	}
	viaLink, err := c.DiscoverWorktree(ctx, DiscoverOptions{InvocationPath: link})
	if err != nil {
		t.Fatalf("DiscoverWorktree(via symlink) error: %v", err)
	}
	if viaLink != direct {
		t.Errorf("via-symlink identity %+v != direct identity %+v", viaLink, direct)
	}
	// The resolved root carries no symlink hop.
	if resolved := canon(t, viaLink.Root); resolved != viaLink.Root {
		t.Errorf("Root %q is not symlink-canonical (resolves to %q)", viaLink.Root, resolved)
	}
}

// TestIntegrationRepoDiscoverWorktreeRejections asserts the typed failure kinds: an empty path
// (invalid-request), a plain non-repo dir, and a bare repository (both
// invalid-repository).
func TestIntegrationRepoDiscoverWorktreeRejections(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)

	_, err := c.DiscoverWorktree(ctx, DiscoverOptions{InvocationPath: ""})
	assertKind(t, err, KindInvalidRequest)

	_, err = c.DiscoverWorktree(ctx, DiscoverOptions{InvocationPath: testsupport.TempDir(t)})
	assertKind(t, err, KindInvalidRepository)

	_, err = c.DiscoverWorktree(ctx, DiscoverOptions{InvocationPath: r.Origin})
	assertKind(t, err, KindInvalidRepository)
}
