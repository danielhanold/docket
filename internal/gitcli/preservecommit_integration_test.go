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

// assertProof pins the full outcome of a ProvePreserved verdict — Outcome, Kind,
// Detail, and membership of every wanted diagnostic path — never a bare
// "not proven" (learning assert-pins-outcome-not-mechanism). Kind is asserted
// even when unproven ("" — set only on a proven verdict), and every wantPath
// must appear in Paths.
func assertProof(t *testing.T, got PreservationCheck, outcome PreservationOutcome, kind PreservationKind, detail string, wantPaths ...RepoPath) {
	t.Helper()
	if got.Outcome != outcome {
		t.Fatalf("Outcome = %q, want %q (full: %+v)", got.Outcome, outcome, got)
	}
	if got.Kind != kind {
		t.Fatalf("Kind = %q, want %q (full: %+v)", got.Kind, kind, got)
	}
	if got.Detail != detail {
		t.Fatalf("Detail = %q, want %q (full: %+v)", got.Detail, detail, got)
	}
	for _, wp := range wantPaths {
		if !containsPath(got.Paths, wp) {
			t.Fatalf("Paths %v does not contain %q (full: %+v)", got.Paths, wp, got)
		}
	}
}

// containsPath reports whether paths contains p.
func containsPath(paths []RepoPath, p RepoPath) bool {
	for _, q := range paths {
		if q == p {
			return true
		}
	}
	return false
}

// commitAll stages every worktree change and commits it with subject, returning
// the resulting commit OID — the multi-path sibling of commitFile.
func commitAll(t *testing.T, dir, subject string) ObjectID {
	t.Helper()
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", subject)
	return ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
}

