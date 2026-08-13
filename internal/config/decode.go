package config

import (
	"strings"

	"go.yaml.in/yaml/v3"
)

// This file is the typed decode stage: it walks one already-node-validated
// layer against the registry and turns each declared leaf into a typed value
// with provenance. It decides only what a single layer says — never what the
// layer is allowed to say. Fences, precedence, defaults, and every cross-leaf
// rule belong to resolution, which is the only stage that knows how the layers
// stack.

// leafDecl is one explicitly declared leaf in one layer: the concrete path the
// user wrote, the registry row it matched, the typed value, and where the
// declaration sits.
type leafDecl struct {
	path  string     // concrete dotted path ("agents.claude.adr.model")
	spec  *pathSpec  // matched registry row (dynamic rows keep their "*" path)
	value any        // string, bool, int, []string, or githubProject
	prov  Provenance // layer, source name, and the KEY node's position
}

// boardSurfaceTokens is the closed set of surfaces v0.9.2 recognises. It lives
// here rather than in the registry because it is not a validation rule: an
// unrecognised surface is warned about and dropped (the v0.9.2 warn-and-ignore
// contract), which the registry's leafValidator cannot express — a validator
// returns a value or diagnostics, never both. Whether `github` is HONORED is a
// separate, layer-dependent question that resolution answers.
var boardSurfaceTokens = []string{"inline", "github"}

// obsoleteAutoCaptureRemedy is the nested shape that replaced the pre-0127
// scalar `auto_capture:` switch.
const obsoleteAutoCaptureRemedy = "auto_capture:\n  enabled: true|false\n  types: all"

// decodeLayer walks the parsed mapping against the registry, returning every
// declared leaf in document order plus that layer's diagnostics. A nil root is
// an absent layer and decodes to nothing.
//
// Decode never stops at the first problem: one bad setting must not hide the
// rest of the file. It does refuse to descend into an unknown subtree, because
// every path below a typo would be a derived complaint about the same typo.
func decodeLayer(root *yaml.Node, src Source) ([]leafDecl, []Diagnostic) {
	if root == nil {
		return nil, nil
	}
	d := &layerDecoder{src: src}
	d.walk(root, nil)
	return d.leaves, d.diags
}

type layerDecoder struct {
	src    Source
	leaves []leafDecl
	diags  []Diagnostic
}

func (d *layerDecoder) walk(n *yaml.Node, segs []string) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			// A complex (non-scalar) key names no setting docket can look up.
			d.diags = append(d.diags, leafDiag(d.src, strings.Join(segs, "."), CodeUnknownKey, key,
				"section keys must be plain names, got %s", nodeShape(key)))
			continue
		}

		child := make([]string, len(segs)+1)
		copy(child, segs)
		child[len(segs)] = key.Value
		path := strings.Join(child, ".")

		switch m := matchPath(child); {
		case m.spec != nil:
			d.decodeLeaf(path, m.spec, key, val)
		case m.interior:
			d.decodeInterior(path, child, val)
		case m.warn:
			diag := leafDiag(d.src, path, CodeUnknownKey, key,
				"is not a setting docket understands and is ignored")
			diag.Severity = SeverityWarning
			d.diags = append(d.diags, diag)
		default:
			d.diags = append(d.diags, leafDiag(d.src, path, CodeUnknownKey, key,
				"is not a docket configuration setting"))
		}
	}
}

// decodeInterior handles a key that names a section rather than a setting.
func (d *layerDecoder) decodeInterior(path string, segs []string, val *yaml.Node) {
	if val.Kind == yaml.ScalarNode && val.Tag == tagNull {
		return // `finalize:` with nothing under it declares nothing
	}
	// The pre-0127 scalar `auto_capture: true` is detected on the raw node —
	// the condition itself, not a residue of it — so the reader is told about
	// the shape they wrote rather than about a mapping they did not.
	if path == "auto_capture" && val.Kind == yaml.ScalarNode {
		diag := leafDiag(d.src, path, CodeInvalidValue, val,
			"is a section, not a switch: the scalar form is no longer supported")
		diag.Remedy = obsoleteAutoCaptureRemedy
		d.diags = append(d.diags, diag)
		return
	}
	if val.Kind != yaml.MappingNode {
		d.diags = append(d.diags, leafDiag(d.src, path, CodeInvalidType, val,
			"is a section and expects a mapping of settings, got %s", nodeShape(val)))
		return
	}
	d.walk(val, segs)
}

