package app

import (
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

const (
	backlinkSpecPath = "docs/superpowers/specs/x.md"
	backlinkPlanPath = "docs/superpowers/plans/x-plan.md"
)

// backlinkChangeFM is one change whose spec (and, when withPlan, plan) points
// at the shared fixture artifact paths.
func backlinkChangeFM(id, slug string, withPlan bool) string {
	fm := "id: " + id + "\nslug: " + slug + "\ntitle: T\nstatus: proposed\npriority: medium\ntype: feature\ncreated: 2026-08-30\nupdated: 2026-08-30\nspec: " + backlinkSpecPath + "\n"
	if withPlan {
		fm += "plan: " + backlinkPlanPath + "\n"
	}
	return fm
}

func backlinkRecord(id, slug string, withPlan bool) corpusRecord {
	return corpusRecord{
		path:     "docs/changes/active/00" + id + "-" + slug + ".md",
		bytes:    changeRecordBytes(backlinkChangeFM(id, slug, withPlan), ""),
		kind:     repository.KindChange,
		location: repository.LocationActive,
	}
}

// absoluteBacklinkArtifact is a metadata artifact whose generated backlink
// block still carries an absolute same-branch web URL.
const absoluteBacklinkArtifact = "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ **[Change 0007 — T](https://github.com/o/r/blob/docket/docs/changes/active/0007-t.md)**\n<!-- docket:backlink:end -->\n\n# body\n"

// backlinkFindingsFor builds the corpus snapshot over recs and returns only
// the artifact-backlinks findings of the full derived-view pass, so the
// production wiring (derivedViewFindings) is what the tests exercise.
func backlinkFindingsFor(t *testing.T, recs []corpusRecord, artifacts map[string][]byte) []reposetup.DerivedFinding {
	t.Helper()
	cfg := derivedTestConfig()
	corpus := checkCorpus{
		records:   recs,
		link:      render.LinkContext{MetadataBranch: layout.SharedName},
		artifacts: artifacts,
	}
	var out []reposetup.DerivedFinding
	for _, df := range derivedViewFindings(cfg, corpus) {
		if df.View == reposetup.DerivedViewArtifactBacklinks {
			out = append(out, df)
		}
	}
	return out
}

func backlinkSnapshotChange(t *testing.T, recs []corpusRecord, id int) domain.Change {
	t.Helper()
	snap, ok := buildCorpusSnapshot(derivedTestConfig(), recs)
	if !ok {
		t.Fatal("buildCorpusSnapshot failed")
	}
	c, out := snap.Change(domain.ChangeID(id))
	if out != domain.LookupFound {
		t.Fatalf("change %d absent from snapshot (outcome %d)", id, out)
	}
	return c
}

func TestArtifactBacklinkAbsoluteSpecIsStale(t *testing.T) {
	recs := []corpusRecord{backlinkRecord("07", "t", false)}
	got := backlinkFindingsFor(t, recs, map[string][]byte{backlinkSpecPath: []byte(absoluteBacklinkArtifact)})
	if len(got) != 1 {
		t.Fatalf("findings = %+v, want exactly one", got)
	}
	f := got[0]
	if f.Code != reposetup.CodeArtifactBacklinkStale || f.Path != backlinkSpecPath || !f.Repairable {
		t.Errorf("finding = %+v, want a repairable %s on %s", f, reposetup.CodeArtifactBacklinkStale, backlinkSpecPath)
	}
}

func TestArtifactBacklinkCanonicalSpecIsClean(t *testing.T) {
	recs := []corpusRecord{backlinkRecord("07", "t", false)}
	c := backlinkSnapshotChange(t, recs, 7)
	canonical := assembleSpecFile(render.ArtifactBacklinkContent(c, backlinkSpecPath), "# body")
	if got := backlinkFindingsFor(t, recs, map[string][]byte{backlinkSpecPath: canonical}); len(got) != 0 {
		t.Errorf("canonical backlink produced findings: %+v", got)
	}
}

func TestArtifactBacklinkNoBlockIsClean(t *testing.T) {
	recs := []corpusRecord{backlinkRecord("07", "t", false)}
	if got := backlinkFindingsFor(t, recs, map[string][]byte{backlinkSpecPath: []byte("# body\n")}); len(got) != 0 {
		t.Errorf("block-less artifact produced findings: %+v", got)
	}
}

func TestArtifactBacklinkAbsentArtifactIsClean(t *testing.T) {
	recs := []corpusRecord{backlinkRecord("07", "t", false)}
	if got := backlinkFindingsFor(t, recs, nil); len(got) != 0 {
		t.Errorf("absent artifact produced findings: %+v", got)
	}
}

func TestArtifactBacklinkStartMarkerOnlyIsMalformed(t *testing.T) {
	recs := []corpusRecord{backlinkRecord("07", "t", false)}
	src := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ dangling\n\n# body\n"
	got := backlinkFindingsFor(t, recs, map[string][]byte{backlinkSpecPath: []byte(src)})
	if len(got) != 1 || got[0].Code != reposetup.CodeArtifactBacklinkMalformed || got[0].Repairable {
		t.Errorf("findings = %+v, want one non-repairable %s", got, reposetup.CodeArtifactBacklinkMalformed)
	}
}

func TestArtifactBacklinkSharedPathIsManualReview(t *testing.T) {
	recs := []corpusRecord{backlinkRecord("07", "t", false), backlinkRecord("08", "u", false)}
	got := backlinkFindingsFor(t, recs, map[string][]byte{backlinkSpecPath: []byte(absoluteBacklinkArtifact)})
	if len(got) != 1 || got[0].Code != reposetup.CodeArtifactBacklinkShared || got[0].Repairable || got[0].Path != backlinkSpecPath {
		t.Errorf("findings = %+v, want one non-repairable %s on %s", got, reposetup.CodeArtifactBacklinkShared, backlinkSpecPath)
	}
}

func TestArtifactBacklinkAbsolutePlanIsStale(t *testing.T) {
	recs := []corpusRecord{backlinkRecord("07", "t", true)}
	c := backlinkSnapshotChange(t, recs, 7)
	canonicalSpec := assembleSpecFile(render.ArtifactBacklinkContent(c, backlinkSpecPath), "# body")
	got := backlinkFindingsFor(t, recs, map[string][]byte{
		backlinkSpecPath: canonicalSpec,
		backlinkPlanPath: []byte(absoluteBacklinkArtifact),
	})
	if len(got) != 1 || got[0].Code != reposetup.CodeArtifactBacklinkStale || got[0].Path != backlinkPlanPath {
		t.Errorf("findings = %+v, want one %s on %s", got, reposetup.CodeArtifactBacklinkStale, backlinkPlanPath)
	}
}

// TestCanonicalArtifactBacklinkRewritesOnlyTheBlock proves the one renderer
// replaces the block interior and leaves every other byte alone.
func TestCanonicalArtifactBacklinkRewritesOnlyTheBlock(t *testing.T) {
	recs := []corpusRecord{backlinkRecord("07", "t", false)}
	c := backlinkSnapshotChange(t, recs, 7)
	out, has, err := canonicalArtifactBacklink([]byte(absoluteBacklinkArtifact), c, backlinkSpecPath)
	if err != nil || !has {
		t.Fatalf("canonicalArtifactBacklink: has=%v err=%v", has, err)
	}
	want := assembleSpecFile(render.ArtifactBacklinkContent(c, backlinkSpecPath), "# body")
	if string(out) != string(want) {
		t.Errorf("out = %q\nwant %q", out, want)
	}
	src := []byte("# body\n")
	out, has, err = canonicalArtifactBacklink(src, c, backlinkSpecPath)
	if err != nil || has || string(out) != string(src) {
		t.Errorf("block-less: out=%q has=%v err=%v, want unchanged, false, nil", out, has, err)
	}
}
