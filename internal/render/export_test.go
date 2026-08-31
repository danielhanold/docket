package render

// BoardClassifyForTest exposes the unexported boardClassify to the external
// render_test package so classification can be asserted directly, without
// routing every case through the full Board() render (change 0367).
var BoardClassifyForTest = boardClassify
