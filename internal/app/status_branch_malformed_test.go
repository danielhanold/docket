package app

import (
	"context"
	"reflect"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
)

// Change 0454: a change whose recorded branch: is not a valid git branch name
// must not fail the whole-repository reads that share the whole-corpus branch
// probe (stackBranches). Such a name cannot exist on the remote, so the probe
// skips it and the domain reports its stacked children stack-base-unresolved.

// malformedStackCorpus is the shared fixture: two in-progress parents whose
// recorded branches are malformed — feat/a..parent (the historical shape) and
// feat/a:b (rejected only by the completed check-ref-format grammar) — each
// with a stacked child; a well-formed parent/child pair; and an unrelated
// ordinary build-ready change.
func malformedStackCorpus(t *testing.T) []StatusBlob {
	t.Helper()
	return []StatusBlob{
		stackFixtureBlob(1, "parent-dots", "in-progress", "feat/a..parent", ""),
		stackFixtureBlob(2, "parent-colon", "in-progress", "feat/a:b", ""),
		stackFixtureBlob(3, "child-a", "proposed", "", "stacked_on: 1\n"),
		stackFixtureBlob(4, "child-b", "proposed", "", "stacked_on: 2\n"),
		stackFixtureBlob(5, "parent-ok", "in-progress", "feat/ok", ""),
		stackFixtureBlob(6, "child-ok", "proposed", "", "stacked_on: 5\n"),
		stackFixtureBlob(7, "plain", "proposed", "", ""),
	}
}

// TestStackBranchesSkipsMalformedNames: the whole-corpus probe set leaves out
// every recorded ancestor branch that fails gitcli.ValidBranchName.
func TestStackBranchesSkipsMalformedNames(t *testing.T) {
	snap := snapshotOf(t, malformedStackCorpus(t))
	got := stackBranches(snap)
	for _, b := range got {
		if b == "feat/a..parent" || b == "feat/a:b" {
			t.Errorf("stackBranches includes malformed name %q", b)
		}
	}
	want := []string{"feat/ok"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stackBranches = %v, want %v", got, want)
	}
}

// TestStatusSurvivesMalformedStackBranch: status over the fixture applies, the
// malformed names are never asked of the probe (poisoned), the malformed
// parents' children read stack-base-unresolved and stay out of ready, and the
// unrelated change still renders.
func TestStatusSurvivesMalformedStackBranch(t *testing.T) {
	pin := docketPin(t)
	inner := &fakeReader{pin: pin, corpus: malformedStackCorpus(t), facts: domain.NewBranchFacts(map[string]bool{"feat/ok": true})}
	reader := poisoned(inner, "feat/a..parent", "feat/a:b")
	res := Status(context.Background(), reader, StatusOptions{RepoDir: "."})
	if res.Result != ResultApplied {
		t.Fatalf("Status over a malformed stack branch = %s (%s: %s), want applied", res.Result, res.Reason, res.Message)
	}
	rows := map[int]StatusChange{}
	for _, c := range res.Changes {
		rows[c.ID] = c
	}
	for _, id := range []int{3, 4} { // children of the malformed parents
		if rows[id].Readiness != string(domain.ReadyStackBaseUnresolved) {
			t.Errorf("change %04d readiness = %q, want %q", id, rows[id].Readiness, domain.ReadyStackBaseUnresolved)
		}
		for _, r := range res.Ready {
			if r == id {
				t.Errorf("change %04d is in ready, want excluded", id)
			}
		}
	}
	if rows[7].ID != 7 {
		t.Errorf("unrelated change 0007 missing from the rendered backlog")
	}
}

// TestStatusStillFailsOnUnprobeableWellFormedBranch (Review Focus 1): the
// filter must not widen into swallowing probe errors — a well-formed recorded
// branch whose probe fails still fails the whole read external-failed.
func TestStatusStillFailsOnUnprobeableWellFormedBranch(t *testing.T) {
	pin := docketPin(t)
	inner := &fakeReader{pin: pin, corpus: malformedStackCorpus(t), facts: domain.NewBranchFacts(nil)}
	reader := poisoned(inner, "feat/ok") // well-formed, but the probe errors
	res := Status(context.Background(), reader, StatusOptions{RepoDir: "."})
	if res.Result != ResultExternalFailed || res.Reason != ReasonStatusExternal {
		t.Fatalf("Status with an unprobeable well-formed branch = %s (%s: %s), want external-failed", res.Result, res.Reason, res.Message)
	}
}
