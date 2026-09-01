package app

import (
	"strings"
	"testing"

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
