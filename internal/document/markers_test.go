package document

import (
	"errors"
	"strings"
	"testing"
)

const artifactsBlock = "<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| a |\n<!-- docket:artifacts:end -->\n"

func TestBlockDiscoveryWithAnnotation(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n\n"+artifactsBlock)
	b, ok := d.Block("artifacts")
	if !ok {
		t.Fatal("artifacts block not found")
	}
	if b.Annotation != "generated — do not hand-edit" {
		t.Fatalf("annotation = %q", b.Annotation)
	}
	if got := string(d.Source()[b.Interior.Start:b.Interior.End]); got != "| a |\n" {
		t.Fatalf("interior = %q", got)
	}
	src := d.Source()
	if got := string(src[b.Start.Start:b.Start.End]); got != "<!-- docket:artifacts:start (generated — do not hand-edit) -->\n" {
		t.Fatalf("start marker span = %q", got)
	}
	if got := string(src[b.End.Start:b.End.End]); got != "<!-- docket:artifacts:end -->\n" {
		t.Fatalf("end marker span = %q", got)
	}
}

func TestStartMarkerWithoutAnnotationValid(t *testing.T) {
	d := mustParse(t, "<!-- docket:backlink:start -->\nx\n<!-- docket:backlink:end -->\n")
	b, ok := d.Block("backlink")
	if !ok {
		t.Fatal("annotation-free start marker is valid")
	}
	if b.Annotation != "" {
		t.Fatalf("annotation = %q, want empty", b.Annotation)
	}
}

func TestMarkerInsideCodeFenceIsContent(t *testing.T) {
	src := "example:\n\n```text\n<!-- docket:example:start -->\n```\n"
	d := mustParse(t, src)
	if len(d.Blocks()) != 0 {
		t.Fatal("marker-shaped text inside a fenced code block is authored content")
	}
}

func TestTildeFenceAlsoShieldsMarkers(t *testing.T) {
	d := mustParse(t, "~~~\n<!-- docket:x:start -->\n~~~\n")
	if len(d.Blocks()) != 0 {
		t.Fatal("tilde fences shield markers too")
	}
}

func TestFenceOfOneCharacterDoesNotCloseALongerFence(t *testing.T) {
	// A closing run must be at least as long as the opener's, and of the same
	// character: the "```" inside the "````" block stays content.
	d := mustParse(t, "````\n```\n<!-- docket:x:start -->\n````\n")
	if len(d.Blocks()) != 0 {
		t.Fatal("a shorter fence run must not close a longer one")
	}
}

func TestMarkersResumeAfterAClosedFence(t *testing.T) {
	d := mustParse(t, "```\n<!-- docket:shielded:start -->\n```\n<!-- docket:real:start -->\nx\n<!-- docket:real:end -->\n")
	blocks := d.Blocks()
	if len(blocks) != 1 || blocks[0].Name != "real" {
		t.Fatalf("blocks = %+v, want just the post-fence pair", blocks)
	}
}

func TestDanglingStartRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:start -->\nno end\n"))
	if !IsKind(err, KindMarkerImbalance) {
		t.Fatalf("got %v", err)
	}
}

func TestEndBeforeStartRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:end -->\n<!-- docket:a:start -->\n"))
	if !IsKind(err, KindMarkerImbalance) {
		t.Fatalf("got %v", err)
	}
}

func TestDuplicatePairRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:start -->\n<!-- docket:a:end -->\n<!-- docket:a:start -->\n<!-- docket:a:end -->\n"))
	if !IsKind(err, KindMarkerImbalance) {
		t.Fatalf("got %v", err)
	}
}

func TestNestedMarkersRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:start -->\n<!-- docket:b:start -->\n<!-- docket:b:end -->\n<!-- docket:a:end -->\n"))
	if !IsKind(err, KindMarkerImbalance) {
		t.Fatalf("got %v", err)
	}
}

func TestMismatchedEndNameRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:start -->\n<!-- docket:b:end -->\n"))
	if !IsKind(err, KindMarkerImbalance) {
		t.Fatalf("got %v", err)
	}
}

func TestMalformedMarkerShapedLineRejected(t *testing.T) {
	// docket-marker prefix, but bad name (uppercase) — malformed, not prose.
	_, err := Parse([]byte("<!-- docket:BadName:start -->\n"))
	if !IsKind(err, KindMalformedMarker) {
		t.Fatalf("got %v", err)
	}
}

func TestEndMarkerWithAnnotationRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:start -->\n<!-- docket:a:end (nope) -->\n"))
	if !IsKind(err, KindMalformedMarker) {
		t.Fatalf("got %v", err)
	}
}

func TestIndentedMarkerIsProse(t *testing.T) {
	// The grammar is column-zero exact: an indented marker-shaped line is not a
	// marker at all, so it is neither a block nor a malformed-marker error.
	_, err := Parse([]byte("  <!-- docket:a:start -->\n"))
	if err != nil {
		t.Fatalf("an indented marker-shaped line is prose, not a marker: %v", err)
	}
}

func TestOrdinaryHTMLCommentIsProse(t *testing.T) {
	d := mustParse(t, "<!-- just a comment -->\n")
	if len(d.Blocks()) != 0 {
		t.Fatal("plain comments are not markers")
	}
}

func TestMarkersInsideFrontmatterNotScanned(t *testing.T) {
	d := mustParse(t, "---\ntitle: 'has <!-- docket:x:start --> inside'\n---\n")
	if len(d.Blocks()) != 0 {
		t.Fatal("frontmatter bytes are not marker territory")
	}
}

func TestMarkerLineWithoutTerminatorIsStillAMarker(t *testing.T) {
	// A final unterminated end-marker line closes its block; the End span then
	// simply runs to EOF.
	d := mustParse(t, "<!-- docket:a:start -->\nbody\n<!-- docket:a:end -->")
	b, ok := d.Block("a")
	if !ok {
		t.Fatal("unterminated end marker must still close the block")
	}
	if b.End.End != len(d.Source()) {
		t.Fatalf("end span = %+v, want it to run to EOF (%d)", b.End, len(d.Source()))
	}
}

func TestBlocksReturnsFreshSliceInSourceOrder(t *testing.T) {
	d := mustParse(t, "<!-- docket:a:start -->\n<!-- docket:a:end -->\n<!-- docket:b:start -->\n<!-- docket:b:end -->\n")
	blocks := d.Blocks()
	if len(blocks) != 2 || blocks[0].Name != "a" || blocks[1].Name != "b" {
		t.Fatalf("blocks = %+v, want a then b", blocks)
	}
	blocks[0].Name = "clobbered"
	if again := d.Blocks(); again[0].Name != "a" {
		t.Fatal("Blocks() returned a slice aliasing the document's index")
	}
	if _, ok := d.Block("nope"); ok {
		t.Fatal("Block reported an absent name")
	}
}

func TestCRLFMarkerLinesDiscovered(t *testing.T) {
	d := mustParse(t, "<!-- docket:a:start -->\r\nx\r\n<!-- docket:a:end -->\r\n")
	b, ok := d.Block("a")
	if !ok {
		t.Fatal("CRLF marker lines must be discovered")
	}
	if got := string(d.Source()[b.Interior.Start:b.Interior.End]); got != "x\r\n" {
		t.Fatalf("interior = %q", got)
	}
}

