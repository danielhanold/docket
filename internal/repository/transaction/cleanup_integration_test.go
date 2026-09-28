//go:build integration

package transaction

import (
	"context"
	"os"
	"sort"
	"testing"
)

// TestIntegrationTxnRecoveryPruneReportEmptyOnCleanRoot proves a sweep of a repository with no candidates
// returns an empty, non-error report — the transactions root need not pre-exist.
func TestIntegrationTxnRecoveryPruneReportEmptyOnCleanRoot(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, _, repo := recoveryEngine(t, r)

	rep, err := eng.PruneAbandoned(context.Background(), repo)
	if err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	if len(rep.Entries) != 0 {
		t.Errorf("entries = %+v, want empty", rep.Entries)
	}
}

// TestIntegrationTxnRecoveryPruneReportDeterministicOrder proves the report lists every candidate exactly
// once in ascending ID order regardless of filesystem enumeration order.
func TestIntegrationTxnRecoveryPruneReportDeterministicOrder(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, client, repo := recoveryEngine(t, r)

	var ids []string
	for i := 0; i < 3; i++ {
		c := abandonRegistered(t, client, repo, r, targetTip(t, r))
		ids = append(ids, c.id)
	}

	rep, err := eng.PruneAbandoned(context.Background(), repo)
	if err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	if len(rep.Entries) != len(ids) {
		t.Fatalf("entries = %+v, want %d", rep.Entries, len(ids))
	}

	var gotOrder []string
	for _, e := range rep.Entries {
		gotOrder = append(gotOrder, e.ID)
		if e.Verdict != verdictPruned {
			t.Errorf("candidate %s verdict = %q, want pruned", e.ID, e.Verdict)
		}
	}
	wantOrder := append([]string(nil), gotOrder...)
	sort.Strings(wantOrder)
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("report not sorted by ID: %v", gotOrder)
			break
		}
	}
}

// TestIntegrationTxnRecoveryPruneDocketModeIgnoresLinkedWorktree proves recovery prunes a candidate under
// the docket-mode topology and never mistakes the unrelated linked .docket worktree
// for a candidate registration.
func TestIntegrationTxnRecoveryPruneDocketModeIgnoresLinkedWorktree(t *testing.T) {
	requireGit(t)
	r := newDocketModeRepos(t)
	eng, client, repo := recoveryEngine(t, r)

	c := abandonRegistered(t, client, repo, r, targetTip(t, r))

	rep, err := eng.PruneAbandoned(context.Background(), repo)
	if err != nil {
		t.Fatalf("PruneAbandoned: %v", err)
	}
	e := pruneEntryFor(t, rep, c.id)
	if e.Verdict != verdictPruned {
		t.Fatalf("verdict = %q, want pruned (detail: %s)", e.Verdict, e.Detail)
	}
	if _, err := os.Stat(c.root); !os.IsNotExist(err) {
		t.Errorf("candidate root still present: %v", err)
	}

	// The linked .docket worktree is untouched and still registered.
	infos, err := client.ListWorktrees(context.Background(), repo)
	if err != nil {
		t.Fatalf("ListWorktrees: %v", err)
	}
	if !hasWorktreeNamed(infos, ".docket") {
		t.Errorf("linked .docket worktree missing after sweep: %+v", infos)
	}
}

// TestIntegrationTxnRecoveryCleanupRetainsRegisteredCandidateOnListError proves that when the worktree
// listing worktreeRegistered relies on fails during per-candidate cleanup, the
// candidate directory is RETAINED with a cleanup-pending warning rather than
// removed. A list error is indistinguishable from a genuine "not registered", so a
// direct removal would orphan the candidate's still-live worktree registration —
// administrative state PruneAbandoned can never reclaim once the directory is gone.
// This mirrors the retain-on-uncertainty posture of the RemoveWorktree-failed
// branch. (0309 review finding 1.)
func TestIntegrationTxnRecoveryCleanupRetainsRegisteredCandidateOnListError(t *testing.T) {
	requireGit(t)
	r := newMainModeRepos(t)
	eng, client, repo := recoveryEngine(t, r)

	c := abandonRegistered(t, client, repo, r, targetTip(t, r))

	// A cancelled context forces every git invocation — including the worktree
	// listing — to fail, simulating a transient ListWorktrees error for a
	// candidate that IS registered.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	warnings := eng.cleanupCandidate(ctx, repo, c)

	if _, err := os.Stat(c.root); err != nil {
		t.Fatalf("candidate root was removed on worktree-list error, want retained: %v", err)
	}
	if !hasCleanupPending(warnings, c.id) {
		t.Errorf("warnings = %v, want a %q entry", warnings, "cleanup-pending: "+c.id)
	}
}
