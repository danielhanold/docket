// This file is package harness_test rather than package harness — the guard
// below plans with all four adapter packages, and each of them imports
// harness, so an internal test file importing them would be an import cycle.
// The external test package is the only place a whole-inventory cross-adapter
// assert can live.
package harness_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/claude"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/harness/cursor"
	"github.com/danielhanold/docket/internal/harness/opencode"
	"github.com/danielhanold/docket/internal/install"
)

// The launch spelling each harness's own runner answers to. A rendered agent
// definition naming another harness's runner is a cross-harness delegation
// instruction: an agent told to shell out to a sibling harness would run
// docket's work at a model, a permission posture, and a session the hosting
// harness never authorized. The material docket authors is harness-neutral, so
// the correct count of these substrings in any agent payload is zero — but the
// asset bundle does carry them in skill prose, which is why the guard is keyed
// on the agent payloads an adapter renders rather than on the bundle.
var runnerTokens = map[string]string{
	"claude":   "claude -p",
	"codex":    "codex exec",
	"cursor":   "cursor-agent",
	"opencode": "opencode run",
}

const (
	crossHome   = "/home/u"
	crossConfig = "/home/u/.config"
	crossAssets = "/data/versions/sha256-x/assets"
)

// crossRoots is the one set of roots every adapter plans against, so a path
// landing in a sibling's tree is visible as a collision rather than hidden
// behind per-adapter fixtures.
func crossRoots() install.UserRoots {
	return install.UserRoots{
		Home:       crossHome,
		DataRoot:   crossHome + "/.local/share/docket",
		ConfigHome: crossConfig,
		BinDir:     crossHome + "/.local/bin",
	}
}

// ownedRoots is every directory a harness's targets may sit under. Codex owns
// two: its own home, and the harness-neutral ~/.agents skills root it reads
// its skills from.
func ownedRoots(name string) []string {
	switch name {
	case "claude":
		return []string{filepath.Join(crossHome, ".claude")}
	case "codex":
		return []string{filepath.Join(crossHome, ".codex"), filepath.Join(crossHome, ".agents")}
	case "cursor":
		return []string{filepath.Join(crossHome, ".cursor")}
	case "opencode":
		return []string{filepath.Join(crossConfig, "opencode")}
	}
	return nil
}

func under(path, root string) bool {
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

// crossPlanInput pins every harness on every adapter's own table, so a
// renderer reading a sibling's row would show up as a foreign model string as
// well as a foreign path.
func crossPlanInput(t *testing.T) harness.PlanInput {
	t.Helper()
	c, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatalf("EmbeddedCatalog: %v", err)
	}
	agents := config.AgentsTable{}
	agents["claude"] = map[string]config.AgentSetting{
		"build-standard": {Model: config.Value[string]{Value: "claude-opus-5[1m]"}, Effort: config.Value[string]{Value: "high"}},
	}
	agents["codex"] = map[string]config.AgentSetting{
		"build-standard": {Model: config.Value[string]{Value: "gpt-5.5-codex"}, Effort: config.Value[string]{Value: "high"}},
	}
	agents["cursor"] = map[string]config.AgentSetting{
		"build-standard": {Model: config.Value[string]{Value: "gpt-5.5-cursor"}, Effort: config.Value[string]{Value: "high"}},
	}
	agents["opencode"] = map[string]config.AgentSetting{
		"build-standard": {Model: config.Value[string]{Value: "openrouter/anthropic/claude-opus-5"}, Effort: config.Value[string]{Value: "high"}},
	}
	return harness.PlanInput{
		Assets:    c,
		Mode:      harness.ModeRelease,
		AssetsDir: crossAssets,
		Roots:     crossRoots(),
		Agents:    agents,
	}
}

// TestNoCrossHarnessDelegation plans all four adapters under one shared input
// and asserts the two ways an installation could bleed across harnesses: an
// agent definition that instructs its harness to launch a sibling's runner,
// and a target written into a sibling's root.
func TestNoCrossHarnessDelegation(t *testing.T) {
	in := crossPlanInput(t)
	adapters := map[string]harness.Adapter{
		"claude":   claude.New(),
		"codex":    codex.New(),
		"cursor":   cursor.New(),
		"opencode": opencode.New(),
	}
	if len(adapters) != len(harness.Order) {
		t.Fatalf("the guard covers %d adapters for %d harnesses in Order", len(adapters), len(harness.Order))
	}

	sources, err := harness.ParseInventory(in.Assets)
	if err != nil {
		t.Fatalf("ParseInventory: %v", err)
	}

	for _, name := range harness.Order {
		a, ok := adapters[name]
		if !ok {
			t.Fatalf("no adapter for %q", name)
		}
		if a.Name() != name {
			t.Fatalf("adapter under %q names itself %q", name, a.Name())
		}
		targets, err := a.Plan(in)
		if err != nil {
			t.Fatalf("%s Plan: %v", name, err)
		}
		mine := ownedRoots(name)
		if len(mine) == 0 {
			t.Fatalf("no owned roots declared for %q", name)
		}

		agentPayloads := 0
		for _, tg := range targets {
			// Path containment: inside one of this harness's own roots, and
			// inside none of the other three's.
			ownedBy := false
			for _, root := range mine {
				if under(tg.Path, root) {
					ownedBy = true
				}
			}
			if !ownedBy {
				t.Errorf("%s target %q sits outside its own roots %v", name, tg.Path, mine)
			}
			for _, other := range harness.Order {
				if other == name {
					continue
				}
				for _, root := range ownedRoots(other) {
					if under(tg.Path, root) {
						t.Errorf("%s target %q sits under %s's root %q", name, tg.Path, other, root)
					}
				}
			}

			if tg.Role != "agent" {
				continue
			}
			agentPayloads++
			payload := strings.ToLower(string(tg.Content))
			for other, token := range runnerTokens {
				if other == name {
					continue
				}
				if strings.Contains(payload, token) {
					t.Errorf("%s agent definition %q names %s's runner (%q)", name, filepath.Base(tg.Path), other, token)
				}
			}
		}
		// Non-vacuity: the payload assert above is only meaningful if every
		// agent source actually rendered a payload for this harness.
		if agentPayloads != len(sources) {
			t.Errorf("%s rendered %d agent payloads for %d agent sources", name, agentPayloads, len(sources))
		}
	}
}
