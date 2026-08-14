package repository

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
)

// corpusRoot is the frozen v0.9.2 sample: real records copied byte for byte
// out of the metadata branch. They are never regenerated — a corpus that
// drifts with the repository proves nothing about the format it froze.
const corpusRoot = "testdata/corpus"

// loadCorpus reads every frozen record, classifying kind and location by the
// directory it sits in, exactly as a composer would.
func loadCorpus(t *testing.T) BuildInput {
	t.Helper()
	in := BuildInput{Config: effectiveConfig()}
	err := filepath.WalkDir(corpusRoot, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".md") {
			return err
		}
		source, readErr := os.ReadFile(name)
		if readErr != nil {
			return readErr
		}
		rel := filepath.ToSlash(strings.TrimPrefix(name, corpusRoot+string(filepath.Separator)))
		kind, location := classifyCorpusPath(rel)
		in.Documents = append(in.Documents, InputDocument{
			Kind: kind, Location: location, Path: rel, Document: mustParse(t, string(source)),
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	if len(in.Documents) == 0 {
		t.Fatal("corpus is empty")
	}
	return in
}

// classifyCorpusPath maps a repository-relative path to the kind and location
// the composer would declare for it.
func classifyCorpusPath(rel string) (RecordKind, RecordLocation) {
	switch {
	case strings.HasPrefix(rel, "docs/adrs/"):
		return KindADR, LocationLedger
	case strings.HasPrefix(rel, "docs/changes/learnings/"):
		return KindLearning, LocationLedger
	case strings.HasPrefix(rel, "docs/changes/archive/"):
		return KindChange, LocationArchive
	default:
		return KindChange, LocationActive
	}
}

// effectiveConfig is a resolved configuration with the four leaves the
// repository policy is allowed to carry.
func effectiveConfig() config.Effective {
	var cfg config.Effective
	cfg.IntegrationBranch.Value = "main"
	cfg.ChangeTypes.Value = []string{"feat", "fix", "chore", "docs"}
	cfg.Reclaim.LeaseTTL.Value = 24
	cfg.Learnings.Enabled.Value = true
	return cfg
}

// severityCodes lists the distinct codes at one severity, sorted.
func severityCodes(report domain.ValidationReport, severity domain.Severity) []string {
	var out []string
	for _, f := range report.Findings() {
		if f.Severity == severity && !slices.Contains(out, f.Code) {
			out = append(out, f.Code)
		}
	}
	slices.Sort(out)
	return out
}

func TestBuildSnapshotOverFrozenCorpusIsClean(t *testing.T) {
	result, err := BuildSnapshot(loadCorpus(t))
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	for _, f := range result.Report.Findings() {
		if f.Severity == domain.SeverityError {
			t.Errorf("unexpected error finding: %s %s %+v %v", f.Code, f.Field, f.Entity, f.Detail)
		}
	}
	if result.Report.HasErrors() {
		t.Fatal("frozen corpus must build without error findings")
	}

	// The sample is a sample: references out of it stay visible as warnings,
	// and the unallocated ADR ids below the highest one are the ledger's
	// standing gap posture. Nothing else may appear.
	want := []string{CodeChangeReferenceDangling, CodeLearningReferenceDangling, domain.CodeADRIDGap}
	slices.Sort(want)
	if got := severityCodes(result.Report, domain.SeverityWarning); !slices.Equal(got, want) {
		t.Errorf("warning codes = %v, want %v", got, want)
	}

	snap := result.Snapshot
	if len(snap.Changes()) != 12 || len(snap.ADRs()) != 9 || len(snap.Learnings()) != 1 {
		t.Errorf("corpus entities = %d changes, %d adrs, %d learnings",
			len(snap.Changes()), len(snap.ADRs()), len(snap.Learnings()))
	}
}

func TestBuildSnapshotCorpusAccountsForEveryPath(t *testing.T) {
	in := loadCorpus(t)
	result, err := BuildSnapshot(in)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	accounted := make(map[string]bool)
	for _, entry := range snapshotPaths(result.Snapshot) {
		accounted[entry.path] = true
	}
	for _, doc := range in.Documents {
		if !accounted[doc.Path] {
			t.Errorf("supplied path %q reached no entity", doc.Path)
		}
	}
	if len(accounted) != len(in.Documents) {
		t.Errorf("accounted %d paths, supplied %d", len(accounted), len(in.Documents))
	}
	if left := unaccountedPaths(in, result.Snapshot, result.Report.Findings()); len(left) != 0 {
		t.Errorf("unaccounted paths: %v", left)
	}
}

func TestBuildSnapshotCorpusCoversEveryFrozenRole(t *testing.T) {
	result, err := BuildSnapshot(loadCorpus(t))
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	roles := map[string]bool{}
	for _, c := range result.Snapshot.Changes() {
		switch {
		case c.Status() == domain.StatusDone && c.Location() == LocationArchive:
			roles["archived-done"] = true
		case c.Status() == domain.StatusKilled && c.Location() == LocationArchive:
			roles["archived-killed"] = true
		case c.Status() == domain.StatusDeferred:
			roles["deferred"] = true
		case c.Status() == domain.StatusProposed && c.Location() == LocationActive &&
			len(c.DependsOn()) > 0 && present(c.Spec()):
			roles["proposed-with-deps-and-spec"] = true
		}
	}
	for _, a := range result.Snapshot.ADRs() {
		switch a.Status().Kind {
		case domain.ADRAccepted:
			roles["adr-accepted"] = true
		case domain.ADRSupersededBy:
			roles["adr-superseded"] = true
		}
	}
	if len(result.Snapshot.Learnings()) > 0 {
		roles["learning"] = true
	}
	for _, role := range []string{"archived-done", "archived-killed", "deferred",
		"proposed-with-deps-and-spec", "adr-accepted", "adr-superseded", "learning"} {
		if !roles[role] {
			t.Errorf("corpus does not cover role %q", role)
		}
	}
}

func TestBuildSnapshotTranslatesPolicy(t *testing.T) {
	cfg := effectiveConfig()
	cfg.IntegrationBranch.Value = "trunk"
	cfg.ChangeTypes.Value = []string{"feat", "chore"}
	cfg.Reclaim.LeaseTTL.Value = 6
	cfg.Learnings.Enabled.Value = false

	result, err := BuildSnapshot(BuildInput{Config: cfg})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	policy := result.Snapshot.Policy()
	if policy.IntegrationBranch != "trunk" {
		t.Errorf("IntegrationBranch = %q", policy.IntegrationBranch)
	}
	if !slices.Equal(policy.ChangeTypes, []string{"feat", "chore"}) {
		t.Errorf("ChangeTypes = %v", policy.ChangeTypes)
	}
	if policy.ReclaimTTLHours != 6 {
		t.Errorf("ReclaimTTLHours = %d", policy.ReclaimTTLHours)
	}
	if policy.LearningsEnabled {
		t.Error("LearningsEnabled = true, want false")
	}
}

func TestBuildSnapshotRejectsViolatedCallShape(t *testing.T) {
	doc := input(t, KindChange, LocationActive, "docs/changes/active/0001-x.md", minimalChange(1, "x", "proposed"))

	unknown := doc
	unknown.Kind = "manifest"
	if _, err := BuildSnapshot(BuildInput{Documents: []InputDocument{unknown}}); err == nil {
		t.Error("unknown record kind: want error")
	}

	unpathed := doc
	unpathed.Path = ""
	if _, err := BuildSnapshot(BuildInput{Documents: []InputDocument{unpathed}}); err == nil {
		t.Error("empty path: want error")
	}
}

func TestBuildSnapshotKeepsUndecodableRecordsAccountedFor(t *testing.T) {
	broken := "# A change record with no frontmatter at all\n"
	in := BuildInput{Documents: []InputDocument{
		input(t, KindChange, LocationActive, "docs/changes/active/0001-broken.md", broken),
		input(t, KindArtifact, LocationArtifact, "docs/specs/design.md", "# design\n"),
		input(t, KindDerived, LocationDerived, "docs/changes/BOARD.md", "# board\n"),
	}}
	result, err := BuildSnapshot(in)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if !hasFinding(result.Report.Findings(), CodeRecordUndecodable, "") {
		t.Errorf("want %s, got %v", CodeRecordUndecodable, findingCodes(result.Report.Findings()))
	}
	if left := unaccountedPaths(in, result.Snapshot, result.Report.Findings()); len(left) != 0 {
		t.Errorf("unaccounted paths: %v", left)
	}
	if len(result.Snapshot.Artifacts()) != 1 || len(result.Snapshot.DerivedViews()) != 1 {
		t.Errorf("artifact/derived accounting = %d/%d",
			len(result.Snapshot.Artifacts()), len(result.Snapshot.DerivedViews()))
	}
	for _, f := range result.Report.Findings() {
		if f.Code == CodeRecordUnaccounted {
			t.Errorf("unexpected %s for %q", f.Code, f.Entity.Path)
		}
	}
}

func TestBuildSnapshotRetainsDuplicateIDs(t *testing.T) {
	result := buildRecords(t,
		record(t, KindChange, LocationActive, "docs/changes/active/0042-first.md",
			minimalChange(42, "first", "proposed")),
		record(t, KindChange, LocationActive, "docs/changes/active/0042-second.md",
			minimalChange(42, "second", "proposed")),
	)
	if len(result.Snapshot.Changes()) != 2 {
		t.Fatalf("changes = %d, want 2 (both duplicates retained)", len(result.Snapshot.Changes()))
	}
	if _, out := result.Snapshot.Change(42); out != domain.LookupAmbiguous {
		t.Errorf("lookup outcome = %v, want ambiguous", out)
	}
	if !hasFinding(result.Report.Findings(), CodeChangeIDDuplicate, "id") {
		t.Errorf("want %s, got %v", CodeChangeIDDuplicate, findingCodes(result.Report.Findings()))
	}
}
