package domain

// ReadinessKind tags a change's readiness outcome. Readiness is a typed value
// rather than a Boolean: a caller that only wants "can this be built" reads
// Kind == ReadyBuildReady, while a caller reporting to a human keeps the exact
// reason the change is not ready.
type ReadinessKind string

// The closed set of readiness outcomes.
const (
	ReadyBuildReady          ReadinessKind = "build-ready"
	ReadyNeedsBrainstorm     ReadinessKind = "needs-brainstorm"
	ReadyAutoGroomBlocked    ReadinessKind = "auto-groom-blocked"
	ReadyWaitingDependency   ReadinessKind = "waiting-dependency"
	ReadyStackBaseUnresolved ReadinessKind = "stack-base-unresolved"
	ReadyInvalid             ReadinessKind = "invalid"
	ReadyNotProposed         ReadinessKind = "not-proposed"
)

// Readiness is the tagged result of evaluating one change's build readiness.
// Dependency carries the full dependency evaluation whenever dependencies were
// evaluated — populated for ReadyWaitingDependency, and satisfied for every
// outcome reached past that step. StackBase is the zero value unless stack
// resolution was actually consulted, which happens only for a change that is
// otherwise build-ready.
type Readiness struct {
	Kind       ReadinessKind
	Dependency DependencyEvaluation
	StackBase  EffectiveBase
}

// EvaluateReadiness reports c's readiness against s, preserving the retained
// precedence:
//
//  1. a change that is not proposed is not-proposed — no other condition is
//     considered;
//  2. a change whose identity cannot be trusted is invalid: an ID more than one
//     record claims (no single record can be attributed to it), a non-positive
//     ID, or a slug outside the shared record-slug grammar. The latter two are
//     not cosmetic — a build-ready verdict is a licence to claim the change,
//     and Claim derives its branch name from the slug and refuses one that is
//     not a usable branch component;
//  3. an unmet dependency reports waiting-dependency BEFORE missing design is
//     considered;
//  4. missing design reports needs-brainstorm, or auto-groom-blocked when the
//     record carries the retained historical marker;
//  5. stack resolution is consulted ONLY for a change that would otherwise be
//     build-ready, and an unresolved effective base reports
//     stack-base-unresolved; otherwise
//  6. the change is build-ready.
//
// A change carries design when it has a non-empty spec: reference or is marked
// trivial; a present-but-empty spec: counts as no spec.
func EvaluateReadiness(s Snapshot, c Change, facts BranchFacts) Readiness {
	if c.Status() != StatusProposed {
		return Readiness{Kind: ReadyNotProposed}
	}
	if _, out := s.Change(c.ID()); out == LookupAmbiguous {
		return Readiness{Kind: ReadyInvalid}
	}
	if c.ID() <= 0 || !ValidSlugToken(c.Slug()) {
		return Readiness{Kind: ReadyInvalid}
	}

	deps := EvaluateDependencies(s, c)
	if !deps.Satisfied {
		return Readiness{Kind: ReadyWaitingDependency, Dependency: deps}
	}

	if !hasDesign(c) {
		kind := ReadyNeedsBrainstorm
		if c.HasAutoGroomBlocked() {
			kind = ReadyAutoGroomBlocked
		}
		return Readiness{Kind: kind, Dependency: deps}
	}

	base := ResolveEffectiveBase(s, c, facts)
	kind := ReadyBuildReady
	if base.Kind != BaseResolved {
		kind = ReadyStackBaseUnresolved
	}
	return Readiness{Kind: kind, Dependency: deps, StackBase: base}
}

// NeedsDesign reports whether c still needs a design pass: proposed, carrying
// no spec, and not marked trivial. It deliberately ignores dependency
// satisfaction, so interactive grooming can design a change ahead of the work
// it depends on.
func NeedsDesign(c Change) bool {
	return c.Status() == StatusProposed && !hasDesign(c)
}

// hasDesign reports whether c carries a design outcome — a non-empty spec:
// reference or trivial: true. A spec: key present with no value is no spec.
func hasDesign(c Change) bool {
	spec := c.Spec()
	return c.Trivial() || (spec.State == FieldPresent && spec.Value != "")
}
