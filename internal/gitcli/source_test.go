package gitcli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rawGitOut runs real git directly and returns UNtrimmed stdout bytes — the
// exact-byte oracle for NUL-delimited plumbing output, where gitOut's TrimSpace
// would corrupt embedded whitespace, tabs, and newlines in hostile paths.
func rawGitOut(t *testing.T, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout bytes.Buffer
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git -C %s %s: %v: %s", dir, strings.Join(args, " "), err, stderr.String())
	}
	return stdout.Bytes()
}

// oracleEntry is one independently-parsed `ls-tree -r -z` record: the fields as
// git emitted them, keyed by raw path.
type oracleEntry struct {
	mode string
	typ  string
	oid  string
}

// parseLsTreeOracle parses `git ls-tree -r -z --full-tree <commit>` output with
// code distinct from the adapter's own parser, so an equality test is a genuine
// cross-check rather than a tautology.
func parseLsTreeOracle(t *testing.T, raw []byte) map[string]oracleEntry {
	t.Helper()
	m := map[string]oracleEntry{}
	if len(raw) == 0 {
		return m
	}
	recs := bytes.Split(raw, []byte{0})
	if len(recs[len(recs)-1]) != 0 {
		t.Fatalf("oracle output not NUL-terminated")
	}
	for _, rec := range recs[:len(recs)-1] {
		tab := bytes.IndexByte(rec, '\t')
		if tab < 0 {
			t.Fatalf("oracle record missing tab: %q", rec)
		}
		header := string(rec[:tab])
		path := string(rec[tab+1:])
		f := strings.Split(header, " ")
		if len(f) != 3 {
			t.Fatalf("oracle header not three fields: %q", header)
		}
		m[path] = oracleEntry{mode: f[0], typ: f[1], oid: f[2]}
	}
	return m
}

// openMainSource discovers the invocation repo, resolves refs/heads/main, and
// opens a source pinned at that commit.
func openMainSource(t *testing.T, c *Client, r *testRepos) (Repository, ObjectSource) {
	t.Helper()
	ctx := context.Background()
	repo := mustDiscover(t, c, r.Invocation)
	id, err := c.ResolveRef(ctx, repo, "refs/heads/main")
	if err != nil {
		t.Fatalf("ResolveRef(refs/heads/main): %v", err)
	}
	src, err := c.OpenObjectSource(ctx, repo, Revision{Commit: id, Remote: "origin", Ref: "refs/heads/main"})
	if err != nil {
		t.Fatalf("OpenObjectSource: %v", err)
	}
	return repo, src
}

// hasPath reports whether any entry carries the exact raw path p.
func hasPath(entries []TreeEntry, p RepoPath) bool {
	for _, e := range entries {
		if e.Path == p {
			return true
		}
	}
	return false
}

// sameEntries fails unless a and b are element-wise identical (TreeEntry is
// comparable; order is significant since ListTree sorts deterministically).
func sameEntries(t *testing.T, a, b []TreeEntry) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("entry count %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("entry %d differs: %+v != %+v", i, a[i], b[i])
		}
	}
}

// TestListTreeMatchesPlumbingOracle proves ListTree(nil) over the main-mode
// fixture equals an independently-parsed `ls-tree -r -z` listing — every raw
// path (including the tab/space/non-ASCII and embedded-newline hostile names),
// mode, type, and object id.
func TestListTreeMatchesPlumbingOracle(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	_, src := openMainSource(t, c, r)

	entries, err := src.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("ListTree(nil): %v", err)
	}
	oracle := parseLsTreeOracle(t, rawGitOut(t, r.Invocation, "ls-tree", "-r", "-z", "--full-tree", string(src.Revision().Commit)))

	if len(entries) != len(oracle) {
		t.Fatalf("ListTree returned %d entries, oracle has %d", len(entries), len(oracle))
	}
	for _, e := range entries {
		want, ok := oracle[string(e.Path)]
		if !ok {
			t.Errorf("ListTree produced path %q absent from oracle", e.Path)
			continue
		}
		if string(e.Mode) != want.mode || string(e.Type) != want.typ || string(e.ObjectID) != want.oid {
			t.Errorf("entry %q = {%s %s %s}, oracle {%s %s %s}", e.Path, e.Mode, e.Type, e.ObjectID, want.mode, want.typ, want.oid)
		}
	}
	// The distinctive shapes and hostile bytes must be present verbatim.
	for _, p := range []RepoPath{hostilePathTab, hostilePathNewline, "link.md", "tool.sh", "sub"} {
		if !hasPath(entries, p) {
			t.Errorf("listing missing expected path %q", p)
		}
	}
}

// TestListTreePrefixesOverlapAndAbsent verifies prefix scoping, dedup across
// overlapping prefixes, an absent prefix contributing zero entries with a nil
// error, and raw-byte-sorted output.
func TestListTreePrefixesOverlapAndAbsent(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	_, src := openMainSource(t, c, r)

	only, err := src.ListTree(ctx, []RepoPath{"docs"})
	if err != nil {
		t.Fatalf("ListTree([docs]): %v", err)
	}
	if len(only) != 1 || only[0].Path != "docs/changes/active/0001-a.md" {
		t.Fatalf("ListTree([docs]) = %+v, want the single docs leaf", only)
	}

	overlap, err := src.ListTree(ctx, []RepoPath{"docs", "docs/changes"})
	if err != nil {
		t.Fatalf("ListTree([docs docs/changes]): %v", err)
	}
	sameEntries(t, only, overlap)

	absent, err := src.ListTree(ctx, []RepoPath{"nope"})
	if err != nil {
		t.Fatalf("ListTree([nope]) error: %v", err)
	}
	if len(absent) != 0 {
		t.Fatalf("absent prefix returned %d entries, want 0", len(absent))
	}

	full, err := src.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("ListTree(nil): %v", err)
	}
	for i := 1; i < len(full); i++ {
		if bytes.Compare([]byte(full[i-1].Path), []byte(full[i].Path)) >= 0 {
			t.Fatalf("listing not raw-byte sorted at %d: %q >= %q", i, full[i-1].Path, full[i].Path)
		}
	}
}

