package domain

import "fmt"

// The stable finding codes ADR graph validation emits.
const (
	// CodeADRBackpointerMismatch marks a relationship whose two halves
	// disagree: an edge whose target does not carry the verb-matched status,
	// or a *By status whose named ADR carries no reciprocal edge.
	CodeADRBackpointerMismatch = "adr-backpointer-mismatch"
	// CodeADRDanglingReference marks a reference no single record resolves —
	// absent entirely, or claimed by more than one record.
	CodeADRDanglingReference = "adr-dangling-reference"
	// CodeADRIDGap marks an unallocated ID below the highest existing one. It
	// is a warning: a gap is never an allocation target, only a note.
	CodeADRIDGap = "adr-id-gap"
	// CodeADRSelfReference marks an ADR that supersedes, reverses, relates to,
	// or names itself.
	CodeADRSelfReference = "adr-self-reference"
)

// The stable refusal reasons the ADR actions return.
const (
	reasonUnknownADR       = "unknown-adr"
	reasonAmbiguousADR     = "ambiguous-adr"
	reasonInvalidSuccessor = "invalid-successor-id"
	reasonADRSelfReference = "self-reference"
	reasonADRNotAccepted   = "adr-not-accepted"
)

// ValidateADRGraph checks the ADR relationship graph and returns its findings
// in ADR-record order. The rules are symmetric by construction: a supersedes
// edge from X to Y requires Y's status to be exactly SupersededBy(X), a
// reverses edge requires ReversedBy(X), and a *By status on Y naming Z
// requires Z to exist and to carry the verb-matched edge back to Y. Every
// reference that no single record resolves — including one more than one
// record claims — is a dangling error, and unallocated IDs below the highest
// existing one are warnings. Ordering of the returned slice is stable;
// ValidationReport imposes the report's own total order.
func ValidateADRGraph(s Snapshot) []Finding {
	var findings []Finding
	for _, a := range s.ADRs() {
		findings = append(findings, validateADREdges(s, a, "supersedes", a.Supersedes(), ADRSupersededBy)...)
		findings = append(findings, validateADREdges(s, a, "reverses", a.Reverses(), ADRReversedBy)...)
		findings = append(findings, validateADRRelatesTo(s, a)...)
		findings = append(findings, validateADRStatusRef(s, a)...)
		findings = append(findings, validateADRChangeRef(s, a)...)
	}
	return append(findings, adrIDGaps(s)...)
}

// validateADREdges checks one verb's outgoing edges: each target must exist
// and carry exactly the verb-matched *By status pointing back at a.
func validateADREdges(s Snapshot, a ADR, field string, targets []ADRID, kind ADRStatusKind) []Finding {
	var findings []Finding
	for _, target := range targets {
		if target == a.ID() {
			findings = append(findings, adrSelfReference(a, field))
			continue
		}
		other, out := s.ADR(target)
		if out != LookupFound {
			findings = append(findings, adrDangling(a, field, adrRef(target), out))
			continue
		}
		want := ADRStatus{Kind: kind, Ref: a.ID()}
		if other.Status() != want {
			findings = append(findings, Finding{
				Code:     CodeADRBackpointerMismatch,
				Severity: SeverityError,
				Entity:   adrRef(a.ID()),
				Field:    field,
				Related:  []EntityRef{adrRef(target)},
				Detail: map[string]string{
					"expected": want.String(),
					"actual":   other.Status().String(),
				},
			})
		}
	}
	return findings
}

// validateADRRelatesTo checks that every relates_to reference resolves and is
// not the ADR itself. relates_to carries no reciprocity requirement.
func validateADRRelatesTo(s Snapshot, a ADR) []Finding {
	var findings []Finding
	for _, target := range a.RelatesTo() {
		if target == a.ID() {
			findings = append(findings, adrSelfReference(a, "relates_to"))
			continue
		}
		if _, out := s.ADR(target); out != LookupFound {
			findings = append(findings, adrDangling(a, "relates_to", adrRef(target), out))
		}
	}
	return findings
}

// validateADRStatusRef checks the status side of the relationship: a
// SupersededBy/ReversedBy status must name an existing ADR that carries the
// verb-matched edge back. Accepted and Deprecated name nothing and are clean.
func validateADRStatusRef(s Snapshot, a ADR) []Finding {
	status := a.Status()
	if status.Kind != ADRSupersededBy && status.Kind != ADRReversedBy {
		return nil
	}
	if status.Ref == a.ID() {
		return []Finding{adrSelfReference(a, "status")}
	}
	other, out := s.ADR(status.Ref)
	if out != LookupFound {
		return []Finding{adrDangling(a, "status", adrRef(status.Ref), out)}
	}
	edges := other.Supersedes()
	if status.Kind == ADRReversedBy {
		edges = other.Reverses()
	}
	for _, id := range edges {
		if id == a.ID() {
			return nil
		}
	}
	return []Finding{{
		Code:     CodeADRBackpointerMismatch,
		Severity: SeverityError,
		Entity:   adrRef(a.ID()),
		Field:    "status",
		Related:  []EntityRef{adrRef(status.Ref)},
		Detail: map[string]string{
			"status":        status.String(),
			"missing-edge":  edgeFieldFor(status.Kind),
			"expected-on":   fmt.Sprintf("%04d", int(status.Ref)),
			"expected-edge": fmt.Sprintf("%04d", int(a.ID())),
		},
	}}
}

// edgeFieldFor names the edge field a *By status requires on its target.
func edgeFieldFor(kind ADRStatusKind) string {
	if kind == ADRReversedBy {
		return "reverses"
	}
	return "supersedes"
}

