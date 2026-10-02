//go:build integration

package gatedrive

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationGatedriveWorktreeLockKeyIsCanonical (L3) proves every spelling
// of one worktree — a subdirectory, a symlink to the root, /tmp vs /private/tmp
// on darwin, a case variant on a case-insensitive volume — resolves through
// realGit.WorktreeRoot to the same lock, while a separate repository gets its
// own. A wrong key would let one worktree buy two locks.
func TestIntegrationGatedriveWorktreeLockKeyIsCanonical(t *testing.T) {
	root := func(t *testing.T, dir string) string {
		t.Helper()
		r, err := realGit{}.WorktreeRoot(dir)
		if err != nil {
			t.Fatalf("WorktreeRoot(%q): %v", dir, err)
		}
		return r
	}
	assertSameLock := func(t *testing.T, a, b string) {
		t.Helper()
		store := OpenStore(testsupport.TempDir(t))
		held, err := store.TryWorktreeLock(root(t, a), nil)
		if err != nil {
			t.Fatalf("acquire via %q: %v", a, err)
		}
		defer held.Release()
		if _, err := store.TryWorktreeLock(root(t, b), nil); !isOwnershipKind(err, ErrWorktreeBusy) {
			t.Fatalf("acquire via %q while %q holds = %v, want worktree-busy", b, a, err)
		}
	}
	newRepo := func(t *testing.T, parent string) string {
		t.Helper()
		var dir string
		if parent == "" {
			dir = testsupport.TempDir(t)
		} else {
			// tempdir-exempt: /tmp is the mandated parent for the /tmp vs /private/tmp alias case.
			d, err := os.MkdirTemp(parent, "docketfix-wtlock-*")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(d) })
			dir = d
		}
		gitInit(t, dir)
		return dir
	}

	t.Run("subdirectory", func(t *testing.T) {
		repo := newRepo(t, "")
		sub := filepath.Join(repo, "a", "b")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		assertSameLock(t, repo, sub)
		assertSameLock(t, sub, repo)
	})

	t.Run("symlink to root", func(t *testing.T) {
		repo := newRepo(t, "")
		link := filepath.Join(testsupport.TempDir(t), "link")
		if err := os.Symlink(repo, link); err != nil {
			t.Fatal(err)
		}
		assertSameLock(t, repo, link)
		assertSameLock(t, link, repo)
	})

	t.Run("tmp vs private tmp", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("darwin-only spelling pair")
		}
		if r, err := filepath.EvalSymlinks("/tmp"); err != nil || r == "/tmp" {
			t.Skip("/tmp is not a symlink here")
		}
		repo := newRepo(t, "/tmp")
		if !strings.HasPrefix(repo, "/tmp/") {
			t.Fatalf("repo %q not under /tmp", repo)
		}
		private := "/private" + repo
		assertSameLock(t, repo, private)
		assertSameLock(t, private, repo)
	})

	t.Run("case variant", func(t *testing.T) {
		repo := newRepo(t, "")
		variant := filepath.Join(filepath.Dir(repo), strings.ToUpper(filepath.Base(repo)))
		if variant == repo {
			t.Fatalf("case variant of %q is identical", repo)
		}
		if _, err := os.Stat(variant); err != nil {
			t.Skipf("case-sensitive volume: %v", err)
		}
		assertSameLock(t, repo, variant)
		assertSameLock(t, variant, repo)
	})

	t.Run("separate repository gets its own lock", func(t *testing.T) {
		a, b := newRepo(t, ""), newRepo(t, "")
		store := OpenStore(testsupport.TempDir(t))
		held, err := store.TryWorktreeLock(root(t, a), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer held.Release()
		other, err := store.TryWorktreeLock(root(t, b), nil)
		if err != nil {
			t.Fatalf("a separate repository must get its own lock: %v", err)
		}
		other.Release()
	})
}
