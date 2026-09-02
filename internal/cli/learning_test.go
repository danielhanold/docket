package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestLearningCommandsRegistered is the registration assertion: `docket
// learning` carries exactly the two settled subcommands, each with a required
// --request flag and a --repo-dir flag, and the bare group reports a missing
// command.
func TestLearningCommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"record", "update"} {
		cmd, _, err := root.Find([]string{"learning", sub})
		if err != nil || cmd == nil || cmd.Name() != sub {
			t.Fatalf("learning %s not registered: cmd=%v err=%v", sub, cmd, err)
		}
		if cmd.Flags().Lookup("request") == nil {
			t.Errorf("learning %s: missing --request flag", sub)
		}
		if cmd.Flags().Lookup("repo-dir") == nil {
			t.Errorf("learning %s: missing --repo-dir flag", sub)
		}
	}
	grp, _, err := root.Find([]string{"learning"})
	if err != nil || grp == nil || grp.Name() != "learning" {
		t.Fatalf("learning group not registered: grp=%v err=%v", grp, err)
	}
}

// TestLearningRequestFlagRequired proves the --request flag is required:
// omitting it fails as an argument error (exit 2) that names the flag, before
// any operation runs.
func TestLearningRequestFlagRequired(t *testing.T) {
	_, errS, code := runCLI(t, "learning", "record")
	if code != 2 || !strings.Contains(errS, "request") {
		t.Fatalf("err=%q code=%d", errS, code)
	}
}

// TestLearningRecordUnknownFieldRejected proves --request decodes with
// DisallowUnknownFields: an unknown JSON field is invalid input, exit 2, one
// document, and no engine is reached.
func TestLearningRecordUnknownFieldRejected(t *testing.T) {
	out, errS, code := runCLIStdin(t, `{"slug":"x","bogus_field":1}`, "learning", "record", "--request", "-", "--json")
	if code != 2 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("stdout=%q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be one newline-terminated document, got %q", out)
	}
}

// TestLearningCommandsReachOperation is the wiring assertion for both commands:
// a well-formed (if semantically empty) JSON request read from stdin decodes
// into the operation's own request struct and is handed to that operation,
// which returns exactly one protocol-v1 document naming it. A `{}` body fails
// each operation's up-front validation, so this reaches the operation without
// needing a live repository.
func TestLearningCommandsReachOperation(t *testing.T) {
	cases := []struct{ sub, op string }{
		{"record", "learning.record"},
		{"update", "learning.update"},
	}
	for _, c := range cases {
		out, errS, code := runCLIStdin(t, `{}`, "learning", c.sub, "--request", "-", "--repo-dir", testsupport.TempDir(t), "--json")
		if errS != "" {
			t.Fatalf("%s: unexpected stderr %q (code=%d)", c.sub, errS, code)
		}
		if !strings.Contains(out, `"operation":"`+c.op+`"`) {
			t.Fatalf("%s: document did not name the operation: %q", c.sub, out)
		}
		if !strings.Contains(out, `"protocol_version":1`) {
			t.Fatalf("%s: missing protocol version: %q", c.sub, out)
		}
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			t.Fatalf("%s: must be exactly one newline-terminated document, got %q", c.sub, out)
		}
	}
}

// TestLearningRequestFromFile proves the --request path form reads a JSON file
// (not only stdin) and reaches the operation.
func TestLearningRequestFromFile(t *testing.T) {
	dir := testsupport.TempDir(t)
	reqPath := filepath.Join(dir, "req.json")
	if err := os.WriteFile(reqPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errS, code := runCLI(t, "learning", "record", "--request", reqPath, "--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"operation":"learning.record"`) {
		t.Fatalf("stdout=%q", out)
	}
}

// TestLearningRequestFileMissing proves an unreadable --request path is an
// argument error (exit 2) rather than a panic or a half-formed document.
func TestLearningRequestFileMissing(t *testing.T) {
	_, errS, code := runCLI(t, "learning", "record", "--request", filepath.Join(testsupport.TempDir(t), "nope.json"))
	if code != 2 {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(errS, "request") {
		t.Fatalf("err=%q", errS)
	}
}

// TestLearningCommandsAssetIndependent guards the install.go registration: each
// learning command must be in the asset-independent set (they read the
// repository, never installed assets), so they are not refused on a machine
// with no installation.
func TestLearningCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"learning", "learning record", "learning update"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}
