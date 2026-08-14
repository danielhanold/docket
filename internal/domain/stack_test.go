package domain

import (
	"slices"
	"testing"
)

// stackSpec is a compact description of one stacked change for the tests:
// an optional parent edge, a status, and an optional recorded branch.
type stackSpec struct {
	id     ChangeID
	parent OptionalInt
	status Status
	branch string
}

// parentEdge is a present stacked_on edge pointing at id.
func parentEdge(id ChangeID) OptionalInt {
	return OptionalInt{State: FieldPresent, Value: int(id)}
}

// build turns a stackSpec into an immutable Change.
func (sp stackSpec) build() Change {
	spec := ChangeSpec{ID: sp.id, Status: sp.status, StackedOn: sp.parent}
	if sp.branch != "" {
		spec.Branch = OptionalString{State: FieldPresent, Value: sp.branch}
	}
	return NewChange(spec)
}

// stackSnapshot builds a snapshot with "main" as the integration branch.
func stackSnapshot(specs ...stackSpec) Snapshot {
	changes := make([]Change, 0, len(specs))
	for _, sp := range specs {
		changes = append(changes, sp.build())
	}
	return NewSnapshot(SnapshotSpec{
		Policy:  RepositoryPolicy{IntegrationBranch: "main"},
		Changes: changes,
	})
}

// remotes builds the branch facts from a list of remote branch names.
func remotes(names ...string) BranchFacts {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return NewBranchFacts(set)
}

// resolveOf resolves id's effective base against s.
func resolveOf(t *testing.T, s Snapshot, id ChangeID, facts BranchFacts) EffectiveBase {
	t.Helper()
	c, out := s.Change(id)
	if out != LookupFound {
		t.Fatalf("Change(%d) outcome = %v; want LookupFound", id, out)
	}
	return ResolveEffectiveBase(s, c, facts)
}

func TestResolveEffectiveBaseRules(t *testing.T) {
	tests := []struct {
		name     string
		specs    []stackSpec
		branches []string
		subject  ChangeID
		want     EffectiveBase
	}{
		{
			name:    "rule 1: unstacked resolves to the integration branch",
			specs:   []stackSpec{{id: 2, status: StatusProposed}},
			subject: 2,
			want:    EffectiveBase{Kind: BaseResolved, Branch: "main"},
		},
		{
			name:    "rule 1: an empty stacked_on edge is unstacked",
			specs:   []stackSpec{{id: 2, status: StatusProposed, parent: OptionalInt{State: FieldEmpty}}},
			subject: 2,
			want:    EffectiveBase{Kind: BaseResolved, Branch: "main"},
		},
		{
			name: "rule 2: killed parent, even with its branch still on the remote",
			specs: []stackSpec{
				{id: 1, status: StatusKilled, branch: "feat/one"},
				{id: 2, status: StatusProposed, parent: parentEdge(1)},
			},
			branches: []string{"feat/one"},
			subject:  2,
			want:     EffectiveBase{Kind: BaseParentKilled, Cause: 1},
		},
		{
			name: "rule 3: done parent resolves terminally to the integration branch",
			specs: []stackSpec{
				{id: 1, status: StatusDone, branch: "feat/one"},
				{id: 2, status: StatusProposed, parent: parentEdge(1)},
			},
			branches: []string{"feat/one"},
			subject:  2,
			want:     EffectiveBase{Kind: BaseResolved, Branch: "main"},
		},
		{
			name: "rule 4: live parent whose recorded branch is on the remote",
			specs: []stackSpec{
				{id: 1, status: StatusInProgress, branch: "feat/one"},
				{id: 2, status: StatusProposed, parent: parentEdge(1)},
			},
			branches: []string{"feat/one"},
			subject:  2,
			want:     EffectiveBase{Kind: BaseResolved, Branch: "feat/one"},
		},
		{
			name: "rule 5: branchless stacked-merged parent recurses into its own base",
			specs: []stackSpec{
				{id: 1, status: StatusInProgress, branch: "feat/one"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/two"},
				{id: 3, status: StatusProposed, parent: parentEdge(2)},
			},
			branches: []string{"feat/one"},
			subject:  3,
			want:     EffectiveBase{Kind: BaseResolved, Branch: "feat/one"},
		},
		{
			name: "rule 6: missing parent",
			specs: []stackSpec{
				{id: 2, status: StatusProposed, parent: parentEdge(9)},
			},
			subject: 2,
			want:    EffectiveBase{Kind: BaseMissingParent, Cause: 9},
		},
		{
			name: "rule 6: malformed parent edge",
			specs: []stackSpec{
				{id: 2, status: StatusProposed, parent: OptionalInt{State: FieldMalformed, Raw: "later"}},
			},
			subject: 2,
			want:    EffectiveBase{Kind: BaseMalformedEdge, Cause: 2},
		},
		{
			name: "rule 6: live parent with no remote branch",
			specs: []stackSpec{
				{id: 1, status: StatusInProgress, branch: "feat/one"},
				{id: 2, status: StatusProposed, parent: parentEdge(1)},
			},
			subject: 2,
			want:    EffectiveBase{Kind: BaseBranchAbsent, Cause: 1},
		},
		{
			name: "rule 6: live parent with no recorded branch at all",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented},
				{id: 2, status: StatusProposed, parent: parentEdge(1)},
			},
			subject: 2,
			want:    EffectiveBase{Kind: BaseBranchAbsent, Cause: 1},
		},
		{
			name: "rule 6: cycle",
			specs: []stackSpec{
				{id: 1, status: StatusStackedMerged, parent: parentEdge(2)},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1)},
			},
			subject: 1,
			want:    EffectiveBase{Kind: BaseCycle, Cause: 1},
		},
		{
			name: "rule 6: self-cycle",
			specs: []stackSpec{
				{id: 1, status: StatusProposed, parent: parentEdge(1)},
			},
			subject: 1,
			want:    EffectiveBase{Kind: BaseCycle, Cause: 1},
		},
		{
			name: "an ambiguous parent reference is unresolvable",
			specs: []stackSpec{
				{id: 1, status: StatusInProgress, branch: "feat/one"},
				{id: 1, status: StatusInProgress, branch: "feat/one-dup"},
				{id: 2, status: StatusProposed, parent: parentEdge(1)},
			},
			branches: []string{"feat/one"},
			subject:  2,
			want:     EffectiveBase{Kind: BaseMissingParent, Cause: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := stackSnapshot(tt.specs...)
			got := resolveOf(t, s, tt.subject, remotes(tt.branches...))
			if got != tt.want {
				t.Fatalf("ResolveEffectiveBase = %+v; want %+v", got, tt.want)
			}
		})
	}
}

