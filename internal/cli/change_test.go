package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLIStdin is runCLI with caller-supplied stdin, so a `--request -` command
// can be driven with an in-memory JSON body.
func runCLIStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(args, strings.NewReader(stdin), &out, &errBuf, devInfo(), hostFacts())
	return out.String(), errBuf.String(), code
}

// TestChangeCommandsRegistered is the registration assertion: `docket change`
// carries exactly the five settled subcommands, each with a required --request
// flag and a --repo-dir flag, and the bare group reports a missing command.
func TestChangeCommandsRegistered(t *testing.T) {
	root := captureTree(t)
	for _, sub := range []string{"create", "groom", "block", "defer", "kill"} {
		cmd, _, err := root.Find([]string{"change", sub})
		if err != nil || cmd == nil || cmd.Name() != sub {
			t.Fatalf("change %s not registered: cmd=%v err=%v", sub, cmd, err)
		}
		if cmd.Flags().Lookup("request") == nil {
			t.Errorf("change %s: missing --request flag", sub)
		}
		if cmd.Flags().Lookup("repo-dir") == nil {
			t.Errorf("change %s: missing --repo-dir flag", sub)
		}
	}
	// The group itself resolves to a command (not an unknown-command error).
	grp, _, err := root.Find([]string{"change"})
	if err != nil || grp == nil || grp.Name() != "change" {
		t.Fatalf("change group not registered: grp=%v err=%v", grp, err)
	}
}

// TestChangeRequestFlagRequired proves the --request flag is required: omitting
// it fails as an argument error (exit 2) that names the flag, before any
// operation runs.
func TestChangeRequestFlagRequired(t *testing.T) {
	_, errS, code := runCLI(t, "change", "create")
	if code != 2 || !strings.Contains(errS, "request") {
		t.Fatalf("err=%q code=%d", errS, code)
	}
}

// TestChangeCreateUnknownFieldRejected proves --request decodes with
// DisallowUnknownFields: an unknown JSON field is invalid input, exit 2, one
// document, and no engine is reached.
func TestChangeCreateUnknownFieldRejected(t *testing.T) {
	out, errS, code := runCLIStdin(t, `{"title":"x","bogus_field":1}`, "change", "create", "--request", "-", "--json")
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

// TestChangeCommandsReachOperation is the wiring assertion for all five
// commands: a well-formed (if semantically empty) JSON request read from stdin
// decodes into the operation's own request struct and is handed to that
// operation, which returns exactly one protocol-v1 document naming it. A `{}`
// body fails each operation's up-front request-shape validation, so this reaches
// the operation without needing a live repository.
func TestChangeCommandsReachOperation(t *testing.T) {
	cases := []struct{ sub, op string }{
		{"create", "change.create"},
		{"groom", "change.groom"},
		{"block", "change.block"},
		{"defer", "change.defer"},
		{"kill", "change.kill"},
	}
	for _, c := range cases {
		out, errS, code := runCLIStdin(t, `{}`, "change", c.sub, "--request", "-", "--repo-dir", t.TempDir(), "--json")
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

// TestChangeRequestFromFile proves the --request path form reads a JSON file
// (not only stdin) and reaches the operation.
func TestChangeRequestFromFile(t *testing.T) {
	dir := t.TempDir()
	reqPath := filepath.Join(dir, "req.json")
	if err := os.WriteFile(reqPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errS, code := runCLI(t, "change", "create", "--request", reqPath, "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	if !strings.Contains(out, `"operation":"change.create"`) {
		t.Fatalf("stdout=%q", out)
	}
}

// TestChangeRequestFileMissing proves an unreadable --request path is an
// argument error (exit 2) rather than a panic or a half-formed document.
func TestChangeRequestFileMissing(t *testing.T) {
	_, errS, code := runCLI(t, "change", "create", "--request", filepath.Join(t.TempDir(), "nope.json"))
	if code != 2 {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(errS, "request") {
		t.Fatalf("err=%q", errS)
	}
}

// TestChangeCommandsAssetIndependent guards the install.go registration: each
// change command must be in the asset-independent set (they read the repository,
// never installed assets), so they are not refused on a machine with no
// installation.
func TestChangeCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"change", "change create", "change groom", "change block", "change defer", "change kill"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}
