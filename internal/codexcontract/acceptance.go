package codexcontract

import "fmt"

type AcceptanceReceipt struct {
	SchemaVersion         int               `json:"schema_version"`
	SourceCommit          string            `json:"source_commit"`
	BinaryCommit          string            `json:"binary_commit"`
	BinarySHA256          string            `json:"binary_sha256"`
	DefinitionSHA256      map[string]string `json:"definition_sha256"`
	ResourceSHA256        map[string]string `json:"resource_sha256"`
	NativeLineage         []string          `json:"native_lineage"`
	PlanCommit            string            `json:"plan_commit"`
	TaskCommits           []string          `json:"task_commits"`
	ResultsCommit         string            `json:"results_commit"`
	ReviewHEAD            string            `json:"review_head"`
	ImplementationGate    string            `json:"implementation_gate"`
	FinalGate             string            `json:"final_gate"`
	TerminalVerdict       string            `json:"terminal_verdict"`
	EvidenceAuditComplete bool              `json:"evidence_audit_complete"`
}

func ValidateAcceptanceReceipt(r AcceptanceReceipt) error {
	if r.SchemaVersion != 1 || r.SourceCommit == "" || r.BinaryCommit != r.SourceCommit || r.BinarySHA256 == "" {
		return fmt.Errorf("candidate source/binary identity is incomplete")
	}
	if r.EvidenceAuditComplete {
		return fmt.Errorf("native acceptance cannot claim hard-isolation audit completeness")
	}
	if r.ResultsCommit != "" && r.FinalGate == "" {
		return fmt.Errorf("results require an independent final-head gate")
	}
	return nil
}
