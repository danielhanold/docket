package domain

import (
	"fmt"
	"slices"
	"testing"
)

// depChange builds a change with a status and an authored depends_on list.
func depChange(id ChangeID, status Status, deps ...ChangeID) Change {
	return NewChange(ChangeSpec{ID: id, Status: status, DependsOn: deps})
}

// stackChange builds a change whose stacked_on edge points at parent.
func stackChange(id ChangeID, parent ChangeID) Change {
	return NewChange(ChangeSpec{
		ID:        id,
		StackedOn: OptionalInt{State: FieldPresent, Value: int(parent)},
	})
}

// snapshotOf builds a snapshot carrying only the supplied changes.
func snapshotOf(changes ...Change) Snapshot {
	return NewSnapshot(SnapshotSpec{Changes: changes})
}

func TestEvaluateDependenciesSatisfiedOnlyWhenDone(t *testing.T) {
	nonDone := []Status{
		StatusProposed, StatusInProgress, StatusBlocked, StatusDeferred,
		StatusStackedMerged, StatusKilled,
	}
	for _, st := range nonDone {
		t.Run(string(st), func(t *testing.T) {
			s := snapshotOf(depChange(1, st), depChange(2, StatusProposed, 1))
			c, _ := s.Change(2)

			got := EvaluateDependencies(s, c)

			if got.Satisfied {
				t.Fatalf("dependency on %s reported satisfied", st)
			}
			want := []UnmetDependency{{ID: 1, Reason: DepNotBuilt}}
			if !slices.Equal(got.Unmet, want) {
				t.Fatalf("Unmet = %+v; want %+v", got.Unmet, want)
			}
			if got.Summary != DepNotBuilt || got.Representative != 1 {
				t.Fatalf("Summary/Representative = %q/%d; want not-built/1", got.Summary, got.Representative)
			}
		})
	}
}

func TestEvaluateDependenciesDoneIsSatisfied(t *testing.T) {
	s := snapshotOf(depChange(1, StatusDone), depChange(2, StatusProposed, 1))
	c, _ := s.Change(2)

	got := EvaluateDependencies(s, c)

	if !got.Satisfied || len(got.Unmet) != 0 {
		t.Fatalf("EvaluateDependencies = %+v; want satisfied with no unmet", got)
	}
	if got.Summary != "" || got.Representative != 0 {
		t.Fatalf("Summary/Representative = %q/%d; want zero values when satisfied", got.Summary, got.Representative)
	}
}

func TestEvaluateDependenciesNoDependenciesIsSatisfied(t *testing.T) {
	s := snapshotOf(depChange(2, StatusProposed))
	c, _ := s.Change(2)

	if got := EvaluateDependencies(s, c); !got.Satisfied {
		t.Fatalf("EvaluateDependencies = %+v; want satisfied", got)
	}
}

func TestEvaluateDependenciesImplementedNeedsMerge(t *testing.T) {
	s := snapshotOf(depChange(1, StatusImplemented), depChange(2, StatusProposed, 1))
	c, _ := s.Change(2)

	got := EvaluateDependencies(s, c)

	want := []UnmetDependency{{ID: 1, Reason: DepNeedsMerge}}
	if got.Satisfied || !slices.Equal(got.Unmet, want) {
		t.Fatalf("Unmet = %+v (satisfied=%v); want %+v", got.Unmet, got.Satisfied, want)
	}
	if got.Summary != DepNeedsMerge || got.Representative != 1 {
		t.Fatalf("Summary/Representative = %q/%d; want needs-merge/1", got.Summary, got.Representative)
	}
}

func TestEvaluateDependenciesMissingReferenceIsNotBuiltAndMissing(t *testing.T) {
	s := snapshotOf(depChange(2, StatusProposed, 99))
	c, _ := s.Change(2)

	got := EvaluateDependencies(s, c)

	want := []UnmetDependency{{ID: 99, Reason: DepNotBuilt, Missing: true}}
	if !slices.Equal(got.Unmet, want) {
		t.Fatalf("Unmet = %+v; want %+v", got.Unmet, want)
	}
}

func TestEvaluateDependenciesAmbiguousReferenceIsNotBuiltWithoutMissing(t *testing.T) {
	// Two records claim id 1, so the lookup reports ambiguity and no winner is
	// picked: the dependency is not built, but it is not absent either.
	s := snapshotOf(
		depChange(1, StatusDone),
		depChange(1, StatusDone),
		depChange(2, StatusProposed, 1),
	)
	c, _ := s.Change(2)

	got := EvaluateDependencies(s, c)

	want := []UnmetDependency{{ID: 1, Reason: DepNotBuilt, Missing: false}}
	if !slices.Equal(got.Unmet, want) {
		t.Fatalf("Unmet = %+v; want %+v", got.Unmet, want)
	}
}

