package config

import (
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestRegistryPathSetMatchesV092 pins the COMPLETE registry path sequence: the
// concrete v0.9.2 leaves plus the dynamic agents/runners patterns. The compare
// is a single whole-sequence equality, which makes it two-way — dropping a row
// and adding an unlisted one both redden it (a one-way "every want is present"
// loop would miss the addition).
func TestRegistryPathSetMatchesV092(t *testing.T) {
	want := []string{
		"runtime.bash", "metadata_branch", "integration_branch",
		"changes_dir", "adrs_dir", "results_dir",
		"finalize.gate", "finalize.test_command", "finalize.require_pr_approval",
		"finalize.skip_results_only_delta",
		"learnings.enabled", "learnings.cap",
		"reclaim.lease_ttl", "reclaim.auto",
		"build.checkpoint",
		"review.min_fix_severity", "review.max_fix_tasks",
		"gate_observation_budget", "delegation_observation_budget",
		"board_surfaces", "github_project", "terminal_publish", "auto_groom",
		"change_types", "auto_capture.enabled", "auto_capture.types",
		"dummy_mode.enabled", "dummy_mode.persona", "dummy_mode.surfaces",
		"agent_harnesses",
		"skills.brainstorm", "skills.plan", "skills.build", "skills.review", "skills.finish",
		"agents.*.*.model", "agents.*.*.effort", "agents.*.*.runner",
		"runners.codex.sandbox", "runners.codex.network",
		"runners.opencode.permissions",
		"runners.*.shim_model", "runners.*.shim_effort",
	}
	var got []string
	for _, spec := range registry() {
		got = append(got, spec.path)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("registry() paths mismatch\n got: %v\nwant: %v", got, want)
	}
}

func TestRegistryPathsUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, spec := range registry() {
		if seen[spec.path] {
			t.Errorf("registry() declares path %q more than once", spec.path)
		}
		seen[spec.path] = true
	}
}

func TestRegistryEveryRowHasValidator(t *testing.T) {
	for _, spec := range registry() {
		if spec.validate == nil {
			t.Errorf("registry row %q has no validator", spec.path)
		}
	}
}

// TestRegistryFencedSet pins both fence directions exactly: the eight
// coordination-fenced (repo-owned) paths and the single machine-only path.
// Both compares are whole-set, so a row silently gaining or losing a fence
// reddens this test.
func TestRegistryFencedSet(t *testing.T) {
	wantRepoFenced := []string{
		"integration_branch", "changes_dir", "adrs_dir",
		"results_dir", "finalize.skip_results_only_delta", "github_project",
		"terminal_publish",
	}
	wantLocalOnly := []string{"runtime.bash"}

	var repoFenced, localOnly []string
	for _, spec := range registry() {
		switch spec.scope {
		case scopeRepoFenced:
			repoFenced = append(repoFenced, spec.path)
		case scopeLocalOnly:
			localOnly = append(localOnly, spec.path)
		}
	}
	if !reflect.DeepEqual(repoFenced, wantRepoFenced) {
		t.Errorf("scopeRepoFenced set mismatch\n got: %v\nwant: %v", repoFenced, wantRepoFenced)
	}
	if !reflect.DeepEqual(localOnly, wantLocalOnly) {
		t.Errorf("scopeLocalOnly set mismatch\n got: %v\nwant: %v", localOnly, wantLocalOnly)
	}
}

