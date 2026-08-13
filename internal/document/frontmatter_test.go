package document

import (
	"errors"
	"testing"
)

func TestParseCopiesSourceAndDoesNotAlias(t *testing.T) {
	in := []byte("---\nid: 3\n---\nbody\n")
	d, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	in[0] = 'X' // mutate caller buffer after Parse
	if got := d.Source(); string(got) != "---\nid: 3\n---\nbody\n" {
		t.Fatalf("Document captured caller mutations: %q", got)
	}
	out := d.Source()
	out[0] = 'Y'
	if d.Source()[0] == 'Y' {
		t.Fatal("Source() returned an aliasing slice")
	}
}

func TestFrontmatterOnlyWhenFirstLineIsFence(t *testing.T) {
	d, err := Parse([]byte("intro\n---\nnot: frontmatter\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.HasFrontmatter() {
		t.Fatal("a body horizontal rule must not become frontmatter")
	}
}

func TestCRLFFenceDetectedAndLineEndingRecorded(t *testing.T) {
	d, err := Parse([]byte("---\r\nid: 1\r\n---\r\nbody\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !d.HasFrontmatter() || d.LineEnding() != "\r\n" {
		t.Fatalf("HasFrontmatter=%v LineEnding=%q", d.HasFrontmatter(), d.LineEnding())
	}
}

func TestLineEndingDefaultsToLF(t *testing.T) {
	d, err := Parse([]byte("no terminator"))
	if err != nil {
		t.Fatal(err)
	}
	if d.LineEnding() != "\n" {
		t.Fatalf("LineEnding = %q, want the LF default", d.LineEnding())
	}
}

func TestUnclosedFrontmatterIsTyped(t *testing.T) {
	_, err := Parse([]byte("---\nid: 1\nbody without closer\n"))
	if !IsKind(err, KindUnclosedFrontmatter) {
		t.Fatalf("want unclosed-frontmatter, got %v", err)
	}
}

func TestInvalidUTF8Rejected(t *testing.T) {
	_, err := Parse([]byte{'-', '-', '-', '\n', 0xff, 0xfe, '\n', '-', '-', '-', '\n'})
	if !IsKind(err, KindInvalidUTF8) {
		t.Fatalf("want invalid-utf8, got %v", err)
	}
	var de *Error
	if !errors.As(err, &de) {
		t.Fatalf("error is not a *Error: %v", err)
	}
	if de.Offset != 4 {
		t.Fatalf("Offset = %d, want the first invalid byte at 4", de.Offset)
	}
}

func TestNoFrontmatterDocumentIsValid(t *testing.T) {
	d, err := Parse([]byte("# Just a spec\n\nprose\n"))
	if err != nil {
		t.Fatal(err)
	}
	if d.HasFrontmatter() {
		t.Fatal("plain Markdown must parse with no frontmatter")
	}
	if err := d.DecodeFrontmatter(&struct{}{}); !IsKind(err, KindMissingFrontmatter) {
		t.Fatalf("DecodeFrontmatter without frontmatter: want missing-frontmatter, got %v", err)
	}
}

func TestEmptyDocumentParses(t *testing.T) {
	d, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.HasFrontmatter() || len(d.Source()) != 0 {
		t.Fatal("empty input must parse to an empty, frontmatterless document")
	}
}

func TestScanLinesSpansCoverEveryByteExactlyOnce(t *testing.T) {
	src := []byte("a\r\nb\n\nc")
	lines := scanLines(src)
	want := []struct {
		text   string
		ending string
	}{
		{"a", "\r\n"},
		{"b", "\n"},
		{"", "\n"},
		{"c", ""},
	}
	if len(lines) != len(want) {
		t.Fatalf("scanLines returned %d lines, want %d", len(lines), len(want))
	}
	next := 0
	for i, ln := range lines {
		if ln.span.Start != next {
			t.Fatalf("line %d starts at %d, want %d — spans must tile the source", i, ln.span.Start, next)
		}
		next = ln.span.End
		if got := string(src[ln.text.Start:ln.text.End]); got != want[i].text {
			t.Errorf("line %d text = %q, want %q", i, got, want[i].text)
		}
		if ln.ending != want[i].ending {
			t.Errorf("line %d ending = %q, want %q", i, ln.ending, want[i].ending)
		}
		if !lineIsExactly(src, ln, want[i].text) {
			t.Errorf("lineIsExactly(%q) = false", want[i].text)
		}
	}
	if next != len(src) {
		t.Fatalf("spans end at %d, want %d", next, len(src))
	}
}

func TestFrontmatterFenceSpansIncludeTerminators(t *testing.T) {
	src := "---\nid: 1\n---\nbody\n"
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !d.HasFrontmatter() {
		t.Fatal("want frontmatter")
	}
	if got := string(d.Source()[d.fmOpen.Start:d.fmOpen.End]); got != "---\n" {
		t.Errorf("fmOpen = %q", got)
	}
	if got := string(d.Source()[d.fmClose.Start:d.fmClose.End]); got != "---\n" {
		t.Errorf("fmClose = %q", got)
	}
	if got := string(d.Source()[d.fmOpen.End:d.fmClose.Start]); got != "id: 1\n" {
		t.Errorf("interior = %q", got)
	}
}

func TestUnterminatedClosingFenceStillCloses(t *testing.T) {
	d, err := Parse([]byte("---\nid: 1\n---"))
	if err != nil {
		t.Fatal(err)
	}
	if !d.HasFrontmatter() {
		t.Fatal("a final unterminated --- still closes the frontmatter")
	}
}
