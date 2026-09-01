package config

import (
	"reflect"
	"strings"
	"testing"
)

// repoSource is the layer every decode case is written against; the fence
// rules that make the layer matter belong to resolution, not to decode.
func repoSource(data string) Source {
	return Source{Layer: LayerRepository, Name: ".docket.yml", Data: []byte(data)}
}

// decodeDoc runs the real pipeline — parseLayer then decodeLayer — so a case
// is written as the YAML a user would type. A node-stage diagnostic on a
// decode fixture means the fixture is wrong, so it fails the test outright.
func decodeDoc(t *testing.T, data string) ([]leafDecl, []Diagnostic) {
	t.Helper()
	src := repoSource(data)
	root, diags := parseLayer(src)
	if len(diags) != 0 {
		t.Fatalf("fixture %q failed the node stage: %+v", data, diags)
	}
	if root == nil {
		t.Fatalf("fixture %q parsed to an absent layer", data)
	}
	return decodeLayer(root, src)
}

// decodeCase declares one registry row's acceptance: the concrete path a user
// writes, in block spelling and (where the kind admits one) flow spelling,
// and the typed value decode must produce.
type decodeCase struct {
	row      string // the registry row this case covers (dynamic rows keep their "*" spelling)
	path     string // the concrete path the fixture declares
	block    string
	flow     string // "" when the kind has no flow spelling
	value    any
	obsolete bool // expect an obsolete-setting warning and NO leaf
}

