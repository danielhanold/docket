package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// repairArchivedDone is a minimal valid ARCHIVED done record (the same shape the
// migration fixtures use) carrying the given claimed_at line.
func repairArchivedDone(claimedLine string) []byte {
	return []byte("---\nid: 3\nslug: archived-change\nstatus: done\ntitle: Change archived-change\ntype: feature\n" +
		claimedLine + "\n---\n\nBody for archived-change.\n")
}

const repairArchivedPath = "docs/changes/archive/2026-01-02-0003-archived-change.md"

func repairLink() render.LinkContext {
	return render.LinkContext{MetadataBranch: layout.SharedName}
}

// TestPlanRepositoryRepairComposesFrontmatterAndArtifactLinks proves a record that
// needs BOTH a frontmatter repair (an unsafe `title: yes`) and an artifact-links
// re-render is written once, as the composed bytes, and that a re-check over the
// composed bytes reports neither finding (Review Focus 2).
func TestPlanRepositoryRepairComposesFrontmatterAndArtifactLinks(t *testing.T) {
	cfg := derivedTestConfig()
	path := "docs/changes/active/0001-example.md"
	fm := strings.Replace(derivedChangeFM, "title: Example change", "title: yes", 1)
	rec := corpusRecord{path: path, bytes: changeRecordBytes(fm, ""), kind: repository.KindChange, location: repository.LocationActive}
	corpus := checkCorpus{records: []corpusRecord{rec}, link: repairLink()}

	plan, err := planRepositoryRepair(setupContext{cfg: cfg}, corpus)
	if err != nil {
		t.Fatalf("planRepositoryRepair: %v", err)
	}
	if len(plan.files) != 1 || plan.files[0] != path {
		t.Fatalf("files = %v, want exactly [%s]", plan.files, path)
	}
	if len(plan.frontmatter) != 1 || plan.frontmatter[0].Code != reposetup.RepairQuoteScalar {
		t.Errorf("frontmatter = %+v, want one quote-unsafe-scalar repair", plan.frontmatter)
	}
	if findingByCode(plan.derived, reposetup.CodeArtifactLinksStale) == nil {
		t.Errorf("derived = %+v, want artifact-links-stale computed over the repaired corpus", plan.derived)
	}
	got := plan.contents[path]
	if !bytes.Contains(got, []byte("title: 'yes'")) || !bytes.Contains(got, []byte("| Spec |")) {
		t.Fatalf("composed bytes must carry BOTH repairs; got:\n%s", got)
	}
	recheck := checkCorpus{records: []corpusRecord{{path: path, bytes: got, kind: repository.KindChange, location: repository.LocationActive}}, link: repairLink()}
	for _, f := range corpusFindings(cfg, recheck.records) {
		t.Errorf("re-check frontmatter finding on composed bytes: %+v", f)
	}
	for _, f := range derivedViewFindings(cfg, recheck) {
		t.Errorf("re-check derived finding on composed bytes: %+v", f)
	}
}

// TestPlanRepositoryRepairBoardRecomputedOverRepairedCorpus proves the board is
// re-rendered from the snapshot of the FRONTMATTER-REPAIRED corpus, so the board
// blob and the record blob land canonical in one pass.
func TestPlanRepositoryRepairBoardRecomputedOverRepairedCorpus(t *testing.T) {
	cfg := derivedTestConfig()
	path, canonical := canonicalChangeRecord(t, cfg)
	stamped := corpusRecord{path: repairArchivedPath, bytes: repairArchivedDone("claimed_at: 2026-08-01T10:00:00Z"), kind: repository.KindChange, location: repository.LocationArchive}
	active := corpusRecord{path: path, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive}
	corpus := checkCorpus{
		records: []corpusRecord{active, stamped},
		link:    repairLink(),
		board:   corpusFile{present: true, bytes: []byte("# Backlog\n\nstale bytes\n")},
	}
	plan, err := planRepositoryRepair(setupContext{cfg: cfg}, corpus)
	if err != nil {
		t.Fatalf("planRepositoryRepair: %v", err)
	}
	board := boardCorpusPath(cfg)
	if !planHasFile(plan.files, board) || !planHasFile(plan.files, repairArchivedPath) {
		t.Fatalf("files = %v, want the board and the stamped archived record", plan.files)
	}
	repairedRecs := []corpusRecord{active, {path: repairArchivedPath, bytes: plan.contents[repairArchivedPath], kind: repository.KindChange, location: repository.LocationArchive}}
	snap, ok := buildCorpusSnapshot(cfg, repairedRecs)
	if !ok {
		t.Fatal("snapshot of the repaired corpus failed")
	}
	want, err := renderCanonicalBoard(snap, corpusBoardUnrenderable(cfg, repairedRecs), boardPresentation(cfg))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.Equal(plan.contents[board], want) {
		t.Errorf("board bytes are not the canonical render over the repaired corpus")
	}
	if bytes.Contains(plan.contents[repairArchivedPath], []byte("claimed_at")) {
		t.Errorf("stamped archived record still carries claimed_at:\n%s", plan.contents[repairArchivedPath])
	}
}

