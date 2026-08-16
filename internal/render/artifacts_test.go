package render_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
)

// readArtifactGolden loads a frozen block/backlink snapshot from
// testdata/artifacts. These are historical snapshots of the Bash renderers
// (see testdata/artifacts/PROVENANCE.md); the byte-equality asserts below are
// their drift guard.
func readArtifactGolden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "artifacts", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return b
}

func optString(v string) domain.OptionalString {
	return domain.OptionalString{State: domain.FieldPresent, Value: v}
}

// alphaChange is fixture A: spec + adrs [1,2], no plan/results.
func alphaChange() domain.Change {
	return domain.NewChange(domain.ChangeSpec{
		ID:       7,
		Slug:     "alpha-change",
		Title:    "Alpha change",
		ADRs:     adrIDs(1, 2),
		Spec:     optString("docs/superpowers/specs/2026-08-16-alpha-change-design.md"),
		Location: domain.LocationActive,
		Path:     "docs/changes/active/0007-alpha-change.md",
	})
}

// betaChange is fixture B: spec + plan + results, no adrs.
func betaChange() domain.Change {
	return domain.NewChange(domain.ChangeSpec{
		ID:       8,
		Slug:     "beta-change",
		Title:    "Beta change",
		Spec:     optString("docs/superpowers/specs/2026-08-16-beta-change-design.md"),
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Results:  optString("docs/results/2026-08-16-beta-change-results.md"),
		Location: domain.LocationActive,
		Path:     "docs/changes/active/0008-beta-change.md",
	})
}

// adrSnapshot resolves the two fixture ADRs to their canonical paths.
func adrSnapshot() domain.Snapshot {
	return domain.NewSnapshot(domain.SnapshotSpec{
		ADRs: []domain.ADR{
			domain.NewADR(domain.ADRSpec{ID: 1, Slug: "first-decision", Title: "First decision", Path: "docs/adrs/0001-first-decision.md"}),
			domain.NewADR(domain.ADRSpec{ID: 2, Slug: "second-decision", Title: "Second decision", Path: "docs/adrs/0002-second-decision.md"}),
		},
	})
}

var githubLink = render.LinkContext{RepoWebURL: "https://github.com/danielhanold/docket", MetadataBranch: "docket"}
var relativeLink = render.LinkContext{RepoWebURL: "", MetadataBranch: "docket"}

func TestArtifactBlockContentSpecADRsGitHub(t *testing.T) {
	got, err := render.ArtifactBlockContent(alphaChange(), adrSnapshot(), githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := readArtifactGolden(t, "block-spec-adrs.github.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("block mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestArtifactBlockContentSpecPlanResultsGitHub(t *testing.T) {
	got, err := render.ArtifactBlockContent(betaChange(), domain.Snapshot{}, githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := readArtifactGolden(t, "block-spec-plan-results.github.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("block mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestArtifactBlockContentSpecADRsRelative(t *testing.T) {
	got, err := render.ArtifactBlockContent(alphaChange(), adrSnapshot(), relativeLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := readArtifactGolden(t, "block-spec-adrs.relative.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("block mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestArtifactBlockContentSpecPlanResultsRelative(t *testing.T) {
	got, err := render.ArtifactBlockContent(betaChange(), domain.Snapshot{}, relativeLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := readArtifactGolden(t, "block-spec-plan-results.relative.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("block mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestArtifactBlockContentEmpty: a change with no spec/plan/results/adrs yields
// the empty string — the caller still writes the (empty) managed block.
func TestArtifactBlockContentEmpty(t *testing.T) {
	c := domain.NewChange(domain.ChangeSpec{ID: 9, Slug: "gamma", Title: "Gamma", Path: "docs/changes/active/0009-gamma.md"})
	got, err := render.ArtifactBlockContent(c, domain.Snapshot{}, githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty content, got:\n%s", got)
	}
}

// TestArtifactBlockContentADRLinksCommaSeparated pins the comma-separated ADR
// cell in GitHub mode independently of the golden.
func TestArtifactBlockContentADRLinksCommaSeparated(t *testing.T) {
	got, err := render.ArtifactBlockContent(alphaChange(), adrSnapshot(), githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	wantCell := "| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-first-decision.md), " +
		"[ADR-0002](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0002-second-decision.md) |"
	if !strings.Contains(got, wantCell) {
		t.Fatalf("ADR cell not comma-separated as expected:\n%s", got)
	}
}

// TestArtifactBlockContentUnresolvableADRErrors: an ADR id the snapshot cannot
// resolve is a caller error, not a silent degradation.
func TestArtifactBlockContentUnresolvableADRErrors(t *testing.T) {
	c := domain.NewChange(domain.ChangeSpec{ID: 7, Slug: "alpha", Title: "Alpha", ADRs: adrIDs(1, 42), Path: "docs/changes/active/0007-alpha.md"})
	if _, err := render.ArtifactBlockContent(c, adrSnapshot(), githubLink); err == nil {
		t.Fatalf("expected error for unresolvable ADR id 42, got nil")
	}
}

func TestArtifactBlockContentDeterministic(t *testing.T) {
	a, err := render.ArtifactBlockContent(alphaChange(), adrSnapshot(), githubLink)
	if err != nil {
		t.Fatal(err)
	}
	b, err := render.ArtifactBlockContent(alphaChange(), adrSnapshot(), githubLink)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("non-deterministic output:\n%s\n---\n%s", a, b)
	}
}

func TestBacklinkContentActiveGitHub(t *testing.T) {
	got, err := render.BacklinkContent(alphaChange(), githubLink)
	if err != nil {
		t.Fatalf("BacklinkContent: %v", err)
	}
	want := readArtifactGolden(t, "backlink-active.github.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("backlink mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestBacklinkContentArchiveGitHub pins the kill-retarget shape: the backlink
// targets the change's CURRENT (archive) canonical path.
func TestBacklinkContentArchiveGitHub(t *testing.T) {
	c := domain.NewChange(domain.ChangeSpec{
		ID:       7,
		Slug:     "alpha-change",
		Title:    "Alpha change",
		Location: domain.LocationArchive,
		Path:     "docs/changes/archive/2026-08-16-0007-alpha-change.md",
	})
	got, err := render.BacklinkContent(c, githubLink)
	if err != nil {
		t.Fatalf("BacklinkContent: %v", err)
	}
	want := readArtifactGolden(t, "backlink-archive.github.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("backlink mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBacklinkContentRelative(t *testing.T) {
	got, err := render.BacklinkContent(alphaChange(), relativeLink)
	if err != nil {
		t.Fatalf("BacklinkContent: %v", err)
	}
	want := readArtifactGolden(t, "backlink-active.relative.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Fatalf("backlink mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBacklinkContentDeterministic(t *testing.T) {
	a, err := render.BacklinkContent(alphaChange(), githubLink)
	if err != nil {
		t.Fatal(err)
	}
	b, err := render.BacklinkContent(alphaChange(), githubLink)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("non-deterministic backlink:\n%s\n---\n%s", a, b)
	}
}
