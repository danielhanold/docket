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

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strconv"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// Span is a half-open byte range [Start, End) into Document.Source().
type Span struct {
	Start int
	End   int
}

// FieldShape classifies how a located field's value is written in source.
type FieldShape int

const (
	ShapeEmpty       FieldShape = iota // "key:" — a null with no value token
	ShapeInline                        // single-line plain/quoted scalar
	ShapeFlowSeq                       // single-line flow sequence, e.g. [3, 7]
	ShapeUnsupported                   // block scalar, block collection, multi-line flow, anchor/alias…
)

// String names the shape for diagnostics and test failures.
func (s FieldShape) String() string {
	switch s {
	case ShapeEmpty:
		return "empty"
	case ShapeInline:
		return "inline"
	case ShapeFlowSeq:
		return "flow-seq"
	default:
		return "unsupported"
	}
}

// Field is one located column-zero frontmatter mapping entry.
type Field struct {
	Name  string
	Entry Span // the full physical line(s) of the entry, terminator included
	Value Span // the value token; Start==End for ShapeEmpty (insertion point)
	Shape FieldShape
}

// Document is an immutable parsed view over an exact byte copy of one record.
type Document struct {
	source     []byte
	lineEnding string // "\n" or "\r\n" — the document-level ending (first terminated line's)
	hasFM      bool
	fmOpen     Span       // opening fence line, terminator included
	fmClose    Span       // closing fence line, terminator included
	fields     []Field    // located frontmatter entries, in source order
	yamlRoot   *yaml.Node // private; never crosses the package boundary
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
	if d.hasFM {
		root, err := parseFrontmatterYAML(src[d.fmOpen.End:d.fmClose.Start], d.fmOpen.End, frontmatterLineOffset)
		if err != nil {
			return Document{}, err
		}
		d.yamlRoot = root
		fields, err := locateFields(src, lines, d.fmOpen, d.fmClose, root)
		if err != nil {
			return Document{}, err
		}
		d.fields = fields
	}
	return d, nil
}

// frontmatterLineOffset converts a line number reported inside the
// frontmatter interior into a document line number: the interior starts on the
// line after the opening fence, which is document line 1.
const frontmatterLineOffset = 1

// yamlErrLine matches the "line N" position yaml.v3 embeds in its syntax
// errors ("yaml: line 1: did not find expected ',' or ']'").
var yamlErrLine = regexp.MustCompile(`line (\d+):`)

// parseFrontmatterYAML takes the frontmatter interior bytes to the validated
// mapping root of its single YAML document. interiorOffset is the interior's
// byte offset in the source and lineOffset the document line the interior's
// line 1 sits on, so diagnostics are reported in document coordinates.
//
// The rules enforced here are the semantic half of Parse: exactly one YAML
// document, a mapping root, no duplicate key, and no unresolved alias. An
// empty (or comments-only) interior is an empty mapping, not an error.
func parseFrontmatterYAML(interior []byte, interiorOffset, lineOffset int) (*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(interior))

	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return emptyMapping(), nil // empty or comments-only frontmatter
		}
		return nil, classifyYAMLError(err, interiorOffset, lineOffset)
	}

	// A second successful decode means a second document in the stream —
	// mirrors internal/config's parseLayer two-decode check.
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case err == nil:
		return nil, &Error{
			Kind:   KindInvalidYAML,
			Offset: interiorOffset,
			Line:   extra.Line + lineOffset,
			Column: extra.Column,
			Msg:    "frontmatter must contain exactly one YAML document",
		}
	case !errors.Is(err, io.EOF):
		return nil, classifyYAMLError(err, interiorOffset, lineOffset)
	}

	root := &doc
	if root.Kind == yaml.DocumentNode { // unwrap to the document's single root node
		if len(root.Content) == 0 {
			return emptyMapping(), nil
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, &Error{
			Kind:   KindInvalidYAML,
			Offset: interiorOffset,
			Line:   root.Line + lineOffset,
			Column: root.Column,
			Msg:    "frontmatter must be a mapping of fields",
		}
	}
	if err := rejectDuplicateKeys(root, interiorOffset, lineOffset); err != nil {
		return nil, err
	}
	return root, nil
}

// emptyMapping is the synthetic root standing in for frontmatter that carries
// no fields, so callers never have to nil-check the semantic tree.
func emptyMapping() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

// rejectDuplicateKeys reports the SECOND occurrence of a repeated mapping key,
// recursively. yaml.v3 only detects duplicates while decoding into a Go map or
// struct — decoding into a yaml.Node builds the tree with both entries and no
// error — so the check has to be made here or the last one silently wins.
func rejectDuplicateKeys(n *yaml.Node, interiorOffset, lineOffset int) error {
	switch n.Kind {
	case yaml.MappingNode:
		seen := make(map[string]int, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, value := n.Content[i], n.Content[i+1]
			if key.Kind == yaml.ScalarNode {
				if first, dup := seen[key.Value]; dup {
					return &Error{
						Kind:   KindDuplicateField,
						Name:   key.Value,
						Offset: interiorOffset,
						Line:   key.Line + lineOffset,
						Column: key.Column,
						Msg: "field is declared more than once (first declared on line " +
							strconv.Itoa(first+lineOffset) + ")",
					}
				}
				seen[key.Value] = key.Line
			}
			if err := rejectDuplicateKeys(value, interiorOffset, lineOffset); err != nil {
				return err
			}
		}
	case yaml.SequenceNode, yaml.DocumentNode:
		for _, child := range n.Content {
			if err := rejectDuplicateKeys(child, interiorOffset, lineOffset); err != nil {
				return err
			}
		}
	}
	return nil
}

// classifyYAMLError turns a yaml.v3 decode failure into a typed *Error. Every
// such failure — malformed YAML, an unresolved alias ("unknown anchor 'x'
// referenced") — is invalid-yaml; the library's own "line N" position is
// recovered best-effort and shifted into document coordinates.
func classifyYAMLError(err error, interiorOffset, lineOffset int) error {
	e := &Error{Kind: KindInvalidYAML, Offset: interiorOffset, Msg: err.Error()}
	if m := yamlErrLine.FindStringSubmatch(err.Error()); m != nil {
		if n, convErr := strconv.Atoi(m[1]); convErr == nil && n >= 1 {
			e.Line, e.Column = n+lineOffset, 1
		}
	}
	return e
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

// Fields returns the located frontmatter entries in source order, as a fresh
// slice: mutating it cannot reach the document's own index.
func (d Document) Fields() []Field { return append([]Field(nil), d.fields...) }

// Field returns the located entry named name, and whether it exists. Only
// column-zero plain keys matching the Docket key grammar are indexed, so a
// quoted, capitalized, or explicit YAML key is never reported here even though
// DecodeFrontmatter still sees it.
func (d Document) Field(name string) (Field, bool) {
	for _, f := range d.fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

// DecodeFrontmatter decodes the frontmatter mapping into destination.
// Unknown keys are compatibility data, never errors: known-field rejection
// stays off, so a record written by a newer Docket still decodes here.
func (d Document) DecodeFrontmatter(destination any) error {
	if !d.hasFM {
		return &Error{Kind: KindMissingFrontmatter, Offset: -1,
			Msg: "document has no frontmatter"}
	}
	if err := d.yamlRoot.Decode(destination); err != nil {
		return &Error{Kind: KindInvalidYAML, Offset: -1,
			Msg: "decode frontmatter: " + err.Error()}
	}
	return nil
}