// TestListTreeQuotePathTrueFixture proves the NUL-delimited read defeats
// core.quotePath=true: hostile paths return as raw bytes, never C-quoted
// spellings (no backslash escapes, no octal "\303" for the non-ASCII rune).
func TestListTreeQuotePathTrueFixture(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	// Fixture sets core.quotePath=true; a display-form read would C-quote "né".
	if got := gitOut(t, r.Invocation, "config", "core.quotePath"); got != "true" {
		t.Fatalf("fixture core.quotePath = %q, want true", got)
	}
	_, src := openMainSource(t, c, r)

	entries, err := src.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("ListTree(nil): %v", err)
	}
	if !hasPath(entries, hostilePathTab) {
		t.Errorf("hostile tab/space/non-ASCII path not returned as raw bytes")
	}
	if !hasPath(entries, hostilePathNewline) {
		t.Errorf("hostile embedded-newline path not returned as raw bytes")
	}
	for _, e := range entries {
		if strings.Contains(string(e.Path), "\\") {
			t.Errorf("path %q contains a backslash — display-form quoting leaked", e.Path)
		}
	}
}

// TestListTreePinnedAfterFetch opens a source at commit A, advances origin/main
// to B in the writer, fetches B locally, and proves the source still lists A's
// tree — not B's (the ListTree-local slice of the Task 8 proof matrix).
func TestListTreePinnedAfterFetch(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	revA, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/main")
	if err != nil {
		t.Fatalf("FetchBranch A: %v", err)
	}
	src, err := c.OpenObjectSource(ctx, repo, revA)
	if err != nil {
		t.Fatalf("OpenObjectSource A: %v", err)
	}
	before, err := src.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("ListTree before advance: %v", err)
	}

	r.writerCommit(t, "main", map[string]string{"added-after.md": "later\n"})
	revB, err := c.FetchBranch(ctx, repo, "origin", "refs/heads/main")
	if err != nil {
		t.Fatalf("FetchBranch B: %v", err)
	}
	if revB.Commit == revA.Commit {
		t.Fatal("writer did not advance origin/main")
	}

	after, err := src.ListTree(ctx, nil)
	if err != nil {
		t.Fatalf("ListTree after advance: %v", err)
	}
	sameEntries(t, before, after)
	if hasPath(after, "added-after.md") {
		t.Fatal("pinned source leaked B's added-after.md")
	}
}

// TestOpenObjectSourceRejections proves OpenObjectSource rejects a non-commit
// object (a blob id) as unexpected-object and a well-formed but absent id as
// ref-unavailable.
func TestOpenObjectSourceRejections(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	blobID := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD:README.md"))
	_, err := c.OpenObjectSource(ctx, repo, Revision{Commit: blobID, Remote: "origin", Ref: "refs/heads/main"})
	assertKind(t, err, KindUnexpectedObject)

	unknown := ObjectID(strings.Repeat("0", 40))
	_, err = c.OpenObjectSource(ctx, repo, Revision{Commit: unknown, Remote: "origin", Ref: "refs/heads/main"})
	assertKind(t, err, KindRefUnavailable)

	_, err = c.OpenObjectSource(ctx, repo, Revision{Commit: "not-hex", Remote: "origin", Ref: "refs/heads/main"})
	assertKind(t, err, KindInvalidRequest)
}

// TestListTreeMalformedOutput drives ListTree against a helper-process git whose
// ls-tree output is deliberately malformed — a truncated (unterminated) record,
// a record with no tab, a bad mode, and a short object id — asserting each is
// reported as invalid-output with no partial result. The payload is delivered
// via a file (GITCLI_HELPER_STDOUT_FILE) so it can carry NUL delimiters an
// environment variable cannot.
func TestListTreeMalformedOutput(t *testing.T) {
	validOID := strings.Repeat("a", 40)
	cases := []struct {
		name    string
		payload []byte
	}{
		{"truncated unterminated record", []byte("100644 blob " + validOID + "\tREADME.md")},
		{"missing tab", []byte("100644 blob " + validOID + " README.md\x00")},
		{"bad mode", []byte("999999 blob " + validOID + "\tREADME.md\x00")},
		{"short oid", []byte("100644 blob abc\tREADME.md\x00")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			payloadFile := filepath.Join(dir, "payload")
			if err := os.WriteFile(payloadFile, tc.payload, 0o644); err != nil {
				t.Fatal(err)
			}
			c := helperClient(t, "script", "GITCLI_HELPER_STDOUT_FILE="+payloadFile)
			src := &objectSource{
				client: c,
				repo:   Repository{PrimaryWorktree: dir},
				rev:    Revision{Commit: ObjectID(validOID), Remote: "origin", Ref: "refs/heads/main"},
			}
			entries, err := src.ListTree(context.Background(), nil)
			assertKind(t, err, KindInvalidOutput)
			if entries != nil {
				t.Fatalf("want nil entries on malformed output, got %d", len(entries))
			}
		})
	}
}
