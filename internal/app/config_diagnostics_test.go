package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposetup"
)

func sampleDiags() []config.Diagnostic {
	return []config.Diagnostic{
		{Code: "unknown-key", Severity: config.SeverityError, Path: "bogus_key",
			Message:    "is not a docket configuration setting",
			Provenance: &config.Provenance{Layer: config.LayerRepository, Source: ".docket.yml", Line: 2, Column: 1}},
		{Code: "obsolete-setting", Severity: config.SeverityWarning, Path: "runtime.bash",
			Message: "ignored", Remedy: "remove runtime.bash from this file",
			Provenance: &config.Provenance{Layer: config.LayerGlobal, Source: "/home/x/.config/docket/config.yml", Line: 3}},
	}
}

// ConfigDiagnostics unwraps both refusal shapes and returns nil for anything else.
func TestConfigDiagnosticsUnwrapsBothErrorTypes(t *testing.T) {
	diags := sampleDiags()

	rre := &RepoResolutionError{Reason: ReasonInvalidConfig, Err: config.ErrInvalidConfig, Diagnostics: diags}
	if got := ConfigDiagnostics(fmt.Errorf("wrapped: %w", rre)); len(got) != 2 {
		t.Fatalf("ConfigDiagnostics(RepoResolutionError) = %d diags, want 2", len(got))
	}

	eic := &errInvalidConfiguration{diagnostics: diags, err: config.ErrInvalidConfig}
	if got := ConfigDiagnostics(fmt.Errorf("wrapped: %w", eic)); len(got) != 2 {
		t.Fatalf("ConfigDiagnostics(errInvalidConfiguration) = %d diags, want 2", len(got))
	}

	if got := ConfigDiagnostics(errors.New("unrelated")); got != nil {
		t.Fatalf("ConfigDiagnostics(unrelated) = %v, want nil", got)
	}
	// A RepoResolutionError with no diagnostics (e.g. LoadFilesystemSources failed
	// before resolution) yields nil, not an empty non-nil slice.
	bare := &RepoResolutionError{Reason: ReasonInvalidConfig, Err: config.ErrInvalidConfig}
	if got := ConfigDiagnostics(bare); got != nil {
		t.Fatalf("ConfigDiagnostics(bare RepoResolutionError) = %v, want nil", got)
	}
}

// The typed operational error classifies as ErrStatusInvalidInput and keeps
// today's message text byte-for-byte, so every existing caller is unchanged.
func TestErrInvalidConfigurationCompatibility(t *testing.T) {
	eic := &errInvalidConfiguration{diagnostics: sampleDiags(), err: config.ErrInvalidConfig}
	if !errors.Is(eic, ErrStatusInvalidInput) {
		t.Fatal("errors.Is(errInvalidConfiguration, ErrStatusInvalidInput) = false, want true")
	}
	if !errors.Is(eic, config.ErrInvalidConfig) {
		t.Fatal("errors.Is(errInvalidConfiguration, config.ErrInvalidConfig) = false, want true")
	}
	legacy := fmt.Errorf("%w: %v", ErrStatusInvalidInput, config.ErrInvalidConfig)
	if eic.Error() != legacy.Error() {
		t.Fatalf("Error() = %q, want the legacy wrap text %q", eic.Error(), legacy.Error())
	}
}

