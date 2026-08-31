package app

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

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
	board, err := render.Board(render.BoardInput{Snapshot: built})
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
	if err := includeBoard(context.Background(), tree, boardPath, snap, &files); err != nil {
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
	if err := includeBoard(context.Background(), tree, boardPath, snap, &files); err != nil {
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
	if err := includeBoard(context.Background(), tree, boardPath, snap, &files); err != nil {
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
	if err := includeBoard(context.Background(), errReadTree{}, boardPath, snap, &files); err == nil {
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
	if err := includeADRIndex(context.Background(), tree, snap, adrIndexPath, &files); err != nil {
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
	if err := includeADRIndex(context.Background(), tree, snap, adrIndexPath, &files); err != nil {
		t.Fatalf("includeADRIndex: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files len = %d, want 1", len(files))
	}
	if files[0].Kind != transaction.MutationReplace {
		t.Errorf("Kind = %q, want replace", files[0].Kind)
	}
}
