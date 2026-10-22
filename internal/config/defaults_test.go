package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// shippedDefaultsPath is the agent defaults file docket ships, read where it
// lives. The parity test below compares it with the Go built-in table, so an
// edit to it fails here only when it changes what the table must hold.
const shippedDefaultsPath = "../../agents/harness-defaults.yml"

// shippedDefaultsDoc is the shipped-defaults file's shape: harness → agent
// short name → model/effort pair, plus the optional prompt-cache TTL.
type shippedDefaultsDoc struct {
	Agents map[string]map[string]struct {
		Model    string `yaml:"model"`
		Effort   string `yaml:"effort"`
		CacheTTL string `yaml:"cache_ttl"`
	} `yaml:"agents"`
}

// pair is a (model, effort, cache TTL) tuple compared structurally, so a
// missing or extra harness/agent shows up as a difference rather than being
// skipped.
type pair struct{ Model, Effort, CacheTTL string }

// TestBuiltinAgentsParityWithShippedDefaults compares agents/harness-defaults.yml
// against builtinAgents() as WHOLE structures, both directions in one DeepEqual:
// every (harness, agent) pair present in both, models equal, efforts equal, and
// cache TTLs equal — where the file says `auto` the Go table must hold "".
// Vendor-ID VALIDITY is deliberately not asserted: that oracle lives outside
// the repo.
func TestBuiltinAgentsParityWithShippedDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.FromSlash(shippedDefaultsPath))
	if err != nil {
		t.Fatalf("reading %s: %v", shippedDefaultsPath, err)
	}
	var doc shippedDefaultsDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parsing %s: %v", shippedDefaultsPath, err)
	}
	if len(doc.Agents) == 0 {
		t.Fatalf("%s declares no agents", shippedDefaultsPath)
	}

	want := make(map[string]map[string]pair, len(doc.Agents))
	for harness, agents := range doc.Agents {
		row := make(map[string]pair, len(agents))
		for name, entry := range agents {
			effort := entry.Effort
			if effort == "auto" {
				effort = ""
			}
			row[name] = pair{Model: entry.Model, Effort: effort, CacheTTL: entry.CacheTTL}
		}
		want[harness] = row
	}

	got := make(map[string]map[string]pair, len(builtinAgents()))
	for harness, agents := range builtinAgents() {
		row := make(map[string]pair, len(agents))
		for name, entry := range agents {
			row[name] = pair{Model: entry.Model.Value, Effort: entry.Effort.Value, CacheTTL: entry.CacheTTL.Value}
		}
		got[harness] = row
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("the built-in agent table differs from agents/harness-defaults.yml: "+
			"agents/harness-defaults.yml and the built-in table in internal/config/defaults.go must change together.\n"+
			" got: %v\nwant: %v", got, want)
	}
}

// TestBuiltinAgentsCacheTTLShippedOnImplementNextOnly pins the shipped
// prompt-cache TTL scope: claude's implement-next carries 1h, and no other
// harness or agent carries any TTL. The 1-hour write costs more than the
// 5-minute one, so a TTL on an agent that rarely outlasts five minutes between
// turns is a pure surcharge.
func TestBuiltinAgentsCacheTTLShippedOnImplementNextOnly(t *testing.T) {
	for harness, agents := range builtinAgents() {
		for name, entry := range agents {
			want := ""
			if harness == "claude" && name == "implement-next" {
				want = "1h"
			}
			if entry.CacheTTL.Value != want {
				t.Errorf("%s/%s: built-in cache_ttl = %q, want %q", harness, name, entry.CacheTTL.Value, want)
			}
		}
	}
}

// TestBuiltinAgentsShape pins the table's structure independently of the
// shipped defaults file: exactly the four shipped harnesses, each carrying exactly the 17
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
			if entry.Model.Provenance != builtinProvenance() || entry.Effort.Provenance != builtinProvenance() ||
				entry.CacheTTL.Provenance != builtinProvenance() {
				t.Errorf("%s/%s: provenance = %+v/%+v/%+v, want %+v",
					harness, name, entry.Model.Provenance, entry.Effort.Provenance, entry.CacheTTL.Provenance, builtinProvenance())
			}
			if entry.Model.Explicit || entry.Effort.Explicit || entry.CacheTTL.Explicit {
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
		// metadata_branch is gone from Effective (obsolete tombstone, 0363).
		"visibility":                     eff.Visibility.Value,
		"integration_branch":             eff.IntegrationBranch.Value,
		"changes_dir":                    eff.ChangesDir.Value,
		"adrs_dir":                       eff.ADRsDir.Value,
		"results_dir":                    eff.ResultsDir.Value,
		"finalize.gate":                  eff.Finalize.Gate.Value,
		"finalize.test_command":          eff.Finalize.TestCommand.Value,
		"finalize.require_pr_approval":   eff.Finalize.RequirePRApproval.Value,
		"finalize.resolver_max_attempts": eff.Finalize.ResolverMaxAttempts.Value,
		"finalize.repair_max_attempts":   eff.Finalize.RepairMaxAttempts.Value,
		"build.gate":                     eff.Build.Gate.Value,
		"build.test_command":             eff.Build.TestCommand.Value,
		"build.max_attempts":             eff.Build.MaxAttempts.Value,
		"run.max_attempts":               eff.Run.MaxAttempts.Value,
		"run.max_consecutive_halts":      eff.Run.MaxConsecutiveHalts.Value,
		"learnings.enabled":              eff.Learnings.Enabled.Value,
		"leak_check.match_word":          eff.LeakCheck.MatchWord.Value,
		"open.artifacts":                 eff.Open.Artifacts.Value,
		"review.min_fix_severity":        eff.Review.MinFixSeverity.Value,
		"review.max_fix_tasks":           eff.Review.MaxFixTasks.Value,
		"gate_observation_budget":        eff.GateObservation.Value,
		"board_surfaces":                 eff.BoardSurfaces.Value,
		"change_types":                   eff.ChangeTypes.Value,
	}
	// The board presentation block mirrors the registry the same way: the
	// section-order list and every per-section sort leaf, keyed and walked by
	// BoardSectionTokens so a new section cannot drift the two apart.
	leaves["board.section_order"] = eff.Board.SectionOrder.Value
	for _, s := range BoardSectionTokens {
		leaves["board.sorting."+s+".by"] = eff.Board.Sorting[s].By.Value
		leaves["board.sorting."+s+".direction"] = eff.Board.Sorting[s].Direction.Value
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
	// metadata_branch no longer has a built-in leaf (obsolete tombstone, 0363);
	// integration_branch above is the surviving built-in provenance witness.
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
