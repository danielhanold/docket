package app

import (
	"path/filepath"

	"github.com/danielhanold/docket/internal/codexcontract"
)

const OperationAgentCheckReceipt = "agent.check-receipt"

type CheckReceiptRequest struct {
	RunRoot         string `json:"run_root,omitempty" docketdoc:"Coordinator capture context; absolute clean run root, mutually exclusive with assignment."`
	ExpectedDriveID string `json:"expected_drive_id,omitempty"`
	ExpectedPhase   string `json:"expected_phase,omitempty"`
	Operation       string `json:"operation" docket:"required"`
	Assignment      string `json:"assignment,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	Stdout          string `json:"stdout" docket:"required"`
	Stderr          string `json:"stderr,omitempty"`
	ExitCode        int    `json:"exit_code" docket:"required"`
}
type CheckReceiptResult struct {
	Envelope
	Reason  string                 `json:"reason,omitempty"`
	Receipt *codexcontract.Receipt `json:"receipt,omitempty"`
}

func (r CheckReceiptResult) HumanText() string {
	if r.Reason != "" {
		return r.Reason
	}
	return "agent receipt valid"
}
func CheckAgentReceipt(req CheckReceiptRequest) CheckReceiptResult {
	var a codexcontract.Assignment
	if req.Assignment != "" || req.SHA256 != "" {
		if req.RunRoot != "" {
			return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: "receipt-context-conflict"}
		}
		var err error
		a, err = codexcontract.ReadAssignment(req.Assignment, req.SHA256)
		if err != nil {
			return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: "assignment-invalid: " + err.Error()}
		}
	} else {
		if !filepath.IsAbs(req.RunRoot) || filepath.Clean(req.RunRoot) != req.RunRoot {
			return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: "receipt-context-required: absolute clean run_root or pinned assignment"}
		}
		if req.Operation == "run.gate-claim" && (req.ExpectedDriveID == "" || req.ExpectedPhase == "") {
			return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: "claim-identity-required: expected_drive_id and expected_phase"}
		}
		a.RunRoot = req.RunRoot
	}
	stdout, err := readPinnedFileUnchecked(req.Stdout)
	if err != nil {
		return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: "stdout-invalid: " + err.Error()}
	}
	var stderr []byte
	if req.Stderr != "" {
		stderr, err = readPinnedFileUnchecked(req.Stderr)
		if err != nil {
			return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: "stderr-invalid: " + err.Error()}
		}
	}
	r, err := codexcontract.ParseReceipt(req.Operation, stdout, stderr, req.ExitCode, a, codexcontract.ReceiptExpectation{DriveID: req.ExpectedDriveID, Phase: req.ExpectedPhase})
	if err != nil {
		return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: err.Error(), Receipt: &r}
	}
	return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultApplied), Receipt: &r}
}
