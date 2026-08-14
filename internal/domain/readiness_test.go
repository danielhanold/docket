package domain

import "testing"

// readySpec is a compact description of one change for the readiness tests.
type readySpec struct {
	id           ChangeID
	status       Status
	spec         OptionalString
	trivial      bool
	dependsOn    []ChangeID
	parent       OptionalInt
	branch       string
	groomBlocked bool
}

// specRef is a present, non-empty spec: reference.
func specRef(path string) OptionalString {
	return OptionalString{State: FieldPresent, Value: path}
}

// build turns a readySpec into an immutable Change.
func (rp readySpec) build() Change {
	cs := ChangeSpec{
		ID:        rp.id,
		Status:    rp.status,
		Spec:      rp.spec,
		Trivial:   rp.trivial,
		DependsOn: rp.dependsOn,
		StackedOn: rp.parent,

		HasAutoGroomBlocked: rp.groomBlocked,
	}
	if rp.branch != "" {
		cs.Branch = OptionalString{State: FieldPresent, Value: rp.branch}
	}
	return NewChange(cs)
}

// readySnapshot builds a snapshot with "main" as the integration branch.
func readySnapshot(specs ...readySpec) Snapshot {
	changes := make([]Change, 0, len(specs))
	for _, rp := range specs {
		changes = append(changes, rp.build())
	}
	return NewSnapshot(SnapshotSpec{
		Policy:  RepositoryPolicy{IntegrationBranch: "main"},
		Changes: changes,
	})
}

func TestEvaluateReadinessPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		specs    []readySpec
		branches []string
		subject  ChangeID
		want     ReadinessKind
	}{
		{
			name:    "a non-proposed change is not-proposed regardless of design",
			specs:   []readySpec{{id: 2, status: StatusInProgress, spec: specRef("s.md")}},
			subject: 2,
			want:    ReadyNotProposed,
		},
		{
			name:    "done outranks everything else",
			specs:   []readySpec{{id: 2, status: StatusDone}},
			subject: 2,
			want:    ReadyNotProposed,
		},
		{
			name: "an ambiguous identity is invalid even when otherwise build-ready",
			specs: []readySpec{
				{id: 2, status: StatusProposed, spec: specRef("s.md")},
				{id: 2, status: StatusProposed, spec: specRef("s.md")},
			},
			subject: 2,
			want:    ReadyInvalid,
		},
		{
			name: "an unmet dependency outranks missing design",
			specs: []readySpec{
				{id: 1, status: StatusProposed},
				{id: 2, status: StatusProposed, dependsOn: []ChangeID{1}},
			},
			subject: 2,
			want:    ReadyWaitingDependency,
		},
		{
			name: "an implemented dependency is still unmet",
			specs: []readySpec{
				{id: 1, status: StatusImplemented},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), dependsOn: []ChangeID{1}},
			},
			subject: 2,
			want:    ReadyWaitingDependency,
		},
		{
			name:    "proposed, no spec, not trivial is needs-brainstorm",
			specs:   []readySpec{{id: 2, status: StatusProposed}},
			subject: 2,
			want:    ReadyNeedsBrainstorm,
		},
		{
			name:    "an empty spec field counts as no spec",
			specs:   []readySpec{{id: 2, status: StatusProposed, spec: OptionalString{State: FieldEmpty}}},
			subject: 2,
			want:    ReadyNeedsBrainstorm,
		},
		{
			name:    "the auto-groom-blocked marker distinguishes the missing-design case",
			specs:   []readySpec{{id: 2, status: StatusProposed, groomBlocked: true}},
			subject: 2,
			want:    ReadyAutoGroomBlocked,
		},
		{
			name:    "the marker is ignored once design exists",
			specs:   []readySpec{{id: 2, status: StatusProposed, spec: specRef("s.md"), groomBlocked: true}},
			subject: 2,
			want:    ReadyBuildReady,
		},
		{
			name:    "trivial with no spec is build-ready",
			specs:   []readySpec{{id: 2, status: StatusProposed, trivial: true}},
			subject: 2,
			want:    ReadyBuildReady,
		},
		{
			name: "spec'd with met dependencies, stacked on an unresolvable parent",
			specs: []readySpec{
				{id: 1, status: StatusKilled, branch: "feat/one"},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), parent: parentEdge(1)},
			},
			branches: []string{"feat/one"},
			subject:  2,
			want:     ReadyStackBaseUnresolved,
		},
		{
			name: "stack resolution is not consulted while a dependency is unmet",
			specs: []readySpec{
				{id: 1, status: StatusKilled},
				{id: 3, status: StatusProposed},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), parent: parentEdge(1), dependsOn: []ChangeID{3}},
			},
			subject: 2,
			want:    ReadyWaitingDependency,
		},
		{
			name: "stack resolution is not consulted while design is missing",
			specs: []readySpec{
				{id: 1, status: StatusKilled},
				{id: 2, status: StatusProposed, parent: parentEdge(1)},
			},
			subject: 2,
			want:    ReadyNeedsBrainstorm,
		},
		{
			name: "a resolved stack base leaves the change build-ready",
			specs: []readySpec{
				{id: 1, status: StatusInProgress, branch: "feat/one"},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), parent: parentEdge(1)},
			},
			branches: []string{"feat/one"},
			subject:  2,
			want:     ReadyBuildReady,
		},
		{
			name: "a done dependency is satisfied",
			specs: []readySpec{
				{id: 1, status: StatusDone},
				{id: 2, status: StatusProposed, spec: specRef("s.md"), dependsOn: []ChangeID{1}},
			},
			subject: 2,
			want:    ReadyBuildReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := readySnapshot(tt.specs...)
			c := tt.specs[len(tt.specs)-1].build()
			for _, rp := range tt.specs {
				if rp.id == tt.subject {
					c = rp.build()
					break
				}
			}
			got := EvaluateReadiness(s, c, remotes(tt.branches...))
			if got.Kind != tt.want {
				t.Fatalf("EvaluateReadiness(%d).Kind = %q; want %q", tt.subject, got.Kind, tt.want)
			}
		})
	}
}