// The field mapping (spec §2): code and severity verbatim; ref is
// source:line, source alone at line 0, empty with no provenance; message is
// "path: message" when a key path exists; remedy verbatim; warnings carried.
func TestConfigDiagnosticFindingsMapping(t *testing.T) {
	diags := []config.Diagnostic{
		{Code: "invalid-type", Severity: config.SeverityError, Path: "agents.claude.adr.model",
			Message:    `expects a string, got int "42"`,
			Provenance: &config.Provenance{Source: ".docket.yml", Line: 6}},
		{Code: "invalid-yaml", Severity: config.SeverityError,
			Message:    "broken document",
			Provenance: &config.Provenance{Source: ".docket.yml"}}, // line 0 → source alone
		{Code: "obsolete-setting", Severity: config.SeverityWarning, Path: "runtime.bash",
			Message: "ignored", Remedy: "remove runtime.bash"}, // no provenance → empty ref
	}
	got := configDiagnosticFindings(diags)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (warnings are carried)", len(got))
	}
	want := []reposetup.Finding{
		{Code: "invalid-type", Severity: reposetup.Severity("error"), Ref: ".docket.yml:6",
			Message: `agents.claude.adr.model: expects a string, got int "42"`},
		{Code: "invalid-yaml", Severity: reposetup.Severity("error"), Ref: ".docket.yml",
			Message: "broken document"},
		{Code: "obsolete-setting", Severity: reposetup.Severity("warning"), Ref: "",
			Message: "runtime.bash: ignored", Remedy: "remove runtime.bash"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if configDiagnosticFindings(nil) != nil {
		t.Error("configDiagnosticFindings(nil) must be nil")
	}
}

// The status projection is the same parts mapping in the StatusFinding shape
// (Path carries the ref; severity goes through normalizeSeverity).
func TestConfigDiagnosticStatusFindingsMapping(t *testing.T) {
	diags := []config.Diagnostic{
		{Code: "invalid-type", Severity: config.SeverityError, Path: "agents.claude.adr.model",
			Message:    "expects a string",
			Provenance: &config.Provenance{Source: ".docket.yml", Line: 6}},
		{Code: "some-note", Severity: config.SeverityInfo, Message: "fyi"},
	}
	got := configDiagnosticStatusFindings(diags)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Code != "invalid-type" || got[0].Severity != "error" ||
		got[0].Path != ".docket.yml:6" || got[0].Message != "agents.claude.adr.model: expects a string" {
		t.Errorf("status finding[0] = %+v", got[0])
	}
	if got[1].Severity != "notice" { // normalizeSeverity: info → notice, per the DTO contract
		t.Errorf("info severity projected as %q, want notice", got[1].Severity)
	}
}

// The status projection is a status document, so it must never leak a
// host-absolute global-config path: for a global-layer diagnostic the ref is
// suppressed (mirroring status.go's healthy-path configFinding, which
// deliberately drops the provenance source). The repository family carries no
// such prohibition, so a repository-layer diagnostic keeps its .docket.yml:<line>
// ref in both projections.
func TestConfigDiagnosticStatusFindingsSuppressesGlobalRef(t *testing.T) {
	globalDiag := config.Diagnostic{
		Code: "obsolete-setting", Severity: config.SeverityError, Path: "runtime.bash",
		Message:    "ignored",
		Provenance: &config.Provenance{Layer: config.LayerGlobal, Source: "/Users/x/.config/docket/config.yml", Line: 3},
	}
	repoDiag := config.Diagnostic{
		Code: "unknown-key", Severity: config.SeverityError, Path: "bogus_key",
		Message:    "is not a docket configuration setting",
		Provenance: &config.Provenance{Layer: config.LayerRepository, Source: ".docket.yml", Line: 2},
	}

	got := configDiagnosticStatusFindings([]config.Diagnostic{globalDiag, repoDiag})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	// Global-layer finding: no field carries the host-absolute source string.
	g := got[0]
	if g.Path != "" {
		t.Errorf("global-layer status finding Path = %q, want empty (ref suppressed)", g.Path)
	}
	for name, field := range map[string]string{"Path": g.Path, "Message": g.Message, "Remedy": g.Remedy, "Field": g.Field} {
		if strings.Contains(field, "/Users/x/.config/docket/config.yml") {
			t.Errorf("global-layer status finding %s = %q leaks the host-absolute source", name, field)
		}
	}
	// The setting key path still rides in the message (healthy-path treatment).
	if g.Message != "runtime.bash: ignored" {
		t.Errorf("global-layer message = %q, want %q", g.Message, "runtime.bash: ignored")
	}

	// Repository-layer finding keeps its ref.
	if got[1].Path != ".docket.yml:2" {
		t.Errorf("repository-layer status finding Path = %q, want %q", got[1].Path, ".docket.yml:2")
	}

	// The repository-family projection is unchanged: the global-layer diagnostic
	// still carries its ref there (the repository path prohibition does not apply).
	repoFam := configDiagnosticFindings([]config.Diagnostic{globalDiag})
	if len(repoFam) != 1 || repoFam[0].Ref != "/Users/x/.config/docket/config.yml:3" {
		t.Errorf("repository-family projection Ref = %+v, want the source:line ref (unchanged)", repoFam)
	}
}

// appendConfigFindingBlock renders exactly the block shape
// RepositoryCheckResult.HumanText already emits per finding.
func TestAppendConfigFindingBlock(t *testing.T) {
	findings := []reposetup.Finding{
		{Code: "unknown-key", Severity: reposetup.Severity("error"), Ref: ".docket.yml:2",
			Message: "bogus_key: is not a docket configuration setting"},
		{Code: "obsolete-setting", Severity: reposetup.Severity("warning"),
			Message: "runtime.bash: ignored", Remedy: "remove it"},
	}
	got := appendConfigFindingBlock("header line", findings)
	want := "header line" +
		"\n- [error] unknown-key (.docket.yml:2)\n  bogus_key: is not a docket configuration setting" +
		"\n- [warning] obsolete-setting\n  runtime.bash: ignored\n  remedy: remove it"
	if got != want {
		t.Errorf("block:\n%q\nwant:\n%q", got, want)
	}
	if got := appendConfigFindingBlock("h", nil); got != "h" {
		t.Errorf("empty findings must return the bare header, got %q", got)
	}
}

// invalidConfigYML is the shared invalid fixture (spec §Testing): three
// defects at known lines — unknown-key at 2, invalid-type at 6,
// invalid-value at 7.
const invalidConfigYML = `changes_dir: docs/changes
bogus_key: 1
agents:
  claude:
    adr:
      model: 42
auto_capture: true
`

// TestInvalidConfigFixtureSemantics pins what the resolver actually says
// about the shared fixture: the three error diagnostics, their codes, their
// .docket.yml:<line> provenance, and their order. The integration tests
// assert the lift of exactly this verdict.
func TestInvalidConfigFixtureSemantics(t *testing.T) {
	sources := []config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: []byte(invalidConfigYML)}}
	_, diags, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: "main"})
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("Resolve err = %v, want ErrInvalidConfig", err)
	}
	var errs []config.Diagnostic
	for _, d := range diags {
		if d.Severity == config.SeverityError {
			errs = append(errs, d)
		}
	}
	if len(errs) != 3 {
		t.Fatalf("error diagnostics = %d (%+v), want 3", len(errs), diags)
	}
	wantRefs := map[string]string{
		"unknown-key":   ".docket.yml:2",
		"invalid-type":  ".docket.yml:6",
		"invalid-value": ".docket.yml:7",
	}
	for _, d := range errs {
		want, ok := wantRefs[d.Code]
		if !ok {
			t.Errorf("unexpected error code %q (%+v)", d.Code, d)
			continue
		}
		if got := configDiagnosticParts(d).ref; got != want {
			t.Errorf("code %q ref = %q, want %q", d.Code, got, want)
		}
		delete(wantRefs, d.Code)
	}
	for code := range wantRefs {
		t.Errorf("expected error code %q missing", code)
	}
}

