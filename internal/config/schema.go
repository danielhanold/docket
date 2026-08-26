package config

import (
	"fmt"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// This file is THE schema/policy registry: every configuration path docket
// v0.9.2 understands, its YAML kind, built-in default, merge rule, layer
// scope, Go v1 classification family, and the validator that turns its node
// into a typed value. Decode, resolution, classification, the mutation
// preflight, and the exhaustive tests all read this one table — a second key
// list anywhere else is a bug, because the two would drift.

// valueKind is the YAML shape a leaf accepts.
type valueKind int

const (
	kindString valueKind = iota
	kindBool             // strictly `true` / `false`
	kindInt              // non-negative
	kindStringList
	kindStringOrList // scalar `all` or a list
	kindScalarOrMap  // github_project
	kindMap          // interior nodes: handled structurally, never a leaf
)

// mergeRule says how a higher layer's declaration combines with a lower one's.
// Lists replace whole — docket never merges two lists, because a repository
// that narrows a global list must be able to narrow it.
type mergeRule int

const (
	mergeScalar mergeRule = iota
	mergeListReplace
)

// layerScope is the coordination fence: which layers may declare a setting at
// all. A declaration outside the fence is warned about and excluded from
// resolution, never silently honored.
type layerScope int

const (
	scopeAny        layerScope = iota
	scopeRepoFenced            // machine-layer declaration → fenced-setting-ignored, excluded
	// scopeLocalOnly: a committed-layer declaration may not be honored. Today
	// the only row carrying this scope is the obsolete `runtime.bash`, whose
	// committed-layer fence is enforced at DECODE (it is excluded there as
	// obsolete and never reaches applyFence), so resolution carries no
	// scopeLocalOnly branch. A future non-obsolete scopeLocalOnly row must add
	// the resolution-time fence back in applyFence, together with a test that
	// reddens when that fence is stripped.
	scopeLocalOnly
)

// disposition is the STATIC classification family of a path. The value- and
// layer-dependent half (does this declaration actually activate a deferred
// capability?) is applied by the classifier, not here.
type disposition int

const (
	dispSupported disposition = iota
	dispObsolete
	dispInert
	dispDeferred           // bool-style: false inactive, true blocks
	dispDeferredByValue    // finalize.gate: ci/both block
	dispSupportedOrDropped // board_surfaces: the github token
	dispInertCompanion     // auto_capture.types, dummy_mode.persona/surfaces, runners.*
	dispAgentsLeaf         // agents.<h>.<a>.model/effort: layer-dependent
	dispDeferredActive     // skills.*, agents...runner: any explicit value blocks
)

// leafValidator turns one leaf's YAML node into its typed Go value. It is
// pure: it reads the node and the source's identity, and returns either a
// value or diagnostics — never both, and never a partially built value.
type leafValidator func(src Source, path string, n *yaml.Node) (any, []Diagnostic)

// pathSpec is one registry row.
type pathSpec struct {
	path     string // dotted; dynamic segments are spelled "*" ("agents.*.*.model")
	kind     valueKind
	enum     []string // non-nil for enum-constrained strings
	def      any      // built-in default; nil = no default (absent built-in)
	merge    mergeRule
	scope    layerScope
	disp     disposition
	validate leafValidator
}

// The closed name sets of the dynamic subtrees. A name outside these is a
// typo, and a typo is an error rather than a warning: silently discarding an
// intended model override would run the wrong model.
var (
	agentHarnesses = []string{"default", "claude", "codex", "cursor", "opencode"}

	// agentShortNames is Reference C's canonical order — the same order the
	// built-in 17x4 agent table is written in.
	agentShortNames = []string{
		"adr", "auto-groom", "auto-groom-critic", "brainstorm-consultant",
		"build-economy", "build-standard", "build-premium", "build-max",
		"finalize-change", "implement-next", "integration-repair",
		"plan-writer", "rebase-resolver", "review-lean", "review-standard",
		"review-deep", "status",
	}

	runnerNames = []string{"codex", "cursor", "opencode"}

	skillRoles = []string{"brainstorm", "plan", "build", "review", "finish"}

	// dummySurfaceTokens is the closed token set of dummy_mode.surfaces.
	dummySurfaceTokens = []string{"dialogue", "reports", "results", "change-sections", "pr"}

	// agentHarnessTokens is the closed token set of agent_harnesses: the four
	// shipped parent-facing harnesses. `default` is an agents-table selector,
	// never a harness a repository opts a dispatch surface into, so it is
	// deliberately absent here.
	agentHarnessTokens = []string{"claude", "codex", "cursor", "opencode"}
)

// changeTypeToken is the shape of a change type; `all` and `untyped` match it
// but are reserved selector words, so they are rejected separately.
var changeTypeToken = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var reservedChangeTypes = []string{"all", "untyped"}

// registryTable is built once; registry() hands out the same slice, which
// callers read and never mutate.
var registryTable = buildRegistry()

// registry returns the full static table — Reference B's rows in Reference B's
// order, with rows 32-38 spelled as the dynamic patterns decode matches
// concrete paths against.
func registry() []pathSpec { return registryTable }

func buildRegistry() []pathSpec {
	dirLeaf := stringLeaf(true, false, true)
	return []pathSpec{
		// 1: the Bash runtime is gone; the setting is warned about wherever it
		// appears and never resolved.
		{path: "runtime.bash", kind: kindString, merge: mergeScalar, scope: scopeLocalOnly,
			disp: dispObsolete, validate: stringLeaf(false, false, false)},

		// 2-6: repository identity — coordination-fenced, so a machine layer
		// cannot silently point one clone at a different metadata branch.
		{path: "metadata_branch", kind: kindString, enum: []string{"docket", "main"}, def: "docket",
			merge: mergeScalar, scope: scopeRepoFenced, disp: dispSupported,
			validate: enumLeaf("docket", "main")},
		{path: "integration_branch", kind: kindString, def: "auto",
			merge: mergeScalar, scope: scopeRepoFenced, disp: dispSupported,
			validate: stringLeaf(true, false, false)},
		{path: "changes_dir", kind: kindString, def: "docs/changes",
			merge: mergeScalar, scope: scopeRepoFenced, disp: dispSupported, validate: dirLeaf},
		{path: "adrs_dir", kind: kindString, def: "docs/adrs",
			merge: mergeScalar, scope: scopeRepoFenced, disp: dispSupported, validate: dirLeaf},
		{path: "results_dir", kind: kindString, def: "docs/results",
			merge: mergeScalar, scope: scopeRepoFenced, disp: dispSupported, validate: dirLeaf},

		// 7-10: finalize.
		{path: "finalize.gate", kind: kindString, enum: []string{"local", "ci", "both", "off"}, def: "local",
			merge: mergeScalar, scope: scopeAny, disp: dispDeferredByValue,
			validate: enumLeaf("local", "ci", "both", "off")},
		{path: "finalize.test_command", kind: kindString, def: "auto",
			merge: mergeScalar, scope: scopeAny, disp: dispSupported,
			validate: stringLeaf(false, false, false)},
		{path: "finalize.require_pr_approval", kind: kindBool, def: false,
			merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: boolLeaf()},
		{path: "finalize.skip_results_only_delta", kind: kindBool, def: false,
			merge: mergeScalar, scope: scopeRepoFenced, disp: dispDeferred, validate: boolLeaf()},

		// 11-12: learnings.
		{path: "learnings.enabled", kind: kindBool, def: true,
			merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: boolLeaf()},
		{path: "learnings.cap", kind: kindInt, def: 300,
			merge: mergeScalar, scope: scopeAny, disp: dispInert, validate: intLeaf(0)},

		// 13-14: reclaim.
		{path: "reclaim.lease_ttl", kind: kindInt, def: 72,
			merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(0)},
		{path: "reclaim.auto", kind: kindBool, def: false,
			merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: boolLeaf()},

		// 15: build.
		{path: "build.checkpoint", kind: kindBool, def: false,
			merge: mergeScalar, scope: scopeAny, disp: dispDeferred, validate: boolLeaf()},

		// 16-17: review.
		{path: "review.min_fix_severity", kind: kindString,
			enum: []string{"minor", "important", "blocker"}, def: "minor",
			merge: mergeScalar, scope: scopeAny, disp: dispSupported,
			validate: enumLeaf("minor", "important", "blocker")},
		{path: "review.max_fix_tasks", kind: kindInt, def: 10,
			merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(0)},

		// 18-19: observation budgets.
		{path: "gate_observation_budget", kind: kindInt, def: 30,
			merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: intLeaf(0)},
		{path: "delegation_observation_budget", kind: kindInt, def: 60,
			merge: mergeScalar, scope: scopeAny, disp: dispInert, validate: intLeaf(0)},

		// 20-23: board, project, publish, groom.
		{path: "board_surfaces", kind: kindStringList, def: []string{"inline"},
			merge: mergeListReplace, scope: scopeAny, disp: dispSupportedOrDropped,
			validate: listLeaf(listOpts{})},
		{path: "github_project", kind: kindScalarOrMap, def: "auto",
			merge: mergeScalar, scope: scopeRepoFenced, disp: dispInert,
			validate: githubProjectLeaf()},
		{path: "terminal_publish", kind: kindBool, def: false,
			merge: mergeScalar, scope: scopeRepoFenced, disp: dispDeferred, validate: boolLeaf()},
		{path: "auto_groom", kind: kindBool, def: false,
			merge: mergeScalar, scope: scopeAny, disp: dispDeferred, validate: boolLeaf()},

		// 24: change types.
		{path: "change_types", kind: kindStringList,
			def:   []string{"chore", "docs", "feat", "fix", "refactor", "perf"},
			merge: mergeListReplace, scope: scopeAny, disp: dispSupported,
			validate: listLeaf(listOpts{
				nonEmpty: true,
				dupFree:  true,
				pattern:  changeTypeToken,
				reserved: reservedChangeTypes,
			})},

		// 25-26: auto capture. The subset check against effective change_types
		// cannot run here — it is cross-leaf, so resolution owns it.
		{path: "auto_capture.enabled", kind: kindBool, def: false,
			merge: mergeScalar, scope: scopeAny, disp: dispDeferred, validate: boolLeaf()},
		{path: "auto_capture.types", kind: kindStringOrList, def: "all",
			merge: mergeListReplace, scope: scopeAny, disp: dispInertCompanion,
			validate: stringOrListLeaf("all", listOpts{dupFree: true, pattern: changeTypeToken})},

		// 27-29: dummy mode.
		{path: "dummy_mode.enabled", kind: kindBool, def: false,
			merge: mergeScalar, scope: scopeAny, disp: dispDeferred, validate: boolLeaf()},
		{path: "dummy_mode.persona", kind: kindString, def: "",
			merge: mergeScalar, scope: scopeAny, disp: dispInertCompanion,
			validate: stringLeaf(false, false, false)},
		{path: "dummy_mode.surfaces", kind: kindStringOrList, def: "all",
			merge: mergeListReplace, scope: scopeAny, disp: dispInertCompanion,
			validate: stringOrListLeaf("all", listOpts{allowed: dummySurfaceTokens})},

		// 30: harness list — the repository's explicit parent-facing dispatch
		// opt-in. Provenance decides write authority (see Effective.AgentHarnesses),
		// so resolution is scope-open; only the repository/repository-local layers
		// grant the installer authority to touch repository surfaces.
		{path: "agent_harnesses", kind: kindStringList,
			merge: mergeListReplace, scope: scopeAny, disp: dispSupported,
			validate: listLeaf(listOpts{dupFree: true, allowed: agentHarnessTokens})},

		// 31: skill bindings. Every role is a deferred capability, so ANY
		// explicit leaf blocks — even one repeating the shipped default.
		{path: "skills.brainstorm", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispDeferredActive, validate: stringLeaf(true, false, false)},
		{path: "skills.plan", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispDeferredActive, validate: stringLeaf(true, false, false)},
		{path: "skills.build", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispDeferredActive, validate: stringLeaf(true, false, false)},
		{path: "skills.review", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispDeferredActive, validate: stringLeaf(true, false, false)},
		{path: "skills.finish", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispDeferredActive, validate: stringLeaf(true, false, false)},

		// 32-33: agent pins. Model IDs are opaque passthrough — docket never
		// asserts a vendor ID exists — but they must be single tokens. The
		// built-in defaults are the 17x4 table of Reference C, which lives in
		// defaults.go rather than in a `def` cell here.
		{path: "agents.*.*.model", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispAgentsLeaf, validate: stringLeaf(true, true, false)},
		{path: "agents.*.*.effort", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispAgentsLeaf, validate: stringLeaf(true, true, false)},
		{path: "agents.*.*.runner", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispDeferredActive, validate: stringLeaf(true, false, false)},

		// 34-38: runner settings — inert companions, activated only by an
		// effective agent entry naming that runner.
		{path: "runners.codex.sandbox", kind: kindString,
			enum:  []string{"workspace-write", "danger-full-access"},
			merge: mergeScalar, scope: scopeAny, disp: dispInertCompanion,
			validate: enumLeaf("workspace-write", "danger-full-access")},
		{path: "runners.codex.network", kind: kindBool,
			merge: mergeScalar, scope: scopeAny, disp: dispInertCompanion, validate: boolLeaf()},
		{path: "runners.opencode.permissions", kind: kindString,
			enum:  []string{"ask", "auto-approve"},
			merge: mergeScalar, scope: scopeAny, disp: dispInertCompanion,
			validate: enumLeaf("ask", "auto-approve")},
		{path: "runners.*.shim_model", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispInertCompanion, validate: stringLeaf(true, true, false)},
		{path: "runners.*.shim_effort", kind: kindString, merge: mergeScalar, scope: scopeAny,
			disp: dispInertCompanion, validate: stringLeaf(true, true, false)},
	}
}

// githubProject is the parsed `github_project` value: the `auto` sentinel, or
// an explicit owner/number pair.
type githubProject struct {
	Auto   bool
	Owner  string
	Number int
}

// YAML resolved tags this package reasons about by name.
const (
	tagStr  = "!!str"
	tagBool = "!!bool"
	tagInt  = "!!int"
	tagNull = "!!null"
)

// leafDiag builds an error diagnostic positioned at n and attributed to path.
// It reuses parse.go's nodeDiag so every diagnostic in the package carries
// provenance the same way.
func leafDiag(src Source, path, code string, n *yaml.Node, format string, args ...any) Diagnostic {
	d := nodeDiag(src, code, n.Line, n.Column,
		fmt.Sprintf("%s: %s: %s", src.Name, path, fmt.Sprintf(format, args...)))
	d.Path = path
	return d
}

// nodeShape names what was found, for invalid-type messages. It quotes the
// offending scalar only — never a sibling, and never the document.
func nodeShape(n *yaml.Node) string {
	switch n.Kind {
	case yaml.MappingNode:
		return "a mapping"
	case yaml.SequenceNode:
		return "a list"
	case yaml.AliasNode:
		return "an alias"
	case yaml.ScalarNode:
		if n.Tag == tagNull {
			return "an empty value"
		}
		return fmt.Sprintf("%s %q", strings.TrimPrefix(n.Tag, "!!"), n.Value)
	}
	return "an unsupported value"
}

// scalarWithTag accepts n only when it is a scalar carrying the resolved tag
// wanted. Keying on the RESOLVED tag rather than on the raw text is what makes
// the strict spellings work: `yes`, `off` and `"true"` all resolve to !!str,
// so a bool leaf rejects them without enumerating them.
func scalarWithTag(src Source, path string, n *yaml.Node, tag, want string) []Diagnostic {
	if n.Kind != yaml.ScalarNode || n.Tag != tag {
		return []Diagnostic{leafDiag(src, path, CodeInvalidType, n,
			"expects %s, got %s", want, nodeShape(n))}
	}
	return nil
}

func boolLeaf() leafValidator {
	return func(src Source, path string, n *yaml.Node) (any, []Diagnostic) {
		if diags := scalarWithTag(src, path, n, tagBool, "a boolean (true or false)"); diags != nil {
			return nil, diags
		}
		switch n.Value {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		// Reachable only through an explicit `!!bool` tag on a YAML 1.1
		// spelling; docket honours exactly two spellings.
		return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
			"expects a boolean spelled true or false, got %q", n.Value)}
	}
}

