package config

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// This file is the end-to-end tier: it drives the real filesystem adapter over
// repository fixtures and asserts what Resolve makes of a whole repository,
// rather than what one stage makes of one node. Every other test file in this
// package builds its sources in memory; these read bytes from disk, so a change
// that only looks right against a hand-written literal is caught here.
//
// Most fixtures are the configuration samples under
// testdata/repositories/configs/. They are samples of data docket must keep
// reading: a failing test here means docket broke a configuration that already
// exists, so a sample is never edited to make a test pass (testdata/README.md).
// A test whose input must keep up with docket builds it with writeFixture.
// Tests never write inside testdata/.

// fixtureRoot is the configuration-sample folder, relative to this package's
// directory.
const fixtureRoot = "../../testdata/repositories/configs"

// fixtureAgentHarness and fixtureAgentName are the concrete names
// deferredActiveYAML uses for the dynamic `agents.<h>.<a>` subtree. The
// derivation in repoSettableDeferredPaths substitutes them for the registry's
// "*" segments, so the inline configuration and the expectation cannot drift
// apart.
const (
	fixtureAgentHarness = "claude"
	fixtureAgentName    = "adr"
)

// loadFixture resolves one fixture end to end: LoadFilesystemSources over
// <fixture>/repo (plus <fixture>/xdg/docket/config.yml when the fixture has a
// global layer), then Resolve with DefaultBranch "main".
//
// It pins XDG_CONFIG_HOME and HOME into a temp dir FIRST. The fixtures pass an
// explicit GlobalPath, but the pin has to hold even when a fixture has no
// global layer and even when an assertion below fails: the developer's real
// global configuration must be unreachable from this package's tests.
func loadFixture(t *testing.T, name string) (*Snapshot, []Diagnostic, error) {
	t.Helper()
	return loadFixtureFrom(t, fixtureRoot, name)
}

// loadFixtureFrom is loadFixture over an explicit fixture root instead of
// fixtureRoot — in practice a directory writeFixture built, for the tests whose
// input must keep up with docket (TestFixtureDocketSelf and
// TestFixtureDeferredActive). Every other fixture test uses loadFixture.
func loadFixtureFrom(t *testing.T, root, name string) (*Snapshot, []Diagnostic, error) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)

	repoDir := filepath.Join(root, name, "repo")
	// A missing fixture directory would otherwise resolve to "no layers at
	// all", which is a PASSING sparse-defaults result — a silent vacuum. Fail
	// on the directory instead.
	if info, err := os.Stat(repoDir); err != nil || !info.IsDir() {
		t.Fatalf("fixture %s: repo directory %s is missing (err=%v)", name, repoDir, err)
	}

	opts := FSOptions{RepoDir: repoDir}
	globalPath := filepath.Join(root, name, "xdg", "docket", "config.yml")
	if _, err := os.Stat(globalPath); err == nil {
		opts.GlobalPath = globalPath
	}

	sources, err := LoadFilesystemSources(opts)
	if err != nil {
		t.Fatalf("fixture %s: LoadFilesystemSources: %v", name, err)
	}
	snap, diags, resolveErr := Resolve(sources, ResolveContext{DefaultBranch: "main"})
	return snap, diags, resolveErr
}

// mustResolveFixture is loadFixture for the fixtures that must resolve cleanly.
func mustResolveFixture(t *testing.T, name string) *Snapshot {
	t.Helper()
	return mustResolveFixtureFrom(t, fixtureRoot, name)
}

// mustResolveFixtureFrom is mustResolveFixture over an explicit fixture root —
// see loadFixtureFrom.
func mustResolveFixtureFrom(t *testing.T, root, name string) *Snapshot {
	t.Helper()
	snap, diags, err := loadFixtureFrom(t, root, name)
	if err != nil {
		t.Fatalf("fixture %s: Resolve: %v\ndiagnostics: %s", name, err, formatDiags(diags))
	}
	if snap == nil {
		t.Fatalf("fixture %s: Resolve returned a nil snapshot without an error", name)
	}
	return snap
}

// writeFixture builds a fixture directory from strings in a fresh temp dir:
// <root>/<name>/repo/.docket.yml, plus <root>/<name>/xdg/docket/config.yml when
// globalYAML is non-empty. It returns root, for loadFixtureFrom and
// mustResolveFixtureFrom. A test whose input must keep up with docket builds it
// here rather than reading it from testdata/.
func writeFixture(t *testing.T, name, repoYAML, globalYAML string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, name, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("fixture %s: creating %s: %v", name, filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("fixture %s: writing %s: %v", name, p, err)
		}
	}
	write("repo/.docket.yml", repoYAML)
	if globalYAML != "" {
		write("xdg/docket/config.yml", globalYAML)
	}
	return root
}

