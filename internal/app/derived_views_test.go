package app

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// resolveBoardTestConfig resolves a config whose repository layer sets a
// non-default value on every kind of board leaf: a permuted section_order (with
// proposed first) and board.sorting.proposed = {id, asc}. The remaining sections
// inherit the built-in updated/desc. It is the fixture behind both the lift
// correspondence test and the render-honoring test.
func resolveBoardTestConfig(t *testing.T) config.Effective {
	t.Helper()
	snap, _, err := config.Resolve([]config.Source{{
		Layer: config.LayerRepository,
		Name:  ".docket.yml",
		Data: []byte("board:\n" +
			"  section_order: [proposed, deferred, groomed, blocked, built, in-progress]\n" +
			"  sorting:\n" +
			"    proposed:\n" +
			"      by: id\n" +
			"      direction: asc\n"),
	}}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("config.Resolve: %v", err)
	}
	return snap.Effective
}

// TestBoardPresentationLiftsResolvedConfig pins boardPresentation as a pure type
// lift from the resolved config to the renderer's presentation. It asserts the
// RESOLVED NON-DEFAULT value — the permuted order and id/asc for proposed — which
// is load-bearing: a value that defaulted through would stay green even if the
// wiring were deleted.
func TestBoardPresentationLiftsResolvedConfig(t *testing.T) {
	pres := boardPresentation(resolveBoardTestConfig(t))

	wantOrder := []render.BoardSection{
		render.BoardSectionProposed, render.BoardSectionDeferred, render.BoardSectionGroomed,
		render.BoardSectionBlocked, render.BoardSectionBuilt, render.BoardSectionInProgress,
	}
	if !slices.Equal(pres.SectionOrder, wantOrder) {
		t.Errorf("SectionOrder = %v, want the permuted %v", pres.SectionOrder, wantOrder)
	}

	if got, want := pres.Sorting[render.BoardSectionProposed],
		(render.BoardSort{By: render.BoardSortKeyID, Direction: render.BoardDirectionAsc}); got != want {
		t.Errorf("proposed sort = %+v, want the configured %+v", got, want)
	}
	// An untouched section inherits the built-in updated/desc.
	if got, want := pres.Sorting[render.BoardSectionBuilt],
		(render.BoardSort{By: render.BoardSortKeyUpdated, Direction: render.BoardDirectionDesc}); got != want {
		t.Errorf("built sort = %+v, want the default %+v", got, want)
	}
}

// firstBoardHeading returns the first "## …" section heading in a rendered board.
func firstBoardHeading(board []byte) string {
	for _, line := range strings.Split(string(board), "\n") {
		if strings.HasPrefix(line, "## ") {
			return line
		}
	}
	return ""
}

// TestBoardRenderHonorsConfiguredPresentation proves the lifted presentation
// reaches the renderer: over a two-section fixture, renderCanonicalBoard with the
// configured (proposed-first) presentation differs byte-for-byte from the
// default-presentation render and leads with the Proposed heading, where the
// default leads with Blocked.
func TestBoardRenderHonorsConfiguredPresentation(t *testing.T) {
	blocked := domain.NewChange(domain.ChangeSpec{
		ID: 1, Slug: "blk", Title: "Blocked", Status: domain.StatusBlocked,
		Location: domain.LocationActive, Path: "docs/changes/active/0001-blk.md",
	})
	proposed := domain.NewChange(domain.ChangeSpec{
		ID: 2, Slug: "prop", Title: "Proposed", Status: domain.StatusProposed,
		Location: domain.LocationActive, Path: "docs/changes/active/0002-prop.md",
	})
	snap := domain.NewSnapshot(domain.SnapshotSpec{Changes: []domain.Change{blocked, proposed}})

	def, err := renderCanonicalBoard(snap, nil, render.DefaultBoardPresentation())
	if err != nil {
		t.Fatalf("default render: %v", err)
	}
	got, err := renderCanonicalBoard(snap, nil, boardPresentation(resolveBoardTestConfig(t)))
	if err != nil {
		t.Fatalf("configured render: %v", err)
	}

	if bytes.Equal(def, got) {
		t.Fatal("configured presentation produced the same bytes as the default render")
	}
	if h := firstBoardHeading(got); !strings.Contains(h, "Proposed") {
		t.Errorf("configured first heading = %q, want the permuted Proposed section first", h)
	}
	if h := firstBoardHeading(def); !strings.Contains(h, "Blocked") {
		t.Errorf("default first heading = %q, want Blocked first", h)
	}
}

