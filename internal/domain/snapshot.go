package domain

import "slices"

// SnapshotSpec carries the decoded repository state a Snapshot is built from.
// The repository layer fills it; NewSnapshot copies every slice, so the spec
// stays the caller's to reuse or mutate afterwards.
type SnapshotSpec struct {
	Policy       RepositoryPolicy
	Changes      []Change
	ADRs         []ADR
	Learnings    []Learning
	Artifacts    []Artifact
	DerivedViews []DerivedView
}

// LookupOutcome reports how an identifier resolved against a Snapshot.
type LookupOutcome int

// The closed set of lookup outcomes. LookupFound is the zero value.
const (
	LookupFound    LookupOutcome = iota // exactly one record carries the id
	LookupAbsent                        // no record carries the id
	LookupAmbiguous                     // more than one record carries the id — no winner is picked
)

// ambiguousIndex marks an id that more than one record claims. Picking a winner
// among duplicates would make the choice depend on scan order, so the index
// stores this sentinel instead and the lookup reports LookupAmbiguous.
const ambiguousIndex = -1

// Snapshot is an immutable view of repository state with ambiguity-aware
// lookups. Every accessor hands back a fresh slice or a copy, so a caller
// cannot reach the stored state.
type Snapshot struct {
	policy       RepositoryPolicy
	changes      []Change
	adrs         []ADR
	learnings    []Learning
	artifacts    []Artifact
	derivedViews []DerivedView

	changeIndex   map[ChangeID]int
	adrIndex      map[ADRID]int
	learningIndex map[string]int
}

// NewSnapshot copies s and builds the lookup indexes. Records keep their
// authored input order, duplicates included: the snapshot reports ambiguity
// rather than resolving it, and validation is what turns that into a finding.
func NewSnapshot(s SnapshotSpec) Snapshot {
	snap := Snapshot{
		policy:        NewRepositoryPolicy(s.Policy),
		changes:       slices.Clone(s.Changes),
		adrs:          slices.Clone(s.ADRs),
		learnings:     slices.Clone(s.Learnings),
		artifacts:     slices.Clone(s.Artifacts),
		derivedViews:  slices.Clone(s.DerivedViews),
		changeIndex:   make(map[ChangeID]int, len(s.Changes)),
		adrIndex:      make(map[ADRID]int, len(s.ADRs)),
		learningIndex: make(map[string]int, len(s.Learnings)),
	}
	for i, c := range snap.changes {
		index(snap.changeIndex, c.ID(), i)
	}
	for i, a := range snap.adrs {
		index(snap.adrIndex, a.ID(), i)
	}
	for i, l := range snap.learnings {
		index(snap.learningIndex, l.Slug(), i)
	}
	return snap
}

// index records position i for key k, demoting an already-seen key to the
// ambiguous sentinel.
func index[K comparable](m map[K]int, k K, i int) {
	if _, seen := m[k]; seen {
		m[k] = ambiguousIndex
		return
	}
	m[k] = i
}

// lookup resolves k against an index, returning the record's position and the
// outcome.
func lookup[K comparable](m map[K]int, k K) (int, LookupOutcome) {
	i, ok := m[k]
	switch {
	case !ok:
		return 0, LookupAbsent
	case i == ambiguousIndex:
		return 0, LookupAmbiguous
	default:
		return i, LookupFound
	}
}

// Change resolves id. The returned Change is meaningful only when the outcome
// is LookupFound; it is the zero value otherwise.
func (s Snapshot) Change(id ChangeID) (Change, LookupOutcome) {
	i, out := lookup(s.changeIndex, id)
	if out != LookupFound {
		return Change{}, out
	}
	return s.changes[i], LookupFound
}

// ADR resolves id, with the same zero-value contract as Change.
func (s Snapshot) ADR(id ADRID) (ADR, LookupOutcome) {
	i, out := lookup(s.adrIndex, id)
	if out != LookupFound {
		return ADR{}, out
	}
	return s.adrs[i], LookupFound
}

// Learning resolves slug, with the same zero-value contract as Change.
func (s Snapshot) Learning(slug string) (Learning, LookupOutcome) {
	i, out := lookup(s.learningIndex, slug)
	if out != LookupFound {
		return Learning{}, out
	}
	return s.learnings[i], LookupFound
}

// Changes returns every change in authored input order as a fresh slice.
func (s Snapshot) Changes() []Change { return slices.Clone(s.changes) }

// ADRs returns every ADR in authored input order as a fresh slice.
func (s Snapshot) ADRs() []ADR { return slices.Clone(s.adrs) }

// Learnings returns every learning in authored input order as a fresh slice.
func (s Snapshot) Learnings() []Learning { return slices.Clone(s.learnings) }

// Artifacts returns every artifact in authored input order as a fresh slice.
func (s Snapshot) Artifacts() []Artifact { return slices.Clone(s.artifacts) }

// DerivedViews returns every derived view in authored input order as a fresh
// slice.
func (s Snapshot) DerivedViews() []DerivedView { return slices.Clone(s.derivedViews) }

// Policy returns a deep copy of the repository policy.
func (s Snapshot) Policy() RepositoryPolicy { return NewRepositoryPolicy(s.policy) }