// decodeAcceptanceCases covers every registry row. TestDecodeEveryRegisteredPath
// checks the coverage in both directions against registry() itself, so a row
// added to the schema without a case here reddens rather than going untested.
func decodeAcceptanceCases() []decodeCase {
	cases := []decodeCase{
		{row: "runtime.bash", path: "runtime.bash",
			block: "runtime:\n  bash: /bin/bash\n", flow: "runtime: {bash: /bin/bash}\n",
			obsolete: true},

		{row: "metadata_branch", path: "metadata_branch",
			block: "metadata_branch: main\n", obsolete: true},
		{row: "integration_branch", path: "integration_branch",
			block: "integration_branch: develop\n", value: "develop"},
		{row: "changes_dir", path: "changes_dir",
			block: "changes_dir: docs/plan/changes\n", value: "docs/plan/changes"},
		{row: "adrs_dir", path: "adrs_dir",
			block: "adrs_dir: docs/decisions\n", value: "docs/decisions"},
		{row: "results_dir", path: "results_dir",
			block: "results_dir: docs/outcomes\n", value: "docs/outcomes"},

		{row: "finalize.gate", path: "finalize.gate",
			block: "finalize:\n  gate: off\n", flow: "finalize: {gate: off}\n", value: "off"},
		{row: "finalize.test_command", path: "finalize.test_command",
			block: "finalize:\n  test_command: \"make test\"\n",
			flow:  "finalize: {test_command: \"make test\"}\n", value: "make test"},
		{row: "finalize.require_pr_approval", path: "finalize.require_pr_approval",
			block: "finalize:\n  require_pr_approval: true\n",
			flow:  "finalize: {require_pr_approval: true}\n", value: true},
		{row: "finalize.skip_results_only_delta", path: "finalize.skip_results_only_delta",
			block: "finalize:\n  skip_results_only_delta: true\n",
			flow:  "finalize: {skip_results_only_delta: true}\n", value: true},

		{row: "learnings.enabled", path: "learnings.enabled",
			block: "learnings:\n  enabled: false\n", flow: "learnings: {enabled: false}\n", value: false},
		{row: "learnings.cap", path: "learnings.cap",
			block: "learnings:\n  cap: 42\n", flow: "learnings: {cap: 42}\n", value: 42},

		{row: "reclaim.lease_ttl", path: "reclaim.lease_ttl",
			block: "reclaim:\n  lease_ttl: 24\n", flow: "reclaim: {lease_ttl: 24}\n", value: 24},
		{row: "reclaim.auto", path: "reclaim.auto",
			block: "reclaim:\n  auto: true\n", flow: "reclaim: {auto: true}\n", value: true},

		{row: "build.checkpoint", path: "build.checkpoint",
			block: "build:\n  checkpoint: true\n", flow: "build: {checkpoint: true}\n", value: true},
		{row: "build.gate", path: "build.gate",
			block: "build:\n  gate: off\n", flow: "build: {gate: off}\n", value: "off"},
		{row: "build.test_command", path: "build.test_command",
			block: "build:\n  test_command: \"go test ./...\"\n",
			flow:  "build: {test_command: \"go test ./...\"}\n", value: "go test ./..."},

		{row: "review.min_fix_severity", path: "review.min_fix_severity",
			block: "review:\n  min_fix_severity: blocker\n",
			flow:  "review: {min_fix_severity: blocker}\n", value: "blocker"},
		{row: "review.max_fix_tasks", path: "review.max_fix_tasks",
			block: "review:\n  max_fix_tasks: 3\n", flow: "review: {max_fix_tasks: 3}\n", value: 3},

		{row: "gate_observation_budget", path: "gate_observation_budget",
			block: "gate_observation_budget: 15\n", value: 15},
		{row: "delegation_observation_budget", path: "delegation_observation_budget",
			block: "delegation_observation_budget: 90\n", value: 90},

		{row: "board_surfaces", path: "board_surfaces",
			block: "board_surfaces:\n  - inline\n  - github\n",
			flow:  "board_surfaces: [inline, github]\n", value: []string{"inline", "github"}},
		{row: "github_project", path: "github_project",
			block: "github_project:\n  owner: acme\n  number: 7\n",
			flow:  "github_project: {owner: acme, number: 7}\n",
			value: githubProject{Owner: "acme", Number: 7}},
		{row: "terminal_publish", path: "terminal_publish",
			block: "terminal_publish: true\n", value: true},
		{row: "auto_groom", path: "auto_groom",
			block: "auto_groom: true\n", value: true},

		{row: "change_types", path: "change_types",
			block: "change_types:\n  - feat\n  - fix\n",
			flow:  "change_types: [feat, fix]\n", value: []string{"feat", "fix"}},

		{row: "auto_capture.enabled", path: "auto_capture.enabled",
			block: "auto_capture:\n  enabled: true\n",
			flow:  "auto_capture: {enabled: true}\n", value: true},
		{row: "auto_capture.types", path: "auto_capture.types",
			block: "auto_capture:\n  types:\n    - feat\n",
			flow:  "auto_capture: {types: [feat]}\n", value: []string{"feat"}},

		{row: "dummy_mode.enabled", path: "dummy_mode.enabled",
			block: "dummy_mode:\n  enabled: true\n",
			flow:  "dummy_mode: {enabled: true}\n", value: true},
		{row: "dummy_mode.persona", path: "dummy_mode.persona",
			block: "dummy_mode:\n  persona: parrot\n",
			flow:  "dummy_mode: {persona: parrot}\n", value: "parrot"},
		{row: "dummy_mode.surfaces", path: "dummy_mode.surfaces",
			block: "dummy_mode:\n  surfaces:\n    - dialogue\n",
			flow:  "dummy_mode: {surfaces: [dialogue]}\n", value: []string{"dialogue"}},

		{row: "agent_harnesses", path: "agent_harnesses",
			block: "agent_harnesses:\n  - claude\n  - codex\n",
			flow:  "agent_harnesses: [claude, codex]\n", value: []string{"claude", "codex"}},

		{row: "skills.brainstorm", path: "skills.brainstorm",
			block: "skills:\n  brainstorm: my-brainstorm\n",
			flow:  "skills: {brainstorm: my-brainstorm}\n", value: "my-brainstorm"},
		{row: "skills.plan", path: "skills.plan",
			block: "skills:\n  plan: my-plan\n",
			flow:  "skills: {plan: my-plan}\n", value: "my-plan"},
		{row: "skills.build", path: "skills.build",
			block: "skills:\n  build: my-build\n",
			flow:  "skills: {build: my-build}\n", value: "my-build"},
		{row: "skills.review", path: "skills.review",
			block: "skills:\n  review: my-review\n",
			flow:  "skills: {review: my-review}\n", value: "my-review"},
		{row: "skills.finish", path: "skills.finish",
			block: "skills:\n  finish: my-finish\n",
			flow:  "skills: {finish: my-finish}\n", value: "my-finish"},

		{row: "agents.*.*.model", path: "agents.claude.adr.model",
			block: "agents:\n  claude:\n    adr:\n      model: claude-opus-5\n",
			flow:  "agents: {claude: {adr: {model: claude-opus-5}}}\n", value: "claude-opus-5"},
		{row: "agents.*.*.effort", path: "agents.codex.status.effort",
			block: "agents:\n  codex:\n    status:\n      effort: xhigh\n",
			flow:  "agents: {codex: {status: {effort: xhigh}}}\n", value: "xhigh"},
		{row: "agents.*.*.runner", path: "agents.opencode.build-max.runner",
			block: "agents:\n  opencode:\n    build-max:\n      runner: opencode\n",
			flow:  "agents: {opencode: {build-max: {runner: opencode}}}\n", value: "opencode"},

		{row: "runners.codex.sandbox", path: "runners.codex.sandbox",
			block: "runners:\n  codex:\n    sandbox: workspace-write\n",
			flow:  "runners: {codex: {sandbox: workspace-write}}\n", value: "workspace-write"},
		{row: "runners.codex.network", path: "runners.codex.network",
			block: "runners:\n  codex:\n    network: false\n",
			flow:  "runners: {codex: {network: false}}\n", value: false},
		{row: "runners.opencode.permissions", path: "runners.opencode.permissions",
			block: "runners:\n  opencode:\n    permissions: ask\n",
			flow:  "runners: {opencode: {permissions: ask}}\n", value: "ask"},
		{row: "runners.*.shim_model", path: "runners.cursor.shim_model",
			block: "runners:\n  cursor:\n    shim_model: inherit\n",
			flow:  "runners: {cursor: {shim_model: inherit}}\n", value: "inherit"},
		{row: "runners.*.shim_effort", path: "runners.codex.shim_effort",
			block: "runners:\n  codex:\n    shim_effort: high\n",
			flow:  "runners: {codex: {shim_effort: high}}\n", value: "high"},
	}

	// The board presentation block (change 0367): one acceptance case per row,
	// generated from BoardSectionTokens so a new section cannot ship without its
	// decode coverage. section_order decodes to the token list; each sort leaf
	// decodes to a valid enum member.
	cases = append(cases, decodeCase{
		row:   "board.section_order",
		path:  "board.section_order",
		block: "board:\n  section_order:\n    - in-progress\n    - built\n    - blocked\n    - groomed\n    - proposed\n    - deferred\n",
		flow:  "board: {section_order: [in-progress, built, blocked, groomed, proposed, deferred]}\n",
		value: []string{"in-progress", "built", "blocked", "groomed", "proposed", "deferred"},
	})
	for _, s := range BoardSectionTokens {
		cases = append(cases,
			decodeCase{
				row:   "board.sorting." + s + ".by",
				path:  "board.sorting." + s + ".by",
				block: "board:\n  sorting:\n    " + s + ":\n      by: created\n",
				flow:  "board: {sorting: {" + s + ": {by: created}}}\n",
				value: "created",
			},
			decodeCase{
				row:   "board.sorting." + s + ".direction",
				path:  "board.sorting." + s + ".direction",
				block: "board:\n  sorting:\n    " + s + ":\n      direction: asc\n",
				flow:  "board: {sorting: {" + s + ": {direction: asc}}}\n",
				value: "asc",
			},
		)
	}
	return cases
}