// TestRegistryDefaults compares every row's built-in default against the
// v0.9.2 matrix, both directions: a listed path must carry that default, and
// an unlisted one must carry none. The agents/runners rows have no scalar
// default here — the 17x4 agent table is defaults.go's (Reference C).
func TestRegistryDefaults(t *testing.T) {
	want := map[string]any{
		// metadata_branch has no default: it is an obsolete tombstone (change 0363).
		"integration_branch":               "auto",
		"changes_dir":                      "docs/changes",
		"adrs_dir":                         "docs/adrs",
		"results_dir":                      "docs/results",
		"finalize.gate":                    "local",
		"finalize.test_command":            "auto",
		"finalize.require_pr_approval":     false,
		"finalize.skip_results_only_delta": false,
		"learnings.enabled":                true,
		"learnings.cap":                    300,
		"reclaim.lease_ttl":                72,
		"reclaim.auto":                     false,
		"build.checkpoint":                 false,
		"review.min_fix_severity":          "minor",
		"review.max_fix_tasks":             10,
		"gate_observation_budget":          30,
		"delegation_observation_budget":    60,
		"board_surfaces":                   []string{"inline"},
		"github_project":                   "auto",
		"terminal_publish":                 false,
		"auto_groom":                       false,
		"change_types":                     []string{"chore", "docs", "feat", "fix", "refactor", "perf"},
		"auto_capture.enabled":             false,
		"auto_capture.types":               "all",
		"dummy_mode.enabled":               false,
		"dummy_mode.persona":               "",
		"dummy_mode.surfaces":              "all",
	}
	seen := make(map[string]bool, len(want))
	for _, spec := range registry() {
		expected, listed := want[spec.path]
		if !listed {
			if spec.def != nil {
				t.Errorf("registry row %q has default %#v, want none", spec.path, spec.def)
			}
			continue
		}
		seen[spec.path] = true
		if !reflect.DeepEqual(spec.def, expected) {
			t.Errorf("registry row %q default = %#v, want %#v", spec.path, spec.def, expected)
		}
	}
	for path := range want {
		if !seen[path] {
			t.Errorf("registry() has no row for defaulted path %q", path)
		}
	}
}

// TestRegistryEnumRows pins which rows are enum-constrained and their exact
// member lists — the enum members are policy, not an implementation detail.
func TestRegistryEnumRows(t *testing.T) {
	want := map[string][]string{
		// metadata_branch carries no enum now: it is an obsolete tombstone (0363).
		"finalize.gate":                {"local", "ci", "both", "off"},
		"review.min_fix_severity":      {"minor", "important", "blocker"},
		"runners.codex.sandbox":        {"workspace-write", "danger-full-access"},
		"runners.opencode.permissions": {"ask", "auto-approve"},
	}
	seen := make(map[string]bool, len(want))
	for _, spec := range registry() {
		expected, listed := want[spec.path]
		if !listed {
			if spec.enum != nil {
				t.Errorf("registry row %q carries enum %v, want none", spec.path, spec.enum)
			}
			continue
		}
		seen[spec.path] = true
		if !reflect.DeepEqual(spec.enum, expected) {
			t.Errorf("registry row %q enum = %v, want %v", spec.path, spec.enum, expected)
		}
	}
	for path := range want {
		if !seen[path] {
			t.Errorf("registry() has no row for enum path %q", path)
		}
	}
}

func TestNameSets(t *testing.T) {
	wantHarnesses := []string{"default", "claude", "codex", "cursor", "opencode"}
	if !reflect.DeepEqual(agentHarnesses, wantHarnesses) {
		t.Errorf("agentHarnesses = %v, want %v", agentHarnesses, wantHarnesses)
	}
	wantAgents := []string{
		"adr", "auto-groom", "auto-groom-critic", "brainstorm-consultant",
		"build-economy", "build-standard", "build-premium", "build-max",
		"finalize-change", "implement-next", "integration-repair",
		"plan-writer", "rebase-resolver", "review-lean", "review-standard",
		"review-deep", "status",
	}
	if !reflect.DeepEqual(agentShortNames, wantAgents) {
		t.Errorf("agentShortNames = %v, want %v", agentShortNames, wantAgents)
	}
	if len(agentShortNames) != 17 {
		t.Errorf("agentShortNames has %d entries, want 17", len(agentShortNames))
	}
	wantRunners := []string{"codex", "cursor", "opencode"}
	if !reflect.DeepEqual(runnerNames, wantRunners) {
		t.Errorf("runnerNames = %v, want %v", runnerNames, wantRunners)
	}
	wantRoles := []string{"brainstorm", "plan", "build", "review", "finish"}
	if !reflect.DeepEqual(skillRoles, wantRoles) {
		t.Errorf("skillRoles = %v, want %v", skillRoles, wantRoles)
	}
}

