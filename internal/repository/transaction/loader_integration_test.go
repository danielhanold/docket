//go:build integration

package transaction

import (
	"context"
	"sort"
	"strings"
	"testing"
)

// TestIntegrationTxnApplyLoaderBuildsCleanStateFromCorpus proves the loader turns the harness corpus
// into a complete, error-free LoadedState reading through a real base tree.
func TestIntegrationTxnApplyLoaderBuildsCleanStateFromCorpus(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	ctx := context.Background()

	rev, err := client.FetchBranch(ctx, repo, "origin", r.Target)
	if err != nil {
		t.Fatalf("FetchBranch: %v", err)
	}
	src, err := client.OpenObjectSource(ctx, repo, rev)
	if err != nil {
		t.Fatalf("OpenObjectSource: %v", err)
	}

	st, err := testLoader{}.Load(ctx, newBaseTree(src))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st.Report.HasErrors() {
		t.Fatalf("corpus produced error findings: %v", st.Report.Findings())
	}
	if n := len(st.Snapshot.Changes()); n != 2 {
		t.Errorf("changes = %d, want 2", n)
	}
	if n := len(st.Snapshot.ADRs()); n != 1 {
		t.Errorf("adrs = %d, want 1", n)
	}

	var gotPaths []string
	for p := range st.Sources {
		gotPaths = append(gotPaths, p)
	}
	sort.Strings(gotPaths)
	want := []string{
		"docs/adrs/0001-first-decision.md",
		"docs/changes/active/0001-first-change.md",
		"docs/changes/active/0002-second-change.md",
	}
	sort.Strings(want)
	if strings.Join(gotPaths, "|") != strings.Join(want, "|") {
		t.Errorf("loaded sources = %v, want %v", gotPaths, want)
	}
}