func TestNeutralMarkerBlockRoundTrip(t *testing.T) {
	src := []byte("user line\n<!-- dckt:private-instructions:start (managed — do not hand-edit) -->\nbody\n<!-- dckt:private-instructions:end -->\n<!-- docket:dispatch:start -->\nd\n<!-- docket:dispatch:end -->\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := doc.Block("dckt:private-instructions")
	if !ok || string(src[b.Interior.Start:b.Interior.End]) != "body\n" || b.Annotation != "managed — do not hand-edit" {
		t.Fatalf("neutral block not located: %+v ok=%v", b, ok)
	}
	if _, ok := doc.Block("private-instructions"); ok {
		t.Fatal("a bare name must not find a dckt: block")
	}
	if _, ok := doc.Block("dispatch"); !ok {
		t.Fatal("the docket: block must keep its bare name")
	}
	var p PatchSet
	p.RemoveBlock("dckt:private-instructions")
	out, err := doc.Apply(p)
	if err != nil || string(out) != "user line\n<!-- docket:dispatch:start -->\nd\n<!-- docket:dispatch:end -->\n" {
		t.Fatalf("remove: %q %v", out, err)
	}
}

func TestNeutralMarkerInsertRendersDcktPrefix(t *testing.T) {
	doc := mustParse(t, "keep\n")
	var p PatchSet
	p.InsertBlock("dckt:private-instructions", "managed — do not hand-edit", "x", AtDocumentStart)
	out, err := doc.Apply(p)
	if err != nil {
		t.Fatal(err)
	}
	want := "<!-- dckt:private-instructions:start (managed — do not hand-edit) -->\nx\n<!-- dckt:private-instructions:end -->\nkeep\n"
	if string(out) != want {
		t.Fatalf("insert = %q, want %q", out, want)
	}
	if strings.Contains(string(out), "docket") {
		t.Fatalf("a dckt: block must not spell docket: %q", out)
	}
	again := mustParse(t, string(out))
	if _, ok := again.Block("dckt:private-instructions"); !ok {
		t.Fatal("the inserted dckt: block does not re-parse under its qualified name")
	}
	var r PatchSet
	r.ReplaceBlock("dckt:private-instructions", "y\n")
	out2, err := again.Apply(r)
	if err != nil || !strings.Contains(string(out2), "-->\ny\n<!-- dckt:private-instructions:end -->") {
		t.Fatalf("replace: %q %v", out2, err)
	}
}

func TestMalformedNeutralMarkerRefuses(t *testing.T) {
	_, err := Parse([]byte("<!-- dckt:Bad Name:start -->\nx\n"))
	var de *Error
	if !errors.As(err, &de) || de.Kind != KindMalformedMarker {
		t.Fatalf("err = %v, want malformed marker", err)
	}
}

func TestSameNameInBothNamespacesIsTwoBlocks(t *testing.T) {
	doc := mustParse(t, "<!-- docket:x:start -->\na\n<!-- docket:x:end -->\n<!-- dckt:x:start -->\nb\n<!-- dckt:x:end -->\n")
	src := doc.Source()
	bare, ok := doc.Block("x")
	if !ok || string(src[bare.Interior.Start:bare.Interior.End]) != "a\n" {
		t.Fatalf("docket:x not found: %+v ok=%v", bare, ok)
	}
	neutral, ok := doc.Block("dckt:x")
	if !ok || string(src[neutral.Interior.Start:neutral.Interior.End]) != "b\n" {
		t.Fatalf("dckt:x not found: %+v ok=%v", neutral, ok)
	}
}

func TestNeutralMarkerMismatchedNamespaceIsImbalance(t *testing.T) {
	_, err := Parse([]byte("<!-- dckt:x:start -->\na\n<!-- docket:x:end -->\n"))
	var de *Error
	if !errors.As(err, &de) || de.Kind != KindMarkerImbalance || de.Name != "x" {
		t.Fatalf("err = %v, want imbalance naming the docket: end", err)
	}
}

func TestMarkerSpelling(t *testing.T) {
	for name, want := range map[string]string{
		"dispatch":                  "docket:dispatch",
		"dckt:private-instructions": "dckt:private-instructions",
	} {
		if got := MarkerSpelling(name); got != want {
			t.Fatalf("MarkerSpelling(%q) = %q, want %q", name, got, want)
		}
	}
}
