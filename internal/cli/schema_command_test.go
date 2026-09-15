package cli

// End-to-end tests of the assembled `docket schema` command. They drive the
// real Run entry (buffers, like every other cli test) and assert the contract
// the schema surface promises consumers: a protocol-v1 document callable from a
// bare non-git directory that writes nothing, a single-operation filter, a
// fail-closed unknown-operation refusal, and the read-only capability posture.
// The repository-independence proof mirrors capabilities_command_test.go /
// capability_production_test.go — the same harness, not a new one.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestSchemaCommandRepositoryIndependent proves the same posture capabilities
// holds: callable in a bare non-git temp dir, an applied protocol document, and
// nothing written to disk.
func TestSchemaCommandRepositoryIndependent(t *testing.T) {
	dir := testsupport.TempDir(t)
	home := testsupport.TempDir(t)
	xdg := testsupport.TempDir(t)
	t.Chdir(dir)
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", xdg)

	out, errS, code := runCLI(t, "schema", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("schema did not answer cleanly from an empty non-git dir: out=%q err=%q code=%d", out, errS, code)
	}

	var doc struct {
		ProtocolVersion int `json:"protocol_version"`
		SchemaVersion   int `json:"schema_version"`
		Operations      []struct {
			ID string `json:"id"`
		} `json:"operations"`
		Vocabularies map[string]json.RawMessage `json:"vocabularies"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("schema document is not valid JSON: %v\n%s", err, out)
	}
	if doc.SchemaVersion != 1 || doc.ProtocolVersion != 1 {
		t.Errorf("versions = protocol %d schema %d, want 1/1", doc.ProtocolVersion, doc.SchemaVersion)
	}
	if len(doc.Operations) == 0 || len(doc.Vocabularies) == 0 {
		t.Errorf("empty surface: %d operations, %d vocabularies", len(doc.Operations), len(doc.Vocabularies))
	}

	// Write-independence: every directory it was pointed at stays empty.
	for _, d := range []string{dir, home, xdg} {
		entries, err := os.ReadDir(d)
		if err != nil {
			t.Fatalf("reading %q: %v", d, err)
		}
		if len(entries) != 0 {
			t.Errorf("schema wrote to %q: %v", d, entries)
		}
	}
}

// TestSchemaCommandSingleOperation proves --operation filters to exactly one
// operation and surfaces its real request keys, and that an unknown id fails
// closed as invalid-input (exit 2) carrying the unknown-operation finding code.
func TestSchemaCommandSingleOperation(t *testing.T) {
	out, errS, code := runCLI(t, "schema", "--operation", "change.create", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("schema --operation change.create: out=%q err=%q code=%d", out, errS, code)
	}
	var one struct {
		Operations []struct {
			ID      string `json:"id"`
			Request *struct {
				Fields []struct {
					Key string `json:"key"`
				} `json:"fields"`
			} `json:"request"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(out), &one); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if len(one.Operations) != 1 || one.Operations[0].ID != "change.create" {
		t.Fatalf("--operation change.create returned %d operations, want exactly change.create", len(one.Operations))
	}
	if one.Operations[0].Request == nil {
		t.Fatal("change.create request descriptor is absent")
	}
	keys := map[string]bool{}
	for _, f := range one.Operations[0].Request.Fields {
		keys[f.Key] = true
	}
	for _, want := range []string{"title", "type", "priority", "related"} {
		if !keys[want] {
			t.Errorf("change.create request is missing key %q; got %v", want, keys)
		}
	}

	// Unknown operation fails closed: invalid-input, exit 2, unknown-operation.
	badOut, _, badCode := runCLI(t, "schema", "--operation", "no.such.operation", "--json")
	if badCode != 2 {
		t.Fatalf("unknown --operation exit = %d, want 2\n%s", badCode, badOut)
	}
	var bad struct {
		Result   string `json:"result"`
		Findings []struct {
			Code string `json:"code"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(badOut), &bad); err != nil {
		t.Fatalf("refusal is not valid JSON: %v\n%s", err, badOut)
	}
	if bad.Result != "invalid-input" {
		t.Errorf("unknown-operation result = %q, want invalid-input", bad.Result)
	}
	foundCode := false
	for _, f := range bad.Findings {
		if f.Code == "unknown-operation" {
			foundCode = true
		}
	}
	if !foundCode {
		t.Errorf("unknown-operation refusal missing finding code unknown-operation: %v", bad.Findings)
	}
}

func TestAgentCheckInputsSchemaDisclosesConstructibleDocuments(t *testing.T) {
	out, errS, code := runCLI(t, "schema", "--operation", "agent.check-inputs", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("schema --operation agent.check-inputs: out=%q err=%q code=%d", out, errS, code)
	}
	var doc struct {
		Operations []struct {
			Request struct {
				Fields []struct {
					Key string `json:"key"`
				} `json:"fields"`
			} `json:"request"`
			Documents []struct {
				ID            string `json:"id"`
				SchemaVersion int    `json:"schema_version"`
				Body          struct {
					Fields []struct {
						Key         string `json:"key"`
						Enum        string `json:"enum"`
						Description string `json:"description"`
						Fields      []struct {
							Key string `json:"key"`
						} `json:"fields"`
					} `json:"fields"`
				} `json:"body"`
			} `json:"documents"`
		} `json:"operations"`
		Vocabularies map[string]struct {
			Members []string `json:"members"`
		} `json:"vocabularies"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Operations) != 1 {
		t.Fatalf("operations=%d", len(doc.Operations))
	}
	req := map[string]bool{}
	for _, f := range doc.Operations[0].Request.Fields {
		req[f.Key] = true
	}
	for _, key := range []string{"assignment", "sha256", "payload", "payload_sha256", "stage"} {
		if !req[key] {
			t.Errorf("check-inputs request omits %q", key)
		}
	}
	docs := map[string]map[string]struct {
		enum   string
		nested []string
	}{}
	for _, d := range doc.Operations[0].Documents {
		if d.SchemaVersion != 1 {
			t.Errorf("document %s schema_version=%d", d.ID, d.SchemaVersion)
		}
		fields := map[string]struct {
			enum   string
			nested []string
		}{}
		for _, f := range d.Body.Fields {
			var nested []string
			for _, n := range f.Fields {
				nested = append(nested, n.Key)
			}
			fields[f.Key] = struct {
				enum   string
				nested []string
			}{f.Enum, nested}
		}
		docs[d.ID] = fields
	}
	assignment := docs["assignment"]
	for _, key := range []string{"role", "phase", "mode", "resources", "resource_dependencies", "root_identity", "inherited_fingerprint", "build_evidence"} {
		if _, ok := assignment[key]; !ok {
			t.Errorf("assignment document omits %q", key)
		}
	}
	if len(assignment["root_identity"].nested) == 0 || len(assignment["inherited_fingerprint"].nested) == 0 {
		t.Error("assignment identity/fingerprint shapes are opaque")
	}
	if assignment["role"].enum != "agent_roles" || assignment["mode"].enum != "assignment_modes" {
		t.Errorf("assignment enum references are incomplete: role=%q mode=%q", assignment["role"].enum, assignment["mode"].enum)
	}
	var assignmentDescriptions = map[string]string{}
	for _, d := range doc.Operations[0].Documents {
		if d.ID != "assignment" {
			continue
		}
		for _, f := range d.Body.Fields {
			assignmentDescriptions[f.Key] = f.Description
		}
	}
	for _, key := range []string{"plan_skill", "build_skill", "results_template"} {
		if !strings.Contains(assignmentDescriptions[key], "resources[].logical_id") {
			t.Errorf("assignment %s does not disclose logical-id selector semantics: %q", key, assignmentDescriptions[key])
		}
	}
	for _, clause := range []string{"every declared resource", "direct local Markdown dependencies"} {
		if !strings.Contains(assignmentDescriptions["resource_dependencies"], clause) {
			t.Errorf("resource_dependencies description omits %q: %q", clause, assignmentDescriptions["resource_dependencies"])
		}
	}
	payload := docs["worker-payload"]
	for _, key := range []string{"kind", "assignment_path", "assignment_sha256", "entry_argv", "attempt", "resolver_reservation"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("worker-payload document omits %q", key)
		}
	}
	for _, vocab := range []string{"agent_roles", "assignment_phases", "assignment_modes", "payload_kinds"} {
		if len(doc.Vocabularies[vocab].Members) == 0 {
			t.Errorf("vocabulary %q is absent or empty", vocab)
		}
	}
}

// TestSchemaCapabilityAnnotation walks the real production tree and pins the
// schema entry's read-only posture — a write effect must never creep in.
func TestSchemaCapabilityAnnotation(t *testing.T) {
	entries, err := collectCapabilities(productionRootForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	e, ok := entryByID(entries, "schema")
	if !ok {
		t.Fatal("schema is absent from the capability catalog")
	}
	if len(e.Effects) != 1 || e.Effects[0] != string(EffectRead) {
		t.Errorf("schema effects = %v, want [read]", e.Effects)
	}
}
