//go:build integration

package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// --- metadata checkout follow (TestIntegrationMetadataFollow shard) ----------------
//
// followMetadataCheckout moves the clone's metadata checkout in place to a commit a
// docket write just pushed, when it is clean, attached on the metadata branch, and
// strictly behind; everything else is left exactly as it was. Each direct scenario
// starts from a check-healthy repository whose remote docket branch is one commit
// ahead of .docket, with that commit's objects fetched into the clone (a real write
// creates its commit locally, so its objects are always present).

// discoverFollowRepo discovers the repository at dir with a fresh client.
func discoverFollowRepo(t *testing.T, dir string) gitcli.Repository {
	t.Helper()
	repo, err := newGitClient(t).Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: dir})
	if err != nil {
		t.Fatalf("Discover %s: %v", dir, err)
	}
	return repo
}

// newFollowRepo is newBehindHealthyRepo with the advanced commit fetched into the
// invocation clone. The fetch moves only refs/remotes/origin/docket, never the
// local docket branch or .docket.
func newFollowRepo(t *testing.T) (r *initRepo, oldTip, newTip string) {
	t.Helper()
	r, oldTip, newTip = newBehindHealthyRepo(t)
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf("fixture: .docket HEAD = %s after fetch, want %s", head, oldTip)
	}
	return r, oldTip, newTip
}

// followAt runs followMetadataCheckout for the repository discovered at dir.
func followAt(t *testing.T, dir, ref, pushed string) {
	t.Helper()
	followMetadataCheckout(context.Background(), newGitClient(t), discoverFollowRepo(t, dir),
		gitcli.RefName(ref), gitcli.ObjectID(pushed))
}

func localDocketTip(t *testing.T, r *initRepo) string {
	t.Helper()
	return runGit(t, r.invocation, "rev-parse", "refs/heads/docket")
}

func TestIntegrationMetadataFollowFastForwardsCleanBehindCheckout(t *testing.T) {
	r, _, newTip := newFollowRepo(t)
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket HEAD = %s, want the pushed commit %s", head, newTip)
	}
	if tip := localDocketTip(t, r); tip != newTip {
		t.Fatalf("local docket branch = %s, want %s", tip, newTip)
	}
	b, err := os.ReadFile(filepath.Join(r.invocation, ".docket", "notes", "advance.txt"))
	if err != nil || string(b) != "advanced\n" {
		t.Fatalf("pushed file on disk = %q, %v; want the pushed content", b, err)
	}
	requireCheckHealthy(t, r)
}

// TestIntegrationMetadataFollowFromLinkedWorktree runs the follow from a linked
// feature worktree, as a write from .worktrees/<slug> does: the metadata checkout is
// per clone, resolved from the shared common dir.
func TestIntegrationMetadataFollowFromLinkedWorktree(t *testing.T) {
	r, _, newTip := newFollowRepo(t)
	feat := filepath.Join(testsupport.TempDir(t), "feat")
	runGit(t, r.invocation, "worktree", "add", "-q", "-b", "feat", feat, "main")
	followAt(t, feat, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket HEAD = %s, want %s (follow from a linked worktree)", head, newTip)
	}
}

func TestIntegrationMetadataFollowIgnoresOtherTargetRef(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	followAt(t, r.invocation, "refs/heads/main", newTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf(".docket HEAD = %s, want it untouched at %s for a non-metadata target", head, oldTip)
	}
}

// TestIntegrationMetadataFollowLeavesUncommittedEditAlone edits a tracked file the
// pushed commit does NOT touch. read-tree -u -m alone would carry that edit across
// and move the branch; the clean check must leave everything as it was. The init
// seed commits the empty tree, so the fixture first brings .docket to a commit
// that tracks notes/base.txt, then pushes the commit to follow on top of it.
func TestIntegrationMetadataFollowLeavesUncommittedEditAlone(t *testing.T) {
	r, baseTip, _ := newFollowRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	followAt(t, r.invocation, "refs/heads/docket", baseTip)
	oldTip := r.advanceRemoteDocket(t, "notes/base.txt", "base\n", "track a base file")
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	followAt(t, r.invocation, "refs/heads/docket", oldTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf("fixture: .docket HEAD = %s, want %s", head, oldTip)
	}
	newTip := r.advanceRemoteDocket(t, "notes/pushed.txt", "pushed\n", "the write to follow")
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	if tracked := strings.Fields(runGit(t, dot, "ls-files", "--", "notes/base.txt")); len(tracked) != 1 {
		t.Fatalf("fixture: .docket tracks %v, want notes/base.txt", tracked)
	}
	edited := filepath.Join(dot, "notes", "base.txt")
	orig, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	want := string(orig) + "\nlocal edit\n"
	if err := os.WriteFile(edited, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf(".docket HEAD = %s, want it untouched at %s", head, oldTip)
	}
	if tip := localDocketTip(t, r); tip != oldTip {
		t.Fatalf("local docket branch = %s, want %s", tip, oldTip)
	}
	if b, _ := os.ReadFile(edited); string(b) != want {
		t.Fatalf("local edit lost: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dot, "notes", "pushed.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("pushed file appeared in a dirty checkout (err=%v)", err)
	}
}

func TestIntegrationMetadataFollowLeavesUntrackedFileAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	scratch := filepath.Join(dot, "notes", "scratch.txt")
	writeRepoFile(t, dot, "notes/scratch.txt", "mine\n")
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf(".docket HEAD = %s, want it untouched at %s", head, oldTip)
	}
	if b, _ := os.ReadFile(scratch); string(b) != "mine\n" {
		t.Fatalf("untracked file changed: %q", b)
	}
}

