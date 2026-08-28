package reposetup

import (
	"errors"
	"path"

	"github.com/danielhanold/docket/internal/config"
)

// SpecsDir is the convention location of specs. Specs have no config key, so
// the copy set names this constant once rather than a scattered literal.
const SpecsDir = "docs/superpowers/specs"

// CopySet names the whole repo-relative prefixes the migration copies into the
// metadata branch. Whole prefixes so unknown files under them are
// loss-preserved.
type CopySet struct{ Prefixes []string }

// RemovalSet names the paths the migration prunes from the integration branch:
// the Docket-managed active dir, board, and entry-point README.
type RemovalSet struct {
	ActiveDir  string
	BoardPath  string
	ReadmePath string
}

// MigrationPlan is the pure effect plan for `docket repository migrate`. The
// two receipts are pre-composed with everything decidable at plan time; the app
// layer fills SeedReceipt.CopyDigest from the composed seed tree listing and
// PruneReceipt.MetadataRevision after the seed publishes.
type MigrationPlan struct {
	Copy         CopySet
	Removal      RemovalSet
	ConfigEdit   bool    // legacy metadata_branch key present in .docket.yml → one edit (0363 Task 3 rewires the predicate to raw bytes)
	SeedReceipt  Receipt // OpMigrateSeed: source revision + repair digest (+ copy digest later)
	PruneReceipt Receipt // OpMigratePrune: source revision (+ metadata revision later)
}

// PlanMigration is pure: it names WHICH prefixes to copy and WHICH paths to
// remove, and composes the receipts, from the configured directories and the
// pinned integration source revision. It never reads disk and never computes
// the copy digest (that requires the composed tree the app layer builds).
//
// ConfigEdit keys on the legacy metadata_branch key being present in the
// repository-layer .docket.yml. It used to read that off config resolution
// (explicit repository-layer provenance on MetadataBranch), but change 0363
// turned metadata_branch into an obsolete tombstone that no longer resolves, so
// this is temporarily false — 0363 Task 3 re-derives it from the committed
// .docket.yml raw bytes (RemoveMetadataBranchKey's `removed` result).
func PlanMigration(cfg config.Effective, sourceRevision string, repairs []RepairFinding) (MigrationPlan, error) {
	if sourceRevision == "" {
		return MigrationPlan{}, errors.New("reposetup: PlanMigration requires a pinned source revision")
	}
	changes := cfg.ChangesDir.Value
	return MigrationPlan{
		Copy: CopySet{Prefixes: []string{changes, cfg.ADRsDir.Value, SpecsDir}},
		Removal: RemovalSet{
			ActiveDir:  path.Join(changes, "active"),
			BoardPath:  path.Join(changes, "BOARD.md"),
			ReadmePath: path.Join(changes, "README.md"),
		},
		// 0363 Task 3 removes this: config.Effective.MetadataBranch is gone (it is
		// now an obsolete tombstone), so ConfigEdit can no longer key on the
		// resolved field. Task 3 re-derives it from the committed .docket.yml raw
		// bytes (RemoveMetadataBranchKey's `removed` result); until then the
		// planner reports no config edit.
		ConfigEdit: false,
		SeedReceipt: Receipt{
			Operation:      OpMigrateSeed,
			SourceRevision: sourceRevision,
			RepairDigest:   RepairDigest(repairs),
		},
		PruneReceipt: Receipt{
			Operation:      OpMigratePrune,
			SourceRevision: sourceRevision,
		},
	}, nil
}
