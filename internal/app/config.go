package app

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/config"
)

// Stable machine reasons for the configuration operations, alongside the CLI
// reasons in clierror.go. Message is explanatory prose and must not be parsed.
const (
	ReasonInvalidConfig            = "invalid-config"
	ReasonMissingResolutionContext = "missing-resolution-context"
	ReasonDeferredCapRequested     = "deferred-capability-requested"
	ReasonInternalError            = "internal-error"
)

// sourceModeFilesystem is the only source mode this generation ships. It is a
// field rather than an assumption so a later in-memory or remote source mode
// is an added spelling, not a protocol break.
const sourceModeFilesystem = "filesystem"

// ConfigInspectionResult is the diagnostic.config / config.preflight document.
//
// Effective and Capabilities are omitted on failure results — there is no
// snapshot to report — while Diagnostics is always present, because whatever
// parsing DID produce is the only thing that explains the failure.
type ConfigInspectionResult struct {
	Envelope
	SourceMode      string              `json:"source_mode"`
	MutationAllowed bool                `json:"mutation_allowed"`
	Effective       *config.Effective   `json:"effective,omitempty"`
	Capabilities    []config.Capability `json:"capabilities,omitempty"`
	Diagnostics     []config.Diagnostic `json:"diagnostics"`
	Reason          string              `json:"reason,omitempty"`
	Message         string              `json:"message,omitempty"`
}

// DiagnosticConfig resolves the supplied layers and computes the whole
// outcome. forMutation selects the config.preflight operation and, with it,
// the only mode in which a blocked configuration is a failure: inspection
// reports the block as data under an applied result, because reading a
// configuration you may not mutate is exactly what inspection is for.
func DiagnosticConfig(sources []config.Source, rctx config.ResolveContext, forMutation bool) ConfigInspectionResult {
	operation := "diagnostic.config"
	if forMutation {
		operation = "config.preflight"
	}

	snap, diags, err := config.Resolve(sources, rctx)
	if err != nil {
		result, reason := failureOutcome(err)
		return ConfigInspectionResult{
			Envelope:    NewEnvelope(operation, result),
			SourceMode:  sourceModeFilesystem,
			Diagnostics: nonNilDiagnostics(diags),
			Reason:      reason,
			Message:     err.Error(),
		}
	}

	decision := config.PreflightMutation(snap)
	out := ConfigInspectionResult{
		Envelope:        NewEnvelope(operation, ResultApplied),
		SourceMode:      sourceModeFilesystem,
		MutationAllowed: decision.Allowed,
		Effective:       &snap.Effective,
		Capabilities:    nonNilCapabilities(snap.Capabilities),
		Diagnostics:     nonNilDiagnostics(snap.Diagnostics),
	}
	if forMutation && !decision.Allowed {
		out.Result = ResultUnsupportedConfig
		out.Reason = ReasonDeferredCapRequested
		out.Message = fmt.Sprintf("%d %s deferred or dropped behavior docket does not ship yet; a mutation cannot proceed until %s withdrawn (%s)",
			len(decision.Blockers), pluralize(len(decision.Blockers), "setting requests", "settings request"),
			pluralize(len(decision.Blockers), "it is", "they are"), strings.Join(blockerPaths(decision.Blockers), ", "))
	}
	return out
}

// failureOutcome maps a resolution error to the result and stable machine
// reason it is reported under. Only the two sentinel errors describe something
// the USER can act on — a bad configuration, or a missing resolution context.
// Everything else out of config.Resolve is a caller-contract violation
// (misordered or repeated source layers, a registry/Effective mismatch), which
// is docket's bug, not the user's: reporting it as invalid input would send
// the user off to edit a perfectly valid .docket.yml with no diagnostics to
// go on, so it surfaces as an internal error carrying the error's own text.
func failureOutcome(err error) (Result, string) {
	switch {
	case errors.Is(err, config.ErrMissingResolutionContext):
		return ResultInvalidInput, ReasonMissingResolutionContext
	case errors.Is(err, config.ErrInvalidConfig):
		return ResultInvalidInput, ReasonInvalidConfig
	default:
		return ResultInternalError, ReasonInternalError
	}
}

