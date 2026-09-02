//go:build integration

package gitcli

import (
	"context"
	"github.com/danielhanold/docket/internal/testsupport"
	"testing"
)

// historyRepo initializes a fresh non-bare repository with a deterministic
// committer identity under testsupport.TempDir(t) and returns its path plus the Repository
// the history primitives run against. Callers add whatever history they need.
func historyRepo(t *testing.T) (string, Repository) {
	t.Helper()
	requireGit(t)
	dir := testsupport.TempDir(t)
	gitOut(t, dir, "init", "-b", "main")
	configRepoIdentity(t, dir)
	return dir, Repository{PrimaryWorktree: dir}
}

// commitFile writes rel with content, stages and commits it with subject, and
// returns the resulting commit OID.
func commitFile(t *testing.T, dir, rel, content, subject string) ObjectID {
	t.Helper()
	writeWorktreeFile(t, dir, rel, content)
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", subject)
	return ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
}

// TestIntegrationHistoryListTreesLinear proves ListHistoryTrees walks the
// complete reachable history newest-first, pairing each commit with its exact
// root tree OID (the git plumbing `%T` is the oracle).
func TestIntegrationHistoryListTreesLinear(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	c0 := commitFile(t, dir, "a.txt", "a\n", "c0")
	c1 := commitFile(t, dir, "b.txt", "b\n", "c1")
	c2 := commitFile(t, dir, "c.txt", "c\n", "c2")

	entries, err := c.ListHistoryTrees(ctx, repo, c2)
	if err != nil {
		t.Fatalf("ListHistoryTrees: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("ListHistoryTrees returned %d entries, want 3: %v", len(entries), entries)
	}
	wantCommits := []ObjectID{c2, c1, c0} // newest-first
	for i, want := range wantCommits {
		if entries[i].Commit != want {
			t.Errorf("entry[%d].Commit = %q, want %q", i, entries[i].Commit, want)
		}
		wantTree := ObjectID(gitOut(t, dir, "rev-parse", "--verify", string(want)+"^{tree}"))
		if entries[i].Tree != wantTree {
			t.Errorf("entry[%d].Tree = %q, want %q", i, entries[i].Tree, wantTree)
		}
	}
}

// TestIntegrationHistorySharedAncestryDisjoint proves HasSharedAncestry reports
// false for two genuinely disjoint parentless lineages.
func TestIntegrationHistorySharedAncestryDisjoint(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	main := commitFile(t, dir, "a.txt", "a\n", "main root")

	gitOut(t, dir, "checkout", "-q", "--orphan", "other")
	gitOut(t, dir, "rm", "-rfq", "--cached", ".")
	writeWorktreeFile(t, dir, "b.txt", "b\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", "orphan root")
	other := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

	shared, err := c.HasSharedAncestry(ctx, repo, main, other)
	if err != nil {
		t.Fatalf("HasSharedAncestry: %v", err)
	}
	if shared {
		t.Errorf("HasSharedAncestry(disjoint lineages) = true, want false")
	}
}

// TestIntegrationHistorySharedAncestrySameRoot proves two branches descending
// from a shared root report true.
func TestIntegrationHistorySharedAncestrySameRoot(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	commitFile(t, dir, "a.txt", "a\n", "shared root")
	root := ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))

	branchA := commitFile(t, dir, "a2.txt", "a2\n", "branch a")

	gitOut(t, dir, "checkout", "-q", "-b", "sideb", string(root))
	branchB := commitFile(t, dir, "b2.txt", "b2\n", "branch b")

	shared, err := c.HasSharedAncestry(ctx, repo, branchA, branchB)
	if err != nil {
		t.Fatalf("HasSharedAncestry: %v", err)
	}
	if !shared {
		t.Errorf("HasSharedAncestry(same-root branches) = false, want true")
	}
}

// TestIntegrationHistorySharedAncestrySelf proves a commit shares ancestry with
// itself.
func TestIntegrationHistorySharedAncestrySelf(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	only := commitFile(t, dir, "a.txt", "a\n", "only")

	shared, err := c.HasSharedAncestry(ctx, repo, only, only)
	if err != nil {
		t.Fatalf("HasSharedAncestry: %v", err)
	}
	if !shared {
		t.Errorf("HasSharedAncestry(commit, itself) = false, want true")
	}
}

