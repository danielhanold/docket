//go:build integration

package transaction

import (
	"strings"
	"testing"
)

// TestIntegrationTxnApplyHarnessBuildersProduceExpectedTopology proves each builder yields the
// topology the engine tests depend on: a bare origin, a clean invocation checkout,
// the corpus on the target branch, and (docket mode) a linked docket worktree.
func TestIntegrationTxnApplyHarnessBuildersProduceExpectedTopology(t *testing.T) {
	requireGit(t)

	t.Run("main", func(t *testing.T) {
		r := newMainModeRepos(t)
		if got := hgitOut(t, r.Origin, "rev-parse", "--is-bare-repository"); got != "true" {
			t.Errorf("origin is-bare = %q, want true", got)
		}
		if got := hgitOut(t, r.Invocation, "status", "--porcelain"); got != "" {
			t.Errorf("invocation status not clean:\n%s", got)
		}
		names := hgitOut(t, r.Origin, "ls-tree", "-r", "--name-only", "main")
		for _, want := range []string{"docs/adrs/0001-first-decision.md", "docs/changes/active/0001-first-change.md"} {
			if !strings.Contains(names, want) {
				t.Errorf("main tree missing corpus record %q", want)
			}
		}
	})

	t.Run("docket", func(t *testing.T) {
		r := newDocketModeRepos(t)
		if got := hgitOut(t, r.Invocation, "status", "--porcelain"); got != "" {
			t.Errorf("docket-mode invocation status not clean:\n%s", got)
		}
		docketNames := hgitOut(t, r.Invocation, "ls-tree", "-r", "--name-only", "origin/docket")
		if !strings.Contains(docketNames, "docs/changes/active/0001-first-change.md") {
			t.Errorf("docket branch missing corpus:\n%s", docketNames)
		}
		mainNames := hgitOut(t, r.Invocation, "ls-tree", "-r", "--name-only", "main")
		if strings.Contains(mainNames, "0001-first-change.md") {
			t.Errorf("main tree unexpectedly carries docket corpus:\n%s", mainNames)
		}
	})
}
