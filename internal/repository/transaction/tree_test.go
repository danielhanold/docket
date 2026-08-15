package transaction

import (
	"bytes"
	"context"
	"sort"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// fakeSource is an in-memory gitcli.ObjectSource backed by a path→entry map. It
// mirrors the real source's observable semantics closely enough to exercise the
// tree overlay: prefix-scoped, path-sorted, deduped ListTree and request-ordered
// ReadBlobs with fresh byte copies.
type fakeSource struct {
	rev     gitcli.Revision
	entries map[gitcli.RepoPath]fakeEntry
}

type fakeEntry struct {
	mode  gitcli.FileMode
	typ   gitcli.ObjectType
	oid   gitcli.ObjectID
	bytes []byte
}

// Compile-time proof the fake satisfies the interface the tree consumes.
var _ gitcli.ObjectSource = (*fakeSource)(nil)

func (f *fakeSource) Revision() gitcli.Revision { return f.rev }

func (f *fakeSource) ListTree(_ context.Context, prefixes []gitcli.RepoPath) ([]gitcli.TreeEntry, error) {
	var out []gitcli.TreeEntry
	for p, e := range f.entries {
		if !fakeMatches(p, prefixes) {
			continue
		}
		out = append(out, gitcli.TreeEntry{Path: p, Mode: e.mode, Type: e.typ, ObjectID: e.oid})
	}
	sort.Slice(out, func(i, j int) bool {
		return bytes.Compare([]byte(out[i].Path), []byte(out[j].Path)) < 0
	})
	return out, nil
}

func (f *fakeSource) ReadBlobs(_ context.Context, paths []gitcli.RepoPath) ([]gitcli.BlobResult, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	out := make([]gitcli.BlobResult, len(paths))
	for i, p := range paths {
		out[i].Path = p
		e, ok := f.entries[p]
		if !ok {
			continue
		}
		cp := make([]byte, len(e.bytes))
		copy(cp, e.bytes)
		out[i].Found = true
		out[i].Blob = gitcli.Blob{Mode: e.mode, ObjectID: e.oid, Bytes: cp}
	}
	return out, nil
}

// fakeMatches replicates git's `-- <prefix>` scoping: empty prefixes (or one
// containing "") lists everything; otherwise a path matches a prefix when it
// equals it or sits beneath it.
func fakeMatches(path gitcli.RepoPath, prefixes []gitcli.RepoPath) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, pre := range prefixes {
		if pre == "" || path == pre {
			return true
		}
		if len(path) > len(pre) && string(path[:len(pre)]) == string(pre) && path[len(pre)] == '/' {
			return true
		}
	}
	return false
}

