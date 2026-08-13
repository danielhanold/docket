package config

import (
	"reflect"
	"testing"
)

// capWant is one expected capability entry, flattened to the four fields the
// matrix decides plus the layer the entry must be attributed to.
type capWant struct {
	path   string
	class  Classification
	active bool
	block  bool
	layer  LayerKind
}

func gotCaps(caps []Capability) []capWant {
	out := make([]capWant, 0, len(caps))
	for _, c := range caps {
		out = append(out, capWant{c.Path, c.Classification, c.Active, c.MutationBlock, c.Provenance.Layer})
	}
	return out
}

func mustSnapshot(t *testing.T, sources ...Source) *Snapshot {
	t.Helper()
	snap, diags, err := Resolve(sources, mainCtx)
	if err != nil {
		t.Fatalf("Resolve: unexpected error %v; diagnostics %+v", err, diags)
	}
	return snap
}

// blockerPaths is the ordered path list of the blocking diagnostics, which is
// what the mutation preflight will hand a user to repair in one pass.
func blockerPaths(snap *Snapshot) []string {
	var out []string
	for _, d := range snap.Diagnostics {
		if d.Code == CodeDeferredCapRequested {
			out = append(out, d.Path)
		}
	}
	return out
}

func diagPathsWithCode(snap *Snapshot, code string) []string {
	var out []string
	for _, d := range snap.Diagnostics {
		if d.Code == code {
			out = append(out, d.Path)
		}
	}
	return out
}

