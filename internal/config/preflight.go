package config

import "fmt"

// PreflightDecision is the mutation gate's verdict on one resolved snapshot.
// Blockers is the COMPLETE set of blocking diagnostics in path order, never a
// first-only sample: a user repairing a configuration should need one pass.
type PreflightDecision struct {
	Allowed  bool
	Blockers []Diagnostic
}

// PreflightMutation asks whether the snapshot's configuration permits mutating
// operations. It reads the snapshot only — the classifier already decided what
// is active; the preflight merely collects its verdict. Inspection-only
// operations never consult this.
func PreflightMutation(s *Snapshot) PreflightDecision {
	if s == nil {
		return PreflightDecision{Allowed: true}
	}
	var blockers []Diagnostic
	for _, d := range s.Diagnostics {
		if d.Code == CodeDeferredCapRequested {
			blockers = append(blockers, d)
		}
	}
	return PreflightDecision{Allowed: len(blockers) == 0, Blockers: blockers}
}

// GuardMutation is the seam every transaction-shaped operation calls. On a
// blocked configuration it returns without calling continue_ at all: a
// mutation refused after the fact is not a refusal.
func GuardMutation(s *Snapshot, continue_ func() error) error {
	decision := PreflightMutation(s)
	if !decision.Allowed {
		return fmt.Errorf("%w: %d blocker(s), first: %s", ErrUnsupportedConfig, len(decision.Blockers), decision.Blockers[0].Path)
	}
	return continue_()
}
