package pdfrender

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

const deterministicDoc = "# Widget\n\nSome *text* with `code`, → arrows — and ✅ done.\n\n## Next\n\nMore.\n"

func TestRenderDeterministicPDF(t *testing.T) {
	first, _, err := Render([]byte(deterministicDoc), "doc")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(first, []byte("%PDF-")) || !bytes.Contains(first, []byte("%%EOF")) {
		t.Fatalf("output is not a PDF: %q", first[:min(len(first), 40)])
	}
	second, _, err := Render([]byte(deterministicDoc), "doc")
	if err != nil {
		t.Fatalf("Render again: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two renders of the same input differ")
	}

	var wg sync.WaitGroup
	outs := make([][]byte, 2)
	errs := make([]error, 2)
	for i := range outs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outs[i], _, errs[i] = Render([]byte(deterministicDoc), "doc")
		}()
	}
	wg.Wait()
	for i := range outs {
		if errs[i] != nil {
			t.Fatalf("concurrent Render %d: %v", i, errs[i])
		}
		if !bytes.Equal(outs[i], first) {
			t.Fatalf("concurrent Render %d differs from the serial render", i)
		}
	}

	// The metadata carries a Title and names nothing else: no author, creator,
	// subject, or keywords.
	if !bytes.Contains(first, []byte("/Title ")) {
		t.Error("PDF has no /Title")
	}
	for _, key := range []string{"/Author", "/Creator", "/Subject", "/Keywords"} {
		if bytes.Contains(first, []byte(key)) {
			t.Errorf("PDF metadata sets %s", key)
		}
	}
}

// assertBMPOnly fails when any recorded text holds a rune above U+FFFF or
// invalid UTF-8: those are exactly what fpdf cannot take.
func assertBMPOnly(t *testing.T, r *recorder) {
	t.Helper()
	for _, d := range r.ops {
		if !utf8.ValidString(d.s) {
			t.Errorf("drawn text is not valid UTF-8: %q", d.s)
		}
		for _, c := range d.s {
			if c > 0xFFFF {
				t.Errorf("drawn text %q holds %U above U+FFFF", d.s, c)
			}
		}
	}
}

func TestRenderNonBMPIsPlaceholderEverywhere(t *testing.T) {
	md := "# T 🟡\n\npara 🗑 *em 🟢*\n\n`code 🟡`\n\n[link 🟡](http://x)\n"
	r, title, n := renderRec(t, md)
	if n != 5 {
		t.Errorf("placeholders = %d, want 5", n)
	}
	assertBMPOnly(t, r)
	if got := strings.Count(texts(r), string(Placeholder)); got != 5 {
		t.Errorf("drew %d placeholders, want 5", got)
	}
	if title != "T □" {
		t.Errorf("title = %q, want %q", title, "T □")
	}
	pdf, pn, err := Render([]byte(md), "x")
	if err != nil || pn != 5 || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("Render = (%d bytes, %d, %v), want a PDF, 5, nil", len(pdf), pn, err)
	}

	// A numeric character reference resolves to the rune and is screened too.
	r, _, n = renderRec(t, "x &#x1F7E1; y\n")
	if n != 1 {
		t.Errorf("entity: placeholders = %d, want 1", n)
	}
	assertBMPOnly(t, r)

	// The fallback title is screened the same way, and is not counted: it is
	// not drawn.
	_, title, n = renderRec(t, "## x\n")
	if n != 0 || title != "fallback-title" {
		t.Errorf("fallback = (%q, %d)", title, n)
	}
	title, n = render([]byte("## x\n"), "a🟡b", newRecorder())
	if title != "a□b" || n != 0 {
		t.Errorf("screened fallback = (%q, %d), want (%q, 0)", title, n, "a□b")
	}
}

