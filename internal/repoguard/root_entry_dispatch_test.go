package repoguard

import (
	"bytes"
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
