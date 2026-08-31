package domain

import (
	"testing"
	"time"
)

// selSpec is a compact description of one change for the selection tests. Its
// zero value builds a build-ready change, so a case states only the fields the
// selection order — or the exclusion being probed — actually reads.
type selSpec struct {
	id       ChangeID
	slug     string
	priority Priority
	created  OptionalTime
	kind     string
	status   Status
	noDesign bool
	spec     OptionalString
}

// build turns a selSpec into an immutable Change, defaulting to a
// build-ready shape (proposed, and trivial when no spec: reference is given)
// unless the case asks for a change carrying no design.
func (sp selSpec) build() Change {
	status := sp.status
	if status == "" {
		status = StatusProposed
	}
	trivial := !sp.noDesign && sp.spec.State != FieldPresent
	slug := sp.slug
	if slug == "" {
		slug = "a-slug"
	}
	return NewChange(ChangeSpec{
		ID:       sp.id,
		Slug:     slug,
		Status:   status,
		Priority: sp.priority,
		Created:  sp.created,
		Type:     sp.kind,
		Trivial:  trivial,
		Spec:     sp.spec,
	})
}

// selSnapshot builds a snapshot with "main" as the integration branch.
func selSnapshot(specs ...selSpec) Snapshot {
	changes := make([]Change, 0, len(specs))
	for _, sp := range specs {
		changes = append(changes, sp.build())
	}
	return NewSnapshot(SnapshotSpec{
		Policy:  RepositoryPolicy{IntegrationBranch: "main"},
		Changes: changes,
	})
}

// createdOn is a well-formed date-only created field.
func createdOn(date string) OptionalTime {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return OptionalTime{State: FieldPresent, Value: t, Raw: date}
}

// badCreated is a present-but-unparseable created field.
func badCreated(raw string) OptionalTime {
	return OptionalTime{State: FieldMalformed, Raw: raw}
}

// ids extracts the ID sequence of a selection result.
func ids(changes []Change) []ChangeID {
	out := make([]ChangeID, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.ID())
	}
	return out
}

