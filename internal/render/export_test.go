package render

// BoardClassifyForTest exposes the unexported boardClassify to the external
// render_test package so classification can be asserted directly, without
// routing every case through the full Board() render (change 0367).
var BoardClassifyForTest = boardClassify

// SortBoardSectionForTest exposes the unexported sortBoardSection to the
// external render_test package so the per-section comparator can be asserted
// directly, without routing every case through the full Board() render
// (change 0367).
var SortBoardSectionForTest = sortBoardSection

// BoardSortKeysForTest and BoardDirectionsForTest expose the render sort-key
// and direction value vocabularies to the external render_test package, derived
// from the typed render constants (not string literals) so the cross-package
// config↔render vocabulary guard reddens when a render value string is renamed
// (change 0367). The section vocabulary needs no hook: it is already reachable
// through the exported DefaultBoardPresentation().SectionOrder.
var (
	BoardSortKeysForTest = []BoardSortKey{
		BoardSortKeyID, BoardSortKeyUpdated, BoardSortKeyCreated,
	}
	BoardDirectionsForTest = []BoardDirection{
		BoardDirectionAsc, BoardDirectionDesc,
	}
)
