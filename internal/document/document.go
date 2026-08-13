// Package document reads Docket Markdown records as two coordinated views: an
// immutable copy of the exact source bytes carrying half-open byte spans for
// frontmatter fields and managed blocks, plus a semantic YAML tree used only
// for typed decoding and shape classification.
//
// Existing documents are never emitted through a YAML encoder: the byte
// locator is authoritative for every edit, so a patch validates its complete
// edit set, replaces only the spans it owns, and reparses the candidate before
// returning bytes.
package document

import "unicode/utf8"

// Span is a half-open byte range [Start, End) into Document.Source().
type Span struct {
	Start int
	End   int
}

// Document is an immutable parsed view over an exact byte copy of one record.
type Document struct {
	source     []byte
	lineEnding string // "\n" or "\r\n" — the document-level ending (first terminated line's)
	hasFM      bool
	fmOpen     Span // opening fence line, terminator included
	fmClose    Span // closing fence line, terminator included
}

// Parse copies source and builds the byte view over it: the UTF-8 gate, the
// physical line scan, and frontmatter fence discovery. The copy makes the
// returned Document independent of the caller's buffer.
func Parse(source []byte) (Document, error) {
	src := append([]byte(nil), source...)
	if off, ok := firstInvalidUTF8(src); ok {
		return Document{}, &Error{
			Kind:   KindInvalidUTF8,
			Offset: off,
			Msg:    "source is not valid UTF-8",
		}
	}
	lines := scanLines(src)
	d := Document{source: src, lineEnding: documentLineEnding(lines)}
	hasFM, open, closing, err := locateFrontmatter(src, lines)
	if err != nil {
		return Document{}, err
	}
	d.hasFM, d.fmOpen, d.fmClose = hasFM, open, closing
	return d, nil
}

// firstInvalidUTF8 returns the byte offset of the first invalid UTF-8 encoding
// in src, and whether one was found.
func firstInvalidUTF8(src []byte) (int, bool) {
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if r == utf8.RuneError && size <= 1 {
			return i, true
		}
		i += size
	}
	return 0, false
}

// Source returns a fresh copy of the document's exact bytes.
func (d Document) Source() []byte { return append([]byte(nil), d.source...) }

// HasFrontmatter reports whether the document opens with a frontmatter fence.
func (d Document) HasFrontmatter() bool { return d.hasFM }

// LineEnding returns the document-level line ending: the first terminated
// line's ending, or "\n" when no line is terminated.
func (d Document) LineEnding() string { return d.lineEnding }

// DecodeFrontmatter decodes the frontmatter mapping into destination.
func (d Document) DecodeFrontmatter(destination any) error {
	if !d.hasFM {
		return &Error{Kind: KindMissingFrontmatter, Offset: -1,
			Msg: "document has no frontmatter"}
	}
	// Task 3 replaces this happy path with the semantic YAML decode.
	return nil
}
