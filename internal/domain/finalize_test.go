package domain

import (
	"fmt"
	"testing"
)

// finDefaultBranch is the recorded feature branch finChange stamps by default,
// and the head branch the actionable-candidate fixtures pair with it so an
// implemented open/merged PR reconciles cleanly and bands rather than surfacing
// an identity skip. A shared constant keeps the ordering fixtures branch-uniform.
const finDefaultBranch = "feat/head"

// finChange builds an implemented change carrying a PR reference and the recorded
// feature branch stamped at claim time, the default finalize-candidate shape.
// Options mutate the spec for the case under test.
func finChange(id ChangeID, opts ...func(*ChangeSpec)) Change {
	spec := ChangeSpec{
		ID:     id,
		Slug:   fmt.Sprintf("c-%d", id),
		Status: StatusImplemented,
		PR:     OptionalString{State: FieldPresent, Value: fmt.Sprintf("#%d", id)},
		Branch: OptionalString{State: FieldPresent, Value: finDefaultBranch},
	}
	for _, o := range opts {
		o(&spec)
	}
	return NewChange(spec)
}

func withStatus(s Status) func(*ChangeSpec) { return func(sp *ChangeSpec) { sp.Status = s } }

// withBranch sets the recorded feature branch; noBranch clears it (a record with
// no usable branch). Together they drive the identity classification cases.
func withBranch(b string) func(*ChangeSpec) {
	return func(sp *ChangeSpec) { sp.Branch = OptionalString{State: FieldPresent, Value: b} }
}
func noBranch() func(*ChangeSpec)         { return func(sp *ChangeSpec) { sp.Branch = OptionalString{} } }
func withSlug(s string) func(*ChangeSpec) { return func(sp *ChangeSpec) { sp.Slug = s } }
func withPriority(p Priority) func(*ChangeSpec) {
	return func(sp *ChangeSpec) { sp.Priority = p }
}
func withCreated(o OptionalTime) func(*ChangeSpec) {
	return func(sp *ChangeSpec) { sp.Created = o }
}
func withDeps(ids ...ChangeID) func(*ChangeSpec) {
	return func(sp *ChangeSpec) { sp.DependsOn = ids }
}
func noPR() func(*ChangeSpec) { return func(sp *ChangeSpec) { sp.PR = OptionalString{} } }

// finSnapshot builds a snapshot with "main" as the integration branch.
func finSnapshot(changes ...Change) Snapshot {
	return NewSnapshot(SnapshotSpec{
		Policy:  RepositoryPolicy{IntegrationBranch: "main"},
		Changes: changes,
	})
}

// candidateIDs extracts the ID order of a finalize queue.
func candidateIDs(cands []FinalizeCandidate) []ChangeID {
	out := make([]ChangeID, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.ID)
	}
	return out
}

// bandOf returns the reported band for id, or "" when absent.
func bandOf(cands []FinalizeCandidate, id ChangeID) string {
	for _, c := range cands {
		if c.ID == id {
			return c.Band
		}
	}
	return ""
}

func eqIDs(t *testing.T, got, want []ChangeID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("id order = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("id order = %v, want %v", got, want)
		}
	}
}

