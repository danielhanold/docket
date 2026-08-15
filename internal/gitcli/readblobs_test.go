package gitcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// lsTreeBlobEntry is one synthetic ls-tree record for the helper-process fake
// git: a blob at path with the given object id.
type lsTreeBlobEntry struct {
	oid  string
	path string
}

// buildLsTreeZ renders synthetic entries as the NUL-delimited `ls-tree -z`
// stream the resolve stage parses ("<mode> blob <oid>\t<path>\x00").
func buildLsTreeZ(entries []lsTreeBlobEntry) []byte {
	var b bytes.Buffer
	for _, e := range entries {
		b.WriteString("100644 blob " + e.oid + "\t" + e.path)
		b.WriteByte(0)
	}
	return b.Bytes()
}

// catFrame renders one cat-file --batch frame with an explicit size field and
// payload, so a test can deliberately desynchronize the two.
func catFrame(oid, typ string, size int, payload string) string {
	return oid + " " + typ + " " + strconv.Itoa(size) + "\n" + payload + "\n"
}

// scriptBlobSource builds an objectSource backed by a helper-process fake git
// whose ls-tree and cat-file answers come from the given canned payloads, so
// the batch pipeline can be driven with hostile or malformed output no real git
// would emit. extraEnv carries additional helper controls (e.g. a spawn log).
func scriptBlobSource(t *testing.T, lstree, catfile []byte, extraEnv ...string) *objectSource {
	t.Helper()
	dir := t.TempDir()
	lp := filepath.Join(dir, "lstree")
	cp := filepath.Join(dir, "catfile")
	if err := os.WriteFile(lp, lstree, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cp, catfile, 0o644); err != nil {
		t.Fatal(err)
	}
	env := append([]string{
		"GITCLI_HELPER_LSTREE_FILE=" + lp,
		"GITCLI_HELPER_CATFILE_FILE=" + cp,
	}, extraEnv...)
	c := helperClient(t, "script", env...)
	return &objectSource{
		client: c,
		repo:   Repository{PrimaryWorktree: dir},
		rev:    Revision{Commit: ObjectID(strings.Repeat("a", 40)), Remote: "origin", Ref: "refs/heads/main"},
	}
}

// TestReadBlobsMatchesOracleInRequestOrder reads a mix of ordinary, empty,
// executable, symlink, and both hostile-path blobs from the main-mode fixture
// and asserts every result carries the exact plumbing-oracle bytes, mode, and
// id, in the REQUEST order (deliberately not the tree's raw-byte sort order).
func TestReadBlobsMatchesOracleInRequestOrder(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	_, src := openMainSource(t, c, r)

	req := []RepoPath{"tool.sh", "README.md", "empty.txt", "link.md", hostilePathTab, hostilePathNewline}
	oracle := parseLsTreeOracle(t, rawGitOut(t, r.Invocation, "ls-tree", "-r", "-z", "--full-tree", string(src.Revision().Commit)))

	results, err := src.ReadBlobs(ctx, req)
	if err != nil {
		t.Fatalf("ReadBlobs: %v", err)
	}
	if len(results) != len(req) {
		t.Fatalf("ReadBlobs returned %d results, want %d", len(results), len(req))
	}
	for i, p := range req {
		got := results[i]
		if got.Path != p {
			t.Fatalf("result %d path = %q, want %q (results out of request order)", i, got.Path, p)
		}
		if !got.Found {
			t.Fatalf("result %d (%q) not found", i, p)
		}
		want, ok := oracle[string(p)]
		if !ok {
			t.Fatalf("oracle missing path %q", p)
		}
		if string(got.Blob.Mode) != want.mode {
			t.Errorf("%q mode = %q, want %q", p, got.Blob.Mode, want.mode)
		}
		if string(got.Blob.ObjectID) != want.oid {
			t.Errorf("%q oid = %q, want %q", p, got.Blob.ObjectID, want.oid)
		}
		if err := validateObjectID(got.Blob.ObjectID); err != nil {
			t.Errorf("%q oid not valid: %v", p, err)
		}
		wantBytes := rawGitOut(t, r.Invocation, "cat-file", "blob", want.oid)
		if !bytes.Equal(got.Blob.Bytes, wantBytes) {
			t.Errorf("%q bytes = %q, want %q", p, got.Blob.Bytes, wantBytes)
		}
	}
}

