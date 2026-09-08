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
