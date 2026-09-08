//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// preserveRemoteFixture builds two independent non-bare repositories: a remote
// whose main branch tip (remoteHead) is reachable but ABSENT from the local
// repository, and a local repository whose "origin" remote points at it with a
// commit of its own (localHead). It is the seam the ensurePreservationInputs
// fetch cases exercise — a required exact object that lives only on the remote.
func preserveRemoteFixture(t *testing.T) (dir string, repo Repository, localHead, remoteHead ObjectID) {
	t.Helper()
	requireGit(t)

	remote := testsupport.TempDir(t)
	gitOut(t, remote, "init", "-b", "main")
	configRepoIdentity(t, remote)
	commitFile(t, remote, "r0.txt", "r0\n", "r0")
	remoteHead = commitFile(t, remote, "r1.txt", "r1\n", "r1")

	dir = testsupport.TempDir(t)
	gitOut(t, dir, "init", "-b", "main")
	configRepoIdentity(t, dir)
	localHead = commitFile(t, dir, "l0.txt", "l0\n", "l0")
	gitOut(t, dir, "remote", "add", "origin", remote)
	return dir, Repository{PrimaryWorktree: dir}, localHead, remoteHead
}

// TestIntegrationPreserveCommitMergeBasesAll proves mergeBasesAll returns EVERY
// best common ancestor: the single base of a linear pair, both bases of a
// criss-cross, and the empty slice (no error) for two orphan roots.
func TestIntegrationPreserveCommitMergeBasesAll(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)

	t.Run("linear-single-base", func(t *testing.T) {
		dir, repo := historyRepo(t)
		a := commitFile(t, dir, "a.txt", "a\n", "a")
		b := commitFile(t, dir, "b.txt", "b\n", "b")

		bases, f := c.mergeBasesAll(ctx, repo, a, b)
		if f != nil {
			t.Fatalf("mergeBasesAll: %v", f)
		}
		if len(bases) != 1 || bases[0] != a {
			t.Fatalf("bases = %v, want single [%s]", bases, a)
		}
	})

	t.Run("criss-cross-two-bases", func(t *testing.T) {
		dir, repo := historyRepo(t)
		commitFile(t, dir, "root.txt", "root\n", "root")
		root := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

		// branch x: root + a
		gitOut(t, dir, "checkout", "-q", "-b", "x", string(root))
		a := commitFile(t, dir, "a.txt", "a\n", "a")
		// branch y: root + b
		gitOut(t, dir, "checkout", "-q", "-b", "y", string(root))
		b := commitFile(t, dir, "b.txt", "b\n", "b")

		// x merges b (parents a, b); y merges a (parents b, a): a criss-cross whose
		// two merge bases are exactly a and b. Merge the captured commit IDs, never
		// the branch names — the first merge advances branch x to m1, so a later
		// `merge x` would fold m1 in and collapse the criss-cross to a single base.
		gitOut(t, dir, "checkout", "-q", "x")
		gitOut(t, dir, "merge", "--no-ff", "--no-edit", string(b))
		m1 := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
		gitOut(t, dir, "checkout", "-q", "y")
		gitOut(t, dir, "merge", "--no-ff", "--no-edit", string(a))
		m2 := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

		bases, f := c.mergeBasesAll(ctx, repo, m1, m2)
		if f != nil {
			t.Fatalf("mergeBasesAll: %v", f)
		}
		if len(bases) != 2 {
			t.Fatalf("bases = %v, want two (criss-cross)", bases)
		}
		got := map[ObjectID]bool{bases[0]: true, bases[1]: true}
		if !got[a] || !got[b] {
			t.Fatalf("bases = %v, want the set {%s, %s}", bases, a, b)
		}
	})

	t.Run("orphan-roots-no-base", func(t *testing.T) {
		dir, repo := historyRepo(t)
		main := commitFile(t, dir, "a.txt", "a\n", "main root")

		gitOut(t, dir, "checkout", "-q", "--orphan", "other")
		gitOut(t, dir, "rm", "-rfq", "--cached", ".")
		other := commitFile(t, dir, "b.txt", "b\n", "orphan root")

		bases, f := c.mergeBasesAll(ctx, repo, main, other)
		if f != nil {
			t.Fatalf("mergeBasesAll: %v", f)
		}
		if len(bases) != 0 {
			t.Fatalf("bases = %v, want empty (disjoint roots)", bases)
		}
	})
}

