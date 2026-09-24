package app

import (
	"bytes"
	"context"
	"reflect"
	"strings"
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
// ordinary build-ready change. Change 0454's finding tests also lean on the
// pr: shapes: 0001 carries a parseable PR, 0002 none, and 0008 a third
// malformed parent (feat/a:c) whose pr: is present but unparseable.
func malformedStackCorpus(t *testing.T) []StatusBlob {
	t.Helper()
	return []StatusBlob{
		stackFixtureBlob(1, "parent-dots", "in-progress", "feat/a..parent", "pr: 'https://github.com/danielhanold/docket/pull/77'\n"),
		stackFixtureBlob(2, "parent-colon", "in-progress", "feat/a:b", ""),
		stackFixtureBlob(3, "child-a", "proposed", "", "stacked_on: 1\n"),
		stackFixtureBlob(4, "child-b", "proposed", "", "stacked_on: 2\n"),
		stackFixtureBlob(5, "parent-ok", "in-progress", "feat/ok", ""),
		stackFixtureBlob(6, "child-ok", "proposed", "", "stacked_on: 5\n"),
		stackFixtureBlob(7, "plain", "proposed", "", ""),
		stackFixtureBlob(8, "parent-badpr", "in-progress", "feat/a:c", "pr: 'broken'\n"),
	}
}

// TestStackBranchesSkipsMalformedNames: the whole-corpus probe set leaves out
// every recorded ancestor branch that fails gitcli.ValidBranchName.
func TestStackBranchesSkipsMalformedNames(t *testing.T) {
	snap := snapshotOf(t, malformedStackCorpus(t))
	got := stackBranches(snap)
	for _, b := range got {
		if b == "feat/a..parent" || b == "feat/a:b" || b == "feat/a:c" {
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
	reader := poisoned(inner, "feat/a..parent", "feat/a:b", "feat/a:c")
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

// TestStatusBranchMalformedFindings: every displayed active change whose
// recorded branch: is not a valid git branch name gets exactly one error
// finding, and its remedy is valid in the exact state that produced it
// (printed-remedy-state-validity): a parseable pr: names the typed
// repair-identity adopt-pr-head command with id, version, and PR number
// filled in; no pr: or an unparseable one (Review Focus 2) gets the hand-edit
// plus repository migrate remedy, never a fabricated PR number.
func TestStatusBranchMalformedFindings(t *testing.T) {
	pin := docketPin(t)
	inner := &fakeReader{pin: pin, corpus: malformedStackCorpus(t), facts: domain.NewBranchFacts(map[string]bool{"feat/ok": true})}
	reader := poisoned(inner, "feat/a..parent", "feat/a:b", "feat/a:c")
	res := Status(context.Background(), reader, StatusOptions{RepoDir: "."})
	if res.Result != ResultApplied {
		t.Fatalf("Status = %s (%s: %s), want applied", res.Result, res.Reason, res.Message)
	}
	byIdentity := map[string][]StatusFinding{}
	for _, f := range res.Findings {
		if f.Code == string(FCBranchMalformed) {
			byIdentity[f.Identity] = append(byIdentity[f.Identity], f)
		}
	}
	// Exactly one finding per malformed displayed change; none for well-formed ones.
	for _, id := range []string{"0001", "0002", "0008"} {
		fs := byIdentity[id]
		if len(fs) != 1 {
			t.Fatalf("change %s: %d branch-malformed findings, want 1", id, len(fs))
		}
		f := fs[0]
		if f.Severity != string(domain.SeverityError) || f.Entity != string(domain.EntityChange) || f.Field != "branch" {
			t.Errorf("change %s finding shape = %+v", id, f)
		}
		if !strings.Contains(f.Message, "not a valid git branch name") {
			t.Errorf("change %s message = %q", id, f.Message)
		}
	}
	if len(byIdentity) != 3 {
		t.Errorf("branch-malformed identities = %v, want exactly 0001 0002 0008", byIdentity)
	}
	prRemedy := byIdentity["0001"][0].Remedy
	for _, want := range []string{"change repair-identity", "--id 1 ", "--expect-version blobchange0001", "--adopt-pr-head", "--expect-pr 77", "PR #77"} {
		if !strings.Contains(prRemedy, want) {
			t.Errorf("PR-case remedy %q lacks %q", prRemedy, want)
		}
	}
	for _, id := range []string{"0002", "0008"} {
		r := byIdentity[id][0].Remedy
		if strings.Contains(r, "repair-identity") || !strings.Contains(r, "repository migrate") {
			t.Errorf("change %s remedy = %q, want the hand-edit + repository migrate remedy", id, r)
		}
	}
}

// retyped rewrites a stackFixtureBlob record's type: (the helper stamps feat).
func retyped(t *testing.T, b StatusBlob, typ string) StatusBlob {
	t.Helper()
	const from = "\ntype: feat\n"
	if !bytes.Contains(b.Data, []byte(from)) {
		t.Fatalf("fixture %s has no %q line", b.Path, from)
	}
	b.Data = bytes.Replace(b.Data, []byte(from), []byte("\ntype: "+typ+"\n"), 1)
	return b
}

// TestStatusBranchMalformedDisplayScope (Review Focus 5): the finding covers
// displayed active changes only — a --type projection that filters out the
// malformed parents yields no branch-malformed finding — while the probe skip
// stays corpus-wide: the filtered-out parents' malformed names are still never
// asked of the probe, and their displayed children still read
// stack-base-unresolved.
func TestStatusBranchMalformedDisplayScope(t *testing.T) {
	corpus := malformedStackCorpus(t)
	for i, b := range corpus {
		switch {
		case strings.Contains(b.Path, "/0001-"), strings.Contains(b.Path, "/0002-"), strings.Contains(b.Path, "/0008-"):
			corpus[i] = retyped(t, b, "fix")
		}
	}
	pin := docketPin(t)
	inner := &fakeReader{pin: pin, corpus: corpus, facts: domain.NewBranchFacts(map[string]bool{"feat/ok": true})}
	reader := poisoned(inner, "feat/a..parent", "feat/a:b", "feat/a:c")
	res := Status(context.Background(), reader, StatusOptions{RepoDir: ".", Types: []string{"feat"}})
	if res.Result != ResultApplied {
		t.Fatalf("Status = %s (%s: %s), want applied", res.Result, res.Reason, res.Message)
	}
	rows := map[int]StatusChange{}
	for _, c := range res.Changes {
		rows[c.ID] = c
	}
	for _, id := range []int{1, 2, 8} {
		if _, shown := rows[id]; shown {
			t.Fatalf("change %04d displayed under --type feat; the fixture must filter it out", id)
		}
	}
	if rows[7].ID != 7 {
		t.Fatalf("change 0007 missing from the --type feat projection")
	}
	for _, f := range res.Findings {
		if f.Code == string(FCBranchMalformed) {
			t.Errorf("branch-malformed finding for a change not displayed: %+v", f)
		}
	}
	for _, id := range []int{3, 4} {
		if rows[id].Readiness != string(domain.ReadyStackBaseUnresolved) {
			t.Errorf("change %04d readiness = %q, want %q", id, rows[id].Readiness, domain.ReadyStackBaseUnresolved)
		}
	}
}
