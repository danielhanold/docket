package repoguard

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
)

// committedHarnesses returns the agent_harnesses the committed .docket.yml
// declares. Only the repository layer is read: the committed AGENTS.md renders
// from the committed declaration, and a machine-local or global layer must not
// change what this guard expects of a checked-in file.
func committedHarnesses(t *testing.T) map[string]bool {
	t.Helper()
	src := config.Source{Layer: config.LayerRepository, Name: ".docket.yml",
		Data: []byte(readMaintained(t, guardRoot(t), ".docket.yml"))}
	snap, _, err := config.Resolve([]config.Source{src}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("resolve committed .docket.yml: %v", err)
	}
	out := map[string]bool{}
	for _, h := range snap.Effective.AgentHarnesses.Value {
		out[h] = true
	}
	return out
}

// requireCodex skips a guard on the Codex clause of the committed AGENTS.md
// when the repository does not enable the codex harness: the reposeed plan
// renders that clause only for a codex opt-in.
func requireCodex(t *testing.T) {
	t.Helper()
	if !committedHarnesses(t)["codex"] {
		t.Skip("codex is not in the committed agent_harnesses; AGENTS.md carries no Codex dispatch clause")
	}
}

// Docket dogfoods its dispatch route through its checked-in always-loaded
// policy. Testing the renderer alone would leave a stale instruction in that
// file green. Replacing the block must be a byte-identical no-op. The block
// carries the Codex clause iff codex is enabled, mirroring the reposeed plan;
// with neither codex nor opencode enabled the block is not planned at all.
func TestCommittedCodexDispatchMatchesGenerator(t *testing.T) {
	harnesses := committedHarnesses(t)
	if !harnesses["codex"] && !harnesses["opencode"] {
		t.Skip("neither codex nor opencode is in the committed agent_harnesses; AGENTS.md carries no dispatch block")
	}
	src := []byte(readMaintained(t, guardRoot(t), "AGENTS.md"))
	doc, err := document.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := harness.RunTracker(catalog)
	if err != nil {
		t.Fatal(err)
	}
	interior := harness.DispatchInterior(gate)
	if harnesses["codex"] {
		interior = harness.CodexDispatchInterior(gate)
	}
	var patch document.PatchSet
	patch.ReplaceBlock("dispatch", interior)
	want, err := doc.Apply(patch)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, want) {
		t.Fatal("AGENTS.md dispatch block is stale; regenerate it from the reposeed dispatch interior")
	}
}

// TestCommittedCodexDispatchRoutesEveryScope catches an AGENTS.md update that
// keeps the root coordinator path but loses the feature-worktree or metadata
// branch of the generated marker policy.
func TestCommittedCodexDispatchRoutesEveryScope(t *testing.T) {
	requireCodex(t)
	content := readMaintained(t, guardRoot(t), "AGENTS.md")
	for _, clause := range []string{
		"`[docket launch: root-coordinator]` takes precedence",
		"foreground catalog-resolved `agent.enter` at the caller cwd",
		"`[docket worktree: feature]` requires foreground catalog-resolved `agent.enter`",
		"exact `--worktree`; an unmarked metadata child uses direct native named-agent dispatch",
	} {
		if !strings.Contains(content, clause) {
			t.Errorf("AGENTS.md Codex dispatch policy lacks %q", clause)
		}
	}
}

// TestCommittedCodexDispatchObservesYieldedEntrySession catches a parent that
// mistakes a shell-tool liveness yield for agent.enter's terminal return and
// advances while the original foreground task is still running.
func TestCommittedCodexDispatchObservesYieldedEntrySession(t *testing.T) {
	requireCodex(t)
	content := readMaintained(t, guardRoot(t), "AGENTS.md")
	for _, clause := range []string{
		"shell-tool yield carrying a live task/session identity is a liveness transition, not completion",
		"retain that exact task/session identity and collect its terminal exit and final output through the harness-native observation/wait mechanism",
		"Never re-run `agent.enter`, start a second watcher, or return a completion report while the original task remains live or unobserved",
		"Only after terminal output is collected may implement-next run the parent's keyed `run.verdict`",
	} {
		if !strings.Contains(content, clause) {
			t.Errorf("AGENTS.md Codex dispatch policy lacks yielded-session barrier %q", clause)
		}
	}
}

// TestCodexLaunchMatrixOperatorProse keeps the executable operator guidance
// aligned with the typed Codex entry boundary: root coordinators retain the
// caller cwd, feature children enter their verified worktree with an unchanged
// payload, and only metadata children use native named-agent dispatch. The
// clauses intentionally name route markers and scope types, never a roster of
// roles, so the guard follows the inventory-owned abstraction.
func TestCodexLaunchMatrixOperatorProse(t *testing.T) {
	root := guardRoot(t)
	for _, contract := range []struct {
		file    string
		present []string
		absent  []string
	}{
		{
			file: "docs/install/codex.md",
			present: []string{
				"`[docket launch: root-coordinator]` → foreground `agent.enter` at the\ncaller's cwd",
				"`[docket worktree: feature]` → foreground `agent.enter` with a verified canonical\n`--worktree` and the unchanged structured payload",
				"unmarked metadata-scoped ordinary child →\nnative named-agent dispatch",
			},
			absent: []string{
				"nested dispatch uses Codex's direct named-agent dispatch from the active top-level\ntool surface",
			},
		},
		{
			file: "skills/docket-convention/references/agent-layer.md",
			present: []string{
				"Root-coordinator entry starts its root thread at the caller's absolute\ncwd",
				"Feature-child entry validates `--worktree`, then starts its root thread at the verified canonical\nfeature-worktree root",
				"both the process and thread cwd",
			},
			absent: []string{
				"starts a root thread with the caller's absolute cwd, approval policy, and\nsandbox, and passes an unchanged request file as the root turn. A feature role carries",
			},
		},
		{
			file: "docs/reference/harness/validation-runbook.md",
			present: []string{
				"Metadata-scoped ordinary child roles may continue to use direct registered-agent invocation.",
			},
			absent: []string{
				"Ordinary\nMetadata-scoped ordinary child roles",
			},
		},
	} {
		content := readMaintained(t, root, contract.file)
		for _, clause := range contract.present {
			if !strings.Contains(content, clause) {
				t.Errorf("%s lacks Codex launch-matrix clause %q", contract.file, clause)
			}
		}
		for _, retired := range contract.absent {
			if strings.Contains(content, retired) {
				t.Errorf("%s retains obsolete Codex launch-matrix clause %q", contract.file, retired)
			}
		}
	}
}
