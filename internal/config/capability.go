package config

import (
	"fmt"
	"sort"
	"strings"
)

// This file is classification: it reads a finished resolution and answers, for
// every honored declaration, what that setting MEANS for Go v1 — supported,
// carried but inert, deferred, or obsolete — and whether the deferred behavior
// is actually being requested. Classification is deliberately separate from
// validity: a configuration that asks for a deferred capability is well-formed
// and inspectable, it simply may not be mutated until the request is withdrawn.
//
// Two facts drive every answer: the registry's static disposition, and the
// RESOLVED configuration. The second is why activation lives here and not in
// decode — whether `auto_capture.types` matters is a question about
// `auto_capture.enabled`, which only resolution knows the winner of.

// classify returns the capability entries and the classifier's own
// diagnostics. It never invalidates a snapshot: every code it mints is outside
// invalidClass, deferred-capability-requested included.
func classify(res *resolution) ([]Capability, []Diagnostic) {
	c := &classifier{res: res}
	c.obsoleteCapabilities()
	for _, path := range sortedPaths(res.declared) {
		c.declaration(res.declared[path])
	}
	c.learningsNotice()

	sort.SliceStable(c.caps, func(i, j int) bool { return c.caps[i].Path < c.caps[j].Path })
	return c.caps, c.diags
}

type classifier struct {
	res   *resolution
	caps  []Capability
	diags []Diagnostic
}

