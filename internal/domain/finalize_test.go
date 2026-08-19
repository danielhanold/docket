package domain

import (
	"fmt"
	"testing"
)

// finChange builds an implemented change carrying a PR reference, the default
// finalize-candidate shape. Options mutate the spec for the case under test.
func finChange(id ChangeID, opts ...func(*ChangeSpec)) Change {
	spec := ChangeSpec{
		ID:     id,
		Slug:   fmt.Sprintf("c-%d", id),
		Status: StatusImplemented,
		PR:     OptionalString{State: FieldPresent, Value: fmt.Sprintf("#%d", id)},
	}
	for _, o := range opts {
		o(&spec)
	}
	return NewChange(spec)
}

func withStatus(s Status) func(*ChangeSpec) { return func(sp *ChangeSpec) { sp.Status = s } }
func withSlug(s string) func(*ChangeSpec)   { return func(sp *ChangeSpec) { sp.Slug = s } }
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
		1: {State: "merged"},
		2: {State: "open", Approved: true, Mergeable: "MERGEABLE", ChangedFiles: 5, DiffLines: 100},
		3: {State: "open", Approved: true, Mergeable: "MERGEABLE", ChangedFiles: 2, DiffLines: 999},
		4: {State: "open", Approved: true, Mergeable: "CONFLICTING", ChangedFiles: 1, DiffLines: 50},
		5: {State: "open", Approved: true, Mergeable: "UNKNOWN", ChangedFiles: 1, DiffLines: 1},
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
	tf := PRFacts{State: "open", Approved: true, Mergeable: "MERGEABLE", ChangedFiles: 3, DiffLines: 3}
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
		1: {State: "open", Approved: true, Mergeable: "MERGEABLE", ChangedFiles: 1},
		2: {State: "open", Approved: true, Mergeable: "MERGEABLE", ChangedFiles: 2},
		3: {State: "open", Approved: true, Mergeable: "MERGEABLE", ChangedFiles: 3},
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
	facts := map[ChangeID]PRFacts{1: {State: "open", Approved: true, Mergeable: "MERGEABLE"}}
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
