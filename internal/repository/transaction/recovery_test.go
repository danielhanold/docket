package transaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file is the ownership-and-recovery matrix for PruneAbandoned. Every
// scenario runs against a real Git topology so the six-point proof is exercised
// against actual worktree registrations, real HEADs, and real flocks — never a
// fake. The single invariant under test: PruneAbandoned removes ONLY candidates
// that pass ALL SIX ownership checks and leaves everything else byte-untouched
// with a verdict, never resetting a branch, deleting a ref, or globally pruning.

// recoveryEngine discovers the invocation repository and builds an Engine over the
// real git client and the pinned engine clock.
func recoveryEngine(t *testing.T, r *testRepos) (*Engine, *gitcli.Client, gitcli.Repository) {
	t.Helper()
	client, repo := r.discover(t)
	eng, err := NewEngine(client, engineClock)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return eng, client, repo
}

// baseValidManifest returns a fully valid manifest for id whose repository identity
// is commonDir and whose target is refs/heads/main. Individual tests corrupt one
// field to exercise a single failing check while every other field stays canonical.
func baseValidManifest(id, commonDir string) manifest {
	stamp := txnTestClock.Now().UTC().Format(time.RFC3339)
	return manifest{
		Schema:        manifestSchemaVersion,
		TransactionID: id,
		CommonDir:     commonDir,
		Remote:        "origin",
		TargetRef:     "refs/heads/main",
		BaseCommit:    fixedBase,
		WorktreeRel:   worktreeDirName,
		Phase:         phaseAllocating,
		CreatedUTC:    stamp,
		UpdatedUTC:    stamp,
		PID:           os.Getpid(),
	}
}

// mkOwnedDir creates an owned-shape candidate directory (0700) under repo's
// transactions root and returns its path. It ensures the root exists first.
func mkOwnedDir(t *testing.T, repo gitcli.Repository, id string) string {
	t.Helper()
	root := transactionsRoot(repo)
	if err := ensureTransactionsRoot(root); err != nil {
		t.Fatalf("ensure transactions root: %v", err)
	}
	candRoot := filepath.Join(root, id)
	if err := mkdirMode(candRoot, txnDirMode); err != nil {
		t.Fatalf("mkdir candidate: %v", err)
	}
	return candRoot
}

// hexID returns a valid 32-hex transaction id built from a distinguishing prefix
// so table rows sort deterministically and read clearly in failures.
func hexID(prefix string) string {
	id := prefix
	for len(id) < 32 {
		id += "0"
	}
	return id[:32]
}

// hashTree returns a content hash of the entire tree rooted at path: every entry's
// relative name, type, and permission bits, plus regular-file contents and symlink
// targets. filepath.WalkDir never follows symlinks, so a symlinked root hashes as
// the link itself. A byte-untouched survival must produce an identical hash.
func hashTree(t *testing.T, path string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, rerr := filepath.Rel(path, p)
		if rerr != nil {
			return rerr
		}
		info, ierr := os.Lstat(p)
		if ierr != nil {
			return ierr
		}
		fmt.Fprintf(h, "path=%s type=%v perm=%o\n", rel, info.Mode()&os.ModeType, info.Mode().Perm())
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			fmt.Fprintf(h, "link=%s\n", target)
		case info.Mode().IsRegular():
			data, derr := os.ReadFile(p)
			if derr != nil {
				return derr
			}
			h.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("hash tree %s: %v", path, err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// abandonRegistered allocates a candidate, registers its detached worktree at
// commit, then releases the live lock — the exact shape of a normally abandoned
// candidate a crashed run left behind: valid manifest, released lock, registered
// worktree. It returns the candidate.
func abandonRegistered(t *testing.T, client *gitcli.Client, repo gitcli.Repository, r *testRepos, commit gitcli.ObjectID) *candidate {
	t.Helper()
	c, err := allocateCandidate(txnTestClock, repo, "origin", r.Target, fixedBase)
	if err != nil {
		t.Fatalf("allocateCandidate: %v", err)
	}
	if err := client.AddDetachedWorktree(context.Background(), repo, c.worktree, commit); err != nil {
		_ = c.live.release()
		t.Fatalf("AddDetachedWorktree: %v", err)
	}
	if err := c.live.release(); err != nil {
		t.Fatalf("release live lock: %v", err)
	}
	return c
}

// targetTip resolves the invocation's current target-ref commit — an ancestor of
// which is "already pushed" from recovery's point of view.
func targetTip(t *testing.T, r *testRepos) gitcli.ObjectID {
	t.Helper()
	return gitcli.ObjectID(hgitOut(t, r.Invocation, "rev-parse", string(r.Target)))
}

// pruneEntryFor returns the report entry for id, failing if absent.
func pruneEntryFor(t *testing.T, rep PruneReport, id string) PruneEntry {
	t.Helper()
	for _, e := range rep.Entries {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("no report entry for %q in %+v", id, rep.Entries)
	return PruneEntry{}
}

// assertGoneAndDeregistered proves the candidate directory is removed and its
// worktree is no longer registered with Git.
func assertGoneAndDeregistered(t *testing.T, client *gitcli.Client, repo gitcli.Repository, c *candidate) {
	t.Helper()
	if _, err := os.Stat(c.root); !os.IsNotExist(err) {
		t.Errorf("candidate root still present: %v", err)
	}
	infos, err := client.ListWorktrees(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	want := canonicalPath(c.worktree)
	for _, info := range infos {
		if canonicalPath(info.Path) == want {
			t.Errorf("worktree still registered after prune: %s", info.Path)
		}
	}
}

// headCommit resolves the invocation repo's current HEAD commit for a worktree add.
func headCommit(t *testing.T, repo gitcli.Repository) gitcli.ObjectID {
	t.Helper()
	return gitcli.ObjectID(hgitOut(t, repo.PrimaryWorktree, "rev-parse", "HEAD"))
}
