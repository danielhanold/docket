package reposetup

import (
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

// freshFacts returns a Facts value that classifies fresh: a remotely anchored
// repository with no metadata branch and no live surface.
func freshFacts() Facts {
	return Facts{
		RemoteConfigured:    PresencePresent,
		RemoteDefaultBranch: BranchFact{Presence: PresencePresent, Tip: "def0"},
		RemoteIntegration:   BranchFact{Presence: PresencePresent, Tip: "int0"},
		RemoteMetadata:      BranchFact{Presence: PresenceAbsent},
		LiveSurface:         PresenceAbsent,
	}
}

func initCfg() config.Effective {
	// metadata_branch is gone from config.Effective (obsolete tombstone, 0363);
	// PlanInit pins the fixed metadata branch itself (bridged in initplan.go until
	// Task 3 sources it from reposetup.MetadataBranchName).
	return config.Effective{
		ChangesDir: config.Value[string]{Value: "docs/changes"},
		ADRsDir:    config.Value[string]{Value: "docs/adrs"},
	}
}

// TestPlanInitFreshEffects proves a fresh plan carries exactly the six named
// effects, an empty-tree orphan root (no seed-corpus files anywhere), and only
// the operation trailer on the root commit.
func TestPlanInitFreshEffects(t *testing.T) {
	const primary = "/home/user/repo"
	plan, err := PlanInit(initCfg(), freshFacts(), primary)
	if err != nil {
		t.Fatalf("PlanInit on fresh facts errored: %v", err)
	}
	if plan.RootSubject == "" {
		t.Error("RootSubject is empty")
	}
	if plan.MetadataRef != "refs/heads/docket" {
		t.Errorf("MetadataRef = %q, want refs/heads/docket", plan.MetadataRef)
	}
	if plan.WorktreePath != filepath.Join(primary, ".docket") {
		t.Errorf("WorktreePath = %q, want %q", plan.WorktreePath, filepath.Join(primary, ".docket"))
	}
	if plan.GitignorePath != ".gitignore" {
		t.Errorf("GitignorePath = %q, want .gitignore", plan.GitignorePath)
	}
	// The orphan root is the empty tree: the only trailer is the versioned
	// operation marker, and no sample-corpus path rides along.
	if len(plan.RootTrailers) != 1 {
		t.Fatalf("RootTrailers = %d, want exactly 1 (operation only): %+v", len(plan.RootTrailers), plan.RootTrailers)
	}
	if plan.RootTrailers[0].Key != TrailerOperation || plan.RootTrailers[0].Value != OpInitRoot {
		t.Errorf("root operation trailer = %+v, want %s: %s", plan.RootTrailers[0], TrailerOperation, OpInitRoot)
	}
	if _, ok := ParseReceipt(plan.RootTrailers); !ok {
		t.Error("root trailers do not parse back as a receipt")
	}
	if plan.SeedInput.WorktreeRoot != primary {
		t.Errorf("SeedInput.WorktreeRoot = %q, want %q", plan.SeedInput.WorktreeRoot, primary)
	}
}

// TestPlanInitRejectsNonFresh proves PlanInit re-classifies defensively and
// refuses any non-fresh input.
func TestPlanInitRejectsNonFresh(t *testing.T) {
	// Legacy: a live surface with no metadata branch.
	legacy := freshFacts()
	legacy.LiveSurface = PresencePresent
	if _, err := PlanInit(initCfg(), legacy, "/home/user/repo"); err == nil {
		t.Error("PlanInit accepted legacy facts")
	}
	// Unknown: an unproven required probe.
	unknown := freshFacts()
	unknown.RemoteConfigured = PresenceUnknown
	if _, err := PlanInit(initCfg(), unknown, "/home/user/repo"); err == nil {
		t.Error("PlanInit accepted unknown facts")
	}
	// Healthy topology already present.
	if _, err := PlanInit(initCfg(), healthyFacts(), "/home/user/repo"); err == nil {
		t.Error("PlanInit accepted healthy facts")
	}
}

// TestPlanInitSeedHarnessesOptIn proves SeedInput.Harnesses is populated ONLY
// from an explicit repo/repo-local agent_harnesses declaration; an absent or
// global-only declaration yields an empty harness set (opt-in is a signal, not
// file presence).
func TestPlanInitSeedHarnessesOptIn(t *testing.T) {
	base := initCfg()

	explicitRepo := base
	explicitRepo.AgentHarnesses = config.Value[[]string]{
		Value:      []string{"claude", "codex"},
		Explicit:   true,
		Provenance: config.Provenance{Layer: config.LayerRepository},
	}
	plan, err := PlanInit(explicitRepo, freshFacts(), "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if got := plan.SeedInput.Harnesses; len(got) != 2 || got[0] != "claude" || got[1] != "codex" {
		t.Errorf("explicit repo harnesses = %v, want [claude codex]", got)
	}

	// Absent (non-explicit) → empty.
	absentCfg := base
	absentCfg.AgentHarnesses = config.Value[[]string]{Value: []string{"claude"}, Explicit: false}
	planAbsent, err := PlanInit(absentCfg, freshFacts(), "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if len(planAbsent.SeedInput.Harnesses) != 0 {
		t.Errorf("non-explicit harnesses = %v, want empty", planAbsent.SeedInput.Harnesses)
	}

	// Explicit but global-layer → not honored → empty.
	globalCfg := base
	globalCfg.AgentHarnesses = config.Value[[]string]{
		Value:      []string{"claude"},
		Explicit:   true,
		Provenance: config.Provenance{Layer: config.LayerGlobal},
	}
	planGlobal, err := PlanInit(globalCfg, freshFacts(), "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if len(planGlobal.SeedInput.Harnesses) != 0 {
		t.Errorf("global-layer harnesses = %v, want empty", planGlobal.SeedInput.Harnesses)
	}

	// Repo-local layer IS honored.
	localCfg := base
	localCfg.AgentHarnesses = config.Value[[]string]{
		Value:      []string{"cursor"},
		Explicit:   true,
		Provenance: config.Provenance{Layer: config.LayerRepositoryLocal},
	}
	planLocal, err := PlanInit(localCfg, freshFacts(), "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if got := planLocal.SeedInput.Harnesses; len(got) != 1 || got[0] != "cursor" {
		t.Errorf("repo-local harnesses = %v, want [cursor]", got)
	}
}
