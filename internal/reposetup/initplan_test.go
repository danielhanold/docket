package reposetup

import (
	"io/fs"
	"path/filepath"
	"strings"
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
	plan, err := PlanInit(initCfg(), freshFacts(), mapTree{}, primary)
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

// TestPlanInitDetectedSuiteRendersBothCommands proves a fresh init over a tree
// carrying one Go suite renders a `.docket.yml` edit that sets build AND finalize
// test_command to the detected command, and reports the detected outcome.
func TestPlanInitDetectedSuiteRendersBothCommands(t *testing.T) {
	tree := mapTree{"go.mod": "module x\n", "x_test.go": ""}
	plan, err := PlanInit(initCfg(), freshFacts(), tree, "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if plan.DocketYMLPath != ".docket.yml" {
		t.Errorf("DocketYMLPath = %q, want .docket.yml", plan.DocketYMLPath)
	}
	if plan.TestDiscovery.Kind != DiscoveryDetected {
		t.Fatalf("TestDiscovery.Kind = %q, want detected", plan.TestDiscovery.Kind)
	}
	got := string(plan.DocketYMLBytes)
	// Both build and finalize carry the detected command as separate settings.
	if strings.Count(got, "test_command: go test ./...") != 2 {
		t.Errorf("DocketYMLBytes must set test_command on both build and finalize:\n%s", got)
	}
	if !strings.Contains(got, "build:") || !strings.Contains(got, "finalize:") {
		t.Errorf("DocketYMLBytes missing a build/finalize block:\n%s", got)
	}
}

// TestPlanInitNoSuiteRendersGateOff proves a fresh init over a tree with no
// recognizable suite renders both gates as the quoted scalar "off" and writes no
// fabricated command.
func TestPlanInitNoSuiteRendersGateOff(t *testing.T) {
	plan, err := PlanInit(initCfg(), freshFacts(), mapTree{}, "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if plan.TestDiscovery.Kind != DiscoveryNone {
		t.Fatalf("TestDiscovery.Kind = %q, want none", plan.TestDiscovery.Kind)
	}
	got := string(plan.DocketYMLBytes)
	if strings.Count(got, `gate: "off"`) != 2 {
		t.Errorf("both gates must be the quoted scalar \"off\":\n%s", got)
	}
	if strings.Contains(got, "test_command") {
		t.Errorf("a none outcome must write no test_command:\n%s", got)
	}
}

// TestPlanInitAmbiguousReportsCandidatesWithoutWriting proves an ambiguous tree
// does NOT fail init and writes no config bytes — the candidates ride on the
// TestDiscovery outcome for the caller to report.
func TestPlanInitAmbiguousReportsCandidatesWithoutWriting(t *testing.T) {
	tree := mapTree{"go.mod": "module x\n", "x_test.go": "", "Cargo.toml": "[package]\n"}
	plan, err := PlanInit(initCfg(), freshFacts(), tree, "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit must not fail on ambiguity: %v", err)
	}
	if plan.TestDiscovery.Kind != DiscoveryAmbiguous {
		t.Fatalf("TestDiscovery.Kind = %q, want ambiguous", plan.TestDiscovery.Kind)
	}
	if plan.DocketYMLBytes != nil {
		t.Errorf("ambiguous init must write no config bytes, got %q", plan.DocketYMLBytes)
	}
	if len(plan.TestDiscovery.Candidates) != 2 {
		t.Errorf("ambiguous outcome must carry both candidates, got %+v", plan.TestDiscovery.Candidates)
	}
}

// TestPlanInitAlreadyConfiguredWritesNothing proves that when build/finalize
// already carry explicit commands, discovery short-circuits configured and no
// edit is planned (DocketYMLBytes nil).
func TestPlanInitAlreadyConfiguredWritesNothing(t *testing.T) {
	cfg := initCfg()
	cfg.Build.TestCommand = config.Value[string]{Value: "go test ./..."}
	cfg.Finalize.TestCommand = config.Value[string]{Value: "make check"}
	// panicTree proves the configured short-circuit never probes the worktree.
	plan, err := PlanInit(cfg, freshFacts(), configuredInitTree{}, "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if plan.TestDiscovery.Kind != DiscoveryConfigured {
		t.Errorf("TestDiscovery.Kind = %q, want configured", plan.TestDiscovery.Kind)
	}
	if plan.DocketYMLBytes != nil {
		t.Errorf("configured init must write no config bytes, got %q", plan.DocketYMLBytes)
	}
}

// configuredInitTree returns fs.ErrNotExist for the .docket.yml probe (so PlanInit
// reads no existing file) but panics on any OTHER probe — proving the configured
// short-circuit reaches DiscoverTests without running a detector.
type configuredInitTree struct{}

func (configuredInitTree) Exists(string) (bool, error) {
	panic("configured init must not probe (Exists)")
}
func (configuredInitTree) ReadFile(p string) ([]byte, error) {
	if p == ".docket.yml" {
		return nil, fs.ErrNotExist
	}
	panic("configured init must not probe (ReadFile)")
}
func (configuredInitTree) Glob(string) ([]string, error) {
	panic("configured init must not probe (Glob)")
}

// TestPlanInitRejectsNonFresh proves PlanInit re-classifies defensively and
// refuses any non-fresh input.
func TestPlanInitRejectsNonFresh(t *testing.T) {
	// Legacy: a live surface with no metadata branch.
	legacy := freshFacts()
	legacy.LiveSurface = PresencePresent
	if _, err := PlanInit(initCfg(), legacy, mapTree{}, "/home/user/repo"); err == nil {
		t.Error("PlanInit accepted legacy facts")
	}
	// Unknown: an unproven required probe.
	unknown := freshFacts()
	unknown.RemoteConfigured = PresenceUnknown
	if _, err := PlanInit(initCfg(), unknown, mapTree{}, "/home/user/repo"); err == nil {
		t.Error("PlanInit accepted unknown facts")
	}
	// Healthy topology already present.
	if _, err := PlanInit(initCfg(), healthyFacts(), mapTree{}, "/home/user/repo"); err == nil {
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
	plan, err := PlanInit(explicitRepo, freshFacts(), mapTree{}, "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if got := plan.SeedInput.Harnesses; len(got) != 2 || got[0] != "claude" || got[1] != "codex" {
		t.Errorf("explicit repo harnesses = %v, want [claude codex]", got)
	}

	// Absent (non-explicit) → empty.
	absentCfg := base
	absentCfg.AgentHarnesses = config.Value[[]string]{Value: []string{"claude"}, Explicit: false}
	planAbsent, err := PlanInit(absentCfg, freshFacts(), mapTree{}, "/home/user/repo")
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
	planGlobal, err := PlanInit(globalCfg, freshFacts(), mapTree{}, "/home/user/repo")
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
	planLocal, err := PlanInit(localCfg, freshFacts(), mapTree{}, "/home/user/repo")
	if err != nil {
		t.Fatalf("PlanInit errored: %v", err)
	}
	if got := planLocal.SeedInput.Harnesses; len(got) != 1 || got[0] != "cursor" {
		t.Errorf("repo-local harnesses = %v, want [cursor]", got)
	}
}
