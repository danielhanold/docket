package gitcli

import (
	"reflect"
	"strings"
	"testing"
)

// TestParseAddedLines proves the -U0 patch reader attributes every added line to
// its file and its 1-based line number in head's version, across the patch
// shapes a push can carry: a modified file, a new file, a deletion, a
// "\ No newline at end of file" marker inside a hunk and after one, inter-hunk
// context lines, a C-quoted "+++" header, a "+++" header with Git's trailing
// TAB, and an added line whose own content looks like a header. Every malformed
// shape is an error, never a shorter list: a misread patch would otherwise
// silently un-scan lines.
func TestParseAddedLines(t *testing.T) {
	patch := strings.Join([]string{
		"diff --git a/notes.txt b/notes.txt",
		"index 1111111..2222222 100644",
		"--- a/notes.txt",
		"+++ b/notes.txt",
		"@@ -2 +2 @@",
		"-b",
		"+B",
		"@@ -3,0 +4,2 @@ func context",
		"+c",
		"+++ looks like a header",
		"diff --git a/new.txt b/new.txt",
		"new file mode 100644",
		"index 0000000..3333333",
		"--- /dev/null",
		"+++ b/new.txt",
		"@@ -0,0 +1,2 @@",
		"+one",
		"+two",
		"diff --git a/gone.txt b/gone.txt",
		"deleted file mode 100644",
		"index 4444444..0000000",
		"--- a/gone.txt",
		"+++ /dev/null",
		"@@ -1,2 +0,0 @@",
		"-x",
		"-y",
		"diff --git a/tail.txt b/tail.txt",
		"index 5555555..6666666 100644",
		"--- a/tail.txt",
		"+++ b/tail.txt",
		"@@ -1 +1,2 @@",
		"-old",
		`\ No newline at end of file`,
		"+new",
		"+more",
		`\ No newline at end of file`,
		"diff --git a/ctx.txt b/ctx.txt",
		"index 7777777..8888888 100644",
		"--- a/ctx.txt",
		"+++ b/ctx.txt",
		"@@ -1,3 +1,3 @@",
		"-p",
		"+P",
		" q",
		"-r",
		"+R",
		`diff --git "a/we\"ird.txt" "b/we\"ird.txt"`,
		"new file mode 100644",
		"index 0000000..9999999",
		"--- /dev/null",
		`+++ "b/we\"ird.txt"`,
		"@@ -0,0 +1 @@",
		"+q",
		"diff --git a/has space.txt b/has space.txt",
		"new file mode 100644",
		"index 0000000..aaaaaaa",
		"--- /dev/null",
		"+++ b/has space.txt\t",
		"@@ -0,0 +1 @@",
		"+s",
		"",
	}, "\n")

	got, err := parseAddedLines([]byte(patch))
	if err != nil {
		t.Fatalf("parseAddedLines: %v", err)
	}
	want := []OutgoingLine{
		{Path: "notes.txt", Line: 2, Text: "B"},
		{Path: "notes.txt", Line: 4, Text: "c"},
		{Path: "notes.txt", Line: 5, Text: "++ looks like a header"},
		{Path: "new.txt", Line: 1, Text: "one"},
		{Path: "new.txt", Line: 2, Text: "two"},
		{Path: "tail.txt", Line: 1, Text: "new"},
		{Path: "tail.txt", Line: 2, Text: "more"},
		{Path: "ctx.txt", Line: 1, Text: "P"},
		{Path: "ctx.txt", Line: 3, Text: "R"},
		{Path: `we"ird.txt`, Line: 1, Text: "q"},
		{Path: "has space.txt", Line: 1, Text: "s"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("added lines mismatch\n got: %+v\nwant: %+v", got, want)
	}

	empty, err := parseAddedLines(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty patch = %+v, %v; want no lines, no error", empty, err)
	}

	head := "diff --git a/x b/x\n--- a/x\n+++ b/x\n"
	bad := []struct {
		name  string
		patch string
	}{
		{"truncated hunk", head + "@@ -0,0 +1,2 @@\n+a\n"},
		{"overlong old side", head + "@@ -1 +1,2 @@\n-a\n-b\n+c\n+d\n"},
		{"overlong new side", head + "@@ -1,2 +1 @@\n+a\n+b\n-c\n-d\n"},
		{"added line after its hunk", head + "@@ -0,0 +1 @@\n+a\n+b\n"},
		{"removed line after its hunk", head + "@@ -1 +0,0 @@\n-a\n-b\n"},
		{"context line after its hunk", head + "@@ -0,0 +1 @@\n+a\n b\n"},
		{"unexpected line inside a hunk", head + "@@ -0,0 +1,2 @@\n+a\nb\n"},
		{"second +++ header in one file", head + "@@ -0,0 +1 @@\n+a\n+++ b/y\n"},
		{"+++ header before any diff --git", "+++ b/x\n@@ -0,0 +1 @@\n+a\n"},
		{"hunk before its +++ header", "diff --git a/x b/x\n@@ -0,0 +1 @@\n+a\n"},
		{"added line in a deleted file", "diff --git a/x b/x\n--- a/x\n+++ /dev/null\n@@ -1 +1 @@\n-a\n+b\n"},
		{"header path lacks b/", "diff --git a/x b/x\n--- a/x\n+++ x\n@@ -0,0 +1 @@\n+a\n"},
		{"unterminated quoted path", "diff --git a/x b/x\n--- a/x\n+++ \"b/x\n@@ -0,0 +1 @@\n+a\n"},
		{"malformed hunk header", head + "@@ -0,0 @@\n+a\n"},
		{"non-numeric hunk count", head + "@@ -0,0 +1,z @@\n+a\n"},
		{"signed hunk start", head + "@@ -0,0 +-1 @@\n+a\n"},
		{"hunk header without closing marker", head + "@@ -0,0 +1\n+a\n"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := parseAddedLines([]byte(tc.patch))
			if err == nil {
				t.Fatalf("parseAddedLines accepted a malformed patch: %+v", lines)
			}
			if lines != nil {
				t.Fatalf("malformed patch returned a partial list: %+v", lines)
			}
		})
	}
}

