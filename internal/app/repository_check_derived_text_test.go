package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// derivedScenarios builds one check corpus per generated-view finding code the
// check can produce, each with a `.docket` record locator, so the text tests
// read every producer's wording through the real derived-view pass.
func derivedScenarios(t *testing.T) []checkCorpus {
	t.Helper()
	cfg := derivedTestConfig()
	path, canonical := canonicalChangeRecord(t, cfg)
	canonicalRec := corpusRecord{path: path, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive}
	link := render.LinkContext{MetadataBranch: layout.SharedName}
	loc := reposetup.RecordLocator{Checkout: ".docket"}
	active := func(p string, b []byte) corpusRecord {
		return corpusRecord{path: p, bytes: b, kind: repository.KindChange, location: repository.LocationActive}
	}
	malformedFM := strings.Replace(derivedChangeFM, "id: 1\nslug: example", "id: 4\nslug: malformed", 1)
	malformedRec := active("docs/changes/active/0004-malformed.md", []byte("---\n"+malformedFM+"---\n\n## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| Artifact | Link |\n\n## Why\n\nbody\n"))
	noBlockRec := active("docs/changes/active/0001-example.md", []byte("---\n"+derivedChangeFM+"---\n\n## Why\n\nbody\n"))
	startOnlyBacklink := "<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ dangling\n\n# body\n"

	return []checkCorpus{
		{records: []corpusRecord{canonicalRec}, link: link, locator: loc,
			board: corpusFile{present: true, bytes: []byte("# Backlog\n\nstale bytes\n")}},
		{records: []corpusRecord{canonicalRec}, link: link, locator: loc,
			adrIndex: corpusFile{present: true, bytes: []byte("# ADRs\n\nstale bytes\n")}},
		{records: []corpusRecord{active("docs/changes/active/0001-example.md", changeRecordBytes(derivedChangeFM, ""))}, link: link, locator: loc},
		{records: []corpusRecord{noBlockRec}, link: link, locator: loc},
		{records: []corpusRecord{malformedRec}, link: link, locator: loc},
		{records: []corpusRecord{backlinkRecord("07", "t", false)}, link: link, locator: loc,
			artifacts: map[string][]byte{backlinkSpecPath: []byte(absoluteBacklinkArtifact)}},
		{records: []corpusRecord{backlinkRecord("07", "t", false)}, link: link, locator: loc,
			artifacts: map[string][]byte{backlinkSpecPath: []byte(startOnlyBacklink)}},
		{records: []corpusRecord{backlinkRecord("07", "t", false), backlinkRecord("08", "u", false)}, link: link, locator: loc,
			artifacts: map[string][]byte{backlinkSpecPath: []byte(absoluteBacklinkArtifact)}},
	}
}

// derivedScenarioFindings runs every scenario through the derived-view pass
// and returns the first lifted finding per code.
func derivedScenarioFindings(t *testing.T) map[string]reposetup.Finding {
	t.Helper()
	cfg := derivedTestConfig()
	out := map[string]reposetup.Finding{}
	for _, corpus := range derivedScenarios(t) {
		for _, df := range derivedViewFindings(cfg, corpus) {
			if _, ok := out[df.Code]; !ok {
				out[df.Code] = df.Finding()
			}
		}
	}
	return out
}

// TestRepairableGeneratedViewRemedyOffersNoHandEdit is the generated-view
// remedy guard: a generated file is never hand-edited, so a repairable
// finding's remedy names only `docket repository repair`.
//
// Mutation probe: append ", or edit the file by hand." to one producer's
// remedy — this test must redden.
func TestRepairableGeneratedViewRemedyOffersNoHandEdit(t *testing.T) {
	repairable := 0
	for code, f := range derivedScenarioFindings(t) {
		if f.Repairable == nil || !*f.Repairable {
			continue
		}
		repairable++
		if !strings.HasPrefix(f.Remedy, "Run `docket repository repair`") {
			t.Errorf("%s remedy must open with the repair command: %q", code, f.Remedy)
		}
		low := strings.ToLower(f.Remedy)
		if strings.Contains(low, " or ") || strings.Contains(low, "hand") {
			t.Errorf("%s remedy offers an alternative to the repair: %q", code, f.Remedy)
		}
	}
	if repairable < 5 {
		t.Errorf("population floor: %d repairable generated-view findings, want >= 5", repairable)
	}
	if r := (reposetup.DerivedFinding{Repairable: true}).Finding().Remedy; strings.Contains(strings.ToLower(r), "hand") {
		t.Errorf("default repairable remedy offers a hand edit: %q", r)
	}
}

// TestGeneratedViewFindingsCarryThreeParts proves every producible
// generated-view code carries a message, an impact, and a remedy in plain
// words, with the record or artifact it is about named.
func TestGeneratedViewFindingsCarryThreeParts(t *testing.T) {
	got := derivedScenarioFindings(t)
	for _, code := range []string{
		reposetup.CodeBoardStale, reposetup.CodeADRIndexStale,
		reposetup.CodeArtifactLinksStale, reposetup.CodeArtifactLinksMissing, reposetup.CodeArtifactLinksMalformed,
		reposetup.CodeArtifactBacklinkStale, reposetup.CodeArtifactBacklinkMalformed, reposetup.CodeArtifactBacklinkShared,
	} {
		f, ok := got[code]
		if !ok {
			t.Errorf("no scenario produced %s (got %v)", code, derivedFindingCodes(got))
			continue
		}
		if f.Message == "" || f.Impact == "" || f.Remedy == "" {
			t.Errorf("%s is missing a part: %+v", code, f)
		}
		if strings.Contains(strings.ToLower(f.Message), "canonical") {
			t.Errorf("%s message uses internal vocabulary: %q", code, f.Message)
		}
	}
	if r := got[reposetup.CodeArtifactLinksMalformed].Remedy; !strings.Contains(r, "`.docket/docs/changes/active/0004-malformed.md`") {
		t.Errorf("artifact-links-malformed remedy does not name the record in .docket: %q", r)
	}
	if m := got[reposetup.CodeArtifactBacklinkShared].Message; !strings.Contains(m, "0007") || !strings.Contains(m, "0008") {
		t.Errorf("artifact-backlink-shared message does not name both changes: %q", m)
	}
}

// TestCorpusUnreadableText pins the corpus-unreadable wording.
func TestCorpusUnreadableText(t *testing.T) {
	f := corpusUnreadableFinding(errors.New("object missing"))
	if want := "docket couldn't read the records on the metadata branch: object missing."; f.Message != want {
		t.Errorf("Message = %q, want %q", f.Message, want)
	}
	if want := "None of the record checks ran, so this result says nothing about the records."; f.Impact != want {
		t.Errorf("Impact = %q, want %q", f.Impact, want)
	}
	if want := "Fix the read error, then run `docket repository check` again."; f.Remedy != want {
		t.Errorf("Remedy = %q, want %q", f.Remedy, want)
	}
}

func derivedFindingCodes(m map[string]reposetup.Finding) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
