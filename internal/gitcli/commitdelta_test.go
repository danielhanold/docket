package gitcli

import (
	"context"
	"strings"
	"testing"
)

// TestCommitChangedPathsSingleAndMultiArtifact proves CommitChangedPaths reports
// exactly the paths a commit changes against its first parent, with rename
// detection OFF: a single-file commit yields one path, a two-file commit yields
// two, and a rename yields the source delete plus the destination add (never one
// collapsed rename record). It is the single-artifact-delta oracle the plan and
// results attachment guards depend on.
func TestCommitChangedPathsSingleAndMultiArtifact(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	ctx := context.Background()
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Writer)

	assertDelta := func(t *testing.T, commit ObjectID, want []string) {
		t.Helper()
		got, err := c.CommitChangedPaths(ctx, repo, commit)
		if err != nil {
			t.Fatalf("CommitChangedPaths: %v", err)
		}
		var gotS []string
		for _, p := range got {
			gotS = append(gotS, string(p))
		}
		if strings.Join(gotS, ",") != strings.Join(want, ",") {
			t.Errorf("delta = %v, want %v", gotS, want)
		}
	}

	// A single new artifact under a plans-shaped directory: exactly one path.
	single := r.writerCommit(t, "main", map[string]string{
		"docs/superpowers/plans/2026-08-17-a-plan.md": "# Plan\n",
	})
	assertDelta(t, single, []string{"docs/superpowers/plans/2026-08-17-a-plan.md"})

	// A two-file commit: both paths surface (the multi-artifact-delta guard).
	multi := r.writerCommit(t, "main", map[string]string{
		"docs/superpowers/plans/2026-08-17-b-plan.md": "# Plan B\n",
		"src/extra.go": "package src\n",
	})
	assertDelta(t, multi, []string{"docs/superpowers/plans/2026-08-17-b-plan.md", "src/extra.go"})

	// A rename with detection off surfaces as a delete + an add — two paths — so a
	// single-artifact predicate reddens (learning diff-derived-allowlist-needs-no-renames).
	gitOut(t, r.Writer, "mv", "docs/superpowers/plans/2026-08-17-a-plan.md", "docs/superpowers/plans/2026-08-17-renamed.md")
	gitOut(t, r.Writer, "commit", "-q", "-m", "rename the plan")
	renamed := ObjectID(gitOut(t, r.Writer, "rev-parse", "HEAD"))
	assertDelta(t, renamed, []string{
		"docs/superpowers/plans/2026-08-17-a-plan.md",
		"docs/superpowers/plans/2026-08-17-renamed.md",
	})
}

// TestCommitChangedPathsRejectsMalformedID proves an invalid commit id is a typed
// invalid-request Failure, before any process runs.
func TestCommitChangedPathsRejectsMalformedID(t *testing.T) {
	requireGit(t)
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Writer)

	_, err := c.CommitChangedPaths(context.Background(), repo, ObjectID("not-a-hash"))
	f, ok := AsFailure(err)
	if !ok || f.Kind != KindInvalidRequest {
		t.Fatalf("err = %v, want an invalid-request Failure", err)
	}
}