// TestDecodeEveryRegisteredPath decodes a minimal valid document per registry
// row, in every spelling the row's kind admits, and pins the resulting leaf:
// concrete path, matched row, typed value, and provenance. The coverage
// compare runs both ways against registry(), so neither a new row nor a stale
// case can hide.
func TestDecodeEveryRegisteredPath(t *testing.T) {
	cases := decodeAcceptanceCases()

	covered := make(map[string]bool, len(cases))
	for _, tc := range cases {
		if covered[tc.row] {
			t.Errorf("registry row %q has more than one acceptance case", tc.row)
		}
		covered[tc.row] = true
		if specByPath(tc.row) == nil {
			t.Errorf("acceptance case names %q, which is not a registry row", tc.row)
		}
	}
	for _, spec := range registry() {
		if !covered[spec.path] {
			t.Errorf("registry row %q has no decode acceptance case", spec.path)
		}
	}

	for _, tc := range cases {
		for _, style := range []struct{ name, doc string }{{"block", tc.block}, {"flow", tc.flow}} {
			if style.doc == "" {
				continue
			}
			t.Run(tc.path+"/"+style.name, func(t *testing.T) {
				leaves, diags := decodeDoc(t, style.doc)

				if tc.obsolete {
					if len(leaves) != 0 {
						t.Errorf("%s: obsolete setting produced leaves %+v, want none", tc.path, leaves)
					}
					if len(diags) != 1 {
						t.Fatalf("%s: want one obsolete-setting diagnostic, got %+v", tc.path, diags)
					}
					if diags[0].Code != CodeObsoleteSetting || diags[0].Severity != SeverityWarning {
						t.Errorf("%s: got %s/%s, want %s/%s", tc.path,
							diags[0].Code, diags[0].Severity, CodeObsoleteSetting, SeverityWarning)
					}
					if diags[0].Path != tc.path {
						t.Errorf("%s: diagnostic path %q", tc.path, diags[0].Path)
					}
					return
				}

				if len(diags) != 0 {
					t.Fatalf("%s: unexpected diagnostics %+v", tc.path, diags)
				}
				if len(leaves) != 1 {
					t.Fatalf("%s: want exactly one leaf, got %+v", tc.path, leaves)
				}
				got := leaves[0]
				if got.path != tc.path {
					t.Errorf("leaf path %q, want %q", got.path, tc.path)
				}
				if got.spec == nil || got.spec.path != tc.row {
					t.Fatalf("%s: matched row %+v, want %q", tc.path, got.spec, tc.row)
				}
				if !reflect.DeepEqual(got.value, tc.value) {
					t.Errorf("%s: value %#v (%T), want %#v (%T)",
						tc.path, got.value, got.value, tc.value, tc.value)
				}
				if got.prov.Layer != LayerRepository || got.prov.Source != ".docket.yml" {
					t.Errorf("%s: provenance %+v, want the repository layer", tc.path, got.prov)
				}
				if got.prov.Line < 1 || got.prov.Column < 1 {
					t.Errorf("%s: provenance %+v has no position", tc.path, got.prov)
				}
			})
		}
	}
}