func formatDiags(diags []Diagnostic) string {
	if len(diags) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(diags))
	for _, d := range diags {
		lines = append(lines, "  "+string(d.Severity)+" "+d.Code+" "+d.Path+": "+d.Message)
	}
	return "\n" + strings.Join(lines, "\n")
}

// diagPathSet is diagPathsWithCode (capability_test.go) as a membership set,
// for the fixtures that ask "was this path reported?" rather than "in what
// order were these paths reported?".
func diagPathSet(snap *Snapshot, code string) map[string]bool {
	out := make(map[string]bool)
	for _, path := range diagPathsWithCode(snap, code) {
		out[path] = true
	}
	return out
}

func assertSameStrings(t *testing.T, what string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d entries %v, want %d entries %v", what, len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}

// TestFixtureSparseDefaults: a repository with no configuration files at all
// resolves to the built-in layer end to end — nothing explicit, nothing
// blocking, and the `auto` integration branch answered by the context.
func TestFixtureSparseDefaults(t *testing.T) {
	snap := mustResolveFixture(t, "sparse-defaults")
	eff := snap.Effective

	for _, leaf := range []struct {
		path     string
		explicit bool
		layer    LayerKind
	}{
		{"integration_branch", eff.IntegrationBranch.Explicit, eff.IntegrationBranch.Provenance.Layer},
		{"changes_dir", eff.ChangesDir.Explicit, eff.ChangesDir.Provenance.Layer},
		{"adrs_dir", eff.ADRsDir.Explicit, eff.ADRsDir.Provenance.Layer},
		{"results_dir", eff.ResultsDir.Explicit, eff.ResultsDir.Provenance.Layer},
		{"finalize.gate", eff.Finalize.Gate.Explicit, eff.Finalize.Gate.Provenance.Layer},
		{"finalize.test_command", eff.Finalize.TestCommand.Explicit, eff.Finalize.TestCommand.Provenance.Layer},
		{"finalize.require_pr_approval", eff.Finalize.RequirePRApproval.Explicit, eff.Finalize.RequirePRApproval.Provenance.Layer},
		{"learnings.enabled", eff.Learnings.Enabled.Explicit, eff.Learnings.Enabled.Provenance.Layer},
		{"reclaim.lease_ttl", eff.Reclaim.LeaseTTL.Explicit, eff.Reclaim.LeaseTTL.Provenance.Layer},
		{"reclaim.auto", eff.Reclaim.Auto.Explicit, eff.Reclaim.Auto.Provenance.Layer},
		{"leak_check.match_word", eff.LeakCheck.MatchWord.Explicit, eff.LeakCheck.MatchWord.Provenance.Layer},
		{"open.artifacts", eff.Open.Artifacts.Explicit, eff.Open.Artifacts.Provenance.Layer},
		{"review.min_fix_severity", eff.Review.MinFixSeverity.Explicit, eff.Review.MinFixSeverity.Provenance.Layer},
		{"review.max_fix_tasks", eff.Review.MaxFixTasks.Explicit, eff.Review.MaxFixTasks.Provenance.Layer},
		{"gate_observation_budget", eff.GateObservation.Explicit, eff.GateObservation.Provenance.Layer},
		{"board_surfaces", eff.BoardSurfaces.Explicit, eff.BoardSurfaces.Provenance.Layer},
		{"change_types", eff.ChangeTypes.Explicit, eff.ChangeTypes.Provenance.Layer},
	} {
		if leaf.explicit {
			t.Errorf("%s: Explicit is true with no configuration file present", leaf.path)
		}
		if leaf.layer != LayerBuiltIn {
			t.Errorf("%s: provenance layer %q, want %q", leaf.path, leaf.layer, LayerBuiltIn)
		}
	}

	// The `auto` sentinel is answered by the resolution context, and the
	// provenance stays built-in: nothing in the repository named a branch.
	if eff.IntegrationBranch.Value != "main" {
		t.Errorf("integration_branch = %q, want %q", eff.IntegrationBranch.Value, "main")
	}
	if eff.Finalize.TestCommand.Value != "" {
		t.Errorf("finalize.test_command = %q, want the auto sentinel resolved to unset", eff.Finalize.TestCommand.Value)
	}
	if !PreflightMutation(snap).Allowed {
		t.Errorf("preflight blocked a repository with no configuration: %v", blockerPaths(snap))
	}
}

// TestFixtureExampleActivated: declaring a setting at its shipped default is
// still declaring it. Values must equal the built-ins while provenance moves
// to the repository file — that is the whole point of the Explicit flag.
func TestFixtureExampleActivated(t *testing.T) {
	snap := mustResolveFixture(t, "example-activated")
	eff := snap.Effective
	def := builtinEffective()

	check := func(path string, explicit bool, prov Provenance) {
		t.Helper()
		if !explicit {
			t.Errorf("%s: Explicit is false, but the fixture declares it", path)
		}
		if prov.Layer != LayerRepository {
			t.Errorf("%s: provenance layer %q, want %q", path, prov.Layer, LayerRepository)
		}
		if prov.Source != ".docket.yml" {
			t.Errorf("%s: provenance source %q, want .docket.yml", path, prov.Source)
		}
		if prov.Line < 1 {
			t.Errorf("%s: provenance line %d, want a real line number", path, prov.Line)
		}
	}

	// metadata_branch is an obsolete tombstone (0363): the fixture still declares
	// it, but it no longer resolves to Effective — the declaration is diagnosed
	// obsolete instead. Its removal is covered by TestMetadataBranchIsObsoleteTombstone.
	check("integration_branch", eff.IntegrationBranch.Explicit, eff.IntegrationBranch.Provenance)
	check("changes_dir", eff.ChangesDir.Explicit, eff.ChangesDir.Provenance)
	check("adrs_dir", eff.ADRsDir.Explicit, eff.ADRsDir.Provenance)
	check("results_dir", eff.ResultsDir.Explicit, eff.ResultsDir.Provenance)
	check("finalize.gate", eff.Finalize.Gate.Explicit, eff.Finalize.Gate.Provenance)
	check("finalize.test_command", eff.Finalize.TestCommand.Explicit, eff.Finalize.TestCommand.Provenance)
	check("finalize.require_pr_approval", eff.Finalize.RequirePRApproval.Explicit, eff.Finalize.RequirePRApproval.Provenance)
	check("learnings.enabled", eff.Learnings.Enabled.Explicit, eff.Learnings.Enabled.Provenance)
	check("reclaim.lease_ttl", eff.Reclaim.LeaseTTL.Explicit, eff.Reclaim.LeaseTTL.Provenance)
	check("reclaim.auto", eff.Reclaim.Auto.Explicit, eff.Reclaim.Auto.Provenance)
	check("review.min_fix_severity", eff.Review.MinFixSeverity.Explicit, eff.Review.MinFixSeverity.Provenance)
	check("review.max_fix_tasks", eff.Review.MaxFixTasks.Explicit, eff.Review.MaxFixTasks.Provenance)
	check("gate_observation_budget", eff.GateObservation.Explicit, eff.GateObservation.Provenance)
	check("board_surfaces", eff.BoardSurfaces.Explicit, eff.BoardSurfaces.Provenance)
	check("change_types", eff.ChangeTypes.Explicit, eff.ChangeTypes.Provenance)

	// Values: identical to the built-in layer, with the two `auto` sentinels
	// resolved exactly as they are for a repository that declared nothing.
	if eff.ChangesDir.Value != def.ChangesDir.Value || eff.ADRsDir.Value != def.ADRsDir.Value || eff.ResultsDir.Value != def.ResultsDir.Value {
		t.Errorf("directory settings drifted from the built-ins: %q %q %q", eff.ChangesDir.Value, eff.ADRsDir.Value, eff.ResultsDir.Value)
	}
	if eff.Finalize.Gate.Value != def.Finalize.Gate.Value || eff.Finalize.RequirePRApproval.Value != def.Finalize.RequirePRApproval.Value {
		t.Errorf("finalize drifted from the built-ins: gate=%q require_pr_approval=%v", eff.Finalize.Gate.Value, eff.Finalize.RequirePRApproval.Value)
	}
	if eff.Finalize.TestCommand.Value != "" {
		t.Errorf("finalize.test_command = %q, want an explicit `auto` resolved to unset", eff.Finalize.TestCommand.Value)
	}
	if eff.IntegrationBranch.Value != "main" {
		t.Errorf("integration_branch = %q, want the context's default branch", eff.IntegrationBranch.Value)
	}
	if !eff.Learnings.Enabled.Value || eff.Reclaim.LeaseTTL.Value != def.Reclaim.LeaseTTL.Value || eff.Reclaim.Auto.Value {
		t.Errorf("learnings/reclaim drifted from the built-ins: %+v %+v", eff.Learnings, eff.Reclaim)
	}
	if eff.Review.MinFixSeverity.Value != def.Review.MinFixSeverity.Value || eff.Review.MaxFixTasks.Value != def.Review.MaxFixTasks.Value {
		t.Errorf("review drifted from the built-ins: %+v", eff.Review)
	}
	if eff.GateObservation.Value != def.GateObservation.Value {
		t.Errorf("gate_observation_budget = %d, want the built-in %d", eff.GateObservation.Value, def.GateObservation.Value)
	}
	assertSameStrings(t, "board_surfaces", eff.BoardSurfaces.Value, def.BoardSurfaces.Value)
	assertSameStrings(t, "change_types", eff.ChangeTypes.Value, def.ChangeTypes.Value)

	if !PreflightMutation(snap).Allowed {
		t.Errorf("preflight blocked a fully supported configuration: %v", blockerPaths(snap))
	}
}

// TestFixtureFourLayerCollision: every layer declares something, and per-leaf
// precedence decides each one separately. Value AND provenance layer are
// asserted together — a resolver that took the right value from the wrong
// layer would still be wrong, because the provenance is what tells a user
// which file to edit.
func TestFixtureFourLayerCollision(t *testing.T) {
	snap := mustResolveFixture(t, "four-layer-collision")
	eff := snap.Effective

	// Scalar collision across all three file layers: the machine-local layer
	// wins. learnings.cap is inert, so it never reaches Effective — its
	// winning declaration is visible through its capability entry.
	var capEntry *Capability
	for i := range snap.Capabilities {
		if snap.Capabilities[i].Path == "learnings.cap" {
			capEntry = &snap.Capabilities[i]
		}
	}
	if capEntry == nil {
		t.Fatalf("learnings.cap has no capability entry; capabilities: %+v", snap.Capabilities)
	}
	if capEntry.Provenance.Layer != LayerRepositoryLocal {
		t.Errorf("learnings.cap provenance layer %q, want %q (250 in .docket.local.yml beats 200 and 100)",
			capEntry.Provenance.Layer, LayerRepositoryLocal)
	}
	if capEntry.Provenance.Source != ".docket.local.yml" {
		t.Errorf("learnings.cap provenance source %q, want .docket.local.yml", capEntry.Provenance.Source)
	}

	// Nested scalar collision the machine-local layer does NOT enter: the
	// repository layer beats the global one and keeps the leaf.
	if eff.Reclaim.LeaseTTL.Value != 200 {
		t.Errorf("reclaim.lease_ttl = %d, want 200", eff.Reclaim.LeaseTTL.Value)
	}
	if eff.Reclaim.LeaseTTL.Provenance.Layer != LayerRepository {
		t.Errorf("reclaim.lease_ttl provenance layer %q, want %q", eff.Reclaim.LeaseTTL.Provenance.Layer, LayerRepository)
	}

	// List collision: lists replace whole, so the repository's list wins
	// outright rather than merging with the global one.
	assertSameStrings(t, "change_types", eff.ChangeTypes.Value,
		[]string{"chore", "docs", "feat", "fix", "refactor", "perf", "spike"})
	if eff.ChangeTypes.Provenance.Layer != LayerRepository {
		t.Errorf("change_types provenance layer %q, want %q", eff.ChangeTypes.Provenance.Layer, LayerRepository)
	}

	// A leaf only the machine-local layer declares.
	if eff.GateObservation.Value != 45 || eff.GateObservation.Provenance.Layer != LayerRepositoryLocal {
		t.Errorf("gate_observation_budget = %d from %q, want 45 from %q",
			eff.GateObservation.Value, eff.GateObservation.Provenance.Layer, LayerRepositoryLocal)
	}

	// Agent collision: the global layer's `agents.default` pin falls back into
	// every SHIPPED harness row, including harnesses the built-in table pins
	// itself. `default` is a write-side alias, not a harness — the effective
	// table carries the harnesses docket actually runs, which is why this
	// iterates the resolved table rather than the registry's name set.
	if len(eff.Agents) == 0 {
		t.Fatalf("agents table is empty")
	}
	for harness, row := range eff.Agents {
		if harness == "default" {
			t.Errorf("agents table carries a %q harness row; default is an override alias, not a harness", harness)
		}
		setting := row[fixtureAgentName]
		if setting.Model.Value != "global-pinned-model" {
			t.Errorf("agents.%s.%s.model = %q, want the global default pin", harness, fixtureAgentName, setting.Model.Value)
		}
		if !setting.Model.Explicit || setting.Model.Provenance.Layer != LayerGlobal {
			t.Errorf("agents.%s.%s.model provenance %+v (explicit=%v), want the global layer",
				harness, fixtureAgentName, setting.Model.Provenance, setting.Model.Explicit)
		}
		// Effort was not pinned, so it keeps its shipped value.
		if setting.Effort.Explicit {
			t.Errorf("agents.%s.%s.effort is explicit, but no layer pinned it", harness, fixtureAgentName)
		}
	}

	if !PreflightMutation(snap).Allowed {
		t.Errorf("preflight blocked a valid four-layer configuration: %v", blockerPaths(snap))
	}
}

// TestFixtureModeMainCustomPaths: a main-mode repository that relocates every
// docket directory and names its integration branch outright.
func TestFixtureModeMainCustomPaths(t *testing.T) {
	snap := mustResolveFixture(t, "mode-main-custom-paths")
	eff := snap.Effective

	for _, tc := range []struct {
		path       string
		got, want  string
		provenance Provenance
	}{
		// metadata_branch is an obsolete tombstone (0363): the sample still
		// carries `metadata_branch: main`, but it no longer resolves — the
		// declaration is diagnosed obsolete and the custom paths below are what
		// this fixture now exercises.
		{"integration_branch", eff.IntegrationBranch.Value, "develop", eff.IntegrationBranch.Provenance},
		{"changes_dir", eff.ChangesDir.Value, "planning/changes", eff.ChangesDir.Provenance},
		{"adrs_dir", eff.ADRsDir.Value, "planning/adrs", eff.ADRsDir.Provenance},
		{"results_dir", eff.ResultsDir.Value, "planning/results", eff.ResultsDir.Provenance},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.path, tc.got, tc.want)
		}
		if tc.provenance.Layer != LayerRepository {
			t.Errorf("%s provenance layer %q, want %q", tc.path, tc.provenance.Layer, LayerRepository)
		}
	}

	// An explicit integration branch must not depend on the context at all:
	// resolving with no default branch supplied still succeeds.
	sources, err := LoadFilesystemSources(FSOptions{RepoDir: filepath.Join(fixtureRoot, "mode-main-custom-paths", "repo")})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	noCtx, _, err := Resolve(sources, ResolveContext{})
	if err != nil {
		t.Fatalf("Resolve with no default branch: %v, want success (integration_branch is explicit)", err)
	}
	if noCtx.Effective.IntegrationBranch.Value != "develop" {
		t.Errorf("integration_branch without a context = %q, want %q", noCtx.Effective.IntegrationBranch.Value, "develop")
	}
}

