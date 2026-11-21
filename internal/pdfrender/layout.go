package pdfrender

import (
	"strconv"
	"strings"
)

// Page geometry, in points: US Letter with 1-inch margins.
const (
	pageWidth  = 612.0
	pageHeight = 792.0
	margin     = 72.0
	textLeft   = margin
	textRight  = pageWidth - margin
	textTop    = margin
	textBottom = pageHeight - margin

	bodySize   = 10.5
	lineFactor = 1.35 // line box height as a multiple of the font size

	footerSize = 9.0
	footerY    = pageHeight - 48 // top of the footer's line box

	tintGray = 236 // inline code background
	ruleGray = 150 // thematic breaks
	tintPad  = 1.0 // how far a tint reaches past its text on each side

	// fitSlack absorbs float rounding when a line is filled exactly.
	fitSlack = 0.001
)

// layout places text on pages. It owns the vertical position, the current
// text box [left, right], and the placeholder count of everything drawn.
type layout struct {
	c            canvas
	y            float64
	left, right  float64
	indents      []float64
	placeholders int

	// markers are list markers waiting for the next line drawn; bars are
	// the open blockquotes' left rules; bottom is the bottom of the last
	// thing drawn on this page. See startLine in blocks.go.
	markers []marker
	bars    []bar
	bottom  float64
}

// newLayout starts the first page.
func newLayout(c canvas) *layout {
	l := &layout{c: c, left: textLeft, right: textRight}
	l.newPage()
	return l
}

// newPage starts a page, draws its footer, and moves to the top of its text
// area.
func (l *layout) newPage() {
	l.closeBars()
	l.c.newPage()
	l.y = textTop
	l.bottom = textTop
	s := strconv.Itoa(l.c.pageNo())
	w := l.c.measure(faceSans, footerSize, s)
	l.c.text((pageWidth-w)/2, footerY, faceSans, footerSize, s)
}

// atTop reports whether nothing has been drawn on the current page yet.
func (l *layout) atTop() bool { return l.y <= textTop }

// ensure starts a new page when h more points do not fit on this one. On a
// fresh page it does nothing, so content taller than a page cannot make it
// start pages forever.
func (l *layout) ensure(h float64) {
	if l.y+h > textBottom && !l.atTop() {
		l.newPage()
	}
}

// gap adds vertical space before a block, except at the top of a page.
func (l *layout) gap(h float64) {
	if !l.atTop() {
		l.y += h
	}
}

// minBoxWidth is the narrowest the text box gets: indentation past it is
// ignored, so deep nesting cannot squeeze text into nothing.
const minBoxWidth = 144.0

// pushIndent moves the left edge of the text box right by dx, never leaving
// it narrower than minBoxWidth, until the matching popIndent.
func (l *layout) pushIndent(dx float64) {
	dx = max(0, min(dx, l.right-minBoxWidth-l.left))
	l.indents = append(l.indents, dx)
	l.left += dx
}

// popIndent undoes the latest pushIndent.
func (l *layout) popIndent() {
	n := len(l.indents) - 1
	l.left -= l.indents[n]
	l.indents = l.indents[:n]
}

// run is inline text in one style, or a hard line break.
type run struct {
	text string
	st   style
	hard bool
}

// word is an unbreakable stretch of pieces, the space that follows it (drawn
// only when another word follows on the same line), and whether a hard break
// ends the line after it.
type word struct {
	parts []piece
	w     float64
	trail *piece
	hard  bool
}

func (l *layout) measured(p piece) piece {
	p.w = l.c.measure(p.f, p.st.size, p.text)
	return p
}

// words screens runs through resolve, counting placeholders, and splits them
// into words on spaces. A word may span several styles; a run of spaces
// collapses to one.
func (l *layout) words(runs []run) []word {
	var out []word
	var cur word
	flush := func() {
		out = append(out, cur)
		cur = word{}
	}
	for _, rn := range runs {
		if rn.hard {
			cur.hard = true
			flush()
			continue
		}
		pieces, n := resolve(rn.text, rn.st)
		l.placeholders += n
		for _, p := range pieces {
			s := p.text
			for s != "" {
				if s[0] == ' ' {
					s = strings.TrimLeft(s, " ")
					if len(cur.parts) > 0 {
						sp := l.measured(piece{text: " ", f: p.f, st: p.st})
						cur.trail = &sp
						flush()
					}
					continue
				}
				i := strings.IndexByte(s, ' ')
				if i < 0 {
					i = len(s)
				}
				part := l.measured(piece{text: s[:i], f: p.f, st: p.st})
				cur.parts = append(cur.parts, part)
				cur.w += part.w
				s = s[i:]
			}
		}
	}
	if len(cur.parts) > 0 {
		flush()
	}
	return out
}