func TestEvaluateDependenciesNeedsMergeOutranksNotBuilt(t *testing.T) {
	// Authored order puts the missing (not-built) dependency first, but
	// needs-merge outranks it for the summary and the representative.
	s := snapshotOf(depChange(1, StatusImplemented), depChange(2, StatusProposed, 99, 1))
	c, _ := s.Change(2)

	got := EvaluateDependencies(s, c)

	want := []UnmetDependency{
		{ID: 99, Reason: DepNotBuilt, Missing: true},
		{ID: 1, Reason: DepNeedsMerge},
	}
	if !slices.Equal(got.Unmet, want) {
		t.Fatalf("Unmet = %+v; want %+v (authored order)", got.Unmet, want)
	}
	if got.Summary != DepNeedsMerge || got.Representative != 1 {
		t.Fatalf("Summary/Representative = %q/%d; want needs-merge/1", got.Summary, got.Representative)
	}
}

func TestEvaluateDependenciesRepresentativeIsFirstInAuthoredOrder(t *testing.T) {
	s := snapshotOf(
		depChange(5, StatusProposed),
		depChange(3, StatusProposed),
		depChange(9, StatusImplemented),
		depChange(7, StatusImplemented),
		depChange(2, StatusProposed, 5, 3),
	)
	c, _ := s.Change(2)

	if got := EvaluateDependencies(s, c); got.Representative != 5 {
		t.Fatalf("Representative = %d; want 5 (first not-built in authored order)", got.Representative)
	}

	s2 := snapshotOf(
		depChange(9, StatusImplemented),
		depChange(7, StatusImplemented),
		depChange(2, StatusProposed, 9, 7),
	)
	c2, _ := s2.Change(2)

	if got := EvaluateDependencies(s2, c2); got.Representative != 9 {
		t.Fatalf("Representative = %d; want 9 (first needs-merge in authored order)", got.Representative)
	}
}

func TestEvaluateDependenciesResultDoesNotAliasCallerState(t *testing.T) {
	s := snapshotOf(depChange(2, StatusProposed, 99))
	c, _ := s.Change(2)

	got := EvaluateDependencies(s, c)
	got.Unmet[0].ID = 12345

	if again := EvaluateDependencies(s, c); again.Unmet[0].ID != 99 {
		t.Fatalf("second evaluation saw %d; want 99 — result aliases stored state", again.Unmet[0].ID)
	}
}

// cycleStrings renders cycles as comparable strings for assertion.
func cycleStrings(cycles []Cycle) []string {
	out := make([]string, 0, len(cycles))
	for _, c := range cycles {
		out = append(out, fmt.Sprint(c.Members))
	}
	return out
}

func TestDependencyCyclesSelfCycle(t *testing.T) {
	s := snapshotOf(depChange(1, StatusProposed, 1))

	got := cycleStrings(DependencyCycles(s))

	want := []string{"[1]"}
	if !slices.Equal(got, want) {
		t.Fatalf("DependencyCycles = %v; want %v", got, want)
	}
}

func TestDependencyCyclesTwoNode(t *testing.T) {
	s := snapshotOf(depChange(2, StatusProposed, 1), depChange(1, StatusProposed, 2))

	got := cycleStrings(DependencyCycles(s))

	want := []string{"[1 2]"} // rotation-normalized to start at the smallest ID
	if !slices.Equal(got, want) {
		t.Fatalf("DependencyCycles = %v; want %v", got, want)
	}
}

func TestDependencyCyclesThreeNodeWithTail(t *testing.T) {
	// 4 -> 2 -> 3 -> 1 -> 2 : the cycle is 2,3,1 and 4 is only a tail into it.
	s := snapshotOf(
		depChange(4, StatusProposed, 2),
		depChange(2, StatusProposed, 3),
		depChange(3, StatusProposed, 1),
		depChange(1, StatusProposed, 2),
	)

	got := cycleStrings(DependencyCycles(s))

	want := []string{"[1 2 3]"}
	if !slices.Equal(got, want) {
		t.Fatalf("DependencyCycles = %v; want %v", got, want)
	}
}

