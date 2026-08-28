package reposetup

import (
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

func migrateCfg() config.Effective {
	return config.Effective{
		MetadataBranch: config.Value[string]{Value: "docket"},
		ChangesDir:     config.Value[string]{Value: "docs/changes"},
		ADRsDir:        config.Value[string]{Value: "docs/adrs"},
		ResultsDir:     config.Value[string]{Value: "docs/results"},
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
	plan, err := PlanMigration(migrateCfg(), "cafebabe", nil)
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
	plan, err := PlanMigration(migrateCfg(), "cafebabe", nil)
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
	plan, err := PlanMigration(cfg, "cafebabe", nil)
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
	plan, err := PlanMigration(migrateCfg(), "cafebabe", nil)
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
	plan, err := PlanMigration(migrateCfg(), "cafebabe", repairs)
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

// TestPlanMigrationConfigEdit proves ConfigEdit is true exactly when the legacy
// metadata_branch key is present in the repository-layer .docket.yml, and false
// otherwise.
func TestPlanMigrationConfigEdit(t *testing.T) {
	// Legacy key present in .docket.yml (repository layer, explicit).
	present := migrateCfg()
	present.MetadataBranch = config.Value[string]{
		Value:      "docket",
		Explicit:   true,
		Provenance: config.Provenance{Layer: config.LayerRepository},
	}
	plan, err := PlanMigration(present, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if !plan.ConfigEdit {
		t.Error("ConfigEdit = false, want true when the legacy key is present in .docket.yml")
	}

	// Not explicit (built-in default) → no edit.
	absent := migrateCfg()
	absent.MetadataBranch = config.Value[string]{Value: "docket", Explicit: false}
	planAbsent, err := PlanMigration(absent, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if planAbsent.ConfigEdit {
		t.Error("ConfigEdit = true, want false when the key is not explicitly declared")
	}

	// Explicit but in the global layer (not .docket.yml) → no edit.
	global := migrateCfg()
	global.MetadataBranch = config.Value[string]{
		Value:      "docket",
		Explicit:   true,
		Provenance: config.Provenance{Layer: config.LayerGlobal},
	}
	planGlobal, err := PlanMigration(global, "cafebabe", nil)
	if err != nil {
		t.Fatalf("PlanMigration errored: %v", err)
	}
	if planGlobal.ConfigEdit {
		t.Error("ConfigEdit = true, want false when the key is declared only in the global layer")
	}
}

// TestPlanMigrationRequiresSourceRevision proves an empty pinned revision is
// refused — the planner never fabricates a source.
func TestPlanMigrationRequiresSourceRevision(t *testing.T) {
	if _, err := PlanMigration(migrateCfg(), "", nil); err == nil {
		t.Error("PlanMigration accepted an empty source revision")
	}
}