// paragraph lays runs out as wrapped lines in the current text box. size is
// the line height's base when a line holds no text.
func (l *layout) paragraph(runs []run, size float64) {
	l.flow(l.words(runs), size)
}

// flow fills lines greedily. A word that does not fit goes to the next line;
// a word wider than a whole line is split by rune.
//
// It always terminates: each pass places a word, ends a non-empty line (so
// the next pass sees an empty one), or splits off at least one rune.
func (l *layout) flow(ws []word, size float64) {
	width := l.right - l.left
	var line []piece
	lineW := 0.0
	var trail *piece
	empty := true
	emit := func() {
		l.drawLine(line, size)
		line, lineW, trail, empty = nil, 0, nil, true
	}
	for i := 0; i < len(ws); {
		w := &ws[i]
		gap := 0.0
		if !empty && trail != nil {
			gap = trail.w
		}
		if lineW+gap+w.w <= width+fitSlack || (empty && len(w.parts) == 0) {
			if !empty && trail != nil {
				line = append(line, *trail)
			}
			line = append(line, w.parts...)
			lineW += gap + w.w
			trail, empty = w.trail, false
			i++
			if w.hard {
				emit()
			}
			continue
		}
		if !empty {
			emit()
			continue
		}
		head, tail := l.split(w.parts, width)
		line = head
		emit()
		if len(tail) == 0 {
			i++
			continue
		}
		w.parts, w.w = tail, 0
		for _, p := range tail {
			w.w += p.w
		}
	}
	if !empty {
		emit()
	}
}

// split cuts parts after the longest run of runes that fits width, always
// taking at least one rune so the caller makes progress even when a single
// rune is wider than the box.
func (l *layout) split(parts []piece, width float64) (head, tail []piece) {
	acc := 0.0
	taken := 0
	for pi, p := range parts {
		var b strings.Builder
		for bi, r := range p.text {
			rw := l.c.measure(p.f, p.st.size, string(r))
			if taken > 0 && acc+rw > width+fitSlack {
				if b.Len() > 0 {
					head = append(head, l.measured(piece{text: b.String(), f: p.f, st: p.st}))
				}
				rest := l.measured(piece{text: p.text[bi:], f: p.f, st: p.st})
				tail = append([]piece{rest}, parts[pi+1:]...)
				return head, tail
			}
			b.WriteRune(r)
			acc += rw
			taken++
		}
		head = append(head, p)
	}
	return head, nil
}

// drawLine draws one line of pieces at the current position and moves below
// it, starting a new page first when the line does not fit. Pieces of
// different sizes share one baseline. Adjacent pieces in the same face and
// style are drawn as one.
func (l *layout) drawLine(line []piece, size float64) {
	maxSize := size
	if len(line) > 0 {
		maxSize = 0
		for _, p := range line {
			maxSize = max(maxSize, p.st.size)
		}
	}
	lh := maxSize * lineFactor
	l.ensure(lh)
	l.startLine(maxSize)

	var merged []piece
	for _, p := range line {
		if n := len(merged); n > 0 && merged[n-1].f == p.f && merged[n-1].st == p.st {
			merged[n-1].text += p.text
			merged[n-1].w += p.w
			continue
		}
		merged = append(merged, p)
	}

	baseline := l.y + maxSize*baselineFactor
	type placed struct {
		p    piece
		x, y float64
	}
	pl := make([]placed, 0, len(merged))
	x := l.left
	for _, p := range merged {
		w := l.c.measure(p.f, p.st.size, p.text)
		p.w = w
		pl = append(pl, placed{p, x, baseline - p.st.size*baselineFactor})
		x += w
	}
	for _, q := range pl {
		if q.p.st.tint {
			box := q.p.st.size * lineFactor
			l.c.fillRect(q.x-tintPad, q.y+box*0.1, q.p.w+2*tintPad, box*0.8, tintGray)
		}
	}
	for _, q := range pl {
		l.c.text(q.x, q.y, q.p.f, q.p.st.size, q.p.text)
	}
	for _, q := range pl {
		if q.p.st.strike {
			mid := q.y + q.p.st.size*lineFactor/2
			l.c.line(q.x, mid, q.x+q.p.w, mid, max(0.5, q.p.st.size*0.06), 0)
		}
	}
	l.y += lh
	l.bottom = l.y
}

// rule draws a thematic break: a horizontal line across the text box.
func (l *layout) rule() {
	h := bodySize * lineFactor
	l.ensure(h)
	l.startLine(bodySize)
	mid := l.y + h/2
	l.c.line(l.left, mid, l.right, mid, 0.6, ruleGray)
	l.y += h
	l.bottom = l.y
}