// TestIntegrationMetadataFollowLeavesInterruptedFastForwardAlone starts from the
// state an in-place fast-forward leaves when killed (or still running) between its
// ref swap and its tree update: the branch is at newTip, the index and files still
// at oldTip. Stacking a second fast-forward on it would run read-tree from newTip
// over an oldTip index, keep the missing newTip content as a "local change", and
// leave a checkout whose staged changes revert newTip under a plain dirty finding.
// It must be left exactly as it was, for the interrupted-fast-forward remedy.
func TestIntegrationMetadataFollowLeavesInterruptedFastForwardAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, dot, "update-ref", "-m", "docket: fast-forward", "refs/heads/docket", newTip, oldTip)
	nextTip := r.advanceRemoteDocket(t, "notes/next.txt", "next\n", "advance docket again")
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	indexBefore := runGit(t, dot, "write-tree")

	followAt(t, r.invocation, "refs/heads/docket", nextTip)
	if tip := localDocketTip(t, r); tip != newTip {
		t.Fatalf("local docket branch = %s, want it untouched at %s", tip, newTip)
	}
	if idx := runGit(t, dot, "write-tree"); idx != indexBefore {
		t.Fatalf(".docket index tree = %s, want it untouched at %s", idx, indexBefore)
	}
	for _, rel := range []string{"notes/advance.txt", "notes/next.txt"} {
		if _, err := os.Stat(filepath.Join(dot, rel)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s appeared in an interrupted checkout (err=%v)", rel, err)
		}
	}
}

// TestIntegrationMetadataFollowLeavesLocalCommitAlone: the local branch carries its
// own commit, so the pushed commit does not descend from it (diverged).
func TestIntegrationMetadataFollowLeavesLocalCommitAlone(t *testing.T) {
	r, _, newTip := newFollowRepo(t)
	local := r.commitInDocket(t, "notes/local.txt", "local\n", "local commit")
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != local {
		t.Fatalf(".docket HEAD = %s, want it untouched at the local commit %s", head, local)
	}
}

// TestIntegrationMetadataFollowLeavesCheckoutAlreadyPastAlone hands the follow a
// commit OLDER than the local tip (an already-applied replay, or a concurrent write
// that already moved the checkout): it must never move the branch backwards.
func TestIntegrationMetadataFollowLeavesCheckoutAlreadyPastAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf("setup: .docket HEAD = %s, want %s", head, newTip)
	}
	followAt(t, r.invocation, "refs/heads/docket", oldTip)
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket HEAD = %s, want it to stay at %s (never backwards)", head, newTip)
	}
}

func TestIntegrationMetadataFollowLeavesDetachedCheckoutAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, dot, "checkout", "-q", "--detach")
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf("detached .docket HEAD = %s, want %s", head, oldTip)
	}
	if tip := localDocketTip(t, r); tip != oldTip {
		t.Fatalf("local docket branch = %s, want %s (no checkout holds it)", tip, oldTip)
	}
}

// TestIntegrationMetadataFollowLeavesUnregisteredCloneAlone puts a standalone clone
// of the metadata branch where .docket belongs: not a worktree of this repository,
// yet on the docket branch, clean, behind, and holding the pushed commit's objects.
// Every Git probe run inside that directory would accept it, so only the worktree
// registration check keeps the follow from moving a repository docket never attached.
func TestIntegrationMetadataFollowLeavesUnregisteredCloneAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	removeDocketWorktree(t, r)
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, r.invocation, "clone", "-q", "-b", "docket", r.origin, dot)
	runGit(t, dot, "reset", "-q", "--hard", oldTip)
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := runGit(t, dot, "rev-parse", "HEAD"); head != oldTip {
		t.Fatalf("standalone clone at .docket moved to %s, want it untouched at %s", head, oldTip)
	}
	if tip := localDocketTip(t, r); tip != oldTip {
		t.Fatalf("local docket branch = %s, want %s", tip, oldTip)
	}
}

func TestIntegrationMetadataFollowNeverAttachesAbsentCheckout(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	removeDocketWorktree(t, r)
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf(".docket was (re)created by the follow (err=%v)", err)
	}
	if tip := localDocketTip(t, r); tip != oldTip {
		t.Fatalf("local docket branch = %s, want %s", tip, oldTip)
	}
}
