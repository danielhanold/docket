//go:build integration

package app

// Change 0465: real-git tests moved out of the default internal/app corpus, which
// must never start real git (see nogit_guard_test.go); run by
// tests/test_go_integration_app_recordops.sh (prefix ^TestIntegrationRecordOps).

import (
	"context"
	"strings"
	"testing"
)

// TestIntegrationRecordOpsChangeCreateNormalizedPrefixReplaysRealGit drives the real engine: a
// create with a messy prefix stores the normalized value, the same request_id
// retyped as the normalized spelling replays, and a differing auto_groomable
// under that id does not apply.
func TestIntegrationRecordOpsChangeCreateNormalizedPrefixReplaysRealGit(t *testing.T) {
	requireRealGit(t)
	repo := newWorkingRepo(t, map[string]string{
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	})
	node := planningDepsFor(t, repo.invocation)
	yes, no := true, false

	req := validChangeCreateRequest()
	req.BranchPrefix, req.AutoGroomable = "Hotfix/", &yes
	first := ChangeCreate(context.Background(), node.deps, node.dir, req)
	if first.Result != ResultApplied || first.Replayed {
		t.Fatalf("first create = %q replayed=%v (findings %v), want a fresh apply", first.Result, first.Replayed, first.Findings)
	}
	body, ok := originFile(t, repo.origin, "docket", first.Path)
	if !ok || !strings.Contains(body, "\nbranch_prefix: 'hotfix'\n") || !strings.Contains(body, "\nauto_groomable: true\n") {
		t.Fatalf("created record lacks the normalized scalars:\n%s", body)
	}
	tip := originTip(t, repo.origin, "docket")

	req.BranchPrefix = "hotfix"
	second := ChangeCreate(context.Background(), node.deps, node.dir, req)
	if second.Result != ResultApplied || !second.Replayed || second.ID != first.ID {
		t.Fatalf("retyped create = %q replayed=%v id=%d (findings %v), want a replay of %d", second.Result, second.Replayed, second.ID, second.Findings, first.ID)
	}

	req.AutoGroomable = &no
	third := ChangeCreate(context.Background(), node.deps, node.dir, req)
	if third.Result == ResultApplied {
		t.Fatalf("a differing auto_groomable under the same request_id applied (replayed=%v)", third.Replayed)
	}
	if got := originTip(t, repo.origin, "docket"); got != tip {
		t.Errorf("replay/conflict moved the metadata branch %s -> %s", tip, got)
	}
}