// TestDecodeRejections pins the per-leaf and per-key policy: which spellings
// are refused, under which code, at which path, and at which severity — the
// two deliberate warn-and-ignore surfaces (unknown skills roles, unknown board
// tokens) included.
func TestDecodeRejections(t *testing.T) {
	cases := []struct {
		name, yaml, wantCode, wantPath string
		wantSev                        Severity
	}{
		{"unknown top-level key", "not_a_key: 1\n", CodeUnknownKey, "not_a_key", SeverityError},
		{"unknown nested key", "finalize:\n  bogus: 1\n", CodeUnknownKey, "finalize.bogus", SeverityError},
		{"unknown skills role is a warning", "skills:\n  deploy: x\n", CodeUnknownKey, "skills.deploy", SeverityWarning},
		{"bool yaml11 alias", "auto_groom: yes\n", CodeInvalidType, "auto_groom", SeverityError},
		{"bool quoted string", "auto_groom: \"true\"\n", CodeInvalidType, "auto_groom", SeverityError},
		{"negative int", "learnings:\n  cap: -1\n", CodeInvalidValue, "learnings.cap", SeverityError},
		{"bad enum", "finalize:\n  gate: sometimes\n", CodeInvalidValue, "finalize.gate", SeverityError},
		{"metadata branch obsolete every layer", "metadata_branch: trunk\n", CodeObsoleteSetting, "metadata_branch", SeverityWarning},
		{"absolute dir", "changes_dir: /etc/changes\n", CodeInvalidValue, "changes_dir", SeverityError},
		{"unclean dir", "changes_dir: docs/../etc\n", CodeInvalidValue, "changes_dir", SeverityError},
		{"empty change_types", "change_types: []\n", CodeInvalidValue, "change_types", SeverityError},
		{"duplicate change type", "change_types: [feat, feat]\n", CodeInvalidValue, "change_types", SeverityError},
		{"reserved change type", "change_types: [all]\n", CodeInvalidValue, "change_types", SeverityError},
		{"bad token pattern", "change_types: [Feat]\n", CodeInvalidValue, "change_types", SeverityError},
		{"obsolete scalar auto_capture true", "auto_capture: true\n", CodeInvalidValue, "auto_capture", SeverityError},
		{"obsolete scalar auto_capture false", "auto_capture: false\n", CodeInvalidValue, "auto_capture", SeverityError},
		{"interior node as scalar", "finalize: local\n", CodeInvalidType, "finalize", SeverityError},
		{"harness typo", "agents:\n  cluade:\n    adr: {model: m, effort: low}\n", CodeUnknownKey, "agents.cluade", SeverityError},
		{"agent typo", "agents:\n  claude:\n    adr-writer: {model: m, effort: low}\n", CodeUnknownKey, "agents.claude.adr-writer", SeverityError},
		{"agent field typo", "agents:\n  claude:\n    adr: {model: m, efort: low}\n", CodeUnknownKey, "agents.claude.adr.efort", SeverityError},
		{"model with space", "agents:\n  claude:\n    adr: {model: claude opus, effort: low}\n", CodeInvalidValue, "agents.claude.adr.model", SeverityError},
		{"runner name typo", "runners:\n  codexx:\n    sandbox: workspace-write\n", CodeUnknownKey, "runners.codexx", SeverityError},
		{"runner key typo", "runners:\n  codex:\n    sandbx: workspace-write\n", CodeUnknownKey, "runners.codex.sandbx", SeverityError},
		{"runner key off its own runner", "runners:\n  cursor:\n    sandbox: workspace-write\n", CodeUnknownKey, "runners.cursor.sandbox", SeverityError},
		{"bad sandbox enum", "runners:\n  codex:\n    sandbox: yolo\n", CodeInvalidValue, "runners.codex.sandbox", SeverityError},
		{"github_project map ok", "github_project: {owner: acme, number: 7}\n", "", "", ""},
		{"github_project bad number", "github_project: {owner: acme, number: 0}\n", CodeInvalidValue, "github_project", SeverityError},
		{"github_project unknown field", "github_project: {owner: acme, num: 7}\n", CodeUnknownKey, "github_project.num", SeverityError},
		{"dummy surfaces bad token", "dummy_mode:\n  surfaces: [banners]\n", CodeInvalidValue, "dummy_mode.surfaces", SeverityError},
		{"unknown board token is a warning", "board_surfaces: [inline, trello]\n", CodeUnknownKey, "board_surfaces", SeverityWarning},
		{"runtime.bash obsolete every layer", "runtime:\n  bash: /bin/bash\n", CodeObsoleteSetting, "runtime.bash", SeverityWarning},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := decodeDoc(t, tc.yaml)
			if tc.wantCode == "" {
				if len(diags) != 0 {
					t.Fatalf("want acceptance, got %+v", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("want exactly one %s diagnostic, got %+v", tc.wantCode, diags)
			}
			d := diags[0]
			if d.Code != tc.wantCode {
				t.Errorf("code %q, want %q", d.Code, tc.wantCode)
			}
			if d.Path != tc.wantPath {
				t.Errorf("path %q, want %q", d.Path, tc.wantPath)
			}
			if d.Severity != tc.wantSev {
				t.Errorf("severity %q, want %q", d.Severity, tc.wantSev)
			}
			if d.Provenance == nil {
				t.Fatal("diagnostic carries no provenance")
			}
			if d.Provenance.Layer != LayerRepository || d.Provenance.Source != ".docket.yml" {
				t.Errorf("provenance %+v, want the repository layer", *d.Provenance)
			}
			if d.Provenance.Line < 1 || d.Provenance.Column < 1 {
				t.Errorf("provenance %+v has no position", *d.Provenance)
			}
			if !strings.Contains(d.Message, ".docket.yml") {
				t.Errorf("message %q does not name the source file", d.Message)
			}
		})
	}
}

