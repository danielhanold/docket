package pdfrender

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Render draws markdown as a PDF and returns its bytes and how many
// characters were drawn as Placeholder. The PDF's Title is the plain text of
// the first H1, else fallbackTitle. The same input always gives the same
// bytes, and Render is safe for concurrent calls. A panic anywhere in parsing,
// layout, or fpdf is returned as an error, never raised.
func Render(markdown []byte, fallbackTitle string) (pdf []byte, placeholders int, err error) {
	defer func() {
		if r := recover(); r != nil {
			pdf, placeholders, err = nil, 0, fmt.Errorf("pdf renderer: %v", r)
		}
	}()
	c := newFpdfCanvas()
	title, n := render(markdown, fallbackTitle, c)
	// title holds only what resolve let through, so fpdf can encode it.
	c.pdf.SetTitle(title, true)
	if err := c.pdf.Error(); err != nil {
		return nil, 0, fmt.Errorf("pdf renderer: %w", err)
	}
	var buf bytes.Buffer
	if err := c.pdf.Output(&buf); err != nil {
		return nil, 0, fmt.Errorf("pdf renderer: %w", err)
	}
	return buf.Bytes(), n, nil
}

// headingSizes are the font sizes of H1 to H6; H6 is the body size.
var headingSizes = [6]float64{22, 18, 15, 13, 11.5, bodySize}

// render parses md and draws it on c, returning the document title and the
// placeholder count. Render and the tests share it.
func render(md []byte, fallbackTitle string, c canvas) (title string, placeholders int) {
	// A fresh parser per call: nothing is shared between concurrent renders.
	p := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList)).Parser()
	doc := p.Parse(text.NewReader(md))
	w := &walker{src: md, l: newLayout(c)}
	w.block(doc)
	if !w.haveTitle {
		w.title = screenTitle([]run{{text: fallbackTitle, st: style{size: bodySize}}})
	}
	return w.title, w.l.placeholders
}

// walker draws a goldmark document block by block.
type walker struct {
	src       []byte
	l         *layout
	title     string
	haveTitle bool
}

var bodyStyle = style{size: bodySize}

func (w *walker) children(n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		w.block(c)
	}
}

func (w *walker) block(n ast.Node) {
	switch n := n.(type) {
	case *ast.Document:
		w.children(n)
	case *ast.Heading:
		w.heading(n)
	case *ast.Paragraph, *ast.TextBlock:
		w.para(w.inlines(n, bodyStyle))
	case *ast.ThematicBreak:
		w.l.gap(bodySize * 0.3)
		w.l.rule()
	default:
		w.fallback(n)
	}
}

// fallback draws a block kind that has no dedicated rendering as plain
// paragraphs of its text: a block of inline content as one paragraph, a
// container through its children, and a block of raw lines (code, HTML) as
// one paragraph with a hard break between lines.
func (w *walker) fallback(n ast.Node) {
	if n.Type() != ast.TypeBlock {
		return
	}
	if fc := n.FirstChild(); fc != nil {
		if fc.Type() == ast.TypeInline {
			w.para(w.inlines(n, bodyStyle))
			return
		}
		w.children(n)
		return
	}
	var lines []string
	segs := n.Lines()
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		lines = append(lines, string(seg.Value(w.src)))
	}
	if h, ok := n.(*ast.HTMLBlock); ok && h.HasClosure() {
		lines = append(lines, string(h.ClosureLine.Value(w.src)))
	}
	var runs []run
	for i, s := range lines {
		if i > 0 {
			runs = append(runs, run{hard: true, st: bodyStyle})
		}
		runs = append(runs, run{text: strings.TrimRight(s, "\r\n"), st: bodyStyle})
	}
	if len(runs) > 0 {
		w.para(runs)
	}
}

func (w *walker) para(runs []run) {
	w.l.gap(bodySize * 0.6)
	w.l.paragraph(runs, bodySize)
}