func TestSelectFinalizeQueueOrdering(t *testing.T) {
	// Bands, then changed-file, then line tiebreaks.
	changes := []Change{
		finChange(1), finChange(2), finChange(3), finChange(4), finChange(5),
	}
	facts := map[ChangeID]PRFacts{
		1: {State: "merged", HeadBranch: finDefaultBranch},
		2: {State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: finDefaultBranch, ChangedFiles: 5, DiffLines: 100},
		3: {State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: finDefaultBranch, ChangedFiles: 2, DiffLines: 999},
		4: {State: "open", Approved: true, Mergeable: "CONFLICTING", HeadBranch: finDefaultBranch, ChangedFiles: 1, DiffLines: 50},
		5: {State: "open", Approved: true, Mergeable: "UNKNOWN", HeadBranch: finDefaultBranch, ChangedFiles: 1, DiffLines: 1},
	}
	got := SelectFinalizeQueue(finSnapshot(changes...), facts, nil, nil)
	eqIDs(t, candidateIDs(got), []ChangeID{1, 3, 2, 5, 4})

	if b := bandOf(got, 1); b != "merged-recovery" {
		t.Errorf("id 1 band = %q, want merged-recovery", b)
	}
	if b := bandOf(got, 3); b != "mergeable" {
		t.Errorf("id 3 band = %q, want mergeable", b)
	}
	if b := bandOf(got, 4); b != "conflicting" {
		t.Errorf("id 4 band = %q, want conflicting", b)
	}
	if b := bandOf(got, 5); b != "unknown" {
		t.Errorf("id 5 band = %q, want unknown", b)
	}

	// Determinism: identical inputs yield identical output.
	again := SelectFinalizeQueue(finSnapshot(changes...), facts, nil, nil)
	eqIDs(t, candidateIDs(again), candidateIDs(got))

	// Priority / created / id tail, once band + file + line all tie.
	tail := []Change{
		finChange(6, withPriority(PriorityLow), withCreated(createdOn("2026-01-02"))),
		finChange(7, withPriority(PriorityHigh), withCreated(createdOn("2026-01-05"))),
		finChange(8, withPriority(PriorityHigh), withCreated(createdOn("2026-01-01"))),
		finChange(9, withPriority(PriorityHigh), withCreated(createdOn("2026-01-01"))),
		finChange(10, withPriority(PriorityMedium)),
	}
	tf := PRFacts{State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: finDefaultBranch, ChangedFiles: 3, DiffLines: 3}
	tfacts := map[ChangeID]PRFacts{6: tf, 7: tf, 8: tf, 9: tf, 10: tf}
	tgot := SelectFinalizeQueue(finSnapshot(tail...), tfacts, nil, nil)
	eqIDs(t, candidateIDs(tgot), []ChangeID{8, 9, 7, 10, 6})
}

func TestSelectFinalizeQueueSkipReasons(t *testing.T) {
	open := PRFacts{State: "open", Approved: true, Mergeable: "MERGEABLE"}
	changes := []Change{
		finChange(10, withStatus(StatusInProgress)), // not-implemented
		finChange(11),               // draft (facts.Draft)
		finChange(12),               // pr-closed
		finChange(13),               // approval-required
		finChange(14),               // finalize-blocked
		finChange(15, withDeps(16)), // dependency-unmerged
		finChange(16, withStatus(StatusImplemented), noPR()), // unmerged dep, out of population
		finChange(18, withSlug("Bad Slug")),                  // malformed
		finChange(19),                                        // pr-unknown
	}
	facts := map[ChangeID]PRFacts{
		10: open,
		11: {State: "open", Draft: true, Approved: true, Mergeable: "MERGEABLE"},
		12: {State: "closed"},
		13: {State: "open", Approved: false, Mergeable: "MERGEABLE"},
		14: open,
		15: open,
		18: open,
		19: {State: "unknown"},
	}
	blocked := map[ChangeID]bool{14: true}
	got := SelectFinalizeQueue(finSnapshot(changes...), facts, blocked, nil)

	want := map[ChangeID]string{
		10: "not-implemented",
		11: "draft",
		12: "pr-closed",
		13: "approval-required",
		14: "finalize-blocked",
		15: "dependency-unmerged",
		18: "malformed",
		19: "pr-unknown",
	}
	reasons := map[ChangeID]string{}
	seen := map[ChangeID]bool{}
	for _, c := range got {
		reasons[c.ID] = c.SkipReason
		seen[c.ID] = true
	}
	for id, tok := range want {
		if !seen[id] {
			t.Errorf("id %d omitted; a skipped candidate must still surface", id)
			continue
		}
		if reasons[id] != tok {
			t.Errorf("id %d skip reason = %q, want %q", id, reasons[id], tok)
		}
	}
	if seen[16] {
		t.Errorf("id 16 has no PR reference; it must not enter the population")
	}
}