func sortedPaths(declared map[string]leafDecl) []string {
	out := make([]string, 0, len(declared))
	for path := range declared {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// declaration classifies one honored declaration. A plain-supported row
// classifies to nothing at all — the capability list is the list of settings a
// reader needs to know something about, not a second copy of the file.
func (c *classifier) declaration(decl leafDecl) {
	switch decl.spec.disp {
	case dispSupported:
		return

	case dispObsolete:
		// Unreachable today: the one obsolete row is excluded from resolution at
		// decode, so it reaches this file as a diagnostic instead
		// (obsoleteCapabilities). Kept so a future obsolete row that IS resolved
		// still classifies rather than vanishing.
		c.emit(decl, Obsolete, false, false,
			fmt.Sprintf("%s names behavior docket no longer ships; the declaration is ignored.", decl.path),
			fmt.Sprintf("remove %s", decl.path))

	case dispInert:
		c.emit(decl, Inert, false, false,
			fmt.Sprintf("%s is preserved and reported, but Go v1 acts on it nowhere.", decl.path), "")
		c.notice(decl, CodeInertSetting, Inert,
			fmt.Sprintf("%s is carried for compatibility and has no effect in Go v1", decl.path), "")

	case dispInertCompanion:
		active := c.companionActive(decl.path)
		c.emit(decl, Inert, active, false, companionReason(decl.path, active), "")
		c.notice(decl, CodeInertSetting, Inert,
			fmt.Sprintf("%s is a companion setting: it is read only once the capability it belongs to is available", decl.path), "")

	case dispDeferred:
		on, _ := decl.value.(bool)
		c.deferred(decl, on, fmt.Sprintf("set %s: false, or remove the key", decl.path))

	case dispDeferredByValue:
		// finalize.gate is the one row whose supported half and deferred half are
		// two values of the same enum: `local` and `off` are policy Go v1 honors,
		// `ci` and `both` ask for a gate it does not run.
		value, _ := decl.value.(string)
		if value != "ci" && value != "both" {
			return
		}
		c.deferred(decl, true, fmt.Sprintf("set %s to local or off, or remove the key", decl.path))

	case dispSupportedOrDropped:
		// Only the fenced `github` token carries a capability question, and a
		// machine layer never reaches here with one: the fence stripped it, and
		// reporting it twice would tell the user to edit a file that is already
		// being ignored.
		tokens, _ := decl.value.([]string)
		if !inList(tokens, boardSurfaceGitHub) {
			return
		}
		c.emit(decl, Deferred, true, true,
			fmt.Sprintf("%s requests the %q board surface, which docket dropped; docket refuses to modify the repository while it is requested.", decl.path, boardSurfaceGitHub),
			fmt.Sprintf("remove the %q token from %s", boardSurfaceGitHub, decl.path))
		c.blockerDiag(decl,
			fmt.Sprintf("%s requests the %q board surface, which Go v1 does not publish to", decl.path, boardSurfaceGitHub),
			fmt.Sprintf("remove the %q token from %s", boardSurfaceGitHub, decl.path))

	case dispAgentsLeaf:
		// The global machine file is the sole override layer. A committed or
		// repository-local pin asks for per-repository model routing — a request,
		// not a value — so it blocks even when it repeats the shipped default.
		if !isRepositoryLayer(decl.prov.Layer) {
			return
		}
		c.emit(decl, Deferred, true, true,
			fmt.Sprintf("%s pins an agent from a repository layer, which requests per-repository model routing that Go v1 defers.", decl.path),
			fmt.Sprintf("move %s to the global docket configuration, or remove it here", decl.path))
		c.blockerDiag(decl,
			fmt.Sprintf("%s pins an agent from a repository layer; only the global configuration may override agent routing in Go v1", decl.path),
			fmt.Sprintf("move %s to the global docket configuration, or remove it here", decl.path))

	case dispDeferredActive:
		// Skill bindings and runner assignments have no inactive spelling: any
		// explicit value is the request.
		c.emit(decl, Deferred, true, true,
			fmt.Sprintf("%s requests behavior Go v1 defers; docket refuses to modify the repository while it is declared.", decl.path),
			fmt.Sprintf("remove %s", decl.path))
		c.blockerDiag(decl,
			fmt.Sprintf("%s requests a capability Go v1 does not implement", decl.path),
			fmt.Sprintf("remove %s", decl.path))
	}
}

// deferred classifies a boolean-shaped deferred switch. Off is off: an explicit
// `false` is a valid, non-blocking declaration that says the user considered
// the feature and declined it.
func (c *classifier) deferred(decl leafDecl, active bool, remedy string) {
	if !active {
		c.emit(decl, Deferred, false, false,
			fmt.Sprintf("%s is declared off, so the deferred behavior it would request is not active.", decl.path), "")
		c.notice(decl, CodeDeferredSetting, Deferred,
			fmt.Sprintf("%s names a capability Go v1 defers; the declaration is inactive and changes nothing", decl.path), "")
		return
	}
	c.emit(decl, Deferred, true, true,
		fmt.Sprintf("%s requests behavior Go v1 defers; docket refuses to modify the repository while it is active.", decl.path),
		remedy)
	c.blockerDiag(decl,
		fmt.Sprintf("%s requests a capability Go v1 does not implement", decl.path), remedy)
}

// obsoleteCapabilities lifts the obsolete rows out of the diagnostics decode
// already produced. They arrive that way because an obsolete setting is
// excluded from resolution in every layer — there is no honored declaration to
// classify, but the reader still needs the setting listed with its file and
// line.
func (c *classifier) obsoleteCapabilities() {
	seen := make(map[string]int)
	for _, d := range c.res.diags {
		if d.Code != CodeObsoleteSetting || d.Provenance == nil {
			continue
		}
		entry := Capability{
			Path:           d.Path,
			Classification: Obsolete,
			Provenance:     *d.Provenance,
			Reason:         fmt.Sprintf("%s selected behavior docket no longer ships; it is ignored everywhere and never blocks a mutation.", d.Path),
			Remedy:         d.Remedy,
		}
		// One entry per setting: a path declared in two layers is one obsolete
		// setting, reported against the highest-precedence declaration (the
		// diagnostics arrive in low-to-high layer order).
		if at, ok := seen[d.Path]; ok {
			c.caps[at] = entry
			continue
		}
		seen[d.Path] = len(c.caps)
		c.caps = append(c.caps, entry)
	}
}

// learningsNotice is the standing exception in the matrix: learnings themselves
// are supported, but the automation around them is not, and a reader whose
// learnings are on deserves to be told which half they are getting. It keys on
// the EFFECTIVE value, so it appears for the built-in default too.
func (c *classifier) learningsNotice() {
	enabled := c.res.effective.Learnings.Enabled
	if !enabled.Value {
		return
	}
	prov := enabled.Provenance
	c.diags = append(c.diags, Diagnostic{
		Code:           CodeDeferredSetting,
		Severity:       SeverityInfo,
		Path:           "learnings.enabled",
		Classification: Deferred,
		Provenance:     &prov,
		Message: "learnings are enabled: manual learning reads and explicit record/update are supported, " +
			"but automatic harvest, index rendering, capacity checks, and promotion are deferred in Go v1",
	})
}

// companionActive applies the aggregate activation rules by SHAPE rather than
// by an enumerated path list: a companion under `<section>.<leaf>` follows that
// section's own `enabled` switch, and a companion under `runners.<name>` follows
// whether any honored agent entry actually assigns that runner.
func (c *classifier) companionActive(path string) bool {
	segs := strings.Split(path, ".")
	if len(segs) < 2 {
		return false
	}
	if segs[0] == "runners" {
		return c.runnerAssigned(segs[1])
	}
	return c.effectiveBool(strings.Join(segs[:len(segs)-1], ".") + ".enabled")
}

func companionReason(path string, active bool) string {
	if active {
		return fmt.Sprintf("%s belongs to a capability that is being requested; it is read once that capability lands, and does not block on its own.", path)
	}
	return fmt.Sprintf("%s belongs to a capability that is not being requested, so it changes nothing.", path)
}

// effectiveBool answers what a boolean leaf resolved to, whether or not it is
// carried in Effective. Deferred switches deliberately never reach Effective —
// it holds supported policy only — so activation reads the honored declaration
// and falls back to the registry's built-in default.
func (c *classifier) effectiveBool(path string) bool {
	if decl, ok := c.res.declared[path]; ok {
		v, _ := decl.value.(bool)
		return v
	}
	spec := specByPath(path)
	if spec == nil {
		return false
	}
	def, _ := spec.def.(bool)
	return def
}

// runnerAssigned reports whether an honored agent entry names this runner. It
// matches the runner leaves by shape ("agents.<h>.<a>.runner"), so a new agent
// or harness name cannot fall out of the activation rule.
func (c *classifier) runnerAssigned(runner string) bool {
	for path, decl := range c.res.declared {
		if !strings.HasPrefix(path, "agents.") || !strings.HasSuffix(path, ".runner") {
			continue
		}
		if name, ok := decl.value.(string); ok && name == runner {
			return true
		}
	}
	return false
}

func isRepositoryLayer(layer LayerKind) bool {
	return layer == LayerRepository || layer == LayerRepositoryLocal
}

func (c *classifier) emit(decl leafDecl, class Classification, active, block bool, reason, remedy string) {
	c.caps = append(c.caps, Capability{
		Path:           decl.path,
		Classification: class,
		Active:         active,
		MutationBlock:  block,
		Provenance:     decl.prov,
		Reason:         reason,
		Remedy:         remedy,
	})
}

// notice records a non-blocking classifier diagnostic against a declaration.
func (c *classifier) notice(decl leafDecl, code string, class Classification, message, remedy string) {
	prov := decl.prov
	c.diags = append(c.diags, Diagnostic{
		Code:           code,
		Severity:       SeverityInfo,
		Path:           decl.path,
		Classification: class,
		Provenance:     &prov,
		Message:        fmt.Sprintf("%s: %s", prov.Source, message),
		Remedy:         remedy,
	})
}

// blockerDiag records the one diagnostic the mutation preflight collects. Its
// severity is error and its code is outside invalidClass on purpose: the
// configuration is valid and fully inspectable, and only a mutation is refused.
func (c *classifier) blockerDiag(decl leafDecl, message, remedy string) {
	prov := decl.prov
	c.diags = append(c.diags, Diagnostic{
		Code:           CodeDeferredCapRequested,
		Severity:       SeverityError,
		Path:           decl.path,
		Classification: Deferred,
		Provenance:     &prov,
		Message:        fmt.Sprintf("%s: %s", prov.Source, message),
		Remedy:         remedy,
	})
}
