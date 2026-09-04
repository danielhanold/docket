package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

func sampleDiags() []config.Diagnostic {
	return []config.Diagnostic{
		{Code: "unknown-key", Severity: config.SeverityError, Path: "bogus_key",
			Message: "is not a docket configuration setting",
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
