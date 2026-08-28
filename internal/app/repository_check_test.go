package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// TestRepositoryCheckExitMapping proves CheckExitCode honors the 0/1/2 contract
// for every classified state: healthy → 0, unknown → 2, every diagnosed state →
// 1, and a healthy classification that nonetheless carries findings is never
// reported clean.
func TestRepositoryCheckExitMapping(t *testing.T) {
	finding := reposetup.Finding{Code: "x", Severity: reposetup.SeverityError}
	cases := []struct {
		state    reposetup.State
		findings []reposetup.Finding
		want     int
	}{
		{reposetup.StateHealthy, nil, 0},
		{reposetup.StateHealthy, []reposetup.Finding{finding}, 1},
		{reposetup.StateUnknown, nil, 2},
		{reposetup.StateFresh, []reposetup.Finding{finding}, 1},
		{reposetup.StateLegacy, []reposetup.Finding{finding}, 1},
		{reposetup.StateNeedsReview, []reposetup.Finding{finding}, 1},
		{reposetup.StatePartial, []reposetup.Finding{finding}, 1},
		{reposetup.StateConflict, []reposetup.Finding{finding}, 1},
	}
	for _, tc := range cases {
		r := RepositoryCheckResult{RepositoryState: string(tc.state), Findings: tc.findings}
		if got := r.CheckExitCode(); got != tc.want {
			t.Errorf("state %q with %d findings: CheckExitCode = %d, want %d", tc.state, len(tc.findings), got, tc.want)
		}
	}
}

// TestRepositoryCheckResultMapping proves the read-only result mapping: an
// undeterminable authority (unknown) is an invalid-state family result, and
// every determinable state is a read-only no-op.
func TestRepositoryCheckResultMapping(t *testing.T) {
	unknown := newCheckResult(reposetup.Classification{State: reposetup.StateUnknown}, reposetup.Facts{}, nil)
	if unknown.Result != ResultInvalidState {
		t.Errorf("unknown Result = %q, want invalid-state", unknown.Result)
	}
	healthy := newCheckResult(reposetup.Classification{State: reposetup.StateHealthy}, reposetup.Facts{}, nil)
	if healthy.Result != ResultNoOp {
		t.Errorf("healthy Result = %q, want no-op", healthy.Result)
	}
}

// TestRepositoryCheckJSONHumanEquivalence proves the JSON and human renderings
// carry the same state and findings — a JSON consumer and a human read the same
// diagnosis.
func TestRepositoryCheckJSONHumanEquivalence(t *testing.T) {
	cls := reposetup.Classification{State: reposetup.StateFresh, Reasons: []string{"no-metadata-no-surface"}}
	facts := reposetup.Facts{RemoteIntegration: reposetup.BranchFact{Tip: "abc123"}}
	findings := reposetup.EvaluateHealth(cls, facts, nil)
	r := newCheckResult(cls, facts, findings)

	buf, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Operation       string              `json:"operation"`
		Result          string              `json:"result"`
		RepositoryState string              `json:"repository_state"`
		Findings        []reposetup.Finding `json:"findings"`
		Revisions       map[string]string   `json:"revisions"`
	}
	if err := json.Unmarshal(buf, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Operation != OperationRepositoryCheck {
		t.Errorf("operation = %q, want %q", decoded.Operation, OperationRepositoryCheck)
	}
	if decoded.RepositoryState != string(reposetup.StateFresh) {
		t.Errorf("json state = %q, want fresh", decoded.RepositoryState)
	}
	if len(decoded.Findings) != len(findings) || len(findings) == 0 {
		t.Fatalf("json findings = %d, struct findings = %d", len(decoded.Findings), len(findings))
	}
	if decoded.Findings[0].Code != findings[0].Code {
		t.Errorf("json finding code = %q, want %q", decoded.Findings[0].Code, findings[0].Code)
	}
	if decoded.Revisions["remote-integration"] != "abc123" {
		t.Errorf("json revisions = %v, want remote-integration abc123", decoded.Revisions)
	}
	human := r.HumanText()
	if !strings.Contains(human, findings[0].Code) {
		t.Errorf("human text %q does not name finding code %q", human, findings[0].Code)
	}
	if !strings.Contains(human, string(reposetup.StateFresh)) {
		t.Errorf("human text %q does not name the state", human)
	}
}

// TestRepositoryCheckJSONIncludesRepairable proves a frontmatter finding's
// repairable flag survives into the JSON, so a consumer can tell a mechanical
// repair apart from a manual-review finding.
func TestRepositoryCheckJSONIncludesRepairable(t *testing.T) {
	cls := reposetup.Classification{State: reposetup.StateNeedsReview, Reasons: []string{"pending-review-paths"}}
	fm := []reposetup.RepairFinding{{
		Path:       "docs/changes/active/0001-example.md",
		Field:      "title",
		Code:       reposetup.RepairQuoteScalar,
		Repairable: true,
		Message:    "unsafe scalar",
	}}
	findings := reposetup.EvaluateHealth(cls, reposetup.Facts{}, fm)
	r := newCheckResult(cls, reposetup.Facts{}, findings)

	buf, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(buf), `"repairable":true`) {
		t.Errorf("json %s does not carry repairable:true", buf)
	}
}

// TestCorpusFindingsSurfacesUndecodable proves the report-only corpus gatherer
// names an undecodable change record as a non-repairable frontmatter finding and
// never fabricates a repair for it.
func TestCorpusFindingsSurfacesUndecodable(t *testing.T) {
	recs := []corpusRecord{{
		path:     "docs/changes/active/0001-broken.md",
		bytes:    []byte("---\nid: 1\nid: 2\n---\nbody\n"), // duplicate key: undecodable
		kind:     repository.KindChange,
		location: repository.LocationActive,
	}}
	got := corpusFindings(config.Effective{}, recs)
	if len(got) == 0 {
		t.Fatal("corpusFindings returned no finding for an undecodable record")
	}
	for _, f := range got {
		if f.Repairable {
			t.Errorf("undecodable record produced a repairable finding: %+v", f)
		}
	}
}
