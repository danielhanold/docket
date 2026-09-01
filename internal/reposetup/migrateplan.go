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

	// ConfigBytes is the EXACT `.docket.yml` bytes the migration commits onto the
	// pruned integration branch — the single authoritative copy folding the legacy
	// metadata_branch removal AND the generated test policy. nil means the file is
	// unchanged. The preview shows these exact bytes and the execution commits
	// exactly them (learning decide-and-act-on-the-same-copy).
	ConfigBytes []byte
	// TestDiscovery is the setup-time suite discovery outcome the config edit was
	// rendered from (the preserve/copy result when a legacy finalize.test_command
	// was carried into build).
	TestDiscovery DiscoveryOutcome
}

// PlanMigration is pure: it names WHICH prefixes to copy and WHICH paths to
// remove, composes the receipts, and decides the single `.docket.yml` edit —
// all from the configured directories, the committed source bytes, and the
// pinned source tree (read only through the TestTree seam). It never computes
// the copy digest (that requires the composed tree the app layer builds).
//
// ConfigBytes is the ONE authoritative `.docket.yml` copy the execution commits,
// folding the legacy metadata_branch removal (pass 1) and the generated test
// policy (pass 2) so the preview and the commit act on the same bytes (learning
// decide-and-act-on-the-same-copy). Pass 2 applies the preserve/copy rule (an
// explicit legacy finalize.test_command carried into build.test_command) or
// setup-time discovery over tree; an ambiguous discovery is a typed
// AmbiguousTestDiscoveryError returning NO plan, so the app layer refuses before
// any remote mutation.
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
func PlanMigration(cfg config.Effective, docketYML []byte, tree TestTree, sourceRevision string, repairs []RepairFinding) (MigrationPlan, error) {
	if sourceRevision == "" {
		return MigrationPlan{}, errors.New("reposetup: PlanMigration requires a pinned source revision")
	}
	// The `.docket.yml` edit is decided once, on the committed source bytes, and
	// carried on the plan so the preview and the execution act on the SAME copy.
	// Pass 1: remove the legacy metadata_branch key, byte-preserving.
	configEdit := false
	working := docketYML
	if docketYML != nil {
		edited, removed, err := RemoveMetadataBranchKey(docketYML)
		if err != nil {
			return MigrationPlan{}, err
		}
		configEdit = removed
		if removed {
			working = edited
		}
	}
	// Pass 2: the preserve/copy rule (an explicit legacy finalize.test_command is
	// carried into build.test_command) OR setup-time discovery — ambiguity is a
	// typed refusal the app layer surfaces BEFORE any remote mutation.
	outcome, err := migrateTestOutcome(cfg, tree)
	if err != nil {
		return MigrationPlan{}, err
	}
	editedYML, changed, err := RenderTestConfigEdit(working, outcome)
	if err != nil {
		return MigrationPlan{}, err
	}
	var configBytes []byte
	switch {
	case changed:
		configBytes = editedYML
	case configEdit:
		configBytes = working // only the metadata_branch removal changed the file
	}

	changes := cfg.ChangesDir.Value
	return MigrationPlan{
		Copy: CopySet{Prefixes: []string{changes, cfg.ADRsDir.Value, SpecsDir}},
		Removal: RemovalSet{
			ActiveDir:  path.Join(changes, "active"),
			BoardPath:  path.Join(changes, "BOARD.md"),
			ReadmePath: path.Join(changes, "README.md"),
		},
		ConfigEdit:    configEdit,
		ConfigBytes:   configBytes,
		TestDiscovery: outcome,
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

// migrateTestOutcome applies the migrate preserve/copy rule then discovery. An
// explicit, non-legacy finalize.test_command short-circuits discovery and is
// carried into build.test_command (a legacy repository configured only finalize);
// otherwise discovery runs and an ambiguous match is a typed refusal naming the
// candidates and the remedy. The preserve/copy rule runs BEFORE discovery so a
// configured legacy command is never re-probed or overridden.
func migrateTestOutcome(cfg config.Effective, tree TestTree) (DiscoveryOutcome, error) {
	finalizeCmd := cfg.Finalize.TestCommand.Value
	if isConfiguredCommand(finalizeCmd) {
		return DiscoveryOutcome{Kind: DiscoveryDetected, Command: finalizeCmd}, nil
	}
	outcome, err := DiscoverTests(tree, cfg.Build.TestCommand.Value, finalizeCmd)
	if err != nil {
		return DiscoveryOutcome{}, err
	}
	if outcome.Kind == DiscoveryAmbiguous {
		return DiscoveryOutcome{}, &AmbiguousTestDiscoveryError{Candidates: outcome.Candidates}
	}
	return outcome, nil
}