// TestResolveEffectiveBaseDoneParentAboveKilledGrandparent is the ADR-0092
// discriminating test: rule 3 is terminal, so the killed ancestor above the
// done parent is never reached.
func TestResolveEffectiveBaseDoneParentAboveKilledGrandparent(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusKilled, branch: "feat/one"},
		stackSpec{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/two"},
		stackSpec{id: 3, status: StatusProposed, parent: parentEdge(2)},
	)

	got := resolveOf(t, s, 3, remotes("feat/one", "feat/two"))

	want := EffectiveBase{Kind: BaseResolved, Branch: "main"}
	if got != want {
		t.Fatalf("ResolveEffectiveBase = %+v; want %+v (ADR-0092: done is terminal)", got, want)
	}
}

func TestResolveEffectiveBaseRecursionThroughTwoStackedMergedParents(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusInProgress, branch: "feat/one"},
		stackSpec{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/two"},
		stackSpec{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/three"},
		stackSpec{id: 4, status: StatusProposed, parent: parentEdge(3)},
	)

	got := resolveOf(t, s, 4, remotes("feat/one"))

	want := EffectiveBase{Kind: BaseResolved, Branch: "feat/one"}
	if got != want {
		t.Fatalf("ResolveEffectiveBase = %+v; want %+v", got, want)
	}
}

func TestResolveEffectiveBaseRecursionReachingKilledAncestor(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusKilled, branch: "feat/one"},
		stackSpec{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/two"},
		stackSpec{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/three"},
		stackSpec{id: 4, status: StatusProposed, parent: parentEdge(3)},
	)

	got := resolveOf(t, s, 4, remotes("feat/one"))

	want := EffectiveBase{Kind: BaseParentKilled, Cause: 1}
	if got != want {
		t.Fatalf("ResolveEffectiveBase = %+v; want %+v", got, want)
	}
}

func TestResolveEffectiveBaseRecursionReachingUnstackedMergedAncestor(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusStackedMerged, branch: "feat/one"},
		stackSpec{id: 2, status: StatusProposed, parent: parentEdge(1)},
	)

	got := resolveOf(t, s, 2, remotes())

	want := EffectiveBase{Kind: BaseResolved, Branch: "main"}
	if got != want {
		t.Fatalf("ResolveEffectiveBase = %+v; want %+v", got, want)
	}
}

func TestResolveEffectiveBaseRecursionTerminatesOnCycle(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusStackedMerged, parent: parentEdge(2)},
		stackSpec{id: 2, status: StatusStackedMerged, parent: parentEdge(1)},
		stackSpec{id: 3, status: StatusProposed, parent: parentEdge(1)},
	)

	got := resolveOf(t, s, 3, remotes())

	if got.Kind != BaseCycle {
		t.Fatalf("ResolveEffectiveBase = %+v; want kind %q", got, BaseCycle)
	}
}

