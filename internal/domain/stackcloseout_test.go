package domain

import (
	"slices"
	"testing"
)

// mergedInto builds PRFacts for a merged pull request whose destination branch
// is base. The exact number/version are irrelevant to closeout derivation,
// which reads only state and base.
func mergedInto(base string) PRFacts {
	return PRFacts{
		Number:      "1",
		Version:     "v1",
		State:       "merged",
		HeadOID:     "head",
		BaseRef:     base,
		MergedAtUTC: "2026-08-18T00:00:00Z",
		MergeCommit: "mergecommit",
	}
}

// proofOf returns the derived proof token for id in set, or a sentinel when id
// is absent so a missing descendant fails loudly rather than reading as proven.
func proofOf(set []CarriedDescendant, id ChangeID) string {
	for _, d := range set {
		if d.ID == id {
			return d.Proof
		}
	}
	return "<absent>"
}

// idsOf projects the carried IDs in order.
func idsOf(set []CarriedDescendant) []ChangeID {
	out := make([]ChangeID, 0, len(set))
	for _, d := range set {
		out = append(out, d.ID)
	}
	return out
}

func TestDeriveRootCloseoutSetHappyChain(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusImplemented, branch: "feat/1"},
		stackSpec{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
		stackSpec{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
	)
	facts := map[ChangeID]PRFacts{
		2: mergedInto("feat/1"),
		3: mergedInto("feat/2"),
	}

	set, fail := DeriveRootCloseoutSet(s, 1, facts)
	if fail != nil {
		t.Fatalf("DeriveRootCloseoutSet returned policy failure %+v; want nil", fail)
	}
	if want := []ChangeID{2, 3}; !slices.Equal(idsOf(set), want) {
		t.Fatalf("carried ids = %v; want %v (parent-first)", idsOf(set), want)
	}
	for _, d := range set {
		if d.Proof != "" {
			t.Fatalf("descendant %d proof = %q; want proven", d.ID, d.Proof)
		}
	}
	if !RootCloseoutProven(set) {
		t.Fatalf("RootCloseoutProven = false; want true for a fully carried chain")
	}
}

func TestDeriveRootCloseoutSetRefusals(t *testing.T) {
	tests := []struct {
		name   string
		specs  []stackSpec
		facts  map[ChangeID]PRFacts
		root   ChangeID
		check  ChangeID
		reason string
	}{
		{
			name: "descendant still in-progress is not-stacked-merged",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusInProgress, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{2: mergedInto("feat/1")},
			root:   1,
			check:  2,
			reason: "not-stacked-merged",
		},
		{
			name: "merged into the wrong branch is destination-mismatch",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{2: mergedInto("feat/wrong")},
			root:   1,
			check:  2,
			reason: "destination-mismatch",
		},
		{
			name: "missing PR facts are pr-unknown",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{},
			root:   1,
			check:  2,
			reason: "pr-unknown",
		},
		{
			name: "an unknown-state probe is pr-unknown, never laundered clean",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{2: {State: "unknown", BaseRef: "feat/1"}},
			root:   1,
			check:  2,
			reason: "pr-unknown",
		},
		{
			name: "a gap mid-chain is chain-broken for the node below it",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusInProgress, parent: parentEdge(1), branch: "feat/2"},
				{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
			},
			facts: map[ChangeID]PRFacts{
				2: mergedInto("feat/1"),
				3: mergedInto("feat/2"),
			},
			root:   1,
			check:  3,
			reason: "chain-broken",
		},
		{
			name: "a cyclic stacked_on graph refuses with cycle and does not loop",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, parent: parentEdge(2), branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{2: mergedInto("feat/1")},
			root:   1,
			check:  2,
			reason: "cycle",
		},
		{
			name: "a killed ancestor mid-chain is killed-ancestor",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusKilled, parent: parentEdge(1), branch: "feat/2"},
				{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
			},
			facts: map[ChangeID]PRFacts{
				2: mergedInto("feat/1"),
				3: mergedInto("feat/2"),
			},
			root:   1,
			check:  3,
			reason: "killed-ancestor",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := stackSnapshot(tc.specs...)
			set, fail := DeriveRootCloseoutSet(s, tc.root, tc.facts)
			if fail != nil {
				t.Fatalf("policy failure %+v; want nil (per-descendant refusal expected)", fail)
			}
			if got := proofOf(set, tc.check); got != tc.reason {
				t.Fatalf("descendant %d proof = %q; want %q", tc.check, got, tc.reason)
			}
			if RootCloseoutProven(set) {
				t.Fatalf("RootCloseoutProven = true; want false when a descendant is unproven")
			}
		})
	}
}

func TestRootCloseoutProvenAllOrNothing(t *testing.T) {
	if !RootCloseoutProven(nil) {
		t.Fatalf("RootCloseoutProven(nil) = false; want true (vacuously proven)")
	}
	allProven := []CarriedDescendant{{ID: 2}, {ID: 3}}
	if !RootCloseoutProven(allProven) {
		t.Fatalf("RootCloseoutProven(all proven) = false; want true")
	}
	oneUnproven := []CarriedDescendant{{ID: 2}, {ID: 3, Proof: "destination-mismatch"}}
	if RootCloseoutProven(oneUnproven) {
		t.Fatalf("RootCloseoutProven(one unproven) = true; want false")
	}
}

