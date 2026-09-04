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
// refusal carries, or nil when err is not such a refusal (or the refusal
// predates resolution and carries none). config.Resolve emits diagnostics in its
// one presentation order — severity, then path, then code — on BOTH the success
// and the refusal path (its done() sorts every exit), so these arrive already
// sorted to match every healthy status/inspection surface; the lift below does
// not reorder them.
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
// family's finding shape, one finding per diagnostic, preserving the sorted
// order config.Resolve already imposes (severity, path, code) so the refusal
// findings match the healthy path — see ConfigDiagnostics for the ordering
// contract, pinned by TestRefusalFindingsSortedLikeHealthyPath.
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
//
// A global-layer diagnostic's ref is suppressed here, mirroring status.go's
// healthy-path configFinding, whose standing rationale is that "a global config
// path is host-absolute and the protocol context forbids one." The setting key
// path (d.Path) is not host-absolute and stays in the message, exactly as the
// healthy path keeps it in Field. This suppression is the status projection's
// alone: the repository family (configDiagnosticFindings) carries no such
// prohibition and keeps its ref.
func configDiagnosticStatusFindings(diags []config.Diagnostic) []StatusFinding {
	if len(diags) == 0 {
		return nil
	}
	out := make([]StatusFinding, 0, len(diags))
	for _, d := range diags {
		p := configDiagnosticParts(d)
		ref := p.ref
		if d.Provenance != nil && d.Provenance.Layer == config.LayerGlobal {
			ref = ""
		}
		out = append(out, StatusFinding{
			Code:     p.code,
			Severity: normalizeSeverity(p.severity),
			Path:     ref,
			Message:  p.message,
			Remedy:   p.remedy,
		})
	}
	return out
}

// ConfigDiagnosticLine renders one config diagnostic the way every human
// surface prints it: severity (padded to the width of "warning"), code, key
// path, <file>:<line>, message — fields two-space separated, omitted when
// empty — with the remedy appended as " | remedy: ..." (spec: one shared
// human rendering, change 0403). The <file>:<line> follows the same ref rule
// as configDiagnosticParts: file alone at line 0, nothing without provenance.
func ConfigDiagnosticLine(d config.Diagnostic) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-7s  %s", string(d.Severity), d.Code)
	if d.Path != "" {
		fmt.Fprintf(&b, "  %s", d.Path)
	}
	if ref := configDiagnosticParts(d).ref; ref != "" {
		fmt.Fprintf(&b, "  %s", ref)
	}
	if d.Message != "" {
		fmt.Fprintf(&b, "  %s", d.Message)
	}
	if d.Remedy != "" {
		fmt.Fprintf(&b, " | remedy: %s", d.Remedy)
	}
	return b.String()
}

// appendFindingBlock writes one finding's human block to b, led by a newline:
// "- [severity] code", an optional " (ref)", then the message and the remedy
// each on their own indented continuation line. It is the SINGLE per-finding
// renderer that RepositoryCheckResult.HumanText's healthy loop and
// appendConfigFindingBlock's refusal loop both call, so the healthy and refusal
// blocks cannot drift by hand-copy (change 0403; finding: the two loops were
// duplicated). RepositoryPrepareResult.HumanText deliberately renders without the
// "(ref)" clause and is not a caller — its refusal path still routes config
// findings through appendConfigFindingBlock (with the ref), so this shared
// renderer governs every surface that prints the ref.
func appendFindingBlock(b *strings.Builder, f reposetup.Finding) {
	fmt.Fprintf(b, "\n- [%s] %s", f.Severity, f.Code)
	if f.Ref != "" {
		fmt.Fprintf(b, " (%s)", f.Ref)
	}
	if f.Message != "" {
		fmt.Fprintf(b, "\n  %s", f.Message)
	}
	if f.Remedy != "" {
		fmt.Fprintf(b, "\n  remedy: %s", f.Remedy)
	}
}

// appendConfigFindingBlock appends the shared per-finding human block to a
// refusal's existing header line — the same "- [severity] code (ref)" block
// RepositoryCheckResult.HumanText emits on the healthy classification path,
// rendered through the shared appendFindingBlock so the two cannot drift.
func appendConfigFindingBlock(header string, findings []reposetup.Finding) string {
	var b strings.Builder
	b.WriteString(header)
	for _, f := range findings {
		appendFindingBlock(&b, f)
	}
	return b.String()
}
