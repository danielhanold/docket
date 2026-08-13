package document

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"unicode/utf8"
)

var (
	// markerPrefixRE matches a line that BEGINS like a Docket marker. It is what
	// separates "malformed marker" from ordinary authored prose: a line opening
	// with this prefix is claiming to be a marker, so failing the grammar is a
	// defect rather than content. The prefix is column-zero exact, so an
	// indented marker-shaped line stays prose.
	markerPrefixRE = regexp.MustCompile(`^<!-- docket:`)
	// markerRE is the exact marker grammar: a lower-case hyphenated name, the
	// start/end kind, and — on a start marker only — a parenthesized annotation
	// carrying no closing paren of its own.
	markerRE = regexp.MustCompile(
		`^<!-- docket:([a-z][a-z0-9-]*):(start|end)(?: \(([^)]*)\))? -->$`)
	// codeFenceRE matches a CommonMark fenced-code-block delimiter: up to three
	// leading spaces, then a run of at least three backticks or tildes.
	codeFenceRE = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)

// blockNameRE is markerRE's name group on its own, so a block a patch CREATES
// is held to exactly the grammar the scanner will later have to recognize. It
// is deliberately not validKey: marker names are hyphenated, field keys are
// underscored.
var blockNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// validBlockName reports whether name may appear in a Docket marker line.
func validBlockName(name string) bool { return blockNameRE.MatchString(name) }

// validAnnotation reports whether s may sit inside a start marker's
// parentheses. The grammar's annotation group is `[^)]*` on a single line, so a
// closing paren or any control character (a newline above all) would produce a
// line the scanner rejects as malformed.
func validAnnotation(s string) error {
	if !utf8.ValidString(s) {
		return invalidValue("annotation is not valid UTF-8")
	}
	for _, r := range s {
		if r == ')' {
			return invalidValue("annotation may not contain a closing parenthesis")
		}
		if r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			return invalidValue(fmt.Sprintf("control character %q in annotation", r))
		}
	}
	return nil
}

// startMarkerLine renders the canonical start marker for name, with annotation
// when one is given. endMarkerLine renders its partner. Neither carries a line
// terminator; the caller supplies the document's own.
func startMarkerLine(name, annotation string) string {
	if annotation == "" {
		return "<!-- docket:" + name + ":start -->"
	}
	return "<!-- docket:" + name + ":start (" + annotation + ") -->"
}

func endMarkerLine(name string) string { return "<!-- docket:" + name + ":end -->" }

// Block is one located managed marker block.
type Block struct {
	Name       string
	Annotation string // start-marker annotation without parentheses; "" if none
	Start      Span   // the full start-marker line, terminator included
	End        Span   // the full end-marker line, terminator included
	Interior   Span   // bytes strictly between the two marker lines
}

// marker is one recognized marker line, before population validation.
type marker struct {
	name       string
	isStart    bool
	annotation string
	span       Span // the full marker line, terminator included
	line       int  // 1-based document line
}

// scanBlocks discovers the managed-block population in the document body and
// validates it as a whole. firstLine is the index of the first body line: the
// frontmatter's bytes are never marker territory, so a document with
// frontmatter starts scanning after its closing fence.
//
// Discovery is Markdown-fence-aware — marker-shaped example text inside a
// fenced code block is authored content — and validation is all-or-nothing: an
// invalid marker anywhere fails Parse, so no caller ever sees a half-trusted
// population.
func scanBlocks(src []byte, lines []sourceLine, firstLine int) ([]Block, error) {
	markers, err := scanMarkers(src, lines, firstLine)
	if err != nil {
		return nil, err
	}
	return pairMarkers(markers)
}

