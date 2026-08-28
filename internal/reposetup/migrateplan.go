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
	ConfigEdit   bool    // legacy metadata_branch key present in the committed .docket.yml bytes → one edit
	SeedReceipt  Receipt // OpMigrateSeed: source revision + repair digest (+ copy digest later)
	PruneReceipt Receipt // OpMigratePrune: source revision (+ metadata revision later)
}

// PlanMigration is pure: it names WHICH prefixes to copy and WHICH paths to
// remove, and composes the receipts, from the configured directories and the
// pinned integration source revision. It never reads disk and never computes
// the copy digest (that requires the composed tree the app layer builds).
//
// ConfigEdit keys on the legacy metadata_branch key being present in the
// COMMITTED repository-layer .docket.yml raw bytes (docketYML — nil when the
// pinned source tree carries no .docket.yml). Change 0363 turned
// metadata_branch into an obsolete tombstone that no longer resolves, so the
// predicate reads the same source-preserving editor the execution phase uses
// (RemoveMetadataBranchKey's `removed` result) rather than resolved
// configuration — a key declared only in a machine layer (global or
// repository-local) is invisible to the committed bytes and plans no edit,
// because migration claims no authority over machine files. Bytes the editor
// refuses to edit fail the plan, so an unremovable key can never be silently
// left behind by an executed migration.
func PlanMigration(cfg config.Effective, docketYML []byte, sourceRevision string, repairs []RepairFinding) (MigrationPlan, error) {
	if sourceRevision == "" {
		return MigrationPlan{}, errors.New("reposetup: PlanMigration requires a pinned source revision")
	}
	configEdit := false
	if docketYML != nil {
		_, removed, err := RemoveMetadataBranchKey(docketYML)
		if err != nil {
			return MigrationPlan{}, err
		}
		configEdit = removed
	}
	changes := cfg.ChangesDir.Value
	return MigrationPlan{
		Copy: CopySet{Prefixes: []string{changes, cfg.ADRsDir.Value, SpecsDir}},
		Removal: RemovalSet{
			ActiveDir:  path.Join(changes, "active"),
			BoardPath:  path.Join(changes, "BOARD.md"),
			ReadmePath: path.Join(changes, "README.md"),
		},
		ConfigEdit: configEdit,
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
