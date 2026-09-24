package app

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
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

// boardPresentation is the ONE path from resolved configuration to renderer
// options. Config has already validated and defaulted every leaf (change 0367),
// so this is a pure type lift — it invents no defaults and can only mistranslate,
// which TestBoardPresentationLiftsResolvedConfig pins. App code never inlines
// render.DefaultBoardPresentation(): every board render flows the resolved
// config's presentation through here, so a section-order or per-section-sort
// override reaches the rendered board.
func boardPresentation(eff config.Effective) render.BoardPresentation {
	order := make([]render.BoardSection, 0, len(eff.Board.SectionOrder.Value))
	for _, s := range eff.Board.SectionOrder.Value {
		order = append(order, render.BoardSection(s))
	}
	sorting := make(map[render.BoardSection]render.BoardSort, len(eff.Board.Sorting))
	for s, srt := range eff.Board.Sorting {
		sorting[render.BoardSection(s)] = render.BoardSort{
			By:        render.BoardSortKey(srt.By.Value),
			Direction: render.BoardDirection(srt.Direction.Value),
		}
	}
	return render.BoardPresentation{SectionOrder: order, Sorting: sorting}
}

// renderCanonicalBoard renders snap through the one canonical board renderer with
// the caller's presentation policy (built via boardPresentation from the resolved
// config) and the records the snapshot cannot see (boardUnrenderable), which the
// board surfaces in its repair notice. This is the only call site of render.Board
// in internal/app.
func renderCanonicalBoard(snap domain.Snapshot, unrenderable []render.BoardUnrenderable, pres render.BoardPresentation) ([]byte, error) {
	return render.Board(render.BoardInput{Snapshot: snap, Presentation: pres, Unrenderable: unrenderable})
}

// boardUnrenderable derives the board's caller-supplied repair entries from a
// loaded state (change 0449): every change record path in st.Sources — under
// the active or archive directory of changesDir — that has no corresponding
// change in st.Snapshot. The key is the SHAPE absent-from-snapshot, never an
// enumerated finding-code list: a record that failed to parse or to decode is
// equally invisible to the renderer. The reason is the Code of the first
// error-severity finding whose entity names that path, else "unreadable". The
// result is sorted by path; a healthy state yields nil.
func boardUnrenderable(st transaction.LoadedState, changesDir string) []render.BoardUnrenderable {
	activePfx := path.Join(changesDir, "active") + "/"
	archivePfx := path.Join(changesDir, "archive") + "/"
	rendered := make(map[string]bool)
	for _, c := range st.Snapshot.Changes() {
		rendered[c.Path()] = true
	}
	return sourcesAbsentFromSnapshot(st, rendered, func(p string) bool {
		return strings.HasPrefix(p, activePfx) || strings.HasPrefix(p, archivePfx)
	})
}

// adrIndexUnrenderable is boardUnrenderable's ADR-index counterpart (change
// 0449): every ADR record path in st.Sources — under adrsDir, excluding the
// generated index itself — that has no corresponding ADR in st.Snapshot, so an
// unparseable or undecodable unrelated ADR is surfaced in the index's repair
// notice instead of silently vanishing from it. Same shape key, same reason
// derivation, sorted by path; a healthy state yields nil.
func adrIndexUnrenderable(st transaction.LoadedState, adrsDir string) []render.BoardUnrenderable {
	adrsPfx := path.Clean(adrsDir) + "/"
	rendered := make(map[string]bool)
	for _, a := range st.Snapshot.ADRs() {
		rendered[a.Path()] = true
	}
	return sourcesAbsentFromSnapshot(st, rendered, func(p string) bool {
		return strings.HasPrefix(p, adrsPfx) && !isDerivedIndex(path.Base(p))
	})
}

