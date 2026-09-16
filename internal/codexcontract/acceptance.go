package codexcontract

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
)

type AcceptanceReceipt struct {
	SchemaVersion         int               `json:"schema_version"`
	SourceCommit          string            `json:"source_commit"`
	BinaryCommit          string            `json:"binary_commit"`
	BinarySHA256          string            `json:"binary_sha256"`
	DefinitionSHA256      map[string]string `json:"definition_sha256"`
	ResourceSHA256        map[string]string `json:"resource_sha256"`
	ConfiguredRoles       []string          `json:"configured_roles"`
	ObservedRoles         []string          `json:"observed_roles"`
	RuntimeRoots          map[string]string `json:"runtime_roots"`
	NativeLineage         []string          `json:"native_lineage"`
	RoleSequence          []string          `json:"role_sequence"`
	PlanCommit            string            `json:"plan_commit"`
	TaskCommits           []string          `json:"task_commits"`
	ResultsCommit         string            `json:"results_commit"`
	ScopeSequence         []string          `json:"scope_sequence"`
	ScopeAcknowledged     bool              `json:"scope_acknowledged"`
	PublicationRevision   string            `json:"publication_revision"`
	AttachmentRevision    string            `json:"attachment_metadata_revision"`
	ReviewHEAD            string            `json:"review_head"`
	ImplementationGate    string            `json:"implementation_gate"`
	FinalGate             string            `json:"final_gate"`
	FinalGateCommit       string            `json:"final_gate_commit"`
	TerminalVerdict       string            `json:"terminal_verdict"`
	EvidenceAuditComplete bool              `json:"evidence_audit_complete"`
}

func ValidateAcceptanceReceipt(r AcceptanceReceipt) error {
	if r.SchemaVersion != 1 || !objectID.MatchString(r.SourceCommit) || r.BinaryCommit != r.SourceCommit || !sha256Digest(r.BinarySHA256) {
		return fmt.Errorf("candidate source/binary identity is incomplete")
	}
	if len(r.DefinitionSHA256) == 0 || len(r.ResourceSHA256) == 0 {
		return fmt.Errorf("candidate definition/resource hashes are incomplete")
	}
	for _, hashes := range []map[string]string{r.DefinitionSHA256, r.ResourceSHA256} {
		for name, digest := range hashes {
			if name == "" || !sha256Digest(digest) {
				return fmt.Errorf("candidate definition/resource hash is invalid")
			}
		}
	}
	wantRoles, gotRoles := append([]string{}, r.ConfiguredRoles...), append([]string{}, r.ObservedRoles...)
	slices.Sort(wantRoles)
	slices.Sort(gotRoles)
	if len(wantRoles) == 0 || !slices.Equal(wantRoles, gotRoles) {
		return fmt.Errorf("configured and observed native roles do not match")
	}
	primary, feature, common := r.RuntimeRoots["primary"], r.RuntimeRoots["feature"], r.RuntimeRoots["common_dir"]
	if primary == feature || !canonicalAbsolute(primary) || !canonicalAbsolute(feature) || !canonicalAbsolute(common) {
		return fmt.Errorf("canonical runtime roots are incomplete")
	}
	if len(r.NativeLineage) < 4 || !validAcceptanceRoleSequence(r.RoleSequence) || !objectID.MatchString(r.PlanCommit) || len(r.TaskCommits) == 0 || !objectID.MatchString(r.ResultsCommit) {
		return fmt.Errorf("native lineage or artifact commits are incomplete")
	}
	for _, commit := range r.TaskCommits {
		if !objectID.MatchString(commit) {
			return fmt.Errorf("task commit is not a full object id")
		}
	}
	if !orderedSequence(r.ScopeSequence, []string{"prepare", "start", "acknowledge"}) || !r.ScopeAcknowledged {
		return fmt.Errorf("scope sequence was not acknowledged")
	}
	if !objectID.MatchString(r.PublicationRevision) || r.PublicationRevision != r.ResultsCommit || !objectID.MatchString(r.AttachmentRevision) || !objectID.MatchString(r.ReviewHEAD) {
		return fmt.Errorf("publication, attachment, or review evidence is incomplete")
	}
	if r.ImplementationGate != "passed" || r.FinalGate != "passed" || r.FinalGateCommit != r.ResultsCommit || r.TerminalVerdict != "gate-stop" {
		return fmt.Errorf("gate evidence is incomplete")
	}
	if r.EvidenceAuditComplete {
		return fmt.Errorf("native acceptance cannot claim hard-isolation audit completeness")
	}
	return nil
}

func sha256Digest(value string) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == 32 && value == stringLower(value)
}

func stringLower(value string) string {
	for _, r := range value {
		if r >= 'A' && r <= 'F' {
			return ""
		}
	}
	return value
}

func validAcceptanceRoleSequence(roles []string) bool {
	return len(roles) == 4 && roles[0] == "docket-implement-next" && roles[1] == "docket-plan-writer" &&
		len(roles[2]) > len("docket-build-") && roles[2][:len("docket-build-")] == "docket-build-" &&
		len(roles[3]) > len("docket-review-") && roles[3][:len("docket-review-")] == "docket-review-"
}

func orderedSequence(got, want []string) bool {
	index := 0
	for _, value := range got {
		if index < len(want) && value == want[index] {
			index++
		}
	}
	return index == len(want)
}

func canonicalAbsolute(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path
}
