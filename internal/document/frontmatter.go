package document

import (
	"bytes"
	"regexp"

	"go.yaml.in/yaml/v3"
)

// sourceLine is one physical line of the source.
type sourceLine struct {
	span   Span   // full line including its terminator
	text   Span   // line content excluding the terminator
	ending string // "\n", "\r\n", or "" for a final unterminated line
}

// scanLines splits src into physical lines. The returned spans tile src
// exactly: every byte belongs to exactly one line's span, in order.
func scanLines(src []byte) []sourceLine {
	var lines []sourceLine
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			end := i + 1
			textEnd, ending := i, "\n"
			if i > start && src[i-1] == '\r' {
				textEnd, ending = i-1, "\r\n"
			}
			lines = append(lines, sourceLine{
				span: Span{start, end}, text: Span{start, textEnd}, ending: ending})
			start = end
		}
	}
	if start < len(src) {
		lines = append(lines, sourceLine{
			span: Span{start, len(src)}, text: Span{start, len(src)}, ending: ""})
	}
	return lines
}

// lineIsExactly reports whether the line's content (terminator excluded) equals s.
func lineIsExactly(src []byte, ln sourceLine, s string) bool {
	return string(src[ln.text.Start:ln.text.End]) == s
}

// documentLineEnding returns the first terminated line's ending, defaulting to
// "\n" when the source has no terminated line at all.
func documentLineEnding(lines []sourceLine) string {
	for _, ln := range lines {
		if ln.ending != "" {
			return ln.ending
		}
	}
	return "\n"
}

// fenceLine is the exact opening/closing frontmatter fence content.
const fenceLine = "---"

// locateFrontmatter finds the frontmatter fences. Frontmatter exists only when
// the very first line is exactly "---"; a "---" further down is a Markdown
// horizontal rule, not a fence. Returns hasFM plus the two full fence-line
// spans (terminators included).
func locateFrontmatter(src []byte, lines []sourceLine) (hasFM bool, open, closing Span, err error) {
	if len(lines) == 0 || !lineIsExactly(src, lines[0], fenceLine) {
		return false, Span{}, Span{}, nil
	}
	for i := 1; i < len(lines); i++ {
		if lineIsExactly(src, lines[i], fenceLine) {
			return true, lines[0].span, lines[i].span, nil
		}
	}
	return false, Span{}, Span{}, &Error{
		Kind:   KindUnclosedFrontmatter,
		Offset: lines[0].span.Start,
		Line:   1,
		Column: 1,
		Msg:    "frontmatter opener has no matching closing fence",
	}
}

// docketKeyLineRE matches a line that opens a patchable frontmatter entry: a
// column-zero plain key in the Docket key grammar followed by its colon. The
// trailing "(\s|$)" keeps "key:value" (not a YAML mapping entry at all) and
// "key:# c" out of the index.
var docketKeyLineRE = regexp.MustCompile(`^([a-z][a-z0-9_]*):(\s|$)`)

