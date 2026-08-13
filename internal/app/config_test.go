package app

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

// The sources below are built in memory rather than read from
// testdata/repositories/: this package tests the MAPPING from a resolution to
// a protocol document, and internal/config already owns the end-to-end fixture
// tier. Keeping the inputs here inline also keeps internal/app free of any
// filesystem dependency, which is what lets these tests say something about
// the mapping alone.

// sparseSources is a repository that declares nothing: every leaf resolves to
// the built-in layer and nothing blocks a mutation.
func sparseSources() []config.Source { return nil }

// blockedSources reproduces the shape of docket's own configuration — three
// deferred capabilities requested from the committed repository file and one
// from the machine-global layer, for four blockers spanning two layers.
func blockedSources() []config.Source {
	return []config.Source{
		{
			Layer: config.LayerGlobal,
			Name:  "/tmp/xdg/docket/config.yml",
			Data:  []byte("# GLOBAL-SECRET-COMMENT\nauto_capture:\n  enabled: true\n"),
		},
		{
			Layer: config.LayerRepository,
			Name:  ".docket.yml",
			Data: []byte("# REPO-SECRET-COMMENT\n" +
				"metadata_branch: docket\n" +
				"integration_branch: main\n" +
				"terminal_publish: true\n" +
				"finalize:\n  skip_results_only_delta: true\n" +
				"build:\n  checkpoint: true\n"),
		},
	}
}

func mainCtx() config.ResolveContext { return config.ResolveContext{DefaultBranch: "main"} }

func blockerCount(r ConfigInspectionResult) int {
	n := 0
	for _, d := range r.Diagnostics {
		if d.Code == config.CodeDeferredCapRequested {
			n++
		}
	}
	return n
}

// TestDiagnosticConfigApplied: the inspection operation over a repository that
// declares nothing at all.
func TestDiagnosticConfigApplied(t *testing.T) {
	got := DiagnosticConfig(sparseSources(), mainCtx(), false)

	if got.Operation != "diagnostic.config" {
		t.Errorf("operation = %q, want diagnostic.config", got.Operation)
	}
	if got.Result != ResultApplied {
		t.Errorf("result = %q, want %q", got.Result, ResultApplied)
	}
	if got.ProtocolVersion != ProtocolVersion {
		t.Errorf("protocol_version = %d, want %d", got.ProtocolVersion, ProtocolVersion)
	}
	if got.SourceMode != "filesystem" {
		t.Errorf("source_mode = %q, want filesystem", got.SourceMode)
	}
	if !got.MutationAllowed {
		t.Errorf("mutation_allowed = false on a repository that declares nothing")
	}
	if got.Effective == nil {
		t.Fatalf("effective is nil on an applied result")
	}
	if got.Effective.MetadataBranch.Value != "docket" {
		t.Errorf("effective.metadata_branch = %q, want docket", got.Effective.MetadataBranch.Value)
	}
	if got.Capabilities == nil {
		t.Errorf("capabilities is nil on a valid snapshot; want a non-nil (possibly empty) slice")
	}
	if got.Diagnostics == nil {
		t.Errorf("diagnostics is nil; the field is never omitted")
	}
	if got.Reason != "" || got.Message != "" {
		t.Errorf("applied result carries reason %q / message %q, want both empty", got.Reason, got.Message)
	}
	if code := ExitCode(got.Result); code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
}

// TestDiagnosticConfigAppliedWhileBlocked: inspection never fails on a blocked
// configuration. The block is DATA under an applied result — reading a
// configuration you may not mutate is exactly what inspection is for.
func TestDiagnosticConfigAppliedWhileBlocked(t *testing.T) {
	got := DiagnosticConfig(blockedSources(), mainCtx(), false)

	if got.Operation != "diagnostic.config" {
		t.Errorf("operation = %q, want diagnostic.config", got.Operation)
	}
	if got.Result != ResultApplied {
		t.Fatalf("result = %q, want %q (a blocked configuration still inspects)", got.Result, ResultApplied)
	}
	if got.MutationAllowed {
		t.Errorf("mutation_allowed = true on a configuration requesting deferred capabilities")
	}
	if got.Effective == nil {
		t.Errorf("effective is nil, but the snapshot is valid")
	}
	if got.Reason != "" {
		t.Errorf("reason = %q on an applied result, want empty", got.Reason)
	}
	if n := blockerCount(got); n != 4 {
		t.Errorf("blocker count = %d, want 4; diagnostics: %+v", n, got.Diagnostics)
	}
}

