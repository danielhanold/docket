package domain

import (
	"cmp"
	"slices"
)

// DependencyReason names why a dependency edge is unmet.
type DependencyReason string

// The closed set of unmet-dependency reasons.
const (
	DepNotBuilt   DependencyReason = "not-built"
	DepNeedsMerge DependencyReason = "needs-merge" // outranks not-built for the summary
)

// UnmetDependency is one unsatisfied depends_on edge. Missing marks a
// reference no record carries at all — an ambiguous reference is unmet too,
// but it is not absent, so it does not set Missing.
type UnmetDependency struct {
	ID      ChangeID
	Reason  DependencyReason
	Missing bool
}

// DependencyEvaluation is the read-only dependency state of one change.
// Summary and Representative are zero values when Satisfied.
type DependencyEvaluation struct {
	Unmet          []UnmetDependency // authored depends_on order
	Summary        DependencyReason  // zero value when Satisfied
	Representative ChangeID          // first unmet dep in authored order matching Summary
	Satisfied      bool
}

// EvaluateDependencies reports c's dependency state against s. A dependency is
// satisfied ONLY when the referenced change is done: an implemented dependency
// yields needs-merge, and every other non-done status — plus a missing or
// ambiguous reference — yields not-built. Unmet edges keep authored
// depends_on order, duplicates included; needs-merge outranks not-built for
// the summary, and within one reason the representative is the first such edge
// in authored order.
func EvaluateDependencies(s Snapshot, c Change) DependencyEvaluation {
	var unmet []UnmetDependency
	for _, id := range c.DependsOn() {
		dep, out := s.Change(id)
		switch {
		case out == LookupAbsent:
			unmet = append(unmet, UnmetDependency{ID: id, Reason: DepNotBuilt, Missing: true})
		case out == LookupAmbiguous:
			unmet = append(unmet, UnmetDependency{ID: id, Reason: DepNotBuilt})
		case dep.Status() == StatusDone:
			// satisfied — done is the only satisfying status
		case dep.Status() == StatusImplemented:
			unmet = append(unmet, UnmetDependency{ID: id, Reason: DepNeedsMerge})
		default:
			unmet = append(unmet, UnmetDependency{ID: id, Reason: DepNotBuilt})
		}
	}

	eval := DependencyEvaluation{Unmet: unmet, Satisfied: len(unmet) == 0}
	if eval.Satisfied {
		return eval
	}
	eval.Summary = DepNotBuilt
	if slices.ContainsFunc(unmet, func(u UnmetDependency) bool { return u.Reason == DepNeedsMerge }) {
		eval.Summary = DepNeedsMerge
	}
	for _, u := range unmet {
		if u.Reason == eval.Summary {
			eval.Representative = u.ID
			break
		}
	}
	return eval
}

// Cycle is one elementary cycle over a change graph. Members are rotation-
// normalized to start at the cycle's smallest ID and then follow the edges
// back to it; a self-cycle has a single member.
type Cycle struct {
	Members []ChangeID
}

// DependencyCycles returns every elementary cycle over the depends_on graph,
// self-cycles included, ordered by first member, then length, then member IDs.
func DependencyCycles(s Snapshot) []Cycle {
	return cyclesOf(dependencyEdges(s))
}

// StackCycles returns every elementary cycle over the stacked_on graph, with
// the same normalization and ordering as DependencyCycles. The two graphs stay
// orthogonal: neither walk follows the other's edges.
func StackCycles(s Snapshot) []Cycle {
	return cyclesOf(stackEdges(s))
}

// graphNodes returns the changes that can carry graph edges, keyed by ID.
// A change whose ID more than one record claims is left out entirely: the
// snapshot picks no winner among duplicates, so neither its outgoing edges nor
// an edge pointing at it can be attributed to a single record.
func graphNodes(s Snapshot) map[ChangeID]Change {
	changes := s.Changes()
	nodes := make(map[ChangeID]Change, len(changes))
	for _, c := range changes {
		if resolved, out := s.Change(c.ID()); out == LookupFound {
			nodes[c.ID()] = resolved
		}
	}
	return nodes
}

// dependencyEdges builds the depends_on adjacency. Successors are deduplicated
// and sorted so the walk is deterministic; edges to unresolvable IDs are
// dropped, since a dangling edge cannot close a cycle.
func dependencyEdges(s Snapshot) map[ChangeID][]ChangeID {
	nodes := graphNodes(s)
	adj := make(map[ChangeID][]ChangeID, len(nodes))
	for id, c := range nodes {
		var succ []ChangeID
		for _, dep := range c.DependsOn() {
			if _, ok := nodes[dep]; ok {
				succ = append(succ, dep)
			}
		}
		slices.Sort(succ)
		adj[id] = slices.Compact(succ)
	}
	return adj
}

// stackEdges builds the stacked_on adjacency: at most one successor per node,
// and only when the parent edge actually parsed.
func stackEdges(s Snapshot) map[ChangeID][]ChangeID {
	nodes := graphNodes(s)
	adj := make(map[ChangeID][]ChangeID, len(nodes))
	for id, c := range nodes {
		adj[id] = nil
		parent := c.StackedOn()
		if parent.State != FieldPresent {
			continue
		}
		if _, ok := nodes[ChangeID(parent.Value)]; ok {
			adj[id] = []ChangeID{ChangeID(parent.Value)}
		}
	}
	return adj
}

// frame is one node of the explicit DFS stack: the node being expanded and the
// index of the next successor to try.
type frame struct {
	node ChangeID
	next int
}

// cyclesOf enumerates every elementary cycle in adj with an iterative
// depth-first search — an explicit stack, never recursion, so no input can
// exhaust the goroutine stack, and the search always terminates. Enumeration
// is exponential in the worst case on dense cyclic graphs; real dependency
// graphs are sparse, and any cycle at all is already a validation error. Each
// cycle is found exactly
// once by searching only from its smallest member: the walk rooted at start
// visits no node below start, so any path returning to start is already
// rotation-normalized.
func cyclesOf(adj map[ChangeID][]ChangeID) []Cycle {
	starts := make([]ChangeID, 0, len(adj))
	for id := range adj {
		starts = append(starts, id)
	}
	slices.Sort(starts)

	var cycles []Cycle
	onPath := make(map[ChangeID]bool, len(adj))
	for _, start := range starts {
		stack := []frame{{node: start}}
		onPath[start] = true
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			succ := adj[top.node]
			if top.next >= len(succ) {
				onPath[top.node] = false
				stack = stack[:len(stack)-1]
				continue
			}
			next := succ[top.next]
			top.next++
			switch {
			case next == start:
				members := make([]ChangeID, 0, len(stack))
				for _, f := range stack {
					members = append(members, f.node)
				}
				cycles = append(cycles, Cycle{Members: members})
			case next > start && !onPath[next]:
				onPath[next] = true
				stack = append(stack, frame{node: next})
			}
		}
	}

	slices.SortFunc(cycles, compareCycles)
	return cycles
}

// compareCycles orders cycles by first member, then length, then member IDs —
// a total order, so the report never depends on discovery order.
func compareCycles(a, b Cycle) int {
	if c := cmp.Compare(a.Members[0], b.Members[0]); c != 0 {
		return c
	}
	if c := cmp.Compare(len(a.Members), len(b.Members)); c != 0 {
		return c
	}
	return slices.Compare(a.Members, b.Members)
}