func (w *walker) heading(n *ast.Heading) {
	level := min(max(n.Level, 1), 6)
	size := headingSizes[level-1]
	runs := w.inlines(n, style{bold: true, size: size})
	if level == 1 && !w.haveTitle {
		if t := screenTitle(runs); t != "" {
			w.title, w.haveTitle = t, true
		}
	}
	w.l.gap(size * 0.8)
	w.l.paragraph(runs, size)
}

// screenTitle is the plain text of runs with whitespace collapsed, screened
// by resolve. Its placeholders are not counted: the title is not drawn.
func screenTitle(runs []run) string {
	var raw strings.Builder
	for _, r := range runs {
		if r.hard {
			raw.WriteByte(' ')
			continue
		}
		raw.WriteString(r.text)
	}
	pieces, _ := resolve(raw.String(), style{size: bodySize})
	var b strings.Builder
	for _, p := range pieces {
		b.WriteString(p.text)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// inlines collects the inline content of n as runs in st.
func (w *walker) inlines(n ast.Node, st style) []run {
	var out []run
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		out = w.inline(c, st, out)
	}
	return out
}

func (w *walker) inline(n ast.Node, st style, out []run) []run {
	switch n := n.(type) {
	case *ast.Text:
		v := n.Value(w.src)
		if !n.IsRaw() {
			v = unescape(v)
		}
		out = append(out, run{text: string(v), st: st})
		switch {
		case n.HardLineBreak():
			out = append(out, run{hard: true, st: st})
		case n.SoftLineBreak():
			out = append(out, run{text: " ", st: st})
		}
		return out
	case *ast.String:
		return append(out, run{text: string(n.Value), st: st})
	case *ast.CodeSpan:
		// Code keeps its text exactly: no escapes or entities are resolved.
		cs := st
		cs.mono, cs.tint = true, true
		var b strings.Builder
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			switch t := c.(type) {
			case *ast.Text:
				b.Write(t.Value(w.src))
			case *ast.String:
				b.Write(t.Value)
			}
		}
		return append(out, run{text: b.String(), st: cs})
	case *ast.Emphasis:
		es := st
		if n.Level >= 2 {
			es.bold = true
		} else {
			es.italic = true
		}
		return append(out, w.inlines(n, es)...)
	case *extast.Strikethrough:
		ss := st
		ss.strike = true
		return append(out, w.inlines(n, ss)...)
	case *ast.AutoLink:
		return append(out, run{text: string(n.Label(w.src)), st: st})
	case *ast.RawHTML:
		var b strings.Builder
		for i := 0; i < n.Segments.Len(); i++ {
			seg := n.Segments.At(i)
			b.Write(seg.Value(w.src))
		}
		return append(out, run{text: b.String(), st: st})
	case *extast.TaskCheckBox:
		box := "☐ "
		if n.IsChecked {
			box = "☑ "
		}
		return append(out, run{text: box, st: st})
	}
	// Links draw their text and images their alt text: both are children.
	return append(out, w.inlines(n, st)...)
}

// unescape resolves backslash escapes and entity references in prose text the
// way CommonMark does: an escaped character is literal, so `\&amp;` stays
// `&amp;`, and references are resolved once, so `&amp;#35;` gives `&#35;`.
func unescape(v []byte) []byte {
	if bytes.IndexByte(v, '\\') < 0 && bytes.IndexByte(v, '&') < 0 {
		return v
	}
	resolveRefs := func(b []byte) []byte {
		return util.ResolveEntityNames(util.ResolveNumericReferences(b))
	}
	var out []byte
	start := 0
	for i := 0; i < len(v); i++ {
		if v[i] == '\\' && i+1 < len(v) && util.IsPunct(v[i+1]) {
			out = append(out, resolveRefs(v[start:i])...)
			out = append(out, v[i+1])
			i++
			start = i + 1
		}
	}
	return append(out, resolveRefs(v[start:])...)
}