func TestRenderDropsInvisibles(t *testing.T) {
	// U+2705 has no Inter or DejaVu glyph, so it falls back to Noto Emoji.
	// U+26A0 has an Inter glyph, so the style's own face draws it.
	r, _, n := renderRec(t, "\u2705\uFE0F ok \u26A0\uFE0F a\u200Db\u200Bc\u2060d\uFEFFe\n")
	if n != 0 {
		t.Errorf("placeholders = %d, want 0", n)
	}
	check := findOne(t, r, "✅")
	if check.f != faceEmoji {
		t.Errorf("✅ drawn in face %d, want faceEmoji", check.f)
	}
	var warn bool
	for _, d := range r.ops {
		if strings.ContainsAny(d.s, "\uFE0F\u200D\u200B\u2060\uFEFF") {
			t.Errorf("drawn text %q holds an invisible", d.s)
		}
		if strings.Contains(d.s, "⚠") {
			warn = true
			if d.f != faceSans {
				t.Errorf("⚠ drawn in face %d, want faceSans", d.f)
			}
		}
	}
	if !warn {
		t.Error("⚠ was not drawn")
	}
	if !strings.Contains(texts(r), "abcde") {
		t.Errorf("zero-width characters left a gap: %q", texts(r))
	}
}

func TestRenderMissingBMPGlyph(t *testing.T) {
	r, _, n := renderRec(t, "中文\n")
	if n != 2 {
		t.Errorf("placeholders = %d, want 2", n)
	}
	if !strings.Contains(texts(r), "□□") {
		t.Errorf("drew %q, want two placeholders", texts(r))
	}
}

func TestRenderInlineStyles(t *testing.T) {
	r, _, _ := renderRec(t, "*i* **b** ***bi*** ~~s~~ `c`\n")
	for _, tc := range []struct {
		s string
		f face
	}{{"i", faceSansOblique}, {"b", faceSansBold}, {"bi", faceSansBoldOblique}, {"s", faceSans}, {"c", faceMono}} {
		if d := findOne(t, r, tc.s); d.f != tc.f {
			t.Errorf("%q drawn in face %d, want %d", tc.s, d.f, tc.f)
		}
	}

	s := findOne(t, r, "s")
	if len(r.lines) != 1 {
		t.Fatalf("want one strike line, got %+v", r.lines)
	}
	ln := r.lines[0]
	mid := s.y + s.size*lineFactor/2
	sw := r.measure(s.f, s.size, s.s)
	if ln.y1 != ln.y2 || math.Abs(ln.y1-mid) > 0.2*s.size {
		t.Errorf("strike at y %.2f..%.2f, want horizontal near %.2f", ln.y1, ln.y2, mid)
	}
	if math.Abs(ln.x1-s.x) > 0.01 || math.Abs(ln.x2-(s.x+sw)) > 0.01 {
		t.Errorf("strike spans %.2f..%.2f, want %.2f..%.2f", ln.x1, ln.x2, s.x, s.x+sw)
	}

	c := findOne(t, r, "c")
	cw := r.measure(c.f, c.size, c.s)
	var tinted bool
	for _, rc := range r.rects {
		if rc.page == c.page && rc.x <= c.x && rc.x+rc.w >= c.x+cw && rc.y >= c.y && rc.y+rc.h <= c.y+c.size*lineFactor {
			tinted = true
		}
	}
	if !tinted {
		t.Errorf("no tint behind the code span: %+v", r.rects)
	}
}

// A code span keeps its interior spaces, while prose still collapses a run of
// spaces to one; a long spaced code span still wraps and ends.
func TestRenderCodeSpanKeepsSpaces(t *testing.T) {
	r, _, _ := renderRec(t, "x  y `a  b   c`\n")
	findOne(t, r, "a  b   c")
	findOne(t, r, "x y ")

	long := strings.Repeat("ab  cd   ", 200)
	var rr *recorder
	within(t, 30*time.Second, func() { rr = record("`" + long + "`\n") })
	got := strings.ReplaceAll(bodyText(rr), " ", "")
	if want := strings.Repeat("abcd", 200); got != want {
		t.Fatalf("long code span text lost or changed: got %d bytes, want %d", len(got), len(want))
	}
	if !strings.Contains(bodyText(rr), "ab  cd   ab") {
		t.Errorf("long code span lost its interior spaces: %q", bodyText(rr)[:80])
	}
	lines := map[float64]bool{}
	for _, d := range body(rr) {
		lines[d.y] = true
	}
	if len(lines) < 2 {
		t.Errorf("long code span did not wrap: drawn on %d line(s)", len(lines))
	}
}