func TestDependencyCyclesDisjointCyclesReportedSeparatelyAndDeterministically(t *testing.T) {
	s := snapshotOf(
		depChange(7, StatusProposed, 8),
		depChange(8, StatusProposed, 7),
		depChange(2, StatusProposed, 3),
		depChange(3, StatusProposed, 2),
		depChange(5, StatusProposed, 5),
		depChange(9, StatusProposed), // acyclic bystander
	)

	got := cycleStrings(DependencyCycles(s))

	want := []string{"[2 3]", "[5]", "[7 8]"}
	if !slices.Equal(got, want) {
		t.Fatalf("DependencyCycles = %v; want %v", got, want)
	}
}

func TestDependencyCyclesOrdersByFirstMemberThenLength(t *testing.T) {
	// 1 -> 2 -> 1 and 1 -> 3 -> 4 -> 1 share the smallest member.
	s := snapshotOf(
		depChange(1, StatusProposed, 2, 3),
		depChange(2, StatusProposed, 1),
		depChange(3, StatusProposed, 4),
		depChange(4, StatusProposed, 1),
	)

	got := cycleStrings(DependencyCycles(s))

	want := []string{"[1 2]", "[1 3 4]"}
	if !slices.Equal(got, want) {
		t.Fatalf("DependencyCycles = %v; want %v", got, want)
	}
}

func TestDependencyCyclesNoneOnAcyclicGraph(t *testing.T) {
	s := snapshotOf(
		depChange(1, StatusDone),
		depChange(2, StatusProposed, 1),
		depChange(3, StatusProposed, 1, 2),
	)

	if got := DependencyCycles(s); len(got) != 0 {
		t.Fatalf("DependencyCycles = %v; want none", cycleStrings(got))
	}
}

func TestDependencyCyclesIgnoreDanglingAndAmbiguousEdges(t *testing.T) {
	// 1 -> 99 dangles; ids 4 and 4 are ambiguous, so neither their edges nor
	// edges into them can be attributed to a single record.
	s := snapshotOf(
		depChange(1, StatusProposed, 99),
		depChange(4, StatusProposed, 4),
		depChange(4, StatusProposed, 4),
	)

	if got := DependencyCycles(s); len(got) != 0 {
		t.Fatalf("DependencyCycles = %v; want none", cycleStrings(got))
	}
}

func TestDependencyCyclesResultDoesNotAliasStoredState(t *testing.T) {
	s := snapshotOf(depChange(2, StatusProposed, 1), depChange(1, StatusProposed, 2))

	got := DependencyCycles(s)
	got[0].Members[0] = 999

	if again := cycleStrings(DependencyCycles(s)); again[0] != "[1 2]" {
		t.Fatalf("second call saw %v; want [1 2]", again)
	}
}

func TestStackCyclesOverStackedOnEdges(t *testing.T) {
	s := snapshotOf(
		stackChange(3, 4),
		stackChange(4, 3),
		stackChange(6, 6),
		NewChange(ChangeSpec{ID: 8}), // no stacked_on at all
		NewChange(ChangeSpec{ID: 9, StackedOn: OptionalInt{State: FieldMalformed, Raw: "9"}}),
	)

	got := cycleStrings(StackCycles(s))

	want := []string{"[3 4]", "[6]"}
	if !slices.Equal(got, want) {
		t.Fatalf("StackCycles = %v; want %v", got, want)
	}
}

func TestStackCyclesIgnoreDependsOnEdges(t *testing.T) {
	s := snapshotOf(depChange(1, StatusProposed, 2), depChange(2, StatusProposed, 1))

	if got := StackCycles(s); len(got) != 0 {
		t.Fatalf("StackCycles = %v; want none — depends_on is a separate graph", cycleStrings(got))
	}
}

func TestDependencyCyclesIgnoreStackedOnEdges(t *testing.T) {
	s := snapshotOf(stackChange(1, 2), stackChange(2, 1))

	if got := DependencyCycles(s); len(got) != 0 {
		t.Fatalf("DependencyCycles = %v; want none — stacked_on is a separate graph", cycleStrings(got))
	}
}

func TestStackCyclesChainIntoCycleReportsOnlyTheCycle(t *testing.T) {
	// 5 -> 4 -> 3 -> 2 -> 3 : the tail 5,4 is not part of the cycle.
	s := snapshotOf(stackChange(5, 4), stackChange(4, 3), stackChange(3, 2), stackChange(2, 3))

	got := cycleStrings(StackCycles(s))

	want := []string{"[2 3]"}
	if !slices.Equal(got, want) {
		t.Fatalf("StackCycles = %v; want %v", got, want)
	}
}