// TestPlanRepositoryRepairEmptyClaimedAtIsClean proves the 269-record shape — a
// final archived record with the empty cleared claimed_at — plans nothing, while
// the same record with a real stamp (the control) plans the drop.
func TestPlanRepositoryRepairEmptyClaimedAtIsClean(t *testing.T) {
	cfg := derivedTestConfig()
	for _, tc := range []struct {
		name, line string
		wantFiles  int
	}{
		{"empty cleared form", "claimed_at:", 0},
		{"control: real stamp", "claimed_at: 2026-08-01T10:00:00Z", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := corpusRecord{path: repairArchivedPath, bytes: repairArchivedDone(tc.line), kind: repository.KindChange, location: repository.LocationArchive}
			plan, err := planRepositoryRepair(setupContext{cfg: cfg}, checkCorpus{records: []corpusRecord{rec}, link: repairLink()})
			if err != nil {
				t.Fatalf("planRepositoryRepair: %v", err)
			}
			if len(plan.files) != tc.wantFiles {
				t.Errorf("files = %v, want %d", plan.files, tc.wantFiles)
			}
			if len(plan.manual) != 0 {
				t.Errorf("manual = %v, want none", plan.manual)
			}
		})
	}
}

// TestPlanRepositoryRepairMalformedMarkerIsManualOnly proves a record under an
// unbalanced managed marker is listed as manual review and never written, while
// the other repairable finding in the same run (the stale board) still plans
// (Review Focus 3).
func TestPlanRepositoryRepairMalformedMarkerIsManualOnly(t *testing.T) {
	cfg := derivedTestConfig()
	path := "docs/changes/active/0001-example.md"
	src := []byte("---\n" + derivedChangeFM + "---\n\n## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| Artifact | Link |\n\n## Why\n\nbody\n")
	corpus := checkCorpus{
		records: []corpusRecord{{path: path, bytes: src, kind: repository.KindChange, location: repository.LocationActive}},
		link:    repairLink(),
		board:   corpusFile{present: true, bytes: []byte("# Backlog\n\nstale bytes\n")},
	}
	plan, err := planRepositoryRepair(setupContext{cfg: cfg}, corpus)
	if err != nil {
		t.Fatalf("planRepositoryRepair: %v", err)
	}
	if _, written := plan.contents[path]; written {
		t.Errorf("the malformed record must never be written")
	}
	if !planHasFile(plan.files, boardCorpusPath(cfg)) {
		t.Errorf("files = %v, want the stale board still repaired", plan.files)
	}
	var sawMalformed bool
	for _, m := range plan.manual {
		if strings.HasPrefix(m, "["+reposetup.CodeArtifactLinksMalformed+"] "+path) {
			sawMalformed = true
		}
	}
	if !sawMalformed {
		t.Errorf("manual = %v, want an artifact-links-malformed line for %s", plan.manual, path)
	}
}

// TestRepairConfirmationRequiredNamesYes proves the unauthorized preview is
// invalid-state / confirmation-required, carries the pinned tip, lists both
// repair kinds, and names --yes.
func TestRepairConfirmationRequiredNamesYes(t *testing.T) {
	plan := repositoryRepairPlan{
		frontmatter: []reposetup.RepairFinding{{Path: repairArchivedPath, Field: "claimed_at", Code: reposetup.RepairDropClaimedAt, Repairable: true, Patch: []byte("-claimed_at: x\n")}},
		derived:     []reposetup.DerivedFinding{{Code: reposetup.CodeBoardStale, Path: "docs/changes/BOARD.md", Repairable: true}},
		files:       []string{"docs/changes/BOARD.md", repairArchivedPath},
		contents:    map[string][]byte{},
	}
	out := repairConfirmationRequired(setupContext{}, "abc123", plan)
	if out.Result != ResultInvalidState || !out.ConfirmationRequired() {
		t.Fatalf("preview = %q/%q, want invalid-state/confirmation-required", out.Result, out.RepositoryState)
	}
	if out.SourceRevision != "abc123" || out.SourceRev() != "abc123" {
		t.Errorf("SourceRevision = %q, want the pinned tip", out.SourceRevision)
	}
	if len(out.RepairedFiles) != 2 || len(out.Repairs) != 1 || len(out.RepairedViews) != 1 {
		t.Errorf("preview sets = files %v repairs %v views %v", out.RepairedFiles, out.Repairs, out.RepairedViews)
	}
	h := out.HumanText()
	for _, want := range []string{"--yes", "[drop-final-claimed-at] " + repairArchivedPath, "[board-stale] docs/changes/BOARD.md", "-claimed_at: x"} {
		if !strings.Contains(h, want) {
			t.Errorf("preview human lacks %q:\n%s", want, h)
		}
	}
}