// equalIDs reports whether two ID sequences match.
func equalIDs(got, want []ChangeID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// selectQueueSignature is a compile-time anchor: SelectQueue's whole input is
// (Snapshot, BranchFacts, SelectionFilter) — snapshot and derived facts only,
// no configuration of any kind. If a future edit threads board presentation (or
// any config) into selection, its signature changes and this assignment stops
// compiling, reddening the package before any assertion runs. This is the
// domain half of the projection-isolation claim (change 0367): the autonomous
// ready-queue cannot even NAME board.section_order / board.sorting, because
// domain does not import the render or config packages that carry them.
var selectQueueSignature func(Snapshot, BranchFacts, SelectionFilter) []Change = SelectQueue

// TestSelectionUnchangedByBoardPresentation proves the autonomous selection
// order is decided by selection's own key (priority, then created age, then
// lowest id) and nothing a board presentation could reorder. The fixture is a
// single priority band whose members tie on priority, so the ONLY thing
// deciding their order is created-then-id — a key board.section_order and
// board.sorting have no way to reach (SelectQueue takes no config; see
// selectQueueSignature). id 5 arrives first and id 2 shares id 1's date, so a
// board-driven or arrival-driven order would produce a different sequence; the
// asserted queue is the selection key's alone.
func TestSelectionUnchangedByBoardPresentation(t *testing.T) {
	specs := []selSpec{
		{id: 5, priority: PriorityHigh}, // undated → sorts last in band
		{id: 3, priority: PriorityHigh, created: createdOn("2026-01-03")},
		{id: 2, priority: PriorityHigh, created: createdOn("2026-01-01")}, // ties id 1's date
		{id: 1, priority: PriorityHigh, created: createdOn("2026-01-01")},
	}
	got := ids(SelectQueue(selSnapshot(specs...), remotes(), SelectionFilter{}))
	// created ascending, id tie-break within the shared date, undated last:
	// 1 and 2 share 2026-01-01 (→ id 1 then 2), 3 is later, 5 is undated.
	want := []ChangeID{1, 2, 3, 5}
	if !equalIDs(got, want) {
		t.Fatalf("SelectQueue order = %v; want %v — selection order must follow its own key, "+
			"not board presentation or arrival order", got, want)
	}
}

func TestSelectQueueOrdering(t *testing.T) {
	tests := []struct {
		name  string
		specs []selSpec
		want  []ChangeID
	}{
		{
			name: "all four priorities interleave by rank, not by input order",
			specs: []selSpec{
				{id: 1, priority: PriorityLow, created: createdOn("2026-01-01")},
				{id: 2, priority: PriorityMedium, created: createdOn("2026-01-01")},
				{id: 3, priority: PriorityCritical, created: createdOn("2026-01-01")},
				{id: 4, priority: PriorityHigh, created: createdOn("2026-01-01")},
			},
			want: []ChangeID{3, 4, 2, 1},
		},
		{
			name: "within a band the earlier created date sorts first",
			specs: []selSpec{
				{id: 7, priority: PriorityHigh, created: createdOn("2026-03-09")},
				{id: 8, priority: PriorityHigh, created: createdOn("2026-01-02")},
				{id: 9, priority: PriorityHigh, created: createdOn("2026-02-28")},
			},
			want: []ChangeID{8, 9, 7},
		},
		{
			name: "malformed and absent created sort after every dated change in the band, tie-broken by ID",
			specs: []selSpec{
				{id: 3, priority: PriorityHigh, created: badCreated("not-a-date")},
				{id: 1, priority: PriorityHigh},
				{id: 2, priority: PriorityHigh, created: OptionalTime{State: FieldEmpty}},
				{id: 4, priority: PriorityHigh, created: createdOn("2030-12-31")},
				{id: 5, priority: PriorityHigh, created: createdOn("2020-01-01")},
			},
			want: []ChangeID{5, 4, 1, 2, 3},
		},
		{
			name: "an unknown stored priority slots with medium",
			specs: []selSpec{
				{id: 1, priority: PriorityLow, created: createdOn("2020-01-01")},
				{id: 2, priority: Priority("urgent-ish"), created: createdOn("2026-05-05")},
				{id: 3, priority: PriorityMedium, created: createdOn("2026-05-04")},
				{id: 4, priority: PriorityHigh, created: createdOn("2030-01-01")},
			},
			want: []ChangeID{4, 3, 2, 1},
		},
		{
			name: "an empty stored priority also ranks as medium",
			specs: []selSpec{
				{id: 1, priority: PriorityLow},
				{id: 2},
				{id: 3, priority: PriorityCritical},
			},
			want: []ChangeID{3, 2, 1},
		},
		{
			name: "numeric ID ascending is the final tie-break, never input order",
			specs: []selSpec{
				{id: 30, priority: PriorityMedium, created: createdOn("2026-06-01")},
				{id: 4, priority: PriorityMedium, created: createdOn("2026-06-01")},
				{id: 12, priority: PriorityMedium, created: createdOn("2026-06-01")},
			},
			want: []ChangeID{4, 12, 30},
		},
		{
			name: "the ID tie-break is numeric, not lexicographic",
			specs: []selSpec{
				{id: 100, priority: PriorityHigh},
				{id: 9, priority: PriorityHigh},
				{id: 21, priority: PriorityHigh},
			},
			want: []ChangeID{9, 21, 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ids(SelectQueue(selSnapshot(tt.specs...), remotes(), SelectionFilter{}))
			if !equalIDs(got, tt.want) {
				t.Fatalf("SelectQueue order = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestSelectQueueExcludesNonBuildReady(t *testing.T) {
	s := selSnapshot(
		selSpec{id: 1, priority: PriorityHigh},                           // proposed + trivial
		selSpec{id: 2, priority: PriorityHigh, status: StatusInProgress}, // not proposed
		selSpec{id: 3, priority: PriorityHigh, status: StatusDone},       // not proposed
		selSpec{id: 4, priority: PriorityHigh, noDesign: true, spec: OptionalString{State: FieldEmpty}},
		selSpec{id: 5, priority: PriorityHigh, spec: specRef("s.md")},
	)

	got := ids(SelectQueue(s, remotes(), SelectionFilter{}))
	if !equalIDs(got, []ChangeID{1, 5}) {
		t.Fatalf("SelectQueue = %v; want only the build-ready changes [1 5]", got)
	}
}

func TestSelectQueueExcludesChangesWithAmbiguousIDs(t *testing.T) {
	s := selSnapshot(
		selSpec{id: 7, priority: PriorityHigh},
		selSpec{id: 7, priority: PriorityHigh},
		selSpec{id: 8, priority: PriorityHigh},
	)

	got := ids(SelectQueue(s, remotes(), SelectionFilter{}))
	if !equalIDs(got, []ChangeID{8}) {
		t.Fatalf("SelectQueue = %v; want [8] — neither record claiming 7 may be selected", got)
	}
}

// Selection has no identity rule of its own: it gates on EvaluateReadiness,
// which reports a record whose id or slug cannot be trusted as invalid. One row
// here is enough to prove the exclusion reaches the queue.
func TestSelectQueueExcludesInvalidIdentities(t *testing.T) {
	s := selSnapshot(
		selSpec{id: 0, priority: PriorityCritical},
		selSpec{id: 8, slug: "Bad_Slug", priority: PriorityCritical},
		selSpec{id: 9, priority: PriorityHigh},
	)

	got := ids(SelectQueue(s, remotes(), SelectionFilter{}))
	if !equalIDs(got, []ChangeID{9}) {
		t.Fatalf("SelectQueue = %v; want [9] — a non-positive id and an ungrammatical slug are invalid", got)
	}
}

func TestSelectQueueExcludesAWaitingDependency(t *testing.T) {
	s := selSnapshot(
		selSpec{id: 1, priority: PriorityHigh, status: StatusImplemented},
		selSpec{id: 2, priority: PriorityHigh},
	)
	dependent := NewChange(ChangeSpec{
		ID: 3, Status: StatusProposed, Priority: PriorityCritical, Trivial: true,
		DependsOn: []ChangeID{1},
	})
	s = NewSnapshot(SnapshotSpec{
		Policy:  RepositoryPolicy{IntegrationBranch: "main"},
		Changes: append(s.Changes(), dependent),
	})

	got := ids(SelectQueue(s, remotes(), SelectionFilter{}))
	if !equalIDs(got, []ChangeID{2}) {
		t.Fatalf("SelectQueue = %v; want [2] — 3 waits on an unmerged dependency", got)
	}
}

func TestSelectQueueFilters(t *testing.T) {
	specs := []selSpec{
		{id: 1, priority: PriorityHigh, kind: "feature", created: createdOn("2026-01-01")},
		{id: 2, priority: PriorityLow, kind: "feature", created: createdOn("2026-01-02")},
		{id: 3, priority: PriorityHigh, kind: "bug", created: createdOn("2026-01-03")},
		{id: 4, priority: PriorityMedium, kind: "chore", created: createdOn("2026-01-04")},
		{id: 5, priority: Priority("weird"), kind: "feature", created: createdOn("2026-01-05")},
	}

	tests := []struct {
		name   string
		filter SelectionFilter
		want   []ChangeID
	}{
		{
			name:   "an empty filter selects everything build-ready",
			filter: SelectionFilter{},
			want:   []ChangeID{1, 3, 4, 5, 2},
		},
		{
			name:   "a single type filter keeps only that type",
			filter: SelectionFilter{Types: []string{"feature"}},
			want:   []ChangeID{1, 5, 2},
		},
		{
			name:   "a multi-type filter is a union",
			filter: SelectionFilter{Types: []string{"bug", "chore"}},
			want:   []ChangeID{3, 4},
		},
		{
			name:   "a priority filter matches the stored value, so an unknown priority is not medium",
			filter: SelectionFilter{Priorities: []Priority{PriorityMedium}},
			want:   []ChangeID{4},
		},
		{
			name:   "a multi-priority filter is a union",
			filter: SelectionFilter{Priorities: []Priority{PriorityHigh, PriorityLow}},
			want:   []ChangeID{1, 3, 2},
		},
		{
			name: "type and priority filters intersect",
			filter: SelectionFilter{
				Types:      []string{"feature"},
				Priorities: []Priority{PriorityHigh},
			},
			want: []ChangeID{1},
		},
		{
			name:   "a filter matching nothing returns an empty queue",
			filter: SelectionFilter{Types: []string{"nope"}},
			want:   []ChangeID{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ids(SelectQueue(selSnapshot(specs...), remotes(), tt.filter))
			if !equalIDs(got, tt.want) {
				t.Fatalf("SelectQueue = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestSelectQueueFilterInputsAreNotRetained(t *testing.T) {
	s := selSnapshot(
		selSpec{id: 1, priority: PriorityHigh, kind: "feature"},
		selSpec{id: 2, priority: PriorityHigh, kind: "bug"},
	)
	types := []string{"feature"}
	filter := SelectionFilter{Types: types}

	first := ids(SelectQueue(s, remotes(), filter))
	types[0] = "bug"
	second := ids(SelectQueue(s, remotes(), SelectionFilter{Types: []string{"feature"}}))

	if !equalIDs(first, []ChangeID{1}) || !equalIDs(second, []ChangeID{1}) {
		t.Fatalf("SelectQueue results = %v then %v; want [1] both times", first, second)
	}
}

func TestSelectQueueReturnsAFreshSlice(t *testing.T) {
	s := selSnapshot(
		selSpec{id: 1, priority: PriorityHigh},
		selSpec{id: 2, priority: PriorityHigh},
	)

	first := SelectQueue(s, remotes(), SelectionFilter{})
	if len(first) != 2 {
		t.Fatalf("len(SelectQueue) = %d; want 2", len(first))
	}
	first[0] = NewChange(ChangeSpec{ID: 999})

	second := SelectQueue(s, remotes(), SelectionFilter{})
	if !equalIDs(ids(second), []ChangeID{1, 2}) {
		t.Fatalf("second SelectQueue = %v; want [1 2] — the first result must not alias stored state", ids(second))
	}
	if !equalIDs(ids(s.Changes()), []ChangeID{1, 2}) {
		t.Fatalf("snapshot changes = %v; want [1 2]", ids(s.Changes()))
	}
}