// validateADRChangeRef checks that a stored producing-change back-link
// resolves to exactly one change. An absent or malformed key is the decoder's
// finding, not the graph's.
func validateADRChangeRef(s Snapshot, a ADR) []Finding {
	ref := a.Change()
	if ref.State != FieldPresent {
		return nil
	}
	if _, out := s.Change(ChangeID(ref.Value)); out != LookupFound {
		return []Finding{adrDangling(a, "change", EntityRef{Kind: EntityChange, ID: ref.Value}, out)}
	}
	return nil
}

// adrIDGaps returns one warning per unallocated ID below the highest existing
// ADR ID. Non-positive IDs are not part of the allocation line.
func adrIDGaps(s Snapshot) []Finding {
	present := make(map[ADRID]bool)
	var highest ADRID
	for _, a := range s.ADRs() {
		if a.ID() <= 0 {
			continue
		}
		present[a.ID()] = true
		if a.ID() > highest {
			highest = a.ID()
		}
	}
	var findings []Finding
	for id := ADRID(1); id < highest; id++ {
		if !present[id] {
			findings = append(findings, Finding{
				Code:     CodeADRIDGap,
				Severity: SeverityWarning,
				Entity:   adrRef(id),
				Detail:   map[string]string{"highest": fmt.Sprintf("%04d", int(highest))},
			})
		}
	}
	return findings
}

// adrRef builds the finding reference for an ADR ID.
func adrRef(id ADRID) EntityRef { return EntityRef{Kind: EntityADR, ID: int(id)} }

// adrSelfReference builds the self-reference finding for one field.
func adrSelfReference(a ADR, field string) Finding {
	return Finding{
		Code:     CodeADRSelfReference,
		Severity: SeverityError,
		Entity:   adrRef(a.ID()),
		Field:    field,
	}
}

// adrDangling builds the dangling-reference finding, recording whether the
// reference was absent or claimed by more than one record.
func adrDangling(a ADR, field string, target EntityRef, out LookupOutcome) Finding {
	lookup := "absent"
	if out == LookupAmbiguous {
		lookup = "ambiguous"
	}
	return Finding{
		Code:     CodeADRDanglingReference,
		Severity: SeverityError,
		Entity:   adrRef(a.ID()),
		Field:    field,
		Related:  []EntityRef{target},
		Detail:   map[string]string{"lookup": lookup},
	}
}

// NextADRID returns the next ID to allocate: max(existing)+1, so gaps below
// the highest ID are never allocation targets. An empty ledger allocates 1.
func NextADRID(s Snapshot) ADRID {
	var highest ADRID
	for _, a := range s.ADRs() {
		if a.ID() > highest {
			highest = a.ID()
		}
	}
	return highest + 1
}

// ADRActionResult is a successful supersede or reverse: the target ADR and the
// status it flips to. Authoring the successor record belongs to a later layer;
// the domain only decides the flip.
type ADRActionResult struct {
	NewStatus ADRStatus // the flipped status for the target
	Target    ADRID
}

// Supersede flips an Accepted target to "Superseded by ADR-<successor>".
func Supersede(s Snapshot, target ADRID, successor ADRID) (ADRActionResult, *PolicyFailure) {
	return adrStatusFlip(s, target, successor, ADRSupersededBy)
}

// Reverse flips an Accepted target to "Reversed by ADR-<successor>".
func Reverse(s Snapshot, target ADRID, successor ADRID) (ADRActionResult, *PolicyFailure) {
	return adrStatusFlip(s, target, successor, ADRReversedBy)
}

// adrStatusFlip is the shared guard for both actions: the successor must be a
// plausible new ID distinct from the target, and the target must resolve to
// exactly one Accepted record. Anything else is a typed refusal.
func adrStatusFlip(s Snapshot, target ADRID, successor ADRID, kind ADRStatusKind) (ADRActionResult, *PolicyFailure) {
	if successor <= 0 {
		return ADRActionResult{}, adrFailure(FailInvalidInput, reasonInvalidSuccessor, target, successor, "")
	}
	if successor == target {
		return ADRActionResult{}, adrFailure(FailInvalidInput, reasonADRSelfReference, target, successor, "")
	}
	current, out := s.ADR(target)
	switch out {
	case LookupAbsent:
		return ADRActionResult{}, adrFailure(FailInvalidInput, reasonUnknownADR, target, successor, "")
	case LookupAmbiguous:
		return ADRActionResult{}, adrFailure(FailInvalidInput, reasonAmbiguousADR, target, successor, "")
	}
	if current.Status().Kind != ADRAccepted {
		return ADRActionResult{}, adrFailure(FailInvalidState, reasonADRNotAccepted, target, successor, current.Status().String())
	}
	return ADRActionResult{NewStatus: ADRStatus{Kind: kind, Ref: successor}, Target: target}, nil
}

// adrFailure builds an ADR-scoped PolicyFailure. The Change and State fields
// stay zero — this refusal is about an ADR, and the operands live in Detail so
// no caller has to parse prose.
func adrFailure(kind PolicyFailureKind, reason string, target ADRID, successor ADRID, status string) *PolicyFailure {
	detail := map[string]string{
		"adr":       fmt.Sprintf("%04d", int(target)),
		"successor": fmt.Sprintf("%04d", int(successor)),
	}
	if status != "" {
		detail["status"] = status
	}
	return &PolicyFailure{Kind: kind, Reason: reason, Detail: detail}
}
