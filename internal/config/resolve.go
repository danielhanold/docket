package config

import (
	"fmt"
	"sort"
	"strings"
)

// This file is resolution: it stacks the layers a caller supplied on top of
// the built-in defaults and answers, for every leaf, what the value is and
// which layer supplied it. It is the only stage that knows how the layers
// relate, so it owns the coordination fences, per-leaf precedence, the two
// `auto` sentinels, and the one cross-leaf rule that cannot run before
// precedence. It never classifies: what a resolved declaration MEANS for Go
// v1 is the classifier's question, and the `resolution` struct is what it
// reads.

// callerLayers is the low-to-high order a caller must supply. The built-in
// layer is synthesized here and must never appear in the input.
var callerLayers = []LayerKind{LayerGlobal, LayerRepository, LayerRepositoryLocal}

// agentOverrideLayers are the layers whose agent pins land in
// Effective.Agents. Global is the sole override layer: a repository-layer pin
// is a deferred-capability request, so it stays in `declared` for the
// classifier and never becomes effective policy.
var agentOverrideLayers = []LayerKind{LayerGlobal}

// invalidClass is the code set that makes a snapshot invalid. Validity keys on
// the code — never on severity alone, because deferred-capability-requested is
// an error that leaves the snapshot valid. The severity test below is the
// mirror of that: `unknown-key` is in this class, but the two deliberate
// v0.9.2 warn-and-ignore surfaces (an unknown skills role, an unknown board
// token) report it as a WARNING precisely because they must not invalidate a
// configuration that worked before.
var invalidClass = []string{
	CodeInvalidYAML, CodeDuplicateKey, CodeUnknownKey, CodeInvalidType, CodeInvalidValue,
}

// resolution is the resolved configuration in the form the classifier needs:
// the effective policy, the honored declarations behind it, and every
// declaration including the ones a fence excluded.
type resolution struct {
	effective Effective

	// declared maps a concrete path to the HIGHEST honored layer's declaration
	// of it, after fence exclusion. The classifier keys activation on these.
	declared map[string]leafDecl

	// allDecls is every declaration INCLUDING fenced-away ones, in low-to-high
	// layer order and document order within a layer, with the values as the
	// declaring layer wrote them.
	allDecls []leafDecl

	diags []Diagnostic
}

// Resolve parses and resolves sources supplied in low-to-high precedence order
// (global, repository, repository-local; the built-in layer is synthesized
// internally and MUST NOT appear in sources).
//
// It returns (snapshot, allDiagnostics, nil) on a valid snapshot;
// (nil, allDiagnostics, ErrInvalidConfig) when any invalid-class diagnostic
// exists; and (nil, diagnostics, ErrMissingResolutionContext) when
// integration_branch resolves to `auto` with no ctx.DefaultBranch.
func Resolve(sources []Source, rctx ResolveContext) (*Snapshot, []Diagnostic, error) {
	res, err := resolve(sources, rctx)
	if res == nil {
		return nil, nil, err
	}
	if err != nil {
		return nil, res.diags, err
	}
	// Classification runs only on a resolved, valid snapshot: what a declaration
	// MEANS is a question about the winning value, and an invalid layer has no
	// winning value to ask about.
	caps, classDiags := classify(res)
	res.diags = append(res.diags, classDiags...)
	sortDiagnostics(res.diags)
	return &Snapshot{Effective: res.effective, Capabilities: caps, Diagnostics: res.diags}, res.diags, nil
}

