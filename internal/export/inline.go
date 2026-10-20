package export

import (
	"regexp"
	"strings"

	"github.com/danielhanold/docket/internal/document"
)

// Placeholder runes for a masked code span. All three come from the Unicode
// private-use area, so no later rule (links, change references) can match inside
// a placeholder: it holds no ASCII, no digits, and no Markdown punctuation.
const (
	maskOpen  = ''
	maskClose = ''
	maskDigit = '' // maskDigit+0 … maskDigit+15 spell the span index in base 16
)

// maskCode replaces every inline code span in line with a placeholder made of
// private-use runes only (no digits, no ASCII), so later rules cannot match inside
// code. restore puts the original spans back byte-for-byte; it accepts any string
// derived from masked, so text around the placeholders may have changed.
//
// A code span follows CommonMark: a run of N backticks opens it and the next run
// of exactly N backticks closes it. A backslash-escaped backtick outside a span
// opens nothing, and a run with no matching closer stays plain text.
func maskCode(line string) (masked string, restore func(string) string) {
	m, r := maskParagraph([]string{line})
	return m[0], r[0]
}

// maskParagraph is maskCode over the lines of one paragraph, where a code span
// may cross a line break: each line's part of such a span is masked on that
// line, and each line gets its own restore.
func maskParagraph(lines []string) (masked []string, restore []func(string) string) {
	joined := strings.Join(lines, "\n")
	ranges := codeSpanRanges(joined)
	masked = make([]string, len(lines))
	restore = make([]func(string) string, len(lines))
	start := 0
	for i, l := range lines {
		end := start + len(l)
		var spans []string
		var b strings.Builder
		pos := start
		for _, r := range ranges {
			from, to := max(r[0], start), min(r[1], end)
			if from >= to {
				continue
			}
			b.WriteString(joined[pos:from])
			b.WriteString(placeholder(len(spans)))
			spans = append(spans, joined[from:to])
			pos = to
		}
		if len(spans) == 0 {
			masked[i], restore[i] = l, func(s string) string { return s }
		} else {
			b.WriteString(joined[pos:end])
			masked[i] = b.String()
			restore[i] = func(s string) string {
				for idx, span := range spans {
					s = strings.ReplaceAll(s, placeholder(idx), span)
				}
				return s
			}
		}
		start = end + 1
	}
	return masked, restore
}

// codeSpanRanges returns the byte range of every inline code span in s,
// backticks included, by the CommonMark rule maskCode describes.
func codeSpanRanges(s string) [][2]int {
	var ranges [][2]int
	for i := 0; i < len(s); {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i += 2
			continue
		}
		if c != '`' {
			i++
			continue
		}
		n := backtickRun(s, i)
		end := closingRun(s, i+n, n)
		if end < 0 {
			i += n
			continue
		}
		ranges = append(ranges, [2]int{i, end})
		i = end
	}
	return ranges
}

// backtickRun returns the length of the backtick run starting at i.
func backtickRun(s string, i int) int {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	return n
}

// closingRun returns the index just past the first run of exactly n backticks at
// or after from, or -1 when there is none.
func closingRun(s string, from, n int) int {
	for j := from; j < len(s); {
		if s[j] != '`' {
			j++
			continue
		}
		m := backtickRun(s, j)
		if m == n {
			return j + m
		}
		j += m
	}
	return -1
}

// placeholder spells span index idx as private-use runes.
func placeholder(idx int) string {
	var digits []rune
	for {
		digits = append([]rune{maskDigit + rune(idx%16)}, digits...)
		idx /= 16
		if idx == 0 {
			break
		}
	}
	return string(maskOpen) + string(digits) + string(maskClose)
}

// linkDefRE matches a CommonMark link-reference definition: up to three spaces of
// indent, a bracketed label, a colon, a destination (angle-bracketed or bare), and
// an optional quoted or parenthesized title that ends the line. A label starting
// with "^" is a footnote, not a link, and is left alone.
var linkDefRE = regexp.MustCompile(`^ {0,3}\[((?:[^\[\]\\]|\\.)+)\]:[ \t]*(?:<[^<>\n]*>|[^\s<]\S*)(?:[ \t]+(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|\((?:[^()\\]|\\.)*\)))?[ \t]*$`)

// isLinkDefinition reports a CommonMark link-reference definition line ("[ref]: url ...").
func isLinkDefinition(line string) bool {
	_, ok := definitionLabel(line)
	return ok
}

// definitionLabel returns the normalized label of a link-reference definition line.
func definitionLabel(line string) (string, bool) {
	m := linkDefRE.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
	if m == nil || strings.HasPrefix(m[1], "^") {
		return "", false
	}
	label := normalizeLabel(m[1])
	if label == "" {
		return "", false
	}
	return label, true
}