func TestDeriveRootCloseoutSetIgnoresRenderedState(t *testing.T) {
	// Node 4 carries no stacked_on edge but a PR whose base is the root's
	// branch — a rendered-table or branch-name heuristic might promote it into
	// the carried set. The stacked_on graph is authoritative: only node 3, which
	// is genuinely stacked on the root, is a descendant.
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusImplemented, branch: "feat/1"},
		stackSpec{id: 3, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/3"},
		stackSpec{id: 4, status: StatusStackedMerged, branch: "feat/4"},
	)
	facts := map[ChangeID]PRFacts{
		3: mergedInto("feat/1"),
		4: mergedInto("feat/1"),
	}

	set, fail := DeriveRootCloseoutSet(s, 1, facts)
	if fail != nil {
		t.Fatalf("policy failure %+v; want nil", fail)
	}
	if want := []ChangeID{3}; !slices.Equal(idsOf(set), want) {
		t.Fatalf("carried ids = %v; want %v (graph wins, node 4 not promoted)", idsOf(set), want)
	}
	if !RootCloseoutProven(set) {
		t.Fatalf("RootCloseoutProven = false; want true")
	}
}

func TestDeriveRootCloseoutSetRootNotFound(t *testing.T) {
	s := stackSnapshot(stackSpec{id: 1, status: StatusImplemented, branch: "feat/1"})

	set, fail := DeriveRootCloseoutSet(s, 99, nil)
	if fail == nil {
		t.Fatalf("DeriveRootCloseoutSet(absent root) returned nil failure; want a policy failure")
	}
	if set != nil {
		t.Fatalf("carried set = %v; want nil for an unresolved root", set)
	}
}

func TestDeriveCarriedSetTransitiveChain(t *testing.T) {
	// parent(1) <- A(2, stacked-merged into feat/1) <- B(3, stacked-merged into
	// feat/2): the live parent branch promises to carry both, proven, in
	// parent-first order (spec Tests §6).
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusImplemented, branch: "feat/1"},
		stackSpec{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
		stackSpec{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
	)
	facts := map[ChangeID]PRFacts{
		2: mergedInto("feat/1"),
		3: mergedInto("feat/2"),
	}

	set, fail := DeriveCarriedSet(s, 1, facts)
	if fail != nil {
		t.Fatalf("DeriveCarriedSet returned policy failure %+v; want nil", fail)
	}
	if want := []ChangeID{2, 3}; !slices.Equal(idsOf(set), want) {
		t.Fatalf("carried ids = %v; want %v (both descendants promised)", idsOf(set), want)
	}
	for _, d := range set {
		if d.Proof != "" {
			t.Fatalf("descendant %d proof = %q; want proven (empty)", d.ID, d.Proof)
		}
	}
	if !RootCloseoutProven(set) {
		t.Fatalf("RootCloseoutProven = false; want true for a fully carried chain")
	}
}

func TestDeriveCarriedSetOpenIntermediateStopsDescent(t *testing.T) {
	// parent(1) <- A(2, in-progress) <- B(3, stacked-merged into A): B's code is
	// merged into a STILL-OPEN child, so the parent branch does not yet promise
	// it. A claims no carry (not stacked-merged) and is not descended, so the
	// set is empty (spec Tests §6).
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusImplemented, branch: "feat/1"},
		stackSpec{id: 2, status: StatusInProgress, parent: parentEdge(1), branch: "feat/2"},
		stackSpec{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
	)
	facts := map[ChangeID]PRFacts{
		2: mergedInto("feat/1"),
		3: mergedInto("feat/2"),
	}

	set, fail := DeriveCarriedSet(s, 1, facts)
	if fail != nil {
		t.Fatalf("policy failure %+v; want nil", fail)
	}
	if len(set) != 0 {
		t.Fatalf("carried set = %v; want empty (open intermediate blocks descent)", idsOf(set))
	}
}

func TestDeriveCarriedSetSurfacedRefusals(t *testing.T) {
	tests := []struct {
		name   string
		specs  []stackSpec
		facts  map[ChangeID]PRFacts
		parent ChangeID
		check  ChangeID
		reason string
	}{
		{
			name: "stacked-merged child with no facts is pr-unknown",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{},
			parent: 1,
			check:  2,
			reason: "pr-unknown",
		},
		{
			name: "merged into the wrong branch is destination-mismatch",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{2: mergedInto("feat/wrong")},
			parent: 1,
			check:  2,
			reason: "destination-mismatch",
		},
		{
			name: "an absent parent branch field is destination-mismatch",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{2: mergedInto("feat/1")},
			parent: 1,
			check:  2,
			reason: "destination-mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := stackSnapshot(tc.specs...)
			set, fail := DeriveCarriedSet(s, tc.parent, tc.facts)
			if fail != nil {
				t.Fatalf("policy failure %+v; want nil (per-descendant refusal expected)", fail)
			}
			if got := proofOf(set, tc.check); got != tc.reason {
				t.Fatalf("descendant %d proof = %q; want %q", tc.check, got, tc.reason)
			}
			if RootCloseoutProven(set) {
				t.Fatalf("RootCloseoutProven = true; want false when a descendant is unproven")
			}
		})
	}
}

