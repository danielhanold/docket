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
