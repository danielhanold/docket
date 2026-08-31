package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// TestSplitDerivedFindings proves the partition keeps repairable findings apart
// from the manual-review diagnostics, in order.
func TestSplitDerivedFindings(t *testing.T) {
	in := []reposetup.DerivedFinding{
		{Code: reposetup.CodeBoardStale, Path: "b", Repairable: true},
		{Code: reposetup.CodeArtifactLinksMalformed, Path: "m", Repairable: false},
		{Code: reposetup.CodeADRIndexStale, Path: "a", Repairable: true},
	}
	repairable, diagnostics := splitDerivedFindings(in)
	if len(repairable) != 2 || len(diagnostics) != 1 {
		t.Fatalf("repairable=%d diagnostics=%d, want 2 and 1", len(repairable), len(diagnostics))
	}
	if diagnostics[0].Code != reposetup.CodeArtifactLinksMalformed {
		t.Errorf("diagnostic = %q, want the malformed finding", diagnostics[0].Code)
	}
}

// TestDerivedRepairFilesSortedUnique proves the file set is sorted and de-duped
// even when two findings touch the same file.
func TestDerivedRepairFilesSortedUnique(t *testing.T) {
	files := derivedRepairFiles([]reposetup.DerivedFinding{
		{Path: "z", Repairable: true},
		{Path: "a", Repairable: true},
		{Path: "a", Repairable: true},
	})
	if len(files) != 2 || files[0] != "a" || files[1] != "z" {
		t.Errorf("files = %v, want [a z]", files)
	}
}

// TestDerivedRepairConfirmationRequiredNamesYes proves the unauthorized preview
// refuses as invalid-state with reason confirmation-required, carries the pinned
// metadata revision, names the repaired file set, and names --yes.
func TestDerivedRepairConfirmationRequiredNamesYes(t *testing.T) {
	out := derivedRepairConfirmationRequired("abc123", []string{"docs/changes/BOARD.md"},
		"docket repository migrate — derived-view repair\n")
	if out.Result != ResultInvalidState {
		t.Fatalf("Result = %q, want invalid-state", out.Result)
	}
	if out.RepositoryState != "confirmation-required" {
		t.Errorf("RepositoryState = %q, want confirmation-required", out.RepositoryState)
	}
	if out.SourceRevision != "abc123" {
		t.Errorf("SourceRevision = %q, want the pinned metadata tip", out.SourceRevision)
	}
	if len(out.RepairedViews) != 1 || out.RepairedViews[0] != "docs/changes/BOARD.md" {
		t.Errorf("RepairedViews = %v, want the file set", out.RepairedViews)
	}
	if !strings.Contains(out.HumanText(), "--yes") {
		t.Errorf("human %q must name --yes", out.HumanText())
	}
}

// TestDerivedRepairAppliedNamesRevisionAndPending proves the applied document
// names the new metadata revision, the repaired files, and a non-destructive
// local sync remedy.
func TestDerivedRepairAppliedNamesRevisionAndPending(t *testing.T) {
	out := derivedRepairApplied(setupContext{}, "newtip", "priortip", []string{"docs/changes/BOARD.md"})
	if out.Result != ResultApplied {
		t.Fatalf("Result = %q, want applied", out.Result)
	}
	if out.MetadataTip != "newtip" {
		t.Errorf("MetadataTip = %q, want newtip", out.MetadataTip)
	}
	if len(out.PendingLocal) == 0 || !strings.Contains(strings.Join(out.PendingLocal, " "), "docket repository prepare") {
		t.Errorf("PendingLocal = %v, want a `docket repository prepare` sync remedy", out.PendingLocal)
	}
}

// TestComposeDerivedRepairBoardBytes proves the composed board repair equals the
// canonical render over the same snapshot — the repair recomputes canonical bytes
// rather than merging stale output.
func TestComposeDerivedRepairBoardBytes(t *testing.T) {
	cfg := derivedTestConfig()
	path, canonical := canonicalChangeRecord(t, cfg)
	rec := corpusRecord{path: path, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive}
	snap, _ := buildCorpusSnapshot(cfg, []corpusRecord{rec})
	corpus := checkCorpus{records: []corpusRecord{rec}, link: render.LinkContext{MetadataBranch: reposetup.MetadataBranchName}}
	recByPath := map[string]corpusRecord{rec.path: rec}

	got, err := composeDerivedRepairBytes(setupContext{cfg: cfg}, snap, corpus, recByPath, boardCorpusPath(cfg))
	if err != nil {
		t.Fatalf("compose board: %v", err)
	}
	want, _ := render.Board(render.BoardInput{Snapshot: snap, Presentation: boardPresentation(cfg)})
	if !bytes.Equal(got, want) {
		t.Errorf("composed board != canonical render")
	}
}

// TestComposeDerivedRepairArtifactLinksBytes proves repairing a stale
// artifact-links record yields the canonical record and repairs the drift: the
// composed bytes carry the Spec row the stale (empty) block lacked, and re-running
// the check over them reports no artifact-links drift.
func TestComposeDerivedRepairArtifactLinksBytes(t *testing.T) {
	cfg := derivedTestConfig()
	path := "docs/changes/active/0001-example.md"
	stale := corpusRecord{path: path, bytes: changeRecordBytes(derivedChangeFM, ""), kind: repository.KindChange, location: repository.LocationActive}
	snap, _ := buildCorpusSnapshot(cfg, []corpusRecord{stale})
	corpus := checkCorpus{records: []corpusRecord{stale}, link: render.LinkContext{MetadataBranch: reposetup.MetadataBranchName}}
	recByPath := map[string]corpusRecord{path: stale}

	got, err := composeDerivedRepairBytes(setupContext{cfg: cfg}, snap, corpus, recByPath, path)
	if err != nil {
		t.Fatalf("compose artifact-links: %v", err)
	}
	if !bytes.Contains(got, []byte("| Spec |")) {
		t.Errorf("repaired record must carry the canonical Spec row; got:\n%s", got)
	}
	// The repaired record is clean under a re-check: no artifact-links drift.
	repaired := corpusRecord{path: path, bytes: got, kind: repository.KindChange, location: repository.LocationActive}
	recheck := checkCorpus{records: []corpusRecord{repaired}, link: corpus.link}
	if f := findingByCode(derivedViewFindings(cfg, recheck), reposetup.CodeArtifactLinksStale); f != nil {
		t.Errorf("re-check of the repaired record still reports stale: %+v", f)
	}
}