// TestClassifyMatrix walks every row of the spec's v0.9.2 setting matrix: each
// case declares the row in a layer that may declare it, and pins the complete
// capability list plus the complete ordered blocker set. A row that classifies
// to nothing pins the empty list, which is the half a "did it warn?" test
// misses.
func TestClassifyMatrix(t *testing.T) {
	cases := []struct {
		name     string
		sources  []Source
		want     []capWant
		blockers []string
	}{
		// Row 1 — obsolete in every layer, never a blocker.
		{
			name:    "runtime.bash is obsolete",
			sources: []Source{srcL("runtime:\n  bash: /bin/bash\n")},
			want:    []capWant{{"runtime.bash", Obsolete, false, false, LayerRepositoryLocal}},
		},

		// Rows 2-6 — repository identity is plain supported policy.
		{
			name: "repository identity classifies to nothing",
			sources: []Source{srcR("metadata_branch: main\nintegration_branch: develop\n" +
				"changes_dir: docs/c\nadrs_dir: docs/a\nresults_dir: docs/r\n")},
		},

		// Row 7 — finalize.gate defers by value.
		{
			name:     "finalize.gate ci blocks",
			sources:  []Source{srcR("finalize:\n  gate: ci\n")},
			want:     []capWant{{"finalize.gate", Deferred, true, true, LayerRepository}},
			blockers: []string{"finalize.gate"},
		},
		{
			name:     "finalize.gate both blocks",
			sources:  []Source{srcR("finalize:\n  gate: both\n")},
			want:     []capWant{{"finalize.gate", Deferred, true, true, LayerRepository}},
			blockers: []string{"finalize.gate"},
		},
		{
			name:    "finalize.gate off is supported",
			sources: []Source{srcR("finalize:\n  gate: off\n")},
		},
		{
			name:    "finalize.gate local is supported",
			sources: []Source{srcR("finalize:\n  gate: local\n")},
		},

		// Rows 8-9 — supported finalize leaves.
		{
			name:    "finalize test_command and require_pr_approval are supported",
			sources: []Source{srcR("finalize:\n  test_command: make test\n  require_pr_approval: true\n")},
		},

		// Row 10 — repository-fenced deferred bool.
		{
			name:     "skip_results_only_delta true blocks",
			sources:  []Source{srcR("finalize:\n  skip_results_only_delta: true\n")},
			want:     []capWant{{"finalize.skip_results_only_delta", Deferred, true, true, LayerRepository}},
			blockers: []string{"finalize.skip_results_only_delta"},
		},
		{
			name:    "skip_results_only_delta explicit false is inactive",
			sources: []Source{srcR("finalize:\n  skip_results_only_delta: false\n")},
			want:    []capWant{{"finalize.skip_results_only_delta", Deferred, false, false, LayerRepository}},
		},

		// Rows 11-12 — learnings.
		{
			name:    "learnings.enabled is supported policy",
			sources: []Source{srcR("learnings:\n  enabled: true\n")},
		},
		{
			name:    "learnings.cap is inert even with learnings enabled",
			sources: []Source{srcR("learnings:\n  enabled: true\n  cap: 42\n")},
			want:    []capWant{{"learnings.cap", Inert, false, false, LayerRepository}},
		},

		// Rows 13-14 — reclaim is supported.
		{
			name:    "reclaim classifies to nothing",
			sources: []Source{srcR("reclaim:\n  lease_ttl: 12\n  auto: true\n")},
		},

		// Row 15 — build.checkpoint.
		{
			name:     "build.checkpoint true blocks",
			sources:  []Source{srcR("build:\n  checkpoint: true\n")},
			want:     []capWant{{"build.checkpoint", Deferred, true, true, LayerRepository}},
			blockers: []string{"build.checkpoint"},
		},
		{
			name:    "build.checkpoint false is inactive",
			sources: []Source{srcG("build:\n  checkpoint: false\n")},
			want:    []capWant{{"build.checkpoint", Deferred, false, false, LayerGlobal}},
		},

		// Rows 16-18 — supported review and gate budget.
		{
			name:    "review and gate_observation_budget classify to nothing",
			sources: []Source{srcR("review:\n  min_fix_severity: blocker\n  max_fix_tasks: 3\ngate_observation_budget: 15\n")},
		},

		// Row 19 — historical budget.
		{
			name:    "delegation_observation_budget is inert",
			sources: []Source{srcR("delegation_observation_budget: 90\n")},
			want:    []capWant{{"delegation_observation_budget", Inert, false, false, LayerRepository}},
		},

		// Row 20 — the board's dropped token, by layer.
		{
			name:     "committed github board surface is an active dropped capability",
			sources:  []Source{srcR("board_surfaces: [inline, github]\n")},
			want:     []capWant{{"board_surfaces", Deferred, true, true, LayerRepository}},
			blockers: []string{"board_surfaces"},
		},
		{
			name:    "machine github board surface is fenced away, not classified",
			sources: []Source{srcL("board_surfaces: [inline, github]\n")},
		},
		{
			name:    "inline board surface is supported",
			sources: []Source{srcR("board_surfaces: [inline]\n")},
		},
		{
			name:    "unknown board token is dropped at decode and classifies to nothing",
			sources: []Source{srcR("board_surfaces: [inline, gitlab]\n")},
		},
		{
			name:    "a machine layer that drops github still wins the leaf",
			sources: []Source{srcR("board_surfaces: [inline, github]\n"), srcL("board_surfaces: [inline]\n")},
		},

		// Rows 21-23 — inert project, deferred publish and groom.
		{
			name:    "github_project is inert",
			sources: []Source{srcR("github_project:\n  owner: acme\n  number: 7\n")},
			want:    []capWant{{"github_project", Inert, false, false, LayerRepository}},
		},
		{
			name:     "terminal_publish true blocks",
			sources:  []Source{srcR("terminal_publish: true\n")},
			want:     []capWant{{"terminal_publish", Deferred, true, true, LayerRepository}},
			blockers: []string{"terminal_publish"},
		},
		{
			name:     "auto_groom true blocks",
			sources:  []Source{srcR("auto_groom: true\n")},
			want:     []capWant{{"auto_groom", Deferred, true, true, LayerRepository}},
			blockers: []string{"auto_groom"},
		},

		// Row 24 — change types are supported.
		{
			name:    "change_types classifies to nothing",
			sources: []Source{srcR("change_types: [feat, fix]\n")},
		},

		// Rows 25-26 — auto capture and its companion.
		{
			name:     "auto_capture.enabled true blocks",
			sources:  []Source{srcR("auto_capture:\n  enabled: true\n")},
			want:     []capWant{{"auto_capture.enabled", Deferred, true, true, LayerRepository}},
			blockers: []string{"auto_capture.enabled"},
		},
		{
			name:    "auto_capture.types is an inactive companion while capture is off",
			sources: []Source{srcR("auto_capture:\n  enabled: false\n  types: [feat]\n")},
			want: []capWant{
				{"auto_capture.enabled", Deferred, false, false, LayerRepository},
				{"auto_capture.types", Inert, false, false, LayerRepository},
			},
		},
		{
			name:    "auto_capture.types is active but blocks only through enabled",
			sources: []Source{srcR("auto_capture:\n  enabled: true\n  types: [feat]\n")},
			want: []capWant{
				{"auto_capture.enabled", Deferred, true, true, LayerRepository},
				{"auto_capture.types", Inert, true, false, LayerRepository},
			},
			blockers: []string{"auto_capture.enabled"},
		},

		// Rows 27-29 — dummy mode and its two companions.
		{
			name:    "dummy mode companions are inactive while dummy mode is off",
			sources: []Source{srcR("dummy_mode:\n  enabled: false\n  persona: pat\n  surfaces: [pr]\n")},
			want: []capWant{
				{"dummy_mode.enabled", Deferred, false, false, LayerRepository},
				{"dummy_mode.persona", Inert, false, false, LayerRepository},
				{"dummy_mode.surfaces", Inert, false, false, LayerRepository},
			},
		},
		{
			name:    "dummy mode enabled blocks and activates its companions",
			sources: []Source{srcR("dummy_mode:\n  enabled: true\n  persona: pat\n  surfaces: all\n")},
			want: []capWant{
				{"dummy_mode.enabled", Deferred, true, true, LayerRepository},
				{"dummy_mode.persona", Inert, true, false, LayerRepository},
				{"dummy_mode.surfaces", Inert, true, false, LayerRepository},
			},
			blockers: []string{"dummy_mode.enabled"},
		},

		// Row 30 — the historical harness list.
		{
			name:    "agent_harnesses is inert",
			sources: []Source{srcR("agent_harnesses: [claude, codex]\n")},
			want:    []capWant{{"agent_harnesses", Inert, false, false, LayerRepository}},
		},

		// Row 31 — every skill binding is an active deferred request.
		{
			name:     "a skill binding repeating the shipped default still blocks",
			sources:  []Source{srcR("skills:\n  build: docket-build\n")},
			want:     []capWant{{"skills.build", Deferred, true, true, LayerRepository}},
			blockers: []string{"skills.build"},
		},
		{
			name:     "skills.review auto blocks",
			sources:  []Source{srcG("skills:\n  review: auto\n")},
			want:     []capWant{{"skills.review", Deferred, true, true, LayerGlobal}},
			blockers: []string{"skills.review"},
		},
		{
			name:    "an unknown skill role is warned away, not classified",
			sources: []Source{srcR("skills:\n  deploy: something\n")},
		},

		// Row 32 — agent pins are layer-dependent.
		{
			name:    "global agent pins are supported",
			sources: []Source{srcG("agents:\n  claude:\n    adr:\n      model: m1\n      effort: high\n")},
		},
		{
			name:     "a repository agent pin equal to the shipped default still blocks",
			sources:  []Source{srcR("agents:\n  claude:\n    adr:\n      model: claude-opus-5\n")},
			want:     []capWant{{"agents.claude.adr.model", Deferred, true, true, LayerRepository}},
			blockers: []string{"agents.claude.adr.model"},
		},
		{
			name:     "a repository-local agent effort pin blocks",
			sources:  []Source{srcL("agents:\n  codex:\n    status:\n      effort: low\n")},
			want:     []capWant{{"agents.codex.status.effort", Deferred, true, true, LayerRepositoryLocal}},
			blockers: []string{"agents.codex.status.effort"},
		},

		// Row 33 — a runner assignment blocks in any layer.
		{
			name:     "an agent runner assignment blocks in the global layer",
			sources:  []Source{srcG("agents:\n  claude:\n    adr:\n      runner: codex\n")},
			want:     []capWant{{"agents.claude.adr.runner", Deferred, true, true, LayerGlobal}},
			blockers: []string{"agents.claude.adr.runner"},
		},

		// Rows 34-38 — runner settings activate only through an assignment.
		{
			name:    "runner settings are inactive with no runner assigned",
			sources: []Source{srcR("runners:\n  codex:\n    sandbox: workspace-write\n    network: false\n")},
			want: []capWant{
				{"runners.codex.network", Inert, false, false, LayerRepository},
				{"runners.codex.sandbox", Inert, false, false, LayerRepository},
			},
		},
		{
			name: "an assigned runner activates its settings without making them block",
			sources: []Source{
				srcG("agents:\n  claude:\n    adr:\n      runner: codex\n"),
				srcR("runners:\n  codex:\n    sandbox: workspace-write\n    shim_model: gpt-x\n" +
					"  opencode:\n    permissions: ask\n"),
			},
			want: []capWant{
				{"agents.claude.adr.runner", Deferred, true, true, LayerGlobal},
				{"runners.codex.sandbox", Inert, true, false, LayerRepository},
				{"runners.codex.shim_model", Inert, true, false, LayerRepository},
				{"runners.opencode.permissions", Inert, false, false, LayerRepository},
			},
			blockers: []string{"agents.claude.adr.runner"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := mustSnapshot(t, tc.sources...)
			want := tc.want
			if want == nil {
				want = []capWant{}
			}
			if got := gotCaps(snap.Capabilities); !reflect.DeepEqual(got, want) {
				t.Errorf("capabilities = %+v, want %+v", got, want)
			}
			if got := blockerPaths(snap); !reflect.DeepEqual(got, tc.blockers) {
				t.Errorf("blocker paths = %v, want %v", got, tc.blockers)
			}
		})
	}
}

