//go:build integration

package workspace

import (
	"strings"
	"testing"
)

// TestIntegrationWorkspaceSetupHarnessBuildersProduceExpectedTopology is the harness self-test: it proves
// each builder produces the topology the Prepare tests depend on.
func TestIntegrationWorkspaceSetupHarnessBuildersProduceExpectedTopology(t *testing.T) {
	requireGit(t)

	t.Run("main", func(t *testing.T) {
		r := mainModeRepo(t)
		if got := gitOut(t, r.Origin, "rev-parse", "--is-bare-repository"); got != "true" {
			t.Errorf("origin is-bare-repository = %q, want true", got)
		}
		if got := gitOut(t, r.Primary, "status", "--porcelain"); got != "" {
			t.Errorf("primary status not clean:\n%s", got)
		}
		if n := countWorktrees(gitOut(t, r.Primary, "worktree", "list", "--porcelain")); n != 1 {
			t.Errorf("registered worktrees = %d, want 1", n)
		}
	})

	t.Run("docket", func(t *testing.T) {
		r := docketModeRepo(t)
		if got := gitOut(t, r.Primary, "status", "--porcelain"); got != "" {
			t.Errorf("docket primary status not clean:\n%s", got)
		}
		wl := gitOut(t, r.Primary, "worktree", "list", "--porcelain")
		if n := countWorktrees(wl); n != 4 {
			t.Errorf("registered worktrees = %d, want 4 (primary + .docket + txn + sibling):\n%s", n, wl)
		}
		for _, want := range []string{"refs/heads/docket", "refs/heads/feat/other"} {
			if !strings.Contains(wl, want) {
				t.Errorf("worktree list missing %q:\n%s", want, wl)
			}
		}
		if len(r.Preserve) != 4 {
			t.Errorf("Preserve = %d worktrees, want 4", len(r.Preserve))
		}
	})
}