// TestParseOutgoingLog proves the NUL-delimited "<hash>\x01<raw message>" log
// records parse into commits carrying each message verbatim, and that a record
// without its separator, or with a malformed hash, is an error rather than a
// skipped commit.
func TestParseOutgoingLog(t *testing.T) {
	const a = "1111111111111111111111111111111111111111"
	const b = "2222222222222222222222222222222222222222"
	out := a + "\x01Add notes\n\nBody (0007)\n\x00" + b + "\x01Edit notes\n\x00"

	got, err := parseOutgoingLog([]byte(out))
	if err != nil {
		t.Fatalf("parseOutgoingLog: %v", err)
	}
	want := []OutgoingCommit{
		{Commit: a, Message: "Add notes\n\nBody (0007)\n"},
		{Commit: b, Message: "Edit notes\n"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commits mismatch\n got: %+v\nwant: %+v", got, want)
	}

	if none, err := parseOutgoingLog(nil); err != nil || len(none) != 0 {
		t.Fatalf("empty log = %+v, %v; want no commits, no error", none, err)
	}

	for name, bad := range map[string]string{
		"record without separator": a + "\x01ok\n\x00" + b + "no separator\n\x00",
		"malformed hash":           "abc\x01msg\n\x00",
	} {
		t.Run(name, func(t *testing.T) {
			commits, err := parseOutgoingLog([]byte(bad))
			if err == nil {
				t.Fatalf("parseOutgoingLog accepted %q: %+v", bad, commits)
			}
		})
	}
}

// TestParseAddedPaths proves the `--name-status -z --diff-filter=A` reader keeps
// each path verbatim (spaces, quotes, non-ASCII, a newline) and that an odd
// token count or a status other than A is an error.
func TestParseAddedPaths(t *testing.T) {
	out := "A\x00has space.txt\x00A\x00we\"ird.txt\x00A\x00café.txt\x00A\x00new\nline.txt\x00"
	got, err := parseAddedPaths([]byte(out))
	if err != nil {
		t.Fatalf("parseAddedPaths: %v", err)
	}
	want := []string{"has space.txt", `we"ird.txt`, "café.txt", "new\nline.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %q, want %q", got, want)
	}
	if none, err := parseAddedPaths(nil); err != nil || len(none) != 0 {
		t.Fatalf("empty output = %q, %v; want no paths, no error", none, err)
	}
	for name, bad := range map[string]string{
		"odd token count": "A\x00x.txt\x00A\x00",
		"non-A status":    "M\x00x.txt\x00",
		"empty path":      "A\x00\x00",
	} {
		t.Run(name, func(t *testing.T) {
			if paths, err := parseAddedPaths([]byte(bad)); err == nil {
				t.Fatalf("parseAddedPaths accepted %q: %q", bad, paths)
			}
		})
	}
}