// TestIntegrationPreserveCommitEnsureInputs proves ensurePreservationInputs
// validates ids, refuses shallow history, resolves commits, and — for a missing
// object — fetches the exact id once from the established remote, distinguishing
// an obtainable object from one absent everywhere and a present non-commit.
func TestIntegrationPreserveCommitEnsureInputs(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)

	t.Run("invalid-id", func(t *testing.T) {
		dir, repo, localHead, _ := preserveRemoteFixture(t)
		_ = dir
		f := c.ensurePreservationInputs(ctx, repo, "origin", ObjectID("abc123"), localHead)
		if f == nil {
			t.Fatal("ensurePreservationInputs with a short-hex id returned nil, want a failure")
		}
		if f.Kind != KindInvalidRequest {
			t.Fatalf("Kind = %q, want %q", f.Kind, KindInvalidRequest)
		}
	})

	t.Run("missing-object-absent-on-remote", func(t *testing.T) {
		_, repo, localHead, _ := preserveRemoteFixture(t)
		absent := ObjectID("0123456789012345678901234567890123456789")
		f := c.ensurePreservationInputs(ctx, repo, "origin", absent, localHead)
		if f == nil {
			t.Fatal("ensurePreservationInputs with an object absent everywhere returned nil, want a failure")
		}
		// The object exists neither locally nor on the remote: an observation
		// failure, never a fabricated proof.
	})

	t.Run("missing-object-present-on-remote", func(t *testing.T) {
		_, repo, localHead, remoteHead := preserveRemoteFixture(t)
		// remoteHead is reachable on origin but absent locally; the resolver must
		// fetch it exactly once and then succeed.
		f := c.ensurePreservationInputs(ctx, repo, "origin", remoteHead, localHead)
		if f != nil {
			t.Fatalf("ensurePreservationInputs should have fetched the present-on-remote object: %v", f)
		}
	})

	t.Run("blob-id-is-not-a-commit", func(t *testing.T) {
		dir, repo, localHead, _ := preserveRemoteFixture(t)
		blob := ObjectID(gitHashObject(t, dir, "some loose blob bytes\n"))
		f := c.ensurePreservationInputs(ctx, repo, "origin", blob, localHead)
		if f == nil {
			t.Fatal("ensurePreservationInputs with a blob id returned nil, want a failure (not a commit)")
		}
	})

	t.Run("shallow-history-refused", func(t *testing.T) {
		remote := testsupport.TempDir(t)
		gitOut(t, remote, "init", "-b", "main")
		configRepoIdentity(t, remote)
		commitFile(t, remote, "r0.txt", "r0\n", "r0")
		commitFile(t, remote, "r1.txt", "r1\n", "r1")

		parent := testsupport.TempDir(t)
		gitOut(t, parent, "clone", "--depth", "1", "file://"+remote, "shallow")
		shallowDir := filepath.Join(parent, "shallow")
		configRepoIdentity(t, shallowDir)
		repo := Repository{PrimaryWorktree: shallowDir}
		head := ObjectID(gitOut(t, shallowDir, "rev-parse", "HEAD"))

		f := c.ensurePreservationInputs(ctx, repo, "origin", head, head)
		if f == nil {
			t.Fatal("ensurePreservationInputs on a shallow clone returned nil, want a failure")
		}
		if f.Kind != KindInvalidRepository {
			t.Fatalf("Kind = %q, want %q (shallow history)", f.Kind, KindInvalidRepository)
		}
	})
}