func TestRenderHeadings(t *testing.T) {
	r, _, _ := renderRec(t, "# A\n\n## B\n\n### C\n\n#### D\n\n##### E\n\n###### F\n")
	prev := math.Inf(1)
	for i, s := range []string{"A", "B", "C", "D", "E", "F"} {
		d := findOne(t, r, s)
		if d.f != faceSansBold {
			t.Errorf("%s drawn in face %d, want faceSansBold", s, d.f)
		}
		if d.size >= prev {
			t.Errorf("%s size %.2f not below the previous %.2f", s, d.size, prev)
		}
		prev = d.size
		if d.size != headingSizes[i] {
			t.Errorf("%s size %.2f, want %.2f", s, d.size, headingSizes[i])
		}
	}
	// GitHub's proportions: H1 to H3 above the body size, H4 at it, H5 and
	// H6 below it.
	for i, cmp := range []int{1, 1, 1, 0, -1, -1} {
		got := 0
		if headingSizes[i] > bodySize {
			got = 1
		} else if headingSizes[i] < bodySize {
			got = -1
		}
		if got != cmp {
			t.Errorf("H%d size %.2f against body %.2f: got side %d, want %d", i+1, headingSizes[i], bodySize, got, cmp)
		}
	}
}

// assertInBox fails when any body op starts left of left or ends right of right.
func assertInBox(t *testing.T, r *recorder, left, right float64) {
	t.Helper()
	for _, d := range body(r) {
		if w := r.measure(d.f, d.size, d.s); d.x < left-0.01 || d.x+w > right+0.01 {
			t.Errorf("op %q spans %.2f..%.2f, outside %.2f..%.2f", d.s, d.x, d.x+w, left, right)
		}
	}
}

func TestRenderWrapsAndBreaks(t *testing.T) {
	words := strings.Fields(strings.Repeat("lorem ipsum dolor sit amet consectetur ", 100))
	long := strings.Repeat("abcdefghij", 300)
	// An over-wide word in the middle of a paragraph, spanning three styles,
	// and one in a heading.
	mixed := strings.Repeat("x", 500) + "`" + strings.Repeat("y", 500) + "`" + strings.Repeat("z", 500)

	// Only the rendering runs under the deadline; assertions stay on the
	// test goroutine.
	var wrapped, hard, soft, longR, mixedR *recorder
	within(t, 60*time.Second, func() {
		wrapped = record(strings.Join(words, " ") + "\n")
		hard = record("a  \nb\n")
		soft = record("a\nb\n")
		longR = record(long + "\n")
		mixedR = record("short " + mixed + " tail\n\n# " + long + "\n")
	})

	ys := map[float64]bool{}
	for _, d := range body(wrapped) {
		ys[d.y] = true
	}
	if len(ys) < 2 {
		t.Errorf("600 words drew on %d line(s)", len(ys))
	}
	assertInBox(t, wrapped, textLeft, textRight)

	if a, b := findOne(t, hard, "a"), findOne(t, hard, "b"); a.y == b.y {
		t.Error("a hard break kept a and b on one line")
	}

	ys = map[float64]bool{}
	for _, d := range body(soft) {
		ys[d.y] = true
	}
	if len(ys) != 1 || bodyText(soft) != "a b" {
		t.Errorf("a soft break drew %q on %d lines, want %q on one", bodyText(soft), len(ys), "a b")
	}

	if len(body(longR)) < 2 {
		t.Errorf("a 3000-character word drew as %d op(s)", len(body(longR)))
	}
	assertInBox(t, longR, textLeft, textRight)
	if bodyText(longR) != long {
		t.Error("the split word does not concatenate back to itself")
	}

	assertInBox(t, mixedR, textLeft, textRight)
	want := "short" + strings.Repeat("x", 500) + strings.Repeat("y", 500) + strings.Repeat("z", 500) + "tail" + long
	if got := strings.ReplaceAll(bodyText(mixedR), " ", ""); got != want {
		t.Errorf("mixed split lost text: got %d runes, want %d", utf8.RuneCountInString(got), utf8.RuneCountInString(want))
	}
}