// ConfigDiagnosticLine (spec §3): severity padded to width of "warning",
// then code, key path, <file>:<line>, message, each omitted when empty and
// separated by two spaces; remedy appended as " | remedy: ...".
func TestConfigDiagnosticLine(t *testing.T) {
	cases := []struct {
		name string
		d    config.Diagnostic
		want string
	}{
		{"full with line", config.Diagnostic{
			Code: "invalid-type", Severity: config.SeverityError, Path: "agents.claude.adr.model",
			Message:    `expects a string, got int "42"`,
			Provenance: &config.Provenance{Source: ".docket.yml", Line: 6},
		}, `error    invalid-type  agents.claude.adr.model  .docket.yml:6  expects a string, got int "42"`},
		{"warning with remedy", config.Diagnostic{
			Code: "obsolete-setting", Severity: config.SeverityWarning, Path: "runtime.bash",
			Message: "it is ignored", Remedy: "remove runtime.bash from this file",
			Provenance: &config.Provenance{Source: "/Users/x/.config/docket/config.yml", Line: 3},
		}, "warning  obsolete-setting  runtime.bash  /Users/x/.config/docket/config.yml:3  it is ignored | remedy: remove runtime.bash from this file"},
		{"line zero keeps file alone", config.Diagnostic{
			Code: "invalid-yaml", Severity: config.SeverityError, Message: "broken",
			Provenance: &config.Provenance{Source: ".docket.yml"},
		}, "error    invalid-yaml  .docket.yml  broken"},
		{"no provenance", config.Diagnostic{
			Code: "unknown-key", Severity: config.SeverityError, Path: "bogus_key", Message: "not a setting",
		}, "error    unknown-key  bogus_key  not a setting"},
	}
	for _, c := range cases {
		if got := ConfigDiagnosticLine(c.d); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}
