//go:build integration

package gitcli

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

// TestIntegrationRepoReadOutgoing proves ReadOutgoing returns exactly what a
// push of a feature branch exposes against real git: the branch's own commits
// (the merge included, the merged-in upstream commit excluded), the paths it
// adds (a space, non-ASCII, and a '"' kept verbatim), and its added lines from
// the merge-base diff — so upstream content merged into the branch is never
// attributed to it, even once the upstream base has moved on past the merge.
// A file Git deems binary still has its lines read, and diff configuration in
// the clone (no prefixes, forced color, inter-hunk context, copy detection)
// changes nothing. An unrelated base (no merge base) is an error, never an empty result.
func TestIntegrationRepoReadOutgoing(t *testing.T) {
	ctx := context.Background()
	r := newMainModeRepos(t)
	c := newRealClient(t)
	repo := mustDiscover(t, c, r.Invocation)
	inv := r.Invocation

	// Configuration a developer may carry must not change what is read: no
	// prefixes, forced color, wide inter-hunk context, rename/copy detection.
	for k, v := range map[string]string{
		"diff.noprefix":         "true",
		"color.ui":              "always",
		"diff.interHunkContext": "5",
		"diff.renames":          "copies",
	} {
		gitOut(t, inv, "config", k, v)
	}

	gitOut(t, inv, "checkout", "-q", "-b", "feat")
	odd := map[string]string{
		"has space.txt": "space line",
		"café.txt":      "accent line",
		`we"ird.txt`:    "quote line",
	}
	writeWorktreeFile(t, inv, "notes.txt", "a\nb\n")
	// A NUL byte makes Git treat the file as binary; its lines are still pushed,
	// so they must still be read.
	writeWorktreeFile(t, inv, "blob.bin", "\x00bin\nhidden line\n")
	for p, line := range odd {
		writeWorktreeFile(t, inv, p, line+"\n")
	}
	gitOut(t, inv, "add", "--", "notes.txt", "blob.bin", "has space.txt", "café.txt", `we"ird.txt`)
	gitOut(t, inv, "commit", "-q", "-m", "Add notes", "-m", "Body (0007)")
	commitA := ObjectID(gitOut(t, inv, "rev-parse", "HEAD"))
	writeWorktreeFile(t, inv, "notes.txt", "a\nB\nc\n")
	gitOut(t, inv, "commit", "-q", "-am", "Edit notes")
	commitB := ObjectID(gitOut(t, inv, "rev-parse", "HEAD"))

	upstream := r.writerCommit(t, "main", map[string]string{"upstream.txt": "docket\n"})
	gitOut(t, inv, "fetch", "-q", "origin")
	gitOut(t, inv, "merge", "-q", "--no-edit", "origin/main")
	head := ObjectID(gitOut(t, inv, "rev-parse", "HEAD"))
	merge := head
	base := ObjectID(gitOut(t, inv, "rev-parse", "origin/main"))
	if base != upstream {
		t.Fatalf("fetched origin/main = %s, want the upstream commit %s", base, upstream)
	}

	check := func(t *testing.T, base ObjectID) {
		t.Helper()
		got, err := c.ReadOutgoing(ctx, repo, base, head)
		if err != nil {
			t.Fatalf("ReadOutgoing: %v", err)
		}

		commits := map[ObjectID]string{}
		for _, oc := range got.Commits {
			commits[oc.Commit] = oc.Message
		}
		if len(got.Commits) != 3 || len(commits) != 3 {
			t.Fatalf("commits = %+v, want exactly (a), (b), and the merge", got.Commits)
		}
		for _, want := range []ObjectID{commitA, commitB, merge} {
			if _, ok := commits[want]; !ok {
				t.Errorf("commit %s missing from %+v", want, got.Commits)
			}
		}
		if _, ok := commits[upstream]; ok {
			t.Errorf("merged-in upstream commit %s reported as outgoing", upstream)
		}
		if msg := commits[commitA]; msg != "Add notes\n\nBody (0007)\n" {
			t.Errorf("commit (a) message = %q, want the raw body", msg)
		}

		paths := append([]string(nil), got.AddedPaths...)
		sort.Strings(paths)
		wantPaths := []string{"blob.bin", "café.txt", "has space.txt", "notes.txt", `we"ird.txt`}
		sort.Strings(wantPaths)
		if !reflect.DeepEqual(paths, wantPaths) {
			t.Errorf("added paths = %q, want %q", paths, wantPaths)
		}

		have := map[OutgoingLine]bool{}
		for _, l := range got.AddedLines {
			have[l] = true
			if l.Path == "upstream.txt" || l.Text == "docket" {
				t.Errorf("upstream line attributed to the branch: %+v", l)
			}
		}
		want := []OutgoingLine{
			{Path: "notes.txt", Line: 2, Text: "B"},
			{Path: "notes.txt", Line: 3, Text: "c"},
			{Path: "blob.bin", Line: 2, Text: "hidden line"},
		}
		for p, line := range odd {
			want = append(want, OutgoingLine{Path: p, Line: 1, Text: line})
		}
		for _, w := range want {
			if !have[w] {
				t.Errorf("added lines missing %+v; got %+v", w, got.AddedLines)
			}
		}
	}

	t.Run("base is the merged upstream tip", func(t *testing.T) { check(t, base) })

	// Upstream moves on past the merge and rewrites the line the branch merged
	// in. Against this newer base the branch still exposes only its own work: a
	// plain two-tree diff would report the branch's stale "docket" line as added.
	r.writerCommit(t, "main", map[string]string{"upstream.txt": "rewritten\n"})
	gitOut(t, inv, "fetch", "-q", "origin")
	later := ObjectID(gitOut(t, inv, "rev-parse", "origin/main"))
	t.Run("base moved past the merge", func(t *testing.T) { check(t, later) })

	t.Run("unrelated base is an error", func(t *testing.T) {
		orphan := ObjectID(gitOut(t, inv, "commit-tree", "HEAD^{tree}", "-m", "orphan"))
		got, err := c.ReadOutgoing(ctx, repo, orphan, head)
		if err == nil {
			t.Fatalf("ReadOutgoing with no merge base succeeded: %+v", got)
		}
		assertKind(t, err, KindCommandFailed)
		if !reflect.DeepEqual(got, Outgoing{}) {
			t.Fatalf("failed read returned a partial result: %+v", got)
		}
	})

	t.Run("malformed id is an invalid request", func(t *testing.T) {
		_, err := c.ReadOutgoing(ctx, repo, "main", head)
		assertKind(t, err, KindInvalidRequest)
		_, err = c.ReadOutgoing(ctx, repo, base, "HEAD")
		assertKind(t, err, KindInvalidRequest)
	})
}