// TestDecodeRejectionProvenanceLine pins that a rejection points at the
// offending declaration rather than at the document: the failing key is on
// line 3, and nothing about line 1 or 2 may appear.
func TestDecodeRejectionProvenanceLine(t *testing.T) {
	_, diags := decodeDoc(t, "integration_branch: develop\nauto_groom: false\nnot_a_key: 1\n")
	if len(diags) != 1 {
		t.Fatalf("want one diagnostic, got %+v", diags)
	}
	if diags[0].Provenance.Line != 3 {
		t.Errorf("line %d, want 3", diags[0].Provenance.Line)
	}
	for _, leak := range []string{"integration_branch", "auto_groom", "develop\n"} {
		if strings.Contains(diags[0].Message, leak) {
			t.Errorf("message %q echoes unrelated document content %q", diags[0].Message, leak)
		}
	}
}

// TestDecodeUnknownSubtreeIsNotDescended: an unknown harness is reported once,
// at the harness, and its contents are not walked — one typo must not turn
// into a cascade of derived complaints.
func TestDecodeUnknownSubtreeIsNotDescended(t *testing.T) {
	_, diags := decodeDoc(t, "agents:\n  cluade:\n    adr:\n      model: m\n      effort: low\n")
	if len(diags) != 1 {
		t.Fatalf("want exactly one diagnostic, got %+v", diags)
	}
	if diags[0].Path != "agents.cluade" {
		t.Errorf("path %q, want agents.cluade", diags[0].Path)
	}
}