// TestFixtureModeDocket: docket-mode with `integration_branch: auto` — the
// value comes from the context, the provenance stays on the declaring file.
func TestFixtureModeDocket(t *testing.T) {
	snap := mustResolveFixture(t, "mode-docket")
	eff := snap.Effective

	// metadata_branch is an obsolete tombstone (0363): the fixture declares
	// `metadata_branch: docket`, which no longer resolves; the surviving subject
	// of this fixture is `integration_branch: auto` answered by the context.
	if eff.IntegrationBranch.Value != "main" {
		t.Errorf("integration_branch = %q, want the context's default branch", eff.IntegrationBranch.Value)
	}
	if eff.IntegrationBranch.Provenance.Layer != LayerRepository || !eff.IntegrationBranch.Explicit {
		t.Errorf("integration_branch provenance %+v (explicit=%v), want the declaring repository file",
			eff.IntegrationBranch.Provenance, eff.IntegrationBranch.Explicit)
	}

	// The same fixture with no default branch supplied is the missing-context
	// error, not a silent "auto" leaking into effective policy.
	sources, err := LoadFilesystemSources(FSOptions{RepoDir: filepath.Join(fixtureRoot, "mode-docket", "repo")})
	if err != nil {
		t.Fatalf("LoadFilesystemSources: %v", err)
	}
	if _, _, err := Resolve(sources, ResolveContext{}); !errors.Is(err, ErrMissingResolutionContext) {
		t.Errorf("Resolve with auto and no default branch: %v, want ErrMissingResolutionContext", err)
	}
}