// normalizeLabel lower-cases a link label and collapses its whitespace, the way
// CommonMark matches a reference to its definition.
func normalizeLabel(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// linkLabels collects the normalized (lower-cased, whitespace-collapsed) labels of every
// link-reference definition line outside fences.
func linkLabels(lines []string, info []document.LineInfo) map[string]bool {
	labels := map[string]bool{}
	for i, line := range lines {
		if i < len(info) && (info[i].Kind == document.LineFence || info[i].Kind == document.LineFrontmatter) {
			continue
		}
		if label, ok := definitionLabel(line); ok {
			labels[label] = true
		}
	}
	return labels
}

var (
	// autolinkRE is a CommonMark autolink: <scheme:target>, a 2–32 character
	// scheme, and no spaces or angle brackets in the target.
	autolinkRE = regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9+.\-]{1,31}:[^\s<>]*)>`)
	// anchorRE is an HTML anchor element on one line.
	anchorRE = regexp.MustCompile(`(?i)^<a(?:\s[^>]*)?>(.*?)</a\s*>`)
)

// stripLinks removes every link form from a masked line and keeps its visible text:
// inline links and images, reference-style links and images whose label is in
// labels, autolinks, and HTML anchors. An image with empty alt text is removed.
// A bare URL is not link syntax and stays.
func stripLinks(masked string, labels map[string]bool) string {
	var b strings.Builder
	i := 0
	for i < len(masked) {
		c := masked[i]
		switch {
		case c == '\\' && i+1 < len(masked):
			b.WriteString(masked[i : i+2])
			i += 2
			continue
		case c == '<':
			if m := autolinkRE.FindStringSubmatch(masked[i:]); m != nil {
				b.WriteString(m[1])
				i += len(m[0])
				continue
			}
			if m := anchorRE.FindStringSubmatch(masked[i:]); m != nil {
				b.WriteString(stripLinks(m[1], labels))
				i += len(m[0])
				continue
			}
		case c == '[' || (c == '!' && i+1 < len(masked) && masked[i+1] == '['):
			if text, end, ok := parseLink(masked, i, labels); ok {
				b.WriteString(stripLinks(text, labels))
				i = end
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// parseLink parses a link or image starting at s[i] ("[" or "!["). It returns the
// visible text, the index just past the link, and whether s[i:] is a link at all.
func parseLink(s string, i int, labels map[string]bool) (text string, end int, ok bool) {
	open := i
	if s[i] == '!' {
		open++
	}
	closeIdx := matchBracket(s, open)
	if closeIdx < 0 {
		return "", 0, false
	}
	text = s[open+1 : closeIdx]
	after := closeIdx + 1
	if after < len(s) && s[after] == '(' {
		if end, ok := parseInlineTail(s, after); ok {
			return text, end, true
		}
	}
	if after < len(s) && s[after] == '[' {
		refClose := matchBracket(s, after)
		if refClose >= 0 {
			ref := s[after+1 : refClose]
			if ref == "" {
				ref = text
			}
			if labels[normalizeLabel(ref)] {
				return text, refClose + 1, true
			}
			// A full or collapsed reference with no definition is not a link, and
			// its first bracket is not a shortcut either.
			return "", 0, false
		}
	}
	if labels[normalizeLabel(text)] && normalizeLabel(text) != "" {
		return text, after, true
	}
	return "", 0, false
}

// matchBracket returns the index of the "]" that closes the "[" at s[open],
// honoring nesting and backslash escapes, or -1.
func matchBracket(s string, open int) int {
	depth := 0
	for j := open; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return j
			}
		}
	}
	return -1
}

// parseInlineTail parses an inline link's "(destination "title")" starting at the
// "(" at s[i], and returns the index just past its ")". The destination is either
// angle-bracketed or a run of non-space characters with balanced parentheses; the
// title is "…", '…', or (…).
func parseInlineTail(s string, i int) (int, bool) {
	j := skipSpace(s, i+1)
	if j < len(s) && s[j] == '<' {
		k := strings.IndexAny(s[j+1:], "<>")
		if k < 0 || s[j+1+k] != '>' {
			return 0, false
		}
		j += k + 2
	} else {
		depth := 0
	dest:
		for ; j < len(s); j++ {
			switch c := s[j]; {
			case c == '\\':
				j++
			case c == ' ' || c == '\t':
				break dest
			case c == '(':
				depth++
			case c == ')':
				if depth == 0 {
					break dest
				}
				depth--
			}
		}
		if depth != 0 {
			return 0, false
		}
	}
	k := skipSpace(s, j)
	if k < len(s) && k > j && (s[k] == '"' || s[k] == '\'' || s[k] == '(') {
		closer := s[k]
		if closer == '(' {
			closer = ')'
		}
		m := k + 1
		for ; m < len(s) && s[m] != closer; m++ {
			if s[m] == '\\' {
				m++
			}
		}
		if m >= len(s) {
			return 0, false
		}
		k = skipSpace(s, m+1)
	}
	if k < len(s) && s[k] == ')' {
		return k + 1, true
	}
	return 0, false
}

// skipSpace returns the first index at or after i that is not a space or tab.
func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}