// TestIntegrationPreserveCommitDelta proves commitDelta reports the exact
// source-side delta of a real base->source pair, rename detection disabled: an
// add, a content modify, a delete (all-zero new side, "000000" mode), a
// mode-only change (chmod +x, same blob oid), and a new symlink (mode 120000).
// Expected new oids come from git rev-parse <source>:<path> — the plumbing
// oracle — never a fabricated constant.
func TestIntegrationPreserveCommitDelta(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	// base: files that will be modified, deleted, and chmod'd at source.
	writeWorktreeFile(t, dir, "mod.txt", "v1\n")
	writeWorktreeFile(t, dir, "del.txt", "bye\n")
	writeWorktreeFile(t, dir, "exe.sh", "#!/bin/sh\necho hi\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "base")
	base := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

	// source: add new.txt, modify mod.txt, delete del.txt, chmod +x exe.sh
	// (content unchanged so the blob oid is stable), add a symlink.
	writeWorktreeFile(t, dir, "new.txt", "brand new\n")
	writeWorktreeFile(t, dir, "mod.txt", "v2\n")
	if err := os.Remove(filepath.Join(dir, "del.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "exe.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("mod.txt", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "source")
	source := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

	entries, f := c.commitDelta(ctx, repo, base, source)
	if f != nil {
		t.Fatalf("commitDelta: %v", f)
	}

	zero := ObjectID(strings.Repeat("0", len(string(source))))
	revOID := func(path string) ObjectID {
		return ObjectID(gitOut(t, dir, "rev-parse", "--verify", string(source)+":"+path))
	}
	want := map[RepoPath]deltaEntry{
		"new.txt": {Path: "new.txt", Status: 'A', NewMode: "100644", NewOID: revOID("new.txt")},
		"mod.txt": {Path: "mod.txt", Status: 'M', NewMode: "100644", NewOID: revOID("mod.txt")},
		"del.txt": {Path: "del.txt", Status: 'D', NewMode: "000000", NewOID: zero},
		"exe.sh":  {Path: "exe.sh", Status: 'M', NewMode: "100755", NewOID: revOID("exe.sh")},
		"link":    {Path: "link", Status: 'A', NewMode: "120000", NewOID: revOID("link")},
	}

	if len(entries) != len(want) {
		t.Fatalf("commitDelta returned %d entries, want %d: %+v", len(entries), len(want), entries)
	}
	got := make(map[RepoPath]deltaEntry, len(entries))
	for _, e := range entries {
		got[e.Path] = e
	}
	for path, w := range want {
		g, ok := got[path]
		if !ok {
			t.Fatalf("commitDelta missing entry for %q; got %+v", path, entries)
		}
		if g != w {
			t.Errorf("entry for %q = %+v, want %+v", path, g, w)
		}
	}

	// exe.sh is a pure mode change: same blob oid on both sides.
	if got["exe.sh"].NewOID != base && got["exe.sh"].NewOID != revOID("exe.sh") {
		t.Fatalf("exe.sh oid = %q; expected the unchanged blob oid", got["exe.sh"].NewOID)
	}
	baseExe := ObjectID(gitOut(t, dir, "rev-parse", "--verify", string(base)+":exe.sh"))
	if got["exe.sh"].NewOID != baseExe {
		t.Errorf("chmod exe.sh oid = %q, want the unchanged base blob %q", got["exe.sh"].NewOID, baseExe)
	}
}

// gitHashObject writes a loose blob into the repo at dir and returns its OID,
// giving the tests a real, present, NON-commit object to probe.
func gitHashObject(t *testing.T, dir, content string) string {
	t.Helper()
	writeWorktreeFile(t, dir, "loose-blob.bin", content)
	return gitOut(t, dir, "hash-object", "-w", "loose-blob.bin")
}