// TestFixtureGuardedMachineKeys: every repo-only setting declared from a
// machine layer is warned about and excluded — and none of it blocks. A guard
// is a coordination rule, not a capability question.
func TestFixtureGuardedMachineKeys(t *testing.T) {
	snap := mustResolveFixture(t, "fenced-machine-keys")
	eff := snap.Effective
	def := builtinEffective()

	// metadata_branch and finalize.skip_results_only_delta are no longer
	// guarded — they are obsolete tombstones, excluded at decode rather than
	// guarded, so a machine-layer declaration produces an obsolete-setting
	// warning, not a shared-setting-ignored one.
	guarded := diagPathSet(snap, CodeSharedSettingIgnored)
	for _, path := range []string{
		"integration_branch", "changes_dir", "adrs_dir", "results_dir",
		"github_project", "terminal_publish", "board_surfaces",
	} {
		if !guarded[path] {
			t.Errorf("%s is declared in a machine layer but produced no shared-setting-ignored warning", path)
		}
	}
	for _, d := range snap.Diagnostics {
		if d.Code == CodeSharedSettingIgnored && d.Severity != SeverityWarning {
			t.Errorf("%s: shared-setting-ignored severity %q, want warning", d.Path, d.Severity)
		}
	}

	// Guarded scalars never become effective: each keeps its built-in value and
	// built-in provenance, exactly as if the machine layers were absent.
	for _, tc := range []struct {
		path      string
		got, want string
		layer     LayerKind
	}{
		{"integration_branch", eff.IntegrationBranch.Value, "main", eff.IntegrationBranch.Provenance.Layer},
		{"changes_dir", eff.ChangesDir.Value, def.ChangesDir.Value, eff.ChangesDir.Provenance.Layer},
		{"adrs_dir", eff.ADRsDir.Value, def.ADRsDir.Value, eff.ADRsDir.Provenance.Layer},
		{"results_dir", eff.ResultsDir.Value, def.ResultsDir.Value, eff.ResultsDir.Provenance.Layer},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want the built-in %q (the machine declaration is guarded)", tc.path, tc.got, tc.want)
		}
		if tc.layer != LayerBuiltIn {
			t.Errorf("%s provenance layer %q, want %q", tc.path, tc.layer, LayerBuiltIn)
		}
	}

	// board_surfaces carries its guard on the TOKEN: `github` is stripped with
	// a warning and the rest of the machine layer's list still competes for
	// the leaf, which is why this one leaf is explicit while the others are not.
	assertSameStrings(t, "board_surfaces", eff.BoardSurfaces.Value, []string{"inline"})
	if eff.BoardSurfaces.Provenance.Layer != LayerRepositoryLocal {
		t.Errorf("board_surfaces provenance layer %q, want %q (the surviving tokens are honored)",
			eff.BoardSurfaces.Provenance.Layer, LayerRepositoryLocal)
	}

	// The point of the fixture: guards never block a mutation.
	if decision := PreflightMutation(snap); !decision.Allowed {
		t.Errorf("preflight blocked on guarded declarations alone: %v", blockerPaths(snap))
	}
}