// TestDecodeObsoleteScalarAutoCaptureRemedy: the pre-0127 scalar shape must
// tell the reader the nested replacement, not merely refuse.
func TestDecodeObsoleteScalarAutoCaptureRemedy(t *testing.T) {
	_, diags := decodeDoc(t, "auto_capture: true\n")
	if len(diags) != 1 {
		t.Fatalf("want one diagnostic, got %+v", diags)
	}
	for _, want := range []string{"enabled:", "types:"} {
		if !strings.Contains(diags[0].Remedy, want) {
			t.Errorf("remedy %q does not show %q", diags[0].Remedy, want)
		}
	}
}

// TestDecodeBoardSurfacesDropsUnknownTokens: an unknown surface is warned
// about and removed, and the tokens around it survive. `github` is NOT dropped
// here — its fence depends on the layer, so resolution owns it.
func TestDecodeBoardSurfacesDropsUnknownTokens(t *testing.T) {
	leaves, diags := decodeDoc(t, "board_surfaces: [inline, trello, github]\n")
	if len(diags) != 1 || diags[0].Code != CodeUnknownKey {
		t.Fatalf("want one unknown-key warning, got %+v", diags)
	}
	if len(leaves) != 1 {
		t.Fatalf("want one leaf, got %+v", leaves)
	}
	if !reflect.DeepEqual(leaves[0].value, []string{"inline", "github"}) {
		t.Errorf("value %#v, want [inline github]", leaves[0].value)
	}
}

// TestDecodeMultipleLeaves: an ordinary multi-section document yields one leaf
// per declared setting, in document order, each carrying its own line.
func TestDecodeMultipleLeaves(t *testing.T) {
	leaves, diags := decodeDoc(t, "integration_branch: develop\nfinalize:\n  gate: off\n  require_pr_approval: true\n")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics %+v", diags)
	}
	wantPaths := []string{"integration_branch", "finalize.gate", "finalize.require_pr_approval"}
	wantLines := []int{1, 3, 4}
	if len(leaves) != len(wantPaths) {
		t.Fatalf("want %d leaves, got %+v", len(wantPaths), leaves)
	}
	for i, leaf := range leaves {
		if leaf.path != wantPaths[i] {
			t.Errorf("leaf %d path %q, want %q", i, leaf.path, wantPaths[i])
		}
		if leaf.prov.Line != wantLines[i] {
			t.Errorf("leaf %d line %d, want %d", i, leaf.prov.Line, wantLines[i])
		}
	}
}

// TestDecodeEmptySection: a section header with nothing under it declares
// nothing — neither a leaf nor a complaint.
func TestDecodeEmptySection(t *testing.T) {
	leaves, diags := decodeDoc(t, "finalize:\nreclaim:\n  auto: true\n")
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics %+v", diags)
	}
	if len(leaves) != 1 || leaves[0].path != "reclaim.auto" {
		t.Fatalf("want only reclaim.auto, got %+v", leaves)
	}
}

// TestDecodeAbsentLayer: a nil root (an empty or comments-only layer) decodes
// to nothing at all.
func TestDecodeAbsentLayer(t *testing.T) {
	leaves, diags := decodeLayer(nil, repoSource(""))
	if len(leaves) != 0 || len(diags) != 0 {
		t.Errorf("absent layer decoded to %+v / %+v", leaves, diags)
	}
}