// derivedViewsSnapshot builds a one-change candidate snapshot plus its canonical
// board and ADR-index bytes for the inclusion-helper tests. The snapshot is the
// same fixture the derived-view check tests use, so render.Board over it
// succeeds (a proposed, spec-linked, build-ready change).
func derivedViewsSnapshot(t *testing.T) (snap domain.Snapshot, boardPath, adrIndexPath string, wantBoard []byte) {
	t.Helper()
	cfg := derivedTestConfig()
	path, canonical := canonicalChangeRecord(t, cfg)
	rec := corpusRecord{path: path, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive}
	built, ok := buildCorpusSnapshot(cfg, []corpusRecord{rec})
	if !ok {
		t.Fatal("buildCorpusSnapshot failed")
	}
	board, err := render.Board(render.BoardInput{Snapshot: built, Presentation: boardPresentation(cfg)})
	if err != nil {
		t.Fatalf("render board: %v", err)
	}
	return built, boardCorpusPath(cfg), adrIndexCorpusPath(cfg), board
}

// errReadTree is a transaction.Tree whose ReadBlobs always errors, to prove the
// inclusion helpers leave files unmodified when the base-tree probe fails.
type errReadTree struct{ transaction.Tree }

func (errReadTree) ReadBlobs(context.Context, []gitcli.RepoPath) ([]gitcli.BlobResult, error) {
	return nil, errors.New("boom")
}

func (errReadTree) ListTree(context.Context, []gitcli.RepoPath) ([]gitcli.TreeEntry, error) {
	return nil, errors.New("boom")
}

func (errReadTree) Revision() gitcli.Revision { return gitcli.Revision{} }

