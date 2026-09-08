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
