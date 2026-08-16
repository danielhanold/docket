package render_test

import (
	"bytes"
	"testing"

	"github.com/danielhanold/docket/internal/render"
)

// TestApplySectionEditsReplacesOnlyOwnedBytes is the plan's reference test: a
// replace touches only the named section, leaving every unowned byte intact.
func TestApplySectionEditsReplacesOnlyOwnedBytes(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nold why\n\n## Custom notes\n\nkeep me\n")
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionReplace, Markdown: "new why\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("## Custom notes\n\nkeep me\n")) {
		t.Fatalf("unowned section mutated:\n%s", out)
	}
	if bytes.Contains(out, []byte("old why")) {
		t.Fatalf("owned body not replaced:\n%s", out)
	}
	want := []byte("---\nid: 7\n---\n\n## Why\n\nnew why\n\n## Custom notes\n\nkeep me\n")
	if !bytes.Equal(out, want) {
		t.Fatalf("replace produced:\n%q\nwant:\n%q", out, want)
	}
}

// TestApplySectionEditsReplaceAppendsAtEOFWhenAbsent: replace of an absent
// section appends it at EOF preceded by exactly one blank line, trailing
// newline preserved.
func TestApplySectionEditsReplaceAppendsAtEOFWhenAbsent(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nwhy body\n")
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why deferred", Intent: render.SectionReplace, Markdown: "deferred reason\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("---\nid: 7\n---\n\n## Why\n\nwhy body\n\n## Why deferred\n\ndeferred reason\n")
	if !bytes.Equal(out, want) {
		t.Fatalf("append produced:\n%q\nwant:\n%q", out, want)
	}
}

// TestApplySectionEditsRemoveDeletesSection: remove deletes heading + body.
func TestApplySectionEditsRemoveDeletesSection(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nold why\n\n## Custom notes\n\nkeep me\n")
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionRemove},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("---\nid: 7\n---\n\n## Custom notes\n\nkeep me\n")
	if !bytes.Equal(out, want) {
		t.Fatalf("remove produced:\n%q\nwant:\n%q", out, want)
	}
}

// TestApplySectionEditsRemoveAbsentErrors: removing a section not present is an
// error.
func TestApplySectionEditsRemoveAbsentErrors(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nold why\n")
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why killed", Intent: render.SectionRemove},
	})
	if err == nil {
		t.Fatalf("remove of absent section returned no error; out:\n%s", out)
	}
}

// TestApplySectionEditsFencedHeadingIsNotASection exercises the fence-state
// guard: a "## Why killed" line inside a fenced code block is authored content,
// not a section, so removing that heading errors (it is absent as a real
// section) and the fenced bytes survive a replace of the enclosing section.
func TestApplySectionEditsFencedHeadingIsNotASection(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n" +
		"## Why\n\n" +
		"Example follows:\n\n" +
		"```\n" +
		"## Why killed\n" +
		"killed prose\n" +
		"```\n\n" +
		"## What changes\n\nchange body\n")

	// The only "## Why killed" is inside a fence, so it is not a section.
	if _, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why killed", Intent: render.SectionRemove},
	}); err == nil {
		t.Fatalf("removing a fenced pseudo-heading should error (absent section)")
	}

	// A replace of the enclosing "## Why" section keeps the fenced block intact.
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## What changes", Intent: render.SectionReplace, Markdown: "new change body\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("```\n## Why killed\nkilled prose\n```\n")) {
		t.Fatalf("fenced block corrupted:\n%s", out)
	}
}

// TestApplySectionEditsDuplicateOwnedHeadingRefuses exercises the
// duplicate-heading guard: a duplicated owned heading refuses the whole edit
// set and leaves the source unchanged.
func TestApplySectionEditsDuplicateOwnedHeadingRefuses(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nfirst\n\n## Why\n\nsecond\n")
	before := append([]byte(nil), src...)
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionReplace, Markdown: "new\n"},
	})
	if err == nil {
		t.Fatalf("duplicate owned heading returned no error; out:\n%s", out)
	}
	if !bytes.Equal(src, before) {
		t.Fatalf("source mutated on refusal")
	}
}

// TestApplySectionEditsEditOutsideOwnedErrors: an edit naming a heading outside
// the owned set is an error.
func TestApplySectionEditsEditOutsideOwnedErrors(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nbody\n")
	if _, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Not owned", Intent: render.SectionReplace, Markdown: "x\n"},
	}); err == nil {
		t.Fatalf("edit outside owned set returned no error")
	}
}

