package app

import "github.com/danielhanold/docket/internal/codexcontract"

const OperationAgentCheckReceipt = "agent.check-receipt"

type CheckReceiptRequest struct {
	Operation  string `json:"operation" docket:"required"`
	Assignment string `json:"assignment" docket:"required"`
	SHA256     string `json:"sha256" docket:"required"`
	Stdout     string `json:"stdout" docket:"required"`
	Stderr     string `json:"stderr,omitempty"`
	ExitCode   int    `json:"exit_code" docket:"required"`
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
	a, err := codexcontract.ReadAssignment(req.Assignment, req.SHA256)
	if err != nil {
		return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: "assignment-invalid: " + err.Error()}
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
	r, err := codexcontract.ParseReceipt(req.Operation, stdout, stderr, req.ExitCode, a)
	if err != nil {
		return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultInvalidInput), Reason: err.Error(), Receipt: &r}
	}
	return CheckReceiptResult{Envelope: NewEnvelope(OperationAgentCheckReceipt, ResultApplied), Receipt: &r}
}