// TestRepairAppliedNamesRevisionAndPending proves the applied document names the
// new and prior tips, the file set, and the `docket repository prepare` sync.
func TestRepairAppliedNamesRevisionAndPending(t *testing.T) {
	out := repairApplied("newtip", "priortip", repositoryRepairPlan{files: []string{"docs/changes/BOARD.md"}}, layout.Layout{Mode: layout.Shared})
	if out.Result != ResultApplied || out.MetadataTip != "newtip" || out.SourceRevision != "priortip" {
		t.Fatalf("applied = %+v", out)
	}
	if !strings.Contains(strings.Join(out.PendingLocal, " "), "docket repository prepare") {
		t.Errorf("PendingLocal = %v, want the prepare sync remedy", out.PendingLocal)
	}
}

// TestRepairAppliedPendingNamesResolvedWorktree proves the local-sync note names
// the layout's metadata worktree: `.docket` byte-for-byte when shared, the
// resolved private checkout path (never `.docket`) when private.
func TestRepairAppliedPendingNamesResolvedWorktree(t *testing.T) {
	const store = "/data/docket/store/proj-abc/checkout"
	plan := repositoryRepairPlan{files: []string{"docs/changes/BOARD.md"}}
	shared := repairApplied("newtip", "priortip", plan, layout.Layout{Mode: layout.Shared})
	if want := []string{"fast-forward your local .docket metadata worktree: re-run `docket repository prepare` to sync it to the repaired metadata revision"}; strings.Join(shared.PendingLocal, "|") != strings.Join(want, "|") {
		t.Errorf("shared PendingLocal = %q, want %q", shared.PendingLocal, want)
	}
	private := repairApplied("newtip", "priortip", plan, layout.Layout{Mode: layout.Private, MetadataWorktree: store})
	got := strings.Join(private.PendingLocal, " ")
	if want := "fast-forward your local " + store + " metadata worktree: re-run `docket repository prepare` to sync it to the repaired metadata revision"; got != want {
		t.Errorf("private PendingLocal = %q, want %q", got, want)
	}
	if strings.Contains(got+private.HumanText(), ".docket") {
		t.Errorf("private repair note names .docket: %q", got+private.HumanText())
	}
}

// TestRepairNoOpListsManualReview proves nothing-repairable is an idempotent
// no-op that still surfaces the manual-review list.
func TestRepairNoOpListsManualReview(t *testing.T) {
	out := repairNoOp("tip", []string{"[artifact-links-malformed] p: m"})
	if out.Result != ResultNoOp {
		t.Fatalf("Result = %q, want no-op", out.Result)
	}
	if !strings.Contains(out.HumanText(), "[artifact-links-malformed] p: m") {
		t.Errorf("no-op human must list manual review: %q", out.HumanText())
	}
}

// TestRepairContendedNamesBothRevisions proves a moved tip is contended and names
// both revisions.
func TestRepairContendedNamesBothRevisions(t *testing.T) {
	out := repairContended("fresh1", "pinned0")
	if out.Result != ResultContended {
		t.Fatalf("Result = %q, want contended", out.Result)
	}
	if h := out.HumanText(); !strings.Contains(h, "fresh1") || !strings.Contains(h, "pinned0") {
		t.Errorf("contended human must name both revisions: %q", h)
	}
}

// TestRepairFromMigrateResultRestampsOperation proves a routed refusal keeps its
// result, state, and remedy but carries the repository.repair operation key.
func TestRepairFromMigrateResultRestampsOperation(t *testing.T) {
	m := migrateRefusal(reposetup.StateConflict, "inspect and resolve it manually")
	out := repairFromMigrateResult(m)
	if out.Operation != OperationRepositoryRepair || out.Result != ResultInvalidState || out.RepositoryState != string(reposetup.StateConflict) {
		t.Fatalf("restamped = %+v", out)
	}
	if h := out.HumanText(); strings.Contains(h, OperationRepositoryMigrate) || !strings.Contains(h, "inspect and resolve it manually") {
		t.Errorf("restamped human = %q", h)
	}
}