// docketSelfGlobalYAML is the machine-global layer of the docket-self fixture:
// one deferred capability requested from outside the repository, so the
// blocker envelope spans layers.
const docketSelfGlobalYAML = `auto_capture:
  enabled: true
`

// docketSelfRederive is the remedy every TestFixtureDocketSelf failure carries.
const docketSelfRederive = "docket's own .docket.yml changed what this test checks: " +
	"re-derive these expectations by running the resolver against it and reading its actual output."

// TestFixtureDocketSelf: this repository's own committed .docket.yml, read where
// it lives, plus a global layer requesting auto-capture — the blocker envelope
// docket runs under. Only that one file is copied into the fixture, never the
// checkout root, so a developer's .docket.local.yml cannot reach this test.
func TestFixtureDocketSelf(t *testing.T) {
	self, err := os.ReadFile(filepath.FromSlash("../../.docket.yml"))
	if err != nil {
		t.Fatalf("reading docket's own .docket.yml: %v", err)
	}
	root := writeFixture(t, "docket-self", string(self), docketSelfGlobalYAML)
	snap := mustResolveFixtureFrom(t, root, "docket-self")

	// docket's own gate is the Go-native whole-suite runner, entered from source
	// so the gate tests the exact checkout under review.
	if got, want := snap.Effective.Finalize.TestCommand.Value, "go run ./cmd/docket development test"; got != want {
		t.Errorf("finalize.test_command = %q, want %q.\n%s", got, want, docketSelfRederive)
	}

	// The committed file requests no deferred capability; the machine-global
	// auto-capture request is the sole blocker.
	decision := PreflightMutation(snap)
	if decision.Allowed {
		t.Fatalf("preflight allowed a configuration still requesting global auto-capture.\n%s", docketSelfRederive)
	}
	assertSameStrings(t, "docket-self blockers ("+docketSelfRederive+")", blockerPaths(snap), []string{
		"auto_capture.enabled",
	})

	// The lone blocker comes from the machine-global layer; asserting the layer
	// keeps the test honest about which file the request lives in.
	wantLayer := map[string]LayerKind{
		"auto_capture.enabled": LayerGlobal,
	}
	for _, b := range decision.Blockers {
		if b.Provenance == nil {
			t.Errorf("blocker %s carries no provenance.\n%s", b.Path, docketSelfRederive)
			continue
		}
		if got := b.Provenance.Layer; got != wantLayer[b.Path] {
			t.Errorf("blocker %s came from layer %q, want %q.\n%s", b.Path, got, wantLayer[b.Path], docketSelfRederive)
		}
		if b.Severity != SeverityError {
			t.Errorf("blocker %s severity %q, want error", b.Path, b.Severity)
		}
	}

	// A blocked configuration is still a VALID one: mustResolve already
	// required a nil error, and the guard is what refuses the mutation.
	if err := GuardMutation(snap, func() error { return nil }); !errors.Is(err, ErrUnsupportedConfig) {
		t.Errorf("GuardMutation = %v, want ErrUnsupportedConfig", err)
	}
}

