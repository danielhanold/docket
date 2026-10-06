package render_test

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/render"
)

// readArtifactGolden loads a block/backlink snapshot from testdata/artifacts
// (see testdata/artifacts/PROVENANCE.md for each golden's source); the
// byte-equality asserts below are their drift guard.
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
// cell independently of the golden: each entry keeps its ADR-NNNN label and
// links the ADR relatively from the record.
func TestArtifactBlockContentADRLinksCommaSeparated(t *testing.T) {
	got, err := render.ArtifactBlockContent(alphaChange(), adrSnapshot(), githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	wantCell := "| ADRs | [ADR-0001](../../adrs/0001-first-decision.md), " +
		"[ADR-0002](../../adrs/0002-second-decision.md) |"
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

// fullChange is a change carrying every artifact kind — spec, plan, results,
// and two ADRs — at the given lifecycle state, location, and record path.
func fullChange(status domain.Status, evidence bool, loc domain.RecordLocation, recPath string) domain.Change {
	return domain.NewChange(domain.ChangeSpec{
		ID:               10,
		Slug:             "delta-change",
		Title:            "Delta change",
		Status:           status,
		Branch:           optString("fix/delta-change"),
		ADRs:             adrIDs(1, 2),
		Spec:             optString("docs/superpowers/specs/2026-08-16-delta-change-design.md"),
		Plan:             optString("docs/superpowers/plans/2026-08-16-delta-change.md"),
		Results:          optString("docs/results/2026-08-16-delta-change-results.md"),
		HasBuildEvidence: evidence,
		Location:         loc,
		Path:             recPath,
	})
}

const (
	deltaActivePath  = "docs/changes/active/0010-delta-change.md"
	deltaArchivePath = "docs/changes/archive/2026-09-14-0010-delta-change.md"
)

// markdownLinkTargets returns every Markdown link target "(...)" following a
// "](" in s, in order.
func markdownLinkTargets(s string) []string {
	var out []string
	for {
		i := strings.Index(s, "](")
		if i < 0 {
			return out
		}
		s = s[i+2:]
		j := strings.IndexByte(s, ')')
		if j < 0 {
			return out
		}
		out = append(out, s[:j])
		s = s[j+1:]
	}
}

// TestArtifactBlockRelativeRowsSurviveArchive: every row of a change whose
// artifacts live on the metadata branch is a link relative to the record, and
// docs/changes/active and docs/changes/archive are siblings, so the block is
// byte-identical before and after archiving. Every link resolves, from the
// record's own directory, to the field's path; no row is an absolute blob URL;
// and the rows render the same with or without a web URL.
func TestArtifactBlockRelativeRowsSurviveArchive(t *testing.T) {
	active := fullChange(domain.StatusInProgress, false, domain.LocationActive, deltaActivePath)
	archived := fullChange(domain.StatusInProgress, false, domain.LocationArchive, deltaArchivePath)

	gotActive, err := render.ArtifactBlockContent(active, adrSnapshot(), githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent(active): %v", err)
	}
	gotArchived, err := render.ArtifactBlockContent(archived, adrSnapshot(), githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent(archived): %v", err)
	}
	if gotActive != gotArchived {
		t.Fatalf("block differs across the archive move:\n--- active ---\n%s\n--- archived ---\n%s", gotActive, gotArchived)
	}
	gotNoWeb, err := render.ArtifactBlockContent(active, adrSnapshot(), relativeLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent(no web URL): %v", err)
	}
	if gotNoWeb != gotActive {
		t.Fatalf("relative rows depend on the web URL:\n--- github ---\n%s\n--- none ---\n%s", gotActive, gotNoWeb)
	}
	if strings.Contains(gotActive, "/blob/") {
		t.Fatalf("a same-branch row is still an absolute blob URL:\n%s", gotActive)
	}

	want := map[string]bool{
		"docs/superpowers/specs/2026-08-16-delta-change-design.md": true,
		"docs/superpowers/plans/2026-08-16-delta-change.md":        true,
		"docs/results/2026-08-16-delta-change-results.md":          true,
		"docs/adrs/0001-first-decision.md":                         true,
		"docs/adrs/0002-second-decision.md":                        true,
	}
	for _, recPath := range []string{deltaActivePath, deltaArchivePath} {
		targets := markdownLinkTargets(gotActive)
		if len(targets) != len(want) {
			t.Fatalf("got %d links, want %d:\n%s", len(targets), len(want), gotActive)
		}
		seen := map[string]bool{}
		for _, link := range targets {
			resolved := path.Join(path.Dir(recPath), link)
			if !want[resolved] {
				t.Errorf("link %q from %q resolves to %q, which is no artifact field", link, recPath, resolved)
			}
			seen[resolved] = true
		}
		if len(seen) != len(want) {
			t.Errorf("links from %q resolve to %d distinct artifacts, want %d", recPath, len(seen), len(want))
		}
	}
}

// TestArtifactBlockLegacyDoneKeepsIntegrationRows pins the legacy rule: a done
// change with no "## Build evidence" section predates the metadata-branch
// cutover, so its plan and results live on the integration branch and their
// rows stay absolute there. Its spec and ADR rows are relative like every
// other record's, and it gets no "Spec (merged)" row.
//
// Mutation probe (Step 7): make the legacy predicate always false -> the
// absolute Plan/Results asserts must redden.
func TestArtifactBlockLegacyDoneKeepsIntegrationRows(t *testing.T) {
	c := fullChange(domain.StatusDone, false, domain.LocationArchive, deltaArchivePath)

	got, err := render.ArtifactBlockContent(c, adrSnapshot(), githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := "| Artifact | Link |\n|---|---|\n" +
		"| Spec | [2026-08-16-delta-change-design.md](../../superpowers/specs/2026-08-16-delta-change-design.md) |\n" +
		"| Plan | [2026-08-16-delta-change.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-08-16-delta-change.md) |\n" +
		"| Results | [2026-08-16-delta-change-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-08-16-delta-change-results.md) |\n" +
		"| ADRs | [ADR-0001](../../adrs/0001-first-decision.md), [ADR-0002](../../adrs/0002-second-decision.md) |\n"
	if got != want {
		t.Fatalf("legacy done block mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	gotNoWeb, err := render.ArtifactBlockContent(c, adrSnapshot(), relativeLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent(no web URL): %v", err)
	}
	wantNoWeb := "| Artifact | Link |\n|---|---|\n" +
		"| Spec | [2026-08-16-delta-change-design.md](../../superpowers/specs/2026-08-16-delta-change-design.md) |\n" +
		"| Plan | `docs/superpowers/plans/2026-08-16-delta-change.md` |\n" +
		"| Results | `docs/results/2026-08-16-delta-change-results.md` |\n" +
		"| ADRs | [ADR-0001](../../adrs/0001-first-decision.md), [ADR-0002](../../adrs/0002-second-decision.md) |\n"
	if gotNoWeb != wantNoWeb {
		t.Fatalf("legacy done block (no web URL) mismatch:\n--- got ---\n%s\n--- want ---\n%s", gotNoWeb, wantNoWeb)
	}
}

// TestArtifactBlockDoneWithEvidenceAddsSpecMerged: a done change that carries
// the "## Build evidence" section was built after the cutover — its plan and
// results stay on the metadata branch (relative rows), and its spec copy
// merged with the PR, so a "Spec (merged)" row links that copy absolutely on
// the integration branch, right after the Spec row. A not-yet-done change with
// the section gets no "Spec (merged)" row: nothing has merged.
func TestArtifactBlockDoneWithEvidenceAddsSpecMerged(t *testing.T) {
	c := fullChange(domain.StatusDone, true, domain.LocationArchive, deltaArchivePath)

	got, err := render.ArtifactBlockContent(c, adrSnapshot(), githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent: %v", err)
	}
	want := "| Artifact | Link |\n|---|---|\n" +
		"| Spec | [2026-08-16-delta-change-design.md](../../superpowers/specs/2026-08-16-delta-change-design.md) |\n" +
		"| Spec (merged) | [2026-08-16-delta-change-design.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/specs/2026-08-16-delta-change-design.md) |\n" +
		"| Plan | [2026-08-16-delta-change.md](../../superpowers/plans/2026-08-16-delta-change.md) |\n" +
		"| Results | [2026-08-16-delta-change-results.md](../../results/2026-08-16-delta-change-results.md) |\n" +
		"| ADRs | [ADR-0001](../../adrs/0001-first-decision.md), [ADR-0002](../../adrs/0002-second-decision.md) |\n"
	if got != want {
		t.Fatalf("done-with-evidence block mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	gotNoWeb, err := render.ArtifactBlockContent(c, adrSnapshot(), relativeLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent(no web URL): %v", err)
	}
	if row := "| Spec (merged) | `docs/superpowers/specs/2026-08-16-delta-change-design.md` |\n"; !strings.Contains(gotNoWeb, row) {
		t.Fatalf("no-web-URL Spec (merged) row missing %q:\n%s", row, gotNoWeb)
	}

	implemented := fullChange(domain.StatusImplemented, true, domain.LocationActive, deltaActivePath)
	gotImpl, err := render.ArtifactBlockContent(implemented, adrSnapshot(), githubLink)
	if err != nil {
		t.Fatalf("ArtifactBlockContent(implemented): %v", err)
	}
	if strings.Contains(gotImpl, "Spec (merged)") {
		t.Fatalf("a not-yet-done change rendered a Spec (merged) row:\n%s", gotImpl)
	}
}

// TestArtifactBacklinkContentRelative: a file on the metadata branch links back
// to its record relatively from the file's own directory, so the link follows
// the record when it is archived (the archive transaction re-stamps it).
func TestArtifactBacklinkContentRelative(t *testing.T) {
	const plan = "docs/superpowers/plans/p.md"
	cases := []struct {
		name string
		c    domain.Change
		want string
	}{
		{"active", alphaChange(), "../../changes/active/0007-alpha-change.md"},
		{"archived", domain.NewChange(domain.ChangeSpec{
			ID: 7, Slug: "alpha-change", Title: "Alpha change",
			Location: domain.LocationArchive,
			Path:     "docs/changes/archive/2026-08-16-0007-alpha-change.md",
		}), "../../changes/archive/2026-08-16-0007-alpha-change.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := render.ArtifactBacklinkContent(tc.c, plan)
			want := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n" +
				"> ↩ **[Change 0007 — Alpha change](" + tc.want + ")**\n" +
				"<!-- docket:backlink:end -->\n"
			if got != want {
				t.Fatalf("backlink mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
			if resolved := path.Join(path.Dir(plan), tc.want); resolved != tc.c.Path() {
				t.Fatalf("backlink resolves to %q, not the record %q", resolved, tc.c.Path())
			}
		})
	}
}

// TestPRArtifactLinksContent: the PR description links the plan and results
// absolutely on the metadata branch (a PR body has no branch to be relative
// to); nothing renders without a web URL or without either field.
func TestPRArtifactLinksContent(t *testing.T) {
	got := render.PRArtifactLinksContent(betaChange(), githubLink)
	want := "- Plan: [2026-08-16-beta-change.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-16-beta-change.md)\n" +
		"- Results: [2026-08-16-beta-change-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-16-beta-change-results.md)\n"
	if got != want {
		t.Fatalf("PR artifact links mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if got := render.PRArtifactLinksContent(betaChange(), relativeLink); got != "" {
		t.Fatalf("no web URL rendered %q, want empty", got)
	}
	if got := render.PRArtifactLinksContent(alphaChange(), githubLink); got != "" {
		t.Fatalf("a change with neither plan nor results rendered %q, want empty", got)
	}
	planOnly := domain.NewChange(domain.ChangeSpec{
		ID: 8, Slug: "beta-change", Title: "Beta change",
		Plan: optString("docs/superpowers/plans/2026-08-16-beta-change.md"),
		Path: "docs/changes/active/0008-beta-change.md",
	})
	wantPlan := "- Plan: [2026-08-16-beta-change.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-16-beta-change.md)\n"
	if got := render.PRArtifactLinksContent(planOnly, githubLink); got != wantPlan {
		t.Fatalf("plan-only PR links = %q, want %q", got, wantPlan)
	}
}
