package domain

import "testing"

func testSnapshot() Snapshot {
	return NewSnapshot(SnapshotSpec{
		Policy: RepositoryPolicy{IntegrationBranch: "main", ChangeTypes: []string{"feat", "fix"}},
		Changes: []Change{
			NewChange(ChangeSpec{ID: 3, Slug: "three"}),
			NewChange(ChangeSpec{ID: 1, Slug: "one"}),
		},
		ADRs: []ADR{
			NewADR(ADRSpec{ID: 7, Slug: "seven"}),
			NewADR(ADRSpec{ID: 2, Slug: "two"}),
		},
		Learnings: []Learning{
			NewLearning(LearningSpec{Slug: "beta"}),
			NewLearning(LearningSpec{Slug: "alpha"}),
		},
		Artifacts:    []Artifact{NewArtifact(ArtifactSpec{Path: "a.md"})},
		DerivedViews: []DerivedView{NewDerivedView(DerivedViewSpec{Path: "BOARD.md"})},
	})
}

func TestSnapshotChangeLookup(t *testing.T) {
	s := testSnapshot()

	c, out := s.Change(1)
	if out != LookupFound || c.Slug() != "one" {
		t.Fatalf("Change(1) = %q, %v; want \"one\", LookupFound", c.Slug(), out)
	}
	if _, out := s.Change(99); out != LookupAbsent {
		t.Fatalf("Change(99) outcome = %v; want LookupAbsent", out)
	}
}

func TestSnapshotADRLookup(t *testing.T) {
	s := testSnapshot()

	a, out := s.ADR(7)
	if out != LookupFound || a.Slug() != "seven" {
		t.Fatalf("ADR(7) = %q, %v; want \"seven\", LookupFound", a.Slug(), out)
	}
	if _, out := s.ADR(99); out != LookupAbsent {
		t.Fatalf("ADR(99) outcome = %v; want LookupAbsent", out)
	}
}

func TestSnapshotLearningLookup(t *testing.T) {
	s := testSnapshot()

	l, out := s.Learning("alpha")
	if out != LookupFound || l.Slug() != "alpha" {
		t.Fatalf("Learning(alpha) = %q, %v; want \"alpha\", LookupFound", l.Slug(), out)
	}
	if _, out := s.Learning("nope"); out != LookupAbsent {
		t.Fatalf("Learning(nope) outcome = %v; want LookupAbsent", out)
	}
}

func TestSnapshotAmbiguousLookups(t *testing.T) {
	s := NewSnapshot(SnapshotSpec{
		Changes: []Change{
			NewChange(ChangeSpec{ID: 1, Slug: "first"}),
			NewChange(ChangeSpec{ID: 1, Slug: "second"}),
		},
		ADRs: []ADR{
			NewADR(ADRSpec{ID: 4, Slug: "first"}),
			NewADR(ADRSpec{ID: 4, Slug: "second"}),
		},
		Learnings: []Learning{
			NewLearning(LearningSpec{Slug: "dup", Hook: "first"}),
			NewLearning(LearningSpec{Slug: "dup", Hook: "second"}),
		},
	})

	if c, out := s.Change(1); out != LookupAmbiguous || c.Slug() != "" {
		t.Fatalf("Change(1) = %q, %v; want zero Change, LookupAmbiguous", c.Slug(), out)
	}
	if a, out := s.ADR(4); out != LookupAmbiguous || a.Slug() != "" {
		t.Fatalf("ADR(4) = %q, %v; want zero ADR, LookupAmbiguous", a.Slug(), out)
	}
	if l, out := s.Learning("dup"); out != LookupAmbiguous || l.Hook() != "" {
		t.Fatalf("Learning(dup) = %q, %v; want zero Learning, LookupAmbiguous", l.Hook(), out)
	}

	// Both duplicates stay in the collection — no winner is picked, nothing is dropped.
	if got := s.Changes(); len(got) != 2 || got[0].Slug() != "first" || got[1].Slug() != "second" {
		t.Fatalf("Changes() = %v; want both duplicates in input order", got)
	}
	if got := s.ADRs(); len(got) != 2 {
		t.Fatalf("ADRs() length = %d; want 2", len(got))
	}
	if got := s.Learnings(); len(got) != 2 {
		t.Fatalf("Learnings() length = %d; want 2", len(got))
	}
}

func TestSnapshotCollectionsPreserveInputOrder(t *testing.T) {
	s := testSnapshot()

	if got := s.Changes(); len(got) != 2 || got[0].ID() != 3 || got[1].ID() != 1 {
		t.Fatalf("Changes() ids = %v; want authored order [3 1]", got)
	}
	if got := s.ADRs(); len(got) != 2 || got[0].ID() != 7 || got[1].ID() != 2 {
		t.Fatalf("ADRs() ids = %v; want authored order [7 2]", got)
	}
	if got := s.Learnings(); len(got) != 2 || got[0].Slug() != "beta" || got[1].Slug() != "alpha" {
		t.Fatalf("Learnings() = %v; want authored order [beta alpha]", got)
	}
	if got := s.Artifacts(); len(got) != 1 || got[0].Path() != "a.md" {
		t.Fatalf("Artifacts() = %v; want [a.md]", got)
	}
	if got := s.DerivedViews(); len(got) != 1 || got[0].Path() != "BOARD.md" {
		t.Fatalf("DerivedViews() = %v; want [BOARD.md]", got)
	}
	if got := s.Policy(); got.IntegrationBranch != "main" {
		t.Fatalf("Policy().IntegrationBranch = %q; want \"main\"", got.IntegrationBranch)
	}
}

