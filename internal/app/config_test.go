package app

import (
	"encoding/json"
	"fmt"
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
	// 0363 Task 5 restores a metadata-branch assertion here in its new shape:
	// config.Effective.MetadataBranch is gone (obsolete tombstone), so this pins
	// the surviving identity leaf instead.
	if got.Effective.IntegrationBranch.Value != "main" {
		t.Errorf("effective.integration_branch = %q, want main", got.Effective.IntegrationBranch.Value)
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
		"capabilities:",
		"diagnostics:",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("HumanText is missing %q\n---\n%s", want, text)
		}
	}

	// metadata_branch is no longer effective configuration: Go v1 supports one
	// metadata topology (the fixed orphan `docket` branch), so the human effective
	// output carries no metadata_branch row at all (change 0363).
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "metadata_branch = ") {
			t.Errorf("effective human output still carries a metadata_branch row: %q", line)
		}
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

// TestHumanTextBoardPresentation pins the inspection surface for change 0367's
// board block: the effective output carries one line for board.section_order
// and two per section (by/direction), in canonical BoardSectionTokens order,
// positioned right after board_surfaces. The Sorting map is never ranged
// directly (map order is random) — the lines follow config.BoardSectionTokens.
func TestHumanTextBoardPresentation(t *testing.T) {
	text := DiagnosticConfig(sparseSources(), mainCtx(), false).HumanText()

	lines := strings.Split(text, "\n")
	indexOf := func(substr string) int {
		for i, line := range lines {
			if strings.Contains(line, substr) {
				return i
			}
		}
		return -1
	}

	// board.section_order renders the full built-in permutation as a list, and
	// sits after board_surfaces.
	surfaces := indexOf("board_surfaces = ")
	if surfaces == -1 {
		t.Fatalf("board_surfaces line missing:\n%s", text)
	}
	order := indexOf("board.section_order = [in-progress, built, blocked, groomed, proposed, deferred]  [built-in]")
	if order == -1 {
		t.Fatalf("board.section_order line missing or malformed:\n%s", text)
	}
	if order <= surfaces {
		t.Errorf("board.section_order (%d) must follow board_surfaces (%d)", order, surfaces)
	}

	// One by/direction pair per section, in canonical order, each defaulting to
	// updated desc from the built-in layer. Assert both presence and that the
	// sections appear in config.BoardSectionTokens order.
	prev := order
	for _, s := range config.BoardSectionTokens {
		byLine := indexOf("board.sorting." + s + ".by = updated  [built-in]")
		dirLine := indexOf("board.sorting." + s + ".direction = desc  [built-in]")
		if byLine == -1 {
			t.Errorf("missing board.sorting.%s.by line:\n%s", s, text)
		}
		if dirLine == -1 {
			t.Errorf("missing board.sorting.%s.direction line:\n%s", s, text)
		}
		if byLine != -1 && byLine <= prev {
			t.Errorf("board.sorting.%s.by (%d) out of canonical order (prev %d)", s, byLine, prev)
		}
		if dirLine != -1 && dirLine <= byLine {
			t.Errorf("board.sorting.%s.direction (%d) must follow its .by (%d)", s, dirLine, byLine)
		}
		if dirLine != -1 {
			prev = dirLine
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
// TestMigrationHostContraction reproduces a representative four-layer
// configuration state — not the migration host's byte-for-byte layers — and
// pins the Go v1 capability fence's verdict on it, so the config contraction
// (change 0326) is proven against the classifier rather than assumed. The
// synthetic global layer here is a supported agent pin, NOT the real host's
// actual global auto_capture.enabled request (that request is why
// internal/config's docket-self fixture still resolves MutationAllowed==false);
// the real-host mutation-allowed proof lives in change 0326's results — the
// operator's .docket.local.yml edit plus `diagnostic config --for-mutation`.
// Three sub-cases:
//
//   - pre-change: the committed switches on, plus a repository-LOCAL auto_capture
//     request, plus a supported GLOBAL agent pin → mutation blocked, and the
//     block names every repository-layer request while EXCLUDING the global pin.
//   - post-change: the switches off and the repository-local layer dropped, the
//     global pin retained → mutation allowed, zero deferred blockers.
//   - negatives: from the post-change state, re-activate exactly one blocker →
//     each fails closed.
//
// The load-bearing premise is the classifier's layer-awareness
// (internal/config/capability.go dispAgentsLeaf gated by isRepositoryLayer): a
// global agent pin is supported, a repository/repository-local one is deferred.
// The global pin and the repository-local agent-pin negative name the SAME agent
// leaf (agents.claude.implement-next), so the LAYER is the only variable that
// flips the block/no-block outcome: a regression keyed on agent name rather than
// layer, or one that started flagging global pins, would redden here.
func TestMigrationHostContraction(t *testing.T) {
	// The supported machine-global agent pin. It must never appear as a blocker.
	globalAgentPin := config.Source{
		Layer: config.LayerGlobal,
		Name:  "/xdg/docket/config.yml",
		Data:  []byte("agents:\n  claude:\n    implement-next:\n      model: m\n      effort: low\n"),
	}
	// The committed repository file with the three owned switches at chosen states.
	repoSwitches := func(terminalPublish, skipResultsOnly, checkpoint bool) config.Source {
		return config.Source{
			Layer: config.LayerRepository,
			Name:  ".docket.yml",
			Data: []byte(fmt.Sprintf(
				"metadata_branch: docket\n"+
					"integration_branch: main\n"+
					"terminal_publish: %v\n"+
					"finalize:\n  skip_results_only_delta: %v\n"+
					"build:\n  checkpoint: %v\n",
				terminalPublish, skipResultsOnly, checkpoint)),
		}
	}
	// The repository-local layer the migration drops: an auto_capture request and
	// a repo-local agent pin, either or both selectable. The pin names the SAME
	// agent leaf as globalAgentPin (agents.claude.implement-next), so a resolution
	// carrying only this repo-local pin differs from one carrying only the global
	// pin by LAYER alone — the classifier's sole discriminator.
	repoLocal := func(autoCapture, agentPin bool) config.Source {
		var b strings.Builder
		if autoCapture {
			b.WriteString("auto_capture:\n  enabled: true\n")
		}
		if agentPin {
			b.WriteString("agents:\n  claude:\n    implement-next:\n      model: m\n      effort: medium\n")
		}
		return config.Source{Layer: config.LayerRepositoryLocal, Name: ".docket.local.yml", Data: []byte(b.String())}
	}

	blockerSet := func(r ConfigInspectionResult) map[string]bool {
		out := make(map[string]bool)
		for _, d := range r.Diagnostics {
			if d.Code == config.CodeDeferredCapRequested {
				out[d.Path] = true
			}
		}
		return out
	}

	// --- Sub-case 1: pre-change — mutation blocked, layer-aware blocker set. ---
	// The repository-local layer carries auto_capture only, so the global
	// implement-next pin resolves unshadowed and the layer-awareness claim below
	// is observable in this combined state (the repo-local pin naming the same
	// leaf is exercised on its own in the sub-case 3 negative).
	pre := DiagnosticConfig(
		[]config.Source{globalAgentPin, repoSwitches(true, true, true), repoLocal(true, false)},
		mainCtx(), true)
	if pre.MutationAllowed {
		t.Errorf("pre-change: mutation_allowed = true, want false (switches + repo-local requests are active)")
	}
	preBlockers := blockerSet(pre)
	for _, want := range []string{
		"terminal_publish",
		"finalize.skip_results_only_delta",
		"build.checkpoint",
		"auto_capture.enabled",
	} {
		if !preBlockers[want] {
			t.Errorf("pre-change blockers missing %q; got %v", want, sortedKeys(preBlockers))
		}
	}
	// The global agent pin is supported and must NOT block — this is the
	// layer-awareness claim the whole change rests on. It names the same leaf a
	// repository layer would (see the sub-case 3 negative), so only the layer
	// separates this supported case from that blocked one.
	for _, global := range []string{"agents.claude.implement-next.model", "agents.claude.implement-next.effort"} {
		if preBlockers[global] {
			t.Errorf("pre-change: global agent pin %q was reported as a blocker, but a global pin is supported", global)
		}
	}

	// --- Sub-case 2: post-change — mutation allowed, zero deferred blockers. ---
	post := DiagnosticConfig(
		[]config.Source{globalAgentPin, repoSwitches(false, false, false)},
		mainCtx(), true)
	if !post.MutationAllowed {
		t.Errorf("post-change: mutation_allowed = false, want true; blockers: %v", sortedKeys(blockerSet(post)))
	}
	if got := blockerSet(post); len(got) != 0 {
		t.Errorf("post-change: %d deferred blockers, want 0: %v", len(got), sortedKeys(got))
	}

	// --- Sub-case 3: one-at-a-time negatives — each re-activated blocker fails closed. ---
	for _, tc := range []struct {
		name    string
		sources []config.Source
	}{
		{"build.checkpoint", []config.Source{globalAgentPin, repoSwitches(false, false, true)}},
		{"finalize.skip_results_only_delta", []config.Source{globalAgentPin, repoSwitches(false, true, false)}},
		{"terminal_publish", []config.Source{globalAgentPin, repoSwitches(true, false, false)}},
		{"auto_capture.enabled", []config.Source{globalAgentPin, repoSwitches(false, false, false), repoLocal(true, false)}},
		{"repo-local agents pin", []config.Source{globalAgentPin, repoSwitches(false, false, false), repoLocal(false, true)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := DiagnosticConfig(tc.sources, mainCtx(), true)
			if got.MutationAllowed {
				t.Errorf("re-activating %q: mutation_allowed = true, want false", tc.name)
			}
			if n := len(blockerSet(got)); n == 0 {
				t.Errorf("re-activating %q: no deferred blocker reported", tc.name)
			}
		})
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

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

// TestDiagnosticConfigStaysStrictOnUnknownKeys (change 0392): the install
// path's tolerance must not leak into the operating commands' strict verdict —
// diagnostic config over a newer-schema layer still reports invalid
// configuration, so a human can always see the strict reading on demand.
func TestDiagnosticConfigStaysStrictOnUnknownKeys(t *testing.T) {
	sources := []config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: []byte("some_future_block: true\n")}}
	got := DiagnosticConfig(sources, mainCtx(), false)

	if got.Result != ResultInvalidInput {
		t.Fatalf("result = %q, want %q — diagnostic config tolerated an unknown key: %+v", got.Result, ResultInvalidInput, got)
	}
	if got.Reason != ReasonInvalidConfig {
		t.Errorf("reason = %q, want %q", got.Reason, ReasonInvalidConfig)
	}
	found := false
	for _, d := range got.Diagnostics {
		if d.Code == config.CodeUnknownKey && d.Severity == config.SeverityError {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %v, want an unknown-key ERROR", got.Diagnostics)
	}
}
