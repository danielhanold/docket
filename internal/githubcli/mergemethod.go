package githubcli

// This file owns the merge-method vocabulary and policy: the closed MergeMethod
// set, the capability probes over GitHub's repository settings and active
// branch rules, and the pure fixed-priority selection "rebase, else merge, else
// squash, else unavailable". There is deliberately NO configuration surface —
// the preference order is product policy. Repository permissions and branch
// rules compose by intersection; unobservable or malformed policy fails closed
// (three-outcome discipline; learnings: probe-error-is-not-clean-absence).

// MergeMethod is the closed vocabulary of merge methods Docket can select.
type MergeMethod string

const (
	// MethodRebase → `gh pr merge --rebase`.
	MethodRebase MergeMethod = "rebase"
	// MethodMerge → `gh pr merge --merge` (a merge commit).
	MethodMerge MergeMethod = "merge"
	// MethodSquash → `gh pr merge --squash`.
	MethodSquash MergeMethod = "squash"
)

// mergeFlag renders the gh `pr merge` flag for a vocabulary member, and nothing
// for anything else — the act path guards on a non-empty flag rather than
// defaulting permissively.
func (m MergeMethod) mergeFlag() string {
	switch m {
	case MethodRebase:
		return "--rebase"
	case MethodMerge:
		return "--merge"
	case MethodSquash:
		return "--squash"
	default:
		return ""
	}
}

// methodSet is a capability set over the three methods.
type methodSet struct {
	rebase, merge, squash bool
}

// intersect composes two capability sets; multiple applicable restrictions
// always narrow, never widen.
func (s methodSet) intersect(o methodSet) methodSet {
	return methodSet{
		rebase: s.rebase && o.rebase,
		merge:  s.merge && o.merge,
		squash: s.squash && o.squash,
	}
}

// list renders the permitted methods in preference order, for diagnostics.
func (s methodSet) list() []MergeMethod {
	out := []MergeMethod{}
	if s.rebase {
		out = append(out, MethodRebase)
	}
	if s.merge {
		out = append(out, MethodMerge)
	}
	if s.squash {
		out = append(out, MethodSquash)
	}
	return out
}

// selectMergeMethod is the pure ordered selection over the effective set:
// rebase, else merge, else squash, else unavailable. `--admin` never widens the
// set or reorders it.
func selectMergeMethod(eff methodSet) (MergeMethod, bool) {
	switch {
	case eff.rebase:
		return MethodRebase, true
	case eff.merge:
		return MethodMerge, true
	case eff.squash:
		return MethodSquash, true
	default:
		return "", false
	}
}