func TestBranchFactsAreCopied(t *testing.T) {
	set := map[string]bool{"feat/one": true}
	facts := NewBranchFacts(set)
	delete(set, "feat/one")

	if !facts.HasBranch("feat/one") {
		t.Fatal("BranchFacts followed a post-construction mutation of the caller's map")
	}
	if facts.HasBranch("feat/absent") {
		t.Fatal("HasBranch reported an unknown branch as present")
	}
}

func TestStackParent(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusProposed},
		stackSpec{id: 2, status: StatusProposed, parent: parentEdge(1)},
		stackSpec{id: 3, status: StatusProposed, parent: parentEdge(9)},
		stackSpec{id: 4, status: StatusProposed, parent: OptionalInt{State: FieldMalformed, Raw: "x"}},
	)

	tests := []struct {
		id      ChangeID
		wantID  ChangeID
		wantOut LookupOutcome
	}{
		{id: 2, wantID: 1, wantOut: LookupFound},
		{id: 1, wantOut: LookupAbsent},
		{id: 3, wantOut: LookupAbsent},
		{id: 4, wantOut: LookupAbsent},
	}
	for _, tt := range tests {
		c, _ := s.Change(tt.id)
		parent, out := StackParent(s, c)
		if out != tt.wantOut {
			t.Fatalf("StackParent(%d) outcome = %v; want %v", tt.id, out, tt.wantOut)
		}
		if out == LookupFound && parent.ID() != tt.wantID {
			t.Fatalf("StackParent(%d) = %d; want %d", tt.id, parent.ID(), tt.wantID)
		}
	}
}

func TestStackAncestorsNearestFirst(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusProposed},
		stackSpec{id: 2, status: StatusProposed, parent: parentEdge(1)},
		stackSpec{id: 3, status: StatusProposed, parent: parentEdge(2)},
	)
	c, _ := s.Change(3)

	got := StackAncestors(s, c)

	want := []ChangeID{2, 1}
	if !slices.Equal(got, want) {
		t.Fatalf("StackAncestors = %v; want %v", got, want)
	}
}

func TestStackAncestorsStopsAtCycle(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusProposed, parent: parentEdge(2)},
		stackSpec{id: 2, status: StatusProposed, parent: parentEdge(1)},
		stackSpec{id: 3, status: StatusProposed, parent: parentEdge(1)},
	)
	c, _ := s.Change(3)

	got := StackAncestors(s, c)

	want := []ChangeID{1, 2}
	if !slices.Equal(got, want) {
		t.Fatalf("StackAncestors = %v; want %v (walk must stop at the cycle)", got, want)
	}
}

func TestStackAncestorsSelfCycle(t *testing.T) {
	s := stackSnapshot(stackSpec{id: 1, status: StatusProposed, parent: parentEdge(1)})
	c, _ := s.Change(1)

	if got := StackAncestors(s, c); len(got) != 0 {
		t.Fatalf("StackAncestors = %v; want empty for a self-cycle", got)
	}
}

func TestStackChildrenAscending(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusProposed},
		stackSpec{id: 5, status: StatusProposed, parent: parentEdge(1)},
		stackSpec{id: 3, status: StatusProposed, parent: parentEdge(1)},
		stackSpec{id: 4, status: StatusProposed, parent: parentEdge(3)},
	)

	got := StackChildren(s, 1)

	want := []ChangeID{3, 5}
	if !slices.Equal(got, want) {
		t.Fatalf("StackChildren = %v; want %v", got, want)
	}
	if got := StackChildren(s, 4); len(got) != 0 {
		t.Fatalf("StackChildren(4) = %v; want empty", got)
	}
}

func TestStackDescendantsParentFirst(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusProposed},
		stackSpec{id: 5, status: StatusProposed, parent: parentEdge(1)},
		stackSpec{id: 2, status: StatusProposed, parent: parentEdge(1)},
		stackSpec{id: 4, status: StatusProposed, parent: parentEdge(2)},
		stackSpec{id: 3, status: StatusProposed, parent: parentEdge(2)},
		stackSpec{id: 6, status: StatusProposed, parent: parentEdge(5)},
	)

	got := StackDescendantsParentFirst(s, 1)

	want := []ChangeID{2, 3, 4, 5, 6}
	if !slices.Equal(got, want) {
		t.Fatalf("StackDescendantsParentFirst = %v; want %v", got, want)
	}
}

func TestStackDescendantsParentFirstTerminatesOnCycle(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusProposed, parent: parentEdge(2)},
		stackSpec{id: 2, status: StatusProposed, parent: parentEdge(1)},
	)

	got := StackDescendantsParentFirst(s, 1)

	want := []ChangeID{2}
	if !slices.Equal(got, want) {
		t.Fatalf("StackDescendantsParentFirst = %v; want %v", got, want)
	}
}
