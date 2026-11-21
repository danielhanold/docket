package pdfrender

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// horizontals is every recorded line op that runs straight across the page.
func horizontals(r *recorder) []lineOp {
	var out []lineOp
	for _, ln := range r.lines {
		if ln.y1 == ln.y2 && ln.x1 != ln.x2 {
			out = append(out, ln)
		}
	}
	return out
}

// columnEdges is the sorted distinct x of every vertical border on page 1:
// the table's left edge, each column boundary, and its right edge.
func columnEdges(r *recorder) []float64 {
	seen := map[float64]bool{}
	var xs []float64
	for _, v := range verticals(r) {
		if v.page == 1 && !seen[v.x1] {
			seen[v.x1] = true
			xs = append(xs, v.x1)
		}
	}
	sort.Float64s(xs)
	return xs
}

// assertInColumn fails for every op of ops not drawn inside [left, right].
func assertInColumn(t *testing.T, r *recorder, ops []drawn, left, right float64) {
	t.Helper()
	for _, d := range ops {
		if w := r.measure(d.f, d.size, d.s); d.x < left-0.01 || d.x+w > right+0.01 {
			t.Errorf("op %q spans %.2f..%.2f, outside its column %.2f..%.2f", d.s, d.x, d.x+w, left, right)
		}
	}
}

func TestRenderTableHeaderBold(t *testing.T) {
	r, _, _ := renderRec(t, "| Name | Note |\n|---|---|\n| a | b |\n")
	for _, s := range []string{"Name", "Note"} {
		if d := findOne(t, r, s); d.f != faceSansBold {
			t.Errorf("header %q drawn in face %v, want faceSansBold", s, d.f)
		}
	}
	for _, s := range []string{"a", "b"} {
		if d := findOne(t, r, s); d.f != faceSans {
			t.Errorf("cell %q drawn in face %v, want faceSans", s, d.f)
		}
	}
	name, a, note := findOne(t, r, "Name"), findOne(t, r, "a"), findOne(t, r, "Note")
	if a.y <= name.y || a.x != name.x || note.y != name.y || note.x <= name.x {
		t.Errorf("cells not in a grid: Name %+v Note %+v a %+v", name, note, a)
	}
	// Three horizontal borders (top, under the header, bottom) and three
	// vertical edges (left, between the columns, right).
	if hs := horizontals(r); len(hs) < 3 {
		t.Errorf("want at least 3 horizontal borders, got %+v", hs)
	}
	edges := columnEdges(r)
	if len(edges) != 3 {
		t.Fatalf("want 3 distinct vertical border x, got %v", edges)
	}
	if !(edges[0] < name.x && name.x < edges[1] && edges[1] < note.x && note.x < edges[2]) {
		t.Errorf("borders %v do not frame Name %.2f and Note %.2f", edges, name.x, note.x)
	}
	if edges[0] < textLeft-0.01 || edges[2] > textRight+0.01 {
		t.Errorf("table edges %v outside the text box", edges)
	}
}

