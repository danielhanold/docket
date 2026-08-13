package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// sidecarPath is the frozen byte-exact copy of agents/harness-defaults.yml.
// The frozen tree is an immutable input (testdata/README.md): this test only
// reads it.
const sidecarPath = "../../testdata/repositories/v0.9.2/agents-harness-defaults.yml"

// sidecarDoc is the shipped-defaults file's shape: harness → agent short name
// → model/effort pair.
type sidecarDoc struct {
	Agents map[string]map[string]struct {
		Model  string `yaml:"model"`
		Effort string `yaml:"effort"`
	} `yaml:"agents"`
}

// pair is a (model, effort) tuple compared structurally, so a missing or extra
// harness/agent shows up as a difference rather than being skipped.
type pair struct{ Model, Effort string }

// TestBuiltinAgentsParityWithFrozenSidecar compares the frozen sidecar against
// builtinAgents() as WHOLE structures, both directions in one DeepEqual: every
// (harness, agent) pair present in both, models equal, and efforts equal —
// where the sidecar says `auto` the Go table must hold "". Vendor-ID VALIDITY
// is deliberately not asserted: that oracle lives outside the repo.
func TestBuiltinAgentsParityWithFrozenSidecar(t *testing.T) {
	data, err := os.ReadFile(filepath.FromSlash(sidecarPath))
	if err != nil {
		t.Fatalf("reading the frozen sidecar: %v", err)
	}
	var doc sidecarDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing the frozen sidecar: %v", err)
	}
	if len(doc.Agents) == 0 {
		t.Fatal("the frozen sidecar declares no agents — the fixture is wrong")
	}

	want := make(map[string]map[string]pair, len(doc.Agents))
	for harness, agents := range doc.Agents {
		row := make(map[string]pair, len(agents))
		for name, entry := range agents {
			effort := entry.Effort
			if effort == "auto" {
				effort = ""
			}
			row[name] = pair{Model: entry.Model, Effort: effort}
		}
		want[harness] = row
	}

	got := make(map[string]map[string]pair, len(builtinAgents()))
	for harness, agents := range builtinAgents() {
		row := make(map[string]pair, len(agents))
		for name, entry := range agents {
			row[name] = pair{Model: entry.Model.Value, Effort: entry.Effort.Value}
		}
		got[harness] = row
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("built-in agent table differs from the frozen sidecar:\n got: %v\nwant: %v", got, want)
	}
}

// TestBuiltinAgentsShape pins the table's structure independently of the
// sidecar: exactly the four shipped harnesses, each carrying exactly the 16
// canonical short names, every model a non-empty single token, and every entry
// carrying built-in provenance with Explicit false.
func TestBuiltinAgentsShape(t *testing.T) {
	table := builtinAgents()
	wantHarnesses := []string{"claude", "codex", "cursor", "opencode"}
	if len(table) != len(wantHarnesses) {
		t.Fatalf("harness count = %d, want %d (%v)", len(table), len(wantHarnesses), wantHarnesses)
	}
	for _, harness := range wantHarnesses {
		agents, ok := table[harness]
		if !ok {
			t.Fatalf("harness %q missing from the built-in table", harness)
		}
		if len(agents) != len(agentShortNames) {
			t.Errorf("harness %q has %d agents, want %d", harness, len(agents), len(agentShortNames))
		}
		for _, name := range agentShortNames {
			entry, ok := agents[name]
			if !ok {
				t.Errorf("harness %q is missing agent %q", harness, name)
				continue
			}
			if entry.Model.Value == "" {
				t.Errorf("%s/%s: model is empty", harness, name)
			}
			if strings.ContainsAny(entry.Model.Value, " \t\n") {
				t.Errorf("%s/%s: model %q is not a single token", harness, name, entry.Model.Value)
			}
			if entry.Model.Provenance != builtinProvenance() || entry.Effort.Provenance != builtinProvenance() {
				t.Errorf("%s/%s: provenance = %+v/%+v, want %+v",
					harness, name, entry.Model.Provenance, entry.Effort.Provenance, builtinProvenance())
			}
			if entry.Model.Explicit || entry.Effort.Explicit {
				t.Errorf("%s/%s: Explicit must be false on built-in defaults", harness, name)
			}
		}
	}
	// Cursor's IDs already encode their variant, so the effort pin is
	// suppressed across that whole block.
	for _, name := range agentShortNames {
		if got := table["cursor"][name].Effort.Value; got != "" {
			t.Errorf("cursor/%s: effort = %q, want \"\" (suppressed)", name, got)
		}
	}
}