// TestApplySectionEditsTwoEditsSpliceCorrectly: two edits in one call both
// apply; the internal end-toward-beginning splice order keeps offsets valid.
func TestApplySectionEditsTwoEditsSpliceCorrectly(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n" +
		"## Why\n\nold why\n\n" +
		"## Custom\n\nunowned mid\n\n" +
		"## Out of scope\n\nold scope\n")
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionReplace, Markdown: "new why\n"},
		{Heading: "## Out of scope", Intent: render.SectionReplace, Markdown: "new scope\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("---\nid: 7\n---\n\n" +
		"## Why\n\nnew why\n\n" +
		"## Custom\n\nunowned mid\n\n" +
		"## Out of scope\n\nnew scope\n")
	if !bytes.Equal(out, want) {
		t.Fatalf("two-edit splice produced:\n%q\nwant:\n%q", out, want)
	}
}

// TestApplySectionEditsUnknownSectionsSurvive: unknown headings between owned
// ones survive byte-identically.
func TestApplySectionEditsUnknownSectionsSurvive(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n" +
		"## Why\n\nold\n\n" +
		"## Unknown Alpha\n\nkeep alpha\n\n" +
		"### Nested three\n\nkeep nested\n\n" +
		"## Out of scope\n\nscope\n")
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionReplace, Markdown: "new\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range [][]byte{
		[]byte("## Unknown Alpha\n\nkeep alpha\n"),
		[]byte("### Nested three\n\nkeep nested\n"),
		[]byte("## Out of scope\n\nscope\n"),
	} {
		if !bytes.Contains(out, keep) {
			t.Fatalf("unowned content %q dropped:\n%s", keep, out)
		}
	}
}

// TestApplySectionEditsCandidateReparseFailureRefuses: a splice that yields a
// document document.Parse rejects (here, a dangling managed-block marker in the
// authored body) refuses.
func TestApplySectionEditsCandidateReparseFailureRefuses(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nold\n")
	if _, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionReplace, Markdown: "<!-- docket:evil:start -->\n"},
	}); err == nil {
		t.Fatalf("candidate with a dangling marker should refuse to reparse")
	}
}

// TestApplySectionEditsReplaceCRLF: a CRLF fixture keeps every byte around the
// replaced section identical.
func TestApplySectionEditsReplaceCRLF(t *testing.T) {
	src := []byte("---\r\nid: 7\r\n---\r\n\r\n## Why\r\n\r\nold why\r\n\r\n## Custom notes\r\n\r\nkeep me\r\n")
	out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionReplace, Markdown: "new why\r\n"},
	})
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte("---\r\nid: 7\r\n---\r\n\r\n")
	suffix := []byte("## Custom notes\r\n\r\nkeep me\r\n")
	if !bytes.HasPrefix(out, prefix) {
		t.Fatalf("CRLF prefix mutated:\n%q", out)
	}
	if !bytes.HasSuffix(out, suffix) {
		t.Fatalf("CRLF suffix mutated:\n%q", out)
	}
	if bytes.Contains(out, []byte("old why")) {
		t.Fatalf("CRLF body not replaced:\n%q", out)
	}
}

// TestApplySectionEditsPreserveIsNoOp: preserve leaves the document untouched
// whether the section is present or absent.
func TestApplySectionEditsPreserveIsNoOp(t *testing.T) {
	for _, src := range [][]byte{
		[]byte("---\nid: 7\n---\n\n## Why\n\nbody\n"),
		[]byte("---\nid: 7\n---\n\n## What changes\n\nbody\n"),
	} {
		out, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
			{Heading: "## Why", Intent: render.SectionPreserve},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out, src) {
			t.Fatalf("preserve mutated document:\n%q", out)
		}
	}
}

// TestApplySectionEditsNonReplaceRejectsMarkdown: preserve/remove must carry
// empty Markdown.
func TestApplySectionEditsNonReplaceRejectsMarkdown(t *testing.T) {
	src := []byte("---\nid: 7\n---\n\n## Why\n\nbody\n")
	if _, err := render.ApplySectionEdits(src, render.ChangeOwnedHeadings, []render.SectionEdit{
		{Heading: "## Why", Intent: render.SectionRemove, Markdown: "nope\n"},
	}); err == nil {
		t.Fatalf("remove carrying Markdown should error")
	}
}
