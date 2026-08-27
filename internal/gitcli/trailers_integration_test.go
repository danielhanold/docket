//go:build integration

package gitcli

import (
	"context"
	"path/filepath"
	"testing"
)

// TestScanCommitTrailersGrammarNotSubstring proves ScanCommitTrailers returns
// only commits whose message carries a matching key in a genuine trailer block —
// parsed with Git's own trailer grammar, not a substring grep. A commit with no
// trailers, and a commit whose subject and body merely contain trailer-looking
// text outside the trailer block, are both excluded; the one real trailer-bearing
// commit returns its full trailer set.
func TestIntegrationRepoScanCommitTrailersGrammarNotSubstring(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	c := newRealClient(t)
	r := newMainModeRepos(t)
	repo := mustDiscover(t, c, r.Invocation)

	base := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	wt := filepath.Join(t.TempDir(), "hist")
	if err := c.AddDetachedWorktree(ctx, repo, wt, base); err != nil {
		t.Fatalf("AddDetachedWorktree: %v", err)
	}

	// c1: a real trailer block with two Docket trailers (one -m paragraph so both
	// lines form the final trailer paragraph).
	writeWorktreeFile(t, wt, "a.md", "a\n")
	gitOut(t, wt, "add", "a.md")
	gitOut(t, wt, "commit", "-q", "-m", "add a", "-m", "Docket-Transaction-ID: txn-1\nDocket-Operation: change.groom")
	c1 := ObjectID(gitOut(t, wt, "rev-parse", "HEAD"))

	// c2: no trailers at all.
	writeWorktreeFile(t, wt, "b.md", "b\n")
	gitOut(t, wt, "add", "b.md")
	gitOut(t, wt, "commit", "-q", "-m", "add b")

	// c3: hostile — the SUBJECT is trailer-looking and the body carries a
	// trailer-looking line, but the last paragraph is prose so there is no trailer
	// block. Must not match.
	writeWorktreeFile(t, wt, "d.md", "d\n")
	gitOut(t, wt, "add", "d.md")
	gitOut(t, wt, "commit", "-q", "-m", "Docket-Result: x", "-m", "Docket-Operation: sneaky", "-m", "this trailing paragraph is prose")
	c3 := ObjectID(gitOut(t, wt, "rev-parse", "HEAD"))

	keys := []string{"Docket-Transaction-ID", "Docket-Operation", "Docket-Result"}
	got, err := c.ScanCommitTrailers(ctx, repo, c3, keys)
	if err != nil {
		t.Fatalf("ScanCommitTrailers: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ScanCommitTrailers returned %d commits, want exactly 1 (c1):\n%+v", len(got), got)
	}
	if got[0].Commit != c1 {
		t.Errorf("matched commit = %q, want c1 %q", got[0].Commit, c1)
	}
	wantTrailers := []Trailer{
		{Key: "Docket-Transaction-ID", Value: "txn-1"},
		{Key: "Docket-Operation", Value: "change.groom"},
	}
	if len(got[0].Trailers) != len(wantTrailers) {
		t.Fatalf("trailer count = %d, want %d:\n%+v", len(got[0].Trailers), len(wantTrailers), got[0].Trailers)
	}
	for i, want := range wantTrailers {
		if got[0].Trailers[i] != want {
			t.Errorf("Trailers[%d] = %+v, want %+v", i, got[0].Trailers[i], want)
		}
	}
}