// docketKeyNameRE is the same grammar applied to a bare name, used to decide
// which semantic mapping keys the locator is obliged to have found.
var docketKeyNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// locateFields builds the byte-level field index over the frontmatter interior.
// The locator is authoritative for every span; the semantic tree supplies only
// the shape verdict and a consistency check.
func locateFields(src []byte, lines []sourceLine, open, closing Span, root *yaml.Node) ([]Field, error) {
	first, last := interiorLineRange(lines, open, closing)

	// Pass one: every line that opens an entry. A line that does not match is a
	// continuation, a comment, indented block content, or a key shape this
	// package deliberately does not expose as a patch target.
	var starts []int
	var names []string
	for i := first; i < last; i++ {
		if m := docketKeyLineRE.FindSubmatch(src[lines[i].text.Start:lines[i].text.End]); m != nil {
			starts = append(starts, i)
			names = append(names, string(m[1]))
		}
	}

	fields := make([]Field, 0, len(starts))
	for k, startLine := range starts {
		// An entry absorbs the lines its value continues onto. Continuation is
		// keyed on indentation rather than on "the next line the locator would
		// index", because a key this package does not expose as a patch target
		// — "worktree-scope:", "alwaysApply:" — still ends the entry before it.
		endLine := startLine + 1 // exclusive
		for endLine < last && isEntryContinuation(src, lines[endLine]) {
			endLine++
		}
		// Blank and column-zero comment lines trailing an entry belong to the
		// document, not to the entry. (A trailing blank line inside a literal
		// block scalar is given up here; YAML clips it anyway, and the shape is
		// unsupported for patching either way.)
		for endLine-1 > startLine && isBlankOrTopLevelComment(src, lines[endLine-1]) {
			endLine--
		}
		entry := Span{lines[startLine].span.Start, lines[endLine-1].span.End}
		value := locateValueToken(src, lines[startLine], names[k])
		shape := classifyShape(root, names[k], endLine-1 == startLine, value.Start < value.End)
		fields = append(fields, Field{Name: names[k], Entry: entry, Value: value, Shape: shape})
	}

	if err := checkLocatorSemanticAgreement(fields, root, open.End); err != nil {
		return nil, err
	}
	return fields, nil
}

// interiorLineRange returns the half-open line-index range strictly between the
// two frontmatter fences.
func interiorLineRange(lines []sourceLine, open, closing Span) (first, last int) {
	first, last = 0, 0
	for i, ln := range lines {
		if ln.span == open {
			first = i + 1
		}
		if ln.span == closing {
			last = i
			break
		}
	}
	return first, last
}

// isEntryContinuation reports whether a line continues the mapping entry above
// it. In a column-zero block mapping every continuation is either blank,
// indented past the key, or a column-zero block-sequence item ("- x", the one
// shape YAML lets sit at the parent's own indentation). Any other column-zero
// line opens something new.
func isEntryContinuation(src []byte, ln sourceLine) bool {
	text := src[ln.text.Start:ln.text.End]
	if len(text) == 0 {
		return true
	}
	if text[0] == ' ' || text[0] == '\t' {
		return true
	}
	return text[0] == '-' && (len(text) == 1 || text[1] == ' ' || text[1] == '\t')
}

// isBlankOrTopLevelComment reports whether the line is whitespace-only or a
// comment introduced at column zero. A column-zero "#" cannot sit inside a
// block scalar (whose content must be indented past its key), so trimming these
// off an entry never eats a field's own value.
func isBlankOrTopLevelComment(src []byte, ln sourceLine) bool {
	text := src[ln.text.Start:ln.text.End]
	if len(text) > 0 && text[0] == '#' {
		return true
	}
	return len(bytes.TrimLeft(text, " \t")) == 0
}

// locateValueToken returns the span of the value token on an entry's key line:
// everything after the colon, less the pre-value spacing, less any inline
// comment, less the spacing before that comment. An empty region yields a
// zero-width insertion point placed AFTER the spacing that follows the colon,
// so writing a value there never has to disturb bytes it does not own.
func locateValueToken(src []byte, ln sourceLine, name string) Span {
	regionStart := ln.text.Start + len(name) + 1 // one past the ':'
	regionEnd := commentStart(src, regionStart, ln.text.End)

	start := regionStart
	for start < regionEnd && (src[start] == ' ' || src[start] == '\t') {
		start++
	}
	end := regionEnd
	for end > start && (src[end-1] == ' ' || src[end-1] == '\t') {
		end--
	}
	if start >= end {
		return Span{start, start}
	}
	return Span{start, end}
}

