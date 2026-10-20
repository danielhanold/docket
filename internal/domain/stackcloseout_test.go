package domain

import (
	"slices"
	"testing"
)

// mergedInto builds PRFacts for a merged pull request whose destination branch
// is base. The exact number/revision are irrelevant to closeout derivation,
// which reads only state and base.
func mergedInto(base string) PRFacts {
	return PRFacts{
		Number:      "1",
		Revision:    "v1",
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

// carryFixture is one stack shape rooted at change 1 plus the live PR facts its
// carry derivation reads.
type carryFixture struct {
	name  string
	specs []stackSpec
	facts map[ChangeID]PRFacts
}

// nonCarriedChildFixtures are the shapes in which root 1's only child (2) is
// not carried: it is neither stacked-merged nor an implemented change whose PR
// merged into the root's recorded branch.
func nonCarriedChildFixtures() []carryFixture {
	child := func(name string, status Status, facts map[ChangeID]PRFacts) carryFixture {
		return carryFixture{
			name: name,
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: status, parent: parentEdge(1), branch: "feat/2"},
			},
			facts: facts,
		}
	}
	mergedFacts := func() map[ChangeID]PRFacts { return map[ChangeID]PRFacts{2: mergedInto("feat/1")} }
	return []carryFixture{
		child("in-progress child", StatusInProgress, mergedFacts()),
		child("proposed child", StatusProposed, mergedFacts()),
		child("blocked child", StatusBlocked, mergedFacts()),
		child("deferred child", StatusDeferred, mergedFacts()),
		child("killed child", StatusKilled, mergedFacts()),
		child("done child", StatusDone, mergedFacts()),
		child("implemented child with no PR", StatusImplemented, map[ChangeID]PRFacts{}),
		child("implemented child with an unmerged PR", StatusImplemented,
			map[ChangeID]PRFacts{2: {State: "closed", Number: "8"}}),
		child("implemented child merged into main", StatusImplemented,
			map[ChangeID]PRFacts{2: mergedInto("main")}),
		child("implemented child with an unknown-state probe", StatusImplemented,
			map[ChangeID]PRFacts{2: {State: "unknown", BaseRef: "feat/1"}}),
	}
}

// openChildStopsDescentFixtures stack a stacked-merged grandchild (3) merged
// into child 2's branch, beneath a child 2 that root 1 does not carry.
func openChildStopsDescentFixtures() []carryFixture {
	shape := func(name string, status Status, childFacts PRFacts) carryFixture {
		return carryFixture{
			name: name,
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: status, parent: parentEdge(1), branch: "feat/2"},
				{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
			},
			facts: map[ChangeID]PRFacts{2: childFacts, 3: mergedInto("feat/2")},
		}
	}
	return []carryFixture{
		shape("in-progress child", StatusInProgress, mergedInto("feat/1")),
		shape("killed child", StatusKilled, mergedInto("feat/1")),
		shape("implemented child merged into main", StatusImplemented, mergedInto("main")),
	}
}

// implementedMergedChildFixture is root 1 with an implemented child (2) whose
// PR merged into the root's recorded branch but whose own closeout never ran.
func implementedMergedChildFixture() carryFixture {
	return carryFixture{
		name: "implemented child merged into the root's branch",
		specs: []stackSpec{
			{id: 1, status: StatusImplemented, branch: "feat/1"},
			{id: 2, status: StatusImplemented, parent: parentEdge(1), branch: "feat/2"},
		},
		facts: map[ChangeID]PRFacts{2: mergedInto("feat/1")},
	}
}