func TestSnapshotAccessorsReturnFreshSlices(t *testing.T) {
	s := testSnapshot()

	s.Changes()[0] = NewChange(ChangeSpec{ID: 42})
	s.ADRs()[0] = NewADR(ADRSpec{ID: 42})
	s.Learnings()[0] = NewLearning(LearningSpec{Slug: "clobbered"})
	s.Artifacts()[0] = NewArtifact(ArtifactSpec{Path: "clobbered"})
	s.DerivedViews()[0] = NewDerivedView(DerivedViewSpec{Path: "clobbered"})
	s.Policy().ChangeTypes[0] = "clobbered"

	if got := s.Changes(); got[0].ID() != 3 {
		t.Fatalf("Changes()[0].ID() = %d after caller mutation; want 3", got[0].ID())
	}
	if got := s.ADRs(); got[0].ID() != 7 {
		t.Fatalf("ADRs()[0].ID() = %d after caller mutation; want 7", got[0].ID())
	}
	if got := s.Learnings(); got[0].Slug() != "beta" {
		t.Fatalf("Learnings()[0].Slug() = %q after caller mutation; want \"beta\"", got[0].Slug())
	}
	if got := s.Artifacts(); got[0].Path() != "a.md" {
		t.Fatalf("Artifacts()[0].Path() = %q after caller mutation; want \"a.md\"", got[0].Path())
	}
	if got := s.DerivedViews(); got[0].Path() != "BOARD.md" {
		t.Fatalf("DerivedViews()[0].Path() = %q after caller mutation; want \"BOARD.md\"", got[0].Path())
	}
	if got := s.Policy(); got.ChangeTypes[0] != "feat" {
		t.Fatalf("Policy().ChangeTypes[0] = %q after caller mutation; want \"feat\"", got.ChangeTypes[0])
	}
}

func TestSnapshotImmutable(t *testing.T) {
	spec := SnapshotSpec{
		Policy:       RepositoryPolicy{ChangeTypes: []string{"feat"}},
		Changes:      []Change{NewChange(ChangeSpec{ID: 1, Slug: "one"})},
		ADRs:         []ADR{NewADR(ADRSpec{ID: 1, Slug: "adr"})},
		Learnings:    []Learning{NewLearning(LearningSpec{Slug: "learn"})},
		Artifacts:    []Artifact{NewArtifact(ArtifactSpec{Path: "a.md"})},
		DerivedViews: []DerivedView{NewDerivedView(DerivedViewSpec{Path: "BOARD.md"})},
	}
	s := NewSnapshot(spec)

	spec.Changes[0] = NewChange(ChangeSpec{ID: 9, Slug: "mutated"})
	spec.ADRs[0] = NewADR(ADRSpec{ID: 9, Slug: "mutated"})
	spec.Learnings[0] = NewLearning(LearningSpec{Slug: "mutated"})
	spec.Artifacts[0] = NewArtifact(ArtifactSpec{Path: "mutated"})
	spec.DerivedViews[0] = NewDerivedView(DerivedViewSpec{Path: "mutated"})
	spec.Policy.ChangeTypes[0] = "mutated"

	if got := s.Changes(); got[0].Slug() != "one" {
		t.Fatalf("Changes()[0].Slug() = %q after spec mutation; want \"one\"", got[0].Slug())
	}
	if got := s.ADRs(); got[0].Slug() != "adr" {
		t.Fatalf("ADRs()[0].Slug() = %q after spec mutation; want \"adr\"", got[0].Slug())
	}
	if got := s.Learnings(); got[0].Slug() != "learn" {
		t.Fatalf("Learnings()[0].Slug() = %q after spec mutation; want \"learn\"", got[0].Slug())
	}
	if got := s.Artifacts(); got[0].Path() != "a.md" {
		t.Fatalf("Artifacts()[0].Path() = %q after spec mutation; want \"a.md\"", got[0].Path())
	}
	if got := s.DerivedViews(); got[0].Path() != "BOARD.md" {
		t.Fatalf("DerivedViews()[0].Path() = %q after spec mutation; want \"BOARD.md\"", got[0].Path())
	}
	if got := s.Policy(); got.ChangeTypes[0] != "feat" {
		t.Fatalf("Policy().ChangeTypes[0] = %q after spec mutation; want \"feat\"", got.ChangeTypes[0])
	}
	if c, out := s.Change(1); out != LookupFound || c.Slug() != "one" {
		t.Fatalf("Change(1) = %q, %v after spec mutation; want \"one\", LookupFound", c.Slug(), out)
	}
}
