package cli

import (
	"strings"
	"testing"
)

// TestRunCommandsRegistered: the `run verify` subcommand is registered under the
// group with its scalar flags, and the bare group reports a missing command.
func TestRunCommandsRegistered(t *testing.T) {
	root := captureTree(t)

	cmd, _, err := root.Find([]string{"run", "verify"})
	if err != nil || cmd == nil || cmd.Name() != "verify" {
		t.Fatalf("run verify not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("run verify: missing --%s flag", flag)
		}
	}

	grp, _, err := root.Find([]string{"run"})
	if err != nil || grp == nil || grp.Name() != "run" {
		t.Fatalf("run group not registered: grp=%v err=%v", grp, err)
	}
}

// TestRunVerifyEmitsOneDocument: `docket run verify --json` reaches app.RunVerify
// and emits exactly one protocol-v1 document naming the operation. Pointed at a
// non-repository directory the operation refuses early, which is still one
// well-formed document that names run.verify.
func TestRunVerifyEmitsOneDocument(t *testing.T) {
	dir := t.TempDir()
	out, errS, _ := runCLI(t, "run", "verify", "--id", "7", "--repo-dir", dir, "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"run.verify"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestRunVerifyHumanMode: without --json the presenter writes the result's
// HumanText() and names the operation.
func TestRunVerifyHumanMode(t *testing.T) {
	dir := t.TempDir()
	out, _, _ := runCLI(t, "run", "verify", "--id", "7", "--repo-dir", dir)
	if !strings.Contains(out, "run.verify") && !strings.Contains(out, "run verify") {
		t.Fatalf("human stdout did not carry the operation text: %q", out)
	}
}

// TestRunVerifyRequiredFlags: verify requires --id; omitting it is an argument
// error (exit 2).
func TestRunVerifyRequiredFlags(t *testing.T) {
	if _, errS, code := runCLI(t, "run", "verify"); code != 2 || !strings.Contains(errS, "id") {
		t.Errorf("missing --id not rejected: err=%q code=%d", errS, code)
	}
}

// TestRunCommandsAssetIndependent guards the install.go registration: run verify
// reads the repository, the workspace, and GitHub, never installed assets, so it
// must be asset-independent.
func TestRunCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"run", "run verify"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}
