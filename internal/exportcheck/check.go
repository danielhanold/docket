// Package exportcheck is the export's self-check: it scans an exported Markdown
// file for docket machinery the export should have removed, and lists the lines
// that mention docket by name.
//
// It is written from the export rules, not from the transform, and it shares no
// code or patterns with it (TestDetectorSharesNothingWithTransform): a detector
// built from the transform's own patterns would agree with it by construction and
// could never find anything. It has its own fence and code-span awareness for the
// same reason.
package exportcheck

// Rule names a kind of leftover.
const (
	RuleMarker      = "marker"
	RuleFrontmatter = "frontmatter"
	RuleBanner      = "banner"
	RuleLink        = "link"
	RuleChangeRef   = "change-reference"
	RuleStamp       = "stamp"
	RuleHeading     = "heading"
)

// Leftover is one piece of docket machinery found in the output.
type Leftover struct {
	Line int    // 1-based output line
	Text string // the matched text
	Rule string
}

// Mentions lists the lines, code included, that name docket or dckt as a word.
type Mentions struct {
	Count int
	Lines []int
}

// Report is the self-check's result.
type Report struct {
	Leftovers []Leftover
	Mentions  Mentions
}

// Check scans md. known reports whether a number is a real change id; only a
// change-reference form whose number is known is a leftover, since the export
// leaves unknown numbers as written.
func Check(md []byte, known func(id int) bool) Report {
	lines := splitLines(string(md))
	code := fencedLines(lines)
	var r Report

	for i, line := range lines {
		if containsWord(line, "docket") || containsWord(line, "dckt") {
			r.Mentions.Count++
			r.Mentions.Lines = append(r.Mentions.Lines, i+1)
		}
	}

	seenH2 := false
	for i, line := range lines {
		if code[i] {
			continue
		}
		n := i + 1
		add := func(rule, text string) {
			r.Leftovers = append(r.Leftovers, Leftover{Line: n, Text: text, Rule: rule})
		}
		if level, _ := headingLevel(line); level == 2 {
			seenH2 = true
		}
		plain := maskSpans(line)

		if text, ok := findMarker(line, plain); ok {
			add(RuleMarker, text)
		}
		if i == 0 && trimRight(line) == "---" {
			add(RuleFrontmatter, "---")
		}
		if idx := indexOf(plain, "↩"); idx >= 0 {
			add(RuleBanner, trimRight(line))
		}
		if text, ok := findLink(line, plain); ok {
			add(RuleLink, text)
		}
		if text, ok := findChangeRef(line, plain, known); ok {
			add(RuleChangeRef, text)
		}
		if !seenH2 && isStamp(plain) {
			add(RuleStamp, trimRight(line))
		}
	}

	r.Leftovers = mergeByLine(r.Leftovers, headingLeftovers(lines, code))
	return r
}

// splitLines splits on LF, dropping a CR before it and the empty piece after a
// final newline.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, trimCR(s[start:i]))
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, trimCR(s[start:]))
	}
	return lines
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