func intLeaf(minimum int) leafValidator {
	return func(src Source, path string, n *yaml.Node) (any, []Diagnostic) {
		if diags := scalarWithTag(src, path, n, tagInt, "an integer"); diags != nil {
			return nil, diags
		}
		// Atoi, not ParseInt(base 0): YAML resolves 0x10 and 1_0 to !!int, and
		// docket accepts plain decimal only.
		v, err := strconv.Atoi(n.Value)
		if err != nil {
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
				"expects a plain decimal integer, got %q", n.Value)}
		}
		if v < minimum {
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
				"must be >= %d, got %d", minimum, v)}
		}
		return v, nil
	}
}

func enumLeaf(allowed ...string) leafValidator {
	return func(src Source, path string, n *yaml.Node) (any, []Diagnostic) {
		if diags := scalarWithTag(src, path, n, tagStr, "a string"); diags != nil {
			return nil, diags
		}
		for _, a := range allowed {
			if n.Value == a {
				return n.Value, nil
			}
		}
		return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
			"must be one of %s, got %q", strings.Join(allowed, ", "), n.Value)}
	}
}

// stringLeaf builds a string validator. nonEmpty rejects ""; spaceFree rejects
// any whitespace (a model or effort pin is one token); relPath additionally
// requires a clean, relative, parent-free path.
func stringLeaf(nonEmpty, spaceFree, relPath bool) leafValidator {
	return func(src Source, path string, n *yaml.Node) (any, []Diagnostic) {
		if diags := scalarWithTag(src, path, n, tagStr, "a string"); diags != nil {
			return nil, diags
		}
		v := n.Value
		if (nonEmpty || relPath) && v == "" {
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
				"must not be empty")}
		}
		if spaceFree && strings.ContainsAny(v, " \t\n") {
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
				"must be a single token with no whitespace, got %q", v)}
		}
		if relPath {
			if diag, bad := relPathDiag(src, path, n, v); bad {
				return nil, []Diagnostic{diag}
			}
		}
		return v, nil
	}
}