// TestPreflightUnsupported: the same configuration under --for-mutation is the
// refusal, and it names every blocker rather than the first.
func TestPreflightUnsupported(t *testing.T) {
	got := DiagnosticConfig(blockedSources(), mainCtx(), true)

	if got.Operation != "config.preflight" {
		t.Errorf("operation = %q, want config.preflight", got.Operation)
	}
	if got.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q, want %q", got.Result, ResultUnsupportedConfig)
	}
	if got.Reason != ReasonDeferredCapRequested {
		t.Errorf("reason = %q, want %q", got.Reason, ReasonDeferredCapRequested)
	}
	if got.Message == "" {
		t.Errorf("message is empty on an unsupported-config result")
	}
	if got.MutationAllowed {
		t.Errorf("mutation_allowed = true on an unsupported-config result")
	}
	if got.Effective == nil {
		t.Errorf("effective is nil, but the snapshot is valid")
	}

	var blockers []string
	for _, d := range got.Diagnostics {
		if d.Code == config.CodeDeferredCapRequested {
			blockers = append(blockers, d.Path)
		}
	}
	sort.Strings(blockers)
	want := []string{"auto_capture.enabled", "build.checkpoint", "finalize.skip_results_only_delta", "terminal_publish"}
	if strings.Join(blockers, ",") != strings.Join(want, ",") {
		t.Errorf("blockers = %v, want %v", blockers, want)
	}
	if code := ExitCode(got.Result); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
}

// TestPreflightAllowedResult: a configuration with nothing deferred passes the
// mutation preflight.
func TestPreflightAllowedResult(t *testing.T) {
	got := DiagnosticConfig(sparseSources(), mainCtx(), true)

	if got.Operation != "config.preflight" {
		t.Errorf("operation = %q, want config.preflight", got.Operation)
	}
	if got.Result != ResultApplied {
		t.Errorf("result = %q, want %q", got.Result, ResultApplied)
	}
	if !got.MutationAllowed {
		t.Errorf("mutation_allowed = false on an unblocked preflight")
	}
	if got.Reason != "" {
		t.Errorf("reason = %q, want empty", got.Reason)
	}
}

// TestInvalidConfigResult: a malformed layer is invalid INPUT, not an
// unsupported configuration — different reason, different exit code.
func TestInvalidConfigResult(t *testing.T) {
	sources := []config.Source{{
		Layer: config.LayerRepository,
		Name:  ".docket.yml",
		Data:  []byte("a: [unclosed\n"),
	}}
	got := DiagnosticConfig(sources, mainCtx(), false)

	if got.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want %q", got.Result, ResultInvalidInput)
	}
	if got.Reason != ReasonInvalidConfig {
		t.Errorf("reason = %q, want %q", got.Reason, ReasonInvalidConfig)
	}
	if got.Message == "" {
		t.Errorf("message is empty on an invalid-input result")
	}
	if got.Effective != nil {
		t.Errorf("effective is non-nil on an invalid configuration: %+v", got.Effective)
	}
	if got.Capabilities != nil {
		t.Errorf("capabilities is non-nil on an invalid configuration: %+v", got.Capabilities)
	}
	if got.MutationAllowed {
		t.Errorf("mutation_allowed = true on an invalid configuration")
	}
	if len(got.Diagnostics) == 0 {
		t.Errorf("no diagnostics on an invalid configuration; parsing produced none")
	}
	if code := ExitCode(got.Result); code != 2 {
		t.Errorf("exit code %d, want 2", code)
	}
}

