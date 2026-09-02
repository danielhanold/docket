package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
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

// TestMaintenancePreflightCommandWiring pins the WIRING of `maintenance
// preflight`: the command exists, resolves --repo-dir, builds the sweep deps and
// status reader, and returns app.MaintenancePreflight's result to the presenter.
// A bare tempdir is no docket repo, so the composition fails past its pin — but
// only after naming itself as maintenance.preflight and carrying the verdict
// field, which proves the result reached the presenter. The app-layer
// composition rules are already proven in internal/app; this is wiring only.
func TestMaintenancePreflightCommandWiring(t *testing.T) {
	out, _, _ := runCLI(t, "maintenance", "preflight", "--repo-dir", t.TempDir(), "--json")
	if !strings.Contains(out, `"operation":"maintenance.preflight"`) {
		t.Fatalf("preflight envelope missing: %s", out)
	}
	if !strings.Contains(out, `"preflight":`) {
		t.Fatalf("verdict field missing: %s", out)
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

// TestMaintenanceSweepScopeFlag: the closed --scope flag is registered with
// full as its omitted default, and the RESOLVED scope is echoed in the
// envelope — asserting the non-default value proves the wiring, since a
// defaulted parameter hides a dropped argument (learnings:
// defaulted-param-hides-caller-wiring).
func TestMaintenanceSweepScopeFlag(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"maintenance", "sweep"})
	if err != nil || cmd == nil {
		t.Fatalf("maintenance sweep not registered: %v", err)
	}
	f := cmd.Flags().Lookup("scope")
	if f == nil || f.DefValue != "full" {
		t.Fatalf("--scope must exist with default full, got %+v", f)
	}

	out, errS, _ := runCLI(t, "maintenance", "sweep", "--repo-dir", t.TempDir(), "--json")
	if errS != "" || !strings.Contains(out, `"scope":"full"`) {
		t.Errorf("omitted scope must resolve and echo full: out=%q err=%q", out, errS)
	}
	out, errS, _ = runCLI(t, "maintenance", "sweep", "--scope", "implementation", "--repo-dir", t.TempDir(), "--json")
	if errS != "" || !strings.Contains(out, `"scope":"implementation"`) {
		t.Errorf("explicit implementation must reach the operation and echo back: out=%q err=%q", out, errS)
	}
}

// TestMaintenanceSweepScopeRefusedBeforeWork: an unknown or empty explicit
// scope is refused before any repo/network/mutation work — no maintenance.sweep
// document is ever produced.
func TestMaintenanceSweepScopeRefusedBeforeWork(t *testing.T) {
	for _, bad := range []string{"bogus", ""} {
		out, errS, code := runCLI(t, "maintenance", "sweep", "--scope", bad, "--repo-dir", t.TempDir(), "--json")
		if code == 0 {
			t.Errorf("scope %q must fail, exit=0", bad)
		}
		if strings.Contains(out, `"operation":"maintenance.sweep"`) {
			t.Errorf("scope %q must refuse BEFORE dispatching the operation: %q", bad, out)
		}
		// In --json mode a RunE refusal renders as the "cli" invalid-input
		// document on stdout, its message carrying the flag name; in human mode
		// it routes to stderr. Assert the flag-naming diagnostic on whichever
		// channel carried it.
		if !strings.Contains(out+errS, "scope") {
			t.Errorf("scope %q: diagnostic must name the flag, got out=%q err=%q", bad, out, errS)
		}
	}
}

