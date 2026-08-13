package document

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