func resolve(sources []Source, rctx ResolveContext) (*resolution, error) {
	if err := checkSourceOrder(sources); err != nil {
		return nil, err
	}

	res := &resolution{declared: make(map[string]leafDecl)}

	// Every layer is parsed and decoded before any is judged: one broken layer
	// must not hide what the others say.
	for _, src := range sources {
		root, diags := parseLayer(src)
		res.diags = append(res.diags, diags...)
		if root == nil {
			continue
		}
		leaves, decodeDiags := decodeLayer(root, src)
		res.diags = append(res.diags, decodeDiags...)
		res.allDecls = append(res.allDecls, leaves...)
	}
	if res.hasInvalid() {
		return res.done(), ErrInvalidConfig
	}

	// Fences first, then precedence: a fenced declaration is not a
	// lower-precedence declaration, it is no declaration at all.
	byLayer := make(map[LayerKind]map[string]leafDecl, len(sources))
	for _, decl := range res.allDecls {
		honored, ok := res.applyFence(decl)
		if !ok {
			continue
		}
		res.declared[honored.path] = honored
		if byLayer[honored.prov.Layer] == nil {
			byLayer[honored.prov.Layer] = make(map[string]leafDecl)
		}
		byLayer[honored.prov.Layer][honored.path] = honored
	}

	eff, err := res.assemble(byLayer)
	if err != nil {
		return nil, err
	}
	res.effective = eff

	// The subset check is the one rule that cannot run before precedence: it
	// compares two resolved leaves, either of which may come from any layer.
	res.checkAutoCaptureTypes()
	if res.hasInvalid() {
		return res.done(), ErrInvalidConfig
	}

	if err := res.resolveIntegrationBranch(rctx); err != nil {
		return res.done(), err
	}
	return res.done(), nil
}

// done sorts the diagnostics and hands back the resolution. Every exit runs
// through it so ordering is a property of the type, not of a caller.
func (r *resolution) done() *resolution {
	sortDiagnostics(r.diags)
	return r
}

func (r *resolution) hasInvalid() bool {
	for _, d := range r.diags {
		if d.Severity == SeverityError && inList(invalidClass, d.Code) {
			return true
		}
	}
	return false
}

// checkSourceOrder enforces the caller's half of the contract. A violation is
// a programming error in docket, not a user's invalid configuration, so it
// returns a plain error that no caller can mistake for ErrInvalidConfig.
func checkSourceOrder(sources []Source) error {
	next := 0
	for i, src := range sources {
		if src.Layer == LayerBuiltIn {
			return fmt.Errorf("config: sources[%d]: the built-in layer is synthesized and must not be supplied", i)
		}
		pos := -1
		for j, layer := range callerLayers {
			if layer == src.Layer {
				pos = j
				break
			}
		}
		if pos < 0 {
			return fmt.Errorf("config: sources[%d]: unknown layer %q", i, src.Layer)
		}
		if pos < next {
			return fmt.Errorf("config: sources[%d]: layer %q is out of order or repeated; supply at most one of %s in that order",
				i, src.Layer, layerList(callerLayers))
		}
		next = pos + 1
	}
	return nil
}

func layerList(layers []LayerKind) string {
	names := make([]string, 0, len(layers))
	for _, l := range layers {
		names = append(names, string(l))
	}
	return strings.Join(names, ", ")
}

// applyFence decides whether a declaration is honored in the layer that made
// it. The diagnostic keys on the DECLARATION's provenance alone — whether some
// other layer also declares the path is a different question, and answering
// them together would make the warning appear and disappear with an unrelated
// file's contents.
func (r *resolution) applyFence(decl leafDecl) (leafDecl, bool) {
	switch {
	case decl.spec.scope == scopeRepoFenced && isMachineLayer(decl.prov.Layer):
		r.fenced(decl, decl.path,
			"coordinates the whole repository and may only be declared in the committed configuration",
			fmt.Sprintf("move %s to the committed .docket.yml, or remove it here", decl.path))
		return decl, false
	}

	if decl.path == "board_surfaces" && isMachineLayer(decl.prov.Layer) {
		decl.value = r.keepUnfencedSurfaces(decl)
	}
	return decl, true
}