func TestRenderTableColumnWidths(t *testing.T) {
	r, _, _ := renderRec(t, "| x | a much longer cell text |\n|---|---|\n| 1 | 2 |\n")
	edges := columnEdges(r)
	if len(edges) != 3 {
		t.Fatalf("want 3 vertical border x, got %v", edges)
	}
	if first, second := edges[1]-edges[0], edges[2]-edges[1]; second <= first {
		t.Errorf("column widths %.2f, %.2f: want the longer column wider", first, second)
	}

	// 400 words across four columns: scaled into the text width.
	cell := strings.TrimSpace(strings.Repeat("lorem ipsum dolor sit amet ", 20))
	row := "| " + strings.Join([]string{cell, cell, cell, cell}, " | ") + " |\n"
	var wide *recorder
	within(t, 30*time.Second, func() { wide = record("| h1 | h2 | h3 | h4 |\n|---|---|---|---|\n" + row) })
	assertInBox(t, wide, textLeft, textRight)
	edges = columnEdges(wide)
	if len(edges) != 5 || edges[0] < textLeft-0.01 || edges[4] > textRight+0.01 {
		t.Errorf("wide table borders %v: want 5 inside %.2f..%.2f", edges, textLeft, textRight)
	}

	// A long cell next to a short one wraps inside its own column.
	long := strings.TrimSpace(strings.Repeat("wrapping words go on ", 40))
	r, _, _ = renderRec(t, "| k | v |\n|---|---|\n| key | "+long+" |\n")
	edges = columnEdges(r)
	if len(edges) != 3 {
		t.Fatalf("want 3 vertical border x, got %v", edges)
	}
	var cellOps []drawn
	ys := map[float64]bool{}
	for _, d := range body(r) {
		if strings.Contains(d.s, "wrapping") || strings.Contains(d.s, "words") {
			cellOps = append(cellOps, d)
			ys[d.y] = true
		}
	}
	if len(ys) < 2 {
		t.Errorf("long cell drawn on %d distinct y, want it wrapped", len(ys))
	}
	assertInColumn(t, r, cellOps, edges[1], edges[2])
	if key := findOne(t, r, "key"); key.x < edges[0] || key.x > edges[1] {
		t.Errorf("key at x %.2f outside its column %.2f..%.2f", key.x, edges[0], edges[1])
	}

	// A 300-character token in a narrow column is split at rune boundaries
	// and stays inside the column.
	token := strings.Repeat("0123456789", 30)
	cols := make([]string, 6)
	for i := range cols {
		cols[i] = token
	}
	md := "| a | b | c | d | e | f |\n|---|---|---|---|---|---|\n| " + strings.Join(cols, " | ") + " |\n"
	var narrow *recorder
	within(t, 30*time.Second, func() { narrow = record(md) })
	edges = columnEdges(narrow)
	if len(edges) != 7 {
		t.Fatalf("want 7 vertical border x, got %v", edges)
	}
	var last []drawn
	for _, d := range body(narrow) {
		if w := narrow.measure(d.f, d.size, d.s); d.x >= edges[5] && d.x+w <= edges[6]+1 && strings.ContainsAny(d.s, "0123456789") {
			last = append(last, d)
		}
	}
	var joined strings.Builder
	for _, d := range last {
		joined.WriteString(d.s)
	}
	if len(last) < 2 || joined.String() != token {
		t.Errorf("last column drew %d pieces joining to %q, want the token split", len(last), joined.String())
	}
	assertInColumn(t, narrow, last, edges[5], edges[6])
	assertInBox(t, narrow, textLeft, textRight)
}

func TestRenderTableHeaderRepeats(t *testing.T) {
	var b strings.Builder
	b.WriteString("Intro paragraph.\n\n| Name | Note |\n|---|---|\n")
	for i := range 150 {
		note := fmt.Sprintf("n%03d", i)
		if i%10 == 0 {
			// Some rows span several lines, so a row could split badly.
			note += " " + strings.TrimSpace(strings.Repeat(fmt.Sprintf("t%03d-word ", i), 30))
		}
		fmt.Fprintf(&b, "| r%03d | %s |\n", i, note)
	}
	var r *recorder
	within(t, 30*time.Second, func() { r = record(b.String()) })

	rowPage := map[int]int{} // row index -> page
	rowTop := map[int]float64{}
	names := map[int][]drawn{}
	for _, d := range body(r) {
		if d.s == "Name" {
			names[d.page] = append(names[d.page], d)
		}
		var i int
		if _, err := fmt.Sscanf(d.s, "r%03d", &i); err == nil && len(d.s) == 4 {
			rowPage[i] = d.page
			rowTop[i] = d.y
		}
	}
	if len(rowPage) != 150 {
		t.Fatalf("found %d of 150 row labels", len(rowPage))
	}
	pages := map[int]bool{}
	for _, p := range rowPage {
		pages[p] = true
	}
	if len(pages) < 2 {
		t.Fatalf("150-row table spans %d pages, want at least 2", len(pages))
	}
	for p := range pages {
		if len(names[p]) != 1 {
			t.Errorf("page %d holds rows and draws the header %d times, want once", p, len(names[p]))
			continue
		}
		for i, rp := range rowPage {
			if rp == p && rowTop[i] <= names[p][0].y {
				t.Errorf("row %d at y %.2f is above the header (y %.2f) on page %d", i, rowTop[i], names[p][0].y, p)
			}
		}
	}

	// No row splits across a page, and no row runs past the bottom margin.
	// Every op of row i holds one of its own tokens r<i>, n<i>, or t<i>.
	for i := range 150 {
		tokens := []string{fmt.Sprintf("r%03d", i), fmt.Sprintf("n%03d", i), fmt.Sprintf("t%03d", i)}
		page, count := 0, 0
		for _, d := range body(r) {
			if !strings.Contains(d.s, tokens[0]) && !strings.Contains(d.s, tokens[1]) && !strings.Contains(d.s, tokens[2]) {
				continue
			}
			count++
			if page == 0 {
				page = d.page
			}
			if d.page != page {
				t.Errorf("row %d split across pages %d and %d", i, page, d.page)
			}
			if d.y+d.size*lineFactor > textBottom+0.01 {
				t.Errorf("row %d text at y %.2f runs past the bottom margin", i, d.y)
			}
		}
		if i%10 == 0 && count < 3 {
			t.Errorf("tall row %d drew %d ops, want it wrapped over several lines", i, count)
		}
	}
	for _, ln := range r.lines {
		if max(ln.y1, ln.y2) > textBottom+0.01 {
			t.Errorf("border %+v runs past the bottom margin", ln)
		}
	}
}

