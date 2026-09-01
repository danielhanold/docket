//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIntegrationRepoFastForwardWorktree(t *testing.T) {
	ctx := context.Background()

	t.Run("clean advance and already current", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		target := r.writerCommit(t, "main", map[string]string{"remote-only.txt": "target\n"})
		gitOut(t, r.Invocation, "fetch", "-q", "origin", "main")

		advanced, err := c.FastForwardWorktree(ctx, r.Invocation, target)
		if err != nil || !advanced {
			t.Fatalf("FastForwardWorktree advance = %v, %v; want true, nil", advanced, err)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != target {
			t.Fatalf("HEAD = %s, want %s", got, target)
		}

		advanced, err = c.FastForwardWorktree(ctx, r.Invocation, target)
		if err != nil || advanced {
			t.Fatalf("FastForwardWorktree already-current = %v, %v; want false, nil", advanced, err)
		}
	})

	t.Run("divergent history is typed and untouched", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		target := r.writerCommit(t, "main", map[string]string{"remote-only.txt": "target\n"})
		gitOut(t, r.Invocation, "fetch", "-q", "origin", "main")
		writeWorktreeFile(t, r.Invocation, "local-only.txt", "local\n")
		gitOut(t, r.Invocation, "add", "--", "local-only.txt")
		gitOut(t, r.Invocation, "commit", "-q", "-m", "diverge locally")
		before := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))

		advanced, err := c.FastForwardWorktree(ctx, r.Invocation, target)
		f, ok := AsFailure(err)
		if advanced || !ok || f.Operation != fastForwardWorktreeOp || f.Kind != KindCommandFailed {
			t.Fatalf("divergent result = %v, %#v; want typed command-failed", advanced, err)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != before {
			t.Fatalf("refused fast-forward moved HEAD from %s to %s", before, got)
		}
	})

	t.Run("any dirty path is refused and preserved", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		target := r.writerCommit(t, "main", map[string]string{"remote-only.txt": "target\n"})
		gitOut(t, r.Invocation, "fetch", "-q", "origin", "main")
		dirtyPath := filepath.Join(r.Invocation, "README.md")
		beforeHead := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		writeWorktreeFile(t, r.Invocation, "README.md", "uncommitted bytes\n")

		advanced, err := c.FastForwardWorktree(ctx, r.Invocation, target)
		f, ok := AsFailure(err)
		if advanced || !ok || f.Operation != fastForwardWorktreeOp || f.Kind != KindInvalidRepository {
			t.Fatalf("dirty result = %v, %#v; want typed invalid-repository", advanced, err)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != beforeHead {
			t.Fatalf("dirty refusal moved HEAD from %s to %s", beforeHead, got)
		}
		got, readErr := os.ReadFile(dirtyPath)
		if readErr != nil || string(got) != "uncommitted bytes\n" {
			t.Fatalf("dirty bytes after refusal = %q, %v", got, readErr)
		}
	})

	t.Run("malformed target is invalid request", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		before := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		advanced, err := c.FastForwardWorktree(ctx, r.Invocation, ObjectID("not-an-oid"))
		f, ok := AsFailure(err)
		if advanced || !ok || f.Operation != fastForwardWorktreeOp || f.Kind != KindInvalidRequest {
			t.Fatalf("malformed result = %v, %#v; want typed invalid-request", advanced, err)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != before {
			t.Fatalf("malformed target moved HEAD from %s to %s", before, got)
		}
	})
}