// keepUnfencedSurfaces drops the coordination-fenced `github` token from a
// machine layer's board_surfaces and returns the surviving tokens: the fence
// is on the token, so the rest of the list still competes for the leaf.
func (r *resolution) keepUnfencedSurfaces(decl leafDecl) []string {
	tokens, ok := decl.value.([]string)
	if !ok {
		return nil
	}
	kept := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token != boardSurfaceGitHub {
			kept = append(kept, token)
			continue
		}
		r.fenced(decl, decl.path,
			fmt.Sprintf("names the %q surface, which coordinates the whole repository and may only be requested by the committed configuration", boardSurfaceGitHub),
			fmt.Sprintf("declare %q in the committed .docket.yml, or drop the token here", boardSurfaceGitHub))
	}
	return kept
}

func (r *resolution) fenced(decl leafDecl, path, why, remedy string) {
	prov := decl.prov
	r.diags = append(r.diags, Diagnostic{
		Code:       CodeFencedIgnored,
		Severity:   SeverityWarning,
		Path:       path,
		Provenance: &prov,
		Message:    fmt.Sprintf("%s: %s %s; the declaration is ignored", prov.Source, path, why),
		Remedy:     remedy,
	})
}

func isMachineLayer(layer LayerKind) bool {
	return layer == LayerGlobal || layer == LayerRepositoryLocal
}

// assemble lays the honored declarations over the built-in defaults. A leaf
// that no honored layer declared keeps its built-in value and provenance, and
// stays non-explicit.
func (r *resolution) assemble(byLayer map[LayerKind]map[string]leafDecl) (Effective, error) {
	eff := builtinEffective()

	var errs []error
	set := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	set(assign(&eff.MetadataBranch, r.declared, "metadata_branch"))
	set(assign(&eff.IntegrationBranch, r.declared, "integration_branch"))
	set(assign(&eff.ChangesDir, r.declared, "changes_dir"))
	set(assign(&eff.ADRsDir, r.declared, "adrs_dir"))
	set(assign(&eff.ResultsDir, r.declared, "results_dir"))
	set(assign(&eff.Finalize.Gate, r.declared, "finalize.gate"))
	set(assign(&eff.Finalize.TestCommand, r.declared, "finalize.test_command"))
	set(assign(&eff.Finalize.RequirePRApproval, r.declared, "finalize.require_pr_approval"))
	set(assign(&eff.Learnings.Enabled, r.declared, "learnings.enabled"))
	set(assign(&eff.Reclaim.LeaseTTL, r.declared, "reclaim.lease_ttl"))
	set(assign(&eff.Reclaim.Auto, r.declared, "reclaim.auto"))
	set(assign(&eff.Review.MinFixSeverity, r.declared, "review.min_fix_severity"))
	set(assign(&eff.Review.MaxFixTasks, r.declared, "review.max_fix_tasks"))
	set(assign(&eff.GateObservation, r.declared, "gate_observation_budget"))
	set(assign(&eff.BoardSurfaces, r.declared, "board_surfaces"))
	set(assign(&eff.ChangeTypes, r.declared, "change_types"))
	if len(errs) > 0 {
		return eff, errs[0]
	}

	eff.Agents = resolveAgents(byLayer)

	// `auto` is how the built-in layer spells "no test command configured";
	// a layer that repeats it means the same thing.
	if eff.Finalize.TestCommand.Value == autoSentinel {
		eff.Finalize.TestCommand.Value = ""
	}
	return eff, nil
}

// autoSentinel is the spelling both `auto`-valued settings use for "docket
// decides": finalize.test_command resolves it to unset, integration_branch to
// the resolution context's default branch.
const autoSentinel = "auto"

// boardSurfaceGitHub is the one board token carrying a coordination fence.
const boardSurfaceGitHub = "github"

// assign lays one honored declaration over a built-in default. A declaration
// whose decoded type does not match the leaf is a mismatch between the
// registry and Effective — a docket bug, reported as a plain error rather than
// silently dropped.
func assign[T any](dst *Value[T], declared map[string]leafDecl, path string) error {
	decl, ok := declared[path]
	if !ok {
		return nil
	}
	value, ok := decl.value.(T)
	if !ok {
		return fmt.Errorf("config: internal: %s decoded to %T, which does not fit its effective leaf", path, decl.value)
	}
	*dst = Value[T]{Value: value, Provenance: decl.prov, Explicit: true}
	return nil
}