// TestInvalidConfigResultUnderPreflight: the mode still selects the operation
// name on the invalid path — the failure does not rename the operation.
func TestInvalidConfigResultUnderPreflight(t *testing.T) {
	sources := []config.Source{{
		Layer: config.LayerRepository,
		Name:  ".docket.yml",
		Data:  []byte("a: [unclosed\n"),
	}}
	got := DiagnosticConfig(sources, mainCtx(), true)

	if got.Operation != "config.preflight" {
		t.Errorf("operation = %q, want config.preflight", got.Operation)
	}
	if got.Result != ResultInvalidInput || got.Reason != ReasonInvalidConfig {
		t.Errorf("result/reason = %q/%q, want invalid-input/invalid-config", got.Result, got.Reason)
	}
}

// TestMissingContextResult: `integration_branch: auto` with no default branch
// supplied is a distinct, separately-spelled failure.
func TestMissingContextResult(t *testing.T) {
	got := DiagnosticConfig(sparseSources(), config.ResolveContext{}, false)

	if got.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want %q", got.Result, ResultInvalidInput)
	}
	if got.Reason != ReasonMissingResolutionContext {
		t.Errorf("reason = %q, want %q", got.Reason, ReasonMissingResolutionContext)
	}
	if got.Effective != nil {
		t.Errorf("effective is non-nil without a resolution context")
	}
	if code := ExitCode(got.Result); code != 2 {
		t.Errorf("exit code %d, want 2", code)
	}
}

// TestHumanTextGrouping: the human rendering is grouped, labeled, and leaks
// nothing from the files it read.
func TestHumanTextGrouping(t *testing.T) {
	text := DiagnosticConfig(blockedSources(), mainCtx(), false).HumanText()

	for _, want := range []string{
		"configuration: valid",
		"mutation: blocked (4 blockers)",
		"effective (winning layer):",
		"metadata_branch = docket",
		"capabilities:",
		"diagnostics:",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("HumanText is missing %q\n---\n%s", want, text)
		}
	}

	// The metadata_branch line names the layer it won from: a value without a
	// layer does not tell the reader which file to edit.
	var effLine string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "metadata_branch = ") {
			effLine = line
		}
	}
	if !strings.Contains(effLine, "repository") {
		t.Errorf("metadata_branch line %q does not name its winning layer", effLine)
	}

	// Diagnostics are grouped by severity with errors first.
	firstDiag := -1
	firstInfo := -1
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if firstDiag == -1 && strings.HasPrefix(trimmed, "error ") {
			firstDiag = i
		}
		if firstInfo == -1 && strings.HasPrefix(trimmed, "info ") {
			firstInfo = i
		}
	}
	if firstDiag == -1 {
		t.Errorf("no error-severity diagnostic line in:\n%s", text)
	}
	if firstInfo != -1 && firstInfo < firstDiag {
		t.Errorf("an info diagnostic precedes the first error diagnostic:\n%s", text)
	}

	// No raw file contents and no environment values: the renderer reports
	// settings and source NAMES, never bytes it read.
	for _, forbidden := range []string{"REPO-SECRET-COMMENT", "GLOBAL-SECRET-COMMENT"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("HumanText leaks raw file content %q:\n%s", forbidden, text)
		}
	}
}

// TestHumanTextInvalid: on an invalid configuration the effective section is
// replaced rather than rendered from a zero value.
func TestHumanTextInvalid(t *testing.T) {
	sources := []config.Source{{
		Layer: config.LayerRepository,
		Name:  ".docket.yml",
		Data:  []byte("a: [unclosed\n"),
	}}
	text := DiagnosticConfig(sources, mainCtx(), false).HumanText()

	for _, want := range []string{
		"configuration: invalid",
		"mutation: n/a",
		"effective: (unavailable — configuration invalid)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("HumanText is missing %q\n---\n%s", want, text)
		}
	}
	if strings.Contains(text, "metadata_branch = ") {
		t.Errorf("HumanText rendered effective leaves on an invalid configuration:\n%s", text)
	}
}