// TestBuiltinEffectiveMatchesRegistryDefaults keeps the one key list honest:
// every registry row carrying a default that lands in Effective must equal
// that row's `def` cell, so defaults.go can never drift from schema.go.
func TestBuiltinEffectiveMatchesRegistryDefaults(t *testing.T) {
	eff := builtinEffective()
	leaves := map[string]any{
		"metadata_branch":              eff.MetadataBranch.Value,
		"integration_branch":           eff.IntegrationBranch.Value,
		"changes_dir":                  eff.ChangesDir.Value,
		"adrs_dir":                     eff.ADRsDir.Value,
		"results_dir":                  eff.ResultsDir.Value,
		"finalize.gate":                eff.Finalize.Gate.Value,
		"finalize.test_command":        eff.Finalize.TestCommand.Value,
		"finalize.require_pr_approval": eff.Finalize.RequirePRApproval.Value,
		"learnings.enabled":            eff.Learnings.Enabled.Value,
		"reclaim.lease_ttl":            eff.Reclaim.LeaseTTL.Value,
		"reclaim.auto":                 eff.Reclaim.Auto.Value,
		"review.min_fix_severity":      eff.Review.MinFixSeverity.Value,
		"review.max_fix_tasks":         eff.Review.MaxFixTasks.Value,
		"gate_observation_budget":      eff.GateObservation.Value,
		"board_surfaces":               eff.BoardSurfaces.Value,
		"change_types":                 eff.ChangeTypes.Value,
	}

	seen := make(map[string]bool, len(leaves))
	for _, spec := range registry() {
		want, ok := leaves[spec.path]
		if !ok {
			continue
		}
		seen[spec.path] = true
		if spec.def == nil {
			t.Errorf("%s: registry row carries no default but lands in Effective", spec.path)
			continue
		}
		if !reflect.DeepEqual(want, spec.def) {
			t.Errorf("%s: builtinEffective() = %#v, registry default = %#v", spec.path, want, spec.def)
		}
	}
	for path := range leaves {
		if !seen[path] {
			t.Errorf("%s: no registry row — Effective and the registry disagree on the key set", path)
		}
	}

	// Every built-in leaf carries built-in provenance and is not explicit;
	// integration_branch still holds the raw `auto` sentinel here, which
	// resolution replaces with the default branch.
	if eff.IntegrationBranch.Value != "auto" {
		t.Errorf("integration_branch default = %q, want the raw \"auto\" sentinel", eff.IntegrationBranch.Value)
	}
	if eff.MetadataBranch.Provenance != builtinProvenance() || eff.MetadataBranch.Explicit {
		t.Errorf("metadata_branch: provenance %+v explicit %v, want %+v / false",
			eff.MetadataBranch.Provenance, eff.MetadataBranch.Explicit, builtinProvenance())
	}
	if !reflect.DeepEqual(eff.Agents, builtinAgents()) {
		t.Error("builtinEffective().Agents must be the built-in agent table")
	}
}

// TestBuiltinProvenance pins the provenance every default carries.
func TestBuiltinProvenance(t *testing.T) {
	want := Provenance{Layer: LayerBuiltIn, Source: "built-in"}
	if got := builtinProvenance(); got != want {
		t.Errorf("builtinProvenance() = %+v, want %+v", got, want)
	}
}
