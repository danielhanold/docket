package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestPRCommandsRegistered: the `pr publish` subcommand is registered under the
// group with its scalar/request flags, and the bare group reports a missing
// command.
func TestPRCommandsRegistered(t *testing.T) {
	root := captureTree(t)

	cmd, _, err := root.Find([]string{"pr", "publish"})
	if err != nil || cmd == nil || cmd.Name() != "publish" {
		t.Fatalf("pr publish not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "head", "body", "evidence", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("pr publish: missing --%s flag", flag)
		}
	}

	grp, _, err := root.Find([]string{"pr"})
	if err != nil || grp == nil || grp.Name() != "pr" {
		t.Fatalf("pr group not registered: grp=%v err=%v", grp, err)
	}
}

// TestPRPublishEmitsOneDocument: `docket pr publish --json` decodes the body
// request and evidence bytes, reaches app.PRPublish, and emits exactly one
// protocol-v1 document naming the operation. A prose-only evidence file does not
// verify, which is still one well-formed document that names pr.publish.
func TestPRPublishEmitsOneDocument(t *testing.T) {
	dir := testsupport.TempDir(t)
	bodyFile := filepath.Join(dir, "body.json")
	if err := os.WriteFile(bodyFile, []byte(`{"title":"Add widget","body":"Some prose.\n"}`), 0o644); err != nil {
		t.Fatalf("seed body: %v", err)
	}
	evFile := filepath.Join(dir, "evidence.md")
	if err := os.WriteFile(evFile, []byte("# just prose, no block\n"), 0o644); err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
	out, errS, _ := runCLI(t, "pr", "publish",
		"--id", "7",
		"--head", "1111111111111111111111111111111111111111",
		"--body", bodyFile,
		"--evidence", evFile,
		"--repo-dir", dir,
		"--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"pr.publish"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestPRPublishHumanMode: without --json the presenter writes the result's
// HumanText() and names the operation.
func TestPRPublishHumanMode(t *testing.T) {
	dir := testsupport.TempDir(t)
	bodyFile := filepath.Join(dir, "body.json")
	if err := os.WriteFile(bodyFile, []byte(`{"title":"Add widget","body":"prose"}`), 0o644); err != nil {
		t.Fatalf("seed body: %v", err)
	}
	evFile := filepath.Join(dir, "evidence.md")
	if err := os.WriteFile(evFile, []byte("# prose\n"), 0o644); err != nil {
		t.Fatalf("seed evidence: %v", err)
	}
	out, _, _ := runCLI(t, "pr", "publish",
		"--id", "7", "--head", "1111111111111111111111111111111111111111",
		"--body", bodyFile, "--evidence", evFile, "--repo-dir", dir)
	if !strings.Contains(out, "pr.publish") {
		t.Fatalf("human stdout did not carry the operation text: %q", out)
	}
}

// TestPRPublishRequiredFlags: publish requires --id, --head, --body, --evidence;
// omitting one is an argument error (exit 2).
func TestPRPublishRequiredFlags(t *testing.T) {
	dir := testsupport.TempDir(t)
	bodyFile := filepath.Join(dir, "body.json")
	_ = os.WriteFile(bodyFile, []byte(`{"title":"t","body":"b"}`), 0o644)
	evFile := filepath.Join(dir, "ev.md")
	_ = os.WriteFile(evFile, []byte("prose\n"), 0o644)

	if _, errS, code := runCLI(t, "pr", "publish", "--head", "abc", "--body", bodyFile, "--evidence", evFile); code != 2 || !strings.Contains(errS, "id") {
		t.Errorf("missing --id not rejected: err=%q code=%d", errS, code)
	}
	if _, errS, code := runCLI(t, "pr", "publish", "--id", "7", "--body", bodyFile, "--evidence", evFile); code != 2 || !strings.Contains(errS, "head") {
		t.Errorf("missing --head not rejected: err=%q code=%d", errS, code)
	}
	if _, errS, code := runCLI(t, "pr", "publish", "--id", "7", "--head", "abc", "--evidence", evFile); code != 2 || !strings.Contains(errS, "body") {
		t.Errorf("missing --body not rejected: err=%q code=%d", errS, code)
	}
	if _, errS, code := runCLI(t, "pr", "publish", "--id", "7", "--head", "abc", "--body", bodyFile); code != 2 || !strings.Contains(errS, "evidence") {
		t.Errorf("missing --evidence not rejected: err=%q code=%d", errS, code)
	}
}

// TestPRCommandsAssetIndependent guards the install.go registration: pr publish
// reads the repository, the workspace, and GitHub, never installed assets, so it
// must be asset-independent.
func TestPRCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"pr", "pr publish"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}
