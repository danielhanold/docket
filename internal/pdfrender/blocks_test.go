package pdfrender

import (
	"strings"
	"testing"
	"time"
)

// verticals is every recorded line op that runs straight down the page.
func verticals(r *recorder) []lineOp {
	var out []lineOp
	for _, ln := range r.lines {
		if ln.x1 == ln.x2 && ln.y1 != ln.y2 {
			out = append(out, ln)
		}
	}
	return out
}

func TestRenderLists(t *testing.T) {
	r, _, _ := renderRec(t, "- a\n- b\n  - c\n")
	a, c := findOne(t, r, "a"), findOne(t, r, "c")
	if c.x <= a.x {
		t.Errorf("nested item c.x = %.2f, want right of a.x = %.2f", c.x, a.x)
	}
	bullets := find(r, "•")
	if len(bullets) != 3 {
		t.Fatalf("want 3 bullets, got %+v", bullets)
	}
	byX := map[float64]int{}
	minX, maxX := bullets[0].x, bullets[0].x
	for _, b := range bullets {
		byX[b.x]++
		minX, maxX = min(minX, b.x), max(maxX, b.x)
	}
	if len(byX) != 2 || byX[minX] != 2 || byX[maxX] != 1 || minX < textLeft-0.01 {
		t.Errorf("bullets %+v: want two at depth 0 and one further right at depth 1", bullets)
	}
	for _, b := range bullets {
		if b.f != faceSans {
			t.Errorf("bullet drawn in face %v, want faceSans", b.f)
		}
	}
	// Each bullet shares its item's line.
	if bl := bullets[0]; bl.y != a.y {
		t.Errorf("first bullet y = %.2f, item a y = %.2f", bl.y, a.y)
	}

	r, _, _ = renderRec(t, "3. x\n4. y\n")
	x, y := findOne(t, r, "x"), findOne(t, r, "y")
	n3, n4 := findOne(t, r, "3."), findOne(t, r, "4.")
	if n3.y != x.y || n4.y != y.y || n3.x >= x.x || n4.x >= y.x {
		t.Errorf("numbers %+v %+v not before their items %+v %+v", n3, n4, x, y)
	}

	r, _, _ = renderRec(t, "- one\n\n- two\n\n- three\n")
	for _, s := range []string{"one", "two", "three"} {
		findOne(t, r, s)
	}
	if got := len(find(r, "•")); got != 3 {
		t.Errorf("loose list drew %d bullets, want 3", got)
	}

	// An empty item still draws its marker.
	r, _, _ = renderRec(t, "-\n- b\n")
	if got := len(find(r, "•")); got != 2 {
		t.Errorf("list with an empty item drew %d bullets, want 2", got)
	}
}

func TestRenderTaskList(t *testing.T) {
	r, _, _ := renderRec(t, "- [ ] todo\n- [x] done\n")
	todo, done := findOne(t, r, "todo"), findOne(t, r, "done")
	open, checked := findOne(t, r, "☐"), findOne(t, r, "☑")
	if open.y != todo.y || open.x >= todo.x {
		t.Errorf("☐ %+v is not before todo %+v", open, todo)
	}
	if checked.y != done.y || checked.x >= done.x {
		t.Errorf("☑ %+v is not before done %+v", checked, done)
	}
	if open.f != faceSans || checked.f != faceSans {
		t.Errorf("boxes drawn in faces %v/%v, want faceSans", open.f, checked.f)
	}
	all := texts(r)
	if strings.Contains(all, "[ ]") || strings.Contains(all, "[x]") {
		t.Errorf("drawn text %q holds a literal task marker", all)
	}
	if len(find(r, "•")) != 0 {
		t.Error("task items also drew a bullet")
	}
}

// codeOps is every body op drawn in a mono face.
func codeOps(r *recorder) []drawn {
	var out []drawn
	for _, d := range body(r) {
		if d.f == faceMono || d.f == faceMonoBold {
			out = append(out, d)
		}
	}
	return out
}

// hasRectBehind reports whether a fillRect on d's page covers d's line box.
func hasRectBehind(r *recorder, d drawn) bool {
	w := r.measure(d.f, d.size, d.s)
	for _, rc := range r.rects {
		if rc.page == d.page && rc.x <= d.x && rc.x+rc.w >= d.x+w-0.01 &&
			rc.y <= d.y+0.01 && rc.y+rc.h >= d.y+d.size-0.01 {
			return true
		}
	}
	return false
}

