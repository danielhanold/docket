package repoguard

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/harness"
)

// Docket dogfoods the Codex route through its checked-in always-loaded policy.
// Testing the renderer alone would leave a stale native-child instruction in
// that file green. Replacing the block must be a byte-identical no-op.
func TestCommittedCodexDispatchMatchesGenerator(t *testing.T) {
	src := []byte(readMaintained(t, guardRoot(t), "AGENTS.md"))
	doc, err := document.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := harness.RunGate(catalog)
	if err != nil {
		t.Fatal(err)
	}
	var patch document.PatchSet
	patch.ReplaceBlock("dispatch", harness.CodexDispatchInterior(gate))
	want, err := doc.Apply(patch)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, want) {
		t.Fatal("AGENTS.md dispatch block is stale; regenerate it from harness.CodexDispatchInterior")
	}
}

// TestCommittedCodexDispatchRoutesEveryScope catches an AGENTS.md update that
// keeps the root coordinator path but loses the feature-worktree or metadata
// branch of the generated marker policy.
func TestCommittedCodexDispatchRoutesEveryScope(t *testing.T) {
	content := readMaintained(t, guardRoot(t), "AGENTS.md")
	for _, clause := range []string{
		"every registered Docket role uses the harness's top-level native named-agent dispatch",
		"`[docket launch: root-coordinator]` and `[docket worktree: feature]` do not select `agent.enter`",
		"Pass the run epoch id to `docket-implement-next`",
		"Keep the caller's gate key and parent capability private",
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
	content := readMaintained(t, guardRoot(t), "AGENTS.md")
	for _, clause := range []string{
		"native dispatch yield carrying a live child identity is a liveness transition, not completion",
		"Retain that exact identity and collect its terminal output through the harness-native observation/wait control",
		"Never launch a replacement watcher or return a completion report while the original child remains live or unobserved",
		"Only after terminal return may implement-next run the parent's keyed `run.gate-verdict`",
	} {
		if !strings.Contains(content, clause) {
			t.Errorf("AGENTS.md Codex dispatch policy lacks yielded-session barrier %q", clause)
		}
	}
}

// TestCodexLaunchMatrixOperatorProse keeps the executable operator guidance
// aligned with the typed Codex entry boundary: every role uses the harness's
// top-level native named-agent dispatch, while feature ownership comes from an
// immutable assignment that the child validates before work. The clauses name
// route markers and scope types, never a roster of roles, so the guard follows
// the inventory-owned abstraction.
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
				"every generated wrapper dispatches further registered roles through Codex's top-level\nnative named-agent control",
				"Each child first validates its pinned assignment\nand role-specific resources",
				"does not fall\nback to `agent.enter`",
			},
			absent: []string{
				"foreground `agent.enter` with a verified canonical",
			},
		},
		{
			file: "skills/docket-convention/references/agent-layer.md",
			present: []string{
				"Codex dispatches every\nregistered role through the harness's top-level native named-agent control",
				"Feature children receive an immutable\nassignment naming the absolute canonical feature-worktree root",
				"validate that assignment before\nthey inspect or mutate the feature checkout",
			},
			absent: []string{
				"starts a root thread with the caller's absolute cwd, approval policy, and\nsandbox, and passes an unchanged request file as the root turn. A feature role carries",
				"agent.enter [",
			},
		},
		{
			file: "docs/reference/harness/validation-runbook.md",
			present: []string{
				"dispatch the registered coordinator through Codex's top-level native named-agent control",
				"Feature-scoped ordinary child roles use native named-agent dispatch and validate their explicit assignment",
				"dispatch the registered resolver through\n  Codex's top-level native named-agent control",
			},
			absent: []string{
				"Ordinary\nMetadata-scoped ordinary child roles",
				"docket agent enter",
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