// decodeLeaf validates one declared setting through its registry row and
// records it, applying the two per-path rules that outlive validation:
// runtime.bash is warned about and never resolved, and unknown board surfaces
// are warned about and dropped.
func (d *layerDecoder) decodeLeaf(path string, spec *pathSpec, key, val *yaml.Node) {
	value, diags := spec.validate(d.src, path, val)
	if len(diags) > 0 {
		d.diags = append(d.diags, diags...)
		return
	}

	switch spec.path {
	case "runtime.bash":
		// Obsolete in EVERY layer: docket no longer has a Bash runtime to
		// select, so the setting is reported and excluded from resolution
		// rather than fenced (a fence would imply some layer could honor it).
		diag := leafDiag(d.src, path, CodeObsoleteSetting, key,
			"selected the Bash implementation, which docket no longer ships; it is ignored")
		diag.Severity = SeverityWarning
		diag.Classification = Obsolete
		diag.Remedy = "remove runtime.bash from this file"
		d.diags = append(d.diags, diag)
		return
	case "board_surfaces":
		value = d.keepKnownSurfaces(path, val, value.([]string))
	}

	d.leaves = append(d.leaves, leafDecl{
		path:  path,
		spec:  spec,
		value: value,
		// The KEY position, not the value's: provenance answers "where is this
		// setting declared", and that is where the reader will look.
		prov: Provenance{Layer: d.src.Layer, Source: d.src.Name, Line: key.Line, Column: key.Column},
	})
}

// keepKnownSurfaces drops unrecognised board surfaces with a warning each and
// returns the surviving tokens in their declared order.
func (d *layerDecoder) keepKnownSurfaces(path string, val *yaml.Node, tokens []string) []string {
	kept := make([]string, 0, len(tokens))
	for i, token := range tokens {
		if inList(boardSurfaceTokens, token) {
			kept = append(kept, token)
			continue
		}
		at := val
		if i < len(val.Content) {
			at = val.Content[i]
		}
		diag := leafDiag(d.src, path, CodeUnknownKey, at,
			"does not name a board surface docket publishes to (%s); it is ignored",
			strings.Join(boardSurfaceTokens, ", "))
		diag.Severity = SeverityWarning
		d.diags = append(d.diags, diag)
	}
	return kept
}

// pathMatch is what one concrete path is: a registry leaf, a section that may
// be descended into, or unknown — where `warn` marks the deliberate v0.9.2
// warn-and-ignore surfaces rather than the default error.
type pathMatch struct {
	spec     *pathSpec
	interior bool
	warn     bool
}

// matchPath resolves a concrete path to its registry row. The dynamic subtrees
// are matched segment by segment so a typo is reported at the segment that is
// wrong — `agents.cluade`, not the model leaf underneath it — and so an
// unknown name in a NAMED position is an error: silently discarding an
// intended model override would run the wrong model.
func matchPath(segs []string) pathMatch {
	switch segs[0] {
	case "agents":
		return matchAgentsPath(segs)
	case "runners":
		return matchRunnersPath(segs)
	case "skills":
		return matchSkillsPath(segs)
	}

	path := strings.Join(segs, ".")
	if spec := specByPath(path); spec != nil {
		return pathMatch{spec: spec}
	}
	prefix := path + "."
	for i := range registryTable {
		if strings.HasPrefix(registryTable[i].path, prefix) {
			return pathMatch{interior: true}
		}
	}
	return pathMatch{}
}

func matchAgentsPath(segs []string) pathMatch {
	switch len(segs) {
	case 1:
		return pathMatch{interior: true}
	case 2:
		if inList(agentHarnesses, segs[1]) {
			return pathMatch{interior: true}
		}
	case 3:
		if inList(agentShortNames, segs[2]) {
			return pathMatch{interior: true}
		}
	case 4:
		if spec := specByPath("agents.*.*." + segs[3]); spec != nil {
			return pathMatch{spec: spec}
		}
	}
	return pathMatch{}
}

func matchRunnersPath(segs []string) pathMatch {
	switch len(segs) {
	case 1:
		return pathMatch{interior: true}
	case 2:
		if inList(runnerNames, segs[1]) {
			return pathMatch{interior: true}
		}
	case 3:
		// A runner-specific row first, then the rows every runner shares. A
		// setting one runner owns (`runners.codex.sandbox`) is unknown under
		// another runner, because there it would never be read.
		if spec := specByPath("runners." + segs[1] + "." + segs[2]); spec != nil {
			return pathMatch{spec: spec}
		}
		if spec := specByPath("runners.*." + segs[2]); spec != nil {
			return pathMatch{spec: spec}
		}
	}
	return pathMatch{}
}

func matchSkillsPath(segs []string) pathMatch {
	switch len(segs) {
	case 1:
		return pathMatch{interior: true}
	case 2:
		if spec := specByPath("skills." + segs[1]); spec != nil {
			return pathMatch{spec: spec}
		}
		// The one place an unknown key is a warning rather than an error:
		// v0.9.2 accepted arbitrary role bindings and ignored the ones it did
		// not run, and a config that worked must not start failing.
		return pathMatch{warn: true}
	}
	return pathMatch{}
}

// specByPath returns the registry row with exactly this path — the dynamic
// rows are looked up by their "*" spelling. The pointer is into the package's
// one table; callers read it and never mutate it.
func specByPath(path string) *pathSpec {
	for i := range registryTable {
		if registryTable[i].path == path {
			return &registryTable[i]
		}
	}
	return nil
}
