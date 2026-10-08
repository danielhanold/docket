package config

import (
	"sort"
	"testing"
)

// TestSettingPathsSupportSplit pins the registry's support verdict to the
// supported key list the documentation promises. The expected supported set is
// written out because it is the contract docs and .docket.example.yml are held
// to; the unsupported side is checked by sampling every disposition family.
// TestRepoOnlyPaths pins the repository-only setting list, in registry order:
// the leaves a machine layer may not declare and a visibility switch never
// carries out of the local file.
func TestRepoOnlyPaths(t *testing.T) {
	want := []string{"integration_branch", "changes_dir", "adrs_dir", "results_dir", "github_project", "terminal_publish"}
	got := RepoOnlyPaths()
	if len(got) != len(want) {
		t.Fatalf("RepoOnlyPaths() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RepoOnlyPaths() = %v, want %v", got, want)
		}
	}
}

func TestSettingPathsSupportSplit(t *testing.T) {
	paths := SettingPaths()
	if len(paths) != len(registry()) {
		t.Fatalf("SettingPaths returned %d rows, registry has %d", len(paths), len(registry()))
	}
	var supported []string
	unsupported := map[string]bool{}
	for _, p := range paths {
		if p.Supported {
			supported = append(supported, p.Path)
		} else {
			unsupported[p.Path] = true
		}
	}
	want := []string{
		"visibility", "integration_branch", "changes_dir", "adrs_dir", "results_dir",
		"finalize.gate", "finalize.test_command", "finalize.require_pr_approval",
		"finalize.resolver_max_attempts", "finalize.repair_max_attempts",
		"build.gate", "build.test_command", "build.max_attempts",
		"run.max_attempts",
		"review.min_fix_severity", "review.max_fix_tasks",
		"reclaim.lease_ttl", "reclaim.auto",
		"leak_check.match_word",
		"open.artifacts",
		"learnings.enabled",
		"gate_observation_budget",
		"board_surfaces", "board.section_order",
		"change_types", "agent_harnesses",
		"agents.*.*.model", "agents.*.*.effort",
	}
	for _, s := range BoardSectionTokens {
		want = append(want, "board.sorting."+s+".by", "board.sorting."+s+".direction")
	}
	sort.Strings(supported)
	sort.Strings(want)
	if len(supported) != len(want) {
		t.Fatalf("supported paths:\n got %v\nwant %v", supported, want)
	}
	for i := range want {
		if supported[i] != want[i] {
			t.Fatalf("supported paths:\n got %v\nwant %v", supported, want)
		}
	}
	// One sample per unsupported disposition family.
	for _, p := range []string{
		"metadata_branch", "runtime.bash", "finalize.skip_results_only_delta", // obsolete
		"learnings.cap", "delegation_observation_budget", "github_project", // inert
		"build.checkpoint", "terminal_publish", "auto_groom", // deferred
		"auto_capture.types", "dummy_mode.persona", "runners.*.shim_model", // inert companion
		"skills.build", "agents.*.*.runner", // deferred-active
	} {
		if !unsupported[p] {
			t.Errorf("%s should be unsupported", p)
		}
	}
}
