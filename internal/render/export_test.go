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
