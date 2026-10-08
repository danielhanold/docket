package app

import (
	"errors"
	"testing"
)

// TestRunCancelOwnerMatchesCancelProofs pins run.cancel's ownership proof
// (ADR-0128 Decision 1) as one pure decision, shared by runCancel and the
// live-run scan (liveRunCancelAuthority) so the two cannot drift (change 0540).
func TestRunCancelOwnerMatchesCancelProofs(t *testing.T) {
	confirmed := RunTrackerClaimBinding{Schema: bindingSchemaVersion, ChangeID: 7, RequestID: "r", Confirmed: true}
	reserved := RunTrackerClaimBinding{Schema: bindingSchemaVersion, ChangeID: 7, RequestID: "r"}
	withCap := RunTrackerRecord{ParentCap: "cap"}
	resume := RunTrackerRecord{ParentCap: "cap", AttributedID: 7}
	none := RunTrackerClaimBinding{}
	cases := []struct {
		name        string
		rec         RunTrackerRecord
		binding     RunTrackerClaimBinding
		has         bool
		berr        error
		runChange   string
		wantID      int
		wantRefusal string
	}{
		{"confirmed binding", withCap, confirmed, true, nil, "7", 7, ""},
		{"confirmed binding, run not yet bound", withCap, confirmed, true, nil, "", 7, ""},
		{"resume-verified record", resume, none, false, nil, "7", 7, ""},
		{"no parent capability", RunTrackerRecord{}, confirmed, true, nil, "7", 0, "authority-unavailable"},
		{"capability checked before binding", RunTrackerRecord{}, none, false, errors.New("corrupt"), "7", 0, "authority-unavailable"},
		{"unreadable binding", withCap, none, false, errors.New("corrupt"), "7", 0, "claim-unreadable"},
		{"unconfirmed reservation", withCap, reserved, true, nil, "7", 0, "claim-unconfirmed"},
		{"reservation beside a resume shape", resume, reserved, true, nil, "7", 0, "claim-unconfirmed"},
		{"no binding and no claim", withCap, none, false, nil, "", 0, "claim-unconfirmed"},
		{"owner differs from the run", withCap, confirmed, true, nil, "8", 0, "claim-mismatch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, refusal := runCancelOwner(c.rec, c.binding, c.has, c.berr, c.runChange)
			if id != c.wantID || refusal != c.wantRefusal {
				t.Fatalf("runCancelOwner = (%d, %q), want (%d, %q)", id, refusal, c.wantID, c.wantRefusal)
			}
		})
	}
}
