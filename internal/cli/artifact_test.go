package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArtifactCommandsRegistered: `docket artifact backlink` is registered under
// the `artifact` group with its three flags, and the bare group reports a missing
// command.
func TestArtifactCommandsRegistered(t *testing.T) {
	root := captureTree(t)

	cmd, _, err := root.Find([]string{"artifact", "backlink"})
	if err != nil || cmd == nil || cmd.Name() != "backlink" {
		t.Fatalf("artifact backlink not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"artifact", "change", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("artifact backlink: missing --%s flag", flag)
		}
	}

	grp, _, err := root.Find([]string{"artifact"})
	if err != nil || grp == nil || grp.Name() != "artifact" {
		t.Fatalf("artifact group not registered: grp=%v err=%v", grp, err)
	}
}

// TestArtifactBacklinkEmitsOneDocument: `docket artifact backlink --json` reaches
// app.ArtifactBacklink and emits exactly one protocol-v1 document naming the
// operation. Against an existing artifact in a non-repository directory the pin
// fails, which is still one well-formed document.
func TestArtifactBacklinkEmitsOneDocument(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte("# Plan\n"), 0o644); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	out, errS, code := runCLI(t, "artifact", "backlink",
		"--artifact", "plan.md", "--change", "docs/changes/active/0315-claim.md",
		"--repo-dir", root, "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"operation":"artifact.backlink"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestArtifactBacklinkRequiredFlags: --artifact and --change are required, so
// omitting one is an argument error (exit 2).
func TestArtifactBacklinkRequiredFlags(t *testing.T) {
	_, errS, code := runCLI(t, "artifact", "backlink", "--change", "docs/changes/active/0315-claim.md")
	if code != 2 || !strings.Contains(errS, "artifact") {
		t.Fatalf("missing --artifact not rejected as an argument error: err=%q code=%d", errS, code)
	}
}

// TestArtifactBacklinkHumanMode: without --json the presenter writes the result's
// HumanText() to stdout and names the operation.
func TestArtifactBacklinkHumanMode(t *testing.T) {
	root := t.TempDir()
	out, _, _ := runCLI(t, "artifact", "backlink",
		"--artifact", "plan.md", "--change", "docs/changes/active/0315-claim.md", "--repo-dir", root)
	if !strings.Contains(out, "artifact") {
		t.Fatalf("human stdout did not carry the operation text: %q", out)
	}
}

// TestArtifactCommandsAssetIndependent guards the install.go registration: the
// artifact commands read the repository and a working-tree file, never installed
// assets, so they must be asset-independent.
func TestArtifactCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"artifact", "artifact backlink"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}
