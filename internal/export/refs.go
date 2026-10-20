package export

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Titles maps a known change id to its raw title. An id the snapshot cannot resolve
// to exactly one change is absent, so it reads as unknown.
type Titles map[int]string

// A change reference is recognized in one of two forms; recognizing a form
// decides nothing on its own, since only a number present in Titles converts.
//
//   - Word form: "change" (whole word, any case), whitespace, optional '#', digits.
//     The plural "changes" takes a list of such numbers joined by ",", "and", or ", and".
//   - Padded form: "#0ddd", or a bare "0ddd" standing as its own word, where the
//     surrounding characters rule out paths, filenames, and ADR numbers.
//
// Bare unpadded "#N" and "PR #N" are never forms.
var (
	wordFormRE   = regexp.MustCompile(`(?i)\bchange(s?)\s+#?(\d+)`)
	listNextRE   = regexp.MustCompile(`^(?:\s*,\s*and\s+|\s*,\s*|\s+and\s+)#?(\d+)`)
	paddedFormRE = regexp.MustCompile(`#?0\d{3}`)
	adrBeforeRE  = regexp.MustCompile(`(?i)\bADR[- ]$`)
	prBeforeRE   = regexp.MustCompile(`\bPR $`)
	// otherRefRE matches a reference token that is not a form: it may share a
	// references-only parenthesis with forms, and it always stays as written.
	otherRefRE = regexp.MustCompile(`^(?:PR #\d+|#\d+|(?i:ADR)[- ]\d+(?:'s)?)`)
	// refSepRE matches the separators allowed between items of a parenthesis.
	refSepRE = regexp.MustCompile(`^(?:\s*,\s*and\s+|\s*,\s*|\s+and\s+|\s+)`)
)

// refItem is one number inside a reference, located by its byte range in the line.
type refItem struct {
	start, end int
	id         int
	known      bool
}

// refSpan is one recognized reference: a singular form, or a plural word-form list.
type refSpan struct {
	start, end int
	items      []refItem
	plural     bool
}

func (s refSpan) allKnown() bool {
	for _, it := range s.items {
		if !it.known {
			return false
		}
	}
	return true
}

func (s refSpan) anyKnown() bool {
	for _, it := range s.items {
		if it.known {
			return true
		}
	}
	return false
}

// findRefs returns every recognized reference in the masked line, in order and
// non-overlapping. Word forms win over the padded numbers they contain.
func findRefs(line string, titles Titles) []refSpan {
	item := func(start, end int) refItem {
		digits := strings.TrimPrefix(line[start:end], "#")
		id, err := strconv.Atoi(digits)
		_, ok := titles[id]
		return refItem{start: start, end: end, id: id, known: err == nil && ok}
	}
	var spans []refSpan
	for _, m := range wordFormRE.FindAllStringSubmatchIndex(line, -1) {
		numStart := m[4]
		if line[numStart-1] == '#' {
			numStart--
		}
		if !numberEnds(line, m[5]) {
			continue
		}
		span := refSpan{start: m[0], end: m[1], items: []refItem{item(numStart, m[5])}, plural: m[3] > m[2]}
		if span.plural {
			for {
				next := listNextRE.FindStringSubmatchIndex(line[span.end:])
				if next == nil || !numberEnds(line, span.end+next[3]) {
					break
				}
				itemStart := span.end + next[2]
				if line[itemStart-1] == '#' {
					itemStart--
				}
				span.items = append(span.items, item(itemStart, span.end+next[3]))
				span.end += next[1]
			}
		}
		spans = append(spans, span)
	}
	for _, m := range paddedFormRE.FindAllStringIndex(line, -1) {
		start, end := m[0], m[1]
		if overlaps(spans, start, end) || !paddedStandsAlone(line, start, end) {
			continue
		}
		spans = append(spans, refSpan{start: start, end: end, items: []refItem{item(start, end)}})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	return spans
}

// numberEnds reports that the digits ending at end are a whole number, not the
// start of a longer word, path, or decimal.
func numberEnds(line string, end int) bool {
	if end >= len(line) {
		return true
	}
	c := line[end]
	if isWordByte(c) {
		return false
	}
	return !(c == '.' && end+1 < len(line) && isAlnum(line[end+1]))
}

// paddedStandsAlone applies the padded form's boundary rules.
func paddedStandsAlone(line string, start, end int) bool {
	if start > 0 && strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_/.#-", rune(line[start-1])) {
		return false
	}
	if end < len(line) {
		c := line[end]
		if isAlnum(c) || c == '_' || c == '/' || c == '-' {
			return false
		}
		if c == '.' && end+1 < len(line) && isAlnum(line[end+1]) {
			return false
		}
	}
	before := line[:start]
	return !adrBeforeRE.MatchString(before) && !prBeforeRE.MatchString(before)
}