// sourcesAbsentFromSnapshot lists every in-scope path of st.Sources that is not
// in rendered, each with the Code of the first error-severity finding naming
// that path (else "unreadable"), sorted by path; nil when none.
func sourcesAbsentFromSnapshot(st transaction.LoadedState, rendered map[string]bool, inScope func(string) bool) []render.BoardUnrenderable {
	var out []render.BoardUnrenderable
	for p := range st.Sources {
		if !inScope(p) {
			continue
		}
		if rendered[p] {
			continue
		}
		reason := "unreadable"
		for _, f := range st.Report.Findings() {
			if f.Severity == domain.SeverityError && f.Entity.Path == p {
				reason = f.Code
				break
			}
		}
		out = append(out, render.BoardUnrenderable{Path: p, Reason: reason})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// renderCanonicalADRIndex renders snap through the one canonical ADR-index
// renderer, with the ADR records the snapshot cannot see (adrIndexUnrenderable)
// surfaced in the index's repair notice. This is the only call site of the
// render.ADRIndex family in internal/app.
func renderCanonicalADRIndex(snap domain.Snapshot, unrenderable []render.BoardUnrenderable) ([]byte, error) {
	return render.ADRIndexWithRepair(snap, unrenderable)
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
// unrenderable is the caller's boardUnrenderable(st.State, changesDir): the
// records the candidate snapshot cannot see, surfaced in the board's repair
// notice so an unparseable unrelated record stays visible (change 0449).
//
// On a render or probe error the function returns the error and leaves files
// unmodified — the append happens only after both the render and the probe
// succeed, so there is never a partial append.
func includeBoard(ctx context.Context, tree transaction.Tree, boardPath string, candidate domain.Snapshot, unrenderable []render.BoardUnrenderable, pres render.BoardPresentation, files *[]transaction.FileMutation) error {
	boardBytes, err := renderCanonicalBoard(candidate, withoutCandidateChanges(unrenderable, candidate), pres)
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

// withoutCandidateChanges drops the entries whose path the candidate snapshot
// carries as a change: the list is derived from the operation's BEFORE state, so
// a record the operation itself made renderable must render as a row, never also
// in the repair notice.
func withoutCandidateChanges(unrenderable []render.BoardUnrenderable, candidate domain.Snapshot) []render.BoardUnrenderable {
	if len(unrenderable) == 0 {
		return unrenderable
	}
	present := make(map[string]bool)
	for _, c := range candidate.Changes() {
		present[c.Path()] = true
	}
	var out []render.BoardUnrenderable
	for _, u := range unrenderable {
		if !present[u.Path] {
			out = append(out, u)
		}
	}
	return out
}

// includeADRIndex renders the candidate after-state through the canonical
// ADR-index renderer and appends the index (README.md) to files. The ADR
// operations always change the index, so the mutation is declared
// unconditionally — a create when the index is absent, a replace otherwise. On a
// render or probe error the function returns the error and leaves files
// unmodified (no partial append).
//
// unrenderable is the caller's adrIndexUnrenderable(st.State, adrsDir): the ADR
// records the candidate cannot see, surfaced in the index's repair notice so an
// unparseable unrelated ADR stays visible (change 0449). Entries the candidate
// does carry are dropped, exactly as includeBoard drops them.
func includeADRIndex(ctx context.Context, tree transaction.Tree, candidate domain.Snapshot, unrenderable []render.BoardUnrenderable, indexPath string, files *[]transaction.FileMutation) error {
	indexBytes, err := renderCanonicalADRIndex(candidate, withoutCandidateADRs(unrenderable, candidate))
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

// withoutCandidateADRs drops the entries whose path the candidate snapshot
// carries as an ADR — withoutCandidateChanges' ADR counterpart.
func withoutCandidateADRs(unrenderable []render.BoardUnrenderable, candidate domain.Snapshot) []render.BoardUnrenderable {
	if len(unrenderable) == 0 {
		return unrenderable
	}
	present := make(map[string]bool)
	for _, a := range candidate.ADRs() {
		present[a.Path()] = true
	}
	var out []render.BoardUnrenderable
	for _, u := range unrenderable {
		if !present[u.Path] {
			out = append(out, u)
		}
	}
	return out
}
