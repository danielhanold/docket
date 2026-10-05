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

// TestRepositoryCheckSupplementalFindingsSerialize: supplemental condition
// findings ride the existing findings pipeline — same JSON serialization and
// the shared human renderer, no envelope change (change 0418).
func TestRepositoryCheckSupplementalFindingsSerialize(t *testing.T) {
	findings := []reposetup.Finding{
		{Code: "postconditions-unmet", Severity: reposetup.SeverityError,
			Message: "The metadata branch exists but not every health postcondition is satisfied.",
			Remedy:  "Resolve the reported issues, then re-run `docket repository check`."},
		{Code: "committed-ignore-invalid", Severity: reposetup.SeverityError, Ref: ".gitignore",
			Message: "The committed .gitignore's managed docket block is missing required entries: .opencode/agents/docket-*.md.",
			Remedy:  "Restore the missing entries (.opencode/agents/docket-*.md) to the managed block, then review, commit, and push the corrected .gitignore."},
	}
	res := newCheckResult(
		reposetup.Classification{State: reposetup.StateConflict, Reasons: []string{"postconditions-unmet"}},
		reposetup.Facts{}, findings)
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"committed-ignore-invalid"`, `".gitignore"`, `.opencode/agents/docket-\*.md`} {
		if !strings.Contains(strings.ReplaceAll(string(raw), `\`, ``), strings.ReplaceAll(want, `\`, ``)) {
			t.Fatalf("JSON lacks %q: %s", want, raw)
		}
	}
	human := res.HumanText()
	if !strings.Contains(human, "committed-ignore-invalid") || !strings.Contains(human, ".opencode/agents/docket-*.md") {
		t.Fatalf("human text lacks the supplemental finding: %q", human)
	}
	if res.CheckExitCode() != 1 {
		t.Fatalf("exit = %d, want 1", res.CheckExitCode())
	}
}

// TestRepositoryCheckPrintsCommittedIgnoreBlockFlushLeft pins that `repository
// check`'s human text prints the committed-ignore-invalid remedy's canonical
// block with every line at column 0 — paste-ready, since leading whitespace is
// part of a .gitignore pattern (change 0500). The finding comes from the real
// EvaluateHealth pipeline over facts carrying an IgnoreDefectFileAbsent detail,
// not a hand-built Finding.
//
// Mutation probes (each must redden this test): make reposetup's
// withCanonicalBlock indent the block lines, or join instruction and block with
// a space; make appendFindingBlock indent remedy continuation lines.
func TestRepositoryCheckPrintsCommittedIgnoreBlockFlushLeft(t *testing.T) {
	facts := healthyConfigureFacts()
	facts.CommittedIgnoreBlock = reposetup.PresenceAbsent
	facts.CommittedIgnoreDetail = reposetup.IgnoreDetail{Defect: reposetup.IgnoreDefectFileAbsent}
	cls := reposetup.Classify(facts)
	findings := reposetup.EvaluateHealth(cls, facts, nil)
	found := false
	for _, f := range findings {
		if f.Code == "committed-ignore-invalid" {
			found = true
		}
	}
	if !found {
		t.Fatalf("fixture produced no committed-ignore-invalid finding (state %q, codes %v)", cls.State, findings)
	}
	block := strings.TrimSuffix(string(reposetup.GitignoreBlock()), "\n")
	text := newCheckResult(cls, facts, findings).HumanText()
	if !strings.Contains(text, "\n"+block) {
		t.Fatalf("check text must carry the canonical block flush left (newline, then the block verbatim):\n%s", text)
	}
}

// TestLocalMetadataRefusalsMatchPrepare: check's ahead/diverged findings and
// prepare's refusals for the same state carry the same code, message, and remedy,
// so the two commands never describe one state two ways.
func TestLocalMetadataRefusalsMatchPrepare(t *testing.T) {
	for _, tc := range []struct {
		rel    reposetup.SyncRelation
		reason string
	}{
		{reposetup.SyncAhead, "local-metadata-ahead"},
		{reposetup.SyncDiverged, "local-metadata-diverged"},
	} {
		f := preparableFacts()
		f.LocalMetadataSync = tc.rel
		f.DocketWorktree.Synchronized = reposetup.PresenceAbsent
		v := prepareRoute(f, prepareHolder{presence: reposetup.PresenceAbsent})
		if v.finding == nil {
			t.Fatalf("%s: prepare produced no finding", tc.reason)
		}
		got := reposetup.EvaluateHealth(reposetup.Classification{State: reposetup.StateConflict, Reasons: []string{tc.reason}}, reposetup.Facts{}, nil)
		if len(got) != 1 {
			t.Fatalf("%s: check findings = %+v", tc.reason, got)
		}
		if got[0].Code != v.finding.Code || got[0].Message != v.finding.Message || got[0].Remedy != v.finding.Remedy {
			t.Errorf("%s: check %+v != prepare %+v", tc.reason, got[0], *v.finding)
		}
	}

	// A dirty worktree, with and without an unfinished Git operation: prepare's
	// refusal message is check's metadata-worktree-dirty message for the same facts.
	for _, op := range []bool{false, true} {
		f := preparableFacts()
		f.DocketWorktree.Clean = reposetup.PresenceAbsent
		f.DocketWorktree.UnfinishedOperation = op
		v := prepareRoute(f, prepareHolder{presence: reposetup.PresenceAbsent})
		if v.finding == nil {
			t.Fatalf("dirty (operation=%v): prepare produced no finding", op)
		}
		got := reposetup.EvaluateHealth(
			reposetup.Classification{State: reposetup.StateConflict, Reasons: []string{"metadata-worktree-dirty"}},
			reposetup.Facts{DocketWorktree: reposetup.WorktreeFact{UnfinishedOperation: op}}, nil)
		if len(got) != 1 {
			t.Fatalf("dirty (operation=%v): check findings = %+v", op, got)
		}
		if got[0].Code != v.finding.Code || got[0].Message != v.finding.Message {
			t.Errorf("dirty (operation=%v): check %+v != prepare %+v", op, got[0], *v.finding)
		}
	}

	// An interrupted in-place fast-forward: check and prepare carry the same code,
	// message, AND remedy, and the remedy never invites committing the revert.
	f := preparableFacts()
	f.DocketWorktree.Clean = reposetup.PresenceAbsent
	f.DocketWorktree.InterruptedFastForward = true
	v := prepareRoute(f, prepareHolder{presence: reposetup.PresenceAbsent})
	if v.finding == nil {
		t.Fatal("interrupted fast-forward: prepare produced no finding")
	}
	got := reposetup.EvaluateHealth(
		reposetup.Classification{State: reposetup.StateConflict, Reasons: []string{"metadata-worktree-dirty"}},
		reposetup.Facts{DocketWorktree: reposetup.WorktreeFact{InterruptedFastForward: true}}, nil)
	if len(got) != 1 {
		t.Fatalf("interrupted fast-forward: check findings = %+v", got)
	}
	if got[0].Code != v.finding.Code || got[0].Message != v.finding.Message || got[0].Remedy != v.finding.Remedy {
		t.Errorf("interrupted fast-forward: check %+v != prepare %+v", got[0], *v.finding)
	}
	if got[0].Message != reposetup.InterruptedFastForwardMessage || got[0].Remedy != reposetup.InterruptedFastForwardRemedy {
		t.Errorf("interrupted fast-forward: finding %+v, want the interrupted message and remedy", got[0])
	}
}