// TestIntegrationHistorySharedAncestryAbsentOIDErrors proves a well-formed but
// absent OID surfaces as an error, never a fabricated false.
func TestIntegrationHistorySharedAncestryAbsentOIDErrors(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	only := commitFile(t, dir, "a.txt", "a\n", "only")
	absent := ObjectID("0123456789012345678901234567890123456789")

	if _, err := c.HasSharedAncestry(ctx, repo, only, absent); err == nil {
		t.Fatal("HasSharedAncestry with an absent OID returned nil error, want an error (not false)")
	}
}

// TestIntegrationHistorySharedAncestryMalformedOIDErrors proves a malformed OID
// is an invalid-request error.
func TestIntegrationHistorySharedAncestryMalformedOIDErrors(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	only := commitFile(t, dir, "a.txt", "a\n", "only")

	_, err := c.HasSharedAncestry(ctx, repo, only, ObjectID("not-a-hex-oid"))
	if err == nil {
		t.Fatal("HasSharedAncestry with a malformed OID returned nil error, want an error")
	}
	assertKind(t, err, KindInvalidRequest)
}

// TestIntegrationHistoryListTreesAbsentOIDErrors proves an absent tip surfaces
// as an error rather than an empty walk.
func TestIntegrationHistoryListTreesAbsentOIDErrors(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	commitFile(t, dir, "a.txt", "a\n", "only")
	absent := ObjectID("0123456789012345678901234567890123456789")

	if _, err := c.ListHistoryTrees(ctx, repo, absent); err == nil {
		t.Fatal("ListHistoryTrees on an absent tip returned nil error, want an error")
	}
}

// TestIntegrationHistoryTreeEntryIDs proves TreeEntryIDs returns the
// non-recursive tree entry for a requested subtree prefix (a `tree` entry, which
// ListTree's recursive-leaf parser would reject) and silently omits absent
// paths rather than erroring.
func TestIntegrationHistoryTreeEntryIDs(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	commit := commitFile(t, dir, "docs/x/f.txt", "hi\n", "seed docs")

	entries, err := c.TreeEntryIDs(ctx, repo, commit, []RepoPath{"docs", "absent"})
	if err != nil {
		t.Fatalf("TreeEntryIDs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("TreeEntryIDs returned %d entries, want 1 (docs only): %v", len(entries), entries)
	}
	got := entries[0]
	if got.Path != "docs" {
		t.Errorf("entry.Path = %q, want %q", got.Path, "docs")
	}
	if got.Type != "tree" {
		t.Errorf("entry.Type = %q, want tree", got.Type)
	}
	if got.Mode != "040000" {
		t.Errorf("entry.Mode = %q, want 040000", got.Mode)
	}
	wantOID := ObjectID(gitOut(t, dir, "rev-parse", "--verify", string(commit)+":docs"))
	if got.ObjectID != wantOID {
		t.Errorf("entry.ObjectID = %q, want %q (git's docs subtree OID)", got.ObjectID, wantOID)
	}
}

// TestIntegrationHistoryTreeEntryIDsBlob proves TreeEntryIDs also returns a
// blob entry for a requested blob path.
func TestIntegrationHistoryTreeEntryIDsBlob(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	commit := commitFile(t, dir, "top.txt", "top\n", "seed")

	entries, err := c.TreeEntryIDs(ctx, repo, commit, []RepoPath{"top.txt"})
	if err != nil {
		t.Fatalf("TreeEntryIDs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("TreeEntryIDs returned %d entries, want 1: %v", len(entries), entries)
	}
	if entries[0].Type != "blob" || entries[0].Path != "top.txt" {
		t.Errorf("entry = %+v, want a blob at top.txt", entries[0])
	}
}

// TestIntegrationHistoryTreeEntryIDsAbsentOnlyEmpty proves requesting only
// absent paths yields no entries and no error.
func TestIntegrationHistoryTreeEntryIDsAbsentOnlyEmpty(t *testing.T) {
	ctx := context.Background()
	c := newRealClient(t)
	dir, repo := historyRepo(t)

	commit := commitFile(t, dir, "top.txt", "top\n", "seed")

	entries, err := c.TreeEntryIDs(ctx, repo, commit, []RepoPath{"no/such/path"})
	if err != nil {
		t.Fatalf("TreeEntryIDs: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("TreeEntryIDs for an absent path = %v, want no entries", entries)
	}
}
