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

var githubLink = render.LinkContext{RepoWebURL: "https://github.com/danielhanold/docket", MetadataBranch: "docket", IntegrationBranch: "main"}
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

// gammaChange is fixture C: beta's spec/plan/results paths, plus the in-flight
// lifecycle state 0417 pins on — status implemented, feature branch set.
func gammaChange() domain.Change {
	return domain.NewChange(domain.ChangeSpec{
		ID:       9,
		Slug:     "gamma-change",
		Title:    "Gamma change",
		Status:   domain.StatusImplemented,
		Branch:   optString("fix/gamma-change"),
		Spec:     optString("docs/superpowers/specs/2026-08-16-beta-change-design.md"),
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Results:  optString("docs/results/2026-08-16-beta-change-results.md"),
		Location: domain.LocationActive,
		Path:     "docs/changes/active/0009-gamma-change.md",
	})
}

// deltaChange is fixture D: gamma after merge — done, archived.
func deltaChange() domain.Change {
	return domain.NewChange(domain.ChangeSpec{
		ID:       10,
		Slug:     "delta-change",
		Title:    "Delta change",
		Status:   domain.StatusDone,
		Branch:   optString("fix/delta-change"),
		Spec:     optString("docs/superpowers/specs/2026-08-16-beta-change-design.md"),
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Results:  optString("docs/results/2026-08-16-beta-change-results.md"),
		Location: domain.LocationArchive,
		Path:     "docs/changes/archive/2026-09-14-0010-delta-change.md",
	})
}

// TestArtifactBlockLifecyclePinsPlanResults is 0417's core assert and its
// mutation-tested guard (guards-are-code): the Plan/Results rows of a
// not-yet-done change resolve onto the FEATURE branch, of a done change onto
// the INTEGRATION branch, while the Spec row stays on the metadata branch in
// every state. Asserts pin the exact produced URL, positive and negative
// (assert-pins-outcome-not-mechanism / assert-detects-removal-not-replacement):
// each case also proves the docket-pinned spelling is GONE for Plan/Results.
//
// Mutation probes (run with -count=1; cp-backup artifacts.go first):
//
//	(a) in ArtifactBlockContent, pass link.MetadataBranch instead of
//	    lifecycleBranch(c, link) for the Plan and Results rows (the pre-0417
//	    hardcoding) -> the implemented and done cases must redden;
//	(b) in lifecycleBranch, delete the StatusDone arm -> the done case must
//	    redden;
//	(c) in lifecycleBranch, delete the empty-branch fallback -> the
//	    fallback case must redden.
func TestArtifactBlockLifecyclePinsPlanResults(t *testing.T) {
	const base = "https://github.com/danielhanold/docket/blob/"
	cases := []struct {
		name    string
		c       domain.Change
		wantRef string // branch the Plan/Results URLs must use
	}{
		{"implemented pins the feature branch", gammaChange(), "fix/gamma-change"},
		{"done pins the integration branch", deltaChange(), "main"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := render.ArtifactBlockContent(tc.c, domain.Snapshot{}, githubLink)
			if err != nil {
				t.Fatalf("ArtifactBlockContent: %v", err)
			}
			wantPlan := "| Plan | [2026-08-16-beta-change.md](" + base + tc.wantRef + "/docs/superpowers/plans/2026-08-16-beta-change.md) |"
			wantResults := "| Results | [2026-08-16-beta-change-results.md](" + base + tc.wantRef + "/docs/results/2026-08-16-beta-change-results.md) |"
			wantSpec := "| Spec | [2026-08-16-beta-change-design.md](" + base + "docket/docs/superpowers/specs/2026-08-16-beta-change-design.md) |"
			for _, want := range []string{wantPlan, wantResults, wantSpec} {
				if !strings.Contains(got, want) {
					t.Errorf("block missing row %q\ngot:\n%s", want, got)
				}
			}
			for _, banned := range []string{
				base + "docket/docs/superpowers/plans/",
				base + "docket/docs/results/",
			} {
				if strings.Contains(got, banned) {
					t.Errorf("Plan/Results row still pinned to the metadata branch (%q present)\ngot:\n%s", banned, got)
				}
			}
		})
	}
}

// TestArtifactBlockStackedMergedUsesFeatureBranch documents the spec's
// explicit choice: stacked-merged is NOT done, so it takes the feature-branch
// leg of the same test — no special handling.
func TestArtifactBlockStackedMergedUsesFeatureBranch(t *testing.T) {
	spec := domain.ChangeSpec{
		ID: 11, Slug: "stacked-change", Title: "Stacked change",
		Status:   domain.StatusStackedMerged,
		Branch:   optString("fix/stacked-change"),
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Location: domain.LocationActive,
		Path:     "docs/changes/active/0011-stacked-change.md",
	}
	got, err := render.ArtifactBlockContent(domain.NewChange(spec), domain.Snapshot{}, githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := "https://github.com/danielhanold/docket/blob/fix/stacked-change/docs/superpowers/plans/2026-08-16-beta-change.md"
	if !strings.Contains(got, want) {
		t.Errorf("stacked-merged Plan row does not use the feature branch\ngot:\n%s", got)
	}
}

// TestArtifactBlockMissingBranchFallsBackToMetadata pins the defensive
// default: a Plan path present with no branch: renders today's metadata-branch
// URL, never a malformed one. (betaChange — no status, no branch — exercises
// the same path via the frozen golden; this case makes the implemented-state
// variant explicit.)
func TestArtifactBlockMissingBranchFallsBackToMetadata(t *testing.T) {
	spec := domain.ChangeSpec{
		ID: 12, Slug: "branchless-change", Title: "Branchless change",
		Status:   domain.StatusImplemented,
		Plan:     optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Location: domain.LocationActive,
		Path:     "docs/changes/active/0012-branchless-change.md",
	}
	got, err := render.ArtifactBlockContent(domain.NewChange(spec), domain.Snapshot{}, githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := "https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-16-beta-change.md"
	if !strings.Contains(got, want) {
		t.Errorf("branchless Plan row did not fall back to the metadata branch\ngot:\n%s", got)
	}
}

// TestArtifactBlockRelativeModeIgnoresLifecycle: relative rendering embeds no
// branch, so an implemented change with a feature branch must produce the
// exact bytes of the frozen relative golden (which was generated from a
// lifecycle-free fixture with the same artifact paths).
func TestArtifactBlockRelativeModeIgnoresLifecycle(t *testing.T) {
	got, err := render.ArtifactBlockContent(gammaChange(), domain.Snapshot{}, relativeLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := readArtifactGolden(t, "block-spec-plan-results.relative.golden")
	if !bytes.Equal([]byte(got), want) {
		t.Errorf("relative-mode output diverged from frozen golden\ngot:\n%s\nwant:\n%s", got, want)
	}
}
