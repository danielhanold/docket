package pdfrender

import (
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

const (
	codeSize = 9.5 // code block font size
	codePad  = 4.0 // gap between a code block's background edge and its text
	codeGray = 242 // code block background
	tabWidth = 4   // code tab stops, in columns

	listIndent = 16.0 // the least a list item is indented
	markerGap  = 5.0  // space between a list marker and its item's text

	quoteIndent = 14.0 // how far each blockquote level moves the text right
	quoteBarX   = 3.0  // where a blockquote's rule sits inside its indent
	quoteWidth  = 1.5  // the rule's line width
	quoteGray   = 190  // the rule's grey
)

// marker is a list marker waiting to be drawn at the start of the next line;
// right is the x its text ends at.
type marker struct {
	parts []piece
	right float64
}

// bar is an open blockquote's left rule. It is drawn as one segment per page,
// from the first line drawn inside the quote on that page to the bottom of
// the last.
type bar struct {
	x       float64
	start   float64
	started bool
}

// startLine runs at the top of every line, after any page break, for a line
// whose largest text is size. It draws pending list markers on the line's
// baseline and starts the open blockquote rules on this page.
func (l *layout) startLine(size float64) {
	for _, m := range l.markers {
		x := m.right
		for _, p := range m.parts {
			x -= p.w
		}
		for _, p := range m.parts {
			l.c.text(x, l.y+(size-p.st.size)*baselineFactor, p.f, p.st.size, p.text)
			x += p.w
		}
	}
	l.markers = l.markers[:0]
	for i := range l.bars {
		if !l.bars[i].started {
			l.bars[i].start, l.bars[i].started = l.y, true
		}
	}
}

// closeBars draws the segment of every open blockquote rule on this page.
func (l *layout) closeBars() {
	for i := range l.bars {
		l.closeBar(&l.bars[i])
	}
}

func (l *layout) closeBar(b *bar) {
	if b.started && l.bottom > b.start {
		l.c.line(b.x, b.start, b.x, l.bottom, quoteWidth, quoteGray)
	}
	b.started = false
}

// screened resolves s in st into measured pieces, counting placeholders.
func (l *layout) screened(s string, st style) []piece {
	pieces, n := resolve(s, st)
	l.placeholders += n
	for i := range pieces {
		pieces[i] = l.measured(pieces[i])
	}
	return pieces
}

// list draws a bullet, ordered, or task list. Each item is indented past its
// marker, which is drawn on the item's first line, right-aligned.
func (w *walker) list(n *ast.List) {
	if _, nested := n.Parent().(*ast.ListItem); !nested {
		w.l.gap(bodySize * 0.35)
	}
	var markers [][]piece
	indent := listIndent
	num := n.Start
	for it := n.FirstChild(); it != nil; it = it.NextSibling() {
		label := "•"
		switch {
		case taskBox(it) != nil:
			label = "☐"
			if taskBox(it).IsChecked {
				label = "☑"
			}
		case n.IsOrdered():
			label = strconv.Itoa(num) + string(n.Marker)
			num++
		}
		parts := w.l.screened(label, bodyStyle)
		width := 0.0
		for _, p := range parts {
			width += p.w
		}
		indent = max(indent, width+markerGap+1)
		markers = append(markers, parts)
	}
	i := 0
	for it := n.FirstChild(); it != nil; it = it.NextSibling() {
		w.l.pushIndent(indent)
		w.l.markers = append(w.l.markers, marker{parts: markers[i], right: w.l.left - markerGap})
		i++
		w.children(it)
		if len(w.l.markers) > 0 {
			// An empty item still shows its marker.
			w.l.drawLine(nil, bodySize)
		}
		w.l.popIndent()
	}
}

// taskBox is the task-list checkbox opening item, or nil.
func taskBox(item ast.Node) *extast.TaskCheckBox {
	if fc := item.FirstChild(); fc != nil {
		if box, ok := fc.FirstChild().(*extast.TaskCheckBox); ok {
			return box
		}
	}
	return nil
}

// blockquote draws its children indented, with a rule down its left edge.
func (w *walker) blockquote(n *ast.Blockquote) {
	w.l.bars = append(w.l.bars, bar{x: w.l.left + quoteBarX})
	w.l.pushIndent(quoteIndent)
	w.children(n)
	w.l.popIndent()
	last := len(w.l.bars) - 1
	w.l.closeBar(&w.l.bars[last])
	w.l.bars = w.l.bars[:last]
	w.l.gap(bodySize * 0.3)
}

// rawLines is the text of each source line of a raw block (code or HTML),
// without its line ending.
func (w *walker) rawLines(n ast.Node) []string {
	var lines []string
	segs := n.Lines()
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		lines = append(lines, strings.TrimRight(string(seg.Value(w.src)), "\r\n"))
	}
	if h, ok := n.(*ast.HTMLBlock); ok && h.HasClosure() {
		lines = append(lines, strings.TrimRight(string(h.ClosureLine.Value(w.src)), "\r\n"))
	}
	return lines
}

// literalLines draws a raw block's lines (an HTML block's, say) as literal
// text in the body face, one paragraph with a hard break between lines.
func (w *walker) literalLines(n ast.Node) {
	var runs []run
	for i, s := range w.rawLines(n) {
		if i > 0 {
			runs = append(runs, run{hard: true, st: bodyStyle})
		}
		runs = append(runs, run{text: s, st: bodyStyle})
	}
	if len(runs) > 0 {
		w.para(runs)
	}
}

// codeBlock draws a fenced or indented code block line by line in mono on a
// grey background. Whitespace is kept and tabs expanded; a line wider than
// the box wraps by rune and is never reflowed.
func (w *walker) codeBlock(n ast.Node) {
	l := w.l
	l.gap(bodySize * 0.6)
	st := style{mono: true, size: codeSize}
	for _, s := range w.rawLines(n) {
		parts := l.screened(expandTabs(s), st)
		for {
			head, tail := parts, []piece(nil)
			if width := l.right - l.left - 2*codePad; sumWidth(parts) > width+fitSlack {
				head, tail = l.split(parts, width)
			}
			l.codeLine(head)
			if len(tail) == 0 {
				break
			}
			parts = tail
		}
	}
}

func sumWidth(parts []piece) float64 {
	total := 0.0
	for _, p := range parts {
		total += p.w
	}
	return total
}

// codeLine draws one visual line of code with its background.
func (l *layout) codeLine(parts []piece) {
	lh := codeSize * lineFactor
	l.ensure(lh)
	l.startLine(codeSize)
	l.c.fillRect(l.left, l.y, l.right-l.left, lh, codeGray)
	x := l.left + codePad
	for _, p := range parts {
		l.c.text(x, l.y, p.f, p.st.size, p.text)
		x += p.w
	}
	l.y += lh
	l.bottom = l.y
}

// expandTabs replaces each tab with spaces up to the next multiple of
// tabWidth columns, counting one column per rune.
func expandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}