// resolveAgents lays each override layer's agent pins over the built-in 17x4
// table. Model and effort resolve independently, and within one layer a
// harness-specific pin falls back to that layer's `agents.default` before the
// next layer is consulted — so a global default pin is not silently outranked
// by a built-in harness pin.
func resolveAgents(byLayer map[LayerKind]map[string]leafDecl) AgentsTable {
	table := builtinAgents()
	for _, layer := range agentOverrideLayers {
		declared := byLayer[layer]
		if len(declared) == 0 {
			continue
		}
		for harness, row := range table {
			for name, setting := range row {
				if decl, ok := agentDecl(declared, harness, name, "model"); ok {
					if model, ok := decl.value.(string); ok {
						setting.Model = Value[string]{Value: model, Provenance: decl.prov, Explicit: true}
					}
				}
				if decl, ok := agentDecl(declared, harness, name, "effort"); ok {
					if effort, ok := decl.value.(string); ok {
						// `auto` suppresses the pin: the harness picks.
						if effort == autoSentinel {
							effort = ""
						}
						setting.Effort = Value[string]{Value: effort, Provenance: decl.prov, Explicit: true}
					}
				}
				row[name] = setting
			}
		}
	}
	return table
}

func agentDecl(declared map[string]leafDecl, harness, agent, field string) (leafDecl, bool) {
	if decl, ok := declared["agents."+harness+"."+agent+"."+field]; ok {
		return decl, true
	}
	decl, ok := declared["agents.default."+agent+"."+field]
	return decl, ok
}

// checkAutoCaptureTypes is the one cross-leaf rule: an auto-capture type list
// that names a change type the repository does not define would silently
// capture nothing.
func (r *resolution) checkAutoCaptureTypes() {
	decl, ok := r.declared["auto_capture.types"]
	if !ok {
		return
	}
	types, ok := decl.value.([]string)
	if !ok {
		return // the `all` sentinel asks no subset question
	}
	known := r.effective.ChangeTypes.Value
	for _, token := range types {
		if inList(known, token) {
			continue
		}
		prov := decl.prov
		r.diags = append(r.diags, Diagnostic{
			Code:       CodeInvalidValue,
			Severity:   SeverityError,
			Path:       decl.path,
			Provenance: &prov,
			Message: fmt.Sprintf("%s: %s names %q, which is not one of the effective change_types (%s)",
				prov.Source, decl.path, token, strings.Join(known, ", ")),
			Remedy: fmt.Sprintf("remove %q from %s, or add it to change_types", token, decl.path),
		})
	}
}

// resolveIntegrationBranch replaces the `auto` sentinel with the branch the
// caller supplied. The provenance stays with whichever layer wrote `auto`:
// that file is where a reader changes the answer.
func (r *resolution) resolveIntegrationBranch(rctx ResolveContext) error {
	if r.effective.IntegrationBranch.Value != autoSentinel {
		return nil
	}
	if rctx.DefaultBranch == "" {
		return ErrMissingResolutionContext
	}
	r.effective.IntegrationBranch.Value = rctx.DefaultBranch
	return nil
}

// severityRank orders severities most-serious first.
func severityRank(s Severity) int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	case SeverityInfo:
		return 2
	}
	return 3
}

// sortDiagnostics puts diagnostics in the package's one presentation order:
// severity, then path, then code. The sort is stable, so two diagnostics that
// tie on all three keep the order the layers produced them in.
func sortDiagnostics(diags []Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := severityRank(diags[i].Severity), severityRank(diags[j].Severity)
		if a != b {
			return a < b
		}
		if diags[i].Path != diags[j].Path {
			return diags[i].Path < diags[j].Path
		}
		return diags[i].Code < diags[j].Code
	})
}