// TestSelectFinalizeQueueIdentityClassification pins the recorded-branch vs
// exact-PR-head reconciliation: a clean match bands normally, a disagreement or
// an unusable recorded branch surfaces a structured identity skip, and — the
// regression pin — a head mismatch NEVER reclassifies a cleanly closed PR (which
// stays pr-closed) nor an unknown probe (which stays pr-unknown). Identity is
// computed only against cleanly observed open/merged evidence.
func TestSelectFinalizeQueueIdentityClassification(t *testing.T) {
	const recorded = "feature/renamed-head"
	open := func(head string) PRFacts {
		return PRFacts{State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: head}
	}
	changes := []Change{
		finChange(1, withBranch(recorded)),       // open, head matches -> banded
		finChange(2, withBranch(recorded)),       // open, head differs -> mismatch
		finChange(3, noBranch()),                 // open, recorded absent -> branch-missing
		finChange(4, withBranch("refs/heads/x")), // open, recorded shape-invalid -> branch-malformed
		finChange(5, withBranch(recorded)),       // unknown probe -> pr-unknown regardless of branch
		finChange(6, withBranch(recorded)),       // cleanly closed -> pr-closed regardless of head
	}
	facts := map[ChangeID]PRFacts{
		1: open(recorded),
		2: open("feature/other"),
		3: open("feature/other"), // the exact PR's head is present; it is the recorded value that is missing
		4: open("feature/other"),
		5: {State: "unknown", HeadBranch: "feature/other"},
		6: {State: "closed", HeadBranch: "feature/other"},
	}
	got := SelectFinalizeQueue(finSnapshot(changes...), facts, nil, nil)

	bands := map[ChangeID]string{}
	skips := map[ChangeID]string{}
	for _, c := range got {
		bands[c.ID] = c.Band
		skips[c.ID] = c.SkipReason
	}
	if bands[1] != "mergeable" || skips[1] != "" {
		t.Errorf("id 1 (head matches recorded) = band %q skip %q, want mergeable/actionable", bands[1], skips[1])
	}
	wantSkip := map[ChangeID]string{
		2: "branch-pr-head-mismatch",
		3: "branch-missing",
		4: "branch-malformed",
		5: "pr-unknown",
		6: "pr-closed",
	}
	for id, want := range wantSkip {
		if skips[id] != want {
			t.Errorf("id %d skip = %q, want %q", id, skips[id], want)
		}
	}
}

func TestSelectFinalizeQueueExplicitOverride(t *testing.T) {
	// approval-required and finalize-blocked are skip reasons in auto mode; the
	// app layer (Task 10) overrides them for an explicit --id. Here we only
	// assert the tokens exist so that override has something to key on.
	changes := []Change{finChange(1), finChange(2)}
	facts := map[ChangeID]PRFacts{
		1: {State: "open", Approved: false, Mergeable: "MERGEABLE"}, // approval-required
		2: {State: "open", Approved: true, Mergeable: "MERGEABLE"},  // finalize-blocked
	}
	blocked := map[ChangeID]bool{2: true}
	got := SelectFinalizeQueue(finSnapshot(changes...), facts, blocked, nil)
	reasons := map[ChangeID]string{}
	for _, c := range got {
		reasons[c.ID] = c.SkipReason
	}
	if reasons[1] != "approval-required" {
		t.Errorf("id 1 skip = %q, want approval-required", reasons[1])
	}
	if reasons[2] != "finalize-blocked" {
		t.Errorf("id 2 skip = %q, want finalize-blocked", reasons[2])
	}
}

func TestSelectFinalizeQueueAllowlist(t *testing.T) {
	changes := []Change{finChange(1), finChange(2), finChange(3)}
	facts := map[ChangeID]PRFacts{
		1: {State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: finDefaultBranch, ChangedFiles: 1},
		2: {State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: finDefaultBranch, ChangedFiles: 2},
		3: {State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: finDefaultBranch, ChangedFiles: 3},
	}
	full := SelectFinalizeQueue(finSnapshot(changes...), facts, nil, nil)
	eqIDs(t, candidateIDs(full), []ChangeID{1, 2, 3})

	// Allowlist bounds membership without reordering survivors.
	bounded := SelectFinalizeQueue(finSnapshot(changes...), facts, nil, []ChangeID{3, 1})
	eqIDs(t, candidateIDs(bounded), []ChangeID{1, 3})
}

