package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposetup"
)

// healthyFacts returns a Facts value that classifies healthy, so the guard's
// happy path is exercised without a real repository.
func healthyConfigureFacts() reposetup.Facts {
	return reposetup.Facts{
		RemoteConfigured:     reposetup.PresencePresent,
		RemoteDefaultBranch:  reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "aaa"},
		RemoteIntegration:    reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "bbb"},
		RemoteMetadata:       reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "ccc"},
		MetadataRoot:         reposetup.RootParentless,
		LocalMetadata:        reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "ccc"},
		LiveSurface:          reposetup.PresenceAbsent,
		CommittedIgnoreBlock: reposetup.PresencePresent,
		DocketWorktree: reposetup.WorktreeFact{
			Presence: reposetup.PresencePresent, Registered: reposetup.PresencePresent,
			Clean: reposetup.PresencePresent, Synchronized: reposetup.PresencePresent,
			HooksOff: reposetup.PresencePresent,
		},
		LegacyConfigKey:      reposetup.PresenceAbsent,
		PrimaryClean:         reposetup.PresencePresent,
		PrimaryOnIntegration: reposetup.PresencePresent,
		PrimaryAtRemoteTip:   reposetup.PresencePresent,
	}
}

// TestConfigureTestsGuardAdmitsHealthy proves the healthy topology is admitted
// (no refusal), so configure-tests may proceed to discover and write.
func TestConfigureTestsGuardAdmitsHealthy(t *testing.T) {
	facts := healthyConfigureFacts()
	cls, refusal := configureTestsGuard(facts)
	if cls.State != reposetup.StateHealthy {
		t.Fatalf("fixture classified %q, want healthy (adjust the fixture)", cls.State)
	}
	if refusal != nil {
		t.Fatalf("healthy topology must be admitted, got refusal: %s", refusal.HumanText())
	}
}

// TestConfigureTestsGuardRefusesFreshNamesInit proves a fresh repository is
// refused with the remedy naming init — configure-tests is an upgrade path, not
// a bootstrap.
func TestConfigureTestsGuardRefusesFreshNamesInit(t *testing.T) {
	// No metadata branch and no live surface classifies fresh.
	facts := reposetup.Facts{
		RemoteConfigured:    reposetup.PresencePresent,
		RemoteDefaultBranch: reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "aaa"},
		RemoteIntegration:   reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "bbb"},
		RemoteMetadata:      reposetup.BranchFact{Presence: reposetup.PresenceAbsent},
		LiveSurface:         reposetup.PresenceAbsent,
	}
	cls, refusal := configureTestsGuard(facts)
	if cls.State != reposetup.StateFresh {
		t.Fatalf("fixture classified %q, want fresh", cls.State)
	}
	if refusal == nil {
		t.Fatal("a fresh repository must be refused")
	}
	if refusal.Result != ResultInvalidState {
		t.Errorf("result = %q, want invalid-state", refusal.Result)
	}
	if !strings.Contains(refusal.HumanText(), "docket repository init") {
		t.Errorf("fresh remedy %q must name `docket repository init`", refusal.HumanText())
	}
}

// TestConfigureTestsGuardRefusesLegacyNamesMigrate proves a legacy single-branch
// repository is refused with the remedy naming migrate.
func TestConfigureTestsGuardRefusesLegacyNamesMigrate(t *testing.T) {
	// A live planning surface without a metadata branch classifies legacy.
	facts := reposetup.Facts{
		RemoteConfigured:    reposetup.PresencePresent,
		RemoteDefaultBranch: reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "aaa"},
		RemoteIntegration:   reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: "bbb"},
		RemoteMetadata:      reposetup.BranchFact{Presence: reposetup.PresenceAbsent},
		LiveSurface:         reposetup.PresencePresent,
	}
	cls, refusal := configureTestsGuard(facts)
	if cls.State != reposetup.StateLegacy {
		t.Fatalf("fixture classified %q, want legacy", cls.State)
	}
	if refusal == nil {
		t.Fatal("a legacy repository must be refused")
	}
	if !strings.Contains(refusal.HumanText(), "docket repository migrate") {
		t.Errorf("legacy remedy %q must name `docket repository migrate`", refusal.HumanText())
	}
}
func testPolicyCfg(buildGate, buildCmd, finalizeGate, finalizeCmd string) config.Effective {
	var eff config.Effective
	eff.Build.Gate = config.Value[string]{Value: buildGate}
	eff.Build.TestCommand = config.Value[string]{Value: buildCmd}
	eff.Finalize.Gate = config.Value[string]{Value: finalizeGate}
	eff.Finalize.TestCommand = config.Value[string]{Value: finalizeCmd}
	return eff
}

const alreadyConfiguredText = "already configured"

func TestConfigureTestsDiscoveryTextNoneNamesCommandRemedy(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryNone}, testPolicyCfg("off", "", "off", ""))
	if !strings.Contains(got, "no supported test suite was found") || !strings.Contains(got, reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("none text %q must say no suite was found and name %q", got, reposetup.ConfigureTestsCommandRemedy)
	}
	if strings.Contains(got, alreadyConfiguredText) {
		t.Errorf("none text %q must not claim the policy is already configured", got)
	}
}

// The none plan is preserve-explicit on gate, so an explicit `local` gate with
// no command also reaches the none no-op: the text must report the RESOLVED
// gates, never assume off.
func TestConfigureTestsDiscoveryTextNoneReportsResolvedGates(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryNone}, testPolicyCfg("local", "", "off", ""))
	if !strings.Contains(got, "build gate `local`") || !strings.Contains(got, "finalize gate `off`") {
		t.Errorf("none text %q must report the resolved gates (build local, finalize off)", got)
	}
}

