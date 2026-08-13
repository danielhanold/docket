package document

import "testing"

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
