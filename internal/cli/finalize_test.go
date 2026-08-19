package cli

import (
	"strings"
	"testing"
)

// TestFinalizeRetargetChildrenRegistered proves the subcommand is wired under the
// finalize group carrying the scalar --id/--version identity flags and the
// --input request-file flag (the authored authorization set rides in the file,
// never shell-escaped flags), plus --repo-dir.
func TestFinalizeRetargetChildrenRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"finalize", "retarget-children"})
	if err != nil || cmd == nil || cmd.Name() != "retarget-children" {
		t.Fatalf("finalize retarget-children not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"id", "version", "input", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("finalize retarget-children: missing --%s flag", flag)
		}
	}
}

// TestFinalizeRetargetChildrenAssetIndependent guards the install.go registration:
// the command reads the repository, never installed assets.
func TestFinalizeRetargetChildrenAssetIndependent(t *testing.T) {
	if !assetIndependent["finalize retarget-children"] {
		t.Errorf("%q is not registered asset-independent", "finalize retarget-children")
	}
}

// TestFinalizeRetargetChildrenFlagsRequired proves --id, --version, and --input are
// required: omitting them is an argument error (exit 2) before any operation runs.
func TestFinalizeRetargetChildrenFlagsRequired(t *testing.T) {
	_, errS, code := runCLI(t, "finalize", "retarget-children")
	if code != 2 || errS == "" {
		t.Fatalf("err=%q code=%d, want a required-flag argument error", errS, code)
	}
	for _, flag := range []string{"id", "version", "input"} {
		if !strings.Contains(errS, flag) {
			t.Errorf("required-flag error does not name %q: %q", flag, errS)
		}
	}
}

// TestFinalizeRetargetChildrenReachesOperation proves the command decodes its flags
// and --input body and reaches the operation, which returns exactly one protocol-v1
// document naming it. A bare tempdir is no docket repo, so the operation fails past
// its shape check — but only after naming itself.
func TestFinalizeRetargetChildrenReachesOperation(t *testing.T) {
	out, errS, _ := runCLIStdin(t, `{"children":[{"id":81,"pr_number":810,"pr_version":"cv810"}]}`,
		"finalize", "retarget-children",
		"--id", "80", "--version", "1234123412341234123412341234123412341234",
		"--input", "-", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"finalize.retarget-children"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestFinalizeRetargetChildrenUnknownFieldRejected proves --input decodes with
// DisallowUnknownFields: an unknown JSON key (e.g. a scalar identity that belongs
// on a flag, not in the authored set) is an argument error (exit 2).
func TestFinalizeRetargetChildrenUnknownFieldRejected(t *testing.T) {
	_, errS, code := runCLIStdin(t, `{"id":80,"children":[]}`,
		"finalize", "retarget-children",
		"--id", "80", "--version", "1234123412341234123412341234123412341234",
		"--input", "-", "--json")
	if code != 2 || errS != "" {
		t.Fatalf("err=%q code=%d, want an invalid-input argument error", errS, code)
	}
}

// TestFinalizeGroupRegistered: the top-level `finalize` command group is
// registered and reports a missing command when invoked bare, so the
// terminal-half mutation subcommands added in later tasks have a tree to attach
// to.
func TestFinalizeGroupRegistered(t *testing.T) {
	root := captureTree(t)

	grp, _, err := root.Find([]string{"finalize"})
	if err != nil || grp == nil || grp.Name() != "finalize" {
		t.Fatalf("finalize group not registered: grp=%v err=%v", grp, err)
	}
}

// TestFinalizeGroupBareReportsMissingCommand: `docket finalize` with no
// subcommand is a missing-command argument error (exit 2), not a silent success.
func TestFinalizeGroupBareReportsMissingCommand(t *testing.T) {
	_, errS, code := runCLI(t, "finalize")
	if code != 2 {
		t.Fatalf("bare finalize exit code = %d, want 2", code)
	}
	if !strings.Contains(errS, "missing command") {
		t.Fatalf("bare finalize stderr = %q, want a missing-command error", errS)
	}
}

// TestFinalizeGroupAssetIndependent guards the install.go registration: the
// finalize commands operate on the repository, never installed assets, so the
// group must be asset-independent.
func TestFinalizeGroupAssetIndependent(t *testing.T) {
	if !assetIndependent["finalize"] {
		t.Errorf("%q is not registered asset-independent", "finalize")
	}
}