// chainThroughImplementedMiddleFixture alternates implemented and
// stacked-merged links, each merged into its parent's recorded branch.
func chainThroughImplementedMiddleFixture() carryFixture {
	return carryFixture{
		name: "chain through an implemented middle",
		specs: []stackSpec{
			{id: 1, status: StatusImplemented, branch: "feat/1"},
			{id: 2, status: StatusImplemented, parent: parentEdge(1), branch: "feat/2"},
			{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
			{id: 4, status: StatusImplemented, parent: parentEdge(3), branch: "feat/4"},
		},
		facts: map[ChangeID]PRFacts{
			2: mergedInto("feat/1"),
			3: mergedInto("feat/2"),
			4: mergedInto("feat/3"),
		},
	}
}

func TestDeriveRootCloseoutSetNonCarriedChildrenDoNotBlock(t *testing.T) {
	for _, fx := range nonCarriedChildFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			set, fail := DeriveRootCloseoutSet(stackSnapshot(fx.specs...), 1, fx.facts)
			if fail != nil {
				t.Fatalf("policy failure %+v; want nil", fail)
			}
			if len(set) != 0 {
				t.Fatalf("carried set = %+v; want empty (a child that is not carried never blocks the root)", set)
			}
			if !RootCloseoutProven(set) {
				t.Fatalf("RootCloseoutProven = false; want true")
			}
		})
	}
}

func TestDeriveRootCloseoutSetOpenChildStopsDescent(t *testing.T) {
	// A stacked-merged grandchild merged into a child the root does not carry
	// is that child's to carry, not the root's: it is neither carried nor a
	// blocker for the root.
	for _, fx := range openChildStopsDescentFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			set, fail := DeriveRootCloseoutSet(stackSnapshot(fx.specs...), 1, fx.facts)
			if fail != nil {
				t.Fatalf("policy failure %+v; want nil", fail)
			}
			if len(set) != 0 {
				t.Fatalf("carried set = %+v; want empty (descent stops at a child that is not carried)", set)
			}
			if !RootCloseoutProven(set) {
				t.Fatalf("RootCloseoutProven = false; want true")
			}
		})
	}
}

func TestDeriveCarriedSetImplementedMergedChildIsCarried(t *testing.T) {
	fx := implementedMergedChildFixture()
	derivations := map[string]func(Snapshot, ChangeID, map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure){
		"DeriveCarriedSet":      DeriveCarriedSet,
		"DeriveRootCloseoutSet": DeriveRootCloseoutSet,
	}
	for name, derive := range derivations {
		t.Run(name, func(t *testing.T) {
			set, fail := derive(stackSnapshot(fx.specs...), 1, fx.facts)
			if fail != nil {
				t.Fatalf("policy failure %+v; want nil", fail)
			}
			if want := []ChangeID{2}; !slices.Equal(idsOf(set), want) {
				t.Fatalf("carried ids = %v; want %v", idsOf(set), want)
			}
			if got := proofOf(set, 2); got != "" {
				t.Fatalf("descendant 2 proof = %q; want proven (empty)", got)
			}
			if !RootCloseoutProven(set) {
				t.Fatalf("RootCloseoutProven = false; want true")
			}
		})
	}
}

func TestDeriveCarriedSetChainThroughImplementedMiddle(t *testing.T) {
	fx := chainThroughImplementedMiddleFixture()
	set, fail := DeriveCarriedSet(stackSnapshot(fx.specs...), 1, fx.facts)
	if fail != nil {
		t.Fatalf("policy failure %+v; want nil", fail)
	}
	if want := []ChangeID{2, 3, 4}; !slices.Equal(idsOf(set), want) {
		t.Fatalf("carried ids = %v; want %v", idsOf(set), want)
	}
	for _, d := range set {
		if d.Proof != "" {
			t.Fatalf("descendant %d proof = %q; want proven (empty)", d.ID, d.Proof)
		}
	}
	if !RootCloseoutProven(set) {
		t.Fatalf("RootCloseoutProven = false; want true")
	}
}

func TestDeriveCarriedSetImplementedChildUnderBranchlessParentIsNotCarried(t *testing.T) {
	// With no recorded parent branch there is no destination to have merged
	// into, so even a PR whose base is empty never matches.
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusImplemented},
		stackSpec{id: 2, status: StatusImplemented, parent: parentEdge(1), branch: "feat/2"},
	)
	set, fail := DeriveCarriedSet(s, 1, map[ChangeID]PRFacts{2: mergedInto("")})
	if fail != nil {
		t.Fatalf("policy failure %+v; want nil", fail)
	}
	if len(set) != 0 {
		t.Fatalf("carried set = %+v; want empty (no recorded parent branch)", set)
	}
}