// TestFixtureDeferredPairs: every deferred capability that has an inactive
// spelling, declared inactive. Zero blockers, one informational notice each —
// declining a capability is a complete, valid answer.
func TestFixtureDeferredPairs(t *testing.T) {
	snap := mustResolveFixture(t, "deferred-pairs")

	if decision := PreflightMutation(snap); !decision.Allowed {
		t.Fatalf("preflight blocked on explicitly inactive deferred settings: %v", blockerPaths(snap))
	}

	// Derived, not hand-listed: every boolean-shaped deferred row a repository
	// may declare must produce its notice, so a new row cannot be added to the
	// registry and silently go unreported here.
	notices := diagPathSet(snap, CodeDeferredSetting)
	for _, spec := range registry() {
		// The scopeLocalOnly row is registry-present but decode-excluded, so a
		// repository can never declare it and it can never produce a notice.
		if spec.disp != dispDeferred || spec.scope == scopeLocalOnly {
			continue
		}
		if !notices[spec.path] {
			t.Errorf("%s is declared inactive but produced no deferred-setting notice", spec.path)
		}
	}

	// The standing learnings notice rides on the EFFECTIVE value, so it is
	// present even though this fixture never mentions learnings.
	if !notices["learnings.enabled"] {
		t.Errorf("no deferred-setting notice for learnings.enabled; diagnostics: %s", formatDiags(snap.Diagnostics))
	}
	for _, d := range snap.Diagnostics {
		if d.Code == CodeDeferredSetting && d.Severity != SeverityInfo {
			t.Errorf("%s: deferred-setting severity %q, want info", d.Path, d.Severity)
		}
	}
}

