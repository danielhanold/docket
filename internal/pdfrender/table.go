package pdfrender

import extast "github.com/yuin/goldmark/extension/ast"

const (
	cellPadX    = 4.0 // gap between a column's border and its text
	cellPadY    = 2.5 // gap between a row's border and its text
	borderWidth = 0.5
	borderGray  = 140
)

// tableRow is a row laid out for drawing: each cell's wrapped lines, and the
// row's height, which is its tallest cell's plus padding. Rows are laid out
// once, so the header can be redrawn on every page without screening its
// text, or counting its placeholders, again.
type tableRow struct {
	cells [][][]piece
	h     float64
}

// table draws a GFM table: thin borders, a bold header row, and columns as
// wide as their widest cell's single line, scaled down proportionally when
// they do not fit the text box. Cell text wraps inside its column.
//
// A row that fits on a page is never split: one that does not fit on this
// page starts a new one, under a fresh copy of the header. A row taller than
// a page (less its header) is the exception: its cells' lines fill the rest
// of this page, and each cell's remaining lines carry on under the header on
// the next, page after page, so no line runs past the bottom margin.
func (w *walker) table(n *extast.Table) {
	l := w.l
	var rows [][][]word
	var aligns []extast.Alignment
	hasHeader := false
	for r := n.FirstChild(); r != nil; r = r.NextSibling() {
		st := bodyStyle
		if _, ok := r.(*extast.TableHeader); ok && len(rows) == 0 {
			st.bold, hasHeader = true, true
		}
		var cells [][]word
		for c := r.FirstChild(); c != nil; c = c.NextSibling() {
			col := len(cells)
			if col == len(aligns) {
				aligns = append(aligns, extast.AlignNone)
			}
			if cell, ok := c.(*extast.TableCell); ok && aligns[col] == extast.AlignNone {
				aligns[col] = cell.Alignment
			}
			cells = append(cells, l.words(w.inlines(c, st)))
		}
		rows = append(rows, cells)
	}
	cols := len(aligns)
	if cols == 0 {
		return
	}

	natural := make([]float64, cols)
	for i := range natural {
		natural[i] = 2 * cellPadX
	}
	for _, cells := range rows {
		for i, ws := range cells {
			natural[i] = max(natural[i], singleLineWidth(ws)+2*cellPadX)
		}
	}
	widths := fitColumns(natural, l.right-l.left)
	edges := make([]float64, cols+1)
	edges[0] = l.left
	for i, cw := range widths {
		edges[i+1] = edges[i] + cw
	}

	laid := make([]tableRow, len(rows))
	for ri, cells := range rows {
		lines := make([][][]piece, cols)
		for i, ws := range cells {
			l.wrap(ws, widths[i]-2*cellPadX, func(line []piece) { lines[i] = append(lines[i], line) })
		}
		laid[ri] = newTableRow(lines)
	}

	var header *tableRow
	body := laid
	if hasHeader {
		header, body = &laid[0], laid[1:]
	}

	l.gap(bodySize * 0.6)
	first := 0.0
	if header != nil {
		first += header.h
	}
	if len(body) > 0 {
		first += body[0].h
	}
	// The header never sits alone at the foot of a page.
	l.ensure(first)
	// fresh is true while nothing but the header is drawn on this page, so
	// a row that does not fit now would not fit on a new page either.
	fresh := l.atTop()
	top := true
	if header != nil {
		w.tableRow(header, edges, aligns, top)
		top = false
	}
	nextPage := func() {
		l.newPage()
		fresh, top = true, true
		if header != nil {
			w.tableRow(header, edges, aligns, top)
			top = false
		}
	}
	freshSpace := textBottom - textTop
	if header != nil {
		freshSpace -= header.h
	}
	for i := range body {
		row := &body[i]
		for row != nil && l.y+row.h > textBottom {
			if !fresh && row.h <= freshSpace {
				nextPage()
				continue
			}
			head, tail := splitRow(*row, textBottom-l.y, fresh)
			if head == nil {
				nextPage()
				continue
			}
			w.tableRow(head, edges, aligns, top)
			top, row = false, tail
			if tail != nil {
				nextPage()
			}
		}
		if row != nil {
			w.tableRow(row, edges, aligns, top)
		}
		top, fresh = false, false
	}
}

// newTableRow lays out a row from each cell's wrapped lines: its height is
// its tallest cell's, at least one body line, plus padding.
func newTableRow(cells [][][]piece) tableRow {
	textH := bodySize * lineFactor
	for _, lines := range cells {
		h := 0.0
		for _, line := range lines {
			h += lineSize(line, bodySize) * lineFactor
		}
		textH = max(textH, h)
	}
	return tableRow{cells: cells, h: textH + 2*cellPadY}
}