// relPathDiag enforces the directory-setting rule: relative, already clean,
// and with no parent segment. `docs/../etc` is rejected on the clean test
// rather than resolved, so what the file says is what the tool uses.
func relPathDiag(src Source, spath string, n *yaml.Node, v string) (Diagnostic, bool) {
	if filepath.IsAbs(v) || strings.HasPrefix(v, "/") {
		return leafDiag(src, spath, CodeInvalidValue, n,
			"must be a relative path, got %q", v), true
	}
	if slashpath.Clean(v) != v {
		return leafDiag(src, spath, CodeInvalidValue, n,
			"must be a clean relative path (%q would resolve to %q)", v, slashpath.Clean(v)), true
	}
	for _, seg := range strings.Split(v, "/") {
		if seg == ".." {
			return leafDiag(src, spath, CodeInvalidValue, n,
				"must not escape the repository, got %q", v), true
		}
	}
	return Diagnostic{}, false
}

// listOpts constrains a list of string tokens.
type listOpts struct {
	nonEmpty bool           // the list itself must have at least one element
	dupFree  bool           // no token may repeat
	allowed  []string       // non-nil: closed token set
	pattern  *regexp.Regexp // non-nil: every token must match
	reserved []string       // tokens that are syntactically fine but reserved words
}

func listLeaf(o listOpts) leafValidator {
	return func(src Source, path string, n *yaml.Node) (any, []Diagnostic) {
		if n.Kind != yaml.SequenceNode {
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidType, n,
				"expects a list, got %s", nodeShape(n))}
		}
		if o.nonEmpty && len(n.Content) == 0 {
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
				"must list at least one value")}
		}
		out := make([]string, 0, len(n.Content))
		seen := make(map[string]bool, len(n.Content))
		for _, el := range n.Content {
			if diags := scalarWithTag(src, path, el, tagStr, "a list of strings"); diags != nil {
				return nil, diags
			}
			v := el.Value
			if v == "" {
				return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, el,
					"list entries must not be empty")}
			}
			if o.dupFree && seen[v] {
				return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, el,
					"lists %q more than once", v)}
			}
			if o.pattern != nil && !o.pattern.MatchString(v) {
				return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, el,
					"entry %q must match %s", v, o.pattern.String())}
			}
			for _, r := range o.reserved {
				if v == r {
					return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, el,
						"%q is a reserved selector word and cannot be declared", v)}
				}
			}
			if o.allowed != nil && !inList(o.allowed, v) {
				return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, el,
					"entry %q must be one of %s", v, strings.Join(o.allowed, ", "))}
			}
			seen[v] = true
			out = append(out, v)
		}
		return out, nil
	}
}

