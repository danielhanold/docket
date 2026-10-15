package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// TestSchemaPublishesFindingImpact pins that the impact part of a finding is
// published on both result documents that carry record-level findings.
//
// status findings are package-app structs, so the schema descends into them and
// the key path is checked exactly. repository.check findings are
// reposetup.Finding, a foreign struct describeField publishes as an opaque
// "object" with no nested keys; there the schema pins the findings key and the
// JSON wire form pins the impact key (present when set, omitted when empty).
func TestSchemaPublishesFindingImpact(t *testing.T) {
	keys := schemaKeyPaths(t)
	for _, want := range []string{"status RES findings.impact", "repository.check RES findings"} {
		if !keys[want] {
			t.Errorf("schema lacks key path %q", want)
		}
	}

	res := RepositoryCheckResult{Findings: []reposetup.Finding{
		{Code: "c", Severity: reposetup.SeverityWarning, Message: "m", Impact: "it matters"},
		{Code: "d", Severity: reposetup.SeverityWarning, Message: "m"},
	}}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Findings) != 2 {
		t.Fatalf("findings = %d, want 2: %s", len(doc.Findings), raw)
	}
	if got, ok := doc.Findings[0]["impact"]; !ok || got != "it matters" {
		t.Errorf("repository.check finding impact = %v (present %v), want %q: %s", got, ok, "it matters", raw)
	}
	if _, ok := doc.Findings[1]["impact"]; ok {
		t.Errorf("empty impact must be omitted: %s", raw)
	}
	if strings.Contains(string(raw), `"Impact"`) {
		t.Errorf("impact must be spelled lower-case on the wire: %s", raw)
	}
}
