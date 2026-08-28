package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// Change 0363 Task 5: the mode-shaped fields are removed from serialized
// protocol output. They are absent — not empty, not a constant compatibility
// value. Clients key on the surviving revision/integration identity instead.

// modeShapedProtocolKeys is the closed set of JSON keys change 0363 removes from
// public protocol output. metadata_revision and the integration/default branch
// identity keys survive and are asserted present elsewhere.
var modeShapedProtocolKeys = []string{
	"metadata_mode",
	"repo_mode",
}

func TestStatusContextProtocolOmitsModeFields(t *testing.T) {
	// A fully-populated StatusContext must never serialize a mode-shaped key.
	ctx := StatusContext{
		DefaultBranch:         "main",
		DefaultBranchRevision: "0000000000000000000000000000000000000000",
		IntegrationBranch:     "main",
		IntegrationRevision:   "1111111111111111111111111111111111111111",
		MetadataRevision:      "2222222222222222222222222222222222222222",
	}
	b, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal StatusContext: %v", err)
	}
	got := string(b)
	for _, key := range []string{"metadata_mode", "metadata_branch"} {
		if strings.Contains(got, `"`+key+`"`) {
			t.Errorf("StatusContext serialization still carries removed key %q: %s", key, got)
		}
	}
	// The surviving identity must remain.
	if !strings.Contains(got, `"metadata_revision"`) {
		t.Errorf("StatusContext dropped metadata_revision, which must survive: %s", got)
	}
	if !strings.Contains(got, `"integration_branch"`) {
		t.Errorf("StatusContext dropped integration_branch, which must survive: %s", got)
	}
}

func TestFinalizeAndWorkflowContextOmitRepoMode(t *testing.T) {
	fp, err := json.Marshal(FinalizePolicy{})
	if err != nil {
		t.Fatalf("marshal FinalizePolicy: %v", err)
	}
	if strings.Contains(string(fp), `"repo_mode"`) {
		t.Errorf("FinalizePolicy still carries removed key repo_mode: %s", fp)
	}
	cw, err := json.Marshal(ContextWorkflow{})
	if err != nil {
		t.Fatalf("marshal ContextWorkflow: %v", err)
	}
	if strings.Contains(string(cw), `"repo_mode"`) {
		t.Errorf("ContextWorkflow still carries removed key repo_mode: %s", cw)
	}
}
