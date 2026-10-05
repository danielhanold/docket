//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationRepoIsAncestorIgnoringReplacements proves the relationship probe
// ignores both history-rewrite mechanisms: a replace ref and the legacy graft file
// each make an unrelated orphan commit look like a descendant to plain IsAncestor,
// and IsAncestorIgnoringReplacements must still answer false.
func TestIntegrationRepoIsAncestorIgnoringReplacements(t *testing.T) {
	ctx := context.Background()
	newOrphan := func(t *testing.T, r *testRepos) (base, orphan ObjectID) {
		t.Helper()
		base = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		tree := gitOut(t, r.Invocation, "rev-parse", "HEAD^{tree}")
		orphan = ObjectID(strings.TrimSpace(gitOut(t, r.Invocation,
			"-c", "user.name=t", "-c", "user.email=t@example.com",
			"commit-tree", tree, "-m", "orphan")))
		return base, orphan
	}

	t.Run("replace ref", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		base, orphan := newOrphan(t, r)
		gitOut(t, r.Invocation, "replace", "--graft", string(orphan), string(base))
		repo := Repository{PrimaryWorktree: r.Invocation}

		plain, err := c.IsAncestor(ctx, repo, base, orphan)
		if err != nil || !plain {
			t.Fatalf("premise: plain IsAncestor under a replace graft = %v, %v; want true, nil", plain, err)
		}
		got, err := c.IsAncestorIgnoringReplacements(ctx, repo, base, orphan)
		if err != nil || got {
			t.Fatalf("IsAncestorIgnoringReplacements = %v, %v; want false, nil (replace refs ignored)", got, err)
		}
	})

	t.Run("graft file", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		base, orphan := newOrphan(t, r)
		grafts := filepath.Join(r.Invocation, ".git", "info", "grafts")
		if err := os.MkdirAll(filepath.Dir(grafts), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(grafts, []byte(string(orphan)+" "+string(base)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		repo := Repository{PrimaryWorktree: r.Invocation}

		plain, err := c.IsAncestor(ctx, repo, base, orphan)
		if err != nil || !plain {
			t.Fatalf("premise: plain IsAncestor under a graft file = %v, %v; want true, nil", plain, err)
		}
		got, err := c.IsAncestorIgnoringReplacements(ctx, repo, base, orphan)
		if err != nil || got {
			t.Fatalf("IsAncestorIgnoringReplacements = %v, %v; want false, nil (graft file ignored)", got, err)
		}
	})

	t.Run("real ancestry still true", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		base := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		writeWorktreeFile(t, r.Invocation, "next.txt", "next\n")
		gitOut(t, r.Invocation, "add", "--", "next.txt")
		gitOut(t, r.Invocation, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "next")
		next := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		got, err := c.IsAncestorIgnoringReplacements(ctx, Repository{PrimaryWorktree: r.Invocation}, base, next)
		if err != nil || !got {
			t.Fatalf("real ancestry = %v, %v; want true, nil", got, err)
		}
	})
}