func TestRenderPagesAndFooter(t *testing.T) {
	var b strings.Builder
	for i := range 300 {
		b.WriteString("Paragraph " + strconv.Itoa(i) + " with a few words in it.\n\n")
	}
	r, _, _ := renderRec(t, b.String())
	pages := r.pageNo()
	if pages < 3 {
		t.Fatalf("300 paragraphs gave %d page(s)", pages)
	}
	for p := 1; p <= pages; p++ {
		var footers []drawn
		for _, d := range r.ops {
			if d.page == p && d.y == footerY {
				footers = append(footers, d)
			}
		}
		if len(footers) != 1 {
			t.Fatalf("page %d has %d footers", p, len(footers))
		}
		f := footers[0]
		if f.s != strconv.Itoa(p) {
			t.Errorf("page %d footer reads %q", p, f.s)
		}
		if mid := f.x + r.measure(f.f, f.size, f.s)/2; math.Abs(mid-pageWidth/2) > 1 {
			t.Errorf("page %d footer centred at %.2f, want %.2f", p, mid, pageWidth/2)
		}
	}
	for _, d := range body(r) {
		if d.y < textTop-0.01 || d.y+d.size*lineFactor > textBottom+0.01 {
			t.Errorf("op %q on page %d at y %.2f leaves the text area", d.s, d.page, d.y)
		}
	}
}

func TestRenderTitle(t *testing.T) {
	if _, title, _ := renderRec(t, "# Widget *design*\n\n# Second\n"); title != "Widget design" {
		t.Errorf("title = %q, want %q", title, "Widget design")
	}
	if _, title, _ := renderRec(t, "## x\n"); title != "fallback-title" {
		t.Errorf("title = %q, want the fallback", title)
	}
}

func TestRenderLinksImagesHTML(t *testing.T) {
	r, _, _ := renderRec(t, "[text](http://x)\n\n<https://a.test>\n\n![alt text](p.png)\n\na <b>x</b>\n")
	all := texts(r)
	for _, want := range []string{"text", "https://a.test", "alt text", "<b>", "</b>"} {
		if !strings.Contains(all, want) {
			t.Errorf("drew %q, missing %q", all, want)
		}
	}
	for _, not := range []string{"http://x", "p.png"} {
		if strings.Contains(all, not) {
			t.Errorf("drew %q, which holds the destination %q", all, not)
		}
	}
}

func TestRenderEscapesAndEntities(t *testing.T) {
	r, _, _ := renderRec(t, "a \\*b\\* &amp; c\n\n`\\*&amp;`\n\n\\&amp; &#35; &amp;#35;\n")
	all := texts(r)
	for _, want := range []string{"a *b* & c", "&amp; # &#35;"} {
		if !strings.Contains(all, want) {
			t.Errorf("drew %q, missing %q", all, want)
		}
	}
	if d := findOne(t, r, `\*&amp;`); d.f != faceMono {
		t.Errorf("code span drawn in face %d, want faceMono", d.f)
	}
}

func TestRenderThematicBreak(t *testing.T) {
	r, _, _ := renderRec(t, "a\n\n---\n\nb\n")
	if len(r.lines) != 1 {
		t.Fatalf("want one line op, got %+v", r.lines)
	}
	ln := r.lines[0]
	if ln.y1 != ln.y2 || math.Abs((ln.x2-ln.x1)-(textRight-textLeft)) > 1 {
		t.Errorf("rule %+v is not a horizontal line across the text width", ln)
	}
}

var oddInputs = []string{
	"",
	"\x00\x01",
	"\t",
	"\xff\xfe",
	"a\xe2\x82 b \xf0\x9f c",
	strings.Repeat(">", 200) + " deep",
	strings.Repeat("x", 20000),
	"| a | b |\n|---|---|\n| 1 | 2 |\n",
	"- [ ] todo\n- [x] done\n  1. nested\n",
	"```\ncode\twith tab\n```\n\n    indented\n\n<div>\nhi\n</div>\n",
	"a\r\nb\r\n",
	"#\n\n# \n",
}

func TestRenderOddInputs(t *testing.T) {
	type result struct {
		pdf []byte
		err error
		rec *recorder
	}
	results := make([]result, len(oddInputs))
	within(t, 60*time.Second, func() {
		for i, in := range oddInputs {
			pdf, _, err := Render([]byte(in), "odd")
			results[i] = result{pdf, err, record(in)}
		}
	})
	for i, in := range oddInputs {
		got := results[i]
		if got.err != nil || !bytes.HasPrefix(got.pdf, []byte("%PDF-")) {
			t.Errorf("Render(%q) = (%d bytes, %v), want a PDF and no error", in[:min(len(in), 30)], len(got.pdf), got.err)
		}
		assertBMPOnly(t, got.rec)
	}
}