func trimRight(s string) string {
	end := len(s)
	for end > 0 && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[:end]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isWord(c byte) bool   { return isLetter(c) || isDigit(c) || c == '_' }

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// hasFoldAt reports s[i:] starting with word, ignoring ASCII case.
func hasFoldAt(s string, i int, word string) bool {
	if i < 0 || i+len(word) > len(s) {
		return false
	}
	for j := 0; j < len(word); j++ {
		if lower(s[i+j]) != word[j] {
			return false
		}
	}
	return true
}

// containsWord reports word (lower case) in s as a whole word, ignoring case.
func containsWord(s, word string) bool {
	for i := 0; i+len(word) <= len(s); i++ {
		if hasFoldAt(s, i, word) && (i == 0 || !isWord(s[i-1])) && (i+len(word) == len(s) || !isWord(s[i+len(word)])) {
			return true
		}
	}
	return false
}

// fencedLines marks every line that is a code-fence delimiter or sits inside a
// fenced block. A fence opens with three or more backticks or tildes indented at
// most three spaces, and closes with a run of the same character at least as
// long and nothing after it; an unclosed fence runs to the end.
func fencedLines(lines []string) []bool {
	code := make([]bool, len(lines))
	var fenceChar byte
	fenceLen := 0
	for i, line := range lines {
		indent := 0
		for indent < len(line) && indent < 4 && line[indent] == ' ' {
			indent++
		}
		run, char := 0, byte(0)
		if indent <= 3 && indent < len(line) && (line[indent] == '`' || line[indent] == '~') {
			char = line[indent]
			for indent+run < len(line) && line[indent+run] == char {
				run++
			}
		}
		rest := line[min(len(line), indent+run):]
		if fenceLen == 0 {
			if run >= 3 && !(char == '`' && indexOf(rest, "`") >= 0) {
				fenceChar, fenceLen = char, run
				code[i] = true
			}
			continue
		}
		code[i] = true
		if char == fenceChar && run >= fenceLen && trimRight(rest) == "" {
			fenceLen = 0
		}
	}
	return code
}

// maskSpans blanks every inline code span, backticks included, with NUL bytes of
// the same length, so offsets still index the original line. A backslash-escaped
// backtick does not open a span, and a backtick run with no closing run of the
// same length is literal text.
func maskSpans(line string) string {
	b := []byte(line)
	for i := 0; i < len(b); {
		if b[i] == '\\' {
			i += 2
			continue
		}
		if b[i] != '`' {
			i++
			continue
		}
		open := i
		for i < len(b) && b[i] == '`' {
			i++
		}
		n := i - open
		closeAt := -1
		for j := i; j < len(b); {
			if b[j] != '`' {
				j++
				continue
			}
			k := j
			for k < len(b) && b[k] == '`' {
				k++
			}
			if k-j == n {
				closeAt = k
				break
			}
			j = k
		}
		if closeAt < 0 {
			continue
		}
		for j := open; j < closeAt; j++ {
			b[j] = 0
		}
		i = closeAt
	}
	return string(b)
}

// findMarker finds managed-block marker text: an HTML comment that opens with a
// docket: or dckt: name.
func findMarker(line, plain string) (string, bool) {
	for i := 0; i+4 <= len(plain); i++ {
		if plain[i:i+4] != "<!--" {
			continue
		}
		j := i + 4
		for j < len(plain) && (plain[j] == ' ' || plain[j] == '\t') {
			j++
		}
		for _, ns := range []string{"docket:", "dckt:"} {
			if j+len(ns) <= len(plain) && plain[j:j+len(ns)] == ns {
				end := indexOf(plain[j:], "-->")
				if end < 0 {
					return line[i:], true
				}
				return line[i : j+end+3], true
			}
		}
	}
	return "", false
}

// findLink finds any link syntax: an inline link or image ("](" after a bracketed
// text), a full or collapsed reference link ("][" after a bracketed text), a
// link-reference definition line, an autolink "<scheme:…>", or an HTML anchor.
// A shortcut "[text]" alone is not flagged: it is a link only when a definition
// exists, and a definition is itself flagged.
func findLink(line, plain string) (string, bool) {
	// Definition: up to three spaces, "[label]:".
	indent := 0
	for indent < len(plain) && plain[indent] == ' ' {
		indent++
	}
	if indent <= 3 && indent < len(plain) && plain[indent] == '[' {
		if c := indexOf(plain[indent:], "]"); c > 1 && indent+c+1 < len(plain) && plain[indent+c+1] == ':' {
			return trimRight(line), true
		}
	}
	for i := 0; i < len(plain); i++ {
		switch plain[i] {
		case '[':
			c := indexOf(plain[i+1:], "]")
			if c < 0 {
				continue
			}
			after := i + 1 + c + 1
			if after < len(plain) && (plain[after] == '(' || plain[after] == '[') {
				start := i
				if start > 0 && plain[start-1] == '!' {
					start--
				}
				return line[start : after+1], true
			}
		case '<':
			if text, ok := autolinkAt(line, plain, i); ok {
				return text, true
			}
			if i+2 < len(plain) && lower(plain[i+1]) == 'a' && (plain[i+2] == ' ' || plain[i+2] == '\t' || plain[i+2] == '>') {
				return line[i : i+2], true
			}
		}
	}
	return "", false
}

// autolinkAt matches "<scheme:rest>" at i: a letter, then 1–31 letters, digits,
// '+', '.', or '-', a colon, and no space, '<', or '>' before the closing '>'.
func autolinkAt(line, plain string, i int) (string, bool) {
	j := i + 1
	if j >= len(plain) || !isLetter(plain[j]) {
		return "", false
	}
	k := j + 1
	for k < len(plain) && (isLetter(plain[k]) || isDigit(plain[k]) || plain[k] == '+' || plain[k] == '.' || plain[k] == '-') {
		k++
	}
	if n := k - j; n < 2 || n > 32 || k >= len(plain) || plain[k] != ':' {
		return "", false
	}
	for m := k + 1; m < len(plain); m++ {
		switch plain[m] {
		case '>':
			return line[i : m+1], true
		case ' ', '\t', '<':
			return "", false
		}
	}
	return "", false
}

// numberAt reads the digits at i and reports where they end, or ok=false when
// none start there.
func numberAt(s string, i int) (id, end int, ok bool) {
	end = i
	for end < len(s) && isDigit(s[end]) && end-i < 9 {
		id = id*10 + int(s[end]-'0')
		end++
	}
	if end == i || (end < len(s) && isDigit(s[end])) {
		return 0, end, false
	}
	return id, end, true
}

// wholeNumber reports the number ending at end is not followed by a word
// character or a dot that continues into a word (a filename extension).
func wholeNumber(s string, end int) bool {
	if end >= len(s) {
		return true
	}
	if isWord(s[end]) {
		return false
	}
	return !(s[end] == '.' && end+1 < len(s) && (isLetter(s[end+1]) || isDigit(s[end+1])))
}

// findChangeRef finds a change-reference form whose number is a known change:
// the word "change"/"changes" with a number (or, for the plural, a list), or a
// zero-padded four-digit id standing as its own word. ADR numbers, "PR #N", bare
// "#N", paths, and filenames are never forms.
func findChangeRef(line, plain string, known func(int) bool) (string, bool) {
	if text, ok := findWordRef(line, plain, known); ok {
		return text, true
	}
	return findPaddedRef(line, plain, known)
}

func findWordRef(line, plain string, known func(int) bool) (string, bool) {
	for i := 0; i+6 <= len(plain); i++ {
		if !hasFoldAt(plain, i, "change") || (i > 0 && isWord(plain[i-1])) {
			continue
		}
		j := i + 6
		plural := j < len(plain) && lower(plain[j]) == 's'
		if plural {
			j++
		}
		if j < len(plain) && isWord(plain[j]) {
			continue
		}
		ws := j
		for j < len(plain) && (plain[j] == ' ' || plain[j] == '\t') {
			j++
		}
		if j == ws {
			continue
		}
		if j < len(plain) && plain[j] == '#' {
			j++
		}
		id, end, ok := numberAt(plain, j)
		if !ok || !wholeNumber(plain, end) {
			continue
		}
		hit := known(id)
		for plural {
			k, sep := end, false
			for k < len(plain) && plain[k] == ' ' {
				k++
			}
			if k < len(plain) && plain[k] == ',' {
				sep = true
				k++
				for k < len(plain) && plain[k] == ' ' {
					k++
				}
			}
			if hasFoldAt(plain, k, "and ") {
				sep = true
				k += 4
				for k < len(plain) && plain[k] == ' ' {
					k++
				}
			}
			if !sep {
				break
			}
			if k < len(plain) && plain[k] == '#' {
				k++
			}
			next, nend, ok := numberAt(plain, k)
			if !ok || !wholeNumber(plain, nend) {
				break
			}
			hit = hit || known(next)
			end = nend
		}
		if hit {
			return line[i:end], true
		}
	}
	return "", false
}

func findPaddedRef(line, plain string, known func(int) bool) (string, bool) {
	for i := 0; i+4 <= len(plain); i++ {
		if plain[i] != '0' || (i > 0 && isDigit(plain[i-1])) {
			continue
		}
		id, end, ok := numberAt(plain, i)
		if !ok || end-i != 4 {
			continue
		}
		start := i
		if start > 0 && plain[start-1] == '#' {
			start--
		}
		if start > 0 {
			p := plain[start-1]
			if isWord(p) || p == '/' || p == '.' || p == '#' || p == '-' {
				continue
			}
		}
		if end < len(plain) {
			c := plain[end]
			if isWord(c) || c == '/' || c == '-' {
				continue
			}
			if c == '.' && end+1 < len(plain) && (isLetter(plain[end+1]) || isDigit(plain[end+1])) {
				continue
			}
		}
		if afterADR(plain, start) || afterPR(plain, start) {
			continue
		}
		if known(id) {
			return line[start:end], true
		}
	}
	return "", false
}

// afterADR reports "ADR " or "ADR-" (any case, whole word) right before at.
func afterADR(s string, at int) bool {
	i := at - 4
	return i >= 0 && hasFoldAt(s, i, "adr") && (s[at-1] == ' ' || s[at-1] == '-') && (i == 0 || !isWord(s[i-1]))
}

// afterPR reports "PR " (whole word) right before at.
func afterPR(s string, at int) bool {
	i := at - 3
	return i >= 0 && s[i:at] == "PR " && (i == 0 || !isWord(s[i-1]))
}

// isStamp reports a grooming change stamp by its shape, not by a list of
// spellings: a "Change:" or "**Change:**" label whatever follows it, opening
// the line or a "·"-separated field of it; or a line opening with "Change" and
// a quoted converted title, or with "Change", an optional "[" and "#", and a
// number that ends the stamp: end of line, one of ".,;:]", or a space and then
// "(", "·", or a dash.
func isStamp(plain string) bool {
	s := trimRight(plain)
	if changeLabel(s) {
		return true
	}
	for t := s; ; {
		i := indexOf(t, "·")
		if i < 0 {
			break
		}
		t = t[i+len("·"):]
		for hasPrefix(t, " ") {
			t = t[1:]
		}
		if changeLabel(t) {
			return true
		}
	}
	if hasPrefix(s, "Change \"") {
		return true
	}
	if !hasPrefix(s, "Change ") {
		return false
	}
	k := len("Change ")
	if k < len(s) && s[k] == '[' {
		k++
	}
	if k < len(s) && s[k] == '#' {
		k++
	}
	_, end, ok := numberAt(s, k)
	if !ok {
		return false
	}
	rest := s[end:]
	if rest == "" || indexOf(".,;:]", rest[:1]) >= 0 {
		return true
	}
	if rest[0] != ' ' {
		return false
	}
	rest = rest[1:]
	for _, p := range []string{"(", "·", "—", "–"} {
		if hasPrefix(rest, p) {
			return true
		}
	}
	return false
}

// changeLabel reports s opening with a "Change:" or "**Change:**" label.
func changeLabel(s string) bool {
	for _, label := range []string{"**Change:**", "Change:"} {
		if hasPrefix(s, label) && (len(s) == len(label) || s[len(label)] == ' ' || s[len(label)] == '\t') {
			return true
		}
	}
	return false
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

// headingLevel returns the level of an ATX heading line (up to three spaces of
// indent, one to six '#', then a space, a tab, or the end), or 0.
func headingLevel(line string) (int, bool) {
	i := 0
	for i < len(line) && i < 4 && line[i] == ' ' {
		i++
	}
	if i > 3 {
		return 0, false
	}
	n := 0
	for i+n < len(line) && line[i+n] == '#' {
		n++
	}
	if n < 1 || n > 6 {
		return 0, false
	}
	if i+n < len(line) && line[i+n] != ' ' && line[i+n] != '\t' {
		return 0, false
	}
	return n, true
}

// headingLeftovers applies the heading rules outside code: exactly one H1, the
// H1 is the first heading, and no level is skipped going deeper. With no H1 at
// all, the first heading (or line 1, with no heading) is reported.
func headingLeftovers(lines []string, code []bool) []Leftover {
	var out []Leftover
	report := func(i int) {
		out = append(out, Leftover{Line: i + 1, Text: trimRight(lines[i]), Rule: RuleHeading})
	}
	h1s, first, prev := 0, -1, 0
	for i, line := range lines {
		if code[i] {
			continue
		}
		level, ok := headingLevel(line)
		if !ok {
			continue
		}
		switch {
		case level == 1:
			h1s++
			if h1s > 1 || first >= 0 {
				report(i)
			}
		case prev > 0 && level > prev+1:
			report(i)
		}
		if first < 0 {
			first = i
		}
		prev = level
	}
	if h1s == 0 {
		if first >= 0 {
			report(first)
		} else if len(lines) > 0 {
			report(0)
		}
	}
	return out
}

// mergeByLine merges two line-ordered leftover lists into one, keeping a's
// entries first on a shared line.
func mergeByLine(a, b []Leftover) []Leftover {
	out := make([]Leftover, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		if j >= len(b) || (i < len(a) && a[i].Line <= b[j].Line) {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	return out
}