// TestSweepDepsCarrySweepNetworkPolicies proves the sweep-only dependency
// builder resolves the non-default 30s read / 60s write network deadlines onto
// its real Git and GitHub clients. It reads the RESOLVED accessor value, not the
// argument (learnings: defaulted-param-hides-caller-wiring) — a dropped option
// would leave the five-minute default here and pass a shallower argument check.
func TestSweepDepsCarrySweepNetworkPolicies(t *testing.T) {
	// Pin the policy's absolute values, then assert every seam resolves to them:
	// the constants ARE the 30s/60s contract, so a later re-tune reddens here too.
	if sweepNetworkReadTimeout != 30*time.Second {
		t.Fatalf("sweepNetworkReadTimeout = %v, want 30s", sweepNetworkReadTimeout)
	}
	if sweepNetworkWriteTimeout != 60*time.Second {
		t.Fatalf("sweepNetworkWriteTimeout = %v, want 60s", sweepNetworkWriteTimeout)
	}
	deps, err := newSweepFinalizeDeps()
	if err != nil {
		t.Fatalf("newSweepFinalizeDeps: %v", err)
	}
	if got := deps.Planning.Client.NetworkReadTimeout(); got != sweepNetworkReadTimeout {
		t.Errorf("git read timeout = %v, want %v (30s)", got, sweepNetworkReadTimeout)
	}
	if got := deps.Planning.Client.NetworkWriteTimeout(); got != sweepNetworkWriteTimeout {
		t.Errorf("git write timeout = %v, want %v (60s)", got, sweepNetworkWriteTimeout)
	}
	gh, ok := deps.GitHub.(*githubcli.Client)
	if !ok {
		t.Fatalf("deps.GitHub is %T, want *githubcli.Client", deps.GitHub)
	}
	if got := gh.NetworkReadTimeout(); got != sweepNetworkReadTimeout {
		t.Errorf("github read timeout = %v, want %v (30s)", got, sweepNetworkReadTimeout)
	}
	if got := gh.NetworkWriteTimeout(); got != sweepNetworkWriteTimeout {
		t.Errorf("github write timeout = %v, want %v (60s)", got, sweepNetworkWriteTimeout)
	}
}

// TestSweepDepsShareOneClientAcrossNestedSeams proves no nested dependency
// escapes the sweep network policy: the reachable Git seams (Planning.Client and
// CleanupGit) are the SAME *gitcli.Client instance, and the GitHub seam is a
// *githubcli.Client carrying the sweep deadlines. The remaining Git seams
// (Engine, Reader, Workspace, Gate) and GitHub seams (PRProber, PRBatch) wrap
// their client through unexported fields, so they are unreachable by pointer;
// the single-client construction in newSweepFinalizeDeps threads that one policy
// -carrying instance into all of them. The CleanupGit pointer check is the guard
// the mutation probe reddens (build it from a second gitcli.NewClient()).
func TestSweepDepsShareOneClientAcrossNestedSeams(t *testing.T) {
	deps, err := newSweepFinalizeDeps()
	if err != nil {
		t.Fatalf("newSweepFinalizeDeps: %v", err)
	}
	cleanupGit, ok := deps.CleanupGit.(*gitcli.Client)
	if !ok {
		t.Fatalf("deps.CleanupGit is %T, want *gitcli.Client", deps.CleanupGit)
	}
	if cleanupGit != deps.Planning.Client {
		t.Errorf("CleanupGit (%p) is not the same *gitcli.Client as Planning.Client (%p)", cleanupGit, deps.Planning.Client)
	}
	if _, ok := deps.GitHub.(*githubcli.Client); !ok {
		t.Fatalf("deps.GitHub is %T, want *githubcli.Client", deps.GitHub)
	}
}

// TestStandaloneDepsKeepDefaultPolicies proves the sweep-only deadlines never
// leak into the standalone finalize subcommands: newFinalizeDeps clients report
// the five-minute default on both budgets.
func TestStandaloneDepsKeepDefaultPolicies(t *testing.T) {
	deps, err := newFinalizeDeps()
	if err != nil {
		t.Fatalf("newFinalizeDeps: %v", err)
	}
	if got := deps.Planning.Client.NetworkReadTimeout(); got != 5*time.Minute {
		t.Errorf("standalone git read timeout = %v, want 5m", got)
	}
	if got := deps.Planning.Client.NetworkWriteTimeout(); got != 5*time.Minute {
		t.Errorf("standalone git write timeout = %v, want 5m", got)
	}
	gh, ok := deps.GitHub.(*githubcli.Client)
	if !ok {
		t.Fatalf("deps.GitHub is %T, want *githubcli.Client", deps.GitHub)
	}
	if got := gh.NetworkReadTimeout(); got != 5*time.Minute {
		t.Errorf("standalone github read timeout = %v, want 5m", got)
	}
	if got := gh.NetworkWriteTimeout(); got != 5*time.Minute {
		t.Errorf("standalone github write timeout = %v, want 5m", got)
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