// TestResolveEveryRune feeds every Unicode scalar value through resolve in
// several styles and checks that every resulting piece is valid UTF-8, holds
// only BMP runes, and is drawn in a face that covers each of them.
func TestResolveEveryRune(t *testing.T) {
	var b strings.Builder
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if utf8.ValidRune(r) {
			b.WriteRune(r)
		}
	}
	all := b.String()
	for _, st := range []style{
		{size: bodySize},
		{bold: true, size: bodySize},
		{italic: true, size: bodySize},
		{mono: true, italic: true, bold: true, size: bodySize},
	} {
		pieces, n := resolve(all, st)
		if n == 0 {
			t.Errorf("style %+v: no placeholders over all of Unicode", st)
		}
		var drawnPlaceholders int
		for _, p := range pieces {
			if !utf8.ValidString(p.text) {
				t.Fatalf("style %+v: piece is not valid UTF-8", st)
			}
			for _, r := range p.text {
				if r > 0xFFFF {
					t.Fatalf("style %+v: piece holds %U", st, r)
				}
				if !p.f.covers(r) {
					t.Fatalf("style %+v: face %d does not cover %U", st, p.f, r)
				}
				if r == Placeholder {
					drawnPlaceholders++
				}
			}
		}
		// The input holds one literal Placeholder, drawn as itself.
		if drawnPlaceholders != n+1 {
			t.Errorf("style %+v: drew %d placeholders for %d counted", st, drawnPlaceholders, n)
		}
	}
}

// TestMeasureMatchesFpdf pins measure to fpdf's own string width, so text
// laid out by measure lands where fpdf draws it.
func TestMeasureMatchesFpdf(t *testing.T) {
	c := newFpdfCanvas()
	c.newPage()
	for _, f := range allFaces {
		var b strings.Builder
		for _, r := range "Hello, World — → … ① ⚠ ✅ ⭐ ☑ ─│ 0123 ijl WM" {
			if f.covers(r) {
				b.WriteRune(r)
			}
		}
		s := b.String()
		c.pdf.SetFont(f.family(), f.style(), 13)
		want := c.pdf.GetStringWidth(s)
		if got := c.measure(f, 13, s); math.Abs(got-want) > 1e-9 {
			t.Errorf("face %d: measure = %.6f, fpdf = %.6f", f, got, want)
		}
	}
}

func FuzzRender(f *testing.F) {
	for _, in := range oddInputs {
		f.Add(in)
	}
	f.Add("# T 🟡\n\npara 🗑 *em 🟢*\n\n`code 🟡`\n\n[link 🟡](http://x)\n")
	f.Add(deterministicDoc)
	f.Fuzz(func(t *testing.T, in string) {
		r := newRecorder()
		title, _ := render([]byte(in), "fuzz", r)
		assertBMPOnly(t, r)
		for _, c := range title {
			if c > 0xFFFF {
				t.Errorf("title holds %U", c)
			}
		}
		if _, _, err := Render([]byte(in), "fuzz"); err != nil {
			t.Errorf("Render: %v", err)
		}
	})
}

// TestRenderHeadingKeepsWithNext checks a heading is never left alone at a
// page bottom: for every amount of text before it, the heading lands on the
// same page as the first line that follows it.
func TestRenderHeadingKeepsWithNext(t *testing.T) {
	for n := 30; n < 80; n++ {
		md := strings.Repeat("filler line\n\n", n) + "## Keepheading\n\nFollowbody\n"
		r := record(md)
		hp, bp := -1, -1
		for _, d := range r.ops {
			switch d.s {
			case "Keepheading":
				hp = d.page
			case "Followbody":
				bp = d.page
			}
		}
		if hp < 0 || bp < 0 {
			t.Fatalf("n=%d: heading page %d, body page %d: text not found", n, hp, bp)
		}
		if hp != bp {
			t.Fatalf("n=%d: heading on page %d, its next line on page %d", n, hp, bp)
		}
	}
}
