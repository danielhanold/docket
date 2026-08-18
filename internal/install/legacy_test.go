package install

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacyHarnesses is the four shipped harnesses with a named v0.9.2 emitter —
// the harness dimension of the frozen corpus.
var legacyHarnesses = []string{"claude", "codex", "cursor", "opencode"}

// legacyShapes is the four captured pin shapes.
var legacyShapes = []string{"default", "fully-pinned", "partially-pinned", "unpinned"}

// legacyAgentDirName maps a harness token to the on-disk directory that holds
// its user-level agent definitions, matching the real install target paths the
// harness adapters emit: three hang a dotted dir off the home directory, while
// opencode's root is an undotted dir under the XDG config home.
var legacyAgentDirName = map[string]string{
	"claude":   ".claude",
	"codex":    ".codex",
	"cursor":   ".cursor",
	"opencode": "opencode",
}

// pinsForShape builds the resolved (model, effort) pins the v0.9.2 emitters were
// fed for each captured shape, transcribed from testdata/legacy/README.md's
// shape table. The synthetic shapes are uniform across every harness and both
// agents; only `default` varies per harness/agent (the realistic install).
func pinsForShape(shape string) map[string]AgentPin {
	switch shape {
	case "default":
		return map[string]AgentPin{
			"status": {ByHarness: map[string]HarnessPin{
				"claude":   {Model: "claude-haiku-4-5-20251001", Effort: "medium"},
				"cursor":   {Model: "cursor-grok-4.5-low-fast", Effort: "auto"},
				"codex":    {Model: "gpt-5.6-luna", Effort: "xhigh"},
				"opencode": {Model: "openrouter/deepseek/deepseek-v4-flash-0731", Effort: "low"},
			}},
			"brainstorm-consultant": {ByHarness: map[string]HarnessPin{
				"claude":   {Model: "claude-opus-5", Effort: "medium"},
				"cursor":   {Model: "cursor-grok-4.5-high", Effort: "auto"},
				"codex":    {Model: "gpt-5.6-sol", Effort: "medium"},
				"opencode": {Model: "openrouter/moonshotai/kimi-k3", Effort: "medium"},
			}},
		}
	default:
		var model, effort string
		switch shape {
		case "fully-pinned":
			model, effort = "legacy-pinned-model", "high"
		case "partially-pinned":
			model, effort = "legacy-pinned-model", "auto"
		case "unpinned":
			model, effort = "inherit", "auto"
		default:
			panic("unknown shape " + shape)
		}
		uniform := func() AgentPin {
			by := map[string]HarnessPin{}
			for _, h := range legacyHarnesses {
				by[h] = HarnessPin{Model: model, Effort: effort}
			}
			return AgentPin{ByHarness: by}
		}
		return map[string]AgentPin{
			"status":                uniform(),
			"brainstorm-consultant": uniform(),
		}
	}
}

func legacyCorpusDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("testdata", "legacy")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("legacy corpus dir missing: %v", err)
	}
	return dir
}

// TestLegacyReproducer_NativeAgents covers every captured (harness, shape,
// agent) golden: it builds the matching LegacyInputs, calls the reproducer with
// the corresponding install-target path, and asserts the reproduced bytes equal
// the frozen golden byte-for-byte.
func TestLegacyReproducer_NativeAgents(t *testing.T) {
	corpus := legacyCorpusDir(t)
	covered := 0
	for _, harness := range legacyHarnesses {
		for _, shape := range legacyShapes {
			agentsDir := filepath.Join(corpus, harness, shape, "agents")
			entries, err := os.ReadDir(agentsDir)
			if err != nil {
				t.Fatalf("reading %s: %v", agentsDir, err)
			}
			inputs := LegacyInputs{
				Harnesses: append([]string(nil), legacyHarnesses...),
				AgentPins: pinsForShape(shape),
			}
			rep := NewLegacyReproducer(inputs)
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				golden, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
				if err != nil {
					t.Fatalf("reading golden %s: %v", e.Name(), err)
				}
				targetPath := filepath.Join("/legacyroot", legacyAgentDirName[harness], "agents", e.Name())
				got, ok := rep(Target{Path: targetPath, Kind: KindFile, Role: roleAgent})
				if !ok {
					t.Errorf("%s/%s/%s: reproducer reported no legacy spelling for %s",
						harness, shape, e.Name(), targetPath)
					continue
				}
				if !bytes.Equal(got, golden) {
					t.Errorf("%s/%s/%s: bytes differ:\n%s", harness, shape, e.Name(), firstDiff(got, golden))
					continue
				}
				covered++
			}
		}
	}
	// Four harnesses x four shapes x two agents = 32 captured goldens.
	if covered != 32 {
		t.Fatalf("expected to cover 32 goldens, covered %d", covered)
	}
}

