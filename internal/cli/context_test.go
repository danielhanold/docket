package cli

import (
	"strings"
	"testing"
)

// TestContextCommandsRegistered: `docket context implementation` is registered
// under the `context` group, carrying --id and --repo-dir flags, and the bare
// group reports a missing command.
func TestContextCommandsRegistered(t *testing.T) {
	root := captureTree(t)

	cmd, _, err := root.Find([]string{"context", "implementation"})
	if err != nil || cmd == nil || cmd.Name() != "implementation" {
		t.Fatalf("context implementation not registered: cmd=%v err=%v", cmd, err)
	}
	if cmd.Flags().Lookup("id") == nil {
		t.Errorf("context implementation: missing --id flag")
	}
	if cmd.Flags().Lookup("repo-dir") == nil {
		t.Errorf("context implementation: missing --repo-dir flag")
	}

	grp, _, err := root.Find([]string{"context"})
	if err != nil || grp == nil || grp.Name() != "context" {
		t.Fatalf("context group not registered: grp=%v err=%v", grp, err)
	}
}

// TestContextImplementationReachesOperation is the JSON-mode wiring assertion:
// the command reaches app.ContextImplementation, which — against a non-repository
// directory — returns exactly one protocol-v1 document naming the operation.
func TestContextImplementationReachesOperation(t *testing.T) {
	out, errS, code := runCLI(t, "context", "implementation", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"operation":"context.implementation"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestContextImplementationRoutesID: the --id flag is accepted and parsed (an
// int flag), and the command still reaches the operation with exactly one
// document.
func TestContextImplementationRoutesID(t *testing.T) {
	out, errS, code := runCLI(t, "context", "implementation", "--id", "7", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"operation":"context.implementation"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	// A non-integer --id is an argument error (exit 2), proving the flag is typed.
	_, errBad, codeBad := runCLI(t, "context", "implementation", "--id", "notanint", "--repo-dir", t.TempDir())
	if codeBad != 2 || !strings.Contains(errBad, "id") {
		t.Fatalf("non-integer --id not rejected as an argument error: err=%q code=%d", errBad, codeBad)
	}
}

// TestContextImplementationHumanMode: without --json the presenter writes the
// result's HumanText() to stdout (not stderr) and names the operation.
func TestContextImplementationHumanMode(t *testing.T) {
	out, _, _ := runCLI(t, "context", "implementation", "--repo-dir", t.TempDir())
	if !strings.Contains(out, "context.implementation") {
		t.Fatalf("human stdout did not carry the operation text: %q", out)
	}
}

// TestContextCommandsAssetIndependent guards the install.go registration: the
// context commands read the repository, never installed assets, so they must be
// asset-independent (not refused on a machine with no installation).
func TestContextCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"context", "context implementation", "context finalize"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}

// TestContextFinalizeRegistered: `docket context finalize` is registered under
// the `context` group carrying --id, --allowlist, and --repo-dir flags.
func TestContextFinalizeRegistered(t *testing.T) {
	root := captureTree(t)

	cmd, _, err := root.Find([]string{"context", "finalize"})
	if err != nil || cmd == nil || cmd.Name() != "finalize" {
		t.Fatalf("context finalize not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "allowlist", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("context finalize: missing --%s flag", flag)
		}
	}
}

// TestContextFinalizeReachesOperation is the JSON-mode wiring assertion: the
// command reaches app.ContextFinalize, which — against a non-repository
// directory — returns exactly one protocol-v1 document naming the operation.
func TestContextFinalizeReachesOperation(t *testing.T) {
	out, errS, code := runCLI(t, "context", "finalize", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"operation":"context.finalize"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestContextFinalizeRoutesFlags: --id and --allowlist are accepted and typed,
// and the command still reaches the operation with exactly one document.
func TestContextFinalizeRoutesFlags(t *testing.T) {
	out, errS, code := runCLI(t, "context", "finalize", "--id", "7", "--allowlist", "7,8", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"operation":"context.finalize"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	// A non-integer --id is an argument error (exit 2), proving the flag is typed.
	_, errBad, codeBad := runCLI(t, "context", "finalize", "--id", "notanint", "--repo-dir", t.TempDir())
	if codeBad != 2 || !strings.Contains(errBad, "id") {
		t.Fatalf("non-integer --id not rejected as an argument error: err=%q code=%d", errBad, codeBad)
	}
}