// TestReadBlobsMissingDuplicateEmpty proves an absent path yields Found:false at
// its slot while other slots stay intact, a duplicate request path is rejected
// invalid-request, and empty input returns an empty result WITHOUT spawning any
// git process (asserted via an empty spawn log).
func TestReadBlobsMissingDuplicateEmpty(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	_, src := openMainSource(t, c, r)

	results, err := src.ReadBlobs(ctx, []RepoPath{"README.md", "does/not/exist.md"})
	if err != nil {
		t.Fatalf("ReadBlobs with a missing path errored: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if !results[0].Found || results[0].Path != "README.md" {
		t.Errorf("slot 0 = %+v, want found README.md", results[0])
	}
	if results[1].Found || results[1].Path != "does/not/exist.md" {
		t.Errorf("slot 1 = %+v, want not-found does/not/exist.md", results[1])
	}

	_, err = src.ReadBlobs(ctx, []RepoPath{"README.md", "README.md"})
	assertKind(t, err, KindInvalidRequest)

	// Empty input: no process may be spawned. A helper-backed source whose
	// spawn log stays empty proves it.
	spawnlog := filepath.Join(t.TempDir(), "spawns")
	hsrc := scriptBlobSource(t, nil, nil, "GITCLI_HELPER_SPAWNLOG="+spawnlog)
	empty, err := hsrc.ReadBlobs(ctx, nil)
	if err != nil {
		t.Fatalf("ReadBlobs(nil) errored: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("ReadBlobs(nil) = %d results, want 0", len(empty))
	}
	if b, statErr := os.ReadFile(spawnlog); statErr == nil && len(bytes.TrimSpace(b)) != 0 {
		t.Fatalf("empty input spawned a process; spawn log:\n%s", b)
	}
}

// TestReadBlobsSymlinkGitlinkDirectory proves a symlink returns its stored
// target string (never the target file's contents), and that a gitlink path or
// a directory path requested as a blob fails the whole call with
// unexpected-object.
func TestReadBlobsSymlinkGitlinkDirectory(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	_, src := openMainSource(t, c, r)

	results, err := src.ReadBlobs(ctx, []RepoPath{"link.md"})
	if err != nil {
		t.Fatalf("ReadBlobs([link.md]): %v", err)
	}
	if len(results) != 1 || !results[0].Found {
		t.Fatalf("link.md not found: %+v", results)
	}
	if string(results[0].Blob.Mode) != "120000" {
		t.Errorf("link.md mode = %q, want 120000", results[0].Blob.Mode)
	}
	if string(results[0].Blob.Bytes) != "README.md" {
		t.Errorf("link.md bytes = %q, want the target string \"README.md\"", results[0].Blob.Bytes)
	}

	_, err = src.ReadBlobs(ctx, []RepoPath{"sub"})
	assertKind(t, err, KindUnexpectedObject)

	_, err = src.ReadBlobs(ctx, []RepoPath{"docs"})
	assertKind(t, err, KindUnexpectedObject)
}

// TestReadBlobsUsesOneBatchProcess proves a 5-path read spawns exactly two git
// processes — one ls-tree resolve and one cat-file batch — not one per blob,
// counted through the helper's spawn log.
func TestReadBlobsUsesOneBatchProcess(t *testing.T) {
	paths := []RepoPath{"a.md", "b.md", "c.md", "d.md", "e.md"}
	entries := make([]lsTreeBlobEntry, len(paths))
	var catfile bytes.Buffer
	for i, p := range paths {
		oid := strings.Repeat(string(rune('a'+i)), 40)
		content := "content-" + string(p)
		entries[i] = lsTreeBlobEntry{oid: oid, path: string(p)}
		catfile.WriteString(catFrame(oid, "blob", len(content), content))
	}

	spawnlog := filepath.Join(t.TempDir(), "spawns")
	src := scriptBlobSource(t, buildLsTreeZ(entries), catfile.Bytes(), "GITCLI_HELPER_SPAWNLOG="+spawnlog)

	results, err := src.ReadBlobs(context.Background(), paths)
	if err != nil {
		t.Fatalf("ReadBlobs: %v", err)
	}
	if len(results) != len(paths) {
		t.Fatalf("got %d results, want %d", len(results), len(paths))
	}
	for i, p := range paths {
		if !results[i].Found || results[i].Path != p {
			t.Fatalf("result %d = %+v, want found %q", i, results[i], p)
		}
	}
	log, err := os.ReadFile(spawnlog)
	if err != nil {
		t.Fatalf("read spawn log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(log), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("spawned %d git processes, want exactly 2 (ls-tree + cat-file):\n%s", len(lines), log)
	}
	if !strings.Contains(lines[0], "ls-tree") {
		t.Errorf("first spawn is not ls-tree: %q", lines[0])
	}
	if !strings.Contains(lines[1], "cat-file") {
		t.Errorf("second spawn is not cat-file: %q", lines[1])
	}
}

// TestReadBlobsMalformedBatchFrames drives ReadBlobs against valid ls-tree
// resolution but deliberately corrupted cat-file frames — truncated payload,
// wrong oid, wrong type, short and long size, an extra trailing frame, and
// reordered frames — asserting each is reported invalid-output with no partial
// result.
func TestReadBlobsMalformedBatchFrames(t *testing.T) {
	o0 := strings.Repeat("a", 40)
	o1 := strings.Repeat("b", 40)
	other := strings.Repeat("c", 40)
	entries := []lsTreeBlobEntry{{oid: o0, path: "p0"}, {oid: o1, path: "p1"}}
	valid0 := catFrame(o0, "blob", 5, "hello")
	valid1 := catFrame(o1, "blob", 6, "world!")

	cases := []struct {
		name    string
		catfile string
	}{
		{"truncated payload", o0 + " blob 5\nhel"},
		{"wrong oid", catFrame(other, "blob", 5, "hello") + valid1},
		{"wrong type tree", catFrame(o0, "tree", 5, "hello") + valid1},
		{"size too short", catFrame(o0, "blob", 3, "hello") + valid1},
		{"size too long", catFrame(o0, "blob", 99, "hello") + valid1},
		{"extra trailing frame", valid0 + valid1 + catFrame(other, "blob", 1, "x")},
		{"reordered frames", valid1 + valid0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := scriptBlobSource(t, buildLsTreeZ(entries), []byte(tc.catfile))
			results, err := src.ReadBlobs(context.Background(), []RepoPath{"p0", "p1"})
			assertKind(t, err, KindInvalidOutput)
			if results != nil {
				t.Fatalf("want nil results on malformed frames, got %d", len(results))
			}
		})
	}
}

// TestReadBlobsResultOwnership proves each result owns a fresh byte copy that
// shares no backing array with the multi-frame parse buffer. A blob that is not
// the last frame in the cat-file stream would, if returned as a sub-slice of
// that buffer, carry capacity reaching into the following frames; a genuine copy
// has capacity equal to its length. The check is driven through a helper-process
// git so the buffer deterministically holds a trailing frame after the one under
// test, and the returned slice is mutated in place to confirm nothing panics or
// depends on the shared array.
func TestReadBlobsResultOwnership(t *testing.T) {
	o0 := strings.Repeat("a", 40)
	o1 := strings.Repeat("b", 40)
	c0 := "hello"
	entries := []lsTreeBlobEntry{{oid: o0, path: "p0"}, {oid: o1, path: "p1"}}
	catfile := catFrame(o0, "blob", len(c0), c0) + catFrame(o1, "blob", 6, "world!")
	src := scriptBlobSource(t, buildLsTreeZ(entries), []byte(catfile))

	first, err := src.ReadBlobs(context.Background(), []RepoPath{"p0", "p1"})
	if err != nil {
		t.Fatalf("ReadBlobs: %v", err)
	}
	b := first[0].Blob.Bytes
	if string(b) != c0 {
		t.Fatalf("p0 bytes = %q, want %q", b, c0)
	}
	// A fresh copy has cap == len; a sub-slice of the multi-frame buffer would
	// have spare capacity reaching into p1's frame.
	if cap(b) != len(b) {
		t.Fatalf("p0 bytes cap = %d, len = %d: result aliases the parse buffer", cap(b), len(b))
	}
	// The second result's bytes are an independent slice from the same parse:
	// mutating p0 must not touch p1.
	p1before := string(first[1].Blob.Bytes)
	for i := range b {
		b[i] ^= 0xff
	}
	if string(first[1].Blob.Bytes) != p1before {
		t.Fatalf("mutating p0 bytes corrupted p1: %q != %q", first[1].Blob.Bytes, p1before)
	}
}
