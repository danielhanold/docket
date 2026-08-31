package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// derivedTestConfig is the minimal resolved config the derived-view comparison
// reads: the changes and ADR directories.
func derivedTestConfig() config.Effective {
	return config.Effective{
		ChangesDir: config.Value[string]{Value: "docs/changes"},
		ADRsDir:    config.Value[string]{Value: "docs/adrs"},
	}
}

// changeRecordBytes builds a change record with the given frontmatter body and a
// managed artifacts block whose interior is blockBody (empty for an empty block).
func changeRecordBytes(frontmatter, blockBody string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(frontmatter)
	b.WriteString("---\n\n## Artifacts\n\n")
	b.WriteString("<!-- docket:artifacts:start (generated — do not hand-edit) -->\n")
	if blockBody != "" {
		b.WriteString(blockBody)
	}
	b.WriteString("<!-- docket:artifacts:end -->\n\n## Why\n\nbody\n")
	return []byte(b.String())
}

const derivedChangeFM = "id: 1\nslug: example\ntitle: Example change\nstatus: proposed\npriority: medium\ntype: feature\ncreated: 2026-08-30\nupdated: 2026-08-30\nspec: docs/superpowers/specs/2026-08-30-example-design.md\n"

// canonicalChangeRecord returns a change record whose artifact-links block is the
// canonical render for its frontmatter — the byte state a clean check accepts.
func canonicalChangeRecord(t *testing.T, cfg config.Effective) (path string, canonical []byte) {
	t.Helper()
	path = "docs/changes/active/0001-example.md"
	empty := changeRecordBytes(derivedChangeFM, "")
	snap, ok := buildCorpusSnapshot(cfg, []corpusRecord{{
		path: path, bytes: empty, kind: repository.KindChange, location: repository.LocationActive,
	}})
	if !ok {
		t.Fatal("buildCorpusSnapshot failed for the example record")
	}
	c, out := snap.Change(1)
	if out != 0 { // domain.LookupFound == 0
		t.Fatalf("example change absent from snapshot (outcome %d)", out)
	}
	body, err := render.ArtifactBlockContent(c, snap, render.LinkContext{MetadataBranch: reposetup.MetadataBranchName})
	if err != nil {
		t.Fatalf("render artifact block: %v", err)
	}
	doc, err := document.Parse(empty)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	canonical, err = applyArtifactBlock(doc, body)
	if err != nil {
		t.Fatalf("apply canonical block: %v", err)
	}
	return path, canonical
}

// TestDerivedViewFindingsCleanCorpus proves a corpus whose board, ADR index, and
// artifact-links blocks all match the canonical render yields no derived finding.
func TestDerivedViewFindingsCleanCorpus(t *testing.T) {
	cfg := derivedTestConfig()
	path, canonical := canonicalChangeRecord(t, cfg)
	rec := corpusRecord{path: path, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive}

	snap, _ := buildCorpusSnapshot(cfg, []corpusRecord{rec})
	board, _ := render.Board(render.BoardInput{Snapshot: snap})
	adr, _ := render.ADRIndex(snap)

	corpus := checkCorpus{
		records:  []corpusRecord{rec},
		link:     render.LinkContext{MetadataBranch: reposetup.MetadataBranchName},
		board:    corpusFile{present: true, bytes: board},
		adrIndex: corpusFile{present: true, bytes: adr},
	}
	if got := derivedViewFindings(cfg, corpus); len(got) != 0 {
		t.Errorf("clean corpus produced findings: %+v", got)
	}
}

// TestDerivedViewFindingsStaleBoard proves board bytes that differ from the
// canonical render surface a repairable board-stale finding, and matching board
// bytes do not.
func TestDerivedViewFindingsStaleBoard(t *testing.T) {
	cfg := derivedTestConfig()
	path, canonical := canonicalChangeRecord(t, cfg)
	rec := corpusRecord{path: path, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive}
	corpus := checkCorpus{
		records: []corpusRecord{rec},
		link:    render.LinkContext{MetadataBranch: reposetup.MetadataBranchName},
		board:   corpusFile{present: true, bytes: []byte("# Backlog\n\nstale bytes\n")},
	}
	got := findingByCode(derivedViewFindings(cfg, corpus), reposetup.CodeBoardStale)
	if got == nil {
		t.Fatal("stale board did not produce a board-stale finding")
	}
	if !got.Repairable {
		t.Errorf("board-stale finding must be repairable")
	}
}