// TestJSONShape: the applied document carries exactly the Reference D keys.
// It resolves a configuration that HAS capabilities to report, because an
// empty capability list is omitted by `omitempty` and would make the key-set
// assertion say less than it looks like it says.
func TestJSONShape(t *testing.T) {
	raw, err := json.Marshal(DiagnosticConfig(blockedSources(), mainCtx(), false))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := []string{
		"protocol_version", "operation", "result", "source_mode",
		"mutation_allowed", "effective", "capabilities", "diagnostics",
	}
	wantSet := make(map[string]bool, len(want))
	for _, k := range want {
		wantSet[k] = true
		if _, ok := got[k]; !ok {
			t.Errorf("key %q is absent from the applied document", k)
		}
	}
	for k := range got {
		if !wantSet[k] {
			t.Errorf("unexpected key %q in the applied document", k)
		}
	}
	if string(got["capabilities"]) == "null" {
		t.Errorf("capabilities marshalled as JSON null; want an array")
	}
}

// TestJSONNoNullArrays: a valid snapshot with nothing to report must never
// marshal a null in place of an array — a consumer iterating the document
// should not have to distinguish "empty" from "absent" from "null".
func TestJSONNoNullArrays(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sources []config.Source
		rctx    config.ResolveContext
	}{
		{"applied", sparseSources(), mainCtx()},
		{"invalid", []config.Source{{
			Layer: config.LayerRepository,
			Name:  ".docket.yml",
			Data:  []byte("a: [unclosed\n"),
		}}, mainCtx()},
		{"missing-context", sparseSources(), config.ResolveContext{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(DiagnosticConfig(tc.sources, tc.rctx, false))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			for _, key := range []string{"capabilities", "diagnostics"} {
				if v, ok := got[key]; ok && string(v) == "null" {
					t.Errorf("%s marshalled as JSON null", key)
				}
			}
			if _, ok := got["diagnostics"]; !ok {
				t.Errorf("diagnostics is absent; the field is never omitted")
			}
		})
	}
}

// TestJSONShapeInvalid: the invalid document omits the snapshot halves and
// carries the failure fields instead.
func TestJSONShapeInvalid(t *testing.T) {
	sources := []config.Source{{
		Layer: config.LayerRepository,
		Name:  ".docket.yml",
		Data:  []byte("a: [unclosed\n"),
	}}
	raw, err := json.Marshal(DiagnosticConfig(sources, mainCtx(), false))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, absent := range []string{"effective", "capabilities"} {
		if _, ok := got[absent]; ok {
			t.Errorf("key %q is present on an invalid-input document", absent)
		}
	}
	for _, present := range []string{"reason", "message", "diagnostics"} {
		if _, ok := got[present]; !ok {
			t.Errorf("key %q is absent from an invalid-input document", present)
		}
	}
}

// TestDiagnosticConfigInternalError: a caller-contract violation — here the
// source layers handed to Resolve out of precedence order — is docket's own
// bug, not a bad .docket.yml. It must surface as an internal error rather than
// collapse into invalid-input, which would send the user off to edit a valid
// configuration with no diagnostics explaining what to change.
func TestDiagnosticConfigInternalError(t *testing.T) {
	misordered := []config.Source{
		{Layer: config.LayerRepository, Name: ".docket.yml"},
		{Layer: config.LayerGlobal, Name: "/tmp/xdg/docket/config.yml"},
	}
	got := DiagnosticConfig(misordered, mainCtx(), false)

	if got.Result != ResultInternalError {
		t.Errorf("result = %q, want %q", got.Result, ResultInternalError)
	}
	if got.Reason != ReasonInternalError {
		t.Errorf("reason = %q, want %q", got.Reason, ReasonInternalError)
	}
	if got.Message == "" {
		t.Error("message is empty; the resolver's own error text must survive")
	}
	if got.Effective != nil {
		t.Error("effective is present on a failure document")
	}
}