func TestSelectFinalizeQueueDependencyOrder(t *testing.T) {
	// depends_on names an unmerged change -> dependency-unmerged.
	unmerged := []Change{
		finChange(1, withDeps(2)),
		finChange(2, withStatus(StatusImplemented), noPR()),
	}
	facts := map[ChangeID]PRFacts{1: {State: "open", Approved: true, Mergeable: "MERGEABLE", HeadBranch: finDefaultBranch}}
	got := SelectFinalizeQueue(finSnapshot(unmerged...), facts, nil, nil)
	if len(got) != 1 || got[0].ID != 1 || got[0].SkipReason != "dependency-unmerged" {
		t.Fatalf("got %+v, want single id 1 dependency-unmerged", got)
	}

	// Once the dependency is done, the change is actionable.
	merged := []Change{
		finChange(1, withDeps(2)),
		finChange(2, withStatus(StatusDone), noPR()),
	}
	got2 := SelectFinalizeQueue(finSnapshot(merged...), facts, nil, nil)
	if len(got2) != 1 || got2[0].ID != 1 || got2[0].SkipReason != "" || got2[0].Band != "mergeable" {
		t.Fatalf("got %+v, want single id 1 actionable mergeable", got2)
	}
}

func TestSelectFinalizeQueueNilSafe(t *testing.T) {
	if got := SelectFinalizeQueue(finSnapshot(), nil, nil, nil); len(got) != 0 {
		t.Fatalf("empty snapshot yielded %d candidates, want 0", len(got))
	}
	// A change with no facts entry surfaces as pr-unknown, never a panic.
	got := SelectFinalizeQueue(finSnapshot(finChange(1)), nil, nil, nil)
	if len(got) != 1 || got[0].SkipReason != "pr-unknown" {
		t.Fatalf("got %+v, want single id 1 pr-unknown", got)
	}
}

func TestMergeConjunctsFirstFailure(t *testing.T) {
	all := MergeConjuncts{
		Implemented: true, PRIdentityMatch: true, HeadsAgree: true, OpenNonDraft: true,
		BaseIsEffectiveBase: true, GateSatisfied: true, ApprovalSatisfied: true,
		NoOpenChildren: true, NotSuperseded: true,
	}
	if !all.AllHold() {
		t.Fatalf("all-true AllHold() = false")
	}
	if got := all.FirstFailure(); got != "" {
		t.Fatalf("all-true FirstFailure() = %q, want \"\"", got)
	}

	cases := []struct {
		name  string
		mut   func(*MergeConjuncts)
		token string
	}{
		{"implemented", func(m *MergeConjuncts) { m.Implemented = false }, "not-implemented"},
		{"pr-identity", func(m *MergeConjuncts) { m.PRIdentityMatch = false }, "pr-identity-mismatch"},
		{"heads", func(m *MergeConjuncts) { m.HeadsAgree = false }, "head-moved"},
		{"open-nondraft", func(m *MergeConjuncts) { m.OpenNonDraft = false }, "not-open-nondraft"},
		{"base", func(m *MergeConjuncts) { m.BaseIsEffectiveBase = false }, "base-mismatch"},
		{"gate", func(m *MergeConjuncts) { m.GateSatisfied = false }, "gate-unsatisfied"},
		{"approval", func(m *MergeConjuncts) { m.ApprovalSatisfied = false }, "approval-required"},
		{"children", func(m *MergeConjuncts) { m.NoOpenChildren = false }, "open-children"},
		{"superseded", func(m *MergeConjuncts) { m.NotSuperseded = false }, "superseded"},
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		m := all
		tc.mut(&m)
		got := m.FirstFailure()
		if got != tc.token {
			t.Errorf("%s: FirstFailure() = %q, want %q", tc.name, got, tc.token)
		}
		if m.AllHold() {
			t.Errorf("%s: AllHold() = true with a false field", tc.name)
		}
		if seen[got] {
			t.Errorf("%s: token %q not distinct", tc.name, got)
		}
		seen[got] = true
	}
}
