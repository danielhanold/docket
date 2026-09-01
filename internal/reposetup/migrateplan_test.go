package reposetup

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

func migrateCfg() config.Effective {
	// metadata_branch is gone from config.Effective (obsolete tombstone, 0363).
	return config.Effective{
		ChangesDir: config.Value[string]{Value: "docs/changes"},
		ADRsDir:    config.Value[string]{Value: "docs/adrs"},
		ResultsDir: config.Value[string]{Value: "docs/results"},
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestPlanMigrationCopySet proves the copy set is exactly the changes dir, the
// ADR dir, and the specs dir — whole prefixes, and nothing else. Plans,
// results, and source paths never appear.
func TestPlanMigrationCopySet(t *testing.T) {
	plan, err := PlanMigration(migrateCfg(), nil, mapTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	want := []string{"docs/changes", "docs/adrs", SpecsDir}
	if len(plan.Copy.Prefixes) != len(want) {
		t.Fatalf("copy prefixes = %v, want %v", plan.Copy.Prefixes, want)
	}
	for _, w := range want {
		if !contains(plan.Copy.Prefixes, w) {
			t.Errorf("copy prefixes %v missing %q", plan.Copy.Prefixes, w)
		}
	}
	// Never copy plans, results, or a source tree.
	for _, forbidden := range []string{"docs/results", "docs/superpowers/plans", "docs/changes/results", "src"} {
		if contains(plan.Copy.Prefixes, forbidden) {
			t.Errorf("copy prefixes must not include %q, got %v", forbidden, plan.Copy.Prefixes)
		}
	}
	if SpecsDir != "docs/superpowers/specs" {
		t.Errorf("SpecsDir = %q, want docs/superpowers/specs", SpecsDir)
	}
}

// TestPlanMigrationRemovalSet proves the removal set is exactly the active dir,
// the board, and the managed README under the configured changes dir — and
// never a plans/results/source path.
func TestPlanMigrationRemovalSet(t *testing.T) {
	plan, err := PlanMigration(migrateCfg(), nil, mapTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if plan.Removal.ActiveDir != "docs/changes/active" {
		t.Errorf("ActiveDir = %q, want docs/changes/active", plan.Removal.ActiveDir)
	}
	if plan.Removal.BoardPath != "docs/changes/BOARD.md" {
		t.Errorf("BoardPath = %q, want docs/changes/BOARD.md", plan.Removal.BoardPath)
	}
	if plan.Removal.ReadmePath != "docs/changes/README.md" {
		t.Errorf("ReadmePath = %q, want docs/changes/README.md", plan.Removal.ReadmePath)
	}
	for _, forbidden := range []string{"docs/results", "docs/superpowers/plans", "docs/superpowers/specs"} {
		if plan.Removal.ActiveDir == forbidden || plan.Removal.BoardPath == forbidden || plan.Removal.ReadmePath == forbidden {
			t.Errorf("removal set must not name %q", forbidden)
		}
	}
}

// TestPlanMigrationRemovalHonorsChangesDir proves the removal set is derived
// from the configured changes dir, not a hardcoded default.
func TestPlanMigrationRemovalHonorsChangesDir(t *testing.T) {
	cfg := migrateCfg()
	cfg.ChangesDir = config.Value[string]{Value: "work/changes"}
	plan, err := PlanMigration(cfg, nil, mapTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if plan.Removal.ActiveDir != "work/changes/active" {
		t.Errorf("ActiveDir = %q, want work/changes/active", plan.Removal.ActiveDir)
	}
	if !contains(plan.Copy.Prefixes, "work/changes") {
		t.Errorf("copy prefixes %v missing configured changes dir", plan.Copy.Prefixes)
	}
}

// TestPlanMigrationReceipts proves the seed and prune receipts carry the pinned
// source revision, the correct versioned operations, and the repair digest on
// the seed.
func TestPlanMigrationReceipts(t *testing.T) {
	plan, err := PlanMigration(migrateCfg(), nil, mapTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if plan.SeedReceipt.Operation != OpMigrateSeed {
		t.Errorf("seed operation = %q, want %q", plan.SeedReceipt.Operation, OpMigrateSeed)
	}
	if plan.SeedReceipt.SourceRevision != "cafebabe" {
		t.Errorf("seed source revision = %q, want cafebabe", plan.SeedReceipt.SourceRevision)
	}
	if plan.SeedReceipt.RepairDigest != RepairDigest(nil) {
		t.Errorf("seed repair digest = %q, want %q", plan.SeedReceipt.RepairDigest, RepairDigest(nil))
	}
	// CopyDigest is filled by the app layer from the composed tree listing.
	if plan.SeedReceipt.CopyDigest != "" {
		t.Errorf("seed copy digest = %q, want empty (app layer fills it)", plan.SeedReceipt.CopyDigest)
	}
	if plan.PruneReceipt.Operation != OpMigratePrune {
		t.Errorf("prune operation = %q, want %q", plan.PruneReceipt.Operation, OpMigratePrune)
	}
	if plan.PruneReceipt.SourceRevision != "cafebabe" {
		t.Errorf("prune source revision = %q, want cafebabe", plan.PruneReceipt.SourceRevision)
	}
	// MetadataRevision is filled after the seed publishes.
	if plan.PruneReceipt.MetadataRevision != "" {
		t.Errorf("prune metadata revision = %q, want empty (filled after seed)", plan.PruneReceipt.MetadataRevision)
	}
}

// TestPlanMigrationRepairDigestFlows proves a non-empty repair plan changes the
// seed receipt's repair digest.
func TestPlanMigrationRepairDigestFlows(t *testing.T) {
	repairs := []RepairFinding{
		{Path: "docs/changes/active/0001.md", Field: "title", Code: RepairQuoteScalar, Repairable: true, Patch: []byte("patch")},
	}
	plan, err := PlanMigration(migrateCfg(), nil, mapTree{}, "cafebabe", repairs)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if plan.SeedReceipt.RepairDigest != RepairDigest(repairs) {
		t.Errorf("seed repair digest = %q, want %q", plan.SeedReceipt.RepairDigest, RepairDigest(repairs))
	}
	if plan.SeedReceipt.RepairDigest == RepairDigest(nil) {
		t.Error("non-empty repairs did not change the repair digest")
	}
}

// TestPlanMigrationConfigEdit proves ConfigEdit is true exactly when the
// legacy metadata_branch key is present in the COMMITTED .docket.yml raw bytes
// (the same source-preserving editor the execution phase uses), and false
// otherwise — a key declared only in a machine layer (global) never reaches
// the committed bytes, so it plans no edit.
func TestPlanMigrationConfigEdit(t *testing.T) {
	// Legacy key present in the committed .docket.yml bytes.
	present := []byte("metadata_branch: main\nintegration_branch: main\n")
	plan, err := PlanMigration(migrateCfg(), present, mapTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if !plan.ConfigEdit {
		t.Error("ConfigEdit = false, want true when the legacy key is present in .docket.yml")
	}

	// Committed bytes without the key → no edit.
	absent := []byte("integration_branch: main\n")
	planAbsent, err := PlanMigration(migrateCfg(), absent, mapTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if planAbsent.ConfigEdit {
		t.Error("ConfigEdit = true, want false when the committed bytes carry no metadata_branch key")
	}

	// No committed .docket.yml at all (nil bytes) — the global-layer-only case:
	// a machine-layer declaration is invisible to the committed bytes and
	// migration claims no authority over machine files, so no edit is planned.
	planGlobal, err := PlanMigration(migrateCfg(), nil, mapTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if planGlobal.ConfigEdit {
		t.Error("ConfigEdit = true, want false when the key exists only outside the committed repository layer")
	}

	// Bytes the editor refuses to edit fail the plan rather than silently
	// skipping the edit.
	if _, err := PlanMigration(migrateCfg(), []byte("{metadata_branch: main, a: b}"), mapTree{}, "cafebabe", nil); err == nil {
		t.Error("PlanMigration accepted bytes the config editor refuses to edit")
	}
}

// TestPlanMigrationPreserveCopiesFinalizeCommandIntoBuild proves the migrate
// preserve/copy rule: an explicit legacy finalize.test_command is carried into
// build.test_command in the generated ConfigBytes (discovery never runs), and
// finalize's own command is left intact.
func TestPlanMigrationPreserveCopiesFinalizeCommandIntoBuild(t *testing.T) {
	cfg := migrateCfg()
	cfg.Finalize.TestCommand = config.Value[string]{Value: "make check"}
	committed := []byte("finalize:\n  test_command: make check\n")
	// panicTree proves the preserve/copy rule short-circuits discovery: an
	// explicit finalize command is copied without probing the source tree.
	plan, err := PlanMigration(cfg, committed, panicTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	got := string(plan.ConfigBytes)
	if !strings.Contains(got, "build:") || !strings.Contains(got, "test_command: make check") {
		t.Errorf("ConfigBytes must copy finalize.test_command into build:\n%s", got)
	}
	// build.test_command is make check too.
	if strings.Count(got, "test_command: make check") != 2 {
		t.Errorf("both build and finalize must carry `make check`:\n%s", got)
	}
	if !strings.Contains(got, "finalize:") {
		t.Errorf("finalize block missing from ConfigBytes:\n%s", got)
	}
}

// TestPlanMigrationAutoRunsDiscovery proves a legacy finalize.test_command of the
// literal `auto` is treated as unconfigured, so discovery runs over the source
// tree and a detected suite is written under both keys.
func TestPlanMigrationAutoRunsDiscovery(t *testing.T) {
	cfg := migrateCfg()
	cfg.Finalize.TestCommand = config.Value[string]{Value: "auto"}
	tree := mapTree{"go.mod": "module x\n", "x_test.go": ""}
	plan, err := PlanMigration(cfg, nil, tree, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if plan.TestDiscovery.Kind != DiscoveryDetected {
		t.Fatalf("TestDiscovery.Kind = %q, want detected (auto is unconfigured)", plan.TestDiscovery.Kind)
	}
	if strings.Count(string(plan.ConfigBytes), "test_command: go test ./...") != 2 {
		t.Errorf("ConfigBytes must set the detected command on both keys:\n%s", plan.ConfigBytes)
	}
}

// TestPlanMigrationAmbiguousRefusesWithNoPlan proves an ambiguous discovery
// during migrate returns a typed AmbiguousTestDiscoveryError naming the
// candidates and the remedy, and NO plan — so the app layer refuses before any
// remote mutation.
func TestPlanMigrationAmbiguousRefusesWithNoPlan(t *testing.T) {
	tree := mapTree{"go.mod": "module x\n", "x_test.go": "", "Cargo.toml": "[package]\n"}
	plan, err := PlanMigration(migrateCfg(), nil, tree, "cafebabe", nil)
	if err == nil {
		t.Fatal("ambiguous discovery must fail the plan")
	}
	var amb *AmbiguousTestDiscoveryError
	if !errors.As(err, &amb) {
		t.Fatalf("error = %T (%v), want *AmbiguousTestDiscoveryError", err, err)
	}
	if len(amb.Candidates) != 2 {
		t.Errorf("typed error must name both candidates, got %+v", amb.Candidates)
	}
	if !strings.Contains(amb.Error(), "docket repository configure-tests") {
		t.Errorf("remedy %q must name `docket repository configure-tests`", amb.Error())
	}
	// No plan is composed: ConfigBytes and the receipts are the zero value.
	if plan.ConfigBytes != nil || plan.SeedReceipt.Operation != "" {
		t.Errorf("ambiguous migrate must return no plan, got %+v", plan)
	}
}

// TestPlanMigrationConfigBytesFoldsMetadataRemoval proves the generated
// ConfigBytes fold BOTH the legacy metadata_branch removal and the test policy
// into one authoritative copy: the metadata_branch key is gone and the test
// policy is present, byte-preserving the surrounding comments/keys.
func TestPlanMigrationConfigBytesFoldsMetadataRemoval(t *testing.T) {
	committed := []byte("# legacy config\nmetadata_branch: docket\nintegration_branch: main\n")
	plan, err := PlanMigration(migrateCfg(), committed, mapTree{}, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	got := string(plan.ConfigBytes)
	if strings.Contains(got, "metadata_branch") {
		t.Errorf("ConfigBytes must drop the legacy metadata_branch key:\n%s", got)
	}
	if !strings.Contains(got, "integration_branch: main") || !strings.Contains(got, "# legacy config") {
		t.Errorf("ConfigBytes must byte-preserve the surrounding keys/comments:\n%s", got)
	}
	// No suite in the empty tree → both gates off.
	if strings.Count(got, `gate: "off"`) != 2 {
		t.Errorf("ConfigBytes must declare both gates off for a no-suite tree:\n%s", got)
	}
}

// TestPlanMigrationRequiresSourceRevision proves an empty pinned revision is
// refused — the planner never fabricates a source.
func TestPlanMigrationRequiresSourceRevision(t *testing.T) {
	if _, err := PlanMigration(migrateCfg(), nil, mapTree{}, "", nil); err == nil {
		t.Error("PlanMigration accepted an empty source revision")
	}
}