// TestRepairResultJSONFieldNames pins the protocol-v1 keys.
func TestRepairResultJSONFieldNames(t *testing.T) {
	out := repairApplied("newtip", "priortip", repositoryRepairPlan{
		frontmatter: []reposetup.RepairFinding{{Path: repairArchivedPath, Code: reposetup.RepairDropClaimedAt, Repairable: true}},
		derived:     []reposetup.DerivedFinding{{Code: reposetup.CodeBoardStale, Path: "docs/changes/BOARD.md", Repairable: true}},
		manual:      []string{"[x] y: z"},
		files:       []string{"docs/changes/BOARD.md", repairArchivedPath},
	}, layout.Layout{Mode: layout.Shared})
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"protocol_version", "operation", "result", "repository_state", "source_revision",
		"metadata_revision", "repairs", "repaired_views", "repaired_files", "manual_review", "pending_local"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("result JSON missing %q: %s", key, raw)
		}
	}
	if decoded["operation"] != OperationRepositoryRepair {
		t.Errorf("operation = %v", decoded["operation"])
	}
}

// planHasFile reports whether files contains p.
func planHasFile(files []string, p string) bool {
	for _, f := range files {
		if f == p {
			return true
		}
	}
	return false
}

// TestPlanRepositoryRepairBoardRendersFrontmatterRepairedRecord is the
// discriminating companion to the test above, whose claimed_at drop never changes
// what the board renders: a scalar id-list field (`depends_on: 3`) decodes as a
// malformed list with no dependency, while the repaired `[3]` decodes as a
// dependency on #3, so the board rendered over the unrepaired corpus differs from
// the one rendered over the frontmatter-REPAIRED corpus. The planned board must
// be the latter (building the snapshot from the unrepaired records turns this
// red).
func TestPlanRepositoryRepairBoardRendersFrontmatterRepairedRecord(t *testing.T) {
	cfg := derivedTestConfig()
	path := "docs/changes/active/0001-example.md"
	listed := corpusRecord{path: path, bytes: changeRecordBytes(derivedChangeFM+"depends_on: 3\n", ""), kind: repository.KindChange, location: repository.LocationActive}
	archived := corpusRecord{path: repairArchivedPath, bytes: repairArchivedDone("claimed_at:"), kind: repository.KindChange, location: repository.LocationArchive}
	corpus := checkCorpus{
		records: []corpusRecord{listed, archived},
		link:    repairLink(),
		board:   corpusFile{present: true, bytes: []byte("# Backlog\n\nstale bytes\n")},
	}
	plan, err := planRepositoryRepair(setupContext{cfg: cfg}, corpus)
	if err != nil {
		t.Fatalf("planRepositoryRepair: %v", err)
	}
	if len(plan.frontmatter) != 1 || plan.frontmatter[0].Code != reposetup.RepairScalarToList {
		t.Fatalf("frontmatter = %+v, want one scalar-to-list repair", plan.frontmatter)
	}
	board := boardCorpusPath(cfg)
	if !planHasFile(plan.files, board) || !planHasFile(plan.files, path) {
		t.Fatalf("files = %v, want the board and the repaired record", plan.files)
	}
	render := func(recs []corpusRecord) []byte {
		t.Helper()
		snap, ok := buildCorpusSnapshot(cfg, recs)
		if !ok {
			t.Fatal("snapshot build failed")
		}
		b, err := renderCanonicalBoard(snap, corpusBoardUnrenderable(cfg, recs), boardPresentation(cfg))
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		return b
	}
	unrepaired := render(corpus.records)
	repaired := render([]corpusRecord{{path: path, bytes: plan.contents[path], kind: repository.KindChange, location: repository.LocationActive}, archived})
	if bytes.Equal(unrepaired, repaired) {
		t.Fatal("fixture does not discriminate: the board renders identically before and after the frontmatter repair")
	}
	if !bytes.Equal(plan.contents[board], repaired) {
		t.Errorf("board bytes are not the canonical render over the repaired corpus; got:\n%s\nwant:\n%s", plan.contents[board], repaired)
	}
}

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

// TestComposeDerivedRepairBoardBytes proves the composed board repair equals the
// canonical render over the same snapshot — the repair recomputes canonical bytes
// rather than merging stale output.
func TestComposeDerivedRepairBoardBytes(t *testing.T) {
	cfg := derivedTestConfig()
	path, canonical := canonicalChangeRecord(t, cfg)
	rec := corpusRecord{path: path, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive}
	snap, _ := buildCorpusSnapshot(cfg, []corpusRecord{rec})
	corpus := checkCorpus{records: []corpusRecord{rec}, link: render.LinkContext{MetadataBranch: layout.SharedName}}
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
	corpus := checkCorpus{records: []corpusRecord{stale}, link: render.LinkContext{MetadataBranch: layout.SharedName}}
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