// TestClassifyMultiBlockerCompleteSet is the one-pass-repair promise: four
// active blockers spread over two layers come back together, in path order,
// not one per invocation.
func TestClassifyMultiBlockerCompleteSet(t *testing.T) {
	snap := mustSnapshot(t,
		srcG("auto_capture:\n  enabled: true\n"),
		srcR("build:\n  checkpoint: true\nterminal_publish: true\nfinalize:\n  skip_results_only_delta: true\n"),
	)
	want := []string{
		"auto_capture.enabled",
		"build.checkpoint",
		"finalize.skip_results_only_delta",
		"terminal_publish",
	}
	if got := blockerPaths(snap); !reflect.DeepEqual(got, want) {
		t.Fatalf("blocker paths = %v, want %v", got, want)
	}
	for _, d := range snap.Diagnostics {
		if d.Code != CodeDeferredCapRequested {
			continue
		}
		if d.Severity != SeverityError || d.Classification != Deferred {
			t.Errorf("%s: severity/classification = %s/%s, want error/deferred", d.Path, d.Severity, d.Classification)
		}
		if d.Remedy == "" {
			t.Errorf("%s: a blocker must name the edit that clears it", d.Path)
		}
		if d.Provenance == nil || d.Provenance.Source == "" {
			t.Errorf("%s: a blocker must say which file declares it", d.Path)
		}
	}
	// A blocked snapshot is still a VALID snapshot: inspection must keep working
	// so the user can read the whole remedy.
	if snap.Effective.MetadataBranch.Value != "docket" {
		t.Errorf("effective policy = %q, want the resolved snapshot intact", snap.Effective.MetadataBranch.Value)
	}
}

