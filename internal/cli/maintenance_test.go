package cli

import (
	"strings"
	"testing"
)

// TestMaintenanceSweepRegistered proves the subcommand is wired under the
// maintenance group carrying only --repo-dir: the sweep takes no authored request
// body and no scalar identity, since it processes a whole pinned inventory.
func TestMaintenanceSweepRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"maintenance", "sweep"})
	if err != nil || cmd == nil || cmd.Name() != "sweep" {
		t.Fatalf("maintenance sweep not registered: cmd=%v err=%v", cmd, err)
	}
	if cmd.Flags().Lookup("repo-dir") == nil {
		t.Errorf("maintenance sweep: missing --repo-dir flag")
	}
}

// TestMaintenanceSweepAssetIndependent guards the install.go registration: the
// command reads the repository and drives Git/GitHub, never installed assets.
func TestMaintenanceSweepAssetIndependent(t *testing.T) {
	if !assetIndependent["maintenance sweep"] {
		t.Errorf("%q is not registered asset-independent", "maintenance sweep")
	}
	if !assetIndependent["maintenance"] {
		t.Errorf("%q group is not registered asset-independent", "maintenance")
	}
}

// TestMaintenanceGroupMissingCommand proves the bare group reports a missing
// command rather than doing anything, mirroring finalize and gate.
func TestMaintenanceGroupMissingCommand(t *testing.T) {
	_, errS, code := runCLI(t, "maintenance")
	if code == 0 || errS == "" {
		t.Fatalf("bare `maintenance` should error; err=%q code=%d", errS, code)
	}
}

// TestMaintenanceSweepReachesOperation proves the command decodes its flag and
// reaches the operation, which returns exactly one protocol-v1 document naming
// it. A bare tempdir is no docket repo, so the operation fails past its pin — but
// only after naming itself as maintenance.sweep.
func TestMaintenanceSweepReachesOperation(t *testing.T) {
	out, errS, _ := runCLI(t, "maintenance", "sweep", "--repo-dir", t.TempDir(), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"operation":"maintenance.sweep"`) {
		t.Fatalf("document did not name the operation: %q", out)
	}
	if !strings.Contains(out, `"protocol_version":1`) {
		t.Fatalf("missing protocol version: %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}
