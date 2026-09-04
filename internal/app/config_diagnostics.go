// Package app: config_diagnostics carries resolver diagnostics across an
// invalid-configuration refusal and renders them for humans and findings
// arrays (change 0403). The resolver stays the single author of diagnostic
// content; this file only transports and formats it.
package app

import (
	"errors"

	"github.com/danielhanold/docket/internal/config"
)

// errInvalidConfiguration is the operational loader's invalid-config refusal.
// It carries the resolver's diagnostics so the refusing operation can lift
// them into its findings, and it classifies as ErrStatusInvalidInput so every
// existing errors.Is caller is unchanged. Error() preserves the legacy
// fmt.Errorf("%w: %v", ErrStatusInvalidInput, err) text byte-for-byte.
type errInvalidConfiguration struct {
	diagnostics []config.Diagnostic
	err         error
}

func (e *errInvalidConfiguration) Error() string {
	return ErrStatusInvalidInput.Error() + ": " + e.err.Error()
}
func (e *errInvalidConfiguration) Unwrap() error        { return e.err }
func (e *errInvalidConfiguration) Is(target error) bool { return target == ErrStatusInvalidInput }

// ConfigDiagnostics returns the resolver diagnostics an invalid-configuration
// refusal carries, in resolver order, or nil when err is not such a refusal
// (or the refusal predates resolution and carries none).
func ConfigDiagnostics(err error) []config.Diagnostic {
	var rre *RepoResolutionError
	if errors.As(err, &rre) && len(rre.Diagnostics) > 0 {
		return rre.Diagnostics
	}
	var eic *errInvalidConfiguration
	if errors.As(err, &eic) && len(eic.diagnostics) > 0 {
		return eic.diagnostics
	}
	return nil
}