// commentStart scans the value region left to right and returns the offset at
// which an inline comment begins, or end when there is none. A '#' introduces a
// comment only when it is outside a quoted scalar and preceded by whitespace or
// nothing — YAML's own rule.
//
// Quote state opens only where a scalar may legally begin: at the region start
// or just after a '[' or ',' in a flow collection. That distinction is what
// keeps the apostrophe in "title: don't stop # x" from swallowing the comment
// while still shielding "tags: ['a # b']".
func commentStart(src []byte, start, end int) int {
	const (
		outside = iota
		inSingle
		inDouble
	)
	state := outside
	canOpenScalar := true
	for i := start; i < end; i++ {
		b := src[i]
		switch state {
		case inSingle:
			if b == '\'' {
				if i+1 < end && src[i+1] == '\'' { // doubled apostrophe: an escape
					i++
					continue
				}
				state = outside
			}
		case inDouble:
			switch b {
			case '\\':
				i++
			case '"':
				state = outside
			}
		default:
			switch {
			case b == ' ' || b == '\t':
				continue // spacing never changes where a scalar may begin
			case b == '#' && (i == start || src[i-1] == ' ' || src[i-1] == '\t'):
				return i
			case b == '\'' && canOpenScalar:
				state = inSingle
			case b == '"' && canOpenScalar:
				state = inDouble
			}
			canOpenScalar = b == '[' || b == ','
		}
	}
	return end
}

// classifyShape asks the semantic tree how the located value is written.
// singleLine and hasToken come from the byte locator; a disagreement between
// the two views (a semantic null where the bytes show a token, or the reverse)
// fails closed to ShapeUnsupported rather than handing a patch a span it cannot
// safely replace.
func classifyShape(root *yaml.Node, name string, singleLine, hasToken bool) FieldShape {
	value := mappingValue(root, name)
	if value == nil {
		return ShapeUnsupported // reported by checkLocatorSemanticAgreement
	}
	switch value.Kind {
	case yaml.ScalarNode:
		// An empty null is the "key:" form; an explicit "null"/"~" keyword is a
		// real value token and stays inline.
		if value.Tag == "!!null" && value.Value == "" {
			if hasToken {
				return ShapeUnsupported
			}
			return ShapeEmpty
		}
		if !singleLine || !hasToken {
			return ShapeUnsupported
		}
		switch value.Style {
		case 0, yaml.SingleQuotedStyle, yaml.DoubleQuotedStyle:
			return ShapeInline
		}
		return ShapeUnsupported // literal, folded, tagged, anchored
	case yaml.SequenceNode:
		if singleLine && hasToken && value.Style&yaml.FlowStyle != 0 {
			return ShapeFlowSeq
		}
		return ShapeUnsupported
	}
	return ShapeUnsupported // mapping, alias, or anything else
}

// mappingValue returns the value node of the first entry keyed by name.
func mappingValue(root *yaml.Node, name string) *yaml.Node {
	for i := 0; i+1 < len(root.Content); i += 2 {
		if key := root.Content[i]; key.Kind == yaml.ScalarNode && key.Value == name {
			return root.Content[i+1]
		}
	}
	return nil
}

// checkLocatorSemanticAgreement cross-checks the two views over the same
// bytes. Every located field must exist in the semantic mapping, and every
// semantic key that LOOKS like a patch target — plain, column-zero, Docket key
// grammar — must have been located. Either gap is a locator defect, and
// mis-indexing a span is far worse than refusing the document.
func checkLocatorSemanticAgreement(fields []Field, root *yaml.Node, fmOffset int) error {
	located := make(map[string]bool, len(fields))
	for _, f := range fields {
		located[f.Name] = true
	}
	semantic := make(map[string]bool, len(root.Content)/2)
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		semantic[key.Value] = true
		patchShaped := key.Style == 0 && key.Column == 1 && docketKeyNameRE.MatchString(key.Value)
		if patchShaped && !located[key.Value] {
			return &Error{
				Kind:   KindInvalidYAML,
				Name:   key.Value,
				Offset: fmOffset,
				Line:   key.Line + frontmatterLineOffset,
				Column: key.Column,
				Msg:    "locator/semantic mismatch: field is a plain column-zero key but was not located",
			}
		}
	}
	for _, f := range fields {
		if !semantic[f.Name] {
			return &Error{
				Kind:   KindInvalidYAML,
				Name:   f.Name,
				Offset: f.Entry.Start,
				Msg:    "locator/semantic mismatch: located field is absent from the frontmatter mapping",
			}
		}
	}
	return nil
}
