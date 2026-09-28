package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// The PublishHead tests drive the idempotent feature-branch publication flow
// against real bare origins. PublishHead reinspects the owned ready workspace,
// refuses a dirty or inconsistent one, probes the authoritative remote feature
// ref, and reaches the exact local HEAD onto the exact remote ref under an
// absent-ref or expected-old lease — never a force, reset, merge, or rebase. The
// idempotency key is the remote state (the exact commit at the exact remote
// ref), never a clean tree, a local branch, or an upstream configuration.

// commitInWorkspace commits one file on the workspace's feature branch and
// returns the new HEAD, so a test can advance the branch past its base.
func commitInWorkspace(t *testing.T, ws, rel, content string) gitcli.ObjectID {
	t.Helper()
	writeWorktreeFile(t, ws, rel, content)
	gitOut(t, ws, "add", rel)
	gitOut(t, ws, "commit", "-q", "-m", "work: "+rel)
	return gitcli.ObjectID(gitOut(t, ws, "rev-parse", "HEAD"))
}

// originFeatCommit returns the origin's feat/<slug> ref commit and whether it
// exists. Origin is bare, so it is read directly with rev-parse.
func originFeatCommit(t *testing.T, r *wsRepos) (gitcli.ObjectID, bool) {
	t.Helper()
	out, err := gitTry(r.Origin, "rev-parse", "--verify", "--quiet", string(prepFeatureRef()))
	if err != nil {
		return "", false
	}
	return gitcli.ObjectID(strings.TrimSpace(out)), true
}

// chmodTree recursively sets mode on dir and everything beneath it. Directories
// are chmod'd after their contents so a read-only parent does not block
// descent on the way down; on restore the parent must be writable first, so it
// is applied to dir itself last regardless.
func chmodTree(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	// Ensure directories are traversable/writable enough to walk when restoring.
	restore := mode&0o200 != 0
	if restore {
		_ = os.Chmod(dir, 0o700)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// On the read-only pass the dir may already be unreadable; that is fine.
		if restore {
			t.Fatalf("ReadDir %s: %v", dir, err)
		}
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			chmodTree(t, p, mode)
			continue
		}
		if err := os.Chmod(p, mode); err != nil && restore {
			t.Fatalf("chmod %s: %v", p, err)
		}
	}
	if err := os.Chmod(dir, mode); err != nil && restore {
		t.Fatalf("chmod %s: %v", dir, err)
	}
}

// publishHead runs PublishHead and returns the result plus error verbatim.
func publishHead(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target) (PublishResult, error) {
	t.Helper()
	return svc.PublishHead(context.Background(), PublishRequest{Repository: repo, Remote: "origin", Target: tgt})
}