// hasTint reports whether an inline-code tint on d's page sits behind d.
func hasTint(r *recorder, d drawn) bool {
	w := r.measure(d.f, d.size, d.s)
	for _, rc := range r.rects {
		if rc.page == d.page && rc.gray == tintGray && rc.x <= d.x && rc.x+rc.w >= d.x+w-0.01 &&
			rc.y < d.y+d.size && rc.y+rc.h > d.y {
			return true
		}
	}
	return false
}

func TestRenderTableInlineAndNonBMP(t *testing.T) {
	r, _, n := renderRec(t, "| x | y | z |\n|---|---|---|\n| **b** | `c` | 🟡 |\n")
	if d := findOne(t, r, "b"); d.f != faceSansBold {
		t.Errorf("bold cell drawn in face %v, want faceSansBold", d.f)
	}
	c := findOne(t, r, "c")
	if c.f != faceMono {
		t.Errorf("code cell drawn in face %v, want faceMono", c.f)
	}
	if !hasTint(r, c) {
		t.Error("code span in a cell has no tint behind it")
	}
	if n != 1 {
		t.Errorf("placeholders = %d, want 1", n)
	}
	findOne(t, r, string(Placeholder))
	assertBMPOnly(t, r)
}

func TestRenderTableNested(t *testing.T) {
	// A table opening a list item draws the item's bullet on its header line,
	// and sits inside the item's indent.
	r, _, _ := renderRec(t, "- | Name | Note |\n  |---|---|\n  | a | b |\n")
	bullet, name := findOne(t, r, "•"), findOne(t, r, "Name")
	if bullet.y != name.y || bullet.x >= name.x {
		t.Errorf("bullet %+v not on the header line before %+v", bullet, name)
	}
	if edges := columnEdges(r); len(edges) == 0 || edges[0] <= bullet.x {
		t.Errorf("table edges %v not indented past the bullet at %.2f", edges, bullet.x)
	}

	// A table in a blockquote keeps the quote's rule alongside every row.
	r, _, _ = renderRec(t, "> | Name | Note |\n> |---|---|\n> | a | b |\n")
	a := findOne(t, r, "a")
	var rule *lineOp
	for _, v := range verticals(r) {
		if v.gray == quoteGray {
			rule = &v
		}
	}
	if rule == nil {
		t.Fatalf("no quote rule among %+v", r.lines)
	}
	if rule.y1 > findOne(t, r, "Name").y || rule.y2 < a.y+a.size*lineFactor {
		t.Errorf("quote rule %+v does not run alongside the table", *rule)
	}
}

// A row taller than a page is split across pages: every line of its cell is
// drawn, none past the bottom margin, and each page it continues on starts
// with a fresh copy of the header.
func TestRenderTableRowTallerThanPage(t *testing.T) {
	const n = 1500
	words := make([]string, n)
	for i := range words {
		words[i] = fmt.Sprintf("w%04d", i)
	}
	md := "Intro.\n\n| Name | Note |\n|---|---|\n| key | " + strings.Join(words, " ") + " |\n| after | tail |\n"
	var r *recorder
	within(t, 30*time.Second, func() { r = record(md) })

	seen := map[string]int{}
	cellPages := map[int]bool{}
	for _, d := range body(r) {
		for _, f := range strings.Fields(d.s) {
			if len(f) == 5 && f[0] == 'w' {
				seen[f]++
				cellPages[d.page] = true
			}
		}
		if d.y+d.size*lineFactor > textBottom+0.01 {
			t.Errorf("text %q on page %d at y %.2f runs past the bottom margin", d.s, d.page, d.y)
		}
	}
	for _, w := range words {
		if seen[w] != 1 {
			t.Errorf("word %s drawn %d times, want once", w, seen[w])
		}
	}
	if len(cellPages) < 2 {
		t.Fatalf("tall cell drawn on %d pages, want it split over several", len(cellPages))
	}
	headers := map[int][]drawn{}
	for _, d := range find(r, "Name") {
		headers[d.page] = append(headers[d.page], d)
	}
	for p := range cellPages {
		if len(headers[p]) != 1 {
			t.Errorf("page %d continues the tall row and draws the header %d times, want once", p, len(headers[p]))
		}
	}
	if len(find(r, "after")) != 1 || len(find(r, "tail")) != 1 {
		t.Errorf("row after the tall one not drawn once")
	}
	for _, ln := range r.lines {
		if max(ln.y1, ln.y2) > textBottom+0.01 {
			t.Errorf("border %+v runs past the bottom margin", ln)
		}
	}
}