func newFixture() *fakeSource {
	return &fakeSource{
		rev: gitcli.Revision{
			Commit: gitcli.ObjectID("1111111111111111111111111111111111111111"),
			Remote: gitcli.RemoteName("origin"),
			Ref:    gitcli.RefName("refs/heads/docket"),
		},
		entries: map[gitcli.RepoPath]fakeEntry{
			"docs/a.md": {mode: "100644", typ: "blob",
				oid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", bytes: []byte("alpha\n")},
			"docs/b.md": {mode: "100644", typ: "blob",
				oid: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", bytes: []byte("beta\n")},
			"bin/run.sh": {mode: "100755", typ: "blob",
				oid: "cccccccccccccccccccccccccccccccccccccccc", bytes: []byte("#!/bin/sh\n")},
			"link": {mode: "120000", typ: "blob",
				oid: "dddddddddddddddddddddddddddddddddddddddd", bytes: []byte("docs/a.md")},
			"gitlink": {mode: "160000", typ: "commit",
				oid: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", bytes: nil},
		},
	}
}

// validReceipt is a canonical compact JSON object accepted by validateReceipt.
func validReceipt() []byte { return []byte(`{"ok":true}`) }

func makePlan(files ...FileMutation) MutationPlan {
	return MutationPlan{Files: files, CommitSubject: "test subject", Receipt: validReceipt()}
}

func findEntry(entries []gitcli.TreeEntry, path gitcli.RepoPath) (gitcli.TreeEntry, bool) {
	for _, e := range entries {
		if e.Path == path {
			return e, true
		}
	}
	return gitcli.TreeEntry{}, false
}

func readOne(t *testing.T, tr Tree, path gitcli.RepoPath) gitcli.BlobResult {
	t.Helper()
	res, err := tr.ReadBlobs(context.Background(), []gitcli.RepoPath{path})
	if err != nil {
		t.Fatalf("ReadBlobs(%q): %v", path, err)
	}
	if len(res) != 1 {
		t.Fatalf("ReadBlobs(%q): got %d results, want 1", path, len(res))
	}
	return res[0]
}

func TestBaseTreePassesThrough(t *testing.T) {
	src := newFixture()
	bt := newBaseTree(src)

	if bt.Revision() != src.rev {
		t.Fatalf("base Revision = %+v, want %+v", bt.Revision(), src.rev)
	}

	entries, err := bt.ListTree(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if len(entries) != len(src.entries) {
		t.Fatalf("ListTree returned %d entries, want %d", len(entries), len(src.entries))
	}
	e, ok := findEntry(entries, "bin/run.sh")
	if !ok || e.Mode != "100755" {
		t.Fatalf("bin/run.sh entry = %+v (found=%v), want mode 100755", e, ok)
	}

	got := readOne(t, bt, "docs/a.md")
	if !got.Found || !bytes.Equal(got.Blob.Bytes, []byte("alpha\n")) {
		t.Fatalf("base ReadBlobs docs/a.md = %+v, want alpha", got)
	}
}

func TestOverlayReportsBaseRevision(t *testing.T) {
	src := newFixture()
	ov, err := newOverlayTree(newBaseTree(src), makePlan(
		FileMutation{Path: "docs/c.md", Kind: MutationCreate, Bytes: []byte("gamma\n")},
	))
	if err != nil {
		t.Fatalf("newOverlayTree: %v", err)
	}
	if ov.Revision() != src.rev {
		t.Fatalf("overlay Revision = %+v, want base %+v", ov.Revision(), src.rev)
	}
}

func TestOverlayCreate(t *testing.T) {
	ov, err := newOverlayTree(newBaseTree(newFixture()), makePlan(
		FileMutation{Path: "docs/c.md", Kind: MutationCreate, Bytes: []byte("gamma\n")},
	))
	if err != nil {
		t.Fatalf("newOverlayTree: %v", err)
	}

	entries, err := ov.ListTree(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	e, ok := findEntry(entries, "docs/c.md")
	if !ok {
		t.Fatalf("created path docs/c.md absent from ListTree")
	}
	if e.Mode != "100644" || e.Type != "blob" || e.ObjectID != "" {
		t.Fatalf("created entry = %+v, want mode 100644 / blob / empty oid", e)
	}

	got := readOne(t, ov, "docs/c.md")
	if !got.Found || !bytes.Equal(got.Blob.Bytes, []byte("gamma\n")) || got.Blob.Mode != "100644" {
		t.Fatalf("overlay ReadBlobs docs/c.md = %+v, want gamma / 100644", got)
	}
	if got.Blob.ObjectID != "" {
		t.Fatalf("overlay create ObjectID = %q, want empty", got.Blob.ObjectID)
	}
}

func TestOverlayReplacePreservesMode(t *testing.T) {
	ov, err := newOverlayTree(newBaseTree(newFixture()), makePlan(
		FileMutation{Path: "bin/run.sh", Kind: MutationReplace, Bytes: []byte("#!/bin/bash\n")},
	))
	if err != nil {
		t.Fatalf("newOverlayTree: %v", err)
	}

	entries, err := ov.ListTree(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	e, ok := findEntry(entries, "bin/run.sh")
	if !ok || e.Mode != "100755" {
		t.Fatalf("replaced entry = %+v (found=%v), want mode 100755 preserved", e, ok)
	}

	got := readOne(t, ov, "bin/run.sh")
	if !got.Found || !bytes.Equal(got.Blob.Bytes, []byte("#!/bin/bash\n")) || got.Blob.Mode != "100755" {
		t.Fatalf("overlay ReadBlobs bin/run.sh = %+v, want new bytes / mode 100755", got)
	}
}

func TestOverlayDelete(t *testing.T) {
	ov, err := newOverlayTree(newBaseTree(newFixture()), makePlan(
		FileMutation{Path: "docs/b.md", Kind: MutationDelete},
	))
	if err != nil {
		t.Fatalf("newOverlayTree: %v", err)
	}

	entries, err := ov.ListTree(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	if _, ok := findEntry(entries, "docs/b.md"); ok {
		t.Fatalf("deleted path docs/b.md still present in ListTree")
	}

	got := readOne(t, ov, "docs/b.md")
	if got.Found {
		t.Fatalf("overlay ReadBlobs of deleted docs/b.md returned Found=true: %+v", got)
	}
}

func TestOverlayUntouchedServesBaseBytes(t *testing.T) {
	ov, err := newOverlayTree(newBaseTree(newFixture()), makePlan(
		FileMutation{Path: "docs/c.md", Kind: MutationCreate, Bytes: []byte("gamma\n")},
	))
	if err != nil {
		t.Fatalf("newOverlayTree: %v", err)
	}
	got := readOne(t, ov, "docs/a.md")
	if !got.Found || !bytes.Equal(got.Blob.Bytes, []byte("alpha\n")) {
		t.Fatalf("untouched docs/a.md = %+v, want base bytes byte-identical", got)
	}
	if got.Blob.Mode != "100644" || got.Blob.ObjectID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("untouched docs/a.md lost base mode/id: %+v", got)
	}
}

func TestOverlayListTreeHonorsPrefixes(t *testing.T) {
	ov, err := newOverlayTree(newBaseTree(newFixture()), makePlan(
		FileMutation{Path: "docs/c.md", Kind: MutationCreate, Bytes: []byte("gamma\n")},
		FileMutation{Path: "bin/other.sh", Kind: MutationCreate, Bytes: []byte("x\n")},
	))
	if err != nil {
		t.Fatalf("newOverlayTree: %v", err)
	}
	entries, err := ov.ListTree(context.Background(), []gitcli.RepoPath{"docs"})
	if err != nil {
		t.Fatalf("ListTree(docs): %v", err)
	}
	for _, e := range entries {
		if string(e.Path[:4]) != "docs" {
			t.Fatalf("prefix-scoped ListTree leaked non-docs entry %q", e.Path)
		}
	}
	if _, ok := findEntry(entries, "docs/c.md"); !ok {
		t.Fatalf("created docs/c.md missing from prefix-scoped listing")
	}
	if _, ok := findEntry(entries, "bin/other.sh"); ok {
		t.Fatalf("created bin/other.sh leaked into docs-scoped listing")
	}
}

func TestOverlayListTreeSorted(t *testing.T) {
	ov, err := newOverlayTree(newBaseTree(newFixture()), makePlan(
		FileMutation{Path: "docs/aa.md", Kind: MutationCreate, Bytes: []byte("x\n")},
	))
	if err != nil {
		t.Fatalf("newOverlayTree: %v", err)
	}
	entries, err := ov.ListTree(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTree: %v", err)
	}
	for i := 1; i < len(entries); i++ {
		if bytes.Compare([]byte(entries[i-1].Path), []byte(entries[i].Path)) >= 0 {
			t.Fatalf("ListTree not path-sorted at %d: %q then %q", i, entries[i-1].Path, entries[i].Path)
		}
	}
}

func TestOverlayRejections(t *testing.T) {
	cases := []struct {
		name string
		plan MutationPlan
	}{
		{"create-target-exists", makePlan(
			FileMutation{Path: "docs/a.md", Kind: MutationCreate, Bytes: []byte("x\n")})},
		{"replace-target-absent", makePlan(
			FileMutation{Path: "docs/missing.md", Kind: MutationReplace, Bytes: []byte("x\n")})},
		{"delete-target-absent", makePlan(
			FileMutation{Path: "docs/missing.md", Kind: MutationDelete})},
		{"replace-symlink-target", makePlan(
			FileMutation{Path: "link", Kind: MutationReplace, Bytes: []byte("x\n")})},
		{"delete-symlink-target", makePlan(
			FileMutation{Path: "link", Kind: MutationDelete})},
		{"replace-gitlink-target", makePlan(
			FileMutation{Path: "gitlink", Kind: MutationReplace, Bytes: []byte("x\n")})},
		{"delete-gitlink-target", makePlan(
			FileMutation{Path: "gitlink", Kind: MutationDelete})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := newOverlayTree(newBaseTree(newFixture()), tc.plan); err == nil {
				t.Fatalf("newOverlayTree accepted an invalid overlay (%s)", tc.name)
			}
		})
	}
}

func TestOverlayRejectsInvalidPlan(t *testing.T) {
	// A plan whose intrinsic shape is invalid (bad receipt) must be refused at
	// construction — overlay construction validates plan shape.
	bad := MutationPlan{
		Files:         []FileMutation{{Path: "docs/c.md", Kind: MutationCreate, Bytes: []byte("x\n")}},
		CommitSubject: "subject",
		Receipt:       []byte("not json"),
	}
	if _, err := newOverlayTree(newBaseTree(newFixture()), bad); err == nil {
		t.Fatalf("newOverlayTree accepted a plan with an invalid receipt")
	}
}