func TestDeriveRootCloseoutSetMatchesCarriedSet(t *testing.T) {
	// The drift guard: root closeout and the live-branch boundaries (rebase,
	// publish, merge) answer "what does this parent carry" with one rule, so on
	// every non-cyclic shape the two derivations agree entry for entry.
	fixtures := []carryFixture{
		{
			name: "stacked-merged chain",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
				{id: 3, status: StatusStackedMerged, parent: parentEdge(2), branch: "feat/3"},
			},
			facts: map[ChangeID]PRFacts{2: mergedInto("feat/1"), 3: mergedInto("feat/2")},
		},
		{
			name: "stacked-merged child merged into the wrong branch",
			specs: []stackSpec{
				{id: 1, status: StatusImplemented, branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts: map[ChangeID]PRFacts{2: mergedInto("feat/wrong")},
		},
	}
	fixtures = append(fixtures, nonCarriedChildFixtures()...)
	fixtures = append(fixtures, openChildStopsDescentFixtures()...)
	fixtures = append(fixtures, implementedMergedChildFixture(), chainThroughImplementedMiddleFixture())

	proofsOf := func(set []CarriedDescendant) []string {
		out := make([]string, 0, len(set))
		for _, d := range set {
			out = append(out, d.Proof)
		}
		return out
	}
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			s := stackSnapshot(fx.specs...)
			root, rootFail := DeriveRootCloseoutSet(s, 1, fx.facts)
			carried, carriedFail := DeriveCarriedSet(s, 1, fx.facts)
			if rootFail != nil || carriedFail != nil {
				t.Fatalf("policy failures root=%+v carried=%+v; want nil", rootFail, carriedFail)
			}
			if !slices.Equal(idsOf(root), idsOf(carried)) || !slices.Equal(proofsOf(root), proofsOf(carried)) {
				t.Fatalf("DeriveRootCloseoutSet = %+v; DeriveCarriedSet = %+v; want equal", root, carried)
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
			// A killed or proposed child (2) is never carried, whatever its PR
			// facts, and is not descended, so its own stacked-merged grandchild
			// (3) never surfaces.
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

func TestProveDoneCarryTwoLevel(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusDone, branch: "feat/1"},
		stackSpec{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/2"},
	)
	facts := map[ChangeID]PRFacts{
		1: mergedInto("main"),
		2: mergedInto("feat/1"),
	}

	root, token := ProveDoneCarry(s, 2, facts, "main")
	if root != 1 || token != "" {
		t.Fatalf("ProveDoneCarry(2) = (%d, %q); want (1, \"\")", root, token)
	}
}

func TestProveDoneCarryThreeLevel(t *testing.T) {
	s := stackSnapshot(
		stackSpec{id: 1, status: StatusDone, branch: "feat/1"},
		stackSpec{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/2"},
		stackSpec{id: 3, status: StatusDone, parent: parentEdge(2), branch: "feat/3"},
	)
	facts := map[ChangeID]PRFacts{
		1: mergedInto("main"),
		2: mergedInto("feat/1"),
		3: mergedInto("feat/2"),
	}

	root, token := ProveDoneCarry(s, 3, facts, "main")
	if root != 1 || token != "" {
		t.Fatalf("ProveDoneCarry(3) = (%d, %q); want (1, \"\")", root, token)
	}
}

func TestProveDoneCarryRefusals(t *testing.T) {
	tests := []struct {
		name        string
		specs       []stackSpec
		facts       map[ChangeID]PRFacts
		integration string
		check       ChangeID
		reason      string
	}{
		{
			name: "child merged into a branch that is not its parent's is destination-mismatch",
			specs: []stackSpec{
				{id: 1, status: StatusDone, branch: "feat/1"},
				{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{1: mergedInto("main"), 2: mergedInto("feat/other")},
			check:  2,
			reason: "destination-mismatch",
		},
		{
			name: "a killed parent is killed-ancestor",
			specs: []stackSpec{
				{id: 1, status: StatusKilled, branch: "feat/1"},
				{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{1: mergedInto("main"), 2: mergedInto("feat/1")},
			check:  2,
			reason: "killed-ancestor",
		},
		{
			name: "missing child facts are pr-unknown, never a clean carry",
			specs: []stackSpec{
				{id: 1, status: StatusDone, branch: "feat/1"},
				{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{1: mergedInto("main")},
			check:  2,
			reason: "pr-unknown",
		},
		{
			// An ancestor link with no PR facts (a results-artifact, or no PR
			// reference) never names a root, so the walk runs off the top of
			// the stack.
			name: "a done parent with no facts and no parent of its own is chain-broken",
			specs: []stackSpec{
				{id: 1, status: StatusDone, branch: "feat/1"},
				{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{2: mergedInto("feat/1")},
			check:  2,
			reason: "chain-broken",
		},
		{
			name: "a stacked-merged parent (root not yet landed) is chain-broken",
			specs: []stackSpec{
				{id: 1, status: StatusStackedMerged, branch: "feat/1"},
				{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{1: mergedInto("main"), 2: mergedInto("feat/1")},
			check:  2,
			reason: "chain-broken",
		},
		{
			name: "a stacked-merged start is not-done",
			specs: []stackSpec{
				{id: 1, status: StatusDone, branch: "feat/1"},
				{id: 2, status: StatusStackedMerged, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:  map[ChangeID]PRFacts{1: mergedInto("main"), 2: mergedInto("feat/1")},
			check:  2,
			reason: "not-done",
		},
		{
			name: "a stacked_on cycle with no link merged into the integration branch is cycle",
			specs: []stackSpec{
				{id: 2, status: StatusDone, parent: parentEdge(3), branch: "feat/2"},
				{id: 3, status: StatusDone, parent: parentEdge(2), branch: "feat/3"},
			},
			facts:  map[ChangeID]PRFacts{2: mergedInto("feat/3"), 3: mergedInto("feat/2")},
			check:  3,
			reason: "cycle",
		},
		{
			name: "an absent start is chain-broken",
			specs: []stackSpec{
				{id: 1, status: StatusDone, branch: "feat/1"},
			},
			facts:  map[ChangeID]PRFacts{1: mergedInto("main")},
			check:  9,
			reason: "chain-broken",
		},
		{
			name: "an empty integration branch never names a root",
			specs: []stackSpec{
				{id: 1, status: StatusDone, branch: "feat/1"},
				{id: 2, status: StatusDone, parent: parentEdge(1), branch: "feat/2"},
			},
			facts:       map[ChangeID]PRFacts{1: mergedInto(""), 2: mergedInto("feat/1")},
			integration: "-",
			check:       2,
			reason:      "chain-broken",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := stackSnapshot(tc.specs...)
			integration := "main"
			if tc.integration == "-" {
				integration = ""
			}
			root, token := ProveDoneCarry(s, tc.check, tc.facts, integration)
			if token != tc.reason {
				t.Fatalf("ProveDoneCarry(%d) token = %q; want %q", tc.check, token, tc.reason)
			}
			if root != 0 {
				t.Fatalf("ProveDoneCarry(%d) root = %d on refusal; want zero", tc.check, root)
			}
		})
	}
}

func TestProveDoneCarryUnstackedChildIsNotCarried(t *testing.T) {
	// The caller consults the walk only when a done change's PR did not merge
	// into the integration branch; an unstacked change has no stack to carry
	// it, so the walk refuses rather than inventing a root.
	s := stackSnapshot(stackSpec{id: 4, status: StatusDone, branch: "feat/4"})
	facts := map[ChangeID]PRFacts{4: mergedInto("feat/elsewhere")}

	root, token := ProveDoneCarry(s, 4, facts, "main")
	if root != 0 || token != "chain-broken" {
		t.Fatalf("ProveDoneCarry(unstacked) = (%d, %q); want (0, \"chain-broken\")", root, token)
	}
}
