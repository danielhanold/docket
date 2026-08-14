package domain

import (
	"maps"
	"slices"
)

// BranchFacts carries the precomputed branch-existence facts effective-base
// resolution consults. Remote-ref discovery itself belongs to the Git layer;
// the domain only reads the resulting set.
type BranchFacts struct {
	remoteBranches map[string]bool
}

// NewBranchFacts returns BranchFacts holding a copy of remote, so a later
// mutation of the caller's map cannot change a resolution's outcome.
func NewBranchFacts(remote map[string]bool) BranchFacts {
	return BranchFacts{remoteBranches: maps.Clone(remote)}
}

// HasBranch reports whether name is present in the supplied remote-branch set.
// The zero BranchFacts knows no branches.
func (f BranchFacts) HasBranch(name string) bool {
	return name != "" && f.remoteBranches[name]
}

// RemoteBranches returns a fresh copy of the remote-branch set.
func (f BranchFacts) RemoteBranches() map[string]bool {
	return maps.Clone(f.remoteBranches)
}

// EffectiveBaseKind tags how a change's effective base resolved. Every
// non-resolution carries its own kind: the outcome is never collapsed into an
// empty branch name that a caller could mistake for the integration branch.
type EffectiveBaseKind string

// The closed set of effective-base outcomes.
const (
	BaseResolved      EffectiveBaseKind = "resolved"
	BaseParentKilled  EffectiveBaseKind = "parent-killed"
	BaseMissingParent EffectiveBaseKind = "missing-parent"
	BaseCycle         EffectiveBaseKind = "cycle"
	BaseMalformedEdge EffectiveBaseKind = "malformed-edge"
	BaseBranchAbsent  EffectiveBaseKind = "branch-absent" // live parent, no remote branch
)

// EffectiveBase is the tagged result of resolving a change's base branch.
// Branch is meaningful only when Kind is BaseResolved. Cause names the exact
// ancestor the walk stopped at — the killed, missing, or branchless ancestor,
// the repeated ancestor that closed a cycle, or the change carrying the
// malformed edge — and is zero when Kind is BaseResolved.
type EffectiveBase struct {
	Kind   EffectiveBaseKind
	Branch string
	Cause  ChangeID
}

// ResolveEffectiveBase resolves the branch c should be based on, applying the
// retained rules in precedence order:
//
//  1. an unstacked change (no stacked_on edge, or an empty one) resolves to the
//     integration branch;
//  2. a killed parent returns parent-killed naming that exact ancestor, even
//     when a branch with its recorded name still exists;
//  3. a done parent resolves directly and terminally to the integration branch
//     — its code is reachable there — and the walk never recurses past it
//     (ADR-0092), so a killed ancestor above a done parent is never reached;
//  4. any other parent whose recorded branch is in facts resolves to that
//     branch;
//  5. a branchless stacked-merged parent recurses into its own effective base,
//     because its commits merged into its parent rather than the integration
//     branch; and
//  6. a missing or ambiguous parent, a cycle, a malformed parent edge, or any
//     other live parent with no supplied remote branch returns its own cause.
//
// The walk is iterative with a visited set, so a malformed or cyclic
// stacked_on chain terminates instead of recursing without bound.
func ResolveEffectiveBase(s Snapshot, c Change, facts BranchFacts) EffectiveBase {
	integration := EffectiveBase{Kind: BaseResolved, Branch: s.Policy().IntegrationBranch}
	visited := map[ChangeID]bool{c.ID(): true}

	for current := c; ; {
		edge := current.StackedOn()
		switch edge.State {
		case FieldAbsent, FieldEmpty:
			return integration // rule 1
		case FieldMalformed:
			return EffectiveBase{Kind: BaseMalformedEdge, Cause: current.ID()} // rule 6
		}

		parentID := ChangeID(edge.Value)
		if visited[parentID] {
			return EffectiveBase{Kind: BaseCycle, Cause: parentID} // rule 6
		}
		// An ambiguous reference is unresolvable for the same reason an absent
		// one is: no single record can be attributed to it.
		parent, out := s.Change(parentID)
		if out != LookupFound {
			return EffectiveBase{Kind: BaseMissingParent, Cause: parentID} // rule 6
		}
		visited[parentID] = true

		switch parent.Status() {
		case StatusKilled:
			return EffectiveBase{Kind: BaseParentKilled, Cause: parentID} // rule 2
		case StatusDone:
			return integration // rule 3 — terminal, never recursing further
		}

		if branch := parent.Branch(); branch.State == FieldPresent && facts.HasBranch(branch.Value) {
			return EffectiveBase{Kind: BaseResolved, Branch: branch.Value} // rule 4
		}
		if parent.Status() == StatusStackedMerged {
			current = parent // rule 5
			continue
		}
		return EffectiveBase{Kind: BaseBranchAbsent, Cause: parentID} // rule 6
	}
}

// StackParent resolves c's stack parent. The outcome is LookupAbsent whenever
// c carries no usable parent edge — unstacked, empty, or malformed — and
// whenever the referenced record does not exist; it is LookupAmbiguous when
// more than one record claims the referenced ID. The returned Change is
// meaningful only for LookupFound.
func StackParent(s Snapshot, c Change) (Change, LookupOutcome) {
	edge := c.StackedOn()
	if edge.State != FieldPresent {
		return Change{}, LookupAbsent
	}
	return s.Change(ChangeID(edge.Value))
}

// StackAncestors returns c's stack ancestors nearest first. The walk follows
// only edges that resolve to exactly one record, and stops at the first
// already-visited ancestor, so a cycle terminates rather than repeating.
func StackAncestors(s Snapshot, c Change) []ChangeID {
	var ancestors []ChangeID
	visited := map[ChangeID]bool{c.ID(): true}
	for current := c; ; {
		parent, out := StackParent(s, current)
		if out != LookupFound || visited[parent.ID()] {
			return ancestors
		}
		visited[parent.ID()] = true
		ancestors = append(ancestors, parent.ID())
		current = parent
	}
}

// StackChildren returns the IDs of the changes stacked directly on id, in
// ascending ID order. A change whose own ID is ambiguous is not a child of
// anything: the snapshot picks no winner among duplicates.
func StackChildren(s Snapshot, id ChangeID) []ChangeID {
	var children []ChangeID
	for _, c := range s.Changes() {
		if _, out := s.Change(c.ID()); out != LookupFound {
			continue
		}
		if parent, out := StackParent(s, c); out == LookupFound && parent.ID() == id {
			children = append(children, c.ID())
		}
	}
	slices.Sort(children)
	return children
}

// StackDescendantsParentFirst returns every change stacked transitively on id
// in preorder — each parent before its children, siblings in ascending ID
// order — excluding id itself. The walk is iterative with a visited set, so a
// cycle in stacked_on terminates and no node is emitted twice.
func StackDescendantsParentFirst(s Snapshot, id ChangeID) []ChangeID {
	var descendants []ChangeID
	visited := map[ChangeID]bool{id: true}
	pending := StackChildren(s, id)
	for len(pending) > 0 {
		next := pending[0]
		pending = pending[1:]
		if visited[next] {
			continue
		}
		visited[next] = true
		descendants = append(descendants, next)
		pending = append(slices.Clone(StackChildren(s, next)), pending...)
	}
	return descendants
}
