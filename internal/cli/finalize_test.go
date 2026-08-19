package cli

import (
	"strings"
	"testing"
)

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
