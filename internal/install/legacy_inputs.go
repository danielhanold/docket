package install

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
)

// This file turns the install path's already-resolved global inputs into the
// LegacyInputs the frozen reproducer consumes (change 0322, Task B5). It is the
// production wiring that makes inspect.go's LegacyReproducer seam non-nil.
//
// The pin resolution is deliberately kept independent of this binary's LIVE
// harness-defaults (config's builtinAgents, agents/harness-defaults.yml). The
// reproducer is FROZEN — what counts as "a legacy install" must never shift
// when a shipped default changes — so the resolution floor here is the embedded
// v0.9.2 harness-defaults SNAPSHOT below, overlaid only by the user's own GLOBAL
// agent overrides. HEAD's live floor has already drifted from v0.9.2 (it adds a
// `plan-writer` row per harness), and it may drift further for the shipped
// agents; reading the resolved config.AgentsTable floor directly would couple
// adoption to that drift (spec "Legacy adoption contract"; learning
// `shared-resource-keeps-first-owner-assumptions`).

// legacyHarnessDefaultsYML is the FROZEN v0.9.2 agents/harness-defaults.yml
// sidecar — the pre-normalization resolution floor the v0.9.2 Bash installer
// resolved its pins against. It is a byte-for-byte copy of the capture input
// testdata/legacy/_inputs/harness-defaults.yml (guarded by
// TestLegacyHarnessDefaultsFrozenCopy), kept under legacydata/ so a later
// change to the live sidecar cannot reach it.
//
//go:embed legacydata/harness-defaults.yml
var legacyHarnessDefaultsYML []byte

// legacyHarnessDefaults is the parsed frozen floor: harness token -> agent
// short-name -> the (model, effort) the v0.9.2 sidecar assigned. Efforts are
// carried as the sidecar spelled them (the `auto` sentinel included, e.g. every
// cursor row) — the reproducer's per-harness emitters apply their own
// normalization, so a floor `auto` renders identically to a resolved empty.
var legacyHarnessDefaults = parseLegacyHarnessDefaults(legacyHarnessDefaultsYML)

// parseLegacyHarnessDefaults reads the frozen sidecar's fixed two-level shape:
//
//	agents:
//	  <harness>:
//	    <agent>: { model: <bare-scalar>, effort: <bare-scalar> }
//
// It is a purpose-built reader for THIS frozen input, not a general YAML parser:
// comment and blank lines are skipped, a 2-space key is a harness header, and a
// 4-space `{ model: …, effort: … }` entry is one agent pin. A parse that does
// not recover the expected 4 harnesses each carrying a complete 16-agent block
// panics at init — a corrupt frozen floor is a build-time defect, never a
// silent empty table that would refuse every adoption.
func parseLegacyHarnessDefaults(data []byte) map[string]map[string]HarnessPin {
	out := map[string]map[string]HarnessPin{}
	harness := ""
	for _, line := range strings.Split(string(data), "\n") {
		body := strings.TrimSpace(line)
		if body == "" || strings.HasPrefix(body, "#") || body == "agents:" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		switch {
		case indent == 2 && !strings.Contains(body, "{") && strings.HasSuffix(body, ":"):
			harness = strings.TrimSuffix(body, ":")
			out[harness] = map[string]HarnessPin{}
		case indent == 4 && harness != "":
			colon := strings.IndexByte(body, ':')
			if colon < 0 {
				continue
			}
			agent := strings.TrimSpace(body[:colon])
			out[harness][agent] = HarnessPin{
				Model:  legacyScalarField(body, "model"),
				Effort: legacyScalarField(body, "effort"),
			}
		}
	}
	if err := validateLegacyFloor(out); err != nil {
		panic("install: frozen legacy harness-defaults floor: " + err.Error())
	}
	return out
}

// legacyScalarField returns the bare scalar following `<key>:` inside a
// `{ … }` flow entry, up to the next `,` or `}` — the same span the v0.9.2
// reader consumed (harness-defaults.yml's "bare scalars, up to the next `,` or
// `}`" rule).
func legacyScalarField(s, key string) string {
	i := strings.Index(s, key+":")
	if i < 0 {
		return ""
	}
	rest := s[i+len(key)+1:]
	if end := strings.IndexAny(rest, ",}"); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(rest)
}