// scanMarkers returns the marker lines outside code fences, in source order.
func scanMarkers(src []byte, lines []sourceLine, firstLine int) ([]marker, error) {
	var markers []marker
	fence := ""          // the open fence's delimiter run; "" when not inside a fence
	fenceChar := byte(0) // '`' or '~'
	for i := firstLine; i < len(lines); i++ {
		text := src[lines[i].text.Start:lines[i].text.End]
		if run, ok := fenceRun(text); ok {
			switch {
			case fence == "":
				fence, fenceChar = run, run[0]
			case run[0] == fenceChar && len(run) >= len(fence) && isBareFence(text, run):
				fence, fenceChar = "", 0
			}
			continue
		}
		if fence != "" {
			continue // inside a fenced code block: marker-shaped text is content
		}
		m := markerRE.FindSubmatch(text)
		if m == nil {
			if markerPrefixRE.Match(text) {
				return nil, &Error{
					Kind:   KindMalformedMarker,
					Offset: lines[i].span.Start,
					Line:   i + 1,
					Column: 1,
					Msg:    "line opens as a docket marker but does not match the marker grammar",
				}
			}
			continue
		}
		isStart := string(m[2]) == "start"
		annotation := string(m[3])
		if !isStart && annotation != "" {
			return nil, &Error{
				Kind:   KindMalformedMarker,
				Name:   string(m[1]),
				Offset: lines[i].span.Start,
				Line:   i + 1,
				Column: 1,
				Msg:    "only a start marker may carry an annotation",
			}
		}
		markers = append(markers, marker{
			name:       string(m[1]),
			isStart:    isStart,
			annotation: annotation,
			span:       lines[i].span,
			line:       i + 1,
		})
	}
	return markers, nil
}

// fenceRun returns the leading fence delimiter run of a code-fence line, and
// whether the line is one.
func fenceRun(text []byte) (string, bool) {
	m := codeFenceRE.FindSubmatch(text)
	if m == nil {
		return "", false
	}
	return string(m[1]), true
}

// isBareFence reports whether the line carries nothing but its delimiter run
// and whitespace — CommonMark's rule for a CLOSING fence, which (unlike an
// opener) may not carry an info string.
func isBareFence(text []byte, run string) bool {
	rest := bytes.TrimLeft(text, " ")[len(run):]
	return len(bytes.TrimSpace(rest)) == 0
}

// pairMarkers validates the whole marker population and returns the blocks it
// describes. A start while another block is open is nesting; an end with no
// open block, or one naming a different block, is out of order; a start left
// open at EOF is dangling; and a name completing more than one pair is a
// duplicate. Every one of them rejects the document.
func pairMarkers(markers []marker) ([]Block, error) {
	var blocks []Block
	seen := make(map[string]int, len(markers)/2) // name -> line of its first pair
	var open marker
	isOpen := false
	for _, m := range markers {
		switch {
		case m.isStart && isOpen:
			return nil, imbalance(m, "start marker for '"+m.name+
				"' appears inside the still-open block '"+open.name+"'")
		case m.isStart:
			open, isOpen = m, true
		case !isOpen:
			return nil, imbalance(m, "end marker has no matching start marker")
		case m.name != open.name:
			return nil, imbalance(m, "end marker closes '"+m.name+
				"' but the open block is '"+open.name+"'")
		default:
			if first, dup := seen[m.name]; dup {
				return nil, imbalance(m, "block name is used by more than one marker pair "+
					"(first pair opens on line "+strconv.Itoa(first)+"); each name occurs at most once")
			}
			seen[m.name] = open.line
			blocks = append(blocks, Block{
				Name:       open.name,
				Annotation: open.annotation,
				Start:      open.span,
				End:        m.span,
				Interior:   Span{open.span.End, m.span.Start},
			})
			isOpen = false
		}
	}
	if isOpen {
		return nil, imbalance(open, "start marker for '"+open.name+"' has no matching end marker")
	}
	return blocks, nil
}

// imbalance builds the marker-imbalance error for a marker line.
func imbalance(m marker, msg string) error {
	return &Error{
		Kind:   KindMarkerImbalance,
		Name:   m.name,
		Offset: m.span.Start,
		Line:   m.line,
		Column: 1,
		Msg:    msg,
	}
}