// TestIntegrationPreserveCommitProve is the preservation-primitive matrix: every
// row builds a real repository and asserts ProvePreserved's exact verdict.
// Ancestry proves directly; otherwise the base->source tracked-entry delta must
// match target exactly (oid+mode, and a source deletion requires absence). The
// rows cover the spec's proof/refusal vocabulary end to end — a rewrite that
// preserves content, a drop, a partial loss, an overlapping edit, deletions,
// renames (rename detection OFF), mode changes, the hostile object shapes
// (symlink, binary, gitlink, dir<->file, unusual path), the base-count refusals,
// an empty delta, a missing object error, and a custom merge driver that cannot
// manufacture proof.
func TestIntegrationPreserveCommitProve(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)

	// Row 1: source is an ancestor of target -> proven by ancestry, no delta walk.
	t.Run("ancestral", func(t *testing.T) {
		dir, repo := historyRepo(t)
		commitFile(t, dir, "a.txt", "a\n", "c0")
		source := commitFile(t, dir, "b.txt", "b\n", "c1")
		target := commitFile(t, dir, "c.txt", "c\n", "c2")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByAncestry, "")
	})

	// Row 2: a rebase rewrites the child onto an advanced main — new commit ids,
	// identical blobs -> proven by exact content.
	t.Run("rebase-rewrite", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		source := commitFile(t, dir, "catalog.yaml", "items: [a, b]\n", "add catalog")
		gitOut(t, dir, "checkout", "-q", "main")
		commitFile(t, dir, "other.txt", "other\n", "advance main")
		gitOut(t, dir, "checkout", "-q", "child")
		gitOut(t, dir, "rebase", "main")
		target := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	// Row 3: two child commits squashed into one on the target line -> proven by
	// exact content.
	t.Run("squash-rewrite", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		commitFile(t, dir, "f1.txt", "one\n", "add f1")
		source := commitFile(t, dir, "f2.txt", "two\n", "add f2")
		gitOut(t, dir, "checkout", "-q", "-b", "squashed", string(b0))
		writeWorktreeFile(t, dir, "f1.txt", "one\n")
		writeWorktreeFile(t, dir, "f2.txt", "two\n")
		target := commitAll(t, dir, "squash f1+f2")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	// Row 4: like row 2, plus target advances further touching only OTHER files —
	// extra target changes outside the delta are allowed -> proven by content.
	t.Run("unrelated-destination-advance", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		source := commitFile(t, dir, "catalog.yaml", "items\n", "add catalog")
		gitOut(t, dir, "checkout", "-q", "main")
		commitFile(t, dir, "other.txt", "other\n", "advance main")
		gitOut(t, dir, "checkout", "-q", "child")
		gitOut(t, dir, "rebase", "main")
		commitFile(t, dir, "more.txt", "more\n", "post-rewrite unrelated advance")
		target := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	// Row 5: the target line omits catalog.yaml entirely — the source blob EXISTS
	// (asserted below), pinning a DROP versus a missing object -> unproven,
	// entry-differs, catalog.yaml differs.
	t.Run("content-dropped", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		source := commitFile(t, dir, "catalog.yaml", "items\n", "add catalog")
		// The source's blob is present — this is a drop, not a missing object.
		gitOut(t, dir, "cat-file", "-e", string(source)+":catalog.yaml")
		gitOut(t, dir, "checkout", "-q", "main")
		target := commitFile(t, dir, "other.txt", "other\n", "advance without catalog")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "catalog.yaml")
	})

	// Row 6: two changed files, one preserved one dropped -> unproven, and only
	// the dropped path is reported differing.
	t.Run("partial-loss", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		writeWorktreeFile(t, dir, "keep.txt", "keep\n")
		writeWorktreeFile(t, dir, "drop.txt", "drop\n")
		source := commitAll(t, dir, "add keep+drop")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		writeWorktreeFile(t, dir, "keep.txt", "keep\n")
		target := commitAll(t, dir, "carry keep only")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "drop.txt")
		if containsPath(got.Paths, "keep.txt") {
			t.Fatalf("keep.txt was preserved but appears in differing Paths %v", got.Paths)
		}
	})

	// Row 7: target carries catalog.yaml with DIFFERENT bytes — matched on oid,
	// not filename presence -> unproven, entry-differs (spec §10 ambiguity refuses).
	t.Run("overlapping-edit", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		source := commitFile(t, dir, "catalog.yaml", "items: original\n", "add catalog")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		target := commitFile(t, dir, "catalog.yaml", "items: edited differently\n", "add edited catalog")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "catalog.yaml")
	})

	// Row 8a: source deletes a file; target absent it too -> proven by content
	// (a source deletion requires absence).
	t.Run("deletion-preserved", func(t *testing.T) {
		dir, repo := historyRepo(t)
		writeWorktreeFile(t, dir, "doomed.txt", "bye\n")
		writeWorktreeFile(t, dir, "keep.txt", "stay\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "rm", "-q", "doomed.txt")
		source := commitAll(t, dir, "delete doomed")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		gitOut(t, dir, "rm", "-q", "doomed.txt")
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "delete doomed on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	// Row 8b: source deletes a file; target still HAS it -> unproven, entry-differs.
	// This is the row the second hostile probe (flip the 'D' match to true) reddens.
	t.Run("deletion-lost", func(t *testing.T) {
		dir, repo := historyRepo(t)
		writeWorktreeFile(t, dir, "doomed.txt", "bye\n")
		writeWorktreeFile(t, dir, "keep.txt", "stay\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "rm", "-q", "doomed.txt")
		source := commitAll(t, dir, "delete doomed")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "keep doomed on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "doomed.txt")
	})

	// Row 9a: source renames old->new (rename detection OFF: delete + add); target
	// preserves both sides -> proven by content.
	t.Run("rename-preserved", func(t *testing.T) {
		dir, repo := historyRepo(t)
		writeWorktreeFile(t, dir, "old.txt", "content\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "mv", "old.txt", "new.txt")
		source := commitAll(t, dir, "rename old->new")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		gitOut(t, dir, "mv", "old.txt", "new.txt")
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "rename old->new on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	// Row 9b: source renames old->new; target keeps the OLD name only -> unproven,
	// both the vanished-delete and the missing-add surface.
	t.Run("rename-lost", func(t *testing.T) {
		dir, repo := historyRepo(t)
		writeWorktreeFile(t, dir, "old.txt", "content\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "mv", "old.txt", "new.txt")
		source := commitAll(t, dir, "rename old->new")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "keep old name on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "old.txt", "new.txt")
	})

	// Row 10a: chmod +x preserved (same blob oid, mode 100755) -> proven by content.
	t.Run("mode-change-preserved", func(t *testing.T) {
		dir, repo := historyRepo(t)
		writeWorktreeFile(t, dir, "script.sh", "#!/bin/sh\necho hi\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		if err := os.Chmod(filepath.Join(dir, "script.sh"), 0o755); err != nil {
			t.Fatal(err)
		}
		source := commitAll(t, dir, "chmod +x script")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		if err := os.Chmod(filepath.Join(dir, "script.sh"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "chmod +x on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	// Row 10b: chmod +x lost — target keeps mode 100644 (same blob) -> unproven,
	// the exact mode mismatch is reported.
	t.Run("mode-change-lost", func(t *testing.T) {
		dir, repo := historyRepo(t)
		writeWorktreeFile(t, dir, "script.sh", "#!/bin/sh\necho hi\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		if err := os.Chmod(filepath.Join(dir, "script.sh"), 0o755); err != nil {
			t.Fatal(err)
		}
		source := commitAll(t, dir, "chmod +x script")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "leave mode unchanged on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "script.sh")
	})

	// Row 11a: symlink preserved vs lost (mode 120000).
	t.Run("symlink-preserved", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "target-file.txt", "data\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		if err := os.Symlink("target-file.txt", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		source := commitAll(t, dir, "add symlink")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		if err := os.Symlink("target-file.txt", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "add symlink on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	t.Run("symlink-lost", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "target-file.txt", "data\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		if err := os.Symlink("target-file.txt", filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		source := commitAll(t, dir, "add symlink")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		target := commitFile(t, dir, "extra.txt", "x\n", "no symlink on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "link")
	})

	// Row 11b: binary blob (embedded NUL bytes) preserved vs lost.
	t.Run("binary-blob-preserved", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		bin := "\x00\x01\x02\xff\xfe\x00payload\x00\n"
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		writeWorktreeFile(t, dir, "blob.bin", bin)
		source := commitAll(t, dir, "add binary")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		writeWorktreeFile(t, dir, "blob.bin", bin)
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "add binary on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	t.Run("binary-blob-lost", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		writeWorktreeFile(t, dir, "blob.bin", "\x00\x01\x02original\x00\n")
		source := commitAll(t, dir, "add binary")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		writeWorktreeFile(t, dir, "blob.bin", "\x00\x01\x02DIFFERENT\x00\n")
		target := commitAll(t, dir, "add different binary on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "blob.bin")
	})

	// Row 11c: gitlink (submodule pin, mode 160000) preserved vs lost. The pinned
	// commit id need not be a present object — it is recorded in the tree.
	t.Run("gitlink-preserved", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		sub := "1111111111111111111111111111111111111111"
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+sub+",sub")
		gitOut(t, dir, "commit", "-q", "-m", "add gitlink")
		source := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		gitOut(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+sub+",sub")
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		gitOut(t, dir, "add", "extra.txt")
		gitOut(t, dir, "commit", "-q", "-m", "add gitlink on dest line")
		target := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	t.Run("gitlink-lost", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "update-index", "--add", "--cacheinfo", "160000,1111111111111111111111111111111111111111,sub")
		gitOut(t, dir, "commit", "-q", "-m", "add gitlink")
		source := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		gitOut(t, dir, "update-index", "--add", "--cacheinfo", "160000,2222222222222222222222222222222222222222,sub")
		gitOut(t, dir, "commit", "-q", "-m", "different gitlink pin on dest line")
		target := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "sub")
	})

	// Row 11d: a file->directory transition. Framed as a directory replaced by a
	// file at the same path so the delta is {xfd/inner.txt D, xfd A(blob)}: the
	// preserved target carries the file; the lost target keeps the directory, whose
	// ls-tree tree entry (mode 040000) can never equal the delta's blob mode.
	t.Run("file-to-directory-preserved", func(t *testing.T) {
		dir, repo := historyRepo(t)
		writeWorktreeFile(t, dir, "xfd/inner.txt", "inner\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "rm", "-q", "xfd/inner.txt")
		writeWorktreeFile(t, dir, "xfd", "now a file\n")
		source := commitAll(t, dir, "dir xfd becomes a file")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		gitOut(t, dir, "rm", "-q", "xfd/inner.txt")
		writeWorktreeFile(t, dir, "xfd", "now a file\n")
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "same transition on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	t.Run("file-to-directory-lost", func(t *testing.T) {
		dir, repo := historyRepo(t)
		writeWorktreeFile(t, dir, "xfd/inner.txt", "inner\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "rm", "-q", "xfd/inner.txt")
		writeWorktreeFile(t, dir, "xfd", "now a file\n")
		source := commitAll(t, dir, "dir xfd becomes a file")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "keep xfd directory on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		// Both the delete (xfd/inner.txt still present) and the add (xfd is a
		// directory, mode 040000 != a blob mode) surface as differing.
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "xfd/inner.txt", "xfd")
	})

	// Row 11e: an unusual path (space + UTF-8) preserved vs lost.
	t.Run("unusual-path-preserved", func(t *testing.T) {
		dir, repo := historyRepo(t)
		odd := "my dir/naïve café.txt"
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		writeWorktreeFile(t, dir, odd, "hi\n")
		source := commitAll(t, dir, "add unusual path")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		writeWorktreeFile(t, dir, odd, "hi\n")
		writeWorktreeFile(t, dir, "extra.txt", "x\n")
		target := commitAll(t, dir, "add unusual path on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationProven, PreservationByContent, "")
	})

	t.Run("unusual-path-lost", func(t *testing.T) {
		dir, repo := historyRepo(t)
		odd := "my dir/naïve café.txt"
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		writeWorktreeFile(t, dir, odd, "hi\n")
		source := commitAll(t, dir, "add unusual path")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		target := commitFile(t, dir, "extra.txt", "x\n", "no unusual path on dest line")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, RepoPath(odd))
	})

	// Row 12: an orphan target root shares no history -> unproven, no-common-base.
	t.Run("no-common-base", func(t *testing.T) {
		dir, repo := historyRepo(t)
		source := commitFile(t, dir, "a.txt", "a\n", "main root")
		gitOut(t, dir, "checkout", "-q", "--orphan", "other")
		gitOut(t, dir, "rm", "-rfq", "--cached", ".")
		target := commitFile(t, dir, "b.txt", "b\n", "orphan root")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveNoCommonBase)
	})

	// Row 13: a criss-cross with two merge bases -> unproven, multiple-bases (never
	// pick an arbitrary base). This is the row the first hostile probe reddens.
	t.Run("multiple-bases", func(t *testing.T) {
		dir, repo := historyRepo(t)
		commitFile(t, dir, "root.txt", "root\n", "root")
		root := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
		gitOut(t, dir, "checkout", "-q", "-b", "x", string(root))
		a := commitFile(t, dir, "a.txt", "a\n", "a")
		gitOut(t, dir, "checkout", "-q", "-b", "y", string(root))
		b := commitFile(t, dir, "b.txt", "b\n", "b")
		gitOut(t, dir, "checkout", "-q", "x")
		gitOut(t, dir, "merge", "--no-ff", "--no-edit", string(b))
		source := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
		gitOut(t, dir, "checkout", "-q", "y")
		gitOut(t, dir, "merge", "--no-ff", "--no-edit", string(a))
		target := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveMultipleBases)
	})

	// Row 14: source is an empty commit on the sole merge base (equal tree) and is
	// not an ancestor of target -> unproven, empty-delta (never manufacture proof
	// from an empty population).
	t.Run("empty-delta-non-ancestral", func(t *testing.T) {
		dir, repo := historyRepo(t)
		b0 := commitFile(t, dir, "base.txt", "base\n", "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		gitOut(t, dir, "commit", "-q", "--allow-empty", "-m", "empty child commit")
		source := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		target := commitFile(t, dir, "advance.txt", "adv\n", "advance dest")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEmptyDelta)
	})

	// Row 15: a 40-hex source absent everywhere is an observation ERROR from input
	// resolution — never an outcome.
	t.Run("missing-source-object", func(t *testing.T) {
		dir, repo := historyRepo(t)
		target := commitFile(t, dir, "a.txt", "a\n", "c0")
		absent := ObjectID("0123456789012345678901234567890123456789")

		got, err := c.ProvePreserved(ctx, repo, "origin", absent, target)
		if err == nil {
			t.Fatalf("ProvePreserved with an absent source object returned nil error, want an observation error (got %+v)", got)
		}
	})

	// Row 16: a custom merge driver that would "resolve" anything cannot
	// manufacture proof — re-run the overlapping edit; the comparison never invokes
	// merge machinery (spec §9).
	t.Run("merge-driver-cannot-help", func(t *testing.T) {
		dir, repo := historyRepo(t)
		gitOut(t, dir, "config", "merge.alwaysours.driver", "true")
		writeWorktreeFile(t, dir, ".gitattributes", "* merge=alwaysours\n")
		writeWorktreeFile(t, dir, "base.txt", "base\n")
		b0 := commitAll(t, dir, "b0")
		gitOut(t, dir, "checkout", "-q", "-b", "child", string(b0))
		source := commitFile(t, dir, "catalog.yaml", "items: original\n", "add catalog")
		gitOut(t, dir, "checkout", "-q", "-b", "dest", string(b0))
		target := commitFile(t, dir, "catalog.yaml", "items: edited differently\n", "add edited catalog")

		got, err := c.ProvePreserved(ctx, repo, "origin", source, target)
		if err != nil {
			t.Fatalf("ProvePreserved: %v", err)
		}
		assertProof(t, got, PreservationUnproven, "", PreserveEntryDiffers, "catalog.yaml")
	})
}
