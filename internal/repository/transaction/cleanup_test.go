package transaction

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

// This file covers PruneReport itself: an empty sweep, deterministic ordering, and
// that recovery works identically under the docket-mode topology (a linked .docket
// worktree present) as under main mode. The full ownership-and-recovery matrix
// lives in recovery_test.go.

// TestPruneReportEmptyOnCleanRoot proves a sweep of a repository with no candidates
// returns an empty, non-error report — the transactions root need not pre-exist.
func TestPruneReportEmptyOnCleanRoot(t *testing.T) {
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

// TestPruneReportDeterministicOrder proves the report lists every candidate exactly
// once in ascending ID order regardless of filesystem enumeration order.
func TestPruneReportDeterministicOrder(t *testing.T) {
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

// TestPruneDocketModeIgnoresLinkedWorktree proves recovery prunes a candidate under
// the docket-mode topology and never mistakes the unrelated linked .docket worktree
// for a candidate registration.
func TestPruneDocketModeIgnoresLinkedWorktree(t *testing.T) {
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

// TestCleanupRetainsRegisteredCandidateOnListError proves that when the worktree
// listing worktreeRegistered relies on fails during per-candidate cleanup, the
// candidate directory is RETAINED with a cleanup-pending warning rather than
// removed. A list error is indistinguishable from a genuine "not registered", so a
// direct removal would orphan the candidate's still-live worktree registration —
// administrative state PruneAbandoned can never reclaim once the directory is gone.
// This mirrors the retain-on-uncertainty posture of the RemoveWorktree-failed
// branch. (0309 review finding 1.)
func TestCleanupRetainsRegisteredCandidateOnListError(t *testing.T) {
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

// hasCleanupPending reports whether warnings names id's cleanup-pending marker.
func hasCleanupPending(warnings []string, id string) bool {
	want := "cleanup-pending: " + id
	for _, w := range warnings {
		if w == want {
			return true
		}
	}
	return false
}

// hasWorktreeNamed reports whether any registration's path basename equals name.
func hasWorktreeNamed(infos []gitcli.WorktreeInfo, name string) bool {
	for _, info := range infos {
		if base := baseName(info.Path); base == name {
			return true
		}
	}
	return false
}

// baseName returns the final path element without importing path/filepath twice in
// assertions; it keeps this file's helper self-contained.
func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}
