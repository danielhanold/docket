package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposetup"
)

// effectiveForMigrateTest is the minimal resolved configuration the migration
// planner and preview read: the configured changes/ADR directories and branch
// names a docket-mode repository carries.
func effectiveForMigrateTest() config.Effective {
	return config.Effective{
		MetadataBranch:    config.Value[string]{Value: "docket"},
		IntegrationBranch: config.Value[string]{Value: "main"},
		ChangesDir:        config.Value[string]{Value: "docs/changes"},
		ADRsDir:           config.Value[string]{Value: "docs/adrs"},
		ResultsDir:        config.Value[string]{Value: "docs/results"},
	}
}

// TestDecideMigrateAuthorizationMatrix pins the two-pass authorization gate: the
// unauthorized preview always needs confirmation (even with --repair-frontmatter
// alone), an authorized run with repairs present but not opted in needs
// --repair-frontmatter, and only --yes (plus --repair-frontmatter when repairs
// exist) proceeds. Dropping the RepairAuthorized conjunct is the mutation probe
// this matrix pins: the "--yes alone with repairs" row would then proceed.
func TestDecideMigrateAuthorizationMatrix(t *testing.T) {
	cases := []struct {
		name          string
		authorized    bool
		repairAuth    bool
		hasRepairable bool
		want          migrateAuthorization
	}{
		{"no flags", false, false, false, migrateConfirmRequired},
		{"no flags with repairs", false, false, true, migrateConfirmRequired},
		{"--repair-frontmatter alone still needs --yes", false, true, true, migrateConfirmRequired},
		{"--yes alone, no repairs, proceeds", true, false, false, migrateProceed},
		{"--yes alone with repairs needs --repair-frontmatter", true, false, true, migrateRepairRequired},
		{"--yes --repair-frontmatter proceeds", true, true, true, migrateProceed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := MigrateOptions{Authorized: tc.authorized, RepairAuthorized: tc.repairAuth}
			if got := decideMigrateAuthorization(o, tc.hasRepairable); got != tc.want {
				t.Errorf("decideMigrateAuthorization(%+v, %v) = %v, want %v", o, tc.hasRepairable, got, tc.want)
			}
		})
	}
}

// TestMigrateSourceMovedContention proves the decide-and-act-on-the-same-copy
// contention check: an empty ExpectedSource (the preview pass) never contends, an
// equal ExpectedSource proceeds, and a differing one contends.
func TestMigrateSourceMovedContention(t *testing.T) {
	if migrateSourceMoved("", "deadbeef") {
		t.Error("an empty ExpectedSource (preview pass) must never contend")
	}
	if migrateSourceMoved("deadbeef", "deadbeef") {
		t.Error("an ExpectedSource equal to the fresh tip must not contend")
	}
	if !migrateSourceMoved("deadbeef", "cafebabe") {
		t.Error("an ExpectedSource that differs from the fresh tip must contend")
	}
}

// TestMigrateResultJSONFieldNames pins the protocol-v1 field names the migration
// document carries.
func TestMigrateResultJSONFieldNames(t *testing.T) {
	out := migrateApplied(
		setupContext{integrationBranch: "main"},
		"aa11", "bb22", "cc33",
		[]string{"docs/changes", "docs/adrs", reposetup.SpecsDir},
		[]string{"docs/changes/active/", "docs/changes/BOARD.md", "docs/changes/README.md"},
		[]reposetup.RepairFinding{{Path: "docs/changes/archive/0001-x.md", Field: "depends_on", Code: reposetup.RepairScalarToList, Repairable: true}},
		[]string{"fast-forward your primary worktree"},
	)
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"protocol_version", "operation", "result", "repository_state",
		"source_revision", "metadata_revision", "integration_revision",
		"copy_prefixes", "removed_paths", "repairs", "pending_local",
	} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("result JSON missing field %q: %s", key, raw)
		}
	}
	if decoded["operation"] != OperationRepositoryMigrate {
		t.Errorf("operation = %v, want %q", decoded["operation"], OperationRepositoryMigrate)
	}
	if decoded["source_revision"] != "cc33" {
		t.Errorf("source_revision = %v, want cc33", decoded["source_revision"])
	}
}

// TestMigrateContendedNamesBothRevisions proves the contended document names the
// fresh integration tip and the pinned source it decided on.
func TestMigrateContendedNamesBothRevisions(t *testing.T) {
	out := migrateContended("freshtip", "pinnedsrc")
	if out.Result != ResultContended {
		t.Fatalf("Result = %q, want contended", out.Result)
	}
	if out.SourceRevision != "pinnedsrc" {
		t.Errorf("SourceRevision = %q, want pinnedsrc", out.SourceRevision)
	}
	h := out.HumanText()
	if !strings.Contains(h, "freshtip") || !strings.Contains(h, "pinnedsrc") {
		t.Errorf("human %q must name both the fresh tip and the pinned source", h)
	}
}

// TestMigrateConfirmationRequiredPreviewCarriesPlan proves the unauthorized
// preview refuses as invalid-state, its human text names the copy set, removal
// set, and the --yes remedy, and its repository_state is confirmation-required.
func TestMigrateConfirmationRequiredPreviewCarriesPlan(t *testing.T) {
	sc := setupContext{
		cfg:               effectiveForMigrateTest(),
		integrationBranch: "main",
		metadataBranch:    "docket",
	}
	plan, err := reposetup.PlanMigration(sc.cfg, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	mr := migrationRepairs{}
	preview := migratePreviewText(sc, plan, mr, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	out := migrateConfirmationRequired("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", plan, mr, preview)

	if out.Result != ResultInvalidState {
		t.Fatalf("Result = %q, want invalid-state", out.Result)
	}
	if out.RepositoryState != "confirmation-required" {
		t.Errorf("RepositoryState = %q, want confirmation-required", out.RepositoryState)
	}
	h := out.HumanText()
	for _, want := range []string{"docs/changes", "docs/adrs", reposetup.SpecsDir, "BOARD.md", "README.md", "--yes"} {
		if !strings.Contains(h, want) {
			t.Errorf("preview human %q must name %q", h, want)
		}
	}
}

// TestMigrateRepairAuthorizationRequiredNamesFlag proves an authorized run whose
// plan carries repairs the caller did not opt into is refused naming
// --repair-frontmatter.
func TestMigrateRepairAuthorizationRequiredNamesFlag(t *testing.T) {
	sc := setupContext{cfg: effectiveForMigrateTest(), integrationBranch: "main", metadataBranch: "docket"}
	plan, err := reposetup.PlanMigration(sc.cfg, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	mr := migrationRepairs{repairable: []reposetup.RepairFinding{{Path: "docs/changes/active/0001-x.md", Field: "title", Code: reposetup.RepairQuoteScalar, Repairable: true}}}
	preview := migratePreviewText(sc, plan, mr, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	out := migrateRepairAuthorizationRequired("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", plan, mr, preview)

	if out.Result != ResultInvalidState {
		t.Fatalf("Result = %q, want invalid-state", out.Result)
	}
	if !strings.Contains(out.HumanText(), "--repair-frontmatter") {
		t.Errorf("human %q must name --repair-frontmatter", out.HumanText())
	}
}