// validateLegacyFloor refuses a floor that does not carry every shipped harness
// with a complete built-in agent block, so a mangled embed fails loudly at init
// rather than silently disabling adoption for the missing rows.
func validateLegacyFloor(floor map[string]map[string]HarnessPin) error {
	for _, harness := range []string{"claude", "codex", "cursor", "opencode"} {
		row, ok := floor[harness]
		if !ok {
			return fmt.Errorf("harness %q missing", harness)
		}
		for _, agent := range legacyBuiltinAgents {
			pin, ok := row[agent]
			if !ok {
				return fmt.Errorf("harness %q missing agent %q", harness, agent)
			}
			if pin.Model == "" || pin.Effort == "" {
				return fmt.Errorf("harness %q agent %q has an empty model/effort", harness, agent)
			}
		}
	}
	return nil
}

// legacyBuiltinAgents is the closed set of v0.9.2 built-in agent short-names —
// the sixteen the frozen sidecar and legacydata bodies both carry. HEAD's
// seventeenth (`plan-writer`) is deliberately absent: v0.9.2 never wrote one, so
// no legacy target exists for it and the reproducer must not invent a pin.
var legacyBuiltinAgents = []string{
	"adr", "auto-groom", "auto-groom-critic", "brainstorm-consultant",
	"build-economy", "build-standard", "build-premium", "build-max",
	"finalize-change", "implement-next", "integration-repair",
	"rebase-resolver", "review-lean", "review-standard", "review-deep", "status",
}

// legacyInputsFor builds the frozen LegacyInputs for one install's resolved
// global inputs: the harness set it plans for, and the resolved (model, effort)
// pin per built-in agent on each of those harnesses. Each pin starts at the
// FROZEN v0.9.2 floor and takes the user's value ONLY where a field was set by
// the GLOBAL configuration layer — the exact overlay v0.9.2 performed (floor ⊕
// global, field by field, the global `agents.default` fallback already folded
// into the resolved table by config.resolveAgents). A field the global layer
// did not touch keeps the frozen floor value, never HEAD's live default.
func legacyInputsFor(harnesses []string, agents config.AgentsTable) LegacyInputs {
	pins := map[string]AgentPin{}
	for _, h := range harnesses {
		floor, ok := legacyHarnessDefaults[h]
		if !ok {
			continue // a harness with no v0.9.2 sidecar block is outside the frozen inventory
		}
		for agent, base := range floor {
			model, effort := base.Model, base.Effort
			if set, ok := agents[h][agent]; ok {
				if set.Model.Provenance.Layer == config.LayerGlobal {
					model = set.Model.Value
				}
				if set.Effort.Provenance.Layer == config.LayerGlobal {
					effort = set.Effort.Value
				}
			}
			ap, exists := pins[agent]
			if !exists {
				ap = AgentPin{ByHarness: map[string]HarnessPin{}}
			}
			ap.ByHarness[h] = HarnessPin{Model: model, Effort: effort}
			pins[agent] = ap
		}
	}
	return LegacyInputs{Harnesses: append([]string(nil), harnesses...), AgentPins: pins}
}

// LegacyReproducerFor is the exported production seam builder the app layer needs
// to proof-gate repository-surface removals with the SAME frozen reproducer the
// machine transaction inspects against (change 0351). It delegates to the
// unexported builder so the two can never diverge; a nil Config disables the seam.
func LegacyReproducerFor(o Options, harnesses []string) LegacyReproducer {
	return legacyReproducerFor(o, harnesses)
}

// legacyReproducerFor is the production seam builder: the frozen reproducer over
// the inputs an install has already resolved, for the harnesses it plans for. A
// nil Config disables the seam (returns nil), preserving the pre-wiring
// behaviour for callers — and tests — that supply no configuration.
func legacyReproducerFor(o Options, harnesses []string) LegacyReproducer {
	if o.Config == nil {
		return nil
	}
	return NewLegacyReproducer(legacyInputsFor(harnesses, o.Config.Effective.Agents))
}
