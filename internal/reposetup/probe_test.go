package reposetup

import "testing"

// TestPresenceZeroValueIsUnknown pins the load-bearing invariant that the zero
// value of Presence is the SAFE Unknown value: a gatherer that leaves a field
// unset must never be read as a proven Absent (learning
// probe-error-is-not-clean-absence). RootShape and PartialPhase share the same
// zero-is-safe contract.
func TestPresenceZeroValueIsUnknown(t *testing.T) {
	if Presence(0) != PresenceUnknown {
		t.Fatalf("Presence(0) = %v, want PresenceUnknown", Presence(0))
	}
	if RootShape(0) != RootUnknown {
		t.Fatalf("RootShape(0) = %v, want RootUnknown", RootShape(0))
	}
	if PartialPhase(0) != PartialNone {
		t.Fatalf("PartialPhase(0) = %v, want PartialNone", PartialPhase(0))
	}
}