// stringOrListLeaf accepts either the bare sentinel scalar (`all`) or a list.
// The returned value is a string for the sentinel and a []string for the list;
// consumers switch on the type.
func stringOrListLeaf(sentinel string, o listOpts) leafValidator {
	list := listLeaf(o)
	return func(src Source, path string, n *yaml.Node) (any, []Diagnostic) {
		if n.Kind == yaml.ScalarNode {
			if n.Tag == tagStr && n.Value == sentinel {
				return sentinel, nil
			}
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
				"expects %q or a list, got %s", sentinel, nodeShape(n))}
		}
		return list(src, path, n)
	}
}

// githubProjectLeaf parses the one scalar-or-map setting: `auto`, or an
// explicit {owner, number} pair. Its own key policy is the strict one — an
// unknown field here is a typo that would otherwise be silently dropped.
func githubProjectLeaf() leafValidator {
	return func(src Source, path string, n *yaml.Node) (any, []Diagnostic) {
		if n.Kind == yaml.ScalarNode {
			if n.Tag == tagStr && n.Value == "auto" {
				return githubProject{Auto: true}, nil
			}
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
				"expects \"auto\" or a mapping of owner and number, got %s", nodeShape(n))}
		}
		if n.Kind != yaml.MappingNode {
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidType, n,
				"expects \"auto\" or a mapping of owner and number, got %s", nodeShape(n))}
		}

		owner := stringLeaf(true, false, false)
		number := intLeaf(1)
		var out githubProject
		var haveOwner, haveNumber bool
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			field := path + "." + key.Value
			switch key.Value {
			case "owner":
				v, diags := owner(src, field, val)
				if diags != nil {
					return nil, diags
				}
				out.Owner, haveOwner = v.(string), true
			case "number":
				v, diags := number(src, field, val)
				if diags != nil {
					// The >= 1 rule is the setting's own, so it is reported
					// against the setting; a type error is reported against
					// the field that carries it.
					if diags[0].Code == CodeInvalidValue {
						return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, val,
							"number must be >= 1, got %q", val.Value)}
					}
					return nil, diags
				}
				out.Number, haveNumber = v.(int), true
			default:
				return nil, []Diagnostic{leafDiag(src, field, CodeUnknownKey, key,
					"is not a github_project field (expected owner, number)")}
			}
		}
		if !haveOwner || !haveNumber {
			return nil, []Diagnostic{leafDiag(src, path, CodeInvalidValue, n,
				"mapping form requires both owner and number")}
		}
		return out, nil
	}
}

func inList(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