func TestConfigureTestsDiscoveryTextAmbiguousListsEveryCandidateCommand(t *testing.T) {
	outcome := reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryAmbiguous, Candidates: []reposetup.DetectedSuite{
		{Family: "go", Command: "go test ./..."}, {Family: "rust", Command: "cargo test"},
	}}
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "", outcome, testPolicyCfg("off", "", "off", ""))
	for _, want := range []string{"go test ./...", "cargo test", reposetup.ConfigureTestsCommandRemedy} {
		if !strings.Contains(got, want) {
			t.Errorf("ambiguous text %q must name %q", got, want)
		}
	}
	if strings.Contains(got, alreadyConfiguredText) {
		t.Errorf("ambiguous text %q must not claim the policy is already configured", got)
	}
}

func TestConfigureTestsDiscoveryTextConfiguredNamesBothCommands(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryConfigured}, testPolicyCfg("local", "make a", "local", "make b"))
	for _, want := range []string{alreadyConfiguredText, "`make a`", "`make b`"} {
		if !strings.Contains(got, want) {
			t.Errorf("configured text %q must contain %q", got, want)
		}
	}
	if strings.Contains(got, reposetup.ConfigureTestsCommandRemedy) {
		t.Errorf("a fully configured pair has no gap; text %q must not name the command remedy", got)
	}
}

func TestConfigureTestsDiscoveryTextConfiguredWithGapNamesCommandRemedy(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryConfigured}, testPolicyCfg("local", "make a", "local", ""))
	for _, want := range []string{"finalize.test_command", reposetup.ConfigureTestsCommandRemedy} {
		if !strings.Contains(got, want) {
			t.Errorf("configured-with-gap text %q must contain %q", got, want)
		}
	}
}

func TestConfigureTestsDiscoveryTextDetectedNoChangeNamesCommand(t *testing.T) {
	outcome := reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryDetected, Command: "go test ./...",
		Candidates: []reposetup.DetectedSuite{{Family: "go", Command: "go test ./..."}}}
	got := configureTestsDiscoveryText(reposetup.StateHealthy, false, "", outcome, testPolicyCfg("off", "", "off", ""))
	if !strings.Contains(got, alreadyConfiguredText) || !strings.Contains(got, "`go test ./...`") {
		t.Errorf("detected-no-change text %q must say already configured and name the command", got)
	}
}

func TestConfigureTestsDiscoveryTextWroteNoneStillNamesRemedy(t *testing.T) {
	got := configureTestsDiscoveryText(reposetup.StateNeedsReview, true, ".docket.yml",
		reposetup.DiscoveryOutcome{Kind: reposetup.DiscoveryNone}, testPolicyCfg("", "", "", ""))
	for _, want := range []string{"review and commit the pending path: .docket.yml", reposetup.ConfigureTestsCommandRemedy} {
		if !strings.Contains(got, want) {
			t.Errorf("wrote-none text %q must contain %q", got, want)
		}
	}
}

func TestConfigureTestsExplicitText(t *testing.T) {
	wrote := configureTestsExplicitText(reposetup.StateNeedsReview, true, ".docket.yml", "sh ./test.sh")
	for _, want := range []string{"`sh ./test.sh`", "review and commit the pending path: .docket.yml"} {
		if !strings.Contains(wrote, want) {
			t.Errorf("explicit wrote text %q must contain %q", wrote, want)
		}
	}
	noop := configureTestsExplicitText(reposetup.StateHealthy, false, "", "sh ./test.sh")
	for _, want := range []string{"no-op", "`sh ./test.sh`", "nothing to write"} {
		if !strings.Contains(noop, want) {
			t.Errorf("explicit no-op text %q must contain %q", noop, want)
		}
	}
}

// Bad input is refused before ANY repository read: the nil Git client would
// fail the gather if validation ran after it.
func TestRunRepositoryConfigureTestsRefusesInvalidCommandBeforeGather(t *testing.T) {
	for _, raw := range []string{"", "   ", "auto"} {
		v := raw
		res := RunRepositoryConfigureTests(context.Background(), SetupDeps{}, ConfigureTestsOptions{Command: &v})
		got, ok := res.(RepositoryOpResult)
		if !ok {
			t.Fatalf("result is %T, want RepositoryOpResult", res)
		}
		if got.Result != ResultInvalidInput {
			t.Errorf("--command %q = %q (%s), want invalid-input", raw, got.Result, got.HumanText())
		}
		if len(got.PendingPaths) != 0 {
			t.Errorf("--command %q wrote pending paths %v", raw, got.PendingPaths)
		}
	}
}

// TestEnsureExplicitTestCommandRefusesUnsplicableFileUntouched proves a
// configure-tests --command edit the splice core cannot make safely (an
// existing folded block-scalar test_command) surfaces as an error and leaves
// .docket.yml byte-identical.
func TestEnsureExplicitTestCommandRefusesUnsplicableFileUntouched(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, docketYMLRel)
	orig := []byte("build:\n  gate: \"off\"\n  test_command: >-\n    sh ./test.sh\n    make test\nfinalize:\n  gate: \"off\"\n")
	if err := os.WriteFile(abs, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	pending, wrote, err := ensureExplicitTestCommand(dir, "make test")
	if err == nil {
		t.Fatalf("expected a refusal; got pending=%q wrote=%v", pending, wrote)
	}
	if wrote || pending != "" {
		t.Errorf("refusal reported pending=%q wrote=%v", pending, wrote)
	}
	if !strings.Contains(err.Error(), "refusing to edit") {
		t.Errorf("error %q does not say the edit was refused", err)
	}
	got, rerr := os.ReadFile(abs)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(got) != string(orig) {
		t.Errorf(".docket.yml changed on refusal:\n%s", got)
	}
}