// splitRow splits row so its head fits in avail points: each cell keeps as
// many of its leading lines as fit, and the tail holds the rest. head is nil
// when no line fits, unless force, when each cell keeps at least its first
// line so a page always makes progress. tail is nil when nothing is left.
func splitRow(row tableRow, avail float64, force bool) (head, tail *tableRow) {
	room := avail - 2*cellPadY
	heads := make([][][]piece, len(row.cells))
	tails := make([][][]piece, len(row.cells))
	fits, left := false, false
	for i, lines := range row.cells {
		n, h := 0, 0.0
		for n < len(lines) {
			lh := lineSize(lines[n], bodySize) * lineFactor
			if h+lh > room {
				break
			}
			h += lh
			n++
		}
		heads[i], tails[i] = lines[:n], lines[n:]
		fits = fits || n > 0
	}
	if !fits {
		if !force {
			return nil, nil
		}
		for i, lines := range row.cells {
			if len(lines) > 0 {
				heads[i], tails[i] = lines[:1], lines[1:]
			}
		}
	}
	for _, lines := range tails {
		left = left || len(lines) > 0
	}
	h := newTableRow(heads)
	head = &h
	if left {
		t := newTableRow(tails)
		tail = &t
	}
	return head, tail
}

// tableRow draws row at the current position between the column edges and
// moves below it. top draws the border above it too: the first row drawn on
// a page has nothing above it to share a border with.
func (w *walker) tableRow(row *tableRow, edges []float64, aligns []extast.Alignment, top bool) {
	l := w.l
	// Pending list markers line up with the cells' first line.
	l.y += cellPadY
	l.startLine(bodySize)
	l.y -= cellPadY

	rowTop := l.y
	for i, lines := range row.cells {
		left, right := edges[i]+cellPadX, edges[i+1]-cellPadX
		y := rowTop + cellPadY
		for _, line := range lines {
			x := left
			if spare := right - left - sumWidth(line); spare > 0 {
				switch aligns[i] {
				case extast.AlignRight:
					x += spare
				case extast.AlignCenter:
					x += spare / 2
				}
			}
			size := lineSize(line, bodySize)
			l.placeLine(line, x, y, size)
			y += size * lineFactor
		}
	}

	bottom := rowTop + row.h
	if top {
		l.c.line(edges[0], rowTop, edges[len(edges)-1], rowTop, borderWidth, borderGray)
	}
	l.c.line(edges[0], bottom, edges[len(edges)-1], bottom, borderWidth, borderGray)
	for _, x := range edges {
		l.c.line(x, rowTop, x, bottom, borderWidth, borderGray)
	}
	l.y = bottom
	l.bottom = l.y
}

// singleLineWidth is the width of ws set on one line, or of its widest line
// when a hard break ends a word.
func singleLineWidth(ws []word) float64 {
	widest, cur := 0.0, 0.0
	for i, wd := range ws {
		cur += wd.w
		if wd.hard {
			widest, cur = max(widest, cur), 0
			continue
		}
		if wd.trail != nil && i+1 < len(ws) {
			cur += wd.trail.w
		}
	}
	return max(widest, cur)
}

// fitColumns returns natural when it fits in avail. Otherwise it scales the
// columns proportionally into avail, but never below a floor: an equal share
// of avail, or a column's natural width when that is narrower. So a short
// column, an id or a status, keeps its natural width beside a long one, and
// no column vanishes. A column that would fall below its floor is held there,
// and what is left is shared among the rest in proportion again.
func fitColumns(natural []float64, avail float64) []float64 {
	total := 0.0
	for _, nw := range natural {
		total += nw
	}
	widths := make([]float64, len(natural))
	if total <= avail {
		copy(widths, natural)
		return widths
	}
	share := avail / float64(len(natural))
	held := make([]bool, len(natural))
	// Each pass holds at least one more column or returns, so it ends.
	for {
		free, freeNatural := avail, 0.0
		for i, nw := range natural {
			if held[i] {
				free -= min(share, nw)
			} else {
				freeNatural += nw
			}
		}
		changed := false
		for i, nw := range natural {
			if held[i] {
				widths[i] = min(share, nw)
				continue
			}
			widths[i] = nw * free / freeNatural
			if widths[i] < min(share, nw) {
				held[i], changed = true, true
			}
		}
		if !changed {
			return widths
		}
	}
}