func TestIncludeBoardCreatesWhenBoardAbsentBytesMatchDirectRender(t *testing.T) {
	snap, boardPath, _, want := derivedViewsSnapshot(t)
	tree := newFakeTree(map[string]string{}) // no board on the base tree
	var files []transaction.FileMutation
	if err := includeBoard(context.Background(), tree, boardPath, snap, nil, boardPresentation(derivedTestConfig()), &files); err != nil {
		t.Fatalf("includeBoard: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	if files[0].Kind != transaction.MutationCreate {
		t.Errorf("Kind = %q, want create", files[0].Kind)
	}
	if string(files[0].Path) != boardPath {
		t.Errorf("Path = %q, want %q", files[0].Path, boardPath)
	}
	if !bytes.Equal(files[0].Bytes, want) {
		t.Errorf("board bytes != direct render.Board bytes")
	}
}

func TestIncludeBoardReplacesWhenBoardPresentAndDiffers(t *testing.T) {
	snap, boardPath, _, want := derivedViewsSnapshot(t)
	tree := newFakeTree(map[string]string{boardPath: "# stale board\n"})
	var files []transaction.FileMutation
	if err := includeBoard(context.Background(), tree, boardPath, snap, nil, boardPresentation(derivedTestConfig()), &files); err != nil {
		t.Fatalf("includeBoard: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	if files[0].Kind != transaction.MutationReplace {
		t.Errorf("Kind = %q, want replace", files[0].Kind)
	}
	if !bytes.Equal(files[0].Bytes, want) {
		t.Errorf("board bytes != direct render.Board bytes")
	}
}

// TestIncludeBoardSkipsWhenByteIdentical proves the declare-only-when-changed
// shape: when the committed board already equals the canonical render (the
// not-board-visible mutations: attach, claim refresh, reconcile, clear-block),
// no board mutation is declared, so the engine's verify-delta does not refuse.
func TestIncludeBoardSkipsWhenByteIdentical(t *testing.T) {
	snap, boardPath, _, want := derivedViewsSnapshot(t)
	tree := newFakeTree(map[string]string{boardPath: string(want)})
	var files []transaction.FileMutation
	if err := includeBoard(context.Background(), tree, boardPath, snap, nil, boardPresentation(derivedTestConfig()), &files); err != nil {
		t.Fatalf("includeBoard: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("files len = %d, want 0 (no declaration when board is unchanged)", len(files))
	}
}

// TestIncludeBoardProbeErrorLeavesFilesUnmodified proves that when the base-tree
// probe fails, includeBoard returns the error and appends nothing — the no
// partial append property. (The render step precedes the probe and returns its
// own error the same way; render.Board only errors on snapshots that fail
// domain validation, which BuildSnapshot rejects before rendering, so the
// reachable early-error path is the probe.)
func TestIncludeBoardProbeErrorLeavesFilesUnmodified(t *testing.T) {
	snap, boardPath, _, _ := derivedViewsSnapshot(t)
	files := []transaction.FileMutation{{Path: "docs/changes/active/0001-example.md", Kind: transaction.MutationReplace, Bytes: []byte("x")}}
	before := len(files)
	if err := includeBoard(context.Background(), errReadTree{}, boardPath, snap, nil, boardPresentation(derivedTestConfig()), &files); err == nil {
		t.Fatal("includeBoard: want an error from the failing probe")
	}
	if len(files) != before {
		t.Errorf("files len = %d, want %d (no partial append on error)", len(files), before)
	}
}

func TestIncludeADRIndexCreatesWhenIndexAbsentBytesMatchDirectRender(t *testing.T) {
	snap, _, adrIndexPath, _ := derivedViewsSnapshot(t)
	want, err := render.ADRIndex(snap)
	if err != nil {
		t.Fatalf("render ADRIndex: %v", err)
	}
	tree := newFakeTree(map[string]string{})
	var files []transaction.FileMutation
	if err := includeADRIndex(context.Background(), tree, snap, nil, adrIndexPath, &files); err != nil {
		t.Fatalf("includeADRIndex: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	if files[0].Kind != transaction.MutationCreate {
		t.Errorf("Kind = %q, want create", files[0].Kind)
	}
	if !bytes.Equal(files[0].Bytes, want) {
		t.Errorf("ADR index bytes != direct render.ADRIndex bytes")
	}
}

func TestIncludeADRIndexReplacesWhenIndexPresent(t *testing.T) {
	snap, _, adrIndexPath, _ := derivedViewsSnapshot(t)
	tree := newFakeTree(map[string]string{adrIndexPath: "# stale index\n"})
	var files []transaction.FileMutation
	if err := includeADRIndex(context.Background(), tree, snap, nil, adrIndexPath, &files); err != nil {
		t.Fatalf("includeADRIndex: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	if files[0].Kind != transaction.MutationReplace {
		t.Errorf("Kind = %q, want replace", files[0].Kind)
	}
}

// --- change 0449: records the snapshot cannot see surface on the board ---

// TestBoardUnrenderableDerivesAbsentChangeRecords pins boardUnrenderable's
// shape-keyed population: every change record (active or archive) present in
// Sources but absent from the snapshot is one entry, its reason taken from the
// error finding naming that path (else "unreadable"); non-change records (an
// ADR, a learning) and records the snapshot does carry never appear.
func TestBoardUnrenderableDerivesAbsentChangeRecords(t *testing.T) {
	b := domain.NewChange(domain.ChangeSpec{
		ID: 3, Slug: "widget", Title: "Widget", Status: domain.StatusProposed,
		Location: domain.LocationActive, Path: "docs/changes/active/0003-widget.md",
	})
	st := transaction.LoadedState{
		Snapshot: domain.NewSnapshot(domain.SnapshotSpec{Changes: []domain.Change{b}}),
		Report: domain.NewValidationReport([]domain.Finding{
			{Code: "unclosed-frontmatter", Severity: domain.SeverityError,
				Entity: domain.EntityRef{Kind: domain.EntityChange, Path: "docs/changes/active/0099-broken.md"}},
			{Code: "some-warning", Severity: domain.SeverityWarning,
				Entity: domain.EntityRef{Kind: domain.EntityChange, Path: "docs/changes/archive/2026-01-01-0042-gone.md"}},
		}),
		Sources: map[string][]byte{
			"docs/changes/active/0003-widget.md":           []byte("healthy"),
			"docs/changes/active/0099-broken.md":           []byte("---\nid: 99\n"),
			"docs/changes/archive/2026-01-01-0042-gone.md": []byte("undecodable"),
			"docs/adrs/0001-broken.md":                     []byte("---\n"),
			"docs/changes/learnings/broken.md":             []byte("---\n"),
		},
	}
	got := boardUnrenderable(st, "docs/changes")
	want := []render.BoardUnrenderable{
		{Path: "docs/changes/active/0099-broken.md", Reason: "unclosed-frontmatter"},
		{Path: "docs/changes/archive/2026-01-01-0042-gone.md", Reason: "unreadable"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("boardUnrenderable = %+v, want %+v", got, want)
	}

	healthy := st
	healthy.Sources = map[string][]byte{"docs/changes/active/0003-widget.md": []byte("healthy")}
	if got := boardUnrenderable(healthy, "docs/changes"); got != nil {
		t.Errorf("healthy state yields %+v, want nil", got)
	}
}

// TestIncludeBoardSurfacesUnrenderable proves includeBoard threads the caller's
// unrenderable entries into the canonical render: the declared board carries
// the repair notice naming the record.
func TestIncludeBoardSurfacesUnrenderable(t *testing.T) {
	snap, boardPath, _, _ := derivedViewsSnapshot(t)
	unr := []render.BoardUnrenderable{{Path: "docs/changes/active/0099-broken.md", Reason: "unclosed-frontmatter"}}
	want, err := render.Board(render.BoardInput{Snapshot: snap, Presentation: boardPresentation(derivedTestConfig()), Unrenderable: unr})
	if err != nil {
		t.Fatalf("render board: %v", err)
	}
	var files []transaction.FileMutation
	if err := includeBoard(context.Background(), newFakeTree(map[string]string{}), boardPath, snap, unr, boardPresentation(derivedTestConfig()), &files); err != nil {
		t.Fatalf("includeBoard: %v", err)
	}
	if len(files) != 1 || !bytes.Equal(files[0].Bytes, want) {
		t.Fatalf("includeBoard did not declare the notice-bearing canonical board")
	}
	if !strings.Contains(string(files[0].Bytes), "| `docs/changes/active/0099-broken.md` | unclosed-frontmatter |") {
		t.Errorf("declared board lacks the repair notice:\n%s", files[0].Bytes)
	}
}

// TestIncludeBoardDropsEntriesTheCandidateRenders proves the before-state
// derived list never double-reports a record the operation itself made
// renderable: an entry whose path the candidate carries as a change renders as
// that change's row only, with no repair notice.
func TestIncludeBoardDropsEntriesTheCandidateRenders(t *testing.T) {
	snap, boardPath, _, want := derivedViewsSnapshot(t)
	var rendered string
	for _, c := range snap.Changes() {
		rendered = c.Path()
	}
	unr := []render.BoardUnrenderable{{Path: rendered, Reason: "unclosed-frontmatter"}}
	var files []transaction.FileMutation
	if err := includeBoard(context.Background(), newFakeTree(map[string]string{}), boardPath, snap, unr, boardPresentation(derivedTestConfig()), &files); err != nil {
		t.Fatalf("includeBoard: %v", err)
	}
	if len(files) != 1 || !bytes.Equal(files[0].Bytes, want) {
		t.Fatalf("a candidate-rendered record also landed in the repair notice:\n%s", files[0].Bytes)
	}
}

// TestDerivedViewFindingsAcceptsNoticeBearingBoard proves the check/migrate
// canonical render derives the same repair entries the mutations do: a
// committed board carrying the notice for an unparseable record is NOT stale,
// while the pre-0449 board that silently omitted the record now is.
func TestDerivedViewFindingsAcceptsNoticeBearingBoard(t *testing.T) {
	cfg := derivedTestConfig()
	p, canonical := canonicalChangeRecord(t, cfg)
	recs := []corpusRecord{
		{path: p, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive},
		{path: "docs/changes/active/0099-broken.md", bytes: []byte("---\nid: 99\nslug: broken\n"), kind: repository.KindChange, location: repository.LocationActive},
	}
	snap, ok := buildCorpusSnapshot(cfg, recs)
	if !ok {
		t.Fatal("buildCorpusSnapshot failed")
	}
	withNotice, err := renderCanonicalBoard(snap, []render.BoardUnrenderable{{Path: "docs/changes/active/0099-broken.md", Reason: "unclosed-frontmatter"}}, boardPresentation(cfg))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(withNotice), "0099-broken.md") {
		t.Fatalf("fixture board lacks the notice:\n%s", withNotice)
	}
	link := render.LinkContext{MetadataBranch: reposetup.MetadataBranchName}
	for _, f := range derivedViewFindings(cfg, checkCorpus{records: recs, link: link, board: corpusFile{present: true, bytes: withNotice}}) {
		if f.Code == reposetup.CodeBoardStale {
			t.Errorf("notice-bearing board reported stale: %+v", f)
		}
	}
	silent, _ := renderCanonicalBoard(snap, nil, boardPresentation(cfg))
	stale := false
	for _, f := range derivedViewFindings(cfg, checkCorpus{records: recs, link: link, board: corpusFile{present: true, bytes: silent}}) {
		stale = stale || f.Code == reposetup.CodeBoardStale
	}
	if !stale {
		t.Error("a board silently omitting the unparseable record is not reported stale")
	}
}

// --- change 0449: ADR records the snapshot cannot see surface on the index ---

// TestADRIndexUnrenderableDerivesAbsentADRRecords pins adrIndexUnrenderable's
// shape-keyed population: every ADR record under the ADR directory present in
// Sources but absent from the snapshot is one entry (reason from the error
// finding naming it, else "unreadable"); the generated index itself, change
// records, and ADRs the snapshot does carry never appear.
func TestADRIndexUnrenderableDerivesAbsentADRRecords(t *testing.T) {
	good := domain.NewADR(domain.ADRSpec{ID: 1, Slug: "good", Title: "Good", RawStatus: "Accepted", Path: "docs/adrs/0001-good.md"})
	st := transaction.LoadedState{
		Snapshot: domain.NewSnapshot(domain.SnapshotSpec{ADRs: []domain.ADR{good}}),
		Report: domain.NewValidationReport([]domain.Finding{
			{Code: "unclosed-frontmatter", Severity: domain.SeverityError,
				Entity: domain.EntityRef{Kind: domain.EntityADR, Path: "docs/adrs/0009-broken.md"}},
		}),
		Sources: map[string][]byte{
			"docs/adrs/0001-good.md":             []byte("healthy"),
			"docs/adrs/0009-broken.md":           []byte("---\nid: 9\n"),
			"docs/adrs/0010-undecodable.md":      []byte("undecodable"),
			"docs/adrs/README.md":                []byte("# index"),
			"docs/changes/active/0099-broken.md": []byte("---\n"),
		},
	}
	got := adrIndexUnrenderable(st, "docs/adrs")
	want := []render.BoardUnrenderable{
		{Path: "docs/adrs/0009-broken.md", Reason: "unclosed-frontmatter"},
		{Path: "docs/adrs/0010-undecodable.md", Reason: "unreadable"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("adrIndexUnrenderable = %+v, want %+v", got, want)
	}
	healthy := st
	healthy.Sources = map[string][]byte{"docs/adrs/0001-good.md": []byte("healthy"), "docs/adrs/README.md": []byte("# index")}
	if got := adrIndexUnrenderable(healthy, "docs/adrs"); got != nil {
		t.Errorf("healthy state yields %+v, want nil", got)
	}
}

// TestIncludeADRIndexSurfacesUnrenderable proves includeADRIndex threads the
// caller's entries into the canonical render — the declared index carries the
// repair notice naming the record — and drops an entry the candidate renders.
func TestIncludeADRIndexSurfacesUnrenderable(t *testing.T) {
	snap, _, adrIndexPath, _ := derivedViewsSnapshot(t)
	unr := []render.BoardUnrenderable{{Path: "docs/adrs/0009-broken.md", Reason: "unclosed-frontmatter"}}
	var files []transaction.FileMutation
	if err := includeADRIndex(context.Background(), newFakeTree(map[string]string{}), snap, unr, adrIndexPath, &files); err != nil {
		t.Fatalf("includeADRIndex: %v", err)
	}
	if len(files) != 1 || !strings.Contains(string(files[0].Bytes), "| `docs/adrs/0009-broken.md` | unclosed-frontmatter |") {
		t.Fatalf("declared ADR index lacks the repair notice:\n%s", files[0].Bytes)
	}

	good := domain.NewADR(domain.ADRSpec{ID: 1, Slug: "good", Title: "Good", RawStatus: "Accepted", Path: "docs/adrs/0001-good.md"})
	withADR := domain.NewSnapshot(domain.SnapshotSpec{ADRs: []domain.ADR{good}})
	want, _ := render.ADRIndex(withADR)
	files = nil
	if err := includeADRIndex(context.Background(), newFakeTree(map[string]string{}), withADR,
		[]render.BoardUnrenderable{{Path: good.Path(), Reason: "unclosed-frontmatter"}}, adrIndexPath, &files); err != nil {
		t.Fatalf("includeADRIndex: %v", err)
	}
	if len(files) != 1 || !bytes.Equal(files[0].Bytes, want) {
		t.Fatalf("a candidate-rendered ADR also landed in the repair notice:\n%s", files[0].Bytes)
	}
}

// TestDerivedViewFindingsAcceptsNoticeBearingADRIndex proves the check/migrate
// canonical ADR-index render derives the same repair entries the ADR mutations
// do: an index carrying the notice for an unparseable ADR is NOT stale, while
// one silently omitting it is.
func TestDerivedViewFindingsAcceptsNoticeBearingADRIndex(t *testing.T) {
	cfg := derivedTestConfig()
	recs := []corpusRecord{
		{path: "docs/adrs/0001-good.md", bytes: []byte(fixtureADR(1, "good")), kind: repository.KindADR, location: repository.LocationLedger},
		{path: "docs/adrs/0009-broken.md", bytes: []byte("---\nid: 9\nslug: broken\n"), kind: repository.KindADR, location: repository.LocationLedger},
	}
	snap, ok := buildCorpusSnapshot(cfg, recs)
	if !ok {
		t.Fatal("buildCorpusSnapshot failed")
	}
	withNotice, err := renderCanonicalADRIndex(snap, []render.BoardUnrenderable{{Path: "docs/adrs/0009-broken.md", Reason: "unclosed-frontmatter"}})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	link := render.LinkContext{MetadataBranch: reposetup.MetadataBranchName}
	for _, f := range derivedViewFindings(cfg, checkCorpus{records: recs, link: link, adrIndex: corpusFile{present: true, bytes: withNotice}}) {
		if f.Code == reposetup.CodeADRIndexStale {
			t.Errorf("notice-bearing ADR index reported stale: %+v", f)
		}
	}
	silent, _ := renderCanonicalADRIndex(snap, nil)
	stale := false
	for _, f := range derivedViewFindings(cfg, checkCorpus{records: recs, link: link, adrIndex: corpusFile{present: true, bytes: silent}}) {
		stale = stale || f.Code == reposetup.CodeADRIndexStale
	}
	if !stale {
		t.Error("an ADR index silently omitting the unparseable record is not reported stale")
	}
}