// TestLegacyReproducer_NonInventory proves the reproducer refuses everything
// outside the closed inventory with (nil, false): a harness not in the input
// set, a non-file kind, a path that is not an agent definition, an unknown
// agent short-name, and an extension that does not match the harness.
func TestLegacyReproducer_NonInventory(t *testing.T) {
	full := LegacyInputs{Harnesses: append([]string(nil), legacyHarnesses...), AgentPins: pinsForShape("default")}

	cases := []struct {
		name   string
		inputs LegacyInputs
		target Target
	}{
		{
			name:   "harness absent from input set",
			inputs: LegacyInputs{Harnesses: []string{"codex"}, AgentPins: pinsForShape("default")},
			target: Target{Path: "/legacyroot/.claude/agents/docket-status.md", Kind: KindFile, Role: roleAgent},
		},
		{
			name:   "non-file kind",
			inputs: full,
			target: Target{Path: "/legacyroot/.claude/CLAUDE.md", Kind: KindManagedBlock, BlockName: "dispatch", Role: "dispatch"},
		},
		{
			name:   "not an agent-definition path",
			inputs: full,
			target: Target{Path: "/legacyroot/.cursor/rules/docket-dispatch.mdc", Kind: KindFile, Role: "dispatch"},
		},
		{
			name:   "unknown agent short-name",
			inputs: full,
			target: Target{Path: "/legacyroot/.claude/agents/docket-nonexistent.md", Kind: KindFile, Role: roleAgent},
		},
		{
			name:   "extension does not match harness (codex must be .toml)",
			inputs: full,
			target: Target{Path: "/legacyroot/.codex/agents/docket-status.md", Kind: KindFile, Role: roleAgent},
		},
		{
			name:   "agent path but no docket- prefix",
			inputs: full,
			target: Target{Path: "/legacyroot/.claude/agents/status.md", Kind: KindFile, Role: roleAgent},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := NewLegacyReproducer(tc.inputs)
			got, ok := rep(tc.target)
			if ok || got != nil {
				t.Fatalf("expected (nil,false) for %q, got ok=%v len(bytes)=%d", tc.target.Path, ok, len(got))
			}
		})
	}
}

// TestLegacyReproducer_EmbedsAllBuiltins proves every v0.9.2 built-in agent
// body is embedded, not just the two with goldens: each of the 16 short-names
// reproduces on the claude harness.
func TestLegacyReproducer_EmbedsAllBuiltins(t *testing.T) {
	want := []string{
		"adr", "auto-groom-critic", "auto-groom", "brainstorm-consultant",
		"build-economy", "build-max", "build-premium", "build-standard",
		"finalize-change", "implement-next", "integration-repair",
		"rebase-resolver", "review-deep", "review-lean", "review-standard", "status",
	}
	rep := NewLegacyReproducer(LegacyInputs{Harnesses: []string{"claude"}})
	for _, short := range want {
		path := filepath.Join("/legacyroot", ".claude", "agents", "docket-"+short+".md")
		got, ok := rep(Target{Path: path, Kind: KindFile, Role: roleAgent})
		if !ok || len(got) == 0 {
			t.Errorf("built-in %q: reproducer returned ok=%v len=%d", short, ok, len(got))
		}
	}
}

// firstDiff renders the first differing line between two byte slices.
func firstDiff(got, want []byte) string {
	gl := strings.SplitAfter(string(got), "\n")
	wl := strings.SplitAfter(string(want), "\n")
	n := len(gl)
	if len(wl) < n {
		n = len(wl)
	}
	for i := 0; i < n; i++ {
		if gl[i] != wl[i] {
			return fmt.Sprintf("first diff at line %d:\n  got:  %q\n  want: %q", i+1, gl[i], wl[i])
		}
	}
	if len(gl) != len(wl) {
		return fmt.Sprintf("line counts differ: got %d, want %d (got len=%d want len=%d)", len(gl), len(wl), len(got), len(want))
	}
	return "no line diff (byte-length equal? got vs want lengths differ)"
}
