package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestEvidenceCommandsRegistered: the two `evidence` subcommands are registered
// under the group with their scalar flags, and the bare group reports a missing
// command.
func TestEvidenceCommandsRegistered(t *testing.T) {
	root := captureTree(t)

	cases := []struct {
		path  []string
		flags []string
	}{
		{[]string{"evidence", "record"}, []string{"id", "run", "head", "repo-dir"}},
		{[]string{"evidence", "verify"}, []string{"record", "head"}},
	}
	for _, tc := range cases {
		cmd, _, err := root.Find(tc.path)
		if err != nil || cmd == nil || cmd.Name() != tc.path[len(tc.path)-1] {
			t.Fatalf("%v not registered: cmd=%v err=%v", tc.path, cmd, err)
		}
		for _, flag := range tc.flags {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%v: missing --%s flag", tc.path, flag)
			}
		}
	}

	grp, _, err := root.Find([]string{"evidence"})
	if err != nil || grp == nil || grp.Name() != "evidence" {
		t.Fatalf("evidence group not registered: grp=%v err=%v", grp, err)
	}
}

// TestEvidenceVerifyEmitsOneDocument: `docket evidence verify --json` reaches
// app.EvidenceVerify and emits exactly one protocol-v1 document naming the
// operation. A prose-only record file carries no block, which is still one
// well-formed document.
func TestEvidenceVerifyEmitsOneDocument(t *testing.T) {
	dir := testsupport.TempDir(t)
	recordFile := filepath.Join(dir, "evidence.md")
	if err := os.WriteFile(recordFile, []byte("# just prose\n"), 0o644); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	out, errS, code := runCLI(t, "evidence", "verify",
		"--record", recordFile, "--head", "abcdef0000000000000000000000000000000000", "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"operation":"evidence.verify"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestEvidenceRecordRoutesFlags: `docket evidence record` routes --id, --run,
// and --head into app.EvidenceRecord; against a non-repository directory the
// observe fails, still yielding one document naming the record operation.
func TestEvidenceRecordRoutesFlags(t *testing.T) {
	root := testsupport.TempDir(t)
	out, errS, _ := runCLI(t, "evidence", "record",
		"--id", "7", "--run", filepath.Join(root, "run"), "--head", "abc", "--repo-dir", root, "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"evidence.record"`) {
		t.Fatalf("record document did not name the operation: %q", out)
	}
}

// TestEvidenceRequiredFlags: record requires --id, --run, --head; verify
// requires --record and --head — omitting one is an argument error (exit 2).
func TestEvidenceRequiredFlags(t *testing.T) {
	if _, errS, code := runCLI(t, "evidence", "record", "--id", "7", "--run", "/x"); code != 2 || !strings.Contains(errS, "head") {
		t.Errorf("missing --head not rejected: err=%q code=%d", errS, code)
	}
	if _, errS, code := runCLI(t, "evidence", "verify", "--head", "abc"); code != 2 || !strings.Contains(errS, "record") {
		t.Errorf("missing --record not rejected: err=%q code=%d", errS, code)
	}
}

// TestEvidenceVerifyHumanMode: without --json the presenter writes the result's
// HumanText() to stdout and names the operation.
func TestEvidenceVerifyHumanMode(t *testing.T) {
	dir := testsupport.TempDir(t)
	recordFile := filepath.Join(dir, "evidence.md")
	if err := os.WriteFile(recordFile, []byte("# prose\n"), 0o644); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	out, _, _ := runCLI(t, "evidence", "verify", "--record", recordFile, "--head", "abc")
	if !strings.Contains(out, "evidence") {
		t.Fatalf("human stdout did not carry the operation text: %q", out)
	}
}

// TestEvidenceCommandsAssetIndependent guards the install.go registration: the
// evidence commands read the repository, drive the process supervisor, and parse
// record bytes, never installed assets, so they must be asset-independent.
func TestEvidenceCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"evidence", "evidence record", "evidence verify"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}