// deferredActiveYAML requests every repository-settable deferred capability at
// once: the complete blocker set in one document. TestFixtureDeferredActive
// derives the expected set from the schema registry rather than from this text,
// so a deferred row added to the registry without a request here fails, naming
// the path to add. The agents block uses fixtureAgentHarness/fixtureAgentName.
const deferredActiveYAML = `finalize:
  gate: ci
  skip_results_only_delta: true

build:
  checkpoint: true

terminal_publish: true
auto_groom: true

auto_capture:
  enabled: true

dummy_mode:
  enabled: true

board_surfaces: [inline, github]

skills:
  brainstorm: docket-brainstorm
  plan: docket-plan
  build: docket-build
  review: docket-review
  finish: docket-finish

agents:
  claude:
    adr:
      model: repo-pinned-model
      effort: high
      cache_ttl: 1h
      runner: codex
`

// TestFixtureDeferredActive: every repo-settable deferred capability requested
// at once, from deferredActiveYAML. The expected blocker set is DERIVED from the
// registry rather than transcribed from the configuration, so adding a deferred
// row without requesting it in deferredActiveYAML fails here, naming the path.
func TestFixtureDeferredActive(t *testing.T) {
	root := writeFixture(t, "deferred-active", deferredActiveYAML, "")
	snap := mustResolveFixtureFrom(t, root, "deferred-active")

	decision := PreflightMutation(snap)
	if decision.Allowed {
		t.Fatalf("preflight allowed a configuration requesting every deferred capability")
	}

	got := blockerPaths(snap)
	want := repoSettableDeferredPaths(t)
	requested := make(map[string]bool, len(got))
	for _, path := range got {
		requested[path] = true
	}
	settable := make(map[string]bool, len(want))
	for _, path := range want {
		settable[path] = true
		if !requested[path] {
			t.Errorf("%s is a repository-settable deferred capability that deferredActiveYAML does not request: "+
				"add it to deferredActiveYAML in internal/config/fixtures_test.go", path)
		}
	}
	for _, path := range got {
		if !settable[path] {
			t.Errorf("deferredActiveYAML requests %s, which blocks but is not a repository-settable deferred registry row", path)
		}
	}
	assertSameStrings(t, "deferred-active blockers", got, want)

	// Each blocker names the file and offers a way out; a blocker a user
	// cannot act on is a dead end.
	for _, b := range decision.Blockers {
		if b.Remedy == "" {
			t.Errorf("blocker %s has no remedy", b.Path)
		}
		if b.Provenance == nil || b.Provenance.Line < 1 {
			t.Errorf("blocker %s has no positioned provenance: %+v", b.Path, b.Provenance)
		}
	}

	// Every blocker has a matching capability entry marked active and
	// mutation-blocking: the two views of the same verdict must agree.
	caps := make(map[string]Capability, len(snap.Capabilities))
	for _, c := range snap.Capabilities {
		caps[c.Path] = c
	}
	for _, path := range blockerPaths(snap) {
		c, ok := caps[path]
		if !ok {
			t.Errorf("blocker %s has no capability entry", path)
			continue
		}
		if !c.Active || !c.MutationBlock || c.Classification != Deferred {
			t.Errorf("capability %s = %+v, want an active, mutation-blocking deferred entry", path, c)
		}
	}
}

