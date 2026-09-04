// Package app: config_diagnostics carries resolver diagnostics across an
// invalid-configuration refusal and renders them for humans and findings
// arrays (change 0403). The resolver stays the single author of diagnostic
// content; this file only transports and formats it.
package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposetup"
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

// configFindingParts is the single internal projection of one resolver
// diagnostic into finding vocabulary. Both finding shapes (reposetup.Finding
// and StatusFinding) are thin views of it, so the two cannot drift (spec §2).
type configFindingParts struct {
	code, severity, ref, message, remedy string
}

func configDiagnosticParts(d config.Diagnostic) configFindingParts {
	ref := ""
	if d.Provenance != nil {
		ref = d.Provenance.Source
		if d.Provenance.Line > 0 {
			ref = fmt.Sprintf("%s:%d", d.Provenance.Source, d.Provenance.Line)
		}
	}
	message := d.Message
	if d.Path != "" {
		message = d.Path + ": " + d.Message
	}
	return configFindingParts{
		code:     d.Code,
		severity: string(d.Severity),
		ref:      ref,
		message:  message,
		remedy:   d.Remedy,
	}
}

// configDiagnosticFindings lifts resolver diagnostics into the repository
// family's finding shape, one finding per diagnostic, in resolver order.
// Warnings ride along so the reader sees the whole resolver verdict; the
// operation's result and exit are decided by the error, never by these.
// Repairable stays nil: config findings are never auto-repairable.
func configDiagnosticFindings(diags []config.Diagnostic) []reposetup.Finding {
	if len(diags) == 0 {
		return nil
	}
	out := make([]reposetup.Finding, 0, len(diags))
	for _, d := range diags {
		p := configDiagnosticParts(d)
		out = append(out, reposetup.Finding{
			Code:     p.code,
			Severity: reposetup.Severity(p.severity),
			Ref:      p.ref,
			Message:  p.message,
			Remedy:   p.remedy,
		})
	}
	return out
}

// configDiagnosticStatusFindings is the same lift in the status DTO's finding
// shape. Severity goes through normalizeSeverity (info → notice) to honor the
// StatusFinding severity vocabulary; the ref rides in Path, the locator slot
// the human renderer already prints.
func configDiagnosticStatusFindings(diags []config.Diagnostic) []StatusFinding {
	if len(diags) == 0 {
		return nil
	}
	out := make([]StatusFinding, 0, len(diags))
	for _, d := range diags {
		p := configDiagnosticParts(d)
		out = append(out, StatusFinding{
			Code:     p.code,
			Severity: normalizeSeverity(p.severity),
			Path:     p.ref,
			Message:  p.message,
			Remedy:   p.remedy,
		})
	}
	return out
}

// appendConfigFindingBlock appends the shared per-finding human block to a
// refusal's existing header line — the same "- [severity] code (ref)" block
// RepositoryCheckResult.HumanText emits on the healthy classification path.
func appendConfigFindingBlock(header string, findings []reposetup.Finding) string {
	var b strings.Builder
	b.WriteString(header)
	for _, f := range findings {
		fmt.Fprintf(&b, "\n- [%s] %s", f.Severity, f.Code)
		if f.Ref != "" {
			fmt.Fprintf(&b, " (%s)", f.Ref)
		}
		if f.Message != "" {
			fmt.Fprintf(&b, "\n  %s", f.Message)
		}
		if f.Remedy != "" {
			fmt.Fprintf(&b, "\n  remedy: %s", f.Remedy)
		}
	}
	return b.String()
}
