package domain

import "slices"

// LearningCatalog is the result of a learning-consumption query. Disabled is
// the explicit "learnings are off in policy" answer: it is never inferred from
// an empty Findings slice, so a caller can tell "off" from "none matched".
type LearningCatalog struct {
	Disabled bool
	Findings []Learning // active = retained + candidate; promoted excluded
}

// LearningCandidates returns the active catalog in authored record order.
// Promoted findings stay readable historical records but are not active, so
// they never appear here. With learning consumption disabled in policy the
// result is the explicit disabled catalog with no findings at all.
func LearningCandidates(s Snapshot) LearningCatalog {
	return FilterLearnings(s, nil, nil)
}

// FilterLearnings returns the active catalog narrowed by explicit topic and
// slug inputs: OR within a dimension, AND across the two when both are
// supplied, and no filtering at all when both are empty. Matching is exact —
// the domain does not judge semantic relevance, which stays with the calling
// skill. The inputs are never retained, and the returned slice is fresh.
func FilterLearnings(s Snapshot, topics []string, slugs []string) LearningCatalog {
	if !s.Policy().LearningsEnabled {
		return LearningCatalog{Disabled: true}
	}
	var out []Learning
	for _, l := range s.Learnings() {
		if l.Promotion() == PromotionPromoted {
			continue
		}
		if len(topics) > 0 && !slices.ContainsFunc(l.Topics(), func(t string) bool {
			return slices.Contains(topics, t)
		}) {
			continue
		}
		if len(slugs) > 0 && !slices.Contains(slugs, l.Slug()) {
			continue
		}
		out = append(out, l)
	}
	return LearningCatalog{Findings: out}
}