// repoSettableDeferredPaths derives the complete set of paths that block a
// mutation when a repository layer requests them: every registry row whose
// disposition can turn into a blocker, minus the rows a repository may not
// declare at all. Dynamic rows are concretized with the fixture's chosen
// harness and agent — and a dynamic row this substitution does not cover is a
// hard failure rather than a silently dropped expectation.
func repoSettableDeferredPaths(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, spec := range registry() {
		switch spec.disp {
		case dispDeferred, dispDeferredByValue, dispDeferredActive, dispAgentsLeaf, dispSupportedOrDropped:
		default:
			continue
		}
		if spec.scope == scopeLocalOnly {
			// Registry-present but decode-excluded: a repository layer cannot
			// declare it, so it cannot block from one.
			continue
		}
		path := strings.Replace(spec.path, "agents.*.*.", "agents."+fixtureAgentHarness+"."+fixtureAgentName+".", 1)
		if strings.Contains(path, "*") {
			t.Fatalf("registry row %q is a blocking dynamic row with no concrete name in this test: "+
				"add a request for it to deferredActiveYAML in internal/config/fixtures_test.go and extend this substitution", spec.path)
		}
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// TestFixtureInvalid drives the malformed corner of the tree. Each fixture is
// invalid for exactly one reason, and the assertion is on the CODE: a resolver
// that rejected the right file for the wrong reason would tell the user to fix
// the wrong thing.
func TestFixtureInvalid(t *testing.T) {
	cases := []struct {
		fixture string
		code    string
	}{
		{"invalid/malformed", CodeInvalidYAML},
		{"invalid/duplicate-key", CodeDuplicateKey},
		{"invalid/alias-merge", CodeInvalidYAML},
		{"invalid/multi-doc", CodeInvalidYAML},
		{"invalid/wrong-type", CodeInvalidType},
		{"invalid/bad-enum", CodeInvalidValue},
		{"invalid/scalar-auto-capture", CodeInvalidValue},
		{"invalid/unknown-key", CodeUnknownKey},
		{"invalid/model-typo", CodeUnknownKey},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			snap, diags, err := loadFixture(t, tc.fixture)
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("Resolve = %v, want ErrInvalidConfig; diagnostics: %s", err, formatDiags(diags))
			}
			if snap != nil {
				t.Errorf("an invalid configuration returned a snapshot; it must return none")
			}
			found := false
			for _, d := range diags {
				if d.Code != tc.code {
					continue
				}
				found = true
				if d.Severity != SeverityError {
					t.Errorf("%s severity %q, want error", tc.code, d.Severity)
				}
				if d.Provenance == nil || d.Provenance.Line < 1 {
					t.Errorf("%s carries no positioned provenance: %+v", tc.code, d.Provenance)
				}
			}
			if !found {
				t.Errorf("no %s diagnostic; got: %s", tc.code, formatDiags(diags))
			}
		})
	}
}