// TestDerivedViewFindingsStaleArtifactLinks proves a change record whose managed
// block is empty while its frontmatter carries a spec is a repairable
// artifact-links-stale finding.
func TestDerivedViewFindingsStaleArtifactLinks(t *testing.T) {
	cfg := derivedTestConfig()
	rec := corpusRecord{
		path:     "docs/changes/active/0001-example.md",
		bytes:    changeRecordBytes(derivedChangeFM, ""), // empty block, but spec present
		kind:     repository.KindChange,
		location: repository.LocationActive,
	}
	corpus := checkCorpus{records: []corpusRecord{rec}, link: render.LinkContext{MetadataBranch: reposetup.MetadataBranchName}}
	got := findingByCode(derivedViewFindings(cfg, corpus), reposetup.CodeArtifactLinksStale)
	if got == nil {
		t.Fatal("stale artifact-links block did not produce an artifact-links-stale finding")
	}
	if !got.Repairable {
		t.Errorf("artifact-links-stale must be repairable")
	}
}

// TestDerivedViewFindingsMissingArtifactLinks proves a change record with a spec
// but NO managed block is a repairable artifact-links-missing finding.
func TestDerivedViewFindingsMissingArtifactLinks(t *testing.T) {
	cfg := derivedTestConfig()
	rec := corpusRecord{
		path:     "docs/changes/active/0001-example.md",
		bytes:    []byte("---\n" + derivedChangeFM + "---\n\n## Why\n\nno block here\n"),
		kind:     repository.KindChange,
		location: repository.LocationActive,
	}
	corpus := checkCorpus{records: []corpusRecord{rec}, link: render.LinkContext{MetadataBranch: reposetup.MetadataBranchName}}
	got := findingByCode(derivedViewFindings(cfg, corpus), reposetup.CodeArtifactLinksMissing)
	if got == nil {
		t.Fatal("missing artifact-links block did not produce an artifact-links-missing finding")
	}
	if !got.Repairable {
		t.Errorf("artifact-links-missing must be repairable")
	}
}

// TestDerivedViewFindingsMalformedMarkers proves a change record whose managed
// artifact-links markers are unbalanced is a NON-repairable artifact-links-malformed
// finding — the automatic repair must never touch an unbalanced block.
func TestDerivedViewFindingsMalformedMarkers(t *testing.T) {
	cfg := derivedTestConfig()
	// A start marker with no matching end: document.Parse rejects it.
	src := []byte("---\n" + derivedChangeFM + "---\n\n## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| Artifact | Link |\n\n## Why\n\nbody\n")
	rec := corpusRecord{path: "docs/changes/active/0001-example.md", bytes: src, kind: repository.KindChange, location: repository.LocationActive}
	corpus := checkCorpus{records: []corpusRecord{rec}, link: render.LinkContext{MetadataBranch: reposetup.MetadataBranchName}}
	got := findingByCode(derivedViewFindings(cfg, corpus), reposetup.CodeArtifactLinksMalformed)
	if got == nil {
		t.Fatal("unbalanced markers did not produce an artifact-links-malformed finding")
	}
	if got.Repairable {
		t.Errorf("artifact-links-malformed must NOT be repairable")
	}
}

// TestCheckCorpusOutcomeReadErrorIsError proves a corpus READ ERROR yields an
// error-severity corpus-unreadable finding — never a clean absence. This is the
// read-error behavior contract: a check must not fabricate absence when it could
// not read the corpus.
func TestCheckCorpusOutcomeReadErrorIsError(t *testing.T) {
	fm, extra := checkCorpusOutcome(derivedTestConfig(), checkCorpus{}, errors.New("object missing"))
	if len(fm) != 0 {
		t.Errorf("read error must not yield frontmatter findings, got %+v", fm)
	}
	if len(extra) != 1 {
		t.Fatalf("read error must yield exactly one finding, got %d: %+v", len(extra), extra)
	}
	if extra[0].Code != reposetup.CodeCorpusUnreadable {
		t.Errorf("code = %q, want %q", extra[0].Code, reposetup.CodeCorpusUnreadable)
	}
	if extra[0].Severity != reposetup.SeverityError {
		t.Errorf("severity = %q, want error", extra[0].Severity)
	}
}

// findingByCode returns a pointer to the first derived finding with the given
// code, or nil.
func findingByCode(findings []reposetup.DerivedFinding, code string) *reposetup.DerivedFinding {
	for i := range findings {
		if findings[i].Code == code {
			return &findings[i]
		}
	}
	return nil
}