func TestDeriveCarriedSetBrokenLinkNotDescended(t *testing.T) {
	// parent(1) <- A(2, stacked-merged, pr-unknown) <- B(3, stacked-merged into
	// A): A's carry cannot be confirmed, so it is surfaced with its token and NOT
	// descended — B never appears. The whole set already blocks.
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusImplemented, branch: "feat/1"},
		stackSpec{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
		stackSpec{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
	)
	facts := map[ChangeID]PRFacts{
		3: mergedInto("feat/2"), // A (2) has no facts -> pr-unknown; B (3) would prove if reached
	}

	set, fail := DeriveCarriedSet(s, 1, facts)
	if fail != nil {
		t.Fatalf("policy failure %+v; want nil", fail)
	}
	if got := proofOf(set, 2); got != "pr-unknown" {
		t.Fatalf("descendant 2 proof = %q; want %q", got, "pr-unknown")
	}
	if got := proofOf(set, 3); got != "<absent>" {
		t.Fatalf("descendant 3 proof = %q; want <absent> (a broken link is not descended)", got)
	}
	if RootCloseoutProven(set) {
		t.Fatalf("RootCloseoutProven = true; want false when a link is unproven")
	}
}

func TestDeriveCarriedSetCycleTerminates(t *testing.T) {
	// 1 <-> 2 mutual stacked_on: walking from parent(1) reaches 2, then 2's
	// child is 1 again — the revisited node reports cycle and derivation
	// terminates instead of looping.
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusImplemented, parent: parentEdge(2), branch: "feat/1"},
		stackSpec{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
	)
	facts := map[ChangeID]PRFacts{2: mergedInto("feat/1")}

	set, fail := DeriveCarriedSet(s, 1, facts)
	if fail != nil {
		t.Fatalf("policy failure %+v; want nil", fail)
	}
	if got := proofOf(set, 1); got != "cycle" {
		t.Fatalf("revisited node 1 proof = %q; want %q", got, "cycle")
	}
	if RootCloseoutProven(set) {
		t.Fatalf("RootCloseoutProven = true; want false when the graph cycles")
	}
}

func TestDeriveCarriedSetParentNotResolved(t *testing.T) {
	t.Run("absent parent", func(t *testing.T) {
		s := stackSnapshot(stackSpec{id: 1, status: StatusImplemented, branch: "feat/1"})
		set, fail := DeriveCarriedSet(s, 99, nil)
		if fail == nil {
			t.Fatalf("DeriveCarriedSet(absent parent) returned nil failure; want a policy failure")
		}
		if fail.Kind != FailInvalidInput {
			t.Fatalf("failure kind = %q; want %q", fail.Kind, FailInvalidInput)
		}
		if set != nil {
			t.Fatalf("carried set = %v; want nil for an unresolved parent", set)
		}
	})
	t.Run("ambiguous parent", func(t *testing.T) {
		s := stackSnapshot(
			stackSpec{id: 5, status: StatusImplemented, branch: "feat/5a"},
			stackSpec{id: 5, status: StatusStackedMerged, branch: "feat/5b"},
		)
		set, fail := DeriveCarriedSet(s, 5, nil)
		if fail == nil {
			t.Fatalf("DeriveCarriedSet(ambiguous parent) returned nil failure; want a policy failure")
		}
		if fail.Kind != FailInvalidInput {
			t.Fatalf("failure kind = %q; want %q", fail.Kind, FailInvalidInput)
		}
		if set != nil {
			t.Fatalf("carried set = %v; want nil for an ambiguous parent", set)
		}
	})
}

func TestDeriveCarriedSetKilledOrProposedChildContributesNothing(t *testing.T) {
	tests := []struct {
		name   string
		status Status
	}{
		{name: "killed child", status: StatusKilled},
		{name: "proposed child", status: StatusProposed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The non-stacked-merged child (2) claims no carry and is not
			// descended, so its own stacked-merged grandchild (3) never surfaces.
			s := stackSnapshot(
				stackSpec{id: 1, status: StatusImplemented, branch: "feat/1"},
				stackSpec{id: 2, status: tc.status, parent: parentEdge(1), branch: "feat/2"},
				stackSpec{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
			)
			facts := map[ChangeID]PRFacts{
				2: mergedInto("feat/1"),
				3: mergedInto("feat/2"),
			}

			set, fail := DeriveCarriedSet(s, 1, facts)
			if fail != nil {
				t.Fatalf("policy failure %+v; want nil", fail)
			}
			if len(set) != 0 {
				t.Fatalf("carried set = %v; want empty (a %s child carries nothing)", idsOf(set), tc.status)
			}
		})
	}
}
