package codexcontract

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// parseClaimReceipt consumes the facade's public envelope. Redemption has already
// happened: generation is the new owner's authority, never a handoff token.
// This checker neither reads private state nor confers backend ownership.
func parseClaimReceipt(r Receipt) (Receipt, error) {
	var wire struct {
		ProtocolVersion int    `json:"protocol_version"`
		Operation       string `json:"operation"`
		Result          string `json:"result"`
		Key             string `json:"key,omitempty"`
		Decision        string `json:"decision,omitempty"`
		Outcome         string `json:"outcome,omitempty"`
		DriveID         string `json:"drive_id,omitempty"`
		Generation      string `json:"generation,omitempty"`
		Phase           string `json:"phase,omitempty"`
		Reason          string `json:"reason,omitempty"`
		Cause           string `json:"cause,omitempty"`
		Terminal        *bool  `json:"terminal"`
	}
	if err := rejectDuplicateKeys(r.Stdout); err != nil {
		return r, fmt.Errorf("invalid claim receipt: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(r.Stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wire); err != nil {
		return r, fmt.Errorf("invalid claim receipt: %w", err)
	}
	if err := ensureJSONEOF(dec); err != nil {
		return r, fmt.Errorf("invalid claim receipt trailer: %w", err)
	}
	if wire.ProtocolVersion != 1 || wire.Operation != r.Operation || wire.Result != "applied" || wire.Terminal == nil || r.ExitCode != 0 {
		return r, fmt.Errorf("claim receipt envelope is inconsistent")
	}
	r.Result, r.Decision, r.Reason = wire.Result, wire.Decision, wire.Reason
	switch wire.Decision {
	case "gate-stop":
		if !*wire.Terminal || wire.Reason == "" || wire.Generation != "" || wire.DriveID != "" || wire.Outcome != "" || wire.Phase != "" {
			return r, fmt.Errorf("claim refusal receipt is inconsistent")
		}
		r.Classification = "halt"
	case "gate-claimed":
		if *wire.Terminal || wire.Key == "" || wire.DriveID == "" || wire.Generation == "" || wire.Phase == "" || wire.Reason != "" || wire.Cause != "" || wire.Outcome == string(gatedrive.HALTED) || !knownDriveOutcome(gatedrive.Outcome(wire.Outcome)) {
			return r, fmt.Errorf("claimed receipt is incomplete or inconsistent")
		}
		r.DriveID, r.Generation, r.Phase, r.Outcome = wire.DriveID, wire.Generation, wire.Phase, wire.Outcome
		r.Classification, r.Authority, r.NextOperation = wire.Outcome, "owner", "gate.drive.advance"
	default:
		return r, fmt.Errorf("unknown claim receipt decision")
	}
	return r, nil
}