// TestParseObjectIDLines proves the rev-list reader keeps every id in order,
// reads empty output as no ids, and refuses an empty line or a malformed id
// rather than skipping a commit.
func TestParseObjectIDLines(t *testing.T) {
	const a = "1111111111111111111111111111111111111111"
	const b = "2222222222222222222222222222222222222222"
	got, err := parseObjectIDLines([]byte(a + "\n" + b + "\n"))
	if err != nil {
		t.Fatalf("parseObjectIDLines: %v", err)
	}
	if want := []ObjectID{a, b}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %q, want %q", got, want)
	}
	if none, err := parseObjectIDLines(nil); err != nil || len(none) != 0 {
		t.Fatalf("empty output = %q, %v; want no ids, no error", none, err)
	}
	for name, bad := range map[string]string{
		"blank line":   a + "\n\n" + b + "\n",
		"lone newline": "\n",
		"malformed id": a + "\nabc\n",
	} {
		t.Run(name, func(t *testing.T) {
			if ids, err := parseObjectIDLines([]byte(bad)); err == nil {
				t.Fatalf("parseObjectIDLines accepted %q: %q", bad, ids)
			}
		})
	}
}

// TestMergeAdded proves every per-commit path and line is kept with its commit,
// in commit order, and a merge-base entry is kept only when no commit already
// adds the same path (and, for a line, the same text) — so a merge's own
// content is still scanned while the branch's net content is not reported twice.
func TestMergeAdded(t *testing.T) {
	const a, b = ObjectID("1111111111111111111111111111111111111111"), ObjectID("2222222222222222222222222222222222222222")
	perCommit := []Outgoing{
		{
			AddedPaths: []OutgoingPath{{Path: "notes.txt", Commit: a}},
			AddedLines: []OutgoingLine{{Path: "notes.txt", Line: 1, Text: "see .docket/x", Commit: a}, {Path: "notes.txt", Line: 2, Text: "keep", Commit: a}},
		},
		{AddedLines: []OutgoingLine{{Path: "notes.txt", Line: 1, Text: "later", Commit: b}}},
	}
	net := Outgoing{
		AddedPaths: []OutgoingPath{{Path: "notes.txt"}, {Path: "merged.txt"}},
		AddedLines: []OutgoingLine{
			{Path: "notes.txt", Line: 1, Text: "later"},
			{Path: "notes.txt", Line: 2, Text: "keep"},
			{Path: "merged.txt", Line: 1, Text: "from the merge"},
			{Path: "other.txt", Line: 4, Text: "keep"},
		},
	}
	paths, lines := mergeAdded(perCommit, net)
	wantPaths := []OutgoingPath{{Path: "notes.txt", Commit: a}, {Path: "merged.txt"}}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Errorf("paths = %+v, want %+v", paths, wantPaths)
	}
	wantLines := []OutgoingLine{
		{Path: "notes.txt", Line: 1, Text: "see .docket/x", Commit: a},
		{Path: "notes.txt", Line: 2, Text: "keep", Commit: a},
		{Path: "notes.txt", Line: 1, Text: "later", Commit: b},
		{Path: "merged.txt", Line: 1, Text: "from the merge"},
		{Path: "other.txt", Line: 4, Text: "keep"},
	}
	if !reflect.DeepEqual(lines, wantLines) {
		t.Errorf("lines =\n  %+v\nwant\n  %+v", lines, wantLines)
	}
}