// specFor looks a concrete registry row up by path for the validator tests.
func specFor(t *testing.T, path string) pathSpec {
	t.Helper()
	for _, spec := range registry() {
		if spec.path == path {
			return spec
		}
	}
	t.Fatalf("registry() has no row for %q", path)
	return pathSpec{}
}

// valueNode parses "k: <doc>" and hands back the value node, so validator
// cases can be written as the YAML a user would actually type.
func valueNode(t *testing.T, doc string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte("k: "+doc+"\n"), &root); err != nil {
		t.Fatalf("test fixture %q does not parse: %v", doc, err)
	}
	return root.Content[0].Content[1]
}

// TestLeafValidators exercises the shared validator constructors through the
// registry rows that use them — every accepted value returns the typed Go
// value with no diagnostics, every rejected one returns the named code with
// the row's path and error severity.
func TestLeafValidators(t *testing.T) {
	src := Source{Layer: LayerRepository, Name: ".docket.yml"}
	cases := []struct {
		name     string
		path     string
		doc      string
		want     any    // expected typed value on acceptance
		wantCode string // "" == acceptance
	}{
		// bool: only the canonical spellings, plain or !!bool-tagged.
		{"bool true", "auto_groom", "true", true, ""},
		{"bool false", "auto_groom", "false", false, ""},
		{"bool yaml11 yes", "auto_groom", "yes", nil, CodeInvalidType},
		{"bool yaml11 off", "auto_groom", "off", nil, CodeInvalidType},
		{"bool quoted", "auto_groom", `"true"`, nil, CodeInvalidType},
		{"bool tagged non-canonical", "auto_groom", "!!bool yes", nil, CodeInvalidValue},
		{"bool from list", "auto_groom", "[true]", nil, CodeInvalidType},

		// int: !!int tag, decimal, non-negative.
		{"int ok", "learnings.cap", "0", 0, ""},
		{"int positive", "learnings.cap", "300", 300, ""},
		{"int negative", "learnings.cap", "-1", nil, CodeInvalidValue},
		{"int hex", "learnings.cap", "0x10", nil, CodeInvalidValue},
		{"int string", "learnings.cap", `"300"`, nil, CodeInvalidType},
		{"int bool", "learnings.cap", "true", nil, CodeInvalidType},

		// enum membership.
		{"enum ok", "finalize.gate", "local", "local", ""},
		{"enum off is a string not a bool", "finalize.gate", "off", "off", ""},
		{"enum bad", "finalize.gate", "sometimes", nil, CodeInvalidValue},
		{"enum wrong type", "finalize.gate", "1", nil, CodeInvalidType},
		// metadata_branch is an obsolete tombstone (0363): its validator is a
		// permissive string leaf and its obsolescence is asserted at decode, so it
		// carries no enum validator case here.

		// strings: relative-path, non-empty, space-free variants.
		{"dir ok", "changes_dir", "docs/changes", "docs/changes", ""},
		{"dir absolute", "changes_dir", "/etc/changes", nil, CodeInvalidValue},
		{"dir unclean", "changes_dir", "docs/../etc", nil, CodeInvalidValue},
		{"dir dotdot", "changes_dir", "../outside", nil, CodeInvalidValue},
		{"dir trailing slash", "changes_dir", "docs/changes/", nil, CodeInvalidValue},
		{"dir empty", "changes_dir", `""`, nil, CodeInvalidValue},
		{"branch non-empty", "integration_branch", "develop", "develop", ""},
		{"branch empty", "integration_branch", `""`, nil, CodeInvalidValue},
		{"test command may contain spaces", "finalize.test_command", `"make test"`, "make test", ""},
		{"persona may be empty", "dummy_mode.persona", `""`, "", ""},
		{"skills role non-empty string", "skills.build", "docket-build", "docket-build", ""},
		{"skills role wrong type", "skills.build", "true", nil, CodeInvalidType},

		// agents/runners leaves: opaque but space-free.
		{"model opaque", "agents.*.*.model", "openrouter/moonshotai/kimi-k3", "openrouter/moonshotai/kimi-k3", ""},
		{"model with space", "agents.*.*.model", `"claude opus"`, nil, CodeInvalidValue},
		{"effort auto is opaque", "agents.*.*.effort", "auto", "auto", ""},
		{"shim model inherit", "runners.*.shim_model", "inherit", "inherit", ""},
		{"sandbox enum", "runners.codex.sandbox", "workspace-write", "workspace-write", ""},
		{"sandbox bad enum", "runners.codex.sandbox", "yolo", nil, CodeInvalidValue},
		{"runner network bool", "runners.codex.network", "false", false, ""},

		// lists.
		{"board surfaces list", "board_surfaces", "[inline, github]", []string{"inline", "github"}, ""},
		{"board surfaces empty list allowed", "board_surfaces", "[]", []string{}, ""},
		{"board surfaces not a list", "board_surfaces", "inline", nil, CodeInvalidType},
		{"board surfaces non-string element", "board_surfaces", "[1]", nil, CodeInvalidType},
		{"agent harnesses list", "agent_harnesses", "[claude, cursor]", []string{"claude", "cursor"}, ""},
		{"agent harnesses empty list allowed", "agent_harnesses", "[]", []string{}, ""},
		{"agent harnesses duplicate", "agent_harnesses", "[claude, claude]", nil, CodeInvalidValue},
		{"agent harnesses out-of-set token", "agent_harnesses", "[emacs]", nil, CodeInvalidValue},
		{"agent harnesses default is not a harness token", "agent_harnesses", "[default]", nil, CodeInvalidValue},
		{"change types ok", "change_types", "[feat, fix]", []string{"feat", "fix"}, ""},
		{"change types empty", "change_types", "[]", nil, CodeInvalidValue},
		{"change types duplicate", "change_types", "[feat, feat]", nil, CodeInvalidValue},
		{"change types reserved all", "change_types", "[all]", nil, CodeInvalidValue},
		{"change types reserved untyped", "change_types", "[untyped]", nil, CodeInvalidValue},
		{"change types bad pattern", "change_types", "[Feat]", nil, CodeInvalidValue},
		{"change types leading dash", "change_types", "[-feat]", nil, CodeInvalidValue},

		// scalar-or-list.
		{"auto_capture types all", "auto_capture.types", "all", "all", ""},
		{"auto_capture types list", "auto_capture.types", "[feat, fix]", []string{"feat", "fix"}, ""},
		{"auto_capture types duplicate", "auto_capture.types", "[feat, feat]", nil, CodeInvalidValue},
		{"auto_capture types bad sentinel", "auto_capture.types", "some", nil, CodeInvalidValue},
		{"dummy surfaces all", "dummy_mode.surfaces", "all", "all", ""},
		{"dummy surfaces subset", "dummy_mode.surfaces", "[dialogue, pr]", []string{"dialogue", "pr"}, ""},
		{"dummy surfaces bad token", "dummy_mode.surfaces", "[banners]", nil, CodeInvalidValue},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := specFor(t, tc.path)
			got, diags := spec.validate(src, spec.path, valueNode(t, tc.doc))
			if tc.wantCode == "" {
				if len(diags) != 0 {
					t.Fatalf("%s = %q: want acceptance, got diagnostics %+v", tc.path, tc.doc, diags)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("%s = %q: value %#v, want %#v", tc.path, tc.doc, got, tc.want)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("%s = %q: want exactly one %s diagnostic, got %+v", tc.path, tc.doc, tc.wantCode, diags)
			}
			d := diags[0]
			if d.Code != tc.wantCode {
				t.Errorf("%s = %q: code %q, want %q", tc.path, tc.doc, d.Code, tc.wantCode)
			}
			if d.Path != spec.path {
				t.Errorf("%s = %q: diagnostic path %q, want %q", tc.path, tc.doc, d.Path, spec.path)
			}
			if d.Severity != SeverityError {
				t.Errorf("%s = %q: severity %q, want %q", tc.path, tc.doc, d.Severity, SeverityError)
			}
			if d.Provenance == nil || d.Provenance.Source != ".docket.yml" || d.Provenance.Line < 1 {
				t.Errorf("%s = %q: provenance %+v, want source .docket.yml with a line", tc.path, tc.doc, d.Provenance)
			}
			if got != nil {
				t.Errorf("%s = %q: rejected value must be nil, got %#v", tc.path, tc.doc, got)
			}
		})
	}
}

