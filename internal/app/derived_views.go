package app

import (
	"bytes"
	"context"
	"fmt"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// derived_views.go is the one shared app-layer inclusion path for the derived
// views — the board, the ADR index, and (via render.ArtifactBlockContent) the
// per-record artifact-links block. It keeps a single call site of each pure
// renderer inside internal/app so a board-authoritative mutation cannot smuggle
// in a private copy of board grouping/ordering/readiness logic (spec
// §Derived-view ownership). The mutation-shape guard in
// derived_views_guard_test.go proves every change-record mutation reaches
// includeBoard.
//
// Artifact links are rendered through the single canonical renderer
// render.ArtifactBlockContent at each change/finalize/ADR op call site: that
// renderer already IS the one shared implementation, and the surrounding
// per-op work (the candidate.Change lookup, whether the block is emitted
// conditionally, whether a missing block is inserted or an existing one
// replaced, and whether a render failure is a plain error or a typed op refusal)
// is genuinely variant across operations. Folding that variance behind a single
// inclusion helper would flatten a real caller distinction (learning
// consolidation-flattens-caller-variance), so no includeArtifactLinks wrapper is
// introduced; the ownership invariant (one renderer, candidate snapshot) already
// holds. The legacy-migration seed publication is likewise NOT routed here: it
// adopts a real bash-era seed byte-for-byte (a canonical re-render would no
// longer byte-match the adopted legacy board and would break bash-seed
// adoption), so its board copy is a deliberate exception owned by
// repository.prepare/migrate, not by these helpers.

// renderCanonicalBoard renders snap through the one canonical board renderer.
// This is the only call site of render.Board in internal/app.
func renderCanonicalBoard(snap domain.Snapshot) ([]byte, error) {
	return render.Board(render.BoardInput{Snapshot: snap})
}

// renderCanonicalADRIndex renders snap through the one canonical ADR-index
// renderer. This is the only call site of render.ADRIndex in internal/app.
func renderCanonicalADRIndex(snap domain.Snapshot) ([]byte, error) {
	return render.ADRIndex(snap)
}

// includeBoard renders the candidate after-state through the canonical board
// renderer and appends BOARD.md to files — but ONLY when the rendered board
// differs from the one committed on the base tree.
//
// This single declare-only-when-changed shape covers every board-authoritative
// mutation. Operations whose edits are always board-visible (create, groom,
// kill, lifecycle, mark-implemented, reclaim, repair, closeout) always render a
// board that differs from the committed one, so the board is always declared —
// as a create when BOARD.md is absent, a replace otherwise. Operations whose
// edits are NOT board-visible (attach, claim refresh, reconcile, clear-block via
// planInlineBoard) can render a board byte-identical to the committed one; the
// transaction engine's verify-delta refuses a declared path that is not an
// actual change ("a declared path is not an actual change"), so those must skip
// the declaration when nothing changed. Rendering the candidate rather than the
// before-state is load-bearing: a stale before-state render would recommit the
// pre-mutation board.
//
// On a render or probe error the function returns the error and leaves files
// unmodified — the append happens only after both the render and the probe
// succeed, so there is never a partial append.
func includeBoard(ctx context.Context, tree transaction.Tree, boardPath string, candidate domain.Snapshot, files *[]transaction.FileMutation) error {
	boardBytes, err := renderCanonicalBoard(candidate)
	if err != nil {
		return fmt.Errorf("rendering board: %w", err)
	}
	results, err := tree.ReadBlobs(ctx, []gitcli.RepoPath{gitcli.RepoPath(boardPath)})
	if err != nil {
		return fmt.Errorf("probing board path: %w", err)
	}
	existing := len(results) == 1 && results[0].Found
	switch {
	case !existing:
		*files = append(*files, transaction.FileMutation{
			Path: gitcli.RepoPath(boardPath), Kind: transaction.MutationCreate, Bytes: boardBytes,
		})
	case !bytes.Equal(results[0].Blob.Bytes, boardBytes):
		*files = append(*files, transaction.FileMutation{
			Path: gitcli.RepoPath(boardPath), Kind: transaction.MutationReplace, Bytes: boardBytes,
		})
	}
	return nil
}

// includeADRIndex renders the candidate after-state through the canonical
// ADR-index renderer and appends the index (README.md) to files. The ADR
// operations always change the index, so the mutation is declared
// unconditionally — a create when the index is absent, a replace otherwise. On a
// render or probe error the function returns the error and leaves files
// unmodified (no partial append).
func includeADRIndex(ctx context.Context, tree transaction.Tree, candidate domain.Snapshot, indexPath string, files *[]transaction.FileMutation) error {
	indexBytes, err := renderCanonicalADRIndex(candidate)
	if err != nil {
		return fmt.Errorf("rendering index: %w", err)
	}
	exists, err := treeHasPath(ctx, tree, indexPath)
	if err != nil {
		return err
	}
	kind := transaction.MutationCreate
	if exists {
		kind = transaction.MutationReplace
	}
	*files = append(*files, transaction.FileMutation{
		Path: gitcli.RepoPath(indexPath), Kind: kind, Bytes: indexBytes,
	})
	return nil
}