// TestClassifyNonBlockingNotices pins the three informational codes: an
// explicitly disabled deferred switch, an explicit historical setting, and the
// standing learnings notice.
func TestClassifyNonBlockingNotices(t *testing.T) {
	t.Run("explicit inactive deferred leaf", func(t *testing.T) {
		snap := mustSnapshot(t, srcR("auto_groom: false\n"))
		if got := diagPathsWithCode(snap, CodeDeferredSetting); !reflect.DeepEqual(got, []string{"auto_groom", "learnings.enabled"}) {
			t.Errorf("deferred-setting paths = %v", got)
		}
		for _, d := range snap.Diagnostics {
			if d.Code == CodeDeferredSetting && d.Severity != SeverityInfo {
				t.Errorf("%s: severity = %s, want info", d.Path, d.Severity)
			}
		}
	})

	t.Run("explicit inert leaf", func(t *testing.T) {
		snap := mustSnapshot(t, srcR("learnings:\n  cap: 5\ndelegation_observation_budget: 1\n"))
		want := []string{"delegation_observation_budget", "learnings.cap"}
		if got := diagPathsWithCode(snap, CodeInertSetting); !reflect.DeepEqual(got, want) {
			t.Errorf("inert-setting paths = %v, want %v", got, want)
		}
		for _, d := range snap.Diagnostics {
			if d.Code == CodeInertSetting {
				if d.Severity != SeverityInfo || d.Classification != Inert {
					t.Errorf("%s: %s/%s, want info/inert", d.Path, d.Severity, d.Classification)
				}
			}
		}
	})

	t.Run("learnings enabled carries one standing notice", func(t *testing.T) {
		snap := mustSnapshot(t)
		got := diagPathsWithCode(snap, CodeDeferredSetting)
		if !reflect.DeepEqual(got, []string{"learnings.enabled"}) {
			t.Fatalf("deferred-setting paths = %v, want exactly the learnings notice", got)
		}
		if len(snap.Capabilities) != 0 {
			t.Errorf("a default configuration declares nothing, so it classifies to nothing; got %+v", snap.Capabilities)
		}
	})

	t.Run("learnings disabled drops the notice", func(t *testing.T) {
		snap := mustSnapshot(t, srcR("learnings:\n  enabled: false\n"))
		if got := diagPathsWithCode(snap, CodeDeferredSetting); len(got) != 0 {
			t.Errorf("deferred-setting paths = %v, want none", got)
		}
	})
}