func TestRenderCodeBlocks(t *testing.T) {
	r, _, _ := renderRec(t, "```\n  lead\tTab\nsecond\n```\n")
	ops := codeOps(r)
	if len(ops) != 2 {
		t.Fatalf("want 2 mono ops, got %+v", ops)
	}
	if ops[0].s != "  lead  Tab" {
		t.Errorf("first code line drawn as %q, want %q (tab to column 8)", ops[0].s, "  lead  Tab")
	}
	if ops[0].size >= bodySize {
		t.Errorf("code size %.2f, want smaller than the body size", ops[0].size)
	}
	for _, d := range ops {
		if !hasRectBehind(r, d) {
			t.Errorf("code line %q has no background", d.s)
		}
	}

	long := strings.Repeat("0123456789", 40)
	var rr *recorder
	within(t, 30*time.Second, func() { rr = record("```\n" + long + "\n```\n") })
	ops = codeOps(rr)
	if len(ops) < 2 {
		t.Fatalf("400-character line drew %d ops, want it wrapped", len(ops))
	}
	var joined strings.Builder
	for _, d := range ops {
		joined.WriteString(d.s)
		if !hasRectBehind(rr, d) {
			t.Errorf("wrapped code line %q has no background", d.s)
		}
	}
	if joined.String() != long {
		t.Errorf("wrapped pieces join to %q, want the original line", joined.String())
	}
	assertInBox(t, rr, textLeft, textRight)

	r, _, _ = renderRec(t, "para\n\n    indented\n")
	if d := findOne(t, r, "indented"); d.f != faceMono {
		t.Errorf("indented code drawn in face %v, want faceMono", d.f)
	}

	var lines strings.Builder
	lines.WriteString("```\n")
	for i := range 200 {
		lines.WriteString("line ")
		lines.WriteString(strings.Repeat("·", i%7))
		lines.WriteString("\n")
	}
	lines.WriteString("```\n")
	r, _, _ = renderRec(t, lines.String())
	ops = codeOps(r)
	if len(ops) != 200 {
		t.Fatalf("200-line block drew %d mono ops", len(ops))
	}
	pages := map[int]bool{}
	for _, d := range ops {
		pages[d.page] = true
		if !hasRectBehind(r, d) {
			t.Errorf("code line on page %d has no background", d.page)
		}
	}
	if len(pages) < 2 {
		t.Errorf("200-line block spans %d pages, want at least 2", len(pages))
	}
}

func TestRenderBlockquote(t *testing.T) {
	r, _, _ := renderRec(t, "> a\n>\n> > b\n")
	a, b := findOne(t, r, "a"), findOne(t, r, "b")
	if !(b.x > a.x && a.x > textLeft) {
		t.Errorf("b.x %.2f, a.x %.2f, body left %.2f: want b > a > left", b.x, a.x, textLeft)
	}
	vs := verticals(r)
	if len(vs) != 2 {
		t.Fatalf("want two vertical rules, got %+v", vs)
	}
	if vs[0].x1 == vs[1].x1 {
		t.Errorf("both rules at x %.2f, want one per level", vs[0].x1)
	}
	for _, v := range vs {
		if v.x1 >= b.x || v.x1 < textLeft {
			t.Errorf("rule at x %.2f is not at the left of the quote", v.x1)
		}
	}

	// A deep quote keeps a usable box.
	r = record(strings.Repeat(">", 200) + " deep words here\n")
	d := findOne(t, r, "deep words here")
	if w := r.measure(d.f, d.size, d.s); d.x+w > textRight+0.01 {
		t.Errorf("deep quote text spans to %.2f, past the right margin", d.x+w)
	}
}

func TestRenderHTMLBlock(t *testing.T) {
	r, _, _ := renderRec(t, "<div>\nhi\n</div>\n")
	for _, s := range []string{"<div>", "hi", "</div>"} {
		if d := findOne(t, r, s); d.f != faceSans {
			t.Errorf("%q drawn in face %v, want the body face", s, d.f)
		}
	}
}