// TestGithubProjectLeaf covers the one scalar-or-map row: the `auto` sentinel,
// the owner/number map, and its own key policy.
func TestGithubProjectLeaf(t *testing.T) {
	src := Source{Layer: LayerRepository, Name: ".docket.yml"}
	spec := specFor(t, "github_project")

	got, diags := spec.validate(src, spec.path, valueNode(t, "auto"))
	if len(diags) != 0 {
		t.Fatalf("github_project: auto rejected: %+v", diags)
	}
	if !reflect.DeepEqual(got, githubProject{Auto: true}) {
		t.Errorf("github_project: auto = %#v, want githubProject{Auto: true}", got)
	}

	got, diags = spec.validate(src, spec.path, valueNode(t, "{owner: acme, number: 7}"))
	if len(diags) != 0 {
		t.Fatalf("github_project map rejected: %+v", diags)
	}
	if !reflect.DeepEqual(got, githubProject{Owner: "acme", Number: 7}) {
		t.Errorf("github_project map = %#v, want {acme 7}", got)
	}

	rejects := []struct{ name, doc, code, path string }{
		{"bad sentinel", "always", CodeInvalidValue, "github_project"},
		{"number below one", "{owner: acme, number: 0}", CodeInvalidValue, "github_project"},
		{"number wrong type", `{owner: acme, number: "7"}`, CodeInvalidType, "github_project.number"},
		{"empty owner", `{owner: "", number: 7}`, CodeInvalidValue, "github_project.owner"},
		{"missing owner", "{number: 7}", CodeInvalidValue, "github_project"},
		{"missing number", "{owner: acme}", CodeInvalidValue, "github_project"},
		{"unknown field", "{owner: acme, num: 7}", CodeUnknownKey, "github_project.num"},
		{"list", "[acme]", CodeInvalidType, "github_project"},
	}
	for _, tc := range rejects {
		t.Run(tc.name, func(t *testing.T) {
			got, diags := spec.validate(src, spec.path, valueNode(t, tc.doc))
			if len(diags) != 1 {
				t.Fatalf("github_project: %s: want one diagnostic, got %+v", tc.doc, diags)
			}
			if diags[0].Code != tc.code || diags[0].Path != tc.path {
				t.Errorf("github_project: %s: got %s/%s, want %s/%s",
					tc.doc, diags[0].Code, diags[0].Path, tc.code, tc.path)
			}
			if diags[0].Severity != SeverityError {
				t.Errorf("github_project: %s: severity %q, want error", tc.doc, diags[0].Severity)
			}
			if got != nil {
				t.Errorf("github_project: %s: rejected value must be nil, got %#v", tc.doc, got)
			}
		})
	}
}

// TestDiagnosticsNeverEchoTheDocument guards the Global Constraint that
// diagnostics name the setting and the source file, never the file's other
// contents: validating one leaf of a document must not leak a sibling's value.
func TestDiagnosticsNeverEchoTheDocument(t *testing.T) {
	src := Source{Layer: LayerGlobal, Name: "/cfg/config.yml"}
	spec := specFor(t, "changes_dir")
	_, diags := spec.validate(src, spec.path, valueNode(t, "/etc/changes"))
	if len(diags) != 1 {
		t.Fatalf("want one diagnostic, got %+v", diags)
	}
	msg := diags[0].Message
	if msg == "" {
		t.Fatal("diagnostic message is empty")
	}
	for _, want := range []string{"/cfg/config.yml", "changes_dir"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not name %q", msg, want)
		}
	}
}