func blockerPaths(blockers []config.Diagnostic) []string {
	out := make([]string, 0, len(blockers))
	for _, b := range blockers {
		out = append(out, b.Path)
	}
	return out
}

// nonNilDiagnostics keeps the always-present diagnostics array from marshalling
// as JSON null.
func nonNilDiagnostics(diags []config.Diagnostic) []config.Diagnostic {
	if diags == nil {
		return []config.Diagnostic{}
	}
	return diags
}

// nonNilCapabilities is nil hygiene for Go callers reading the struct: an
// empty list ranges the same as a nil one, but a caller that appends or
// indexes gets a real slice either way. It does NOT shape the JSON — the
// field's `omitempty` tag omits an empty capability list entirely, so the key
// is simply absent (never null) whenever there is nothing to report,
// failure result or not. Diagnostics is the only unconditional array.
func nonNilCapabilities(caps []config.Capability) []config.Capability {
	if caps == nil {
		return []config.Capability{}
	}
	return caps
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// HumanText renders the deterministic grouped text form: the two-line verdict,
// then the winning value of every effective leaf with the layer it won from,
// then the capabilities and diagnostics that need explaining. It reports
// setting paths, values, and source NAMES — never bytes read from a file or
// values read from the environment.
func (r ConfigInspectionResult) HumanText() string {
	var b strings.Builder

	valid := r.Effective != nil
	if valid {
		b.WriteString("configuration: valid\n")
	} else {
		b.WriteString("configuration: invalid\n")
	}

	switch {
	case !valid:
		b.WriteString("mutation: n/a (configuration invalid)\n")
	case r.MutationAllowed:
		b.WriteString("mutation: allowed\n")
	default:
		n := r.blockerCount()
		fmt.Fprintf(&b, "mutation: blocked (%d %s)\n", n, pluralize(n, "blocker", "blockers"))
	}

	if r.Reason != "" {
		fmt.Fprintf(&b, "reason: %s\n", r.Reason)
		if r.Message != "" {
			fmt.Fprintf(&b, "message: %s\n", r.Message)
		}
	}

	b.WriteString("\n")
	if !valid {
		b.WriteString("effective: (unavailable — configuration invalid)\n")
	} else {
		b.WriteString("effective (winning layer):\n")
		for _, line := range effectiveLines(r.Effective) {
			fmt.Fprintf(&b, "  %s = %s  [%s]\n", line.path, line.value, line.layer)
		}
	}

	if len(r.Capabilities) > 0 {
		b.WriteString("\ncapabilities:\n")
		for _, c := range r.Capabilities {
			state := "inactive"
			if c.Active {
				state = "active"
			}
			if c.MutationBlock {
				state += ", blocks mutation"
			}
			fmt.Fprintf(&b, "  %s: %s (%s) — %s", c.Path, c.Classification, state, c.Reason)
			if c.Remedy != "" {
				fmt.Fprintf(&b, " | remedy: %s", c.Remedy)
			}
			b.WriteString("\n")
		}
	}

	if len(r.Diagnostics) > 0 {
		b.WriteString("\ndiagnostics:\n")
		for _, d := range r.Diagnostics {
			fmt.Fprintf(&b, "  %-7s %s", d.Severity, d.Code)
			if d.Path != "" {
				fmt.Fprintf(&b, " %s", d.Path)
			}
			fmt.Fprintf(&b, " — %s\n", d.Message)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// blockerCount counts the mutation blockers carried by the document itself, so
// the human line agrees with the JSON array a reader can count by hand.
func (r ConfigInspectionResult) blockerCount() int {
	n := 0
	for _, d := range r.Diagnostics {
		if d.Code == config.CodeDeferredCapRequested {
			n++
		}
	}
	return n
}

// effectiveLine is one rendered leaf: its path, its formatted value, and the
// label of the layer whose declaration won it.
type effectiveLine struct {
	path  string
	value string
	layer string
}

// effectiveLines walks the Effective struct in declaration order — the same
// order the JSON document carries — with the agent table grouped last, since
// it is the only subtree whose size depends on the harnesses in play.
func effectiveLines(eff *config.Effective) []effectiveLine {
	lines := []effectiveLine{
		leafLine("metadata_branch", textValue(eff.MetadataBranch.Value), eff.MetadataBranch.Provenance),
		leafLine("integration_branch", textValue(eff.IntegrationBranch.Value), eff.IntegrationBranch.Provenance),
		leafLine("changes_dir", textValue(eff.ChangesDir.Value), eff.ChangesDir.Provenance),
		leafLine("adrs_dir", textValue(eff.ADRsDir.Value), eff.ADRsDir.Provenance),
		leafLine("results_dir", textValue(eff.ResultsDir.Value), eff.ResultsDir.Provenance),
		leafLine("finalize.gate", textValue(eff.Finalize.Gate.Value), eff.Finalize.Gate.Provenance),
		leafLine("finalize.test_command", textValue(eff.Finalize.TestCommand.Value), eff.Finalize.TestCommand.Provenance),
		leafLine("finalize.require_pr_approval", strconv.FormatBool(eff.Finalize.RequirePRApproval.Value), eff.Finalize.RequirePRApproval.Provenance),
		leafLine("learnings.enabled", strconv.FormatBool(eff.Learnings.Enabled.Value), eff.Learnings.Enabled.Provenance),
		leafLine("reclaim.lease_ttl", strconv.Itoa(eff.Reclaim.LeaseTTL.Value), eff.Reclaim.LeaseTTL.Provenance),
		leafLine("reclaim.auto", strconv.FormatBool(eff.Reclaim.Auto.Value), eff.Reclaim.Auto.Provenance),
		leafLine("review.min_fix_severity", textValue(eff.Review.MinFixSeverity.Value), eff.Review.MinFixSeverity.Provenance),
		leafLine("review.max_fix_tasks", strconv.Itoa(eff.Review.MaxFixTasks.Value), eff.Review.MaxFixTasks.Provenance),
		leafLine("gate_observation_budget", strconv.Itoa(eff.GateObservation.Value), eff.GateObservation.Provenance),
		leafLine("board_surfaces", listValue(eff.BoardSurfaces.Value), eff.BoardSurfaces.Provenance),
		leafLine("change_types", listValue(eff.ChangeTypes.Value), eff.ChangeTypes.Provenance),
	}
	return append(lines, agentLines(eff.Agents)...)
}

// agentLines renders the harness × agent table in sorted order, pairing each
// row's model with its effort pin. The bracket names the model's winning
// layer, and adds the effort's own only when the two disagree.
func agentLines(agents config.AgentsTable) []effectiveLine {
	harnesses := make([]string, 0, len(agents))
	for harness := range agents {
		harnesses = append(harnesses, harness)
	}
	sort.Strings(harnesses)

	var out []effectiveLine
	for _, harness := range harnesses {
		row := agents[harness]
		names := make([]string, 0, len(row))
		for name := range row {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			setting := row[name]
			effort := setting.Effort.Value
			if effort == "" {
				effort = "(no effort pin)"
			}
			layer := layerLabel(setting.Model.Provenance)
			if setting.Effort.Provenance != setting.Model.Provenance {
				layer += ", effort: " + layerLabel(setting.Effort.Provenance)
			}
			out = append(out, effectiveLine{
				path:  fmt.Sprintf("agents.%s.%s", harness, name),
				value: fmt.Sprintf("%s / %s", setting.Model.Value, effort),
				layer: layer,
			})
		}
	}
	return out
}

func leafLine(path, value string, prov config.Provenance) effectiveLine {
	return effectiveLine{path: path, value: value, layer: layerLabel(prov)}
}

// layerLabel names the layer, plus the file and line for anything a user can
// actually go and edit. The built-in layer has no file, so it stays bare.
func layerLabel(prov config.Provenance) string {
	if prov.Layer == config.LayerBuiltIn || prov.Source == "" {
		return string(prov.Layer)
	}
	if prov.Line > 0 {
		return fmt.Sprintf("%s %s:%d", prov.Layer, prov.Source, prov.Line)
	}
	return fmt.Sprintf("%s %s", prov.Layer, prov.Source)
}

// textValue renders a string leaf, spelling the empty string so an unset
// setting is not an invisible one.
func textValue(v string) string {
	if v == "" {
		return "(unset)"
	}
	return v
}

func listValue(v []string) string { return "[" + strings.Join(v, ", ") + "]" }