func overlaps(spans []refSpan, start, end int) bool {
	for _, s := range spans {
		if start < s.end && s.start < end {
			return true
		}
	}
	return false
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isWordByte(c byte) bool { return isAlnum(c) || c == '_' }

// refEdit replaces line[start:end] with text.
type refEdit struct {
	start, end int
	text       string
}

func applyEdits(line string, edits []refEdit) string {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var b strings.Builder
	pos := 0
	for _, e := range edits {
		b.WriteString(line[pos:e.start])
		b.WriteString(e.text)
		pos = e.end
	}
	b.WriteString(line[pos:])
	return b.String()
}

// parenEdits applies the references-only parenthesis reading: a parenthesis whose
// whole content is reference items, at least one of them a fully convertible
// form, loses its convertible items; the rest re-join, and an emptied parenthesis
// goes with the one space before it. render gives a kept span's text. The spans
// it consumed are reported so no other reading touches them.
func parenEdits(line string, spans []refSpan, render func(refSpan) string) (edits []refEdit, consumed map[int]bool) {
	consumed = map[int]bool{}
	for open := strings.IndexByte(line, '('); open >= 0; {
		closeRel := strings.IndexAny(line[open+1:], "()")
		if closeRel < 0 {
			break
		}
		closeAt := open + 1 + closeRel
		if line[closeAt] == '(' {
			open = closeAt
			continue
		}
		if e, used, ok := parenReading(line, open, closeAt, spans, render); ok {
			edits = append(edits, e)
			for _, i := range used {
				consumed[i] = true
			}
		}
		next := strings.IndexByte(line[closeAt+1:], '(')
		if next < 0 {
			break
		}
		open = closeAt + 1 + next
	}
	return edits, consumed
}

func parenReading(line string, open, closeAt int, spans []refSpan, render func(refSpan) string) (refEdit, []int, bool) {
	type token struct {
		text    string
		drop    bool
		sepAnd  bool // the separator before this token held "and"
		spanIdx int
	}
	var toks []token
	var used []int
	pos := open + 1
	sepAnd := false
	for {
		if sep := refSepRE.FindString(line[pos:closeAt]); sep != "" {
			sepAnd = strings.Contains(sep, "and")
			pos += len(sep)
		}
		if pos >= closeAt {
			break
		}
		idx := -1
		for i, s := range spans {
			if s.start == pos && s.end <= closeAt {
				idx = i
				break
			}
		}
		switch {
		case idx >= 0:
			s := spans[idx]
			toks = append(toks, token{text: render(s), drop: s.allKnown(), sepAnd: sepAnd, spanIdx: idx})
			used = append(used, idx)
			pos = s.end
		default:
			m := otherRefRE.FindString(line[pos:closeAt])
			if m == "" {
				return refEdit{}, nil, false
			}
			toks = append(toks, token{text: m, sepAnd: sepAnd, spanIdx: -1})
			pos += len(m)
		}
		sepAnd = false
	}
	dropped := false
	var kept []token
	for _, tk := range toks {
		if tk.drop {
			dropped = true
		} else {
			kept = append(kept, tk)
		}
	}
	if !dropped {
		return refEdit{}, nil, false
	}
	if len(kept) == 0 {
		start := open
		if start > 0 && line[start-1] == ' ' {
			start--
		}
		return refEdit{start: start, end: closeAt + 1}, used, true
	}
	lastAnd := toks[len(toks)-1].sepAnd
	var b strings.Builder
	b.WriteByte('(')
	for i, tk := range kept {
		switch {
		case i == 0:
		case i == len(kept)-1 && lastAnd:
			b.WriteString(" and ")
		default:
			b.WriteString(", ")
		}
		b.WriteString(tk.text)
	}
	b.WriteByte(')')
	return refEdit{start: open, end: closeAt + 1, text: b.String()}, used, true
}

// convertRefs runs step 6 on one masked line: every recognized form whose number
// is a known change id is removed (references-only parenthesis, own-title dash)
// or replaced by that change's cleaned title in double quotes.
func convertRefs(masked string, titles Titles) string {
	spans := findRefs(masked, titles)
	if len(spans) == 0 {
		return masked
	}
	quoted := func(id int) string { return `"` + cleanTitle(titles[id], titles) + `"` }
	render := func(s refSpan) string {
		if !s.anyKnown() {
			return masked[s.start:s.end]
		}
		if s.plural && s.allKnown() {
			parts := make([]string, len(s.items))
			for i, it := range s.items {
				parts[i] = quoted(it.id)
			}
			return joinList(parts)
		}
		if !s.plural {
			return quoted(s.items[0].id)
		}
		var edits []refEdit
		for _, it := range s.items {
			if it.known {
				edits = append(edits, refEdit{start: it.start - s.start, end: it.end - s.start, text: quoted(it.id)})
			}
		}
		return applyEdits(masked[s.start:s.end], edits)
	}
	edits, consumed := parenEdits(masked, spans, render)
	for i, s := range spans {
		if consumed[i] || !s.anyKnown() {
			continue
		}
		if end, ok := ownTitleDash(masked, s, titles); ok {
			edits = append(edits, refEdit{start: s.start, end: end})
			continue
		}
		edits = append(edits, refEdit{start: s.start, end: s.end, text: render(s)})
	}
	return applyEdits(masked, edits)
}

// ownTitleDash reports a singular known reference followed by a spaced dash and
// text that begins with that change's raw title, returning where the dash ends.
func ownTitleDash(line string, s refSpan, titles Titles) (int, bool) {
	if s.plural || len(s.items) != 1 || !s.items[0].known {
		return 0, false
	}
	title := titles[s.items[0].id]
	if title == "" {
		return 0, false
	}
	rest := line[s.end:]
	for _, dash := range []string{" — ", " – ", " - "} {
		if strings.HasPrefix(rest, dash) && strings.HasPrefix(rest[len(dash):], title) {
			return s.end + len(dash), true
		}
	}
	return 0, false
}

// joinList joins parts as `A, B and C`.
func joinList(parts []string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// cleanTitle makes the one pass every inserted title gets: a references-only
// parenthesis of convertible forms is removed, any other convertible form
// becomes "another issue" ("other issues" for a plural list), "#N" and ADR
// references stay, and double quotes become single quotes. It never looks a
// title up, so it always terminates. Code spans in the title are left alone.
func cleanTitle(title string, titles Titles) string {
	masked, restore := maskCode(title)
	spans := findRefs(masked, titles)
	render := func(s refSpan) string {
		if !s.anyKnown() {
			return masked[s.start:s.end]
		}
		if s.plural && s.allKnown() {
			return "other issues"
		}
		if !s.plural {
			return "another issue"
		}
		var edits []refEdit
		for _, it := range s.items {
			if it.known {
				edits = append(edits, refEdit{start: it.start - s.start, end: it.end - s.start, text: "another issue"})
			}
		}
		return applyEdits(masked[s.start:s.end], edits)
	}
	edits, consumed := parenEdits(masked, spans, render)
	for i, s := range spans {
		if consumed[i] || !s.anyKnown() {
			continue
		}
		edits = append(edits, refEdit{start: s.start, end: s.end, text: render(s)})
	}
	out := applyEdits(masked, edits)
	return restore(strings.ReplaceAll(out, `"`, "'"))
}
