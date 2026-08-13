package document

import (
	"errors"
	"testing"

	"go.yaml.in/yaml/v3"
)

// yamlNode is a test-local alias keeping the helper signature readable; the
// package never exposes a yaml node across its boundary.
type yamlNode = yaml.Node

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

// mustParse parses src, failing the test on any error.
func mustParse(t *testing.T, src string) Document {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// fieldValueText returns the exact source bytes a field's value span covers.
func fieldValueText(d Document, name string) string {
	f, ok := d.Field(name)
	if !ok {
		return "<absent>"
	}
	return string(d.Source()[f.Value.Start:f.Value.End])
}

func TestFieldSpansCoverValueTokenOnly(t *testing.T) {
	d := mustParse(t, "---\nid: 306\npriority: critical   # keep\nadrs: []\nspec:\n---\n")
	cases := []struct {
		name, want string
		shape      FieldShape
	}{
		{"id", "306", ShapeInline},
		{"priority", "critical", ShapeInline},
		{"adrs", "[]", ShapeFlowSeq},
		{"spec", "", ShapeEmpty},
	}
	for _, c := range cases {
		f, ok := d.Field(c.name)
		if !ok {
			t.Fatalf("field %s not indexed", c.name)
		}
		if got := fieldValueText(d, c.name); got != c.want {
			t.Errorf("%s value = %q, want %q", c.name, got, c.want)
		}
		if f.Shape != c.shape {
			t.Errorf("%s shape = %v, want %v", c.name, f.Shape, c.shape)
		}
	}
}

func TestFieldEntrySpanCoversWholePhysicalLine(t *testing.T) {
	src := "---\nid: 306\npriority: critical   # keep\n---\n"
	d := mustParse(t, src)
	f, ok := d.Field("priority")
	if !ok {
		t.Fatal("priority not indexed")
	}
	if got := string(d.Source()[f.Entry.Start:f.Entry.End]); got != "priority: critical   # keep\n" {
		t.Fatalf("entry = %q — must cover the whole line including its terminator", got)
	}
}

func TestFieldsAreReturnedInSourceOrder(t *testing.T) {
	d := mustParse(t, "---\nstatus: proposed\nid: 306\nslug: x\n---\n")
	var names []string
	for _, f := range d.Fields() {
		names = append(names, f.Name)
	}
	want := []string{"status", "id", "slug"}
	if len(names) != len(want) {
		t.Fatalf("Fields() = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("Fields() = %v, want %v", names, want)
		}
	}
}

func TestFieldsReturnsAFreshSlice(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	got := d.Fields()
	got[0].Name = "clobbered"
	if d.Fields()[0].Name != "id" {
		t.Fatal("Fields() handed out the document's own backing array")
	}
}

func TestInlineCommentExcludedFromValueSpan(t *testing.T) {
	d := mustParse(t, "---\nstatus: proposed # not part of the value\n---\n")
	if got := fieldValueText(d, "status"); got != "proposed" {
		t.Fatalf("value = %q — inline comment must stay outside the span", got)
	}
}

func TestHashInsideQuotesIsNotAComment(t *testing.T) {
	d := mustParse(t, "---\ntitle: 'a # not a comment'\n---\n")
	if got := fieldValueText(d, "title"); got != "'a # not a comment'" {
		t.Fatalf("value = %q", got)
	}
}

func TestHashInsideAQuotedFlowElementIsNotAComment(t *testing.T) {
	d := mustParse(t, "---\ntags: ['a # b', 'c']\n---\n")
	if got := fieldValueText(d, "tags"); got != "['a # b', 'c']" {
		t.Fatalf("value = %q", got)
	}
}

func TestApostropheInPlainScalarDoesNotOpenAQuote(t *testing.T) {
	d := mustParse(t, "---\ntitle: don't stop # trailing\n---\n")
	if got := fieldValueText(d, "title"); got != "don't stop" {
		t.Fatalf("value = %q — a mid-token apostrophe is not a quote opener", got)
	}
}

func TestUnknownKeysIndexedLikeKnownKeys(t *testing.T) {
	d := mustParse(t, "---\nfuture_key: kept\n---\n")
	if _, ok := d.Field("future_key"); !ok {
		t.Fatal("unknown Docket-shaped keys must be indexed")
	}
}

func TestNonDocketKeysAreNotPatchTargets(t *testing.T) {
	d := mustParse(t, "---\n\"quoted\": v\nUpper: v\n? complex\n: v\n---\n")
	for _, name := range []string{"quoted", "Upper", "complex"} {
		if _, ok := d.Field(name); ok {
			t.Errorf("%q must not be a patch target", name)
		}
	}
}

func TestCommentAndBlankLinesAreNotFields(t *testing.T) {
	d := mustParse(t, "---\n# a comment\n\nid: 1\n\n# trailing note\n---\n")
	if len(d.Fields()) != 1 {
		t.Fatalf("Fields() = %+v, want just id", d.Fields())
	}
	f, _ := d.Field("id")
	if got := string(d.Source()[f.Entry.Start:f.Entry.End]); got != "id: 1\n" {
		t.Fatalf("entry = %q — trailing blank and comment lines are not part of the entry", got)
	}
}

func TestBlockShapesIndexedAsUnsupported(t *testing.T) {
	d := mustParse(t, "---\nnotes: |\n  line one\n  line two\nitems:\n  - a\n  - b\n---\n")
	for _, name := range []string{"notes", "items"} {
		f, ok := d.Field(name)
		if !ok {
			t.Fatalf("%s must be indexed", name)
		}
		if f.Shape != ShapeUnsupported {
			t.Errorf("%s shape = %v, want ShapeUnsupported", name, f.Shape)
		}
	}
}

func TestBlockEntrySpanCoversItsContinuationLines(t *testing.T) {
	src := "---\nnotes: |\n  line one\n  line two\nid: 1\n---\n"
	d := mustParse(t, src)
	f, _ := d.Field("notes")
	if got := string(d.Source()[f.Entry.Start:f.Entry.End]); got != "notes: |\n  line one\n  line two\n" {
		t.Fatalf("entry = %q", got)
	}
}

func TestMultiLineFlowSequenceIsUnsupported(t *testing.T) {
	d := mustParse(t, "---\nadrs: [1,\n  2]\n---\n")
	f, ok := d.Field("adrs")
	if !ok {
		t.Fatal("adrs must be indexed")
	}
	if f.Shape != ShapeUnsupported {
		t.Fatalf("a flow sequence spilling onto a second line is not a patch target: %v", f.Shape)
	}
}

func TestMultiLinePlainScalarIsUnsupported(t *testing.T) {
	d := mustParse(t, "---\ntitle: some long\n  continued text\n---\n")
	f, _ := d.Field("title")
	if f.Shape != ShapeUnsupported {
		t.Fatalf("shape = %v, want ShapeUnsupported", f.Shape)
	}
}

func TestExplicitNullKeywordIsInline(t *testing.T) {
	d := mustParse(t, "---\nbranch: null\n---\n")
	f, _ := d.Field("branch")
	if f.Shape != ShapeInline {
		t.Fatalf("shape = %v — an explicit null keyword is a real value token", f.Shape)
	}
	if got := fieldValueText(d, "branch"); got != "null" {
		t.Fatalf("value = %q", got)
	}
}

func TestQuotedInlineScalarsAreInline(t *testing.T) {
	d := mustParse(t, "---\na: 'single'\nb: \"double\"\n---\n")
	for _, c := range []struct{ name, want string }{{"a", "'single'"}, {"b", "\"double\""}} {
		f, ok := d.Field(c.name)
		if !ok {
			t.Fatalf("%s not indexed", c.name)
		}
		if f.Shape != ShapeInline {
			t.Errorf("%s shape = %v, want ShapeInline", c.name, f.Shape)
		}
		if got := fieldValueText(d, c.name); got != c.want {
			t.Errorf("%s value = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestBodyLinesResemblingKeysAreNotIndexed(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\nstatus: this is body prose\n")
	if _, ok := d.Field("status"); ok {
		t.Fatal("status only appears in the body; it must not be indexed")
	}
}

func TestNoFrontmatterMeansNoFields(t *testing.T) {
	d := mustParse(t, "id: 1\nstatus: proposed\n")
	if len(d.Fields()) != 0 {
		t.Fatalf("a frontmatterless document has no fields, got %+v", d.Fields())
	}
}

func TestEmptyValueSpanIsInsertionPoint(t *testing.T) {
	d := mustParse(t, "---\npr:\n---\n")
	f, _ := d.Field("pr")
	if f.Value.Start != f.Value.End {
		t.Fatalf("empty value must have a zero-width span, got %+v", f.Value)
	}
	// The span sits at end-of-line-content: immediately after the colon.
	src := d.Source()
	if src[f.Value.Start-1] != ':' && src[f.Value.Start-1] != ' ' {
		t.Fatalf("insertion point misplaced: byte before is %q", src[f.Value.Start-1])
	}
	if src[f.Value.Start] != '\n' {
		t.Fatalf("insertion point must sit before the terminator, byte is %q", src[f.Value.Start])
	}
}

func TestEmptyValueInsertionPointSitsAfterTrailingSpaces(t *testing.T) {
	src := "---\npr:   \n---\n"
	d := mustParse(t, src)
	f, _ := d.Field("pr")
	if f.Value.Start != f.Value.End {
		t.Fatalf("want a zero-width span, got %+v", f.Value)
	}
	if got := d.Source()[f.Value.Start]; got != '\n' {
		t.Fatalf("insertion point must sit AFTER the trailing spaces, byte is %q", got)
	}
}

func TestCRLFEntryAndValueSpans(t *testing.T) {
	src := "---\r\nstatus: proposed\r\n---\r\n"
	d := mustParse(t, src)
	f, ok := d.Field("status")
	if !ok {
		t.Fatal("status not indexed")
	}
	if got := string(d.Source()[f.Entry.Start:f.Entry.End]); got != "status: proposed\r\n" {
		t.Fatalf("entry = %q — the CRLF terminator belongs to the entry", got)
	}
	if got := fieldValueText(d, "status"); got != "proposed" {
		t.Fatalf("value = %q — the CR must stay outside the value token", got)
	}
}

func TestEmptyFrontmatterHasNoFields(t *testing.T) {
	d := mustParse(t, "---\n---\nbody\n")
	if len(d.Fields()) != 0 {
		t.Fatalf("got %+v", d.Fields())
	}
	if _, ok := d.Field("anything"); ok {
		t.Fatal("Field must report absent for an empty mapping")
	}
}

// parseInteriorRoot returns the semantic mapping root for frontmatter interior
// bytes, so the consistency guard can be driven directly: a genuine
// locator/semantic disagreement cannot be produced from source text, because it
// only exists when the locator itself is wrong.
func parseInteriorRoot(t *testing.T, interior string) *yamlNode {
	t.Helper()
	root, err := parseFrontmatterYAML([]byte(interior), 0, frontmatterLineOffset)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestConsistencyGuardCatchesAnUnlocatedPlainKey(t *testing.T) {
	root := parseInteriorRoot(t, "id: 1\nstatus: proposed\n")
	err := checkLocatorSemanticAgreement([]Field{{Name: "id"}}, root, 4)
	if !IsKind(err, KindInvalidYAML) {
		t.Fatalf("a plain column-zero key the locator missed must be reported: %v", err)
	}
	var de *Error
	if !errors.As(err, &de) || de.Name != "status" {
		t.Fatalf("error must name the missed field, got %v", err)
	}
}

func TestConsistencyGuardCatchesAPhantomField(t *testing.T) {
	root := parseInteriorRoot(t, "id: 1\n")
	err := checkLocatorSemanticAgreement(
		[]Field{{Name: "id"}, {Name: "ghost"}}, root, 4)
	if !IsKind(err, KindInvalidYAML) {
		t.Fatalf("a located field absent from the mapping must be reported: %v", err)
	}
}

func TestConsistencyGuardAcceptsNonPatchTargetKeys(t *testing.T) {
	root := parseInteriorRoot(t, "\"quoted\": v\nUpper: v\n? complex\n: v\nid: 1\n")
	if err := checkLocatorSemanticAgreement([]Field{{Name: "id"}}, root, 4); err != nil {
		t.Fatalf("keys outside the patch-target shape are not the locator's business: %v", err)
	}
}

func TestKeyOutsideTheDocketGrammarEndsThePrecedingEntry(t *testing.T) {
	// Real corpus shape (agents/*.md): a hyphenated key is not a patch target,
	// but it must still terminate the entry above it — otherwise the flow
	// sequence looks multi-line and is wrongly refused as unsupported.
	src := "---\nskills: [docket-adr, docket-convention]\nworktree-scope: metadata\n---\n"
	d := mustParse(t, src)
	f, ok := d.Field("skills")
	if !ok {
		t.Fatal("skills not indexed")
	}
	if got := string(d.Source()[f.Entry.Start:f.Entry.End]); got != "skills: [docket-adr, docket-convention]\n" {
		t.Fatalf("entry = %q — the hyphenated key below is not a continuation", got)
	}
	if f.Shape != ShapeFlowSeq {
		t.Fatalf("shape = %v, want ShapeFlowSeq", f.Shape)
	}
	if _, ok := d.Field("worktree-scope"); ok {
		t.Fatal("a hyphenated key is not a patch target")
	}
}

func TestColumnZeroBlockSequenceIsAContinuation(t *testing.T) {
	src := "---\nitems:\n- a\n- b\nid: 1\n---\n"
	d := mustParse(t, src)
	f, _ := d.Field("items")
	if got := string(d.Source()[f.Entry.Start:f.Entry.End]); got != "items:\n- a\n- b\n" {
		t.Fatalf("entry = %q", got)
	}
	if f.Shape != ShapeUnsupported {
		t.Fatalf("shape = %v, want ShapeUnsupported", f.Shape)
	}
	if _, ok := d.Field("id"); !ok {
		t.Fatal("the entry after a column-zero block sequence must still be indexed")
	}
}