func TestEvaluateReadinessPopulatesDependencyEvaluation(t *testing.T) {
	s := readySnapshot(
		readySpec{id: 1, status: StatusImplemented},
		readySpec{id: 3, status: StatusProposed},
		readySpec{id: 2, status: StatusProposed, spec: specRef("s.md"), dependsOn: []ChangeID{3, 1}},
	)
	c, _ := s.Change(2)

	got := EvaluateReadiness(s, c, remotes())
	if got.Kind != ReadyWaitingDependency {
		t.Fatalf("Kind = %q; want %q", got.Kind, ReadyWaitingDependency)
	}
	if got.Dependency.Satisfied {
		t.Fatal("Dependency.Satisfied = true; want false")
	}
	if len(got.Dependency.Unmet) != 2 {
		t.Fatalf("len(Dependency.Unmet) = %d; want 2", len(got.Dependency.Unmet))
	}
	if got.Dependency.Summary != DepNeedsMerge {
		t.Fatalf("Dependency.Summary = %q; want %q", got.Dependency.Summary, DepNeedsMerge)
	}
	if got.Dependency.Representative != 1 {
		t.Fatalf("Dependency.Representative = %d; want 1", got.Dependency.Representative)
	}
	if got.StackBase != (EffectiveBase{}) {
		t.Fatalf("StackBase = %+v; want the zero value, stack resolution being unconsulted", got.StackBase)
	}
}

func TestEvaluateReadinessPopulatesStackBaseWhenConsulted(t *testing.T) {
	s := readySnapshot(
		readySpec{id: 1, status: StatusKilled},
		readySpec{id: 2, status: StatusProposed, spec: specRef("s.md"), parent: parentEdge(1)},
		readySpec{id: 4, status: StatusInProgress, branch: "feat/four"},
		readySpec{id: 5, status: StatusProposed, trivial: true, parent: parentEdge(4)},
	)

	blocked, _ := s.Change(2)
	got := EvaluateReadiness(s, blocked, remotes())
	if got.Kind != ReadyStackBaseUnresolved {
		t.Fatalf("Kind = %q; want %q", got.Kind, ReadyStackBaseUnresolved)
	}
	if got.StackBase.Kind != BaseParentKilled || got.StackBase.Cause != 1 {
		t.Fatalf("StackBase = %+v; want parent-killed caused by 1", got.StackBase)
	}

	ready, _ := s.Change(5)
	got = EvaluateReadiness(s, ready, remotes("feat/four"))
	if got.Kind != ReadyBuildReady {
		t.Fatalf("Kind = %q; want %q", got.Kind, ReadyBuildReady)
	}
	if got.StackBase.Kind != BaseResolved || got.StackBase.Branch != "feat/four" {
		t.Fatalf("StackBase = %+v; want the resolved parent branch", got.StackBase)
	}
	if !got.Dependency.Satisfied {
		t.Fatal("Dependency.Satisfied = false; want true for a change with no dependencies")
	}
}

func TestNeedsDesign(t *testing.T) {
	tests := []struct {
		name string
		spec readySpec
		want bool
	}{
		{
			name: "proposed with no spec and not trivial needs design",
			spec: readySpec{id: 2, status: StatusProposed},
			want: true,
		},
		{
			name: "an unmet dependency does not suppress design-ahead",
			spec: readySpec{id: 2, status: StatusProposed, dependsOn: []ChangeID{1}},
			want: true,
		},
		{
			name: "an empty spec field counts as no spec",
			spec: readySpec{id: 2, status: StatusProposed, spec: OptionalString{State: FieldEmpty}},
			want: true,
		},
		{
			name: "the auto-groom-blocked marker does not change the predicate",
			spec: readySpec{id: 2, status: StatusProposed, groomBlocked: true},
			want: true,
		},
		{
			name: "a spec'd change needs no design",
			spec: readySpec{id: 2, status: StatusProposed, spec: specRef("s.md")},
			want: false,
		},
		{
			name: "a trivial change needs no design",
			spec: readySpec{id: 2, status: StatusProposed, trivial: true},
			want: false,
		},
		{
			name: "a non-proposed change needs no design",
			spec: readySpec{id: 2, status: StatusInProgress},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsDesign(tt.spec.build()); got != tt.want {
				t.Fatalf("NeedsDesign = %v; want %v", got, tt.want)
			}
		})
	}
}