// TestClassifyReasonsAndRemedies keeps the prose fields honest: every entry
// says what its classification means, and every entry a user can act on names
// the edit.
func TestClassifyReasonsAndRemedies(t *testing.T) {
	snap := mustSnapshot(t,
		srcR("runtime:\n  bash: /bin/bash\nauto_groom: true\nlearnings:\n  cap: 5\n"),
	)
	if len(snap.Capabilities) != 3 {
		t.Fatalf("capabilities = %+v, want three", gotCaps(snap.Capabilities))
	}
	for _, c := range snap.Capabilities {
		if c.Reason == "" {
			t.Errorf("%s: every capability must state what its classification means", c.Path)
		}
		if c.MutationBlock && c.Remedy == "" {
			t.Errorf("%s: a blocking capability must name the edit that clears it", c.Path)
		}
		if c.Provenance.Source != ".docket.yml" || c.Provenance.Line == 0 {
			t.Errorf("%s: provenance = %+v, want the declaring file and line", c.Path, c.Provenance)
		}
	}
}

// TestClassifyFencedDeclarationsAreNotCapabilities: a fenced declaration is no
// declaration at all, so it cannot be a capability request either — the fence
// warning is the whole report.
func TestClassifyFencedDeclarationsAreNotCapabilities(t *testing.T) {
	snap := mustSnapshot(t, srcL("terminal_publish: true\nfinalize:\n  skip_results_only_delta: true\n"))
	if len(snap.Capabilities) != 0 {
		t.Errorf("capabilities = %+v, want none", gotCaps(snap.Capabilities))
	}
	if got := blockerPaths(snap); got != nil {
		t.Errorf("blocker paths = %v, want none", got)
	}
	if got := diagPathsWithCode(snap, CodeFencedIgnored); len(got) != 2 {
		t.Errorf("fenced-setting-ignored paths = %v, want both declarations reported", got)
	}
}
