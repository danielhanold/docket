package domain

import "slices"

// SelectionFilter narrows a selection queue. An empty list means "no filter on
// this dimension", never "match nothing". Both dimensions are matched against
// the change's STORED value, so a filter names exactly what it says: filtering
// on PriorityMedium selects changes stored as medium, not the unknown-priority
// changes that merely rank as medium.
type SelectionFilter struct {
	Types      []string   // empty = all
	Priorities []Priority // empty = all
}

// SelectQueue returns the unambiguous build-ready changes of s that pass
// filter, in a total, deterministic order:
//
//  1. priority rank (critical, high, medium, low), with an unknown or absent
//     stored priority ranking as medium — reporting the bad value is
//     validation's job, not selection's, so selection keeps such a change in
//     the queue rather than dropping or demoting it;
//  2. within a priority band, well-formed created dates ascending, with every
//     change whose created is malformed, empty, or absent sorted AFTER every
//     dated change in that band — an unparseable date carries no position, so
//     it must not be laundered into the zero time and sorted first;
//  3. numeric ID ascending, which makes the order total: no pair of selected
//     changes can tie, since the snapshot excludes ambiguous IDs.
//
// Selection consults EvaluateReadiness, so a change that is not proposed, has
// an unmet dependency, carries no design, or sits on an unresolved stack base
// is excluded — as is a change whose ID more than one record claims, which
// readiness reports as invalid. The returned slice is freshly allocated; the
// caller may sort or truncate it without touching the snapshot.
func SelectQueue(s Snapshot, facts BranchFacts, filter SelectionFilter) []Change {
	queue := make([]Change, 0, len(s.changes))
	for _, c := range s.changes {
		if !filter.matches(c) {
			continue
		}
		if EvaluateReadiness(s, c, facts).Kind != ReadyBuildReady {
			continue
		}
		queue = append(queue, c)
	}
	slices.SortStableFunc(queue, compareSelection)
	return queue
}

// matches reports whether c passes both filter dimensions.
func (f SelectionFilter) matches(c Change) bool {
	if len(f.Types) > 0 && !slices.Contains(f.Types, c.Type()) {
		return false
	}
	if len(f.Priorities) > 0 && !slices.Contains(f.Priorities, c.Priority()) {
		return false
	}
	return true
}

// compareSelection orders two selected changes by the documented key: priority
// rank, then created (well-formed ascending, undated last), then numeric ID.
func compareSelection(a, b Change) int {
	if d := priorityRank(a.Priority()) - priorityRank(b.Priority()); d != 0 {
		return d
	}
	if d := compareCreated(a.Created(), b.Created()); d != 0 {
		return d
	}
	return int(a.ID()) - int(b.ID())
}

// compareCreated orders two optional created dates, treating anything that did
// not parse — malformed, empty, or absent alike — as sorting after every
// well-formed date. Two undated changes compare equal, leaving the ID
// tie-break to decide.
func compareCreated(a, b OptionalTime) int {
	aDated, bDated := a.State == FieldPresent, b.State == FieldPresent
	switch {
	case aDated && bDated:
		return a.Value.Compare(b.Value)
	case aDated:
		return -1
	case bDated:
		return 1
	default:
		return 0
	}
}
